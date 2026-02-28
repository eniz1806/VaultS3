package metadata

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketsBucket           = []byte("buckets")
	keysBucket              = []byte("access_keys")
	objectsBucket           = []byte("objects")
	multipartBucket         = []byte("multipart_uploads")
	partsBucket             = []byte("multipart_parts")
	policiesBucket          = []byte("bucket_policies")
	objectVersionsBucket    = []byte("object_versions")
	lifecycleBucket         = []byte("lifecycle_rules")
	websitesBucket          = []byte("website_configs")
	iamUsersBucket          = []byte("iam_users")
	iamGroupsBucket         = []byte("iam_groups")
	iamPoliciesBucket       = []byte("iam_policies")
	corsBucket              = []byte("cors_configs")
	auditBucket             = []byte("audit_trail")
	notificationBucket      = []byte("notification_configs")
	replicationQueueBucket  = []byte("replication_queue")
	replicationStatusBucket = []byte("replication_status")
	backupHistoryBucket     = []byte("backup_history")
	versionTagsBucket       = []byte("version_tags")
	lambdaTriggersBucket    = []byte("lambda_triggers")
	encryptionConfigBucket  = []byte("encryption_configs")
	publicAccessBlockBucket = []byte("public_access_blocks")
	loggingConfigBucket     = []byte("logging_configs")
	changeLogBucket         = []byte("change_log")
)

type Store struct {
	db *bolt.DB
}

type LifecycleRule struct {
	ExpirationDays int    `json:"expiration_days"`
	Prefix         string `json:"prefix,omitempty"`
	Status         string `json:"status"` // "Enabled" or "Disabled"
}

type WebsiteConfig struct {
	IndexDocument string `json:"index_document"`
	ErrorDocument string `json:"error_document,omitempty"`
}

