import EmbeddedPostgres from 'embedded-postgres';
import assert from 'node:assert/strict';
import { readFile, mkdir, cp, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import { migrate } from './migrate.mjs';

// A new isolated cluster per execution; never connects to an existing database.
const dir = resolve('work', `pg-${Date.now()}`);
await mkdir(dir, { recursive: true });
const pg = new EmbeddedPostgres({
  databaseDir: dir, user: 'postgres', password: randomBytes(24).toString('hex'),
  port: Number(process.env.TEST_PG_PORT || 55432), persistent: true,
  initdbFlags: ['--encoding=UTF8', '--locale=C'],
  postgresFlags: ['-c', 'listen_addresses=127.0.0.1'],
});
let client;
try {
  await pg.initialise();
  await pg.start();
  client = pg.getPgClient();
  await client.connect();
  console.log((await client.query('select version()')).rows);
  const sql = await readFile('database/migrations/001_initial.sql', 'utf8');
  const created = [...sql.matchAll(/CREATE TABLE (\w+\.\w+)/g)].map(x => x[1]).sort();
  const available = new Set();
  for (const statement of sql.split('CREATE TABLE ').slice(1)) {
    const name = statement.match(/^(\w+\.\w+)/)[1];
    for (const ref of statement.matchAll(/REFERENCES (\w+\.\w+)/g))
      assert(available.has(ref[1]) || ref[1] === name, `forward FK ${name} -> ${ref[1]}`);
    available.add(name);
  }
  await client.query(sql);
  const rows = (await client.query(`select schemaname || '.' || tablename as name from pg_tables
    where schemaname in ('identity','crawl','catalog','query','portal','ads','publisher','control')
    order by name`)).rows.map(x => x.name);
  assert.equal(rows.length, 86);
  assert.deepEqual(rows, created);
  assert.equal(new Set(rows.map(x => x.split('.')[0])).size, 8);
  const fks = await client.query("select count(*)::int as count from pg_constraint where contype='f' and convalidated");
  console.log('PASS baseline: 86 tables / 8 schemas; validated foreign keys:', fks.rows[0].count);
  for (const [name, query, code] of [
    ['FK', "insert into identity.sessions(account_id,token_hash,expires_at) values(gen_random_uuid(),'abc',now())", '23503'],
    ['CHECK', "insert into crawl.hosts(scheme,hostname) values('file','example.test')", '23514'],
    ['UNIQUE', "insert into identity.roles values('user','duplicate')", '23505'],
  ]) {
    await assert.rejects(client.query(query), e => e.code === code);
    console.log(`PASS rejected ${name} violation`);
  }
  await client.query('begin');
  await client.query("insert into identity.accounts(email) values ('rollback@example.test')");
  await client.query('rollback');
  assert.equal((await client.query("select count(*)::int as n from identity.accounts")).rows[0].n, 0);
  console.log('PASS rollback leaves no account');
  await client.end();
  await pg.createDatabase('migration_test');
  client = pg.getPgClient('migration_test');
  await client.connect();
  await migrate(client);
  await migrate(client);
  const migrationCount = (await client.query('select count(*)::int n from public.schema_migrations')).rows[0].n;
  assert.equal(migrationCount, 12);
  console.log('PASS forward migrations (001..012) and repeat run');
  const flags = await client.query("select name, enabled from control.feature_flags order by name");
  assert.equal(flags.rows.find(r => r.name === 'vertical.web')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'vertical.image')?.enabled, false);
  assert.equal(flags.rows.find(r => r.name === 'vertical.news')?.enabled, false);
  assert.equal(flags.rows.find(r => r.name === 'crawler.arbitrary_enabled')?.enabled, false);
  assert.equal(flags.rows.find(r => r.name === 'crawler.bounded_corpus_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'indexer.nori_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'indexer.blue_green_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'indexer.tombstone_purge_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'ranking.intent_classification_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'ranking.safesearch_filter_enabled')?.enabled, true);
  assert.equal(flags.rows.find(r => r.name === 'ranking.rrf_hybrid_enabled')?.enabled, true);
  const profiles = await client.query("select count(*)::int n from query.ranking_profiles where active = true");
  assert.ok(profiles.rows[0].n >= 1);
  const hosts = await client.query("select count(*)::int n from crawl.hosts where fetch_enabled = true");
  assert.ok(hosts.rows[0].n >= 10);
  // Verify Phase 3 catalog schema extensions
  const docCols = (await client.query("select column_name from information_schema.columns where table_schema='catalog' and table_name='documents'")).rows.map(x => x.column_name);
  assert.ok(docCols.includes('simhash'));
  assert.ok(docCols.includes('tombstone_reason'));
  assert.ok(docCols.includes('is_representative'));
  // Migrations must not invent licensed publishers or geographic provenance.
  const pubCount = (await client.query("select count(*)::int n from portal.news_publishers where licensed = true")).rows[0].n;
  assert.equal(pubCount, 0, 'migrations must not grant unverified publisher licenses');
  const placeCount = (await client.query("select count(*)::int n from portal.places")).rows[0].n;
  assert.equal(placeCount, 0, 'place ingestion requires verified source provenance');
  for (const name of ['image_search', 'video_search', 'news_search', 'maps_poi', 'shopping_feed']) {
    assert.equal(flags.rows.find(r => r.name === `vertical.${name}_pipeline`)?.enabled, false);
  }
  const hasClusters = (await client.query("select to_regclass('portal.image_clusters') as relation")).rows[0].relation;
  assert.ok(hasClusters);
  const hasAICache = (await client.query("select to_regclass('portal.ai_query_caches') as relation")).rows[0].relation;
  assert.ok(hasAICache);
  const hasAIFeedback = (await client.query("select to_regclass('portal.ai_feedback') as relation")).rows[0].relation;
  assert.ok(hasAIFeedback);
  const hasDomainVerif = (await client.query("select to_regclass('portal.domain_verifications') as relation")).rows[0].relation;
  assert.ok(hasDomainVerif);
  const hasClickEvents = (await client.query("select to_regclass('ads.click_events') as relation")).rows[0].relation;
  assert.ok(hasClickEvents);
  const hasLedger = (await client.query("select to_regclass('ads.ledger_entries') as relation")).rows[0].relation;
  assert.ok(hasLedger);
  const hasElevations = (await client.query("select to_regclass('control.privileged_elevations') as relation")).rows[0].relation;
  assert.ok(hasElevations);
  const hasDSAR = (await client.query("select to_regclass('identity.dsar_requests') as relation")).rows[0].relation;
  assert.ok(hasDSAR);
  const hasDualApprovals = (await client.query("select to_regclass('control.dual_approvals') as relation")).rows[0].relation;
  assert.ok(hasDualApprovals);
  const hasSecurityAudit = (await client.query("select to_regclass('control.security_audit_events') as relation")).rows[0].relation;
  assert.ok(hasSecurityAudit);
  const hasSLO = (await client.query("select to_regclass('control.slo_measurements') as relation")).rows[0].relation;
  assert.ok(hasSLO);
  const hasDR = (await client.query("select to_regclass('control.dr_rehearsals') as relation")).rows[0].relation;
  assert.ok(hasDR);
  const hasDefects = (await client.query("select to_regclass('control.defect_register') as relation")).rows[0].relation;
  assert.ok(hasDefects);
  const hasReleaseGates = (await client.query("select to_regclass('control.release_gates') as relation")).rows[0].relation;
  assert.ok(hasReleaseGates);
  const gateCount = (await client.query("select count(*)::int n from control.release_gates")).rows[0].n;
  assert.equal(gateCount, 10);
  console.log('PASS Phase 1, 2, 3, 4, 5, 6, 7, 8 & 9 feature flags, ranking profiles, vertical schemas, AI tables, ads, PAM and SLO tables seeded');
  await client.query('create role phase0_untrusted');
  await client.query('set role phase0_untrusted');
  await assert.rejects(client.query('select * from identity.accounts'), e => e.code === '42501');
  await client.query('reset role');
  console.log('PASS unprivileged account access denied');

  // Verify Phase 1 RLS tenant/account isolation (BOLA defense at DB layer)
  await client.query('create role phase1_app_user');
  await client.query('grant usage on schema identity to phase1_app_user');
  await client.query('grant select, insert, update, delete on identity.preferences, identity.recent_searches to phase1_app_user');
  const accA = (await client.query("insert into identity.accounts(email) values ('alice@example.test') returning id")).rows[0].id;
  const accB = (await client.query("insert into identity.accounts(email) values ('bob@example.test') returning id")).rows[0].id;
  await client.query("insert into identity.preferences(account_id, safe_search, theme, language, recent_searches_opt_in) values ($1, 'strict', 'dark', 'ko', true), ($2, 'moderate', 'light', 'en', false)", [accA, accB]);
  await client.query("set role phase1_app_user");
  await client.query(`select set_config('app.current_account_id', '${accA}', false)`);
  const visiblePrefs = (await client.query("select account_id, safe_search from identity.preferences")).rows;
  assert.equal(visiblePrefs.length, 1);
  assert.equal(visiblePrefs[0].account_id, accA);
  // Attempting to update Bob's preferences while authenticated as Alice affects 0 rows
  const bolaUpdate = await client.query("update identity.preferences set theme='dark' where account_id=$1", [accB]);
  assert.equal(bolaUpdate.rowCount, 0);
  await client.query('reset role');
  console.log('PASS Phase 1 RLS prevents cross-account preference read/write (BOLA)');

  const copy = resolve(dir, 'migration-fixtures');
  await cp('database/migrations', copy, {recursive:true});
  await writeFile(resolve(copy, '999_failure.sql'), 'CREATE TABLE public.must_rollback(id int); SELECT 1/0;');
  await assert.rejects(migrate(client, copy), e => e.code === '22012');
  assert.equal((await client.query("select to_regclass('public.must_rollback') as relation")).rows[0].relation,null);
  assert.equal((await client.query('select count(*)::int n from public.schema_migrations')).rows[0].n,migrationCount);
  console.log('PASS failed migration rolls back DDL and ledger');
  await writeFile(resolve(copy, '001_initial.sql'), '-- changed');
  await assert.rejects(migrate(client, copy), /checksum mismatch/);
  console.log('PASS checksum drift rejected');
} finally {
  if (client) await client.end();
  await pg.stop();
}
