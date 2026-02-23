//go:build !windows

package fuse

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"syscall"
	"time"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

type MountConfig struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	Bucket     string
	Region     string
}

type VaultFS struct {
	fs.Inode
	cfg    MountConfig
	client *http.Client
}

func Mount(mountpoint string, cfg MountConfig) (*gofuse.Server, error) {
	root := &VaultFS{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	opts := &fs.Options{
		MountOptions: gofuse.MountOptions{
			FsName: "vaults3",
			Name:   "vaults3",
		},
	}

	server, err := fs.Mount(mountpoint, root, opts)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}

	return server, nil
}

// Readdir lists objects as files/directories in the bucket.
func (v *VaultFS) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	prefix := v.getPrefix()

	objects, err := v.listObjects(prefix, "/")
	if err != nil {
		return nil, syscall.EIO
	}

	var entries []gofuse.DirEntry
	seen := make(map[string]bool)

	for _, obj := range objects {
		name := strings.TrimPrefix(obj.key, prefix)
		if name == "" {
			continue
		}

		// Check if it's a "directory" (common prefix)
		if obj.isPrefix {
			dirName := strings.TrimSuffix(name, "/")
			if dirName != "" && !seen[dirName] {
				entries = append(entries, gofuse.DirEntry{
					Name: dirName,
					Mode: syscall.S_IFDIR,
				})
				seen[dirName] = true
			}
		} else {
			if !seen[name] {
				entries = append(entries, gofuse.DirEntry{
					Name: name,
					Mode: syscall.S_IFREG,
				})
				seen[name] = true
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return fs.NewListDirStream(entries), 0
}

// Lookup finds a child entry.
func (v *VaultFS) Lookup(ctx context.Context, name string, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	prefix := v.getPrefix()
	fullKey := prefix + name

	// Check if it's a directory (has objects with this prefix)
	objects, err := v.listObjects(fullKey+"/", "/")
	if err == nil && len(objects) > 0 {
		child := &VaultFS{cfg: v.cfg, client: v.client}
		out.Mode = syscall.S_IFDIR | 0755
		return v.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
	}

	// Check if it's a file
	size, err := v.headObject(fullKey)
	if err != nil {
		return nil, syscall.ENOENT
	}

	child := &VaultFile{cfg: v.cfg, client: v.client, key: fullKey, size: size}
	out.Mode = syscall.S_IFREG | 0644
	out.Size = uint64(size)
	return v.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFREG}), 0
}

func (v *VaultFS) Getattr(ctx context.Context, fh fs.FileHandle, out *gofuse.AttrOut) syscall.Errno {
	out.Mode = syscall.S_IFDIR | 0755
	return 0
}

func (v *VaultFS) getPrefix() string {
	path := v.Path(nil)
	if path == "" {
		return ""
	}
	return path + "/"
}

type objectEntry struct {
	key      string
	size     int64
	isPrefix bool
}

func (v *VaultFS) listObjects(prefix, delimiter string) ([]objectEntry, error) {
	params := url.Values{}
	params.Set("list-type", "2")
	if prefix != "" {
		params.Set("prefix", prefix)
	}
	if delimiter != "" {
		params.Set("delimiter", delimiter)
	}
	params.Set("max-keys", "1000")

	reqURL := fmt.Sprintf("%s/%s?%s", v.cfg.Endpoint, v.cfg.Bucket, params.Encode())
	req, _ := http.NewRequest("GET", reqURL, nil)
	signRequest(req, v.cfg.AccessKey, v.cfg.SecretKey, v.cfg.Region)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	var entries []objectEntry

	// Parse XML response — simple extraction
	for _, match := range extractXMLValues(content, "Key") {
		entries = append(entries, objectEntry{key: match})
	}
	for _, match := range extractXMLValues(content, "Prefix") {
		if match != prefix {
			entries = append(entries, objectEntry{key: match, isPrefix: true})
		}
	}

	return entries, nil
}

func (v *VaultFS) headObject(key string) (int64, error) {
	reqURL := fmt.Sprintf("%s/%s/%s", v.cfg.Endpoint, v.cfg.Bucket, key)
	req, _ := http.NewRequest("HEAD", reqURL, nil)
	signRequest(req, v.cfg.AccessKey, v.cfg.SecretKey, v.cfg.Region)

	resp, err := v.client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("not found")
	}

	return resp.ContentLength, nil
}

// VaultFile represents a file in the FUSE filesystem.
type VaultFile struct {
	fs.Inode
	cfg    MountConfig
	client *http.Client
	key    string
	size   int64
}

func (f *VaultFile) Getattr(ctx context.Context, fh fs.FileHandle, out *gofuse.AttrOut) syscall.Errno {
	out.Mode = syscall.S_IFREG | 0644
	out.Size = uint64(f.size)
	return 0
}

func (f *VaultFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &VaultFileHandle{cfg: f.cfg, client: f.client, key: f.key, size: f.size}, 0, 0
}