type BucketInfo struct {
	Name                 string            `json:"name"`
	CreatedAt            time.Time         `json:"created_at"`
	MaxSizeBytes         int64             `json:"max_size_bytes,omitempty"`         // 0 = unlimited
	MaxObjects           int64             `json:"max_objects,omitempty"`            // 0 = unlimited
	Versioning           string            `json:"versioning,omitempty"`             // "Enabled", "Suspended", or ""
	DefaultRetentionMode string            `json:"default_retention_mode,omitempty"` // "GOVERNANCE" or "COMPLIANCE"
	DefaultRetentionDays int               `json:"default_retention_days,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}

type AccessKey struct {
	AccessKey    string    `json:"access_key"`
	SecretKey    string    `json:"secret_key"`
	CreatedAt    time.Time `json:"created_at"`
	UserID       string    `json:"user_id,omitempty"`
	ExpiresAt    int64     `json:"expires_at,omitempty"`     // unix timestamp, 0=never
	SessionToken string    `json:"session_token,omitempty"`  // STS session identifier
	SourceUserID string    `json:"source_user_id,omitempty"` // user who created this STS key
}

type IAMUser struct {
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	PolicyARNs   []string  `json:"policy_arns,omitempty"`
	Groups       []string  `json:"groups,omitempty"`
	AllowedCIDRs []string  `json:"allowed_cidrs,omitempty"`
}

type IAMGroup struct {
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	PolicyARNs []string  `json:"policy_arns,omitempty"`
}

type IAMPolicy struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Document  string    `json:"document"` // raw JSON policy document
}

type CORSRule struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	MaxAgeSecs     int      `json:"max_age_secs,omitempty"`
}

type CORSConfig struct {
	Rules []CORSRule `json:"rules"`
}

type LambdaTriggerFilter struct {
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
}

type LambdaTrigger struct {
	ID                string              `json:"id"`
	FunctionURL       string              `json:"function_url"`
	Events            []string            `json:"events"`
	Filters           LambdaTriggerFilter `json:"filters,omitempty"`
	OutputBucket      string              `json:"output_bucket,omitempty"`
	OutputKeyTemplate string              `json:"output_key_template,omitempty"`
	IncludeBody       bool                `json:"include_body"`
	MaxBodySize       int64               `json:"max_body_size,omitempty"`
}

type BucketLambdaConfig struct {
	Triggers []LambdaTrigger `json:"triggers"`
}

type BucketEncryptionConfig struct {
	SSEAlgorithm string `json:"sse_algorithm"` // "AES256" or "aws:kms"
	KMSKeyID     string `json:"kms_key_id,omitempty"`
}

type PublicAccessBlockConfig struct {
	BlockPublicAcls       bool `json:"block_public_acls"`
	IgnorePublicAcls      bool `json:"ignore_public_acls"`
	BlockPublicPolicy     bool `json:"block_public_policy"`
	RestrictPublicBuckets bool `json:"restrict_public_buckets"`
}

type BucketLoggingConfig struct {
	TargetBucket string `json:"target_bucket"`
	TargetPrefix string `json:"target_prefix,omitempty"`
}

type NotificationFilterRule struct {
	Name  string `json:"name"` // "prefix" or "suffix"
	Value string `json:"value"`
}

type NotificationEndpointConfig struct {
	ID       string                   `json:"id"`
	Events   []string                 `json:"events"`
	Filters  []NotificationFilterRule `json:"filters,omitempty"`
	Endpoint string                   `json:"endpoint"`
}

type BucketNotificationConfig struct {
	Webhooks []NotificationEndpointConfig `json:"webhooks"`
}

type AuditEntry struct {
	Time       int64  `json:"time"`      // unix nanos
	Principal  string `json:"principal"` // access key
	UserID     string `json:"user_id"`
	Action     string `json:"action"`   // s3:GetObject, etc.
	Resource   string `json:"resource"` // arn:aws:s3:::bucket/key
	Effect     string `json:"effect"`   // Allow, Deny
	SourceIP   string `json:"source_ip"`
	StatusCode int    `json:"status_code"`
}

type ReplicationEvent struct {
	ID          uint64 `json:"id"`
	Type        string `json:"type"` // "put" or "delete"
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	VersionID   string `json:"version_id,omitempty"`
	ETag        string `json:"etag,omitempty"`
	Peer        string `json:"peer"`
	Size        int64  `json:"size"`
	RetryCount  int    `json:"retry_count"`
	NextRetryAt int64  `json:"next_retry_at"` // unix timestamp
	CreatedAt   int64  `json:"created_at"`    // unix timestamp
}

type ReplicationStatus struct {
	Peer         string `json:"peer"`
	QueueDepth   int    `json:"queue_depth"`
	LastSyncTime int64  `json:"last_sync_time"`
	LastError    string `json:"last_error,omitempty"`
	TotalSynced  int64  `json:"total_synced"`
	TotalFailed  int64  `json:"total_failed"`
}

type BackupRecord struct {
	ID          string `json:"id"`
	Type        string `json:"type"`   // "full" or "incremental"
	Target      string `json:"target"` // target name
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	ObjectCount int64  `json:"object_count"`
	TotalSize   int64  `json:"total_size"`
	Status      string `json:"status"` // "running", "completed", "failed"
	Error       string `json:"error,omitempty"`
}

type MultipartUpload struct {
	UploadID    string `json:"upload_id"`
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	CreatedAt   int64  `json:"created_at"`
}

type PartInfo struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

type ObjectMeta struct {
	Bucket         string            `json:"bucket"`
	Key            string            `json:"key"`
	ContentType    string            `json:"content_type"`
	ETag           string            `json:"etag"`
	Size           int64             `json:"size"`
	LastModified   int64             `json:"last_modified"`
	Tags           map[string]string `json:"tags,omitempty"`
	VersionID      string            `json:"version_id,omitempty"`
	IsLatest       bool              `json:"is_latest,omitempty"`
	DeleteMarker   bool              `json:"delete_marker,omitempty"`
	LegalHold      bool              `json:"legal_hold,omitempty"`
	RetentionMode  string            `json:"retention_mode,omitempty"`   // "GOVERNANCE" or "COMPLIANCE"
	RetentionUntil int64             `json:"retention_until,omitempty"`  // unix timestamp
	Tier           string            `json:"tier,omitempty"`             // "hot" or "cold", default "hot"
	LastAccessTime int64             `json:"last_access_time,omitempty"` // unix timestamp
	VectorClock    json.RawMessage   `json:"vector_clock,omitempty"`    // vector clock for active-active replication
}

func NewStore(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(keysBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(objectsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(multipartBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(partsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(policiesBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(objectVersionsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(lifecycleBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(websitesBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(iamUsersBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(iamGroupsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(iamPoliciesBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(corsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(auditBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(notificationBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(replicationQueueBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(replicationStatusBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(backupHistoryBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(versionTagsBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(lambdaTriggersBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(encryptionConfigBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(publicAccessBlockBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(loggingConfigBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(changeLogBucket); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init metadata buckets: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Bucket operations

func (s *Store) CreateBucket(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		if b.Get([]byte(name)) != nil {
			return fmt.Errorf("bucket already exists: %s", name)
		}
		info := BucketInfo{Name: name, CreatedAt: time.Now().UTC()}
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), data)
	})
}

func (s *Store) DeleteBucket(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		if b.Get([]byte(name)) == nil {
			return fmt.Errorf("bucket not found: %s", name)
		}
		return b.Delete([]byte(name))
	})
}

func (s *Store) GetBucket(name string) (*BucketInfo, error) {
	var info *BucketInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", name)
		}
		info = &BucketInfo{}
		return json.Unmarshal(data, info)
	})
	return info, err
}

func (s *Store) ListBuckets() ([]BucketInfo, error) {
	var buckets []BucketInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		return b.ForEach(func(k, v []byte) error {
			var info BucketInfo
			if err := json.Unmarshal(v, &info); err != nil {
				return err
			}
			buckets = append(buckets, info)
			return nil
		})
	})
	return buckets, err
}

func (s *Store) BucketExists(name string) bool {
	exists := false
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		exists = b.Get([]byte(name)) != nil
		return nil
	})
	return exists
}

// Bucket policy operations

func (s *Store) PutBucketPolicy(bucket string, policyJSON []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(policiesBucket)
		return b.Put([]byte(bucket), policyJSON)
	})
}

func (s *Store) GetBucketPolicy(bucket string) ([]byte, error) {
	var policy []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(policiesBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no policy for bucket: %s", bucket)
		}
		policy = make([]byte, len(data))
		copy(policy, data)
		return nil
	})
	return policy, err
}

func (s *Store) DeleteBucketPolicy(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(policiesBucket)
		return b.Delete([]byte(bucket))
	})
}

// IsBucketPublicRead checks if a bucket policy allows public read access.
func (s *Store) IsBucketPublicRead(bucket string) bool {
	policyJSON, err := s.GetBucketPolicy(bucket)
	if err != nil {
		return false
	}
	// Check for public-read pattern in policy
	var policy struct {
		Statement []struct {
			Effect    string      `json:"Effect"`
			Principal interface{} `json:"Principal"`
			Action    interface{} `json:"Action"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		return false
	}
	for _, stmt := range policy.Statement {
		if stmt.Effect != "Allow" {
			continue
		}
		// Check Principal == "*"
		if p, ok := stmt.Principal.(string); ok && p == "*" {
			// Check Action contains s3:GetObject
			switch a := stmt.Action.(type) {
			case string:
				if a == "s3:GetObject" || a == "s3:*" {
					return true
				}
			case []interface{}:
				for _, action := range a {
					if s, ok := action.(string); ok && (s == "s3:GetObject" || s == "s3:*") {
						return true
					}
				}
			}
		}
	}
	return false
}

