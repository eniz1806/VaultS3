package cluster

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// Proxy handles forwarding S3 requests to the correct node in the cluster
// based on the hash ring placement.
type Proxy struct {
	ring      *HashRing
	node      *Node
	placement PlacementConfig
	nodeAddrs map[string]string // nodeID → "host:apiPort"
	mu        sync.RWMutex
	proxies   map[string]*httputil.ReverseProxy // cached per-node proxies
}

// NewProxy creates a new cluster proxy.
func NewProxy(ring *HashRing, node *Node, placement PlacementConfig, nodeAddrs map[string]string) *Proxy {
	applyPlacementDefaults(&placement)
	return &Proxy{
		ring:      ring,
		node:      node,
		placement: placement,
		nodeAddrs: nodeAddrs,
		proxies:   make(map[string]*httputil.ReverseProxy),
	}
}

// ShouldProxy checks if a request for the given bucket/key should be proxied
// to another node. Returns the target node ID if proxying is needed,
// or empty string if this node should handle it.
func (p *Proxy) ShouldProxy(bucket, key string) string {
	if bucket == "" {
		// Service-level operations (ListBuckets) — handle locally
		return ""
	}

	// For bucket-level operations (key == ""), hash on just the bucket
	hashKey := key
	if hashKey == "" {
		hashKey = ""
	}

	primaryNode := p.ring.GetNode(bucket, hashKey)
	if primaryNode == "" || primaryNode == p.node.NodeID() {
		return ""
	}

	return primaryNode
}

// ForwardRequest proxies an HTTP request to the specified target node.
func (p *Proxy) ForwardRequest(w http.ResponseWriter, r *http.Request, targetNodeID string) {
	p.mu.RLock()
	addr, ok := p.nodeAddrs[targetNodeID]
	p.mu.RUnlock()

	if !ok {
		slog.Warn("proxy: unknown target node", "node_id", targetNodeID)
		http.Error(w, "cluster node not found", http.StatusBadGateway)
		return
	}

	proxy := p.getOrCreateProxy(targetNodeID, addr)

	// Mark as internal cluster proxy to prevent infinite proxy loops
	r.Header.Set("X-VaultS3-Proxy", p.node.NodeID())

	slog.Debug("proxy: forwarding request",
		"method", r.Method,
		"path", r.URL.Path,
		"from", p.node.NodeID(),
		"to", targetNodeID,
		"addr", addr,
	)

	proxy.ServeHTTP(w, r)
}

// IsProxied checks if a request was already proxied from another node.
func IsProxied(r *http.Request) bool {
	return r.Header.Get("X-VaultS3-Proxy") != ""
}

// UpdateNodeAddr updates the API address for a node.
func (p *Proxy) UpdateNodeAddr(nodeID, addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodeAddrs[nodeID] = addr
	// Invalidate cached proxy
	delete(p.proxies, nodeID)
}

// RemoveNodeAddr removes the address mapping for a node.
func (p *Proxy) RemoveNodeAddr(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.nodeAddrs, nodeID)
	delete(p.proxies, nodeID)
}

func (p *Proxy) getOrCreateProxy(nodeID, addr string) *httputil.ReverseProxy {
	p.mu.RLock()
	if proxy, ok := p.proxies[nodeID]; ok {
		p.mu.RUnlock()
		return proxy
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if proxy, ok := p.proxies[nodeID]; ok {
		return proxy
	}

	target, err := url.Parse(fmt.Sprintf("http://%s", addr))
	if err != nil {
		slog.Error("proxy: invalid target URL", "addr", addr, "error", err)
		target = &url.URL{Scheme: "http", Host: addr}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("proxy: upstream error", "target", nodeID, "addr", addr, "error", err)
		http.Error(w, "upstream node unavailable", http.StatusBadGateway)
	}

	p.proxies[nodeID] = proxy
	return proxy
}

// NodeAddrs returns a copy of the node address map.
func (p *Proxy) NodeAddrs() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	addrs := make(map[string]string, len(p.nodeAddrs))
	for k, v := range p.nodeAddrs {
		addrs[k] = v
	}
	return addrs
}

// Ring returns the underlying hash ring.
func (p *Proxy) Ring() *HashRing {
	return p.ring
}

// Placement returns the placement config.
func (p *Proxy) Placement() PlacementConfig {
	return p.placement
}
