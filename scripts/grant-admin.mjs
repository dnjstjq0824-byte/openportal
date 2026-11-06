// Operator-only: grant the admin flag to one account by email.
// Usage: DATABASE_URL=... node scripts/grant-admin.mjs <email>
import pg from 'pg';

const email = String(process.argv[2] || '').trim().toLowerCase();
if (!process.env.DATABASE_URL) throw new Error('DATABASE_URL is required');
if (!email || !/^[^\s<>@,]+@[^\s<>@,]+\.[^\s<>@,]+$/.test(email)) throw new Error('usage: grant-admin.mjs <email>');
const client = new pg.Client({ connectionString: process.env.DATABASE_URL });
await client.connect();
try {
  const result = await client.query('update webapp.accounts set is_admin=true where email=$1 returning email', [email]);
  if (!result.rows.length) { console.error(`no account: ${email}`); process.exit(1); }
  console.log(`admin granted: ${result.rows[0].email}`);
} finally { await client.end(); }