// Bucket quota operations

func (s *Store) UpdateBucketQuota(name string, maxSizeBytes, maxObjects int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", name)
		}
		var info BucketInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		info.MaxSizeBytes = maxSizeBytes
		info.MaxObjects = maxObjects
		updated, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), updated)
	})
}

// Access key operations

func (s *Store) CreateAccessKey(key AccessKey) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(keysBucket)
		data, err := json.Marshal(key)
		if err != nil {
			return err
		}
		return b.Put([]byte(key.AccessKey), data)
	})
}

func (s *Store) GetAccessKey(accessKey string) (*AccessKey, error) {
	var key *AccessKey
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(keysBucket)
		data := b.Get([]byte(accessKey))
		if data == nil {
			return fmt.Errorf("access key not found")
		}
		key = &AccessKey{}
		return json.Unmarshal(data, key)
	})
	return key, err
}

func (s *Store) ListAccessKeys() ([]AccessKey, error) {
	var keys []AccessKey
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(keysBucket)
		return b.ForEach(func(k, v []byte) error {
			var key AccessKey
			if err := json.Unmarshal(v, &key); err != nil {
				return err
			}
			keys = append(keys, key)
			return nil
		})
	})
	return keys, err
}

func (s *Store) DeleteAccessKey(accessKey string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(keysBucket)
		return b.Delete([]byte(accessKey))
	})
}

// Object metadata operations

func objectMetaKey(bucket, key string) []byte {
	return []byte(bucket + "/" + key)
}

func (s *Store) PutObjectMeta(meta ObjectMeta) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put(objectMetaKey(meta.Bucket, meta.Key), data)
	})
}

func (s *Store) GetObjectMeta(bucket, key string) (*ObjectMeta, error) {
	var meta *ObjectMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		data := b.Get(objectMetaKey(bucket, key))
		if data == nil {
			return fmt.Errorf("object metadata not found: %s/%s", bucket, key)
		}
		meta = &ObjectMeta{}
		return json.Unmarshal(data, meta)
	})
	return meta, err
}

func (s *Store) DeleteObjectMeta(bucket, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		return b.Delete(objectMetaKey(bucket, key))
	})
}

// UpdateLastAccess updates the last access time on an object's metadata.
func (s *Store) UpdateLastAccess(bucket, key string) {
	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		data := b.Get(objectMetaKey(bucket, key))
		if data == nil {
			return nil
		}
		var meta ObjectMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil
		}
		meta.LastAccessTime = time.Now().Unix()
		updated, _ := json.Marshal(meta)
		return b.Put(objectMetaKey(bucket, key), updated)
	})
}

// AccessUpdater batches last-access time updates and flushes to BoltDB periodically.
type AccessUpdater struct {
	store    *Store
	mu       sync.Mutex
	dirty    map[string]int64
	interval time.Duration
}

// NewAccessUpdater creates an updater that flushes every flushInterval.
func NewAccessUpdater(store *Store, flushInterval time.Duration) *AccessUpdater {
	return &AccessUpdater{
		store:    store,
		dirty:    make(map[string]int64),
		interval: flushInterval,
	}
}

// MarkAccess records an access without writing to BoltDB.
func (u *AccessUpdater) MarkAccess(bucket, key string) {
	now := time.Now().Unix()
	u.mu.Lock()
	u.dirty[bucket+"\x00"+key] = now
	u.mu.Unlock()
}

// Flush writes all pending updates to BoltDB in a single transaction.
func (u *AccessUpdater) Flush() {
	u.mu.Lock()
	if len(u.dirty) == 0 {
		u.mu.Unlock()
		return
	}
	snapshot := u.dirty
	u.dirty = make(map[string]int64)
	u.mu.Unlock()

	u.store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		for compositeKey, ts := range snapshot {
			parts := strings.SplitN(compositeKey, "\x00", 2)
			if len(parts) != 2 {
				continue
			}
			dbKey := objectMetaKey(parts[0], parts[1])
			data := b.Get(dbKey)
			if data == nil {
				continue
			}
			var meta ObjectMeta
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}
			meta.LastAccessTime = ts
			updated, err := json.Marshal(meta)
			if err != nil {
				continue
			}
			b.Put(dbKey, updated)
		}
		return nil
	})
}

// Run starts the background flush loop. Cancel ctx to stop.
func (u *AccessUpdater) Run(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			u.Flush()
		case <-ctx.Done():
			u.Flush()
			return
		}
	}
}

