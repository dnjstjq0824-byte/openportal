import sanitizeHtml from 'sanitize-html';
import net from 'node:net';
import { unseal } from './security.mjs';

const CORE = 'urn:ietf:params:jmap:core';
const MAIL = 'urn:ietf:params:jmap:mail';
const SUBMISSION = 'urn:ietf:params:jmap:submission';
export const MAX_ATTACHMENT = 5 * 1024 * 1024;
export const fail = (code, status = 400) => Object.assign(new Error(code), { status });
export function cleanHTML(html) {
  return sanitizeHtml(html, { allowedTags: ['p','br','div','span','strong','b','em','i','ul','ol','li','blockquote','pre','code','a','h1','h2','h3','table','tr','td','th','tbody','thead'], allowedAttributes: { a: ['href'] }, allowedSchemes: ['http','https','mailto'], allowProtocolRelative: false, transformTags: { a: sanitizeHtml.simpleTransform('a', { rel: 'noreferrer noopener', target: '_blank' }) } });
}
export function scanAttachment(bytes, host, port = 3310) {
  if (!host) throw fail('SCANNER_UNAVAILABLE', 503);
  return new Promise((resolve, reject) => {
    const socket = net.createConnection({ host, port }); let output = ''; let done = false;
    function finish(error) { if (done) return; done = true; socket.destroy(); error ? reject(error) : resolve(); }
    socket.setTimeout(5000, () => finish(fail('SCANNER_UNAVAILABLE', 503)));
    socket.on('error', () => finish(fail('SCANNER_UNAVAILABLE', 503)));
    socket.on('connect', () => {
      socket.write('zINSTREAM\0');
      for (let offset = 0; offset < bytes.length; offset += 65536) {
        const chunk = bytes.subarray(offset, offset + 65536); const size = Buffer.alloc(4); size.writeUInt32BE(chunk.length); socket.write(size); socket.write(chunk);
      }
      socket.write(Buffer.alloc(4));
    });
    socket.on('data', chunk => { output += chunk.toString(); if (output.length > 4096) return finish(fail('SCANNER_UNAVAILABLE', 503)); if (output.includes('\0') || output.includes('\n')) finish(output.includes(': OK') ? undefined : fail('ATTACHMENT_REJECTED', 422)); });
    socket.on('end', () => finish(output.includes(': OK') ? undefined : fail('SCANNER_UNAVAILABLE', 503)));
  });
}

