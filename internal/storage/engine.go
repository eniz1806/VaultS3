package storage

import "io"

// ReadSeekCloser combines io.ReadSeeker and io.Closer.
type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

// Engine defines the interface for object storage backends.
type Engine interface {
	// Bucket operations
	CreateBucketDir(bucket string) error
	DeleteBucketDir(bucket string) error

	// Object operations
	PutObject(bucket, key string, reader io.Reader, size int64) (written int64, etag string, err error)
	GetObject(bucket, key string) (ReadSeekCloser, int64, error)
	DeleteObject(bucket, key string) error
	ObjectExists(bucket, key string) bool
	ObjectSize(bucket, key string) (int64, error)

	// List operations
	ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error)

	// Stats
	BucketSize(bucket string) (int64, int64, error) // totalSize, objectCount, error

	// Paths (for multipart upload temp storage)
	DataDir() string
	ObjectPath(bucket, key string) string
}

// ObjectInfo represents metadata about a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified int64 // unix timestamp
	ETag         string
}
