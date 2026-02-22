package storage

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileSystem implements Engine using the local filesystem.
type FileSystem struct {
	dataDir string
}

func NewFileSystem(dataDir string) (*FileSystem, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &FileSystem{dataDir: dataDir}, nil
}

func (fs *FileSystem) bucketPath(bucket string) string {
	return filepath.Join(fs.dataDir, bucket)
}

func (fs *FileSystem) objectPath(bucket, key string) string {
	return filepath.Join(fs.dataDir, bucket, key)
}

func (fs *FileSystem) CreateBucketDir(bucket string) error {
	return os.MkdirAll(fs.bucketPath(bucket), 0755)
}

func (fs *FileSystem) DeleteBucketDir(bucket string) error {
	return os.RemoveAll(fs.bucketPath(bucket))
}

func (fs *FileSystem) PutObject(bucket, key string, reader io.Reader, size int64) (int64, error) {
	objPath := fs.objectPath(bucket, key)

	// Create parent directories for nested keys (e.g., "folder/file.txt")
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return 0, fmt.Errorf("create object dir: %w", err)
	}

	f, err := os.Create(objPath)
	if err != nil {
		return 0, fmt.Errorf("create object file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, reader)
	if err != nil {
		os.Remove(objPath)
		return 0, fmt.Errorf("write object: %w", err)
	}

	return written, nil
}

func (fs *FileSystem) GetObject(bucket, key string) (io.ReadCloser, int64, error) {
	objPath := fs.objectPath(bucket, key)

	info, err := os.Stat(objPath)
	if err != nil {
		return nil, 0, fmt.Errorf("stat object: %w", err)
	}
	if info.IsDir() {
		return nil, 0, fmt.Errorf("object is a directory")
	}

	f, err := os.Open(objPath)
	if err != nil {
		return nil, 0, fmt.Errorf("open object: %w", err)
	}

	return f, info.Size(), nil
}

func (fs *FileSystem) DeleteObject(bucket, key string) error {
	objPath := fs.objectPath(bucket, key)
	err := os.Remove(objPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object: %w", err)
	}

	// Clean up empty parent directories
	dir := filepath.Dir(objPath)
	bucketDir := fs.bucketPath(bucket)
	for dir != bucketDir {
		entries, _ := os.ReadDir(dir)
		if len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}

	return nil
}

func (fs *FileSystem) ObjectExists(bucket, key string) bool {
	info, err := os.Stat(fs.objectPath(bucket, key))
	return err == nil && !info.IsDir()
}

func (fs *FileSystem) ObjectSize(bucket, key string) (int64, error) {
	info, err := os.Stat(fs.objectPath(bucket, key))
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (fs *FileSystem) ListObjects(bucket, prefix, startAfter string, maxKeys int) ([]ObjectInfo, bool, error) {
	bucketDir := fs.bucketPath(bucket)

	var objects []ObjectInfo
	err := filepath.Walk(bucketDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}

		// Get relative key
		rel, err := filepath.Rel(bucketDir, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes for S3 compatibility
		key := strings.ReplaceAll(rel, string(filepath.Separator), "/")

		// Apply prefix filter
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		// Apply startAfter filter
		if startAfter != "" && key <= startAfter {
			return nil
		}

		objects = append(objects, ObjectInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime().Unix(),
			ETag:         computeETag(path),
		})

		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("walk bucket: %w", err)
	}

	// Sort by key
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})

	// Apply maxKeys limit
	truncated := false
	if maxKeys > 0 && len(objects) > maxKeys {
		objects = objects[:maxKeys]
		truncated = true
	}

	return objects, truncated, nil
}

func (fs *FileSystem) BucketSize(bucket string) (int64, int64, error) {
	var totalSize int64
	var count int64

	bucketDir := fs.bucketPath(bucket)
	err := filepath.Walk(bucketDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			count++
		}
		return nil
	})

	return totalSize, count, err
}

func computeETag(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := md5.New()
	io.Copy(h, f)
	return fmt.Sprintf("\"%x\"", h.Sum(nil))
}
