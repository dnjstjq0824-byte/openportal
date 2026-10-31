// Operator-only CLI. This is deliberately not an HTTP account provisioning API.
import pg from 'pg';
import {configuration} from './server.mjs';
import {MailClient} from './mail.mjs';
import {seal} from './security.mjs';
const config=configuration();const email=process.env.MAILBOX_EMAIL?.trim().toLowerCase();const token=process.env.MAILBOX_TOKEN;
if(!email||!token||!config.databaseURL||!config.mailKey)throw Error('Set DATABASE_URL, MAILBOX_EMAIL, MAILBOX_TOKEN, MAIL_JMAP_URL and MAIL_ENCRYPTION_KEY in the operator environment');
const pool=new pg.Pool({connectionString:config.databaseURL});
try{
  const account=(await pool.query('select id,email from webapp.accounts where email=$1 and verified_at is not null',[email])).rows[0];
  if(!account)throw Error('A verified account is required');
  const ciphertext=seal(token,config.mailKey);
  const candidatePool={query:async()=>({rows:[{token_ciphertext:ciphertext}]})};
  await new MailClient(config,candidatePool,account).connect();
  await pool.query('insert into webapp.mail_connections(account_id,token_ciphertext) values($1,$2) on conflict(account_id) do update set token_ciphertext=excluded.token_ciphertext,updated_at=now()',[account.id,ciphertext]);
  console.log('Mailbox connection verified and stored. No credentials were printed.');
}finally{await pool.end();}
