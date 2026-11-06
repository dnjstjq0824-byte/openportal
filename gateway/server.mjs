import http from 'node:http';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import pg from 'pg';
import nodemailer from 'nodemailer';
import { randomToken, digest, equal, hashPassword, verifyPassword, seal } from './security.mjs';
import { emailText } from './translations.mjs';
import { MailClient, MAX_ATTACHMENT, fail, withMessageLock } from './mail.mjs';
import { braveSearch, hybridSearch, readSearchJSON } from './search.mjs';
import { normalizeUsername, usernameProblem, usernameCandidates, normalizeDomain, parseProfile, displayNameFor } from './signup.mjs';

export function configuration(env = process.env) {
  const origin = new URL(env.PUBLIC_ORIGIN || 'http://127.0.0.1:5173');
  const production = env.NODE_ENV === 'production';
  if (production && (origin.protocol !== 'https:' || !env.DATABASE_URL || !env.RATE_LIMIT_SECRET || !env.GATEWAY_SHARED_SECRET)) throw Error('Production requires HTTPS, DATABASE_URL, RATE_LIMIT_SECRET and GATEWAY_SHARED_SECRET');
  const jmapURL = env.MAIL_JMAP_URL || '';
  if (jmapURL && new URL(jmapURL).protocol !== 'https:' && !(env.NODE_ENV === 'test' && new URL(jmapURL).hostname === '127.0.0.1')) throw Error('JMAP requires HTTPS');
  const mailKey = env.MAIL_ENCRYPTION_KEY ? Buffer.from(env.MAIL_ENCRYPTION_KEY, 'base64') : null;
  if (mailKey && mailKey.length !== 32) throw Error('MAIL_ENCRYPTION_KEY must be 32 bytes in base64');
  const searchProvider = env.SEARCH_PROVIDER || 'opensearch';
  const corpusKind=env.SEARCH_CORPUS_KIND||'development';
  if(!['development','approved-web'].includes(corpusKind))throw Error('Invalid search corpus kind');
  if(corpusKind==='approved-web'&&env.SEARCH_CORPUS_REVIEWED!=='true')throw Error('Approved web corpus requires an operator rights/provenance review');
  if (searchProvider === 'bing') throw Error('The public Bing Search API is retired; configure a supported search provider');
  const validProviders = ['opensearch', 'local', 'brave', 'hybrid'];
  if (!validProviders.includes(searchProvider)) throw Error('Invalid search provider');
  const searchOptions = {
    searchProvider,
    corpusKind,
    braveKey: env.BRAVE_SEARCH_API_KEY,
    braveApproved: env.BRAVE_API_TERMS_APPROVED === 'true',
    braveDailyLimit: Number(env.BRAVE_DAILY_REQUEST_LIMIT || 1000),
    braveRps: Number(env.BRAVE_REQUESTS_PER_SECOND || 1),
  };
  if (!Number.isInteger(searchOptions.braveDailyLimit) || searchOptions.braveDailyLimit < 1 || !Number.isInteger(searchOptions.braveRps) || searchOptions.braveRps < 1) throw Error('Invalid search provider limits');
  if (searchOptions.searchProvider === 'brave' && (!searchOptions.braveKey || !searchOptions.braveApproved)) {
    if (!searchOptions.braveKey) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_SEARCH_API_KEY is missing');
    if (!searchOptions.braveApproved) console.warn('[GATEWAY_CONFIG] WARNING: SEARCH_PROVIDER=brave configured but BRAVE_API_TERMS_APPROVED is not true');
    if (production) throw Error('Production SEARCH_PROVIDER=brave requires BRAVE_SEARCH_API_KEY and BRAVE_API_TERMS_APPROVED=true');
  }
  // Hosted mailbox domain for username@domain signup. Non-production defaults to the reserved .test TLD.
  const mailDomain = env.MAIL_DOMAIN ? normalizeDomain(env.MAIL_DOMAIN) : (production ? null : 'openscanner.test');
  return { ...searchOptions, mailDomain, origin: origin.origin, production, ingressSecret: env.GATEWAY_SHARED_SECRET, databaseURL: env.DATABASE_URL, port: Number(env.GATEWAY_PORT || 8090), host: env.GATEWAY_HOST || '127.0.0.1', searchURL: env.SEARCH_BACKEND_URL || 'http://127.0.0.1:8080', smtpHost: env.SMTP_HOST, smtpPort: Number(env.SMTP_PORT || 587), smtpUser: env.SMTP_USER, smtpPassword: env.SMTP_PASSWORD, smtpFrom: env.SMTP_FROM, smtpSecure: env.SMTP_SECURE === 'true', jmapURL, mailKey, clamHost: env.CLAMAV_HOST, clamPort: Number(env.CLAMAV_PORT || 3310), rateSecret: env.RATE_LIMIT_SECRET || randomToken() };
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
  const searchFetch = dependencies.searchFetch || fetch;
  const pool = dependencies.pool || new pg.Pool({ connectionString: config.databaseURL, max: 10 });
  // Mailbox provisioner: { createMailbox({address,displayName}) -> {token}, deleteMailbox({address}) }.
  // No production adapter exists yet, so hosted signup fails closed unless one is injected.
  const provisioner = dependencies.provisioner || null;
  const signupEnabled = Boolean(config.mailDomain && config.mailKey && config.jmapURL && typeof provisioner?.createMailbox === 'function' && typeof provisioner?.deleteMailbox === 'function');
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
    const result = await pool.query('update webapp.sessions s set last_seen_at=now() from webapp.accounts a where s.account_id=a.id and s.token_hash=$1 and s.expires_at>now() and s.last_seen_at>now()-interval \'30 minutes\' and a.verified_at is not null returning a.id,a.email,a.display_name,a.locale,a.username,a.recovery_email,(a.recovery_email_verified_at is not null) as recovery_email_verified,s.expires_at,a.is_admin', [digest(token)]);
    return result.rows[0] || null;
  }
  async function startSession(res, accountId) {
    const token = randomToken();
    await pool.query('insert into webapp.sessions(token_hash,account_id,expires_at) values($1,$2,now()+interval \'7 days\')', [digest(token), accountId]);
    res.setHeader('Set-Cookie', cookie(sessionName, token, 7 * 86400));
  }
  // Hosted accounts sign in with "username" or "username@domain"; legacy accounts use their email.
  const loginAddress = value => {
    const input = String(value || '').trim().toLowerCase();
    if (!input.includes('@') && config.mailDomain) { const name = normalizeUsername(input); if (usernameProblem(name) === 'format') throw fail('INVALID_INPUT'); return `${name}@${config.mailDomain}`; }
    return normalizeEmail(input);
  };
  async function sendAction(account, purpose, to = account.email) {
    if (!sender) throw fail('EMAIL_NOT_CONFIGURED', 503);
    const token = randomToken();
    await pool.query('insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,$3,now()+interval \'30 minutes\')', [digest(token), account.id, purpose]);
    const strings = emailText[account.locale];
    const link = `${config.origin}/${account.locale}/${purpose === 'verify' ? 'verify-email' : 'reset-password'}#token=${token}`;
    try {
      const result = await sender.sendMail({ from: config.smtpFrom, to, subject: strings[purpose], text: `${strings[purpose + 'Body']}\n\n${link}` });
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
      // Hosted accounts are active at creation; "verify" confirms their recovery email.
      if (purpose === 'verify') await connection.query('update webapp.accounts set verified_at=coalesce(verified_at,now()),recovery_email_verified_at=case when recovery_email is not null then now() else recovery_email_verified_at end where id=$1', [id]);
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
    res.setHeader('Content-Security-Policy',"default-src 'none'; frame-ancestors 'none'"); res.setHeader('X-Frame-Options','DENY'); res.setHeader('Cross-Origin-Resource-Policy','same-origin'); res.setHeader('Permissions-Policy','camera=(), microphone=(), geolocation=(), payment=()');
    if (config.production) res.setHeader('Strict-Transport-Security','max-age=31536000; includeSubDomains');
    const respond = (status, body) => { res.writeHead(status, { 'Content-Type':'application/json; charset=utf-8' }); res.end(JSON.stringify(body)); };
    try {
      const url = new URL(req.url, config.origin); const path = url.pathname;
      const family=path.startsWith('/api/v2/mail/')?'mail':path.startsWith('/api/v2/auth/')?'auth':path==='/api/v2/search'?'search':'other';const started=performance.now();
      res.once('finish',()=>{const key=`${family}:${res.statusCode}`;const previous=metrics.get(key)||{count:0,milliseconds:0};previous.count++;previous.milliseconds+=performance.now()-started;metrics.set(key,previous);});
      if (path === '/healthz') return respond(200, { status: 'ok' });
      if(config.production&&!equal(config.ingressSecret,req.headers['x-gateway-key']))throw fail('NOT_FOUND',404);
      if(config.production)req.clientIp=String(req.headers['x-gateway-client-ip']||req.socket.remoteAddress).slice(0,80);
      if (path === '/readyz') { await pool.query('select 1'); return respond(200, { status: 'ready', smtpConfigured: !!sender, mailConfigured: !!config.jmapURL, mailProvisioning: !!(provisioner && config.mailDomain) }); }
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
      if (path === '/api/v2/auth/username' && req.method === 'GET') {
        await rate(req, 'username-check', 60, 900);
        if (!config.mailDomain) throw fail('MAIL_PROVISIONING_UNAVAILABLE', 503);
        const username = normalizeUsername(url.searchParams.get('name')); const problem = usernameProblem(username);
        if (problem) return respond(200, { available: false, reason: problem, domain: config.mailDomain, signupEnabled, suggestions: [] });
        const address = `${username}@${config.mailDomain}`;
        const taken = (await pool.query('select 1 from webapp.accounts where email=$1 or username=$2', [address, username])).rowCount > 0;
        let suggestions = [];
        if (taken) {
          const candidates = usernameCandidates(username, 6);
          const used = new Set((await pool.query('select username from webapp.accounts where username = any($1)', [candidates])).rows.map(row => row.username));
          suggestions = candidates.filter(name => !used.has(name)).slice(0, 3);
        }
        return respond(200, { available: !taken, reason: taken ? 'taken' : null, address, domain: config.mailDomain, signupEnabled, suggestions });
      }
      if (path === '/api/v2/auth/signup' && req.method === 'POST') {
        const body = await readJSON(req);
        await rate(req, 'signup', 5, 3600);
        if (!signupEnabled) throw fail('MAIL_PROVISIONING_UNAVAILABLE', 503);
        if (body.agree !== true) throw fail('INVALID_INPUT');
        const username = normalizeUsername(body.username); const problem = usernameProblem(username);
        if (problem) throw fail(problem === 'reserved' ? 'USERNAME_RESERVED' : 'INVALID_USERNAME');
        const profile = parseProfile(body, normalizeEmail, config.mailDomain);
        const passwordHash = await hashPassword(body.password);
        const locale = selectLocale(body.locale); const address = `${username}@${config.mailDomain}`; const displayName = displayNameFor(profile, locale);
        // Account row and mailbox are created together: any failure rolls back the row and deletes the mailbox.
        const connection = await pool.connect(); let mailboxCreated = false, accountId;
        try {
          await connection.query('begin');
          const inserted = await connection.query('insert into webapp.accounts(email,username,display_name,password_hash,locale,verified_at,given_name,family_name,birth_date,gender,gender_custom,pronoun,recovery_email,recovery_phone) values($1,$2,$3,$4,$5,now(),$6,$7,$8,$9,$10,$11,$12,$13) on conflict do nothing returning id',
            [address, username, displayName, passwordHash, locale, profile.givenName, profile.familyName, profile.birthDate, profile.gender, profile.genderCustom, profile.pronoun, profile.recoveryEmail, profile.recoveryPhone]);
          if (!inserted.rows.length) throw fail('USERNAME_TAKEN', 409);
          accountId = inserted.rows[0].id;
          let mailbox;
          try { mailbox = await provisioner.createMailbox({ address, displayName }); } catch { throw fail('MAIL_PROVISIONING_FAILED', 502); }
          mailboxCreated = true;
          if (!mailbox || typeof mailbox.token !== 'string' || !mailbox.token) throw fail('MAIL_PROVISIONING_FAILED', 502);
          await connection.query('insert into webapp.mail_connections(account_id,token_ciphertext) values($1,$2)', [accountId, seal(mailbox.token, config.mailKey)]);
          await connection.query('commit');
        } catch (error) {
          await connection.query('rollback');
          if (mailboxCreated) { try { await provisioner.deleteMailbox({ address }); } catch { console.error(JSON.stringify({ event: 'mailbox_compensation_failed', requestId, code: 'ORPHAN_MAILBOX' })); } }
          throw error;
        } finally { connection.release(); }
        await startSession(res, accountId);
        let recoveryEmail = profile.recoveryEmail ? 'not_sent' : 'none';
        if (profile.recoveryEmail && sender) {
          try { await rate(req, 'send', 3, 3600, profile.recoveryEmail); await sendAction({ id: accountId, locale }, 'verify', profile.recoveryEmail); recoveryEmail = 'sent'; } catch { /* account stays usable; resend from the account page */ }
        }
        return respond(201, { created: true, email: address, recoveryEmail });
      }
      if (path === '/api/v2/auth/resend' && req.method === 'POST') {
        // Re-sends the recovery-email confirmation for the signed-in account.
        const account = await user(req); if (!account) throw fail('SESSION_REQUIRED', 401);
        if (!account.recovery_email || account.recovery_email_verified) return respond(202, { accepted: true });
        await rate(req, 'send', 3, 3600, account.recovery_email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED', 503);
        await sendAction(account, 'verify', account.recovery_email);
        return respond(202, { accepted: true });
      }
      if (path === '/api/v2/auth/verify' && req.method === 'POST') { await rate(req,'verify',10); await consumeToken(await readJSON(req),'verify'); return respond(200,{verified:true}); }
      if (path === '/api/v2/auth/forgot-password' && req.method === 'POST') {
        const body = await readJSON(req); const email = loginAddress(body.email ?? body.identifier); await rate(req,'send',3,3600,email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED',503);
        const account = (await pool.query('select id,email,locale,username,recovery_email,recovery_email_verified_at from webapp.accounts where email=$1 and verified_at is not null',[email])).rows[0];
        // Hosted mailboxes cannot receive their own reset while locked out: use the verified recovery email.
        const destination = !account ? null : account.username ? (account.recovery_email_verified_at ? account.recovery_email : null) : account.email;
        if (destination) await sendAction(account,'reset',destination);
        return respond(202,{accepted:true});
      }
      if (path === '/api/v2/auth/reset-password' && req.method === 'POST') { await rate(req,'reset',10); const body = await readJSON(req); const passwordHash = await hashPassword(body.password); await consumeToken(body,'reset',passwordHash); res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{reset:true}); }
      if (path === '/api/v2/auth/login' && req.method === 'POST') {
        const body = await readJSON(req); const email = loginAddress(body.email ?? body.identifier);
        await rate(req,'login',20); await rate(req,'login-email',8,900,email);
        const account = (await pool.query('select * from webapp.accounts where email=$1',[email])).rows[0];
        const valid = await verifyPassword(body.password,account?.password_hash || dummy);
        if (!account || !valid || !account.verified_at) throw fail('INVALID_CREDENTIALS',401);
        await startSession(res, account.id); return respond(200,{authenticated:true});
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
        if ([...q].length > 512) throw fail('INVALID_QUERY');
        if (!q) return respond(200, { query: '', suggestions: [] });
        try {
          const target = new URL('/api/v1/suggest', config.searchURL);
          target.search = new URLSearchParams({ q }).toString();
          const upstream = await searchFetch(target, { redirect: 'error', signal: AbortSignal.timeout(3000) });
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
          const braveRes = await braveSearch(config, q, page, { safeSearch, fetcher: searchFetch });
          return respond(200, { ...braveRes, elapsed_ms: Math.round(performance.now() - started), has_more: Boolean(braveRes.hasMore ?? braveRes.has_more), safe_search: safeSearch });
        }
        if (config.searchProvider === 'hybrid') {
          let searchConfig=config;let budgetLimited=false;
          if(config.braveApproved && config.braveKey){
            if(page>10)throw fail('INVALID_QUERY');
            try{
              await rate(req,'brave-daily',config.braveDailyLimit,86400,'global');
              await rate(req,'brave-per-second',config.braveRps,1,'global');
            }catch(error){if(error.status!==429)throw error;searchConfig={...config,braveApproved:false};budgetLimited=true;}
          }
          const hybridRes = await hybridSearch(searchConfig, q, page, { safeSearch, fetcher: searchFetch });
          return respond(200, { ...hybridRes, corpus_kind:config.corpusKind||'development', degraded: hybridRes.degraded||budgetLimited, has_more: hybridRes.hasMore, elapsed_ms: Math.round(performance.now() - started), safe_search: safeSearch });
        }
        const target = new URL('/api/v1/search', config.searchURL);
        target.search = new URLSearchParams({ q, page: String(page), safe_search: safeSearch }).toString();
        let upstream;
        try { upstream = await searchFetch(target,{redirect:'error',signal:AbortSignal.timeout(8000)}); } catch { throw fail('SEARCH_UNAVAILABLE',503); }
        if (!upstream.ok) throw fail(upstream.status===429?'RATE_LIMITED':'SEARCH_UNAVAILABLE',upstream.status===429?429:503);
        const data = await readSearchJSON(upstream);
        if (!Array.isArray(data.hits) || (data.total !== null && !Number.isInteger(data.total))) throw fail('SEARCH_UNAVAILABLE',502);
        return respond(200, {
          ...data,
          elapsed_ms: Math.round(performance.now() - started),
          has_more: Boolean(data.has_more ?? (data.total !== null && page * 10 < data.total)),
          source: 'opensearch-index',
          corpus_kind: config.corpusKind||'development',
          safe_search: safeSearch,
        });
      }
      const account = await user(req); if (!account) throw fail('SESSION_REQUIRED',401);
      if (path.startsWith('/api/v2/admin/')) {
        if (!account.is_admin) throw fail('FORBIDDEN',403);
        await rate(req,'admin',120,60);
        if (path === '/api/v2/admin/overview' && req.method === 'GET') {
          const counts = (await pool.query(`select (select count(*) from webapp.accounts)::int as accounts,(select count(*) from webapp.sessions where expires_at>now())::int as sessions,(select count(*) from webapp.mail_connections)::int as mailboxes,(select count(*) from webapp.accounts where created_at>now()-interval '24 hours')::int as signups_24h`)).rows[0];
          const recent = (await pool.query('select email,username,display_name,locale,verified_at is not null as verified,created_at from webapp.accounts order by created_at desc limit 10')).rows;
          return respond(200, { ...counts, recent });
        }
        if (path === '/api/v2/admin/accounts' && req.method === 'GET') {
          const q = String(url.searchParams.get('q') || '').trim().slice(0,80);
          const page = Number(url.searchParams.get('page') || 1);
          if (!Number.isInteger(page) || page < 1 || page > 100) throw fail('INVALID_QUERY');
          const where = q ? 'where email ilike $1 or username ilike $1 or display_name ilike $1' : '';
          const args = q ? [`%${q.replace(/[%_\\]/g, '')}%`] : [];
          const total = (await pool.query(`select count(*)::int as total from webapp.accounts ${where}`, args)).rows[0].total;
          const rows = (await pool.query(`select email,username,display_name,locale,verified_at is not null as verified,is_admin,created_at from webapp.accounts ${where} order by created_at desc limit 20 offset ${(page-1)*20}`, args)).rows;
          return respond(200, { total, page, has_more: page*20 < total, accounts: rows });
        }
        throw fail('NOT_FOUND',404);
      }
      if (path === '/api/v2/account' && req.method === 'PATCH') {
        const body = await readJSON(req); const displayName=String(body.displayName||'').trim().slice(0,80); if (!displayName) throw fail('INVALID_INPUT');
        await pool.query('update webapp.accounts set display_name=$1,locale=$2 where id=$3',[displayName,selectLocale(body.locale),account.id]); return respond(200,{updated:true});
      }
      if (path === '/api/v2/account' && req.method === 'DELETE') {
        const body=await readJSON(req); const stored=(await pool.query('select password_hash from webapp.accounts where id=$1',[account.id])).rows[0];
        await rate(req,'delete',5); if (!await verifyPassword(body.password,stored.password_hash)) throw fail('INVALID_CREDENTIALS',401);
        // Provider mailbox deletion requires provider-specific operations. Do not
        // erase the link and falsely claim downstream data was deleted.
        if ((await pool.query('select 1 from webapp.mail_connections where account_id=$1',[account.id])).rows.length) {
          if (!provisioner || !account.username) throw fail('PROVIDER_DELETION_REQUIRED',409);
          try { await provisioner.deleteMailbox({ address: account.email }); } catch { throw fail('PROVIDER_DELETION_FAILED',502); }
        }
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
        if (path === '/api/v2/mail/drafts' && req.method === 'POST') {
          const body=await readJSON(req);
          if(!body.id)return respond(200,await mail.draft(body));
          return respond(200,await withMessageLock(pool,account.id,body.id,async connection=>{
            mail.pool=connection;
            return mail.draft(body);
          }));
        }
        if (path === '/api/v2/mail/send' && req.method === 'POST') {
          await rate(req,'send-mail',30,3600);const body=await readJSON(req);if(typeof body.id!=='string'||body.id.length>256)throw fail('INVALID_INPUT');
          const result=await withMessageLock(pool,account.id,body.id,async connection=>{
            mail.pool=connection;
            const reservation=await connection.query('insert into webapp.mail_send_requests(account_id,email_id) values($1,$2) on conflict do nothing returning email_id',[account.id,body.id]);
            if(!reservation.rows.length){const previous=(await connection.query('select submission_id from webapp.mail_send_requests where account_id=$1 and email_id=$2',[account.id,body.id])).rows[0];if(previous?.submission_id)return {accepted:true,submissionId:previous.submission_id};throw fail('SEND_STATUS_UNKNOWN',409);}
            try{const sent=await mail.send(body.id);await connection.query('update webapp.mail_send_requests set submission_id=$1 where account_id=$2 and email_id=$3',[sent.submissionId,account.id,body.id]);return sent;}catch{throw fail('SEND_STATUS_UNKNOWN',409);}
          });
          return respond(200,result);
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
