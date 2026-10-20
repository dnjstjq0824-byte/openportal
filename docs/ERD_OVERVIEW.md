# DB diagram: logical ownership

```mermaid
flowchart TB
  I[identity: 11 tables] --> Q[query: 12 tables]
  I --> AD[ads: 15 tables]
  I --> P[publisher: 8 tables]
  I --> C[control: 10 tables]
  CR[crawl: 10 tables] --> CAT[catalog: 9 tables]
  CAT --> Q
  CAT --> M[portal: 11 tables]
  Q --> AD
  P --> CR
  C --> CAT
```

This is a schematic of domain references, not an automatically inferred physical ERD. See `SCHEMA_CATALOG.md` for every table and `database/migrations/001_initial.sql` for authoritative PostgreSQL DDL. A production schema migration test remains pending.
