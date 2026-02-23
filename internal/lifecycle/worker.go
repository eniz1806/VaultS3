package lifecycle

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type Worker struct {
	store    *metadata.Store
	engine   storage.Engine
	interval time.Duration
}

func NewWorker(store *metadata.Store, engine storage.Engine, intervalSecs int) *Worker {
	return &Worker{
		store:    store,
		engine:   engine,
		interval: time.Duration(intervalSecs) * time.Second,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once at startup
	w.scan()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

func (w *Worker) scan() {
	now := time.Now().UTC().Unix()

	// Get all buckets and their lifecycle rules
	buckets, err := w.store.ListBuckets()
	if err != nil {
		log.Printf("[Lifecycle] error listing buckets: %v", err)
		return
	}

	rules := make(map[string]*metadata.LifecycleRule)
	for _, b := range buckets {
		rule, err := w.store.GetLifecycleRule(b.Name)
		if err != nil {
			continue // no rule for this bucket
		}
		if rule.Status != "Enabled" {
			continue
		}
		rules[b.Name] = rule
	}

	if len(rules) == 0 {
		return
	}

	var expired int

	w.store.ScanObjects(func(meta metadata.ObjectMeta) bool {
		rule, ok := rules[meta.Bucket]
		if !ok {
			return true // no rule for this bucket, continue
		}

		// Check prefix filter
		if rule.Prefix != "" && !strings.HasPrefix(meta.Key, rule.Prefix) {
			return true
		}

		// Check if object is expired
		expiryTime := meta.LastModified + int64(rule.ExpirationDays)*86400
		if expiryTime > now {
			return true // not expired yet
		}

		// Skip delete markers
		if meta.DeleteMarker {
			return true
		}

		// Check object lock
		if meta.LegalHold {
			return true // skip locked objects
		}
		if meta.RetentionMode != "" && meta.RetentionUntil > 0 && now < meta.RetentionUntil {
			return true // skip retained objects
		}

		// Check if bucket is versioned
		versioning, _ := w.store.GetBucketVersioning(meta.Bucket)
		if versioning == "Enabled" && meta.VersionID != "" {
			// For versioned objects, we skip lifecycle auto-delete of individual versions
			// Only delete the "latest pointer" by creating a delete marker
			// This is simplified — full S3 lifecycle has more nuanced behavior
			return true
		}

		// Delete the object
		if err := w.engine.DeleteObject(meta.Bucket, meta.Key); err != nil {
			log.Printf("[Lifecycle] error deleting %s/%s: %v", meta.Bucket, meta.Key, err)
			return true
		}
		w.store.DeleteObjectMeta(meta.Bucket, meta.Key)
		expired++

		return true
	})

	if expired > 0 {
		log.Printf("[Lifecycle] deleted %d expired object(s)", expired)
	}
}
