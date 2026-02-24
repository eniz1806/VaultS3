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

### Out of Scope

- Denial of service (single-binary, expected to run behind a proxy in production)
- Vulnerabilities in dependencies (report upstream)
- Issues requiring physical access to the server

## Security Features

VaultS3 includes multiple security layers:

- **S3 Signature V4** authentication
- **AES-256-GCM** encryption at rest
- **JWT-based** dashboard authentication (24h tokens)
- **IAM policies** with default-deny evaluation
- **Security headers** (CSP, X-Frame-Options, HSTS, X-Content-Type-Options, Referrer-Policy)
- **CORS origin validation** (same-origin + localhost only)
- **API rate limiting** (token bucket per IP)
- **Input validation** (bucket names, object keys)
- **IP allowlist/blocklist** (global and per-user CIDR)
- **Audit trail** with auto-pruning
- **Object locking (WORM)** for compliance
- **STS temporary credentials** with auto-expiry
