package cluster

import (
	"fmt"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
)

const defaultVnodes = 128

// HashRing implements consistent hashing with virtual nodes.
type HashRing struct {
	mu       sync.RWMutex
	vnodes   int
	ring     []uint64       // sorted hash values
	nodeMap  map[uint64]string // hash → nodeID
	nodes    map[string]bool   // set of nodeIDs
}

// NewHashRing creates a hash ring with the given number of virtual nodes per physical node.
func NewHashRing(vnodes int) *HashRing {
	if vnodes <= 0 {
		vnodes = defaultVnodes
	}
	return &HashRing{
		vnodes:  vnodes,
		nodeMap: make(map[uint64]string),
		nodes:   make(map[string]bool),
	}
}

// AddNode adds a node to the ring.
func (h *HashRing) AddNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.nodes[nodeID] {
		return
	}
	h.nodes[nodeID] = true

	for i := 0; i < h.vnodes; i++ {
		hash := xxhash.Sum64String(fmt.Sprintf("%s-%d", nodeID, i))
		h.ring = append(h.ring, hash)
		h.nodeMap[hash] = nodeID
	}

	sort.Slice(h.ring, func(i, j int) bool { return h.ring[i] < h.ring[j] })
}

// RemoveNode removes a node from the ring.
func (h *HashRing) RemoveNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.nodes[nodeID] {
		return
	}
	delete(h.nodes, nodeID)

	// Rebuild ring without this node
	newRing := make([]uint64, 0, len(h.ring)-h.vnodes)
	for _, hash := range h.ring {
		if h.nodeMap[hash] != nodeID {
			newRing = append(newRing, hash)
		} else {
			delete(h.nodeMap, hash)
		}
	}
	h.ring = newRing
}

// GetNode returns the primary node for a given bucket/key combination.
func (h *HashRing) GetNode(bucket, key string) string {
	nodes := h.GetNodes(bucket, key, 1)
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0]
}

// GetNodes returns up to n distinct nodes for a given bucket/key (primary + replicas).
func (h *HashRing) GetNodes(bucket, key string, n int) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.ring) == 0 {
		return nil
	}

	hash := xxhash.Sum64String(bucket + "/" + key)
	idx := sort.Search(len(h.ring), func(i int) bool { return h.ring[i] >= hash })
	if idx >= len(h.ring) {
		idx = 0
	}

	nodeCount := len(h.nodes)
	if n > nodeCount {
		n = nodeCount
	}

	seen := make(map[string]bool, n)
	result := make([]string, 0, n)

	for i := 0; i < len(h.ring) && len(result) < n; i++ {
		pos := (idx + i) % len(h.ring)
		nodeID := h.nodeMap[h.ring[pos]]
		if !seen[nodeID] {
			seen[nodeID] = true
			result = append(result, nodeID)
		}
	}

	return result
}

// NodeCount returns the number of nodes in the ring.
func (h *HashRing) NodeCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// Nodes returns all node IDs in the ring.
func (h *HashRing) Nodes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]string, 0, len(h.nodes))
	for id := range h.nodes {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// HasNode checks if a node is in the ring.
func (h *HashRing) HasNode(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.nodes[nodeID]
}
