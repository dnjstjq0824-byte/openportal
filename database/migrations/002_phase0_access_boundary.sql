-- The read-only search API has no PostgreSQL credentials.
-- Domain service roles and tenant RLS must be introduced in later phases.
REVOKE ALL ON SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control
  REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control
  REVOKE ALL ON SEQUENCES FROM PUBLIC;