func (f *VaultFile) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	h, ok := fh.(*VaultFileHandle)
	if !ok {
		h = &VaultFileHandle{cfg: f.cfg, client: f.client, key: f.key, size: f.size}
	}
	return h.Read(ctx, dest, off)
}

// VaultFileHandle manages reading from a remote object.
type VaultFileHandle struct {
	cfg    MountConfig
	client *http.Client
	key    string
	size   int64
}

func (h *VaultFileHandle) Read(ctx context.Context, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	if off >= h.size {
		return gofuse.ReadResultData(nil), 0
	}

	end := off + int64(len(dest)) - 1
	if end >= h.size {
		end = h.size - 1
	}

	reqURL := fmt.Sprintf("%s/%s/%s", h.cfg.Endpoint, h.cfg.Bucket, h.key)
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))
	signRequest(req, h.cfg.AccessKey, h.cfg.SecretKey, h.cfg.Region)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, syscall.EIO
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, syscall.EIO
	}

	return gofuse.ReadResultData(data), 0
}

// Write support — creates/updates objects in VaultS3.
func (v *VaultFS) Create(ctx context.Context, name string, flags uint32, mode uint32, out *gofuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	prefix := v.getPrefix()
	fullKey := prefix + name

	wh := &VaultWriteHandle{
		cfg:    v.cfg,
		client: v.client,
		key:    fullKey,
	}

	child := &VaultFile{cfg: v.cfg, client: v.client, key: fullKey, size: 0}
	out.Mode = syscall.S_IFREG | 0644
	inode := v.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFREG})

	return inode, wh, 0, 0
}

type VaultWriteHandle struct {
	cfg    MountConfig
	client *http.Client
	key    string
	buf    bytes.Buffer
}

func (wh *VaultWriteHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	n, _ := wh.buf.Write(data)
	return uint32(n), 0
}

func (wh *VaultWriteHandle) Flush(ctx context.Context) syscall.Errno {
	if wh.buf.Len() == 0 {
		return 0
	}

	reqURL := fmt.Sprintf("%s/%s/%s", wh.cfg.Endpoint, wh.cfg.Bucket, wh.key)
	req, _ := http.NewRequest("PUT", reqURL, bytes.NewReader(wh.buf.Bytes()))
	req.ContentLength = int64(wh.buf.Len())
	signRequest(req, wh.cfg.AccessKey, wh.cfg.SecretKey, wh.cfg.Region)

	resp, err := wh.client.Do(req)
	if err != nil {
		return syscall.EIO
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return syscall.EIO
	}
	return 0
}

func (wh *VaultWriteHandle) Release(ctx context.Context) syscall.Errno {
	return 0
}

// Unlink deletes a file.
func (v *VaultFS) Unlink(ctx context.Context, name string) syscall.Errno {
	prefix := v.getPrefix()
	fullKey := prefix + name

	reqURL := fmt.Sprintf("%s/%s/%s", v.cfg.Endpoint, v.cfg.Bucket, fullKey)
	req, _ := http.NewRequest("DELETE", reqURL, nil)
	signRequest(req, v.cfg.AccessKey, v.cfg.SecretKey, v.cfg.Region)

	resp, err := v.client.Do(req)
	if err != nil {
		return syscall.EIO
	}
	resp.Body.Close()

	return 0
}

// signRequest adds SigV4 headers to a request (simplified).
func signRequest(req *http.Request, accessKey, secretKey, region string) {
	if region == "" {
		region = "us-east-1"
	}
	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzdate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzdate)
	req.Header.Set("Host", req.URL.Host)

	// Canonical request
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.Query().Encode()

	signedHeaders := "host;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", req.URL.Host, amzdate)

	payloadHash := "UNSIGNED-PAYLOAD"
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalRequest := strings.Join([]string{
		req.Method, canonicalURI, canonicalQuery,
		canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	scope := fmt.Sprintf("%s/%s/s3/aws4_request", datestamp, region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzdate, scope, sha256hex([]byte(canonicalRequest)))

	sigKey := deriveKey(secretKey, datestamp, region, "s3")
	signature := hex.EncodeToString(hmacSHA256(sigKey, []byte(stringToSign)))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func deriveKey(secret, datestamp, region, service string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), []byte(datestamp))
	k = hmacSHA256(k, []byte(region))
	k = hmacSHA256(k, []byte(service))
	k = hmacSHA256(k, []byte("aws4_request"))
	return k
}

func extractXMLValues(xml, tag string) []string {
	var values []string
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	for {
		start := strings.Index(xml, openTag)
		if start == -1 {
			break
		}
		start += len(openTag)
		end := strings.Index(xml[start:], closeTag)
		if end == -1 {
			break
		}
		values = append(values, xml[start:start+end])
		xml = xml[start+end+len(closeTag):]
	}
	return values
}
