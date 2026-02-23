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
- **IAM users, groups & policies** — Fine-grained access control with S3-compatible policy evaluation, default deny, wildcard matching
- **CORS per bucket** — S3-compatible CORS configuration with OPTIONS preflight support
- **STS temporary credentials** — Short-lived access keys with configurable TTL, auto-cleanup of expired keys
- **Audit trail** — Persistent audit log with filtering by user, bucket, time range; auto-pruning via lifecycle worker
- **IP allowlist/blocklist** — Global and per-user CIDR-based IP restrictions with IPv4/IPv6 support
- **S3 event notifications** — Per-bucket webhook notifications on object mutations with event type and key prefix/suffix filtering
- **Async replication** — One-way async replication to peer VaultS3 instances with BoltDB-backed queue, retry with exponential backoff, and loop prevention
- **CLI tool** — Standalone `vaults3-cli` binary for bucket, object, user, and replication management without AWS CLI
- **Presigned upload restrictions** — Enforce max file size, content type whitelist, and key prefix on presigned PUT URLs
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
| Bucket CORS | `PUT/GET/DELETE /{bucket}?cors` | Done |
| Presigned URLs | — | Done |
| Metrics | `GET /metrics` | Done |
| IAM (Users/Groups/Policies) | Dashboard API `/api/v1/iam/*` | Done |
| STS Temporary Credentials | `POST /api/v1/sts/session-token` | Done |
| Audit Trail | `GET /api/v1/audit` | Done |
| IP Restrictions | `PUT /api/v1/iam/users/{name}/ip-restrictions` | Done |
| Bucket Notifications | `PUT/GET/DELETE /{bucket}?notification` | Done |
| Notification Configs | `GET /api/v1/notifications` | Done |
| Replication Status | `GET /api/v1/replication/status` | Done |
| Replication Queue | `GET /api/v1/replication/queue` | Done |
| Presigned URL Generation | `POST /api/v1/presign` | Done |

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

security:
  ip_allowlist: []     # global CIDR allow list, empty = allow all
  ip_blocklist: []     # global CIDR deny list
  audit_retention_days: 90
  sts_max_duration_secs: 43200  # max STS token duration (12 hours)
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

### IAM (Users, Groups & Policies)

Fine-grained access control with S3-compatible IAM policies:

```python
import requests, json

API = "http://localhost:9000/api/v1"
headers = {"Authorization": "Bearer <jwt-token>", "Content-Type": "application/json"}

# Create an IAM user
requests.post(f"{API}/iam/users", headers=headers, json={"name": "alice"})

# Attach a built-in policy (ReadOnlyAccess, ReadWriteAccess, FullAccess)
requests.post(f"{API}/iam/users/alice/policies", headers=headers,
    json={"policyName": "ReadOnlyAccess"})

# Create an access key for the user
resp = requests.post(f"{API}/keys", headers=headers, json={"userId": "alice"})
key = resp.json()  # {"accessKey": "...", "secretKey": "..."}

# Create groups and attach policies
requests.post(f"{API}/iam/groups", headers=headers, json={"name": "developers"})
requests.post(f"{API}/iam/groups/developers/policies", headers=headers,
    json={"policyName": "ReadWriteAccess"})

# Add user to group
requests.post(f"{API}/iam/users/alice/groups", headers=headers,
    json={"groupName": "developers"})

# Create custom policies
custom_policy = json.dumps({
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Action": ["s3:GetObject"],
        "Resource": ["arn:aws:s3:::my-bucket/*"]
    }]
})
requests.post(f"{API}/iam/policies", headers=headers,
    json={"name": "MyBucketReadOnly", "document": custom_policy})
```

Policy evaluation follows AWS IAM semantics: default deny, explicit Allow required, explicit Deny always wins. Admin keys and legacy keys (without a user) retain full access.

### CORS per Bucket

Configure Cross-Origin Resource Sharing on a per-bucket basis:

```python
s3.put_bucket_cors(Bucket='my-bucket', CORSConfiguration={
    'CORSRules': [{
        'AllowedOrigins': ['https://example.com'],
        'AllowedMethods': ['GET', 'PUT'],
        'AllowedHeaders': ['*'],
        'MaxAgeSeconds': 3600,
    }]
})
```

The server responds to `OPTIONS` preflight requests with the configured CORS headers. Unknown origins are rejected with 403.

### STS Temporary Credentials

Issue short-lived access keys for temporary access:

