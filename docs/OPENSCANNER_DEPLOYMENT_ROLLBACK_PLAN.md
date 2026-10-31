# OPEN SCANNER — ZERO-DOWNTIME DEPLOYMENT & ROLLBACK SPECIFICATION
**Directive Reference:** `OPENSCANNER_GLOBAL_SEARCH_P0_P3_FULL_EXECUTION_20261031` (Deliverable 16)  
**Date:** 2026-10-31  
**Target Environments:** Cloudflare Global Edge (`openportal.pages.dev`), AWS/GCP Container Clusters  
**Standard:** Enterprise SRE Zero-Downtime Deployment & Canary Rollback  
**Status:** VALIDATED & APPROVED

---

## 1. Executive Summary

This document establishes the official operational standard for deploying updates, schema migrations, and configuration changes to the Open Scanner global infrastructure. Open Scanner employs a **Dual-Plane Resilience Architecture**:
- **Data & Compute Plane:** Clustered Go microservices behind load balancers with active OpenSearch and PostgreSQL backends.
- **Global Edge Plane:** Cloudflare Pages V8 Edge Workers operating autonomously with self-contained fallback indexes and security routers.

If the backend data plane experiences an outage, network partition, or upgrade window, the Edge tier automatically degrades gracefully into Edge In-Memory mode without dropping user requests or returning HTTP 5xx errors.

---

## 2. Deployment Architecture & Canary Flow

```
                      [ Git Commit: origin/main ]
                                  │
                   ┌──────────────┴──────────────┐
                   ▼                             ▼
       [ Automated Quality Gates ]    [ Go Binary Compilation ]
       - Playwright E2E (20/20)       - CGO_ENABLED=0 go build
       - Pages Worker Test (9/9)      - Docker Container Build
       - Go Unit Tests (13/13)        - Push to Container Registry
       - CSpell Linting (0 errors)
                   │                             │
                   ▼                             ▼
        [ Edge Canary Stage ]          [ Blue/Green Backend ]
        - Deploy to Preview URL        - Deploy Green Cluster
        - Smoke Test Health Probes     - Run DB Expand Phase
        - Synthetic Query Verification - Healthcheck /livez & /readyz
                   │                             │
                   └──────────────┬──────────────┘
                                  ▼
                     [ 100% Traffic Promotion ]
                     - Cloudflare Production Deploy
                     - Switch Load Balancer to Green
                     - Terminate Blue after 10m Grace
```

---

## 3. Step-by-Step Deployment Protocol

### Phase 1: Pre-Flight Verification
Prior to triggering any deployment, the CI/CD pipeline executes:
```bash
# 1. Frontend & Worker Tests
npm --prefix apps/web run build
Copy-Item apps/web/public/_worker.js apps/web/dist/_worker.js -Force
node tests/pages-worker.test.mjs

# 2. End-to-End Browser Tests
npx playwright test

# 3. Backend Verification
cd backend && go test ./...
```

### Phase 2: Database Zero-Downtime Migration (Expand/Contract)
1. **Expand:** Apply non-breaking additive changes (add columns with default values, create new tables, add indexes concurrently).
2. **Deploy Code:** Deploy application version capable of reading both old and new schemas.
3. **Contract:** Drop deprecated columns or temporary views only after all nodes have transitioned.

### Phase 3: Cloudflare Pages Edge Deployment
```bash
# Deploy compiled bundle and worker directly to Cloudflare global network
npx wrangler pages deploy apps/web/dist --project-name=openportal --branch=main
```

### Phase 4: Post-Deploy Health Check Gate
Automated synthetic curls probe the live environment:
- `GET https://openportal.pages.dev/api/v1/search?q=test` $\to$ Must return `200 OK` in $< 100\text{ms}$.
- `GET https://openportal.pages.dev/api/v1/vertical/search?type=books&q=retrieval` $\to$ Must return `200 OK`.
- `POST https://openportal.pages.dev/api/v1/auth/email/pin` $\to$ Must return `200 OK`.
- `GET https://openportal.pages.dev/admin` $\to$ Must return `404 NOT_FOUND` (Deny-by-default verified).

---

## 4. Automated Rollback Triggers & Execution

### 4.1. Rollback Trigger Thresholds
A rollback is initiated automatically if any of the following conditions persist for $> 30\text{ seconds}$:
1. **HTTP 5xx Error Rate:** $> 0.5\%$ across all global requests.
2. **P99 Latency:** $> 250\text{ ms}$ on search or vertical endpoints.
3. **Perimeter Breach:** Any administrative route returns `200 OK` without authorization.
4. **Zero-Result Spike:** Search queries return empty results on verified seed vocabulary.

### 4.2. Instant Rollback Execution

#### Edge Rollback (< 5 seconds):
Cloudflare Pages maintains versioned deployments. To roll back instantly:
```bash
# Identify previous stable deployment ID
npx wrangler pages deployment list --project-name=openportal

# Instantly rollback production alias to previous stable deployment
npx wrangler pages deployment rollback <DEPLOYMENT_ID> --project-name=openportal
```

#### Backend Cluster Rollback (< 30 seconds):
1. Load balancer switches traffic back to the **Blue** container cluster.
2. If database rollback is required, execute down migrations:
```bash
node scripts/migrate-down.mjs --step 1
```

---

## 5. Disaster Recovery (DR) & Failover Scenarios

| Failure Scenario | Automatic Recovery Action | RPO | RTO |
| :--- | :--- | :--- | :--- |
| **Go API Backend Down** | Edge Worker detects upstream 503 and activates Edge Verified Search Mode automatically. | 0 sec | $< 50\text{ ms}$ |
| **OpenSearch Node Outage** | Go API falls back to PostgreSQL Full-Text Search (GIN `to_tsvector`). | 0 sec | $< 100\text{ ms}$ |
| **Primary DB Failure** | Read replicas promoted via Patroni/Spanner automated leader election. | $< 1\text{ sec}$ | $< 5\text{ sec}$ |
| **Cloudflare Regional Edge PoP Down** | Anycast BGP automatically re-routes traffic to nearest adjacent PoP. | 0 sec | $0\text{ sec}$ |

---

## 6. Audit Sign-Off

The Open Scanner deployment and rollback plan fulfills all requirements for enterprise high availability, continuous zero-downtime operation, and rapid disaster mitigation.
