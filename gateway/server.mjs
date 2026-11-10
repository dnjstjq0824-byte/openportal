import http from 'node:http';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';
import pg from 'pg';
import nodemailer from 'nodemailer';
import { randomToken, digest, equal, hashPassword, verifyPassword, seal } from './security.mjs';
import { emailText } from './translations.mjs';
import { buildTransactionalEmail, createDualModeMailer } from './mailer.mjs';
import { fail } from './mail.mjs';
import { braveSearch, hybridSearch, readSearchJSON } from './search.mjs';
import { normalizeUsername, usernameProblem, usernameCandidates, parseProfile, displayNameFor } from './signup.mjs';
import { cookieMap, readBody, readJSON, normalizeEmail, selectLocale } from './http.mjs';
import { handleMailRoutes } from './routes-mail.mjs';

// The implementation lives in gateway/config.mjs (Phase 2 split). Imported for
// the main entry below and re-exported so existing importers keep working.
import { configuration } from './config.mjs';
export { configuration };

// C-9 report categories (mirrors the webapp.reports CHECK constraint).
const REPORT_CATEGORIES = new Set(['spam', 'malware', 'phishing', 'adult', 'copyright', 'privacy', 'other']);

export async function createGateway(config, dependencies = {}) {
  const searchFetch = dependencies.searchFetch || fetch;
  const pool = dependencies.pool || new pg.Pool({ connectionString: config.databaseURL, max: 10 });
  // Mailbox provisioner: { createMailbox({address,displayName}) -> {token}, deleteMailbox({address}) }.
  // No production adapter exists yet, so hosted signup fails closed unless one is injected.
  const provisioner = dependencies.provisioner || null;
  const signupEnabled = Boolean(config.mailDomain && config.mailKey && config.jmapURL && typeof provisioner?.createMailbox === 'function' && typeof provisioner?.deleteMailbox === 'function');
  const sender = dependencies.sender || ((config.smtpHost && config.smtpFrom) || config.smtpMode === 'preview' ? createDualModeMailer({ ...config, smtpPass: config.smtpPassword, smtpPoolEnabled: Boolean(config.smtpPoolEnabled) }) : null);
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
    const strings = emailText[account.locale] || emailText.en;
    // Opportunistic hygiene: expired rows free the shared 6-digit OTP namespace.
    await pool.query('delete from webapp.action_tokens where expires_at<=now()');
    // D-5: token rows are bound to the requesting account at insert time and are
    // NEVER reassigned. On a 6-digit OTP hash collision the whole token pair is
    // discarded and regenerated (bounded retries) instead of overwriting the
    // other account's row.
    let token = null, link = null, rendered = null, tokenHash = null, otpHash = null;
    for (let attempt = 0; attempt < 5; attempt++) {
      token = randomToken();
      link = `${config.origin}/${account.locale}/${purpose === 'verify' ? 'verify-email' : 'reset-password'}#token=${token}`;
      rendered = buildTransactionalEmail(purpose, {
        locale: account.locale,
        link,
        token,
        displayName: account.display_name || account.username || '',
        email: to,
        supportEmail: config.smtpReplyTo || 'support@openscanner.com'
      });
      tokenHash = digest(token);
      otpHash = purpose === 'verify' && rendered.otpCode ? digest(`otp:${rendered.otpCode}`) : null;
      const linkInsert = await pool.query('insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,$3,now()+interval \'30 minutes\') on conflict (token_hash) do nothing', [tokenHash, account.id, purpose]);
      if (linkInsert.rowCount !== 1) continue;
      if (!otpHash) break;
      const otpInsert = await pool.query('insert into webapp.action_tokens(token_hash,account_id,purpose,expires_at) values($1,$2,$3,now()+interval \'30 minutes\') on conflict (token_hash) do nothing', [otpHash, account.id, purpose]);
      if (otpInsert.rowCount === 1) break;
      await pool.query('delete from webapp.action_tokens where token_hash=$1 and account_id=$2', [tokenHash, account.id]);
      token = null;
    }
    if (!token) throw fail('OTP_UNAVAILABLE', 503);
    try {
      const result = await sender.sendMail({ from: config.smtpFrom, to, subject: strings[purpose], text: `${strings[purpose + 'Body']}\n\nOTP: ${rendered.otpCode || ''}\n${link}`, html: rendered.html });
      if (!result.accepted?.length && !result.preview) throw Error();
    } catch {
      await pool.query('delete from webapp.action_tokens where token_hash=$1 and account_id=$2', [tokenHash, account.id]);
      if (otpHash) await pool.query('delete from webapp.action_tokens where token_hash=$1 and account_id=$2', [otpHash, account.id]);
      throw fail('EMAIL_DELIVERY_FAILED', 502);
    }
  }
  async function consumeToken(body, purpose, passwordHash) {
    if (typeof body.token !== 'string' || body.token.length > 100) throw fail('INVALID_TOKEN');
    const cleanToken = body.token.trim();
    const isOtp = purpose === 'verify' && /^\d{6}$/.test(cleanToken);
    const candidateHash = isOtp ? digest(`otp:${cleanToken}`) : digest(cleanToken);
    const connection = await pool.connect();
    // settled=true once this call has committed on its own (cleanup paths
    // below); the catch block must not roll back after that.
    let settled = false;
    try {
      await connection.query('begin');
      // D-5: a 6-digit OTP is consumed only for the requesting account. The
      // caller must identify the account; a row owned by anyone else is left
      // untouched (no attempt burned) and the call fails closed.
      let expectedAccountId = null;
      if (isOtp) {
        if (typeof body.email !== 'string' || !body.email.trim()) throw fail('ACCOUNT_REQUIRED', 400);
        const owner = await connection.query('select id from webapp.accounts where lower(email)=lower($1)', [normalizeEmail(body.email)]);
        if (!owner.rows.length) throw fail('INVALID_TOKEN');
        expectedAccountId = owner.rows[0].id;
      }
      const row = await connection.query('select account_id,expires_at,attempts from webapp.action_tokens where token_hash=$1 and purpose=$2 for update', [candidateHash, purpose]);
      if (!row.rows.length) throw fail('INVALID_TOKEN');
      const record = row.rows[0];
      if (isOtp && record.account_id !== expectedAccountId) throw fail('INVALID_TOKEN');
      // Every presentation of a live, account-matching row burns one attempt;
      // the row dies after 5 (replay/double-submit bound). Guessing defense is
      // enforced by endpoint rate limits (per IP + per account).
      const bumped = await connection.query('update webapp.action_tokens set attempts=attempts+1 where token_hash=$1 returning attempts', [candidateHash]);
      if (bumped.rows[0].attempts > 5) {
        await connection.query('delete from webapp.action_tokens where token_hash=$1', [candidateHash]);
        await connection.query('commit');
        settled = true;
        throw fail('INVALID_TOKEN');
      }
      if (new Date(record.expires_at).getTime() <= Date.now()) {
        await connection.query('delete from webapp.action_tokens where token_hash=$1', [candidateHash]);
        await connection.query('commit');
        settled = true;
        throw fail('INVALID_TOKEN');
      }
      const result = await connection.query('delete from webapp.action_tokens where token_hash=$1 and purpose=$2 returning account_id', [candidateHash, purpose]);
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
      return id;
    } catch (error) { if (!settled) await connection.query('rollback'); throw error; } finally { connection.release(); }
  }
  // Phase 4: append-only security audit log. Awaited inline so a lost audit
  // row is impossible on the happy path; if the insert itself fails the user
  // action still succeeds (availability) but the loss is logged loudly.
  // Never pass secrets, tokens, or passwords in meta.
  async function audit({ req, requestId, accountId = null, action, targetType = null, targetId = null, meta = {} }) {
    try {
      const ip = String(req.clientIp || req.socket?.remoteAddress || '').slice(0, 80) || null;
      const userAgent = String(req.headers['user-agent'] || '').slice(0, 300) || null;
      await pool.query(
        'insert into webapp.audit_log(account_id,action,target_type,target_id,ip,user_agent,meta,request_id) values($1,$2,$3,$4,$5,$6,$7,$8)',
        [accountId, action, targetType, targetId, ip, userAgent, meta, requestId || null]
      );
    } catch {
      console.error(JSON.stringify({ event: 'audit_loss', requestId, action, code: 'AUDIT_INSERT_FAILED' }));
    }
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
        // B-5: standard Prometheus exposition (HELP/TYPE + count/sum pair).
        let output='# HELP openportal_requests_total Gateway requests served.\n# TYPE openportal_requests_total counter\n';
        let durations='# HELP openportal_request_duration_seconds Gateway request latency.\n# TYPE openportal_request_duration_seconds summary\n';
        for(const[key,value]of metrics){const[family,status]=key.split(':');output+=`openportal_requests_total{family="${family}",status="${status}"} ${value.count}\n`;durations+=`openportal_request_duration_seconds_sum{family="${family}",status="${status}"} ${value.milliseconds/1000}\nopenportal_request_duration_seconds_count{family="${family}",status="${status}"} ${value.count}\n`;}
        res.writeHead(200,{'Content-Type':'text/plain; version=0.0.4'});return res.end(output+durations);
      }
      if (!path.startsWith('/api/v2/')) return respond(404, { code: 'NOT_FOUND' });
      await rate(req, 'global', 300, 60);
      if (req.method !== 'GET') {
        const token = cookieMap(req)[csrfName];
        const reqOrigin = String(req.headers.origin || '');
        const sameLoopback = !config.production && (
          reqOrigin === config.origin.replace('://127.0.0.1:', '://localhost:') ||
          reqOrigin === config.origin.replace('://localhost:', '://127.0.0.1:')
        );
        if ((reqOrigin !== config.origin && !sameLoopback) || !token || !equal(token, req.headers['x-csrf-token'])) throw fail('CSRF_REJECTED', 403);
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
        await audit({ req, requestId, accountId, action: 'auth.signup' });
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
        await audit({ req, requestId, accountId: account.id, action: 'auth.verify_resent' });
        return respond(202, { accepted: true });
      }
      if (path === '/api/v2/auth/verify' && req.method === 'POST') {
        const body = await readJSON(req);
        await rate(req,'verify',10);
        // D-5: bound 6-digit OTP guessing per account (5/hour), independent of IP rotation.
        if (typeof body.token === 'string' && /^\d{6}$/.test(body.token.trim()) && typeof body.email === 'string' && body.email.trim()) {
          await rate(req,'verify-otp',5,3600,body.email.trim().toLowerCase().slice(0,254));
        }
        const verifiedId = await consumeToken(body,'verify');
        await audit({ req, requestId, accountId: verifiedId, action: 'auth.verify' });
        return respond(200,{verified:true});
      }
      if (path === '/api/v2/auth/forgot-password' && req.method === 'POST') {
        const body = await readJSON(req); const email = loginAddress(body.email ?? body.identifier); await rate(req,'send',3,3600,email);
        if (!sender) throw fail('EMAIL_NOT_CONFIGURED',503);
        const account = (await pool.query('select id,email,locale,username,recovery_email,recovery_email_verified_at from webapp.accounts where email=$1 and verified_at is not null',[email])).rows[0];
        // Hosted mailboxes cannot receive their own reset while locked out: use the verified recovery email.
        const destination = !account ? null : account.username ? (account.recovery_email_verified_at ? account.recovery_email : null) : account.email;
        if (destination) {
          await sendAction(account,'reset',destination);
          await audit({ req, requestId, accountId: account.id, action: 'auth.password_reset_requested' });
        }
        return respond(202,{accepted:true});
      }
      if (path === '/api/v2/auth/reset-password' && req.method === 'POST') { await rate(req,'reset',10); const body = await readJSON(req); const passwordHash = await hashPassword(body.password); const resetId = await consumeToken(body,'reset',passwordHash); await audit({ req, requestId, accountId: resetId, action: 'auth.password_reset' }); res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{reset:true}); }
      if (path === '/api/v2/auth/login' && req.method === 'POST') {
        const body = await readJSON(req); const email = loginAddress(body.email ?? body.identifier);
        await rate(req,'login',20); await rate(req,'login-email',8,900,email);
        const account = (await pool.query('select * from webapp.accounts where email=$1',[email])).rows[0];
        const valid = await verifyPassword(body.password,account?.password_hash || dummy);
        if (!account || !valid || !account.verified_at) {
          await audit({ req, requestId, accountId: account?.id || null, action: 'auth.login_failed' });
          throw fail('INVALID_CREDENTIALS',401);
        }
        await startSession(res, account.id);
        await audit({ req, requestId, accountId: account.id, action: 'auth.login' });
        return respond(200,{authenticated:true});
      }
      if (path === '/api/v2/auth/logout' && req.method === 'POST') {
        const ended = await pool.query('delete from webapp.sessions where token_hash=$1 returning account_id',[digest(cookieMap(req)[sessionName] || '')]);
        if (ended.rows.length) await audit({ req, requestId, accountId: ended.rows[0].account_id, action: 'auth.logout' });
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
        // D-4 / AGENTS.md: the admin surface is NOT implemented. MFA, privileged
        // audit logging, and separate-domain isolation do not exist yet, so every
        // admin route denies by default regardless of is_admin. The previous
        // handler bodies were removed (see git history); reintroduce them only
        // behind the full admin authorization stack.
        await audit({ req, requestId, accountId: account.id, action: 'admin.denied', targetType: 'route', targetId: path.slice(0, 256) });
        throw fail('ADMIN_DISABLED', 403);
      }
      if (path === '/api/v2/account' && req.method === 'PATCH') {
        const body = await readJSON(req); const displayName=String(body.displayName||'').trim().slice(0,80); if (!displayName) throw fail('INVALID_INPUT');
        await pool.query('update webapp.accounts set display_name=$1,locale=$2 where id=$3',[displayName,selectLocale(body.locale),account.id]);
        await audit({ req, requestId, accountId: account.id, action: 'account.updated' });
        return respond(200,{updated:true});
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
        await audit({ req, requestId, accountId: account.id, action: 'account.deleted' });
        await pool.query('delete from webapp.accounts where id=$1',[account.id]); res.setHeader('Set-Cookie',cookie(sessionName,'',0)); return respond(200,{deleted:true});
      }
      // C-9: search-result reports and deletion requests. Authenticated (same
      // bar as the moderation endpoints); review stays manual until the
      // reviewer console lands (admin surface is deny-by-default per D-4).
      if (path === '/api/v2/report' && req.method === 'POST') {
        const body = await readJSON(req);
        await rate(req, 'report', 10, 3600, account.id);
        const documentId = String(body.document_id || '').trim().slice(0, 256);
        const target = String(body.url || '').trim().slice(0, 2048);
        const category = String(body.category || '').trim();
        const details = String(body.details || '').trim().slice(0, 2000);
        if (!documentId || !target || !REPORT_CATEGORIES.has(category)) throw fail('INVALID_INPUT');
        try {
          const parsed = new URL(target);
          if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error('bad protocol');
        } catch { throw fail('INVALID_INPUT'); }
        const row = (await pool.query(
          'insert into webapp.reports(account_id,document_id,url,category,details) values($1,$2,$3,$4,$5) returning id,status,created_at',
          [account.id, documentId, target, category, details]
        )).rows[0];
        await audit({ req, requestId, accountId: account.id, action: 'report.created', targetType: 'report', targetId: row.id });
        return respond(201, { id: row.id, status: row.status, created_at: row.created_at });
      }
      if (path === '/api/v2/report' && req.method === 'GET') {
        const rows = (await pool.query(
          'select id,document_id,url,category,details,status,created_at from webapp.reports where account_id=$1 order by created_at desc limit 50',
          [account.id]
        )).rows;
        return respond(200, { reports: rows });
      }
      if (path.startsWith('/api/v2/mail/')) {
        if (await handleMailRoutes({ req, res, url, path, account, config, pool, rate, respond })) return;
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
