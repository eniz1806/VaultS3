# VaultS3

Lightweight, S3-compatible object storage server. Single binary, low memory, zero external dependencies.

## Features

- **S3-compatible API** — Works with any S3 client (AWS CLI, mc, boto3, minio-js)
- **Single binary** — One file, no runtime dependencies, no Docker required
- **Low memory** — Targets <80MB RAM (vs MinIO's 300-500MB)
- **BoltDB metadata** — Embedded key-value store, no external database needed
- **S3 Signature V4** — Standard AWS authentication
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
| Presigned URLs | — | Done |

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

storage:
  data_dir: "./data"
  metadata_dir: "./metadata"

auth:
  admin_access_key: "vaults3-admin"
  admin_secret_key: "vaults3-secret-change-me"
```

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
│   ├── s3/                    — S3 API handlers (auth, buckets, objects)
│   ├── storage/               — Storage engine interface + filesystem implementation
│   └── metadata/              — BoltDB metadata store
├── configs/vaults3.yaml       — Default configuration
├── Makefile                   — Build commands
└── README.md
```

## Tech Stack

- **Go** — net/http (no frameworks)
- **BoltDB** — Embedded key-value store for metadata
- **Local filesystem** — Object storage backend

## Requirements

- Go 1.21+ (build only)
- No runtime dependencies

## Roadmap

- [ ] Multipart upload
- [ ] Encryption at rest (AES-256)
- [ ] Bucket policies and ACL
- [ ] Range requests
- [ ] Object tagging
- [ ] Web dashboard with built-in UI
- [ ] Object versioning
- [ ] Lifecycle rules
- [ ] Replication
- [ ] Docker image