// SetObjectTier updates the storage tier for an object.
func (s *Store) SetObjectTier(bucket, key, tier string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		data := b.Get(objectMetaKey(bucket, key))
		if data == nil {
			return fmt.Errorf("object not found")
		}
		var meta ObjectMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return err
		}
		meta.Tier = tier
		updated, _ := json.Marshal(meta)
		return b.Put(objectMetaKey(bucket, key), updated)
	})
}

// IterateAllObjects scans all object metadata entries.
// The callback receives bucket, key, and metadata. Return false to stop iteration.
func (s *Store) IterateAllObjects(fn func(bucket, key string, meta ObjectMeta) bool) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		return b.ForEach(func(k, v []byte) error {
			var meta ObjectMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				return nil // skip corrupt entries
			}
			parts := strings.SplitN(string(k), "/", 2)
			if len(parts) != 2 {
				return nil
			}
			if !fn(parts[0], parts[1], meta) {
				return fmt.Errorf("stop") // stop iteration
			}
			return nil
		})
	})
}

// Multipart upload operations

func (s *Store) CreateMultipartUpload(upload MultipartUpload) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(multipartBucket)
		data, err := json.Marshal(upload)
		if err != nil {
			return err
		}
		return b.Put([]byte(upload.UploadID), data)
	})
}

func (s *Store) GetMultipartUpload(uploadID string) (*MultipartUpload, error) {
	var upload *MultipartUpload
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(multipartBucket)
		data := b.Get([]byte(uploadID))
		if data == nil {
			return fmt.Errorf("multipart upload not found: %s", uploadID)
		}
		upload = &MultipartUpload{}
		return json.Unmarshal(data, upload)
	})
	return upload, err
}

func (s *Store) DeleteMultipartUpload(uploadID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(multipartBucket)
		if err := b.Delete([]byte(uploadID)); err != nil {
			return err
		}
		// Also delete all parts for this upload
		pb := tx.Bucket(partsBucket)
		prefix := []byte(uploadID + "/")
		c := pb.Cursor()
		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			if err := pb.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) PutPart(uploadID string, part PartInfo) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(partsBucket)
		data, err := json.Marshal(part)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("%s/%05d", uploadID, part.PartNumber)
		return b.Put([]byte(key), data)
	})
}

func (s *Store) ListParts(uploadID string) ([]PartInfo, error) {
	var parts []PartInfo
	prefix := []byte(uploadID + "/")
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(partsBucket)
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var part PartInfo
			if err := json.Unmarshal(v, &part); err != nil {
				return err
			}
			parts = append(parts, part)
		}
		return nil
	})
	return parts, err
}

func (s *Store) ListMultipartUploads(bucket string) ([]MultipartUpload, error) {
	var uploads []MultipartUpload
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(multipartBucket)
		return b.ForEach(func(k, v []byte) error {
			var u MultipartUpload
			if err := json.Unmarshal(v, &u); err != nil {
				return nil
			}
			if u.Bucket == bucket {
				uploads = append(uploads, u)
			}
			return nil
		})
	})
	return uploads, err
}

func (s *Store) PutBucketTags(bucket string, tags map[string]string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		var info BucketInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		info.Tags = tags
		updated, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), updated)
	})
}

func (s *Store) GetBucketTags(bucket string) (map[string]string, error) {
	var tags map[string]string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		var info BucketInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		tags = info.Tags
		return nil
	})
	return tags, err
}

func (s *Store) DeleteBucketTags(bucket string) error {
	return s.PutBucketTags(bucket, nil)
}

func (s *Store) DeleteBucketObjectMeta(bucket string) error {
	prefix := []byte(bucket + "/")
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		c := b.Cursor()
		for k, _ := c.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// Bucket versioning operations

func (s *Store) SetBucketVersioning(bucket, status string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		var info BucketInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		info.Versioning = status
		updated, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), updated)
	})
}

// SetBucketDefaultRetention sets the default object retention for a bucket.
func (s *Store) SetBucketDefaultRetention(bucket, mode string, days int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketsBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		var info BucketInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}
		info.DefaultRetentionMode = mode
		info.DefaultRetentionDays = days
		updated, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), updated)
	})
}

func (s *Store) GetBucketVersioning(bucket string) (string, error) {
	info, err := s.GetBucket(bucket)
	if err != nil {
		return "", err
	}
	return info.Versioning, nil
}

// Object version operations
// Key format in object_versions bucket: {bucket}\x00{key}\x00{versionID}

func versionKey(bucket, key, versionID string) []byte {
	return []byte(bucket + "\x00" + key + "\x00" + versionID)
}

func versionPrefix(bucket, key string) []byte {
	return []byte(bucket + "\x00" + key + "\x00")
}

func (s *Store) PutObjectVersion(meta ObjectMeta) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectVersionsBucket)
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put(versionKey(meta.Bucket, meta.Key, meta.VersionID), data)
	})
}

func (s *Store) GetObjectVersion(bucket, key, versionID string) (*ObjectMeta, error) {
	var meta *ObjectMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectVersionsBucket)
		data := b.Get(versionKey(bucket, key, versionID))
		if data == nil {
			return fmt.Errorf("version not found: %s/%s?versionId=%s", bucket, key, versionID)
		}
		meta = &ObjectMeta{}
		return json.Unmarshal(data, meta)
	})
	return meta, err
}

