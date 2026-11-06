// Local development stack (DEV ONLY): persistent embedded PostgreSQL on
// 127.0.0.1:55440 plus the gateway on 8090 with simulator-backed mail
// (SMTP capture, JMAP fixture, ClamAV stub, in-memory provisioner).
// Search calls the Go API at SEARCH_BACKEND_URL (default :8081).
// Run from the repository root: node scripts/dev-stack.mjs
import EmbeddedPostgres from 'embedded-postgres';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import net from 'node:net';
import pg from 'pg';
import { migrate } from './migrate.mjs';
import { configuration, createGateway } from '../gateway/server.mjs';
import { seal } from '../gateway/security.mjs';
import { smtpFixture, jmapFixture, listen } from '../tests/helpers/platform-stack.mjs';

const dir = resolve('work/dev-postgres');
await mkdir(dir, { recursive: true });
async function secret(name, bytes, encoding = 'base64') {
  // Beside the data dir: initdb refuses a non-empty directory.
  const path = resolve('work', name);
  try { const value = (await readFile(path, 'utf8')).trim(); if (value) return value; } catch {}
  const value = randomBytes(bytes).toString(encoding);
  await writeFile(path, value + '\n', { mode: 0o600 });
  return value;
}
let fresh = false;
try { await readFile(resolve(dir, 'PG_VERSION'), 'utf8'); } catch { fresh = true; }
const pgPassword = await secret('pg-password', 24, 'base64url');
const mailKey = await secret('mail-key', 32);
const postgres = new EmbeddedPostgres({ databaseDir: dir, user: 'postgres', password: pgPassword, port: 55440, persistent: true, initdbFlags: ['--encoding=UTF8', '--locale=C'], postgresFlags: ['-c', 'listen_addresses=127.0.0.1', '-c', 'log_statement=none', '-c', 'log_min_error_statement=panic'] });
if (fresh) await postgres.initialise();
await postgres.start();
const pool = new pg.Pool({ host: '127.0.0.1', port: 55440, user: 'postgres', password: pgPassword, database: 'postgres' });
const client = await pool.connect();
try { await migrate(client); } finally { client.release(); }
const smtp = await smtpFixture(), jmap = await jmapFixture();
const scanner = net.createServer(socket => { socket.on('data', chunk => { if (chunk.subarray(-4).equals(Buffer.alloc(4))) socket.end('stream: OK\0'); }); });
const clamPort = await listen(scanner);
// Loopback-http JMAP fixture requires the test allowance in configuration().
process.env.NODE_ENV = 'test';
const config = configuration({ ...process.env, PUBLIC_ORIGIN: 'http://127.0.0.1:5173', GATEWAY_PORT: '8090', SEARCH_BACKEND_URL: process.env.SEARCH_BACKEND_URL || 'http://127.0.0.1:8081', SMTP_HOST: '127.0.0.1', SMTP_PORT: String(smtp.port), SMTP_FROM: 'OpenScanner dev <dev@openscanner.test>', MAIL_JMAP_URL: jmap.url, MAIL_ENCRYPTION_KEY: mailKey, CLAMAV_HOST: '127.0.0.1', CLAMAV_PORT: String(clamPort) });
const provisioner = { async createMailbox({ address }) { return jmap.provision(address); }, async deleteMailbox({ address }) { jmap.deprovision(address); } };
const gateway = await createGateway(config, { pool, provisioner });
await listen(gateway.server, 8090);
// The JMAP fixture is in-memory: reconnect mailboxes from earlier boots.
const rows = (await pool.query('select a.email, c.account_id from webapp.mail_connections c join webapp.accounts a on a.id=c.account_id')).rows;
for (const row of rows) {
  const provider = jmap.provision(row.email);
  await pool.query('update webapp.mail_connections set token_ciphertext=$1 where account_id=$2', [seal(provider.token, config.mailKey), row.account_id]);
}
console.log(`dev gateway ready: http://127.0.0.1:8090 (postgres 55440, search ${config.searchURL}, mailboxes reconnected: ${rows.length})`);
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, async () => {
  await gateway.close(); await smtp.close(); await jmap.close();
  await new Promise(resolve => scanner.close(resolve)); await pool.end(); await postgres.stop();
  process.exit(0);
});
