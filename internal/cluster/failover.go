package cluster

import (
	"log/slog"
	"net/http"
)

// FailoverProxy extends the basic Proxy with failure-aware routing.
// When the primary node is down, requests are forwarded to the next
// healthy replica in the hash ring.
type FailoverProxy struct {
	*Proxy
	detector *FailureDetector
}

// NewFailoverProxy creates a failover-aware proxy.
func NewFailoverProxy(proxy *Proxy, detector *FailureDetector) *FailoverProxy {
	return &FailoverProxy{
		Proxy:    proxy,
		detector: detector,
	}
}

// ShouldProxy returns the target node for a request, accounting for failed nodes.
// If the primary node is down, it returns the next healthy replica.
// Returns empty string if this node should handle the request.
func (f *FailoverProxy) ShouldProxy(bucket, key string) string {
	if bucket == "" {
		return ""
	}

	replicaCount := f.placement.ReplicaCount
	if replicaCount <= 1 {
		replicaCount = 2 // at least check primary + 1 replica for failover
	}

	nodes := f.ring.GetNodes(bucket, key, replicaCount)
	selfID := f.node.NodeID()

	for _, nodeID := range nodes {
		if nodeID == selfID {
			// This node is in the replica set — handle locally
			return ""
		}
		if f.detector == nil || !f.detector.IsNodeDown(nodeID) {
			// Found a healthy node that isn't us — proxy to it
			return nodeID
		}
		slog.Debug("failover: skipping down node",
			"node_id", nodeID,
			"bucket", bucket,
			"key", key,
		)
	}

	// All nodes are down or unreachable — handle locally as last resort
	slog.Warn("failover: all target nodes down, handling locally",
		"bucket", bucket,
		"key", key,
	)
	return ""
}

// ForwardWithRetry attempts to forward a request to the primary node,
// falling back to replicas if the primary fails.
func (f *FailoverProxy) ForwardWithRetry(w http.ResponseWriter, r *http.Request, bucket, key string) bool {
	target := f.ShouldProxy(bucket, key)
	if target == "" {
		return false // handle locally
	}
	f.ForwardRequest(w, r, target)
	return true
}

// OnNodeDown is called when the detector declares a node as down.
// It updates the hash ring and proxy state.
func (f *FailoverProxy) OnNodeDown(nodeID string) {
	slog.Warn("failover: node down, traffic will route to replicas",
		"node_id", nodeID,
	)
	// Don't remove from ring — the ring is used to determine ownership.
	// The failover logic in ShouldProxy skips down nodes.
	// This preserves object placement for when the node recovers.
}

// OnNodeRecover is called when a previously down node comes back.
func (f *FailoverProxy) OnNodeRecover(nodeID string) {
	slog.Info("failover: node recovered, resuming normal routing",
		"node_id", nodeID,
	)
}
