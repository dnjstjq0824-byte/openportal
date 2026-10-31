import http from 'node:http';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import pg from 'pg';
import nodemailer from 'nodemailer';
import { randomToken, digest, equal, hashPassword, verifyPassword } from './security.mjs';
import { emailText } from './translations.mjs';
import { MailClient, MAX_ATTACHMENT, fail } from './mail.mjs';
import { braveSearch, bingSearch, hybridSearch } from './search.mjs';

export function configuration(env = process.env) {
  const origin = new URL(env.PUBLIC_ORIGIN || 'http://127.0.0.1:5173');
  const production = env.NODE_ENV === 'production';
  if (production && (origin.protocol !== 'https:' || !env.DATABASE_URL || !env.RATE_LIMIT_SECRET || !env.GATEWAY_SHARED_SECRET)) throw Error('Production requires HTTPS, DATABASE_URL, RATE_LIMIT_SECRET and GATEWAY_SHARED_SECRET');
  const jmapURL = env.MAIL_JMAP_URL || '';
  if (jmapURL && new URL(jmapURL).protocol !== 'https:' && !(env.NODE_ENV === 'test' && new URL(jmapURL).hostname === '127.0.0.1')) throw Error('JMAP requires HTTPS');
  const mailKey = env.MAIL_ENCRYPTION_KEY ? Buffer.from(env.MAIL_ENCRYPTION_KEY, 'base64') : null;
  if (mailKey && mailKey.length !== 32) throw Error('MAIL_ENCRYPTION_KEY must be 32 bytes in base64');
  const searchProvider = env.SEARCH_PROVIDER || 'opensearch';
  const validProviders = ['opensearch', 'local', 'brave', 'bing', 'hybrid'];
  if (!validProviders.includes(searchProvider)) throw Error('Invalid search provider');
  const searchOptions = {
    searchProvider,
    braveKey: env.BRAVE_SEARCH_API_KEY,
    braveApproved: env.BRAVE_API_TERMS_APPROVED === 'true',
    braveDailyLimit: Number(env.BRAVE_DAILY_REQUEST_LIMIT || 1000),
    braveRps: Number(env.BRAVE_REQUESTS_PER_SECOND || 1),
    bingKey: env.BING_SEARCH_API_KEY,
    bingApproved: env.BING_API_TERMS_APPROVED === 'true',
    bingDailyLimit: Number(env.BING_DAILY_REQUEST_LIMIT || 1000),
    bingRps: Number(env.BING_REQUESTS_PER_SECOND || 3),
  };
  if (!Number.isInteger(searchOptions.braveDailyLimit) || searchOptions.braveDailyLimit < 1 || !Number.isInteger(searchOptions.braveRps) || searchOptions.braveRps < 1) throw Error('Invalid search provider limits');
  if (!Number.isInteger(searchOptions.bingDailyLimit) || searchOptions.bingDailyLimit < 1 || !Number.isInteger(searchOptions.bingRps) || searchOptions.bingRps < 1) throw Error('Invalid search provider limits');
  if (searchOptions.searchProvider === 'brave' && (!searchOptions.braveKey || !searchOptions.braveApproved)) {
    if (!searchOptions.braveKey) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_SEARCH_API_KEY is missing');
    if (!searchOptions.braveApproved) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_API_TERMS_APPROVED is not true');
    if (production) throw Error('Production SEARCH_PROVIDER=brave requires BRAVE_SEARCH_API_KEY and BRAVE_API_TERMS_APPROVED=true');
  }
  if (searchOptions.searchProvider === 'bing' && (!searchOptions.bingKey || !searchOptions.bingApproved)) {
    if (!searchOptions.bingKey) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=bing configured but BING_SEARCH_API_KEY is missing');
    if (!searchOptions.bingApproved) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=bing configured but BING_API_TERMS_APPROVED is not true');
    if (production) throw Error('Production SEARCH_PROVIDER=bing requires BING_SEARCH_API_KEY and BING_API_TERMS_APPROVED=true');
  }
  return { ...searchOptions, origin: origin.origin, production, ingressSecret: env.GATEWAY_SHARED_SECRET, databaseURL: env.DATABASE_URL, port: Number(env.GATEWAY_PORT || 8090), host: env.GATEWAY_HOST || '127.0.0.1', searchURL: env.SEARCH_BACKEND_URL || 'http://127.0.0.1:8080', smtpHost: env.SMTP_HOST, smtpPort: Number(env.SMTP_PORT || 587), smtpUser: env.SMTP_USER, smtpPassword: env.SMTP_PASSWORD, smtpFrom: env.SMTP_FROM, smtpSecure: env.SMTP_SECURE === 'true', jmapURL, mailKey, clamHost: env.CLAMAV_HOST, clamPort: Number(env.CLAMAV_PORT || 3310), rateSecret: env.RATE_LIMIT_SECRET || randomToken() };
}
function cookieMap(req) { return Object.fromEntries(String(req.headers.cookie || '').split(';').map(x => x.trim().split('=')).filter(x => x.length === 2)); }
async function readBody(req, max = 600000) {
  let size = 0; const chunks = [];
  for await (const chunk of req) { size += chunk.length; if (size > max) throw fail('PAYLOAD_TOO_LARGE', 413); chunks.push(chunk); }
  return Buffer.concat(chunks);
}
async function readJSON(req) {
  if (!String(req.headers['content-type']).startsWith('application/json')) throw fail('INVALID_INPUT', 415);
  try { const body = JSON.parse((await readBody(req)).toString()); if (!body || Array.isArray(body) || typeof body !== 'object') throw Error(); return body; }
  catch (error) { if (error.status) throw error; throw fail('INVALID_INPUT'); }
}
const normalizeEmail = value => {
  const email = String(value || '').trim().toLowerCase();
  if (email.length > 254 || !/^[^\s<>@,]+@[^\s<>@,]+\.[^\s<>@,]+$/.test(email)) throw fail('INVALID_INPUT');
  return email;
};
const selectLocale = value => ['ko','th','en'].includes(value) ? value : 'en';