```python
import requests, boto3

API = "http://localhost:9000/api/v1"
headers = {"Authorization": "Bearer <jwt-token>", "Content-Type": "application/json"}

# Create temporary credentials for an IAM user (max 12 hours)
resp = requests.post(f"{API}/sts/session-token", headers=headers,
    json={"durationSecs": 3600, "userId": "alice"})
creds = resp.json()  # {"accessKey", "secretKey", "sessionToken", "expiration"}

# Use temporary credentials with any S3 client
s3 = boto3.client("s3", endpoint_url="http://localhost:9000",
    aws_access_key_id=creds["accessKey"],
    aws_secret_access_key=creds["secretKey"])
```

Temporary keys inherit the IAM user's policies. Expired keys are automatically cleaned up by the lifecycle worker.

### Audit Trail

Query the persistent audit log of all S3 operations:

```python
# List recent audit entries
requests.get(f"{API}/audit?limit=50", headers=headers)

# Filter by user, time range, or bucket
requests.get(f"{API}/audit?user=alice&limit=10", headers=headers)
requests.get(f"{API}/audit?from=1700000000&to=1700100000", headers=headers)
requests.get(f"{API}/audit?bucket=my-bucket", headers=headers)
```

Each entry records: timestamp, principal, user ID, action, resource, effect (Allow/Deny), source IP, and status code. Old entries are automatically pruned based on `security.audit_retention_days`.

### IP Restrictions

Control access by IP address at global or per-user level:

```yaml
# Global restrictions in config
security:
  ip_allowlist: ["10.0.0.0/8", "192.168.0.0/16"]  # empty = allow all
  ip_blocklist: ["10.0.0.99/32"]  # deny always wins
```

```python
# Per-user IP restrictions via API
requests.put(f"{API}/iam/users/alice/ip-restrictions", headers=headers,
    json={"allowedCidrs": ["10.0.0.0/8", "::1/128"]})

# Clear restrictions (allow from anywhere)
requests.put(f"{API}/iam/users/alice/ip-restrictions", headers=headers,
    json={"allowedCidrs": []})
```

Evaluation order: global blocklist (deny wins) → global allowlist → per-user allowlist. Admin keys are exempt from IP restrictions. Supports both IPv4 and IPv6 CIDR notation.

### S3 Event Notifications

Configure webhooks on buckets to receive notifications when objects are created or deleted:

```python
from botocore.auth import SigV4Auth
from botocore.credentials import Credentials
from botocore.awsrequest import AWSRequest
import requests

# PUT notification configuration (S3-compatible XML)
notif_xml = b"""<?xml version="1.0" encoding="UTF-8"?>
<NotificationConfiguration>
  <TopicConfiguration>
    <Id>my-webhook</Id>
    <Topic>https://example.com/webhook</Topic>
    <Event>s3:ObjectCreated:*</Event>
    <Event>s3:ObjectRemoved:*</Event>
    <Filter>
      <S3Key>
        <FilterRule>
          <Name>prefix</Name>
          <Value>images/</Value>
        </FilterRule>
      </S3Key>
    </Filter>
  </TopicConfiguration>
</NotificationConfiguration>"""

# Sign and send (using botocore for SigV4)
url = "http://localhost:9000/my-bucket?notification"
creds = Credentials("vaults3-admin", "vaults3-secret-change-me")
req = AWSRequest(method="PUT", url=url, data=notif_xml,
    headers={"Content-Type": "application/xml"})
SigV4Auth(creds, "s3", "us-east-1").add_auth(req)
requests.put(url, headers=dict(req.headers), data=notif_xml)
```

Supported events: `s3:ObjectCreated:Put`, `s3:ObjectCreated:Copy`, `s3:ObjectCreated:CompleteMultipartUpload`, `s3:ObjectRemoved:Delete`. Use wildcards like `s3:ObjectCreated:*`. Webhook payloads follow the AWS S3 event notification JSON format.

Configure webhook delivery in `configs/vaults3.yaml`:

```yaml
notifications:
  max_workers: 4       # concurrent webhook delivery goroutines
  queue_size: 256      # buffered event queue size
  timeout_secs: 10     # webhook HTTP timeout
  max_retries: 3       # retry attempts for failed webhooks
```

### Async Replication

Replicate objects to a peer VaultS3 instance automatically:

```yaml
replication:
  enabled: true
  peers:
    - name: "dc2"
      url: "http://peer-vaults3:9000"
      access_key: "peer-admin"
      secret_key: "peer-secret"
  scan_interval_secs: 30   # queue processing interval
  max_retries: 5           # retry before dead-letter
  batch_size: 100          # events per scan cycle
```

