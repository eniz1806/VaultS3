package storage

import "io"

// Engine defines the interface for object storage backends.
type Engine interface {
	// Bucket operations
	CreateBucketDir(bucket string) error
	DeleteBucketDir(bucket string) error

	// Object operations
	PutObject(bucket, key string, reader io.Reader, size int64) (int64, error)
	GetObject(bucket, key string) (io.ReadCloser, int64, error)
	DeleteObject(bucket, key string) error
	ObjectExists(bucket, key string) bool
	ObjectSize(bucket, key string) (int64, error)

	// List operations
	ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error)

	// Stats
	BucketSize(bucket string) (int64, int64, error) // totalSize, objectCount, error
}

// ObjectInfo represents metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified int64 // unix timestamp
	ETag         string
}
