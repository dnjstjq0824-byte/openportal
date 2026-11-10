import { MailClient, MAX_ATTACHMENT, fail, withMessageLock } from './mail.mjs';
import { readBody, readJSON } from './http.mjs';

// Mail route table, extracted from server.mjs (Phase 2). Returns true when a
// route matched and the response was sent, false to fall through to 404.
export async function handleMailRoutes(ctx) {
  const { req, res, url, path, account, config, pool, rate, respond } = ctx;
  await rate(req,'mail',120,60);
  const mail = await new MailClient(config,pool,account).connect();
  if (path === '/api/v2/mail/mailboxes' && req.method === 'GET') { respond(200,{mailboxes:await mail.mailboxes()}); return true; }
  if (path === '/api/v2/mail/mailboxes' && req.method === 'POST') { respond(200,await mail.manageMailbox(await readJSON(req))); return true; }
  if (path === '/api/v2/mail/messages' && req.method === 'GET') {
    const position=Number(url.searchParams.get('position')||0); if (!Number.isInteger(position)||position<0||position>30000) throw fail('INVALID_INPUT');
    respond(200,await mail.list(url.searchParams.get('mailbox'),String(url.searchParams.get('q')||'').slice(0,512),position)); return true;
  }
  const message=path.match(/^\/api\/v2\/mail\/messages\/([^/]+)$/);
  if (message && req.method === 'GET') { respond(200,await mail.read(decodeURIComponent(message[1]))); return true; }
  if (message && req.method === 'PATCH') { respond(200,await mail.update(decodeURIComponent(message[1]),await readJSON(req))); return true; }
  if (path === '/api/v2/mail/drafts' && req.method === 'POST') {
    const body=await readJSON(req);
    if(!body.id){ respond(200,await mail.draft(body)); return true; }
    respond(200,await withMessageLock(pool,account.id,body.id,async connection=>{
      mail.pool=connection;
      return mail.draft(body);
    })); return true;
  }
  if (path === '/api/v2/mail/send' && req.method === 'POST') {
    await rate(req,'send-mail',30,3600);const body=await readJSON(req);if(typeof body.id!=='string'||body.id.length>256)throw fail('INVALID_INPUT');
    const result=await withMessageLock(pool,account.id,body.id,async connection=>{
      mail.pool=connection;
      const reservation=await connection.query('insert into webapp.mail_send_requests(account_id,email_id) values($1,$2) on conflict do nothing returning email_id',[account.id,body.id]);
      if(!reservation.rows.length){const previous=(await connection.query('select submission_id from webapp.mail_send_requests where account_id=$1 and email_id=$2',[account.id,body.id])).rows[0];if(previous?.submission_id)return {accepted:true,submissionId:previous.submission_id};throw fail('SEND_STATUS_UNKNOWN',409);}
      try{const sent=await mail.send(body.id);await connection.query('update webapp.mail_send_requests set submission_id=$1 where account_id=$2 and email_id=$3',[sent.submissionId,account.id,body.id]);return sent;}catch{throw fail('SEND_STATUS_UNKNOWN',409);}
    });
    respond(200,result); return true;
  }
  if (path === '/api/v2/mail/upload' && req.method === 'POST') {
    const bytes=await readBody(req,MAX_ATTACHMENT); const type=String(req.headers['content-type']||'');
    let name='attachment';try{name=decodeURIComponent(String(req.headers['x-file-name']||name)).replace(/[^\p{L}\p{N}._ -]/gu,'_');}catch{throw fail('INVALID_INPUT');}
    respond(200,await mail.upload(bytes,type,name)); return true;
  }
  const download=path.match(/^\/api\/v2\/mail\/attachments\/([^/]+)\/([^/]+)$/);
  if (download && req.method === 'GET') {
    const file=await mail.download(decodeURIComponent(download[1]),decodeURIComponent(download[2]));
    res.writeHead(200,{'Content-Type':'application/octet-stream','Content-Disposition':`attachment; filename*=UTF-8''${encodeURIComponent(file.name)}`,'Content-Security-Policy':"default-src 'none'; sandbox"}); res.end(file.bytes); return true;
  }
  return false;
}
