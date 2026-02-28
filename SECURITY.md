# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them privately via one of these methods:

1. **GitHub Security Advisories** — [Report a vulnerability](https://github.com/eniz1806/VaultS3/security/advisories/new) (preferred)
2. **Email** — Send details to the repository owner

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### What to Expect

- Acknowledgment within **48 hours**
- Status update within **7 days**
- Fix timeline depends on severity:
  - **Critical** (RCE, auth bypass, data leak): patch within 72 hours
  - **High** (privilege escalation, injection): patch within 1 week
  - **Medium/Low**: patch in next release

### Scope

The following are in scope:
- S3 API authentication bypass
- Encryption key exposure
- Path traversal / unauthorized file access
- JWT token vulnerabilities
- SQL/command injection
- Dashboard XSS or CSRF
- CORS misconfiguration allowing data theft
- Rate limiting bypass
- Raft cluster membership manipulation
- Replication sync endpoint abuse
- Inter-node proxy loop exploitation

### Out of Scope

- Denial of service (single-binary, expected to run behind a proxy in production)
- Vulnerabilities in dependencies (report upstream)
- Issues requiring physical access to the server

## Security Features

VaultS3 includes multiple security layers:

- **S3 Signature V4** authentication with full signature verification (including presigned URLs)
- **AES-256-GCM** encryption at rest
- **JWT-based** dashboard authentication (24h tokens, admin key masked in responses)
- **Constant-time credential comparison** (`crypto/hmac.Equal` prevents timing attacks on login)
- **IAM policies** with default-deny evaluation
- **Security headers** (CSP, X-Frame-Options, HSTS, X-Content-Type-Options, Referrer-Policy)
- **CORS origin validation** (same-origin + localhost only)
- **API rate limiting** (token bucket per RemoteAddr IP, not spoofable via X-Forwarded-For)
- **Input validation** (DNS-compatible bucket names, object key length/null byte/path traversal checks — enforced on all S3, API, and versioning endpoints)
- **Path traversal prevention** (rejects `..` segments in object keys at S3 handler, API, versioning API, CopyObject/UploadPartCopy source, and filesystem layers)
- **SSRF protection** (webhook, notification, and lambda function URLs validated against localhost, private IPs, link-local, and cloud metadata endpoints)
- **Upload size limits** (5GB per PUT object, 5GB per multipart part — enforced via `http.MaxBytesReader`)
- **IP allowlist/blocklist** (global and per-user CIDR)
- **Audit trail** with auto-pruning
- **Object locking (WORM)** for compliance
- **STS temporary credentials** with auto-expiry
- **Content-Disposition sanitization** (filenames escaped to prevent header injection)
- **Non-root Docker container** (runs as `vaults3` user, UID 1000)
- **Default credential warning** (startup log warning when using default admin credentials)
- **Error message sanitization** (OIDC and health check errors return generic messages, no internal detail leaking)
- **Raft consensus** (cluster writes require majority quorum, preventing split-brain)
- **Proxy loop prevention** (`X-VaultS3-Proxy` header prevents infinite request forwarding between cluster nodes)
- **Replication SSRF protection** (peer URLs validated against localhost, private IPs, link-local, and cloud metadata endpoints)
- **Replication peer authentication** (bidirectional sync uses SigV4-signed requests, peer access keys registered at startup)
- **Rebalance isolation** (`X-VaultS3-Rebalance` header marks internal object transfers)

## Deployment Best Practices

- **Always change default credentials** — set `VAULTS3_ACCESS_KEY` and `VAULTS3_SECRET_KEY` environment variables
- **Use HTTPS in production** — set `VAULTS3_TLS_CERT` and `VAULTS3_TLS_KEY`, or run behind a reverse proxy with TLS termination
- **Run behind a reverse proxy** (nginx, Caddy, Traefik) for additional rate limiting, IP filtering, and access logging
- **Enable encryption at rest** — set `VAULTS3_ENCRYPTION_KEY` with a random 64-char hex key (`openssl rand -hex 32`)
- **Restrict network access** — use firewall rules to limit who can reach port 9000 (and Raft port 9001 in cluster mode)
- **Isolate Raft traffic** — bind the Raft port to a private network interface, never expose it publicly
- **Secure replication peers** — use unique access keys per peer, rotate credentials regularly
- **Monitor the audit trail** — review `/api/v1/audit` for suspicious activity
- **Monitor cluster health** — check `/cluster/status` and `/health` endpoints for node failures
- **Keep VaultS3 updated** — pull the latest Docker image regularly
