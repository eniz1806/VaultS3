package replication

import (
	"encoding/json"
	"sort"
)

// VectorClock tracks causal ordering across multiple sites.
// Each entry maps a site ID to a logical counter.
type VectorClock map[string]uint64

// NewVectorClock creates an empty vector clock.
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// ParseVectorClock deserializes a vector clock from JSON bytes.
func ParseVectorClock(data []byte) (VectorClock, error) {
	if len(data) == 0 {
		return NewVectorClock(), nil
	}
	vc := NewVectorClock()
	if err := json.Unmarshal(data, &vc); err != nil {
		return nil, err
	}
	return vc, nil
}

// Bytes serializes the vector clock to JSON.
func (vc VectorClock) Bytes() []byte {
	data, _ := json.Marshal(vc)
	return data
}

// Increment advances the counter for the given site.
func (vc VectorClock) Increment(siteID string) {
	vc[siteID]++
}

// Get returns the counter for a site (0 if absent).
func (vc VectorClock) Get(siteID string) uint64 {
	return vc[siteID]
}

// Merge combines two vector clocks by taking the max of each entry.
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	merged := NewVectorClock()
	for k, v := range vc {
		merged[k] = v
	}
	for k, v := range other {
		if v > merged[k] {
			merged[k] = v
		}
	}
	return merged
}

// Ordering represents the causal relationship between two vector clocks.
type Ordering int

const (
	Equal          Ordering = iota
	HappenedBefore          // vc < other
	HappenedAfter           // vc > other
	Concurrent              // neither dominates
)

// Compare determines the causal ordering between two vector clocks.
func (vc VectorClock) Compare(other VectorClock) Ordering {
	allKeys := make(map[string]struct{})
	for k := range vc {
		allKeys[k] = struct{}{}
	}
	for k := range other {
		allKeys[k] = struct{}{}
	}

	hasLess := false
	hasGreater := false

	for k := range allKeys {
		a := vc[k]
		b := other[k]
		if a < b {
			hasLess = true
		}
		if a > b {
			hasGreater = true
		}
		if hasLess && hasGreater {
			return Concurrent
		}
	}

	if hasLess && !hasGreater {
		return HappenedBefore
	}
	if hasGreater && !hasLess {
		return HappenedAfter
	}
	return Equal
}

// Clone returns a deep copy of the vector clock.
func (vc VectorClock) Clone() VectorClock {
	c := make(VectorClock, len(vc))
	for k, v := range vc {
		c[k] = v
	}
	return c
}

// Sum returns the total of all counters (useful for tie-breaking).
func (vc VectorClock) Sum() uint64 {
	var total uint64
	for _, v := range vc {
		total += v
	}
	return total
}

// Sites returns sorted list of site IDs in the clock.
func (vc VectorClock) Sites() []string {
	sites := make([]string, 0, len(vc))
	for k := range vc {
		sites = append(sites, k)
	}
	sort.Strings(sites)
	return sites
}