export class MailClient {
  constructor(config, pool, user) { this.config = config; this.pool = pool; this.user = user; }
  async connect() {
    if (!this.config.jmapURL || !this.config.mailKey) throw fail('MAIL_NOT_CONFIGURED', 503);
    const connection = (await this.pool.query('select token_ciphertext from webapp.mail_connections where account_id=$1', [this.user.id])).rows[0];
    if (!connection) throw fail('MAIL_NOT_PROVISIONED', 503);
    this.token = unseal(connection.token_ciphertext, this.config.mailKey);
    this.session = await this.request(this.config.jmapURL);
    this.account = this.session.primaryAccounts?.[MAIL];
    if (!this.account) throw fail('MAIL_PROVIDER_ERROR', 502);
    // Per-mailbox tokens only. Provisioning a shared administrator token is invalid.
    const identities = await this.call('Identity/get', {});
    this.identity = identities.list?.find(x => x.email?.toLowerCase() === this.user.email);
    if (!this.identity) throw fail('MAIL_IDENTITY_MISMATCH', 403);
    return this;
  }
  async request(value, options = {}) {
    const url = new URL(value);
    if (url.origin !== new URL(this.config.jmapURL).origin) throw fail('MAIL_PROVIDER_ERROR', 502);
    const response = await fetch(url, { ...options, headers: { ...options.headers, Authorization: `Bearer ${this.token}` }, redirect: 'error', signal: AbortSignal.timeout(12000) });
    if (!response.ok) throw fail(response.status === 429 ? 'RATE_LIMITED' : response.status === 401 ? 'MAIL_CREDENTIAL_EXPIRED' : 'MAIL_PROVIDER_ERROR', response.status === 429 ? 429 : 502);
    let size=0;const chunks=[];
    for await(const chunk of response.body){size+=chunk.length;if(size>12*1024*1024){await response.body.cancel?.().catch(()=>{});throw fail('MAIL_PROVIDER_ERROR',502);}chunks.push(chunk);}
    const bytes = Buffer.concat(chunks);
    if (options.download) return { bytes, type: response.headers.get('content-type') || 'application/octet-stream' };
    try { return JSON.parse(bytes.toString()); } catch { throw fail('MAIL_PROVIDER_ERROR', 502); }
  }
  async call(method, arguments_) {
    const result = await this.request(this.session.apiUrl, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ using: [CORE, MAIL, SUBMISSION], methodCalls: [[method, { ...arguments_, accountId: this.account }, 'r']] }) });
    const reply = result.methodResponses?.[0];
    if (!reply || reply[0] !== method || reply[1].accountId !== this.account) throw fail('MAIL_PROVIDER_ERROR', 502);
    return reply[1];
  }
  async mailboxes() { return (await this.call('Mailbox/get', { properties: ['id','name','role','totalEmails','unreadEmails'] })).list || []; }
  async manageMailbox(input) {
    const name=String(input.name||'').trim();if(!input.destroy&&(!name||name.length>100))throw fail('INVALID_INPUT');
    if(input.id){const mailbox=(await this.mailboxes()).find(x=>x.id===input.id);if(!mailbox||mailbox.role)throw fail('INVALID_INPUT');
      const result=await this.call('Mailbox/set',input.destroy?{destroy:[input.id],onDestroyRemoveEmails:false}:{update:{[input.id]:{name}}});
      if(result.notDestroyed?.[input.id]||result.notUpdated?.[input.id])throw fail('MAIL_PROVIDER_ERROR',502);
      return {updated:true};
    }
    const result=await this.call('Mailbox/set',{create:{folder:{name,parentId:null}}});
    if(!result.created?.folder)throw fail('MAIL_PROVIDER_ERROR',502);return result.created.folder;
  }
  async list(mailbox, query, position) {
    if (!(await this.mailboxes()).some(x => x.id === mailbox)) throw fail('NOT_FOUND', 404);
    const ids = await this.call('Email/query', { filter: { inMailbox: mailbox, ...(query ? { text: query } : {}) }, sort: [{ property: 'receivedAt', isAscending: false }], position, limit: 30, calculateTotal: true });
    const data = ids.ids?.length ? await this.call('Email/get', { ids: ids.ids, properties: ['id','subject','from','to','receivedAt','preview','keywords','hasAttachment','mailboxIds'] }) : { list: [] };
    return { messages: data.list, total: ids.total, position };
  }
  async read(id) {
    const result = await this.call('Email/get', { ids: [id], properties: ['id','subject','from','to','cc','receivedAt','preview','keywords','mailboxIds','textBody','htmlBody','bodyValues','attachments','messageId','references'], fetchTextBodyValues: true, fetchHTMLBodyValues: true, maxBodyValueBytes: 500000 });
    const email = result.list?.find(x => x.id === id);
    if (!email) throw fail('NOT_FOUND', 404);
    email.text = (email.textBody || []).map(x => email.bodyValues?.[x.partId]?.value || '').join('\n');
    email.html = cleanHTML((email.htmlBody || []).map(x => email.bodyValues?.[x.partId]?.value || '').join('\n'));
    delete email.bodyValues;
    return email;
  }
  async update(id, change) {
    await this.read(id); const patch = {};
    if (typeof change.seen === 'boolean') patch['keywords/$seen'] = change.seen || null;
    if (typeof change.flagged === 'boolean') patch['keywords/$flagged'] = change.flagged || null;
    if (change.mailbox) {
      if (!(await this.mailboxes()).some(x => x.id === change.mailbox)) throw fail('NOT_FOUND', 404);
      patch.mailboxIds = { [change.mailbox]: true };
    }
    if (!Object.keys(patch).length) throw fail('INVALID_INPUT');
    const result = await this.call('Email/set', { update: { [id]: patch } });
    if (result.notUpdated?.[id]) throw fail('MAIL_PROVIDER_ERROR', 502);
    return { updated: true };
  }
  async draft(input) {
    const previous=input.id?await this.read(input.id):null;
    if(previous&&!previous.keywords?.$draft)throw fail('INVALID_INPUT');
    const boxes = await this.mailboxes(); const drafts = boxes.find(x => x.role === 'drafts');
    if (!drafts) throw fail('MAIL_PROVIDER_ERROR', 502);
    const recipients = value => String(value || '').split(',').map(x => x.trim()).filter(Boolean).map(email => {
      if (!/^[^\s<>@,]+@[^\s<>@,]+\.[^\s<>@,]+$/.test(email)) throw fail('INVALID_INPUT');
      return { email };
    });
    const email = { mailboxIds: { [drafts.id]: true }, keywords: { $draft: true, $seen: true }, from: [{ email: this.identity.email }], to: recipients(input.to), cc: recipients(input.cc), subject: String(input.subject || '').slice(0, 998), textBody: [{ partId: 'text', type: 'text/plain' }], bodyValues: { text: { value: String(input.text || '').slice(0, 500000), isTruncated: false } } };
    if(input.replyToId){const parent=await this.read(input.replyToId);email.inReplyTo=parent.messageId||[];email.references=[...(parent.references||[]),...(parent.messageId||[])];}
    if (input.attachments?.length) {
      if (!Array.isArray(input.attachments) || input.attachments.length > 5) throw fail('INVALID_INPUT');
      // Upload grants are stored server-side and bound to this account.
      email.attachments = [];
      for (const item of input.attachments) {
        const grant = (await this.pool.query('select blob_id, name, mime_type, size from webapp.upload_grants where account_id=$1 and blob_id=$2 and expires_at>now()', [this.user.id, item.blobId])).rows[0];
        if (!grant) {
          const old=previous?.attachments?.find(x=>x.blobId===item.blobId);
          if(!old)throw fail('ATTACHMENT_REJECTED',422);
          await this.download(previous.id,old.blobId);
          email.attachments.push({blobId:old.blobId,name:old.name,type:old.type,size:old.size,disposition:'attachment'});
        } else email.attachments.push({ blobId: grant.blob_id, name: grant.name, type: grant.mime_type, size: grant.size, disposition: 'attachment' });
      }
    }
    // RFC 8621: message bodies are immutable, including drafts. Replace only
    // after the new draft was successfully created, preserving the old on error.
    const result = await this.call('Email/set', { create: { draft: email } });
    if (!result.created?.draft?.id) throw fail('MAIL_PROVIDER_ERROR', 502);
    if (input.id) await this.call('Email/set', { destroy: [input.id] });
    return { id: result.created.draft.id };
  }
  async send(id) {
    const draft = await this.read(id);
    if (!draft.keywords?.$draft || !draft.to?.length) throw fail('INVALID_INPUT');
    const sent = (await this.mailboxes()).find(x => x.role === 'sent');
    if (!sent) throw fail('MAIL_PROVIDER_ERROR', 502);
    const result = await this.call('EmailSubmission/set', { create: { submission: { emailId: id, identityId: this.identity.id } }, onSuccessUpdateEmail: { '#submission': { mailboxIds: { [sent.id]: true }, 'keywords/$draft': null } } });
    if (!result.created?.submission?.id) throw fail('MAIL_PROVIDER_ERROR', 502);
    return { accepted: true, submissionId: result.created.submission.id };
  }
  async download(emailId, blobId) {
    const email = await this.read(emailId); const part = email.attachments?.find(x => x.blobId === blobId);
    if (!part) throw fail('NOT_FOUND', 404);
    let url = this.session.downloadUrl;
    for (const [name, value] of Object.entries({ accountId: this.account, blobId, name: part.name || 'attachment', type: 'application/octet-stream' })) url = url.replaceAll(`{${name}}`, encodeURIComponent(value));
    const file=await this.request(url, { download: true });
    if(file.bytes.length>MAX_ATTACHMENT)throw fail('ATTACHMENT_REJECTED',422);
    await scanAttachment(file.bytes,this.config.clamHost,this.config.clamPort);
    return { ...file, name: part.name || 'attachment' };
  }
  async upload(bytes, type, name) {
    if (bytes.length > MAX_ATTACHMENT || !['text/plain','application/pdf','image/png','image/jpeg'].includes(type)) throw fail('ATTACHMENT_REJECTED', 422);
    // Check declared binary signatures; never offer executable or HTML uploads.
    if ((type === 'application/pdf' && !bytes.subarray(0,5).equals(Buffer.from('%PDF-'))) || (type === 'image/png' && bytes.subarray(0,8).toString('hex') !== '89504e470d0a1a0a') || (type === 'image/jpeg' && bytes.subarray(0,3).toString('hex') !== 'ffd8ff') || (type === 'text/plain' && bytes.includes(0))) throw fail('ATTACHMENT_REJECTED', 422);
    await scanAttachment(bytes, this.config.clamHost, this.config.clamPort);
    const result = await this.request(this.session.uploadUrl.replace('{accountId}', encodeURIComponent(this.account)), { method: 'POST', headers: { 'Content-Type': type }, body: bytes });
    if (result.accountId !== this.account || !result.blobId) throw fail('MAIL_PROVIDER_ERROR', 502);
    await this.pool.query('insert into webapp.upload_grants(account_id,blob_id,name,mime_type,size,expires_at) values($1,$2,$3,$4,$5,now()+interval \'24 hours\') on conflict(account_id,blob_id) do update set expires_at=excluded.expires_at', [this.user.id, result.blobId, name.slice(0,180), type, bytes.length]);
    return { blobId: result.blobId, name, type, size: bytes.length };
  }
}
