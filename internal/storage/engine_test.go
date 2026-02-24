package storage

import (
	"bytes"
	"io"
	"testing"
)

func newTestEngine(t *testing.T) *FileSystem {
	t.Helper()
	dir := t.TempDir()
	fs, err := NewFileSystem(dir)
	if err != nil {
		t.Fatalf("NewFileSystem: %v", err)
	}
	return fs
}

func TestFileSystem_PutGetDelete(t *testing.T) {
	fs := newTestEngine(t)

	fs.CreateBucketDir("testbucket")

	data := []byte("hello world")
	size, etag, err := fs.PutObject("testbucket", "file.txt", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), size)
	}
	if etag == "" {
		t.Error("expected non-empty etag")
	}

	reader, rsize, err := fs.GetObject("testbucket", "file.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer reader.Close()
	if rsize != int64(len(data)) {
		t.Errorf("GetObject size: expected %d, got %d", len(data), rsize)
	}
	got, _ := io.ReadAll(reader)
	if !bytes.Equal(got, data) {
		t.Errorf("expected %q, got %q", data, got)
	}

	if err := fs.DeleteObject("testbucket", "file.txt"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, _, err := fs.GetObject("testbucket", "file.txt"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestFileSystem_ListObjects(t *testing.T) {
	fs := newTestEngine(t)
	fs.CreateBucketDir("listbucket")

	for _, key := range []string{"a.txt", "b.txt", "dir/c.txt"} {
		fs.PutObject("listbucket", key, bytes.NewReader([]byte("x")), 1)
	}

	objects, _, err := fs.ListObjects("listbucket", "", "", 100)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objects) != 3 {
		t.Errorf("expected 3 objects, got %d", len(objects))
	}

	// With prefix filter
	objects, _, err = fs.ListObjects("listbucket", "dir/", "", 100)
	if err != nil {
		t.Fatalf("ListObjects with prefix: %v", err)
	}
	if len(objects) != 1 || objects[0].Key != "dir/c.txt" {
		t.Errorf("expected [dir/c.txt], got %v", objects)
	}
}

func TestFileSystem_BucketSize(t *testing.T) {
	fs := newTestEngine(t)
	fs.CreateBucketDir("sizebucket")

	fs.PutObject("sizebucket", "a.txt", bytes.NewReader([]byte("hello")), 5)
	fs.PutObject("sizebucket", "b.txt", bytes.NewReader([]byte("world!")), 6)

	size, count, err := fs.BucketSize("sizebucket")
	if err != nil {
		t.Fatalf("BucketSize: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 objects, got %d", count)
	}
	if size != 11 {
		t.Errorf("expected 11 bytes, got %d", size)
	}
}

func TestFileSystem_ObjectExists(t *testing.T) {
	fs := newTestEngine(t)
	fs.CreateBucketDir("existbucket")

	if fs.ObjectExists("existbucket", "nope.txt") {
		t.Error("expected false for non-existent object")
	}

	fs.PutObject("existbucket", "yes.txt", bytes.NewReader([]byte("x")), 1)
	if !fs.ObjectExists("existbucket", "yes.txt") {
		t.Error("expected true for existing object")
	}
}

func TestFileSystem_GetObject_NotFound(t *testing.T) {
	fs := newTestEngine(t)
	fs.CreateBucketDir("emptyb")

	_, _, err := fs.GetObject("emptyb", "missing.txt")
	if err == nil {
		t.Error("expected error for missing object")
	}
}
