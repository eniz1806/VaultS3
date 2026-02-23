# VaultS3

Lightweight, S3-compatible object storage server with built-in web dashboard. Single binary, low memory, zero external dependencies.

## Features

- **S3-compatible API** — Works with any S3 client (AWS CLI, mc, boto3, minio-js)
- **Single binary** — One file, no runtime dependencies, no Docker required
- **Low memory** — Targets <80MB RAM (vs MinIO's 300-500MB)
- **BoltDB metadata** — Embedded key-value store, no external database needed
- **S3 Signature V4** — Standard AWS authentication
- **AES-256-GCM encryption at rest** — Optional server-side encryption with SSE headers
- **Bucket policies** — Public-read, private, custom S3-compatible JSON policies
- **Quota management** — Per-bucket size and object count limits
- **Multipart upload** — Full lifecycle (Create, UploadPart, Complete, Abort)
- **Multiple access keys** — Dynamic key management via BoltDB
- **Object tagging** — Up to 10 tags per object
- **Range requests** — Partial content downloads (206 responses)
- **Copy object** — Same-bucket and cross-bucket copies
- **Batch delete** — Multi-object delete with XML body
- **Virtual-hosted style URLs** — `bucket.domain/key` in addition to path-style
- **Prometheus metrics** — `/metrics` endpoint with storage, request, and runtime stats
- **Presigned URLs** — Pre-authenticated URL generation
- **Web dashboard** — Built-in React UI at `/dashboard/` with JWT auth, file browser, access key management, activity log, storage stats, dark/light theme, responsive layout
- **Health checks** — `/health` (liveness) and `/ready` (readiness) endpoints for load balancers and Kubernetes
- **Graceful shutdown** — Drains in-flight requests on SIGTERM/SIGINT with configurable timeout
- **TLS support** — Optional HTTPS with configurable cert/key paths
- **Object versioning** — Per-bucket versioning with version IDs, delete markers, version-specific GET/DELETE/HEAD
- **Object locking (WORM)** — Legal hold and retention (GOVERNANCE/COMPLIANCE) to prevent deletion
- **Lifecycle rules** — Per-bucket object expiration (auto-delete after N days) with background worker
- **Gzip compression** — Transparent compress-on-write, decompress-on-read with standard gzip
- **Access logging** — Structured JSON lines log file of all S3 operations
- **Static website hosting** — Serve index/error documents from buckets, no auth required
- **Docker image** — Multi-stage Dockerfile with built-in health check
- **YAML config** — Simple configuration, sensible defaults

## Supported S3 Operations

| Operation | Endpoint | Status |
|-----------|----------|--------|
| List Buckets | `GET /` | Done |
| Create Bucket | `PUT /{bucket}` | Done |
| Delete Bucket | `DELETE /{bucket}` | Done |
| Head Bucket | `HEAD /{bucket}` | Done |
| Put Object | `PUT /{bucket}/{key}` | Done |
| Get Object | `GET /{bucket}/{key}` | Done |
| Delete Object | `DELETE /{bucket}/{key}` | Done |
| Head Object | `HEAD /{bucket}/{key}` | Done |
| List Objects V2 | `GET /{bucket}?prefix=&max-keys=` | Done |
| Copy Object | `PUT /{bucket}/{key}` + `x-amz-copy-source` | Done |
| Batch Delete | `POST /{bucket}?delete` | Done |
| Multipart Upload | `POST/PUT/DELETE /{bucket}/{key}?uploads&uploadId` | Done |
| Object Tagging | `PUT/GET/DELETE /{bucket}/{key}?tagging` | Done |
| Bucket Policy | `PUT/GET/DELETE /{bucket}?policy` | Done |
| Bucket Quota | `PUT/GET /{bucket}?quota` | Done |
| Bucket Versioning | `PUT/GET /{bucket}?versioning` | Done |
| List Object Versions | `GET /{bucket}?versions` | Done |
| Object Locking (Legal Hold) | `PUT/GET /{bucket}/{key}?legal-hold` | Done |
| Object Locking (Retention) | `PUT/GET /{bucket}/{key}?retention` | Done |
| Lifecycle Rules | `PUT/GET/DELETE /{bucket}?lifecycle` | Done |
| Website Hosting | `PUT/GET/DELETE /{bucket}?website` | Done |
| Presigned URLs | — | Done |
| Metrics | `GET /metrics` | Done |

## Quick Start

### Build

```bash
make build
```

### Run

```bash
./vaults3
```

Server starts on `http://localhost:9000` by default.

### Configure

Edit `configs/vaults3.yaml`:

```yaml
server:
  address: "0.0.0.0"
  port: 9000
  domain: ""  # set to enable virtual-hosted URLs (e.g. "s3.example.com")
  shutdown_timeout_secs: 30
  tls:
    enabled: false
    cert_file: ""
    key_file: ""

storage:
  data_dir: "./data"
  metadata_dir: "./metadata"

auth:
  admin_access_key: "vaults3-admin"
  admin_secret_key: "vaults3-secret-change-me"

encryption:
  enabled: false
  key: ""  # 64-character hex string (32 bytes) when enabled

compression:
  enabled: false

logging:
  enabled: false
  file_path: "./access.log"

lifecycle:
  scan_interval_secs: 3600
```

### Encryption at Rest

Enable AES-256-GCM encryption by setting `encryption.enabled: true` and providing a 32-byte hex key:

```bash
# Generate a key
openssl rand -hex 32
```

When enabled, all objects are encrypted on disk with a random nonce per object. SSE headers (`x-amz-server-side-encryption: AES256`) are included in responses.

### Virtual-Hosted Style URLs

Set `server.domain` to enable virtual-hosted style access:

```yaml
server:
  domain: "s3.example.com"
```

This enables `bucket-name.s3.example.com/key` in addition to the default `s3.example.com/bucket-name/key` path-style.

### Prometheus Metrics

Access metrics at `GET /metrics`:

```bash
curl http://localhost:9000/metrics
```

Exposes: request counts by method, bytes in/out, per-bucket storage size and object counts, quota usage, Go runtime stats (goroutines, memory, GC).

### Web Dashboard

The built-in dashboard is available at `http://localhost:9000/dashboard/`. Login with your admin credentials. Features:

- Bucket browser — list, create, delete buckets
- Bucket detail — view/edit policies and quotas
- File browser — list, upload (drag & drop), download, delete objects with folder navigation
- Access key management — create, list, revoke S3 API keys
- Activity log — real-time S3 operation feed with auto-refresh
- Storage stats — total storage, per-bucket breakdown, runtime metrics
- Dark/light theme — toggle with system preference detection
- Responsive layout — mobile-friendly with collapsible sidebar
- JWT-based authentication (24h tokens)

The dashboard is embedded into the binary — no separate web server needed.

### Health Checks

```bash
curl http://localhost:9000/health   # liveness: {"status":"ok","uptime":"5h23m"}
curl http://localhost:9000/ready    # readiness: checks BoltDB, returns 503 if unhealthy
```

### TLS

Enable HTTPS by providing cert and key files:

```yaml
server:
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

### Docker

```bash
docker build -t vaults3 .
docker run -p 9000:9000 -v ./data:/data -v ./metadata:/metadata vaults3
```

### Object Versioning

Enable versioning on a bucket to keep multiple versions of objects:

```python
import boto3

s3 = boto3.client('s3', endpoint_url='http://localhost:9000',
    aws_access_key_id='vaults3-admin',
    aws_secret_access_key='vaults3-secret-change-me')

# Enable versioning
s3.put_bucket_versioning(Bucket='my-bucket',
    VersioningConfiguration={'Status': 'Enabled'})

# Upload creates a new version each time
s3.put_object(Bucket='my-bucket', Key='file.txt', Body=b'v1')
s3.put_object(Bucket='my-bucket', Key='file.txt', Body=b'v2')

# Get specific version
s3.get_object(Bucket='my-bucket', Key='file.txt', VersionId='...')

# Delete creates a delete marker (versions preserved)
s3.delete_object(Bucket='my-bucket', Key='file.txt')

# Permanently delete a specific version
s3.delete_object(Bucket='my-bucket', Key='file.txt', VersionId='...')
```

### Object Locking (WORM)

Protect objects from deletion with legal holds or retention policies:

```python
# Legal hold — prevents deletion regardless
s3.put_object_legal_hold(Bucket='my-bucket', Key='file.txt', VersionId='...',
    LegalHold={'Status': 'ON'})

# Retention — prevents deletion until date
s3.put_object_retention(Bucket='my-bucket', Key='file.txt', VersionId='...',
    Retention={'Mode': 'COMPLIANCE', 'RetainUntilDate': '2030-01-01T00:00:00Z'})
```

### Lifecycle Rules

Auto-delete objects after a specified number of days:

```python
s3.put_bucket_lifecycle_configuration(Bucket='my-bucket',
    LifecycleConfiguration={
        'Rules': [{
            'ID': 'expire-logs',
            'Expiration': {'Days': 30},
            'Filter': {'Prefix': 'logs/'},
            'Status': 'Enabled',
        }]
    })
```

The background worker scans objects periodically (configurable interval, default 1 hour) and deletes expired objects. Locked objects (legal hold or retention) are skipped.

### Compression

Enable gzip compression to reduce storage usage:

```yaml
compression:
  enabled: true
```

All objects are transparently compressed on write and decompressed on read. Works with encryption (data is compressed then encrypted on disk).

### Access Logging

Enable structured JSON access logs:

```yaml
logging:
  enabled: true
  file_path: "./access.log"
```

Each S3 operation is logged as a JSON line with timestamp, method, bucket, key, status code, bytes, and client IP.

### Static Website Hosting

Serve static websites directly from buckets:

```python
s3.put_bucket_website(Bucket='my-site',
    WebsiteConfiguration={
        'IndexDocument': {'Suffix': 'index.html'},
        'ErrorDocument': {'Key': 'error.html'}
    })
```

Website-enabled buckets serve `index.html` for directory paths and a custom error page for missing objects. No authentication required for GET/HEAD requests.

### Test with mc (MinIO Client)

```bash
mc alias set vaults3 http://localhost:9000 vaults3-admin vaults3-secret-change-me
mc mb vaults3/my-bucket
mc cp file.txt vaults3/my-bucket/
mc ls vaults3/my-bucket/
mc cat vaults3/my-bucket/file.txt
```

## Project Structure

```
VaultS3/
├── cmd/vaults3/main.go        — Entry point
├── internal/
│   ├── config/                — YAML config loader
│   ├── server/                — HTTP server and routing
│   ├── s3/                    — S3 API handlers (auth, buckets, objects, multipart)
│   ├── storage/               — Storage engine interface + filesystem + encryption
│   ├── metadata/              — BoltDB metadata store
│   ├── metrics/               — Prometheus-compatible metrics collector
│   ├── api/                   — Dashboard REST API (JWT auth)
│   └── dashboard/             — Embedded React SPA
├── web/                       — React dashboard source (Vite + Tailwind)
├── configs/vaults3.yaml       — Default configuration
├── Makefile                   — Build commands
├── Dockerfile                 — Multi-stage Docker build
└── README.md
```

## Tech Stack

- **Go** — net/http (no frameworks)
- **React 19** — Dashboard UI (embedded via `//go:embed`)
- **Tailwind CSS** — Dashboard styling
- **BoltDB** — Embedded key-value store for metadata
- **Local filesystem** — Object storage backend
- **AES-256-GCM** — Server-side encryption (optional)

## Requirements

- Go 1.21+ (build)
- Node.js 18+ (dashboard build only)
- No runtime dependencies

## Roadmap

- [x] Core S3 CRUD operations
- [x] S3 Signature V4 authentication
- [x] Presigned URLs
- [x] Content-Type detection and storage
- [x] Range requests (partial GET)
- [x] Copy object (same/cross-bucket)
- [x] Batch delete
- [x] Multipart upload (full lifecycle)
- [x] Multiple access keys
- [x] Object tagging
- [x] AES-256-GCM encryption at rest
- [x] Bucket policies (public-read, custom)
- [x] Quota management (per-bucket)
- [x] Virtual-hosted style URLs
- [x] Prometheus-compatible metrics
- [x] Web dashboard with built-in UI (login, bucket browser, file management, access keys, activity log, stats, dark/light theme, responsive)
- [x] Health check endpoints (/health, /ready)
- [x] Graceful shutdown (SIGTERM/SIGINT with configurable timeout)
- [x] TLS support (HTTPS with cert/key)
- [x] Docker image (multi-stage build with health check)
- [x] Object versioning (per-bucket, version IDs, delete markers, version-specific operations)
- [x] Object locking / WORM (legal hold, retention with GOVERNANCE/COMPLIANCE modes)
- [x] Lifecycle rules (per-bucket expiration with background worker)
- [x] Gzip compression (transparent compress/decompress)
- [x] Access logging (structured JSON lines)
- [x] Static website hosting (index/error documents, no-auth serving)
- [ ] Replication