export async function createGateway(config, dependencies = {}) {
  const pool = dependencies.pool || new pg.Pool({ connectionString: config.databaseURL, max: 10 });
  const sender = dependencies.sender || (config.smtpHost && config.smtpFrom ? nodemailer.createTransport({ host: config.smtpHost, port: config.smtpPort, secure: config.smtpSecure, requireTLS: config.production && !config.smtpSecure, auth: config.smtpUser ? { user: config.smtpUser, pass: config.smtpPassword } : undefined, connectionTimeout: 8000, greetingTimeout: 8000, socketTimeout: 12000, logger: false, debug: false }) : null);
  config={...config,...(config.searchProvider?{}:{searchProvider:'opensearch'})};
  const dummy = await hashPassword(randomToken());
  const secure = config.production ? '; Secure' : '';
  const sessionName = config.production ? '__Host-op_session' : 'op_session';
  const csrfName = config.production ? '__Host-op_csrf' : 'op_csrf';
  const metrics=new Map();
  const cookie = (name, value, age) => `${name}=${value}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${age}${secure}`;
  async function rate(req, action, limit, windowSeconds = 900, subject = '') {
    const key = digest(`${config.rateSecret}:${action}:${subject || req.clientIp || req.socket.remoteAddress}`);
    const bucket = Math.floor(Date.now() / (windowSeconds * 1000));
    const result = await pool.query('insert into webapp.rate_limits(key_hash,bucket,expires_at) values($1,$2,to_timestamp($3)) on conflict(key_hash,bucket) do update set attempts=webapp.rate_limits.attempts+1 returning attempts', [key, bucket,(bucket+1)*windowSeconds]);
    if (result.rows[0].attempts > limit) throw fail('RATE_LIMITED', 429);
  }
  async function user(req) {
    const token = cookieMap(req)[sessionName]; if (!token) return null;
    const result = await pool.query('update webapp.sessions s set last_seen_at=now() from webapp.accounts a where s.account_id=a.id and s.token_hash=$1 and s.expires_at>now() and s.last_seen_at>now()-interval \'30 minutes\' and a.verified_at is not null returning a.id,a.email,a.display_name,a.locale,s.expires_at', [digest(token)]);
    return result.rows[0] || null;
  }
  async function sendAction(account, purpose) {
    if (!sender) throw fail('EMAIL_NOT_CONFIGURED', 503);
    const token = randomToken();
    await pool.query('insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,$3,now()+interval \'30 minutes\')', [digest(token), account.id, purpose]);
    const strings = emailText[account.locale];
    const link = `${config.origin}/${account.locale}/${purpose === 'verify' ? 'verify-email' : 'reset-password'}#token=${token}`;
    try {
      const result = await sender.sendMail({ from: config.smtpFrom, to: account.email, subject: strings[purpose], text: `${strings[purpose + 'Body']}\n\n${link}` });
      if (!result.accepted?.length) throw Error();
    } catch {
      await pool.query('delete from webapp.action_tokens where token_hash=$1', [digest(token)]);
      throw fail('EMAIL_DELIVERY_FAILED', 502);
    }
  }
  async function consumeToken(body, purpose, passwordHash) {
    if (typeof body.token !== 'string' || body.token.length > 100) throw fail('INVALID_TOKEN');
    const connection = await pool.connect();
    try {
      await connection.query('begin');
      const result = await connection.query('delete from webapp.action_tokens where token_hash=$1 and purpose=$2 and expires_at>now() returning account_id', [digest(body.token), purpose]);
      if (!result.rows.length) throw fail('INVALID_TOKEN');
      const id = result.rows[0].account_id;
      if (purpose === 'verify') await connection.query('update webapp.accounts set verified_at=now() where id=$1', [id]);
      else {
        await connection.query('update webapp.accounts set password_hash=$1 where id=$2', [passwordHash, id]);
        await connection.query('delete from webapp.sessions where account_id=$1', [id]);
      }
      await connection.query('delete from webapp.action_tokens where account_id=$1 and purpose=$2', [id,purpose]);
      await connection.query('commit');
    } catch (error) { await connection.query('rollback'); throw error; } finally { connection.release(); }
  }
  const handler = async (req, res) => {
    const requestId = randomToken().slice(0, 16);
    res.setHeader('Cache-Control', 'no-store'); res.setHeader('X-Content-Type-Options','nosniff'); res.setHeader('Referrer-Policy','no-referrer'); res.setHeader('X-Request-ID',requestId);
    const respond = (status, body) => { res.writeHead(status, { 'Content-Type':'application/json; charset=utf-8' }); res.end(JSON.stringify(body)); };
    try {
      const url = new URL(req.url, config.origin); const path = url.pathname;
      const family=path.startsWith('/api/v2/mail/')?'mail':path.startsWith('/api/v2/auth/')?'auth':path==='/api/v2/search'?'search':'other';const started=performance.now();
      res.once('finish',()=>{const key=`${family}:${res.statusCode}`;const previous=metrics.get(key)||{count:0,milliseconds:0};previous.count++;previous.milliseconds+=performance.now()-started;metrics.set(key,previous);});
      if (path === '/healthz') return respond(200, { status: 'ok' });
      if(config.production&&!equal(config.ingressSecret,req.headers['x-gateway-key']))throw fail('NOT_FOUND',404);
      if(config.production)req.clientIp=String(req.headers['x-gateway-client-ip']||req.socket.remoteAddress).slice(0,80);
      if (path === '/readyz') { await pool.query('select 1'); return respond(200, { status: 'ready', smtpConfigured: !!sender, mailConfigured: !!config.jmapURL }); }
      if(path==='/metrics'&&req.method==='GET'){
        let output='';for(const[key,value]of metrics){const[family,status]=key.split(':');output+=`openportal_requests_total{family="${family}",status="${status}"} ${value.count}\nopenportal_request_duration_seconds_sum{family="${family}",status="${status}"} ${value.milliseconds/1000}\n`;}
        res.writeHead(200,{'Content-Type':'text/plain; version=0.0.4'});return res.end(output);
      }
      if (!path.startsWith('/api/v2/')) return respond(404, { code: 'NOT_FOUND' });
      await rate(req, 'global', 300, 60);
      if (req.method !== 'GET') {
        const token = cookieMap(req)[csrfName];
        if (req.headers.origin !== config.origin || !token || !equal(token, req.headers['x-csrf-token'])) throw fail('CSRF_REJECTED', 403);
      }
      if (path === '/api/v2/session' && req.method === 'GET') {
        const previousCSRF=cookieMap(req)[csrfName];
        const csrf = /^[A-Za-z0-9_-]{43}$/.test(previousCSRF||'')?previousCSRF:randomToken(); res.setHeader('Set-Cookie', cookie(csrfName, csrf, 3600));
        return respond(200, { user: await user(req), csrf });
      }
      if (path === '/api/v2/auth/signup' && req.method === 'POST') {
        const body = await readJSON(req); const email = normalizeEmail(body.email);
        await rate(req, 'signup', 5, 3600); await rate(req, 'send', 3, 3600, email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED', 503);
        const passwordHash = await hashPassword(body.password);
        const displayName = String(body.displayName || '').trim().slice(0,80);
        if (!displayName || body.agree !== true) throw fail('INVALID_INPUT');
        const result = await pool.query('insert into webapp.accounts(email,display_name,password_hash,locale) values($1,$2,$3,$4) on conflict(email) do nothing returning id,email,locale', [email,displayName,passwordHash,selectLocale(body.locale)]);
        if (result.rows[0]) await sendAction(result.rows[0], 'verify');
        return respond(202, { accepted: true });
      }
      if (path === '/api/v2/auth/resend' && req.method === 'POST') {
        const body = await readJSON(req); const email = normalizeEmail(body.email);
        await rate(req,'send',3,3600,email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED',503);
        const account = (await pool.query('select id,email,locale from webapp.accounts where email=$1 and verified_at is null', [email])).rows[0];
        if (account) await sendAction(account,'verify');
        return respond(202,{accepted:true});
      }
      if (path === '/api/v2/auth/verify' && req.method === 'POST') { await rate(req,'verify',10); await consumeToken(await readJSON(req),'verify'); return respond(200,{verified:true}); }
      if (path === '/api/v2/auth/forgot-password' && req.method === 'POST') {
        const body = await readJSON(req); const email = normalizeEmail(body.email); await rate(req,'send',3,3600,email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED',503);
        const account = (await pool.query('select id,email,locale from webapp.accounts where email=$1 and verified_at is not null',[email])).rows[0];
        if (account) await sendAction(account,'reset');
        return respond(202,{accepted:true});
      }
      if (path === '/api/v2/auth/reset-password' && req.method === 'POST') { await rate(req,'reset',10); const body = await readJSON(req); const passwordHash = await hashPassword(body.password); await consumeToken(body,'reset',passwordHash); res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{reset:true}); }
      if (path === '/api/v2/auth/login' && req.method === 'POST') {
        const body = await readJSON(req); const email = normalizeEmail(body.email);
        await rate(req,'login',20); await rate(req,'login-email',8,900,email);
        const account = (await pool.query('select * from webapp.accounts where email=$1',[email])).rows[0];
        const valid = await verifyPassword(body.password,account?.password_hash || dummy);
        if (!account || !valid || !account.verified_at) throw fail('INVALID_CREDENTIALS',401);
        const token = randomToken();
        await pool.query('insert into webapp.sessions(token_hash,account_id,expires_at) values($1,$2,now()+interval \'7 days\')',[digest(token),account.id]);
        res.setHeader('Set-Cookie',cookie(sessionName,token,7*86400)); return respond(200,{authenticated:true});
      }
      if (path === '/api/v2/auth/logout' && req.method === 'POST') {
        await pool.query('delete from webapp.sessions where token_hash=$1',[digest(cookieMap(req)[sessionName] || '')]);
        res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{revoked:true});
      }
      if (path === '/api/v2/auth/refresh' && req.method === 'POST') {
        const account=await user(req); if(!account) throw fail('SESSION_REQUIRED',401);
        const token=randomToken();
        const rotated=await pool.query('update webapp.sessions set token_hash=$1 where token_hash=$2 returning expires_at',[digest(token),digest(cookieMap(req)[sessionName]||'')]);
        if(!rotated.rows.length) throw fail('SESSION_REQUIRED',401);
        const seconds=Math.max(0,Math.floor((new Date(rotated.rows[0].expires_at)-Date.now())/1000));
        res.setHeader('Set-Cookie',cookie(sessionName,token,seconds));return respond(200,{refreshed:true});
      }
      if (path === '/api/v2/suggest' && req.method === 'GET') {
        await rate(req, 'suggest', 120, 60);
        const q = url.searchParams.get('q')?.trim() || '';
        if (!q) return respond(200, { query: '', suggestions: [] });
        try {
          const target = new URL('/api/v1/suggest', config.searchURL);
          target.search = new URLSearchParams({ q }).toString();
          const upstream = await fetch(target, { redirect: 'error', signal: AbortSignal.timeout(3000) });
          if (upstream.ok) {
            const data = await upstream.json();
            const suggestions = Array.isArray(data.suggestions) ? data.suggestions.slice(0, 10) : [];
            return respond(200, { query: q, suggestions, corrected_query: data.corrected_query || undefined });
          }
        } catch {
          // Graceful fallback for suggestions
        }
        return respond(200, { query: q, suggestions: [] });
      }
      if (path === '/api/v2/search' && req.method === 'GET') {
        await rate(req,'search',60,60);
        const q = url.searchParams.get('q')?.trim(); const page = Number(url.searchParams.get('page') || 1);
        const safeSearch = url.searchParams.get('safe_search') || 'moderate';
        if (!q || [...q].length>512 || !Number.isInteger(page) || page<1 || page>100) throw fail('INVALID_QUERY');
        if (!['strict', 'moderate', 'off'].includes(safeSearch)) throw fail('INVALID_INPUT', 400);

        if (config.searchProvider === 'brave') {
          if (!config.braveApproved || !config.braveKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
          await rate(req, 'brave-daily', config.braveDailyLimit, 86400, 'global');
          await rate(req, 'brave-per-second', config.braveRps, 1, 'global');
          const braveRes = await braveSearch(config, q, page, { safeSearch });
          return respond(200, { ...braveRes, elapsed_ms: Math.round(performance.now() - started), has_more: Boolean(braveRes.hasMore ?? braveRes.has_more), safe_search: safeSearch });
        }
        if (config.searchProvider === 'bing') {
          if (!config.bingApproved || !config.bingKey) throw fail('SEARCH_NOT_CONFIGURED', 503);
          await rate(req, 'bing-daily', config.bingDailyLimit, 86400, 'global');
          await rate(req, 'bing-per-second', config.bingRps, 1, 'global');
          const bingRes = await bingSearch(config, q, page, { safeSearch });
          return respond(200, { ...bingRes, elapsed_ms: Math.round(performance.now() - started), has_more: Boolean(bingRes.hasMore ?? bingRes.has_more), safe_search: safeSearch });
        }
        if (config.searchProvider === 'hybrid') {
          const hybridRes = await hybridSearch(config, q, page, { safeSearch });
          return respond(200, { ...hybridRes, elapsed_ms: Math.round(performance.now() - started), safe_search: safeSearch });
        }
        const target = new URL('/api/v1/search', config.searchURL);
        target.search = new URLSearchParams({ q, page: String(page), safe_search: safeSearch }).toString();
        let upstream;
        try { upstream = await fetch(target,{redirect:'error',signal:AbortSignal.timeout(8000)}); } catch { throw fail('SEARCH_UNAVAILABLE',503); }
        if (!upstream.ok) throw fail(upstream.status===429?'RATE_LIMITED':'SEARCH_UNAVAILABLE',upstream.status===429?429:503);
        const data = await upstream.json();
        if (!Array.isArray(data.hits) || (data.total !== null && !Number.isInteger(data.total))) throw fail('SEARCH_UNAVAILABLE',502);
        return respond(200, {
          ...data,
          elapsed_ms: Math.round(performance.now() - started),
          has_more: Boolean(data.has_more ?? (data.total !== null && page * 10 < data.total)),
          source: 'opensearch-index',
          safe_search: safeSearch,
        });
      }
      const account = await user(req); if (!account) throw fail('SESSION_REQUIRED',401);
      if (path === '/api/v2/account' && req.method === 'PATCH') {
        const body = await readJSON(req); const displayName=String(body.displayName||'').trim().slice(0,80); if (!displayName) throw fail('INVALID_INPUT');
        await pool.query('update webapp.accounts set display_name=$1,locale=$2 where id=$3',[displayName,selectLocale(body.locale),account.id]); return respond(200,{updated:true});
      }
      if (path === '/api/v2/account' && req.method === 'DELETE') {
        const body=await readJSON(req); const stored=(await pool.query('select password_hash from webapp.accounts where id=$1',[account.id])).rows[0];
        await rate(req,'delete',5); if (!await verifyPassword(body.password,stored.password_hash)) throw fail('INVALID_CREDENTIALS',401);
        // Provider mailbox deletion requires provider-specific operations. Do not
        // erase the link and falsely claim downstream data was deleted.
        if ((await pool.query('select 1 from webapp.mail_connections where account_id=$1',[account.id])).rows.length) throw fail('PROVIDER_DELETION_REQUIRED',409);
        await pool.query('delete from webapp.accounts where id=$1',[account.id]); res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{deleted:true});
      }
      if (path.startsWith('/api/v2/mail/')) {
        await rate(req,'mail',120,60);
        const mail = await new MailClient(config,pool,account).connect();
        if (path === '/api/v2/mail/mailboxes' && req.method === 'GET') return respond(200,{mailboxes:await mail.mailboxes()});
        if (path === '/api/v2/mail/mailboxes' && req.method === 'POST') return respond(200,await mail.manageMailbox(await readJSON(req)));
        if (path === '/api/v2/mail/messages' && req.method === 'GET') {
          const position=Number(url.searchParams.get('position')||0); if (!Number.isInteger(position)||position<0||position>30000) throw fail('INVALID_INPUT');
          return respond(200,await mail.list(url.searchParams.get('mailbox'),String(url.searchParams.get('q')||'').slice(0,512),position));
        }
        const message=path.match(/^\/api\/v2\/mail\/messages\/([^/]+)$/);
        if (message && req.method === 'GET') return respond(200,await mail.read(decodeURIComponent(message[1])));
        if (message && req.method === 'PATCH') return respond(200,await mail.update(decodeURIComponent(message[1]),await readJSON(req)));
        if (path === '/api/v2/mail/drafts' && req.method === 'POST') return respond(200,await mail.draft(await readJSON(req)));
        if (path === '/api/v2/mail/send' && req.method === 'POST') {
          await rate(req,'send-mail',30,3600);const body=await readJSON(req);if(typeof body.id!=='string'||body.id.length>256)throw fail('INVALID_INPUT');
          const reservation=await pool.query('insert into webapp.mail_send_requests(account_id,email_id) values($1,$2) on conflict do nothing returning email_id',[account.id,body.id]);
          if(!reservation.rows.length){const previous=(await pool.query('select submission_id from webapp.mail_send_requests where account_id=$1 and email_id=$2',[account.id,body.id])).rows[0];if(previous?.submission_id)return respond(200,{accepted:true,submissionId:previous.submission_id});throw fail('SEND_STATUS_UNKNOWN',409);}
          try{const sent=await mail.send(body.id);await pool.query('update webapp.mail_send_requests set submission_id=$1 where account_id=$2 and email_id=$3',[sent.submissionId,account.id,body.id]);return respond(200,sent);}catch{throw fail('SEND_STATUS_UNKNOWN',409);}
        }
        if (path === '/api/v2/mail/upload' && req.method === 'POST') {
          const bytes=await readBody(req,MAX_ATTACHMENT); const type=String(req.headers['content-type']||'');
          let name='attachment';try{name=decodeURIComponent(String(req.headers['x-file-name']||name)).replace(/[^\p{L}\p{N}._ -]/gu,'_');}catch{throw fail('INVALID_INPUT');}
          return respond(200,await mail.upload(bytes,type,name));
        }
        const download=path.match(/^\/api\/v2\/mail\/attachments\/([^/]+)\/([^/]+)$/);
        if (download && req.method === 'GET') {
          const file=await mail.download(decodeURIComponent(download[1]),decodeURIComponent(download[2]));
          res.writeHead(200,{'Content-Type':'application/octet-stream','Content-Disposition':`attachment; filename*=UTF-8''${encodeURIComponent(file.name)}`,'Content-Security-Policy':"default-src 'none'; sandbox"}); return res.end(file.bytes);
        }
      }
      throw fail('NOT_FOUND',404);
    } catch (error) {
      if (res.headersSent) return res.destroy();
      const status=error.status || 500; if (status===429) res.setHeader('Retry-After','900');
      // Only safe error categories and request IDs: no URLs, SQL, bodies or tokens.
      if (!error.status) console.error(JSON.stringify({event:'gateway_failure',requestId,code:'INTERNAL_ERROR'}));
      respond(status,{code:error.status?error.message:'INTERNAL_ERROR',requestId});
    }
  };
  const server=http.createServer(handler); server.requestTimeout=20000; server.headersTimeout=10000;
  const maintenance=setInterval(async()=>{try{await pool.query("delete from webapp.action_tokens where expires_at<now(); delete from webapp.sessions where expires_at<now() or last_seen_at<now()-interval '30 minutes'; delete from webapp.rate_limits where expires_at<now(); delete from webapp.upload_grants where expires_at<now()");}catch{console.error(JSON.stringify({event:'maintenance_failure',code:'DATABASE_UNAVAILABLE'}));}},60000);maintenance.unref();
  return {server,pool,close:async()=>{clearInterval(maintenance);await new Promise(resolve=>server.close(resolve)); if (!dependencies.pool) await pool.end();}};
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const config=configuration(); if (!config.databaseURL) throw Error('DATABASE_URL is required; run migrations before starting');
  const app=await createGateway(config); app.server.listen(config.port,config.host,()=>console.log(`Gateway listening on ${config.host}:${config.port}`));
  for (const signal of ['SIGINT','SIGTERM']) process.on(signal,async()=>{await app.close();process.exit(0);});
}
