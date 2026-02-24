package lifecycle

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type Worker struct {
	store              *metadata.Store
	engine             storage.Engine
	interval           time.Duration
	auditRetentionDays int
}

func NewWorker(store *metadata.Store, engine storage.Engine, intervalSecs, auditRetentionDays int) *Worker {
	return &Worker{
		store:              store,
		engine:             engine,
		interval:           time.Duration(intervalSecs) * time.Second,
		auditRetentionDays: auditRetentionDays,
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
		slog.Error("lifecycle error listing buckets", "error", err)
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
			slog.Error("lifecycle error deleting object", "bucket", meta.Bucket, "key", meta.Key, "error", err)
			return true
		}
		w.store.DeleteObjectMeta(meta.Bucket, meta.Key)
		expired++

		return true
	})

	if expired > 0 {
		slog.Info("lifecycle deleted expired objects", "count", expired)
	}

	// Prune old audit entries
	if w.auditRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -w.auditRetentionDays)
		pruned, err := w.store.PruneAuditEntries(cutoff)
		if err != nil {
			slog.Error("lifecycle error pruning audit entries", "error", err)
		} else if pruned > 0 {
			slog.Info("lifecycle pruned audit entries", "count", pruned)
		}
	}

	// Clean up expired STS keys
	deleted, err := w.store.DeleteExpiredAccessKeys()
	if err != nil {
		slog.Error("lifecycle error cleaning expired keys", "error", err)
	} else if deleted > 0 {
		slog.Info("lifecycle removed expired STS keys", "count", deleted)
	}
}
