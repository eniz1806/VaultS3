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
- **Web dashboard** — Built-in React UI at `/dashboard/` with JWT auth, bucket management
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

storage:
  data_dir: "./data"
  metadata_dir: "./metadata"

auth:
  admin_access_key: "vaults3-admin"
  admin_secret_key: "vaults3-secret-change-me"

encryption:
  enabled: false
  key: ""  # 64-character hex string (32 bytes) when enabled
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
- JWT-based authentication (24h tokens)

The dashboard is embedded into the binary — no separate web server needed.

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
- [x] Web dashboard with built-in UI (login, bucket browser, policy/quota editors)
- [ ] Object versioning
- [ ] Object locking (WORM)
- [ ] Lifecycle rules
- [ ] TLS support
- [ ] Docker image
- [ ] Replication
