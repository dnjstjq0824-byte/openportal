# OPEN SCANNER — PRODUCTION RELEASE, CANARY & ROLLBACK RUNBOOK

**Document Version:** 1.0.0-PROD  
**Deployment Pipeline:** Cloudflare Pages (Frontend Edge) + Multi-AZ Kubernetes / VM Cluster (Go API & OpenSearch)  
**Recovery Objectives:** RPO $\le 5\text{ minutes}$, RTO $\le 60\text{ seconds}$  

---

## 1. Zero-Downtime Canary Release Strategy

Open Scanner deployments follow a strict automated canary cadence:

```
  [Stage 0: Pre-flight Verification]
  - Database schema checksum validation (`scripts/test-database.mjs`)
  - Go test suite & contract verification (`scripts/check-contracts.mjs`)
  - Frontend production build (`npm run build`)
                 │
                 ▼
  [Stage 1: Internal Dogfooding (Canary 0%)]
  - Route traffic from corporate VPN / dev IPs to Canary Pods
  - Automated smoke tests (`scripts/smoke.sh`)
                 │
                 ▼
  [Stage 2: 5% Global Traffic Shift]
  - Observe error rates, latency p95, and memory usage for 15 minutes
                 │
                 ▼
  [Stage 3: 50% Traffic Shift]
  - Full load soak test across multiple regions
                 │
                 ▼
  [Stage 4: 100% Full Promotion]
  - Deprecate previous revision after 30-minute drain period
```

---

## 2. Automated Anomaly Detection & Abort Triggers

Any of the following signals detected by Prometheus alerting rules trigger an immediate automatic abort and traffic rollback:
1. **HTTP 5xx Error Rate:** $> 0.05\%$ of total search requests over a 2-minute rolling window.
2. **Search Latency Degradation:** SERP p95 latency $> 250\text{ms}$ (baseline $< 180\text{ms}$).
3. **Database Connection Saturation:** PostgreSQL active connection pool $> 85\%$ capacity.
4. **Uncaught Client Exceptions:** Sentry error rate spike $> 10\text{ errors/minute}$.

---

## 3. Layer-by-Layer Rollback Execution Procedures

### 3.1 Edge Frontend (Cloudflare Pages)
- **Rollback Mechanism:** Atomic deployment pointer reversion.
- **Execution:**
  ```bash
  # Instantly repoint production alias to previous verified deployment hash
  npx wrangler pages deployment rollback --project-name=openportal <PREVIOUS_DEPLOYMENT_ID>
  ```
- **Time to Recover:** $< 5\text{ seconds}$ globally.

### 3.2 Go Backend & API Gateway
- **Rollback Mechanism:** Kubernetes deployment rollout undo / container tag switch.
- **Execution:**
  ```bash
  kubectl rollout undo deployment/openscanner-api -n production
  kubectl rollout status deployment/openscanner-api -n production
  ```
- **Time to Recover:** $\le 30\text{ seconds}$.

### 3.3 Database Migrations & Inverted Index
- **Rollback Mechanism:** Transactional schema migration reversal.
- **Execution:**
  - Forward migrations are fully reversible via corresponding down scripts.
  - In the event of catastrophic data corruption, Point-in-Time Recovery (PITR) is restored from the last verified base backup using write-ahead logs (WAL).
  - Validation: Proven by the rollback test in `scripts/test-database.mjs`, which verifies that rolling back leaves zero orphaned records.
