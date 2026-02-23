package search

import (
	"strings"
	"sync"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
)

// Result represents a search result entry.
type Result struct {
	Bucket       string            `json:"bucket"`
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	LastModified int64             `json:"last_modified"`
	ETag         string            `json:"etag"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// entry is an internal index entry.
type entry struct {
	meta   metadata.ObjectMeta
	bucket string
	key    string
	// searchable text: lowercased concatenation of key, content type, tags
	text string
}

// Index provides in-memory full-text search over object metadata.
type Index struct {
	mu      sync.RWMutex
	entries map[string]*entry // key: "bucket/key"
	store   *metadata.Store
}

// NewIndex creates a new search index.
func NewIndex(store *metadata.Store) *Index {
	return &Index{
		entries: make(map[string]*entry),
		store:   store,
	}
}

// Build populates the index by scanning all object metadata from BoltDB.
func (idx *Index) Build() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.entries = make(map[string]*entry)

	err := idx.store.IterateAllObjects(func(bucket, key string, meta metadata.ObjectMeta) bool {
		if meta.DeleteMarker {
			return true // skip delete markers
		}
		e := &entry{
			meta:   meta,
			bucket: bucket,
			key:    key,
			text:   buildSearchText(bucket, key, meta),
		}
		idx.entries[bucket+"/"+key] = e
		return true
	})

	// IterateAllObjects returns a "stop" error when iteration is halted early
	if err != nil && err.Error() == "stop" {
		return nil
	}
	return err
}

// Update adds or updates an entry in the index.
func (idx *Index) Update(bucket, key string, meta metadata.ObjectMeta) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if meta.DeleteMarker {
		delete(idx.entries, bucket+"/"+key)
		return
	}

	idx.entries[bucket+"/"+key] = &entry{
		meta:   meta,
		bucket: bucket,
		key:    key,
		text:   buildSearchText(bucket, key, meta),
	}
}

// Remove deletes an entry from the index.
func (idx *Index) Remove(bucket, key string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.entries, bucket+"/"+key)
}

// Search finds objects matching the query string.
// Supports: plain text (substring match on key/tags/content-type),
// "tag:key=value" for tag filtering, "type:content/type" for content-type filtering.
func (idx *Index) Search(query, bucket string, limit int) []Result {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	// Parse structured query parts
	var tagFilters []tagFilter
	var typeFilter string
	var textTerms []string

	for _, part := range strings.Fields(query) {
		lower := strings.ToLower(part)
		if strings.HasPrefix(lower, "tag:") {
			tf := parseTagFilter(strings.TrimPrefix(part, "tag:"))
			if tf.key != "" {
				tagFilters = append(tagFilters, tf)
				continue
			}
		}
		if strings.HasPrefix(lower, "type:") {
			typeFilter = strings.ToLower(strings.TrimPrefix(part, "type:"))
			continue
		}
		textTerms = append(textTerms, strings.ToLower(part))
	}

	var results []Result
	for _, e := range idx.entries {
		if bucket != "" && e.bucket != bucket {
			continue
		}

		// Check tag filters
		if !matchTagFilters(e.meta.Tags, tagFilters) {
			continue
		}

		// Check content-type filter
		if typeFilter != "" && !strings.Contains(strings.ToLower(e.meta.ContentType), typeFilter) {
			continue
		}

		// Check text terms (all must match)
		if len(textTerms) > 0 {
			allMatch := true
			for _, term := range textTerms {
				if !strings.Contains(e.text, term) {
					allMatch = false
					break
				}
			}
			if !allMatch {
				continue
			}
		}

		results = append(results, Result{
			Bucket:       e.bucket,
			Key:          e.key,
			Size:         e.meta.Size,
			ContentType:  e.meta.ContentType,
			LastModified: e.meta.LastModified,
			ETag:         e.meta.ETag,
			Tags:         e.meta.Tags,
		})

		if len(results) >= limit {
			break
		}
	}

	return results
}

// Count returns the number of indexed entries.
func (idx *Index) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

type tagFilter struct {
	key   string
	value string // empty means match any value
}

func parseTagFilter(s string) tagFilter {
	parts := strings.SplitN(s, "=", 2)
	tf := tagFilter{key: parts[0]}
	if len(parts) == 2 {
		tf.value = parts[1]
	}
	return tf
}

func matchTagFilters(tags map[string]string, filters []tagFilter) bool {
	for _, f := range filters {
		v, ok := tags[f.key]
		if !ok {
			return false
		}
		if f.value != "" && !strings.EqualFold(v, f.value) {
			return false
		}
	}
	return true
}

func buildSearchText(bucket, key string, meta metadata.ObjectMeta) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(bucket))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(key))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(meta.ContentType))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(meta.ETag))
	for k, v := range meta.Tags {
		b.WriteByte(' ')
		b.WriteString(strings.ToLower(k))
		b.WriteByte('=')
		b.WriteString(strings.ToLower(v))
	}
	if meta.LastModified > 0 {
		b.WriteByte(' ')
		b.WriteString(time.Unix(0, meta.LastModified).Format("2006-01-02"))
	}
	return b.String()
}
