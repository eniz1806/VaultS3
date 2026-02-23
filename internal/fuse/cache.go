//go:build !windows

package fuse

import (
	"container/list"
	"sync"
	"time"
)

const defaultBlockSize = 256 * 1024 // 256KB

// blockKey identifies a cached block.
type blockKey struct {
	bucket     string
	key        string
	blockIndex int64
}

type blockEntry struct {
	lruElem *list.Element
	bk      blockKey
	data    []byte
}

// BlockCache is a thread-safe LRU cache for fixed-size object data blocks.
type BlockCache struct {
	mu       sync.Mutex
	maxBytes int64
	curBytes int64
	lru      *list.List
	entries  map[blockKey]*blockEntry
}

// NewBlockCache creates a block cache with the given max size in bytes.
func NewBlockCache(maxBytes int64) *BlockCache {
	return &BlockCache{
		maxBytes: maxBytes,
		lru:      list.New(),
		entries:  make(map[blockKey]*blockEntry),
	}
}

// Get returns cached block data or nil on miss.
func (c *BlockCache) Get(bucket, key string, blockIdx int64) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := blockKey{bucket, key, blockIdx}
	if e, ok := c.entries[k]; ok {
		c.lru.MoveToFront(e.lruElem)
		out := make([]byte, len(e.data))
		copy(out, e.data)
		return out
	}
	return nil
}

// Put stores block data, evicting LRU entries if over budget.
func (c *BlockCache) Put(bucket, key string, blockIdx int64, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := blockKey{bucket, key, blockIdx}
	if _, ok := c.entries[k]; ok {
		return
	}
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	e := &blockEntry{bk: k, data: dataCopy}
	e.lruElem = c.lru.PushFront(e)
	c.entries[k] = e
	c.curBytes += int64(len(dataCopy))
	for c.curBytes > c.maxBytes && c.lru.Len() > 0 {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		old := oldest.Value.(*blockEntry)
		c.lru.Remove(oldest)
		delete(c.entries, old.bk)
		c.curBytes -= int64(len(old.data))
	}
}

// Invalidate removes all cached blocks for a given object.
func (c *BlockCache) Invalidate(bucket, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var toDelete []blockKey
	for k := range c.entries {
		if k.bucket == bucket && k.key == key {
			toDelete = append(toDelete, k)
		}
	}
	for _, k := range toDelete {
		e := c.entries[k]
		c.lru.Remove(e.lruElem)
		c.curBytes -= int64(len(e.data))
		delete(c.entries, k)
	}
}

// metaEntry caches a HEAD result.
type metaEntry struct {
	size   int64
	expiry time.Time
}

// listCacheEntry caches a LIST result.
type listCacheEntry struct {
	objects []objectEntry
	expiry  time.Time
}

// MetaCache is a thread-safe TTL cache for HEAD and LIST results.
type MetaCache struct {
	mu      sync.Mutex
	headTTL time.Duration
	listTTL time.Duration
	heads   map[string]*metaEntry      // key: "bucket/key"
	lists   map[string]*listCacheEntry // key: "bucket:prefix:delimiter"
}

// NewMetaCache creates a metadata cache with given TTLs.
func NewMetaCache(headTTL, listTTL time.Duration) *MetaCache {
	return &MetaCache{
		headTTL: headTTL,
		listTTL: listTTL,
		heads:   make(map[string]*metaEntry),
		lists:   make(map[string]*listCacheEntry),
	}
}

// GetHead returns cached size for an object, or ok=false on miss/expiry.
func (m *MetaCache) GetHead(bucket, key string) (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, exists := m.heads[bucket+"/"+key]
	if !exists || time.Now().After(e.expiry) {
		if exists {
			delete(m.heads, bucket+"/"+key)
		}
		return 0, false
	}
	return e.size, true
}

// PutHead caches a HEAD result.
func (m *MetaCache) PutHead(bucket, key string, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heads[bucket+"/"+key] = &metaEntry{
		size:   size,
		expiry: time.Now().Add(m.headTTL),
	}
}

// GetList returns cached LIST result, or ok=false on miss/expiry.
func (m *MetaCache) GetList(bucket, prefix, delimiter string) ([]objectEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cacheKey := bucket + ":" + prefix + ":" + delimiter
	e, exists := m.lists[cacheKey]
	if !exists || time.Now().After(e.expiry) {
		if exists {
			delete(m.lists, cacheKey)
		}
		return nil, false
	}
	out := make([]objectEntry, len(e.objects))
	copy(out, e.objects)
	return out, true
}

// PutList caches a LIST result.
func (m *MetaCache) PutList(bucket, prefix, delimiter string, objects []objectEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]objectEntry, len(objects))
	copy(cp, objects)
	m.lists[bucket+":"+prefix+":"+delimiter] = &listCacheEntry{
		objects: cp,
		expiry:  time.Now().Add(m.listTTL),
	}
}

// InvalidateObject removes HEAD cache for an object and all LIST caches for the bucket.
func (m *MetaCache) InvalidateObject(bucket, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.heads, bucket+"/"+key)
	for k := range m.lists {
		if len(k) >= len(bucket) && k[:len(bucket)] == bucket {
			delete(m.lists, k)
		}
	}
}

// sigCacheKey identifies a derived signing key by secret+region.
type sigCacheKey struct {
	secret string
	region string
}

type sigCacheEntry struct {
	datestamp string
	key       []byte
}

var (
	sigCacheMu      sync.Mutex
	sigCacheEntries = make(map[sigCacheKey]sigCacheEntry)
)

// getCachedDerivedKey returns the HMAC derived key, recomputing only on date change.
func getCachedDerivedKey(secret, datestamp, region, service string) []byte {
	ck := sigCacheKey{secret, region}
	sigCacheMu.Lock()
	defer sigCacheMu.Unlock()
	if e, ok := sigCacheEntries[ck]; ok && e.datestamp == datestamp {
		return e.key
	}
	k := deriveKey(secret, datestamp, region, service)
	sigCacheEntries[ck] = sigCacheEntry{datestamp: datestamp, key: k}
	return k
}
