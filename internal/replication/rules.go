package replication

// ReplicationRule defines a per-bucket replication rule.
type ReplicationRule struct {
	ID          string            `json:"id" xml:"ID"`
	Status      string            `json:"status" xml:"Status"`                       // "Enabled" or "Disabled"
	Priority    int               `json:"priority,omitempty" xml:"Priority,omitempty"`
	Prefix      string            `json:"prefix,omitempty" xml:"Filter>Prefix,omitempty"`
	TagFilter   map[string]string `json:"tag_filter,omitempty"`
	Destination RuleDestination   `json:"destination" xml:"Destination"`
	DeleteMarkerReplication string `json:"delete_marker_replication,omitempty" xml:"DeleteMarkerReplication>Status,omitempty"` // "Enabled" or "Disabled"
	ExistingObjectReplication string `json:"existing_object_replication,omitempty" xml:"ExistingObjectReplication>Status,omitempty"`
}

// RuleDestination specifies where to replicate.
type RuleDestination struct {
	Bucket       string `json:"bucket" xml:"Bucket"`
	StorageClass string `json:"storage_class,omitempty" xml:"StorageClass,omitempty"`
}

// ReplicationConfig holds all rules for a bucket.
type ReplicationConfig struct {
	Role  string            `json:"role,omitempty" xml:"Role,omitempty"`
	Rules []ReplicationRule `json:"rules" xml:"Rule"`
}

// MatchingRules returns all enabled rules that match a given key and tags.
func (c *ReplicationConfig) MatchingRules(key string, tags map[string]string) []ReplicationRule {
	if c == nil {
		return nil
	}
	var matched []ReplicationRule
	for _, rule := range c.Rules {
		if rule.Status != "Enabled" {
			continue
		}
		if rule.Prefix != "" {
			found := false
			if len(key) >= len(rule.Prefix) && key[:len(rule.Prefix)] == rule.Prefix {
				found = true
			}
			if !found {
				continue
			}
		}
		if len(rule.TagFilter) > 0 {
			match := true
			for k, v := range rule.TagFilter {
				if tags[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		matched = append(matched, rule)
	}
	return matched
}