Objects PUT, copied, or deleted on the primary are asynchronously replicated to all configured peers via S3 API. Buckets are auto-created on peers. Failed deliveries retry with exponential backoff (5s, 15s, 45s, 135s, 405s). The `X-VaultS3-Replication` header prevents infinite loops. Monitor via dashboard API:

```bash
curl http://localhost:9000/api/v1/replication/status   # per-peer sync stats
curl http://localhost:9000/api/v1/replication/queue     # pending queue entries
```

### CLI Tool

VaultS3 includes a standalone CLI binary (`vaults3-cli`) for managing the server:

```bash
# Set credentials via environment or flags
export VAULTS3_ENDPOINT=http://localhost:9000
export VAULTS3_ACCESS_KEY=vaults3-admin
export VAULTS3_SECRET_KEY=vaults3-secret-change-me

# Bucket operations
vaults3-cli bucket list
vaults3-cli bucket create my-bucket
vaults3-cli bucket info my-bucket
vaults3-cli bucket delete my-bucket

# Object operations
vaults3-cli object put my-bucket docs/readme.md ./README.md
vaults3-cli object ls my-bucket --prefix=docs/
vaults3-cli object get my-bucket docs/readme.md ./downloaded.md
vaults3-cli object cp my-bucket/file.txt my-bucket/copy.txt
vaults3-cli object rm my-bucket docs/readme.md
vaults3-cli object presign my-bucket file.txt --expires=3600

# IAM user operations
vaults3-cli user list
vaults3-cli user create alice --access-key=ak --secret-key=sk
vaults3-cli user attach-policy alice ReadWriteAccess
vaults3-cli user delete alice

# Replication monitoring
vaults3-cli replication status
vaults3-cli replication queue
```

Build both binaries with `make build` or just the CLI with `make cli`.

### Presigned Upload Restrictions

Generate presigned PUT URLs with server-enforced restrictions:

```python
import requests

API = "http://localhost:9000/api/v1"
headers = {"Authorization": "Bearer <jwt-token>", "Content-Type": "application/json"}

# Generate restricted presigned PUT URL
resp = requests.post(f"{API}/presign", headers=headers, json={
    "bucket": "uploads",
    "key": "images/photo.jpg",
    "method": "PUT",
    "expires": 3600,
    "maxSize": 10485760,               # 10MB max
    "allowTypes": "image/jpeg,image/png",  # only images
    "requirePrefix": "images/"         # must upload to images/
})
url = resp.json()["url"]

# Upload within restrictions — succeeds
requests.put(url, data=image_data, headers={"Content-Type": "image/jpeg"})

# Upload too large / wrong type / wrong prefix — 403 Forbidden
```

Restriction parameters (`X-Vault-MaxSize`, `X-Vault-AllowTypes`, `X-Vault-RequirePrefix`) are embedded in the signed URL and validated server-side.

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
├── cmd/vaults3/main.go        — Server entry point
├── cmd/vaults3-cli/           — CLI tool (bucket, object, user, replication commands)
├── internal/
│   ├── config/                — YAML config loader
│   ├── server/                — HTTP server and routing
│   ├── s3/                    — S3 API handlers (auth, buckets, objects, multipart)
│   ├── storage/               — Storage engine interface + filesystem + encryption
│   ├── metadata/              — BoltDB metadata store
│   ├── metrics/               — Prometheus-compatible metrics collector
│   ├── iam/                   — IAM policy engine, identity, IP access control
│   ├── notify/                — Event notification dispatcher (webhook delivery)
│   ├── replication/           — Async replication worker (SigV4 signer, queue processor)
│   ├── api/                   — Dashboard REST API (JWT auth, IAM, STS, audit)
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
- [x] IAM users, groups & policies (fine-grained access control, policy evaluation engine, built-in policies)
- [x] CORS per bucket (S3-compatible, OPTIONS preflight)
- [x] STS temporary credentials (short-lived keys, auto-cleanup, configurable max duration)
- [x] Audit trail (persistent log, filtering by user/bucket/time, auto-pruning)
- [x] IP allowlist/blocklist (global and per-user CIDR restrictions, IPv4/IPv6)
- [x] S3 event notifications (per-bucket webhooks, event type + prefix/suffix filtering, retry with backoff)
- [x] Async replication (one-way to peer VaultS3 instances, BoltDB queue, retry with exponential backoff, loop prevention)
- [x] CLI tool (`vaults3-cli` — bucket, object, user, replication management)
- [x] Presigned upload restrictions (max size, content type whitelist, key prefix enforcement)
