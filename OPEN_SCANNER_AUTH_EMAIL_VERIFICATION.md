# OPEN SCANNER — UNIFIED AUTHENTICATION & TRANSACTIONAL EMAIL ENGINE (P0)

**Document Version:** 1.0.0-PROD  
**Component:** Identity & Transactional Email Subsystem (`backend/internal/identity`, `backend/internal/email`)  
**Security Classification:** Critical Infrastructure / P0 Core Service  

---

## 1. System Overview

The Open Scanner Unified Account system provides native, autonomous identity management for global searchers, publishers, and developers. It completely eliminates third-party identity dependencies while preserving fail-closed boundaries for external enterprise SSO.

### 1.1 Core Architecture

```
  ┌────────────────────────────────────────────────────────┐
  │                 Open Scanner Web Client                │
  │     (Vue 3 + Vite, Reactive State, Accessible Modal)   │
  └──────────────────────────┬─────────────────────────────┘
                             │ HTTPS / JSON API
                             ▼
  ┌────────────────────────────────────────────────────────┐
  │              Go API Gateway (Port 8080)                │
  │   - Rate Limiter & Abuse Detector                      │
  │   - CORS & Security Headers Middleware                 │
  └─────────────┬───────────────────────────┬──────────────┘
                │                           │
                ▼                           ▼
  ┌──────────────────────────┐  ┌──────────────────────────┐
  │  identity.Store Engine   │  │   email.Service Engine   │
  │  - PBKDF2 Password Hash  │  │  - 6-digit PIN Generator │
  │  - Cryptographic Salt    │  │  - SMTP Relay / Memory   │
  │  - Session Management    │  │  - Rate Limiter (3/15m)  │
  └─────────────┬────────────┘  └───────────┬──────────────┘
                │                           │
                ▼                           ▼
  ┌──────────────────────────┐  ┌──────────────────────────┐
  │  PostgreSQL 17 Database  │  │  External Mail Protocol  │
  │  - identity.accounts     │  │  - TLS / STARTTLS (587)  │
  │  - identity.sessions     │  │  - DKIM / SPF / DMARC    │
  └──────────────────────────┘  └──────────────────────────┘
```

---

## 2. Cryptographic Security Standards

1. **Password Storage:**
   - Algorithm: **PBKDF2-HMAC-SHA256**.
   - Iterations: $100,000$ iterations minimum.
   - Salt: 64 bytes generated via `crypto/rand` per individual account.
   - Storage format: `pbkdf2_sha256$<iterations>$<salt_hex>$<hash_hex>`.
2. **Email Verification PIN:**
   - 6-character alphanumeric code (`[A-Za-z0-9]`), entropy $> 35\text{ bits}$.
   - Time-to-Live (TTL): 15 minutes strict expiration.
   - Attempt Limiting: Max 3 failed attempts before token invalidation.
3. **Session Tokens:**
   - 32-byte cryptographically secure random token (64 hex characters).
   - SHA-256 hashed before storage in database session table to prevent compromise via database dumps.

---

## 3. API Gateway Endpoints & Contract Specification

### 3.1 Registration: `POST /api/v1/auth/register`
- **Request Body:**
  ```json
  {
    "email": "user@openscanner.internal",
    "password": "SecurePassword123!",
    "display_name": "Open Researcher"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "status": "pending_verification",
    "email": "user@openscanner.internal",
    "code": "A8X2K9",
    "message": "verification email sent"
  }
  ```

### 3.2 Verification: `POST /api/v1/auth/email/verify`
- **Request Body:**
  ```json
  {
    "email": "user@openscanner.internal",
    "code": "A8X2K9"
  }
  ```
- **Response (200 OK):**
  ```json
  {
    "status": "verified",
    "email": "user@openscanner.internal",
    "account_id": "8432a1bb-5d1e-4503-bdf2-f8c5c7219df0"
  }
  ```

### 3.3 Authentication: `POST /api/v1/auth/login`
- **Request Body:**
  ```json
  {
    "email": "user@openscanner.internal",
    "password": "SecurePassword123!"
  }
  ```
- **Response (200 OK):**
  ```json
  {
    "token": "d748f3b2075a31a...",
    "account_id": "8432a1bb-5d1e-4503-bdf2-f8c5c7219df0",
    "email": "user@openscanner.internal",
    "display_name": "Open Researcher",
    "expires_at": "2026-11-07T12:00:00Z"
  }
  ```

### 3.4 Password Reset Request: `POST /api/v1/auth/password/reset-request`
- **Request Body:** `{"email": "user@openscanner.internal"}`
- **Response (200 OK):**
  ```json
  {
    "status": "sent",
    "token": "rst_9918231...",
    "message": "password reset email dispatched"
  }
  ```

### 3.5 Password Reset Confirm: `POST /api/v1/auth/password/reset-confirm`
- **Request Body:**
  ```json
  {
    "token": "rst_9918231...",
    "new_password": "NewSecurePassword456!"
  }
  ```
- **Response (200 OK):**
  ```json
  {
    "status": "password_updated"
  }
  ```

---

## 4. Verification Evidence & Test Execution

- Unit tests in `backend/internal/email/email_test.go`: 100% PASS (`TestService_SendAndVerifyRegistrationToken`, `TestService_SendAndPasswordReset`).
- Unit tests in `backend/internal/identity/local_auth_test.go`: 100% PASS (`TestLocalAccountRegistrationAndLogin`, `TestLocalAccountPasswordReset`).
- End-to-end HTTP integration tests in `backend/cmd/api/http_test.go`: 100% PASS (`TestLocalAuthAndEmailVerificationHTTPEndpoints`).
