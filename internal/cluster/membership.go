package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ClusterStatus is the response for the /cluster/status endpoint.
type ClusterStatus struct {
	NodeID    string       `json:"node_id"`
	State     string       `json:"state"` // Leader, Follower, Candidate
	Leader    string       `json:"leader"`
	LeaderID  string       `json:"leader_id"`
	Servers   []ServerInfo `json:"servers"`
	Stats     map[string]string `json:"stats"`
}

// ServerInfo describes a single cluster member.
type ServerInfo struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Suffrage string `json:"suffrage"` // Voter, Nonvoter
}

// StatusHandler returns an HTTP handler for /cluster/status.
func (n *Node) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := ClusterStatus{
			NodeID:   n.cfg.NodeID,
			State:    n.raft.State().String(),
			Leader:   n.LeaderAddr(),
			LeaderID: n.LeaderID(),
			Stats:    n.Stats(),
		}

		if servers, err := n.Servers(); err == nil {
			for _, s := range servers {
				status.Servers = append(status.Servers, ServerInfo{
					ID:       string(s.ID),
					Address:  string(s.Address),
					Suffrage: s.Suffrage.String(),
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// JoinHandler returns an HTTP handler for POST /cluster/join.
// Body: {"node_id": "...", "addr": "host:port"}
func (n *Node) JoinHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			NodeID string `json:"node_id"`
			Addr   string `json:"addr"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.NodeID == "" || req.Addr == "" {
			http.Error(w, "node_id and addr are required", http.StatusBadRequest)
			return
		}

		if err := n.Join(req.NodeID, req.Addr); err != nil {
			if err == ErrNotLeader {
				// Redirect to leader
				leaderAddr := n.LeaderAddr()
				if leaderAddr == "" {
					http.Error(w, "no leader available", http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Location", fmt.Sprintf("http://%s/cluster/join", apiAddrFromRaft(leaderAddr)))
				http.Error(w, "not leader, redirect to: "+leaderAddr, http.StatusTemporaryRedirect)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": fmt.Sprintf("node %s joined at %s", req.NodeID, req.Addr),
		})
	}
}

// LeaveHandler returns an HTTP handler for POST /cluster/leave.
// Body: {"node_id": "..."}
func (n *Node) LeaveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			NodeID string `json:"node_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.NodeID == "" {
			http.Error(w, "node_id is required", http.StatusBadRequest)
			return
		}

		if err := n.Leave(req.NodeID); err != nil {
			if err == ErrNotLeader {
				http.Error(w, "not leader", http.StatusTemporaryRedirect)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": fmt.Sprintf("node %s removed", req.NodeID),
		})
	}
}

// apiAddrFromRaft converts a Raft address (host:raftPort) to an API address (host:apiPort).
// Since we can't know the API port from the Raft port, we use a convention:
// Raft port 9001 → API port 9000 (raftPort - 1).
func apiAddrFromRaft(raftAddr string) string {
	parts := strings.Split(raftAddr, ":")
	if len(parts) != 2 {
		return raftAddr
	}
	return parts[0] + ":9000"
}