func (s *Store) DeleteObjectVersion(bucket, key, versionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectVersionsBucket)
		return b.Delete(versionKey(bucket, key, versionID))
	})
}

// ListObjectVersions returns all versions for a bucket, optionally filtered by prefix.
func (s *Store) ListObjectVersions(bucket, prefix, keyMarker, versionMarker string, maxKeys int) ([]ObjectMeta, bool, error) {
	var versions []ObjectMeta
	bucketPrefix := []byte(bucket + "\x00")

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectVersionsBucket)
		c := b.Cursor()

		startKey := bucketPrefix
		if keyMarker != "" {
			if versionMarker != "" {
				startKey = versionKey(bucket, keyMarker, versionMarker)
			} else {
				startKey = versionPrefix(bucket, keyMarker)
			}
		}

		for k, v := c.Seek(startKey); k != nil; k, v = c.Next() {
			// Stop if we've left this bucket's entries
			if len(k) < len(bucketPrefix) || string(k[:len(bucketPrefix)]) != string(bucketPrefix) {
				break
			}

			var meta ObjectMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				continue
			}

			// Skip the exact marker entry
			if keyMarker != "" && meta.Key == keyMarker && meta.VersionID == versionMarker {
				continue
			}

			// Apply prefix filter
			if prefix != "" && !strings.HasPrefix(meta.Key, prefix) {
				continue
			}

			versions = append(versions, meta)
			if maxKeys > 0 && len(versions) > maxKeys {
				return nil
			}
		}
		return nil
	})

	truncated := false
	if maxKeys > 0 && len(versions) > maxKeys {
		versions = versions[:maxKeys]
		truncated = true
	}

	return versions, truncated, err
}

// SetLatestVersion updates the objects bucket "latest pointer" for a key.
func (s *Store) SetLatestVersion(bucket, key, versionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		vb := tx.Bucket(objectVersionsBucket)
		data := vb.Get(versionKey(bucket, key, versionID))
		if data == nil {
			return fmt.Errorf("version not found")
		}
		ob := tx.Bucket(objectsBucket)
		return ob.Put(objectMetaKey(bucket, key), data)
	})
}

// UpdateObjectVersionMeta updates a version's metadata in-place (for lock operations).
func (s *Store) UpdateObjectVersionMeta(meta ObjectMeta) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectVersionsBucket)
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		if err := b.Put(versionKey(meta.Bucket, meta.Key, meta.VersionID), data); err != nil {
			return err
		}
		// Also update the objects bucket if this is the latest
		if meta.IsLatest {
			ob := tx.Bucket(objectsBucket)
			return ob.Put(objectMetaKey(meta.Bucket, meta.Key), data)
		}
		return nil
	})
}

// Lifecycle rule operations

func (s *Store) PutLifecycleRule(bucket string, rule LifecycleRule) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(lifecycleBucket)
		data, err := json.Marshal(rule)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetLifecycleRule(bucket string) (*LifecycleRule, error) {
	var rule *LifecycleRule
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(lifecycleBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return nil
		}
		rule = &LifecycleRule{}
		return json.Unmarshal(data, rule)
	})
	return rule, err
}

func (s *Store) DeleteLifecycleRule(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(lifecycleBucket)
		return b.Delete([]byte(bucket))
	})
}

// ScanObjects iterates all object metadata entries. Return false from fn to stop.
func (s *Store) ScanObjects(fn func(ObjectMeta) bool) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(objectsBucket)
		return b.ForEach(func(k, v []byte) error {
			var meta ObjectMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				return nil // skip malformed entries
			}
			if !fn(meta) {
				return fmt.Errorf("scan stopped") // break iteration
			}
			return nil
		})
	})
}

// Website config operations

func (s *Store) PutWebsiteConfig(bucket string, cfg WebsiteConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(websitesBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetWebsiteConfig(bucket string) (*WebsiteConfig, error) {
	var cfg *WebsiteConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(websitesBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no website config for bucket: %s", bucket)
		}
		cfg = &WebsiteConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteWebsiteConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(websitesBucket)
		return b.Delete([]byte(bucket))
	})
}

func (s *Store) IsBucketWebsite(bucket string) bool {
	_, err := s.GetWebsiteConfig(bucket)
	return err == nil
}

// IAM User operations

func (s *Store) CreateIAMUser(user IAMUser) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamUsersBucket)
		if b.Get([]byte(user.Name)) != nil {
			return fmt.Errorf("user already exists: %s", user.Name)
		}
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.Name), data)
	})
}

func (s *Store) GetIAMUser(name string) (*IAMUser, error) {
	var user *IAMUser
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamUsersBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("user not found: %s", name)
		}
		user = &IAMUser{}
		return json.Unmarshal(data, user)
	})
	return user, err
}

func (s *Store) ListIAMUsers() ([]IAMUser, error) {
	var users []IAMUser
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamUsersBucket)
		return b.ForEach(func(k, v []byte) error {
			var user IAMUser
			if err := json.Unmarshal(v, &user); err != nil {
				return err
			}
			users = append(users, user)
			return nil
		})
	})
	return users, err
}

func (s *Store) UpdateIAMUser(user IAMUser) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamUsersBucket)
		if b.Get([]byte(user.Name)) == nil {
			return fmt.Errorf("user not found: %s", user.Name)
		}
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.Name), data)
	})
}

