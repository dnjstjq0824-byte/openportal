# OPEN SCANNER — SECURITY, PRIVACY & COMPLIANCE AUDIT REPORT

**Document Version:** 1.0.0-PROD  
**Audit Scope:** Full Application Stack (Frontend, API Gateway, Storage, Search Engine)  
**Security Posture:** Enterprise Commercial Grade / Fail-Closed  

---

## 1. Executive Security Assessment

Open Scanner has been subjected to comprehensive architectural threat modeling and automated regression testing. The platform adheres strictly to the principle of least privilege, zero-trust perimeter boundaries, and zero-tracking privacy architecture.

### 1.1 Summary of Security Controls

| Category | Control Mechanism | Status | Verification Evidence |
|---|---|---|---|
| **Access Control** | Fail-closed administrative boundary | `ENFORCED` | `/admin` and `/partner` return 404/403 (`production_test.go`) |
| **Authentication** | PBKDF2-SHA256 (100k rounds) + 64B Salt | `ENFORCED` | `backend/internal/identity/local_auth_test.go` |
| **Email Verification** | 6-digit cryptographic PIN, 15m TTL | `ENFORCED` | `backend/internal/email/email_test.go` |
| **Session Security** | SHA-256 hashed bearer tokens | `ENFORCED` | `backend/cmd/api/http_test.go` |
| **Data Isolation** | PostgreSQL Row-Level Security (RLS) | `ENFORCED` | BOLA test in `scripts/test-database.mjs` PASS |
| **SSRF Defense** | Scheme validation & private IP filtering | `ENFORCED` | Host check constraint in `001_initial.sql` PASS |
| **Input Validation** | OpenAPI 3.1 & JSON-Schema Ajv gate | `ENFORCED` | `scripts/check-contracts.mjs` PASS |

---

## 2. OWASP Top 10 Detailed Analysis

### 2.1 A01: Broken Access Control (BOLA / IDOR)
- **Vulnerability:** Unauthenticated or horizontal privilege escalation accessing other users' search preferences or identity profiles.
- **Remediation:** PostgreSQL Row-Level Security (`RLS`) strictly binds read/write operations on user preferences to the matching `session.account_id`.
- **Validation:** Executed in `scripts/test-database.mjs`: cross-tenant write attempts are actively rejected with permission denied.

### 2.2 A02: Cryptographic Failures
- **Vulnerability:** Plaintext or weakly hashed credentials (e.g. MD5, SHA1).
- **Remediation:** Passwords are processed with PBKDF2-HMAC-SHA256 with 100,000 iterations and a cryptographically generated 64-byte random salt. All sessions are hashed prior to storage.
- **Validation:** Verified via unit tests in `internal/identity/local_auth_test.go`.

### 2.3 A03: Injection (SQLi, OpenSearch Injection)
- **Vulnerability:** Unsanitized user queries breaking database or search engine query syntax.
- **Remediation:**
  - SQL: All queries use parameterized placeholders (`$1`, `$2`).
  - Search: Queries are converted to a typed AST (`search.ParseQuery`), with fields mapped to boolean leaf nodes.
- **Validation:** High-entropy fuzzing tests and strict schema typing.

### 2.4 A10: Server-Side Request Forgery (SSRF)
- **Vulnerability:** Malicious webmaster submitting loopback (`127.0.0.1`) or file system (`file:///`) seeds to crawler.
- **Remediation:** PostgreSQL check constraint `hosts_scheme_check` restricts host schemes to `http` and `https`. Crawler DNS resolver actively drops RFC 1918 private IP spaces (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`).

---

## 3. Privacy Architecture & Telemetry Guarantees

1. **Anonymous Query Processing:** Unauthenticated search queries are processed statelessly in RAM without associating IP addresses or client fingerprints.
2. **Search History Ownership:** Search history is stored exclusively in client-side `localStorage` unless an authenticated user explicitly opts in.
3. **Right to Erasure:** A one-click purge action immediately wipes all client-side search history and cookies.

