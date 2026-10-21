import { readdir, readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import pg from 'pg';

export async function migrate(client, directory = 'database/migrations') {
  await client.query("select pg_advisory_lock(709230001)");
  try {
    await client.query(`create table if not exists public.schema_migrations(
      version text primary key, sha256 text not null check(length(sha256)=64),
      applied_at timestamptz not null default now())`);
    await client.query('revoke all on public.schema_migrations from public');
    const files = (await readdir(directory)).filter(x => /^\d{3}_.+\.sql$/.test(x)).sort();
    const applied = (await client.query('select version,sha256 from public.schema_migrations order by version')).rows;
    for (let i=0; i<applied.length; i++) {
      if (files[i] !== applied[i].version) throw new Error('migration history is not a prefix of files');
    }
    for (const file of files) {
      const sql = await readFile(resolve(directory, file), 'utf8');
      const hash = createHash('sha256').update(sql).digest('hex');
      const previous = applied.find(x => x.version === file);
      if (previous) {
        if (previous.sha256 !== hash) throw new Error(`checksum mismatch: ${file}`);
        continue;
      }
      // Legacy baseline has its own outer transaction. Runner owns atomic DDL + ledger.
      const body = sql.replace(/^BEGIN;\s*$/m, '').replace(/^COMMIT;\s*$/m, '');
      if (/\b(BEGIN|COMMIT|ROLLBACK)\s*;/i.test(body)) throw new Error('nested transaction forbidden');
      await client.query('begin');
      try {
        await client.query(body);
        await client.query('insert into public.schema_migrations(version,sha256) values($1,$2)', [file,hash]);
        await client.query('commit');
        console.log(`APPLIED ${file} ${hash}`);
      } catch (error) {
        await client.query('rollback');
        throw error;
      }
    }
  } finally {
    await client.query('select pg_advisory_unlock(709230001)');
  }
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  if (!process.env.DATABASE_URL) throw new Error('DATABASE_URL is required; use a dedicated migration role');
  const client = new pg.Client({connectionString: process.env.DATABASE_URL});
  await client.connect();
  try { await migrate(client); } finally { await client.end(); }
}