func (s *Store) DeleteIAMUser(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamUsersBucket)
		return b.Delete([]byte(name))
	})
}

// IAM Group operations

func (s *Store) CreateIAMGroup(group IAMGroup) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamGroupsBucket)
		if b.Get([]byte(group.Name)) != nil {
			return fmt.Errorf("group already exists: %s", group.Name)
		}
		data, err := json.Marshal(group)
		if err != nil {
			return err
		}
		return b.Put([]byte(group.Name), data)
	})
}

func (s *Store) GetIAMGroup(name string) (*IAMGroup, error) {
	var group *IAMGroup
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamGroupsBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("group not found: %s", name)
		}
		group = &IAMGroup{}
		return json.Unmarshal(data, group)
	})
	return group, err
}

func (s *Store) ListIAMGroups() ([]IAMGroup, error) {
	var groups []IAMGroup
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamGroupsBucket)
		return b.ForEach(func(k, v []byte) error {
			var group IAMGroup
			if err := json.Unmarshal(v, &group); err != nil {
				return err
			}
			groups = append(groups, group)
			return nil
		})
	})
	return groups, err
}

func (s *Store) UpdateIAMGroup(group IAMGroup) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamGroupsBucket)
		if b.Get([]byte(group.Name)) == nil {
			return fmt.Errorf("group not found: %s", group.Name)
		}
		data, err := json.Marshal(group)
		if err != nil {
			return err
		}
		return b.Put([]byte(group.Name), data)
	})
}

func (s *Store) DeleteIAMGroup(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamGroupsBucket)
		return b.Delete([]byte(name))
	})
}

// IAM Policy operations

func (s *Store) CreateIAMPolicy(policy IAMPolicy) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamPoliciesBucket)
		if b.Get([]byte(policy.Name)) != nil {
			return fmt.Errorf("policy already exists: %s", policy.Name)
		}
		data, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		return b.Put([]byte(policy.Name), data)
	})
}

func (s *Store) GetIAMPolicy(name string) (*IAMPolicy, error) {
	var policy *IAMPolicy
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamPoliciesBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("policy not found: %s", name)
		}
		policy = &IAMPolicy{}
		return json.Unmarshal(data, policy)
	})
	return policy, err
}

func (s *Store) ListIAMPolicies() ([]IAMPolicy, error) {
	var policies []IAMPolicy
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamPoliciesBucket)
		return b.ForEach(func(k, v []byte) error {
			var policy IAMPolicy
			if err := json.Unmarshal(v, &policy); err != nil {
				return err
			}
			policies = append(policies, policy)
			return nil
		})
	})
	return policies, err
}

func (s *Store) UpdateIAMPolicy(policy IAMPolicy) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamPoliciesBucket)
		data, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		return b.Put([]byte(policy.Name), data)
	})
}

func (s *Store) DeleteIAMPolicy(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(iamPoliciesBucket)
		return b.Delete([]byte(name))
	})
}

// GetUserPolicies resolves all policies for a user (direct + via groups).
func (s *Store) GetUserPolicies(userName string) ([]IAMPolicy, error) {
	user, err := s.GetIAMUser(userName)
	if err != nil {
		return nil, err
	}

	policyNames := make(map[string]bool)
	for _, arn := range user.PolicyARNs {
		policyNames[arn] = true
	}

	// Add policies from groups
	for _, groupName := range user.Groups {
		group, err := s.GetIAMGroup(groupName)
		if err != nil {
			continue
		}
		for _, arn := range group.PolicyARNs {
			policyNames[arn] = true
		}
	}

	var policies []IAMPolicy
	for name := range policyNames {
		policy, err := s.GetIAMPolicy(name)
		if err != nil {
			continue
		}
		policies = append(policies, *policy)
	}
	return policies, nil
}

// CORS config operations

func (s *Store) PutCORSConfig(bucket string, cfg CORSConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(corsBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetCORSConfig(bucket string) (*CORSConfig, error) {
	var cfg *CORSConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(corsBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return nil
		}
		cfg = &CORSConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteCORSConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(corsBucket)
		return b.Delete([]byte(bucket))
	})
}

// Audit trail operations

func auditKey(nanos int64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(nanos))
	return key
}

func (s *Store) PutAuditEntry(entry AuditEntry) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(auditBucket)
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b.Put(auditKey(entry.Time), data)
	})
}

func (s *Store) ListAuditEntries(limit int, fromUnixNano, toUnixNano int64, user, bucket string) ([]AuditEntry, error) {
	var entries []AuditEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(auditBucket)
		c := b.Cursor()

		// Iterate in reverse (newest first)
		var k, v []byte
		if toUnixNano > 0 {
			k, v = c.Seek(auditKey(toUnixNano))
			if k == nil {
				k, v = c.Last()
			} else {
				k, v = c.Prev()
			}
		} else {
			k, v = c.Last()
		}

		for ; k != nil; k, v = c.Prev() {
			if fromUnixNano > 0 {
				ts := int64(binary.BigEndian.Uint64(k))
				if ts < fromUnixNano {
					break
				}
			}

			var entry AuditEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}

			// Apply filters
			if user != "" && entry.UserID != user && entry.Principal != user {
				continue
			}
			if bucket != "" && !strings.Contains(entry.Resource, bucket) {
				continue
			}

			entries = append(entries, entry)
			if limit > 0 && len(entries) >= limit {
				break
			}
		}
		return nil
	})
	return entries, err
}

