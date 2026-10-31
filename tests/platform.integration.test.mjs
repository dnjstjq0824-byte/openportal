import {test,before,after} from 'node:test';
import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import net from 'node:net';
import {startStack,listen} from './helpers/platform-stack.mjs';
import {cleanHTML,scanAttachment} from '../gateway/mail.mjs';
import {createGateway} from '../gateway/server.mjs';
import {safeReturn} from '../gateway/security.mjs';

let stack,base,csrf,cookies='';const origin='http://127.0.0.1:5173';
const email='integration@example.test';const password=randomBytes(18).toString('base64url');
async function api(path,body,method=body?'POST':'GET',extra={}){
  const response=await fetch(base+'/api/v2'+path,{method,headers:{cookie:cookies,origin,'x-csrf-token':csrf||'','content-type':'application/json',...extra},body:body?JSON.stringify(body):undefined});
  for(const cookie of response.headers.getSetCookie()){const part=cookie.split(';')[0];const name=part.split('=')[0];cookies=cookies.split('; ').filter(x=>x&&!x.startsWith(name+'=')).concat(part).join('; ');}
  const data=await response.json();return {status:response.status,data,response};
}
before(async()=>{stack=await startStack({gatewayPort:0});base=`http://127.0.0.1:${stack.gateway.server.address().port}`;csrf=(await api('/session')).data.csrf;});
after(async()=>{if(stack)await stack.close();});
test('registration persists a scrypt hash and sends a token only through real loopback SMTP',async()=>{
  const result=await api('/auth/signup',{email,password,displayName:'Integration',locale:'ko',agree:true});assert.equal(result.status,202);assert.deepEqual(result.data,{accepted:true});
  const account=(await stack.pool.query('select * from webapp.accounts where email=$1',[email])).rows[0];assert.match(account.password_hash,/^scrypt\$32768\$/);assert.ok(!account.password_hash.includes(password));assert.equal(account.verified_at,null);assert.equal(stack.smtp.messages.length,1);
  assert.equal((await api('/auth/login',{email,password})).status,401);
});
test('verification tokens are one-time; login sets HttpOnly SameSite cookies',async()=>{
  const token=stack.smtp.tokenFor(email,'verify');assert.equal((await api('/auth/verify',{token})).status,200);assert.equal((await api('/auth/verify',{token})).status,400);
  const login=await api('/auth/login',{email,password});assert.equal(login.status,200);const cookie=login.response.headers.get('set-cookie');assert.match(cookie,/HttpOnly/);assert.match(cookie,/SameSite=Lax/);assert.ok(!('token'in login.data));
  const current=await api('/session');csrf=current.data.csrf;assert.equal(current.data.user.email,email);
});
test('cross-origin mutations are rejected and unconfigured mail does not invent messages',async()=>{
  assert.equal((await api('/auth/logout',{},'POST',{origin:'https://attacker.invalid'})).status,403);
  const result=await api('/mail/mailboxes');assert.equal(result.status,503);assert.equal(result.data.code,'MAIL_NOT_PROVISIONED');
});
test('session rotation revokes the previous cookie and state survives gateway restart',async()=>{
  const old=cookies;assert.equal((await api('/auth/refresh',{})).status,200);assert.notEqual(cookies,old);
  const stale=await fetch(base+'/api/v2/session',{headers:{cookie:old}});assert.equal((await stale.json()).user,null);
  const restarted=await createGateway(stack.config,{pool:stack.pool});const port=await listen(restarted.server);
  try{const response=await fetch(`http://127.0.0.1:${port}/api/v2/session`,{headers:{cookie:cookies}});assert.equal((await response.json()).user.email,email);}finally{await restarted.close();}
});
let provider,messageId;
test('JMAP protocol simulator: folder/read/update/draft/send and HTML isolation',async()=>{
  provider=await stack.provision(email);messageId=stack.jmap.receive(provider.token,{});
  const boxes=await api('/mail/mailboxes');assert.equal(boxes.status,200);assert.equal(boxes.data.mailboxes.length,6);
  const list=await api(`/mail/messages?mailbox=${provider.id}-inbox`);assert.equal(list.data.messages[0].id,messageId);
  const message=await api('/mail/messages/'+messageId);assert.equal(message.status,200);assert.doesNotMatch(message.data.html,/script|img|tracker/);assert.match(message.data.html,/<p>Hello<\/p>/);
  assert.equal((await api('/mail/messages/'+messageId,{seen:true,flagged:true},'PATCH')).status,200);
  const draft=await api('/mail/drafts',{to:'recipient@example.test',subject:'Protocol test',text:'Body'});assert.equal(draft.status,200);
  const updated=await api('/mail/drafts',{id:draft.data.id,to:'recipient@example.test',subject:'Edited',text:'Updated'});assert.equal(updated.status,200);assert.notEqual(updated.data.id,draft.data.id);
  assert.equal((await api('/mail/messages/'+draft.data.id)).status,404);
  const sent=await api('/mail/send',{id:updated.data.id});assert.equal(sent.status,200);assert.equal(sent.data.accepted,true);
  const retry=await api('/mail/send',{id:updated.data.id});assert.equal(retry.data.submissionId,sent.data.submissionId);
  assert.equal((await api(`/mail/messages?mailbox=${provider.id}-sent`)).data.messages.length,1);
});
test('custom folders can be managed while system and foreign folders are protected',async()=>{
  const created=await api('/mail/mailboxes',{name:'Personal'});assert.equal(created.status,200);
  const id=created.data.id;assert.equal((await api('/mail/mailboxes',{id,name:'Renamed'})).status,200);
  assert.equal((await api('/mail/mailboxes',{id:provider.id+'-inbox',destroy:true})).status,400);
  assert.equal((await api('/mail/mailboxes',{id:'foreign-folder',destroy:true})).status,400);
  assert.equal((await api('/mail/mailboxes',{id,destroy:true})).status,200);
});
test('message, mailbox and attachment identifiers cannot access another provider account',async()=>{
  const other=stack.jmap.provision('other@example.test');const otherId=stack.jmap.receive(other.token,{});
  assert.equal((await api('/mail/messages/'+otherId)).status,404);
  assert.equal((await api('/mail/messages/'+otherId,{seen:true},'PATCH')).status,404);
  assert.equal((await api(`/mail/messages?mailbox=${other.id}-inbox`)).status,404);
  assert.equal((await api(`/mail/attachments/${otherId}/arbitrary`)).status,404);
});
test('scanned upload is account-bound; ungranted blobs cannot be attached',async()=>{
  const response=await fetch(base+'/api/v2/mail/upload',{method:'POST',headers:{cookie:cookies,origin,'x-csrf-token':csrf,'content-type':'text/plain','x-file-name':'test.txt'},body:'Attachment protocol fixture'});assert.equal(response.status,200);const uploaded=await response.json();
  const draft=await api('/mail/drafts',{to:'recipient@example.test',subject:'Attachment',text:'Body',attachments:[uploaded]});assert.equal(draft.status,200);
  const download=await fetch(base+`/api/v2/mail/attachments/${draft.data.id}/${uploaded.blobId}`,{headers:{cookie:cookies}});assert.equal(download.status,200);assert.equal(await download.text(),'Attachment protocol fixture');assert.match(download.headers.get('content-disposition'),/attachment/);
  assert.equal((await api('/mail/drafts',{to:'recipient@example.test',attachments:[{blobId:'foreign'}]})).status,422);
});
test('reset invalidates existing sessions and rejects weak-password token consumption',async()=>{
  assert.equal((await api('/auth/forgot-password',{email})).status,202);const token=stack.smtp.tokenFor(email,'reset');
  assert.equal((await api('/auth/reset-password',{token,password:'short'})).status,400);
  const next=randomBytes(18).toString('base64url');assert.equal((await api('/auth/reset-password',{token,password:next})).status,200);
  const current=await api('/session');csrf=current.data.csrf;assert.equal(current.data.user,null);
  assert.equal((await api('/auth/login',{email,password})).status,401);
  assert.equal((await api('/auth/login',{email,password:next})).status,200);
});
test('generic account responses, login throttling and redirect rejection',async()=>{
  const existing=await api('/auth/signup',{email,password,displayName:'Existing',locale:'ko',agree:true});assert.equal(existing.status,202);assert.deepEqual(existing.data,{accepted:true});
  assert.deepEqual((await api('/auth/forgot-password',{email:'missing@example.test'})).data,{accepted:true});
  for(let i=0;i<9;i++)await api('/auth/login',{email:'unknown@example.test',password});
  assert.equal((await api('/auth/login',{email:'unknown@example.test',password})).status,429);
  for(const input of ['https://evil.invalid/','//evil.invalid/','/\\evil.invalid/','/ko/login'])assert.equal(safeReturn(input,'ko'),'/ko/mail');
  assert.equal(safeReturn('/th/mail?q=x','ko'),'/th/mail?q=x');
});
test('sanitization drops event handlers, external images, styles and unsafe links',()=>{
  const html=cleanHTML('<p style="color:red" onclick="x()">Hi</p><img src="https://tracker.invalid"><a href="javascript:alert(1)">bad</a><script>x()</script>');assert.doesNotMatch(html,/onclick|javascript|tracker|<script|style=/);assert.match(html,/<p>Hi<\/p>/);
});
test('scanner outages and positive malware responses fail closed',async()=>{
  assert.throws(()=>scanAttachment(Buffer.from('test'),''),/SCANNER_UNAVAILABLE/);
  const server=net.createServer(socket=>{socket.on('error',()=>{});socket.once('data',()=>socket.end('stream: Test-Malware FOUND\0'));});const port=await listen(server);
  try{await assert.rejects(scanAttachment(Buffer.from('test'),'127.0.0.1',port),/ATTACHMENT_REJECTED/);}finally{await new Promise(resolve=>server.close(resolve));}
});
test('production ingress requires the operator secret and sets secure host cookies',async()=>{
  const secret=randomBytes(32).toString('hex');const production=await createGateway({...stack.config,production:true,origin:'https://portal.example.test',ingressSecret:secret},{pool:stack.pool});const port=await listen(production.server);
  try{
    const address=`http://127.0.0.1:${port}`;assert.equal((await fetch(address+'/api/v2/session')).status,404);
    const session=await fetch(address+'/api/v2/session',{headers:{'x-gateway-key':secret}});assert.equal(session.status,200);assert.match(session.headers.get('set-cookie'),/__Host-op_csrf=.*; Secure/);
    const metrics=await fetch(address+'/metrics',{headers:{'x-gateway-key':secret}});assert.equal(metrics.status,200);assert.match(await metrics.text(),/openportal_requests_total/);
  }finally{await production.close();}
});