func (s *Store) PruneAuditEntries(olderThan time.Time) (int, error) {
	cutoff := olderThan.UnixNano()
	pruned := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(auditBucket)
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			ts := int64(binary.BigEndian.Uint64(k))
			if ts >= cutoff {
				break
			}
			if err := b.Delete(k); err != nil {
				return err
			}
			pruned++
		}
		return nil
	})
	return pruned, err
}

// Notification config operations

func (s *Store) PutNotificationConfig(bucket string, cfg BucketNotificationConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(notificationBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetNotificationConfig(bucket string) (*BucketNotificationConfig, error) {
	var cfg *BucketNotificationConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(notificationBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no notification config for bucket: %s", bucket)
		}
		cfg = &BucketNotificationConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteNotificationConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(notificationBucket)
		return b.Delete([]byte(bucket))
	})
}

func (s *Store) ListNotificationConfigs() (map[string]BucketNotificationConfig, error) {
	configs := make(map[string]BucketNotificationConfig)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(notificationBucket)
		return b.ForEach(func(k, v []byte) error {
			var cfg BucketNotificationConfig
			if err := json.Unmarshal(v, &cfg); err != nil {
				return nil
			}
			configs[string(k)] = cfg
			return nil
		})
	})
	return configs, err
}

// Replication queue operations

func replicationKey(id uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, id)
	return key
}

func (s *Store) EnqueueReplication(event ReplicationEvent) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		id, _ := b.NextSequence()
		event.ID = id
		if event.CreatedAt == 0 {
			event.CreatedAt = time.Now().Unix()
		}
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return b.Put(replicationKey(id), data)
	})
}

func (s *Store) DequeueReplication(limit int, now int64) ([]ReplicationEvent, error) {
	var events []ReplicationEvent
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		c := b.Cursor()
		for k, v := c.First(); k != nil && len(events) < limit; k, v = c.Next() {
			var event ReplicationEvent
			if err := json.Unmarshal(v, &event); err != nil {
				continue
			}
			if event.NextRetryAt > now {
				continue
			}
			events = append(events, event)
		}
		return nil
	})
	return events, err
}

func (s *Store) AckReplication(id uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		return b.Delete(replicationKey(id))
	})
}

func (s *Store) NackReplication(id uint64, retryCount int, nextRetryAt int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		data := b.Get(replicationKey(id))
		if data == nil {
			return nil
		}
		var event ReplicationEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		event.RetryCount = retryCount
		event.NextRetryAt = nextRetryAt
		updated, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return b.Put(replicationKey(id), updated)
	})
}

func (s *Store) DeadLetterReplication(id uint64) error {
	return s.AckReplication(id) // remove from queue
}

func (s *Store) ReplicationQueueDepth() (int, error) {
	count := 0
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		count = b.Stats().KeyN
		return nil
	})
	return count, err
}

func (s *Store) ListReplicationQueue(limit int) ([]ReplicationEvent, error) {
	var events []ReplicationEvent
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationQueueBucket)
		c := b.Cursor()
		for k, v := c.First(); k != nil && len(events) < limit; k, v = c.Next() {
			var event ReplicationEvent
			if err := json.Unmarshal(v, &event); err != nil {
				continue
			}
			events = append(events, event)
		}
		return nil
	})
	return events, err
}

func (s *Store) PutReplicationStatus(status ReplicationStatus) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationStatusBucket)
		data, err := json.Marshal(status)
		if err != nil {
			return err
		}
		return b.Put([]byte(status.Peer), data)
	})
}

func (s *Store) GetReplicationStatuses() ([]ReplicationStatus, error) {
	var statuses []ReplicationStatus
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(replicationStatusBucket)
		return b.ForEach(func(k, v []byte) error {
			var status ReplicationStatus
			if err := json.Unmarshal(v, &status); err != nil {
				return nil
			}
			statuses = append(statuses, status)
			return nil
		})
	})
	return statuses, err
}

// Backup record operations

func (s *Store) PutBackupRecord(record BackupRecord) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupHistoryBucket)
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return b.Put([]byte(record.ID), data)
	})
}

func (s *Store) ListBackupRecords(limit int) ([]BackupRecord, error) {
	var records []BackupRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(backupHistoryBucket)
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var record BackupRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}
			records = append(records, record)
			if limit > 0 && len(records) >= limit {
				break
			}
		}
		return nil
	})
	return records, err
}

// Version tag operations

func (s *Store) PutVersionTag(key string, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(versionTagsBucket)
		return b.Put([]byte(key), data)
	})
}

func (s *Store) GetVersionTag(key string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(versionTagsBucket)
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("version tag not found")
		}
		data = make([]byte, len(v))
		copy(data, v)
		return nil
	})
	return data, err
}

func (s *Store) DeleteVersionTag(key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(versionTagsBucket)
		return b.Delete([]byte(key))
	})
}

func (s *Store) ListVersionTags(prefix string) ([][]byte, error) {
	var entries [][]byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(versionTagsBucket)
		c := b.Cursor()
		pfx := []byte(prefix)
		for k, v := c.Seek(pfx); k != nil && len(k) >= len(pfx) && string(k[:len(pfx)]) == string(pfx); k, v = c.Next() {
			data := make([]byte, len(v))
			copy(data, v)
			entries = append(entries, data)
		}
		return nil
	})
	return entries, err
}

// DeleteExpiredAccessKeys removes STS keys that have expired.
func (s *Store) DeleteExpiredAccessKeys() (int, error) {
	now := time.Now().Unix()
	deleted := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(keysBucket)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var key AccessKey
			if err := json.Unmarshal(v, &key); err != nil {
				continue
			}
			if key.ExpiresAt > 0 && key.ExpiresAt <= now {
				if err := b.Delete(k); err != nil {
					return err
				}
				deleted++
			}
		}
		return nil
	})
	return deleted, err
}

// Lambda trigger operations

func (s *Store) PutLambdaConfig(bucket string, cfg BucketLambdaConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(lambdaTriggersBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetLambdaConfig(bucket string) (*BucketLambdaConfig, error) {
	var cfg *BucketLambdaConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(lambdaTriggersBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no lambda config for bucket: %s", bucket)
		}
		cfg = &BucketLambdaConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteLambdaConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(lambdaTriggersBucket)
		return b.Delete([]byte(bucket))
	})
}

func (s *Store) ListLambdaConfigs() (map[string]BucketLambdaConfig, error) {
	configs := make(map[string]BucketLambdaConfig)
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(lambdaTriggersBucket)
		return b.ForEach(func(k, v []byte) error {
			var cfg BucketLambdaConfig
			if err := json.Unmarshal(v, &cfg); err != nil {
				return nil
			}
			configs[string(k)] = cfg
			return nil
		})
	})
	return configs, err
}

// Bucket encryption config operations

func (s *Store) PutEncryptionConfig(bucket string, cfg BucketEncryptionConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(encryptionConfigBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetEncryptionConfig(bucket string) (*BucketEncryptionConfig, error) {
	var cfg *BucketEncryptionConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(encryptionConfigBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no encryption config for bucket: %s", bucket)
		}
		cfg = &BucketEncryptionConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteEncryptionConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(encryptionConfigBucket)
		return b.Delete([]byte(bucket))
	})
}

// Public access block operations

func (s *Store) PutPublicAccessBlock(bucket string, cfg PublicAccessBlockConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(publicAccessBlockBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetPublicAccessBlock(bucket string) (*PublicAccessBlockConfig, error) {
	var cfg *PublicAccessBlockConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(publicAccessBlockBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no public access block for bucket: %s", bucket)
		}
		cfg = &PublicAccessBlockConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeletePublicAccessBlock(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(publicAccessBlockBucket)
		return b.Delete([]byte(bucket))
	})
}

// Bucket logging config operations

func (s *Store) PutLoggingConfig(bucket string, cfg BucketLoggingConfig) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(loggingConfigBucket)
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *Store) GetLoggingConfig(bucket string) (*BucketLoggingConfig, error) {
	var cfg *BucketLoggingConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(loggingConfigBucket)
		data := b.Get([]byte(bucket))
		if data == nil {
			return fmt.Errorf("no logging config for bucket: %s", bucket)
		}
		cfg = &BucketLoggingConfig{}
		return json.Unmarshal(data, cfg)
	})
	return cfg, err
}

func (s *Store) DeleteLoggingConfig(bucket string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(loggingConfigBucket)
		return b.Delete([]byte(bucket))
	})
}

// ChangeLogRawEntry is a raw key-value pair from the change log bucket.
type ChangeLogRawEntry struct {
	Key   []byte
	Value []byte
}

// AppendChangeLog appends a new entry to the change log with an auto-incrementing sequence key.
func (s *Store) AppendChangeLog(data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(changeLogBucket)
		seq, err := b.NextSequence()
		if err != nil {
			return err
		}
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, seq)
		return b.Put(key, data)
	})
}

// ReadChangeLog returns entries with sequence numbers greater than sinceSeq, up to limit.
func (s *Store) ReadChangeLog(sinceSeq uint64, limit int) ([]ChangeLogRawEntry, error) {
	var entries []ChangeLogRawEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(changeLogBucket)
		c := b.Cursor()

		startKey := make([]byte, 8)
		binary.BigEndian.PutUint64(startKey, sinceSeq+1)

		count := 0
		for k, v := c.Seek(startKey); k != nil && count < limit; k, v = c.Next() {
			kCopy := make([]byte, len(k))
			copy(kCopy, k)
			vCopy := make([]byte, len(v))
			copy(vCopy, v)
			entries = append(entries, ChangeLogRawEntry{Key: kCopy, Value: vCopy})
			count++
		}
		return nil
	})
	return entries, err
}

// TrimChangeLog removes all entries with sequence numbers less than beforeSeq.
func (s *Store) TrimChangeLog(beforeSeq uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(changeLogBucket)
		c := b.Cursor()

		maxKey := make([]byte, 8)
		binary.BigEndian.PutUint64(maxKey, beforeSeq)

		var keysToDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if binary.BigEndian.Uint64(k) < beforeSeq {
				kCopy := make([]byte, len(k))
				copy(kCopy, k)
				keysToDelete = append(keysToDelete, kCopy)
			} else {
				break
			}
		}

		for _, k := range keysToDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// ChangeLogSeq returns the current highest sequence number in the change log.
func (s *Store) ChangeLogSeq() (uint64, error) {
	var seq uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(changeLogBucket)
		seq = b.Sequence()
		return nil
	})
	return seq, err
}
