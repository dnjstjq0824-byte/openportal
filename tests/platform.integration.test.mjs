import {test,before,after} from 'node:test';
import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import net from 'node:net';
import {startStack,listen} from './helpers/platform-stack.mjs';
import {cleanHTML,scanAttachment,withMessageLock} from '../gateway/mail.mjs';
import {createGateway} from '../gateway/server.mjs';
import {safeReturn} from '../gateway/security.mjs';

let stack,base,csrf,cookies='';const origin='http://127.0.0.1:5173';
const username='integration.user';const email=`${username}@openscanner.test`;const recovery='owner@example.test';
let password=randomBytes(18).toString('base64url');
const profile=(overrides={})=>({givenName:'통합',familyName:'테스트',birthDate:'1990-05-17',gender:'custom',genderCustom:'Non-binary',pronoun:'other',username,password,recoveryEmail:recovery,recoveryPhone:'+82 10-1234-5678',locale:'ko',agree:true,...overrides});
async function api(path,body,method=body?'POST':'GET',extra={},target=base){
  const response=await fetch(target+'/api/v2'+path,{method,headers:{cookie:cookies,origin,'x-csrf-token':csrf||'','content-type':'application/json',...extra},body:body?JSON.stringify(body):undefined});
  for(const cookie of response.headers.getSetCookie()){const part=cookie.split(';')[0];const name=part.split('=')[0];cookies=cookies.split('; ').filter(x=>x&&!x.startsWith(name+'=')).concat(part).join('; ');}
  const data=await response.json();return {status:response.status,data,response};
}
const resetLimits=()=>stack.pool.query('delete from webapp.rate_limits');
const yearsAgo=years=>{const d=new Date();d.setUTCFullYear(d.getUTCFullYear()-years);return d.toISOString().slice(0,10);};
before(async()=>{stack=await startStack({gatewayPort:0});base=`http://127.0.0.1:${stack.gateway.server.address().port}`;csrf=(await api('/session')).data.csrf;});
after(async()=>{if(stack)await stack.close();});
test('hybrid search obeys the shared external budget and falls back to the real local adapter path',async()=>{
  await resetLimits();let externalCalls=0;
  const searchFetch=async target=>{
    if(new URL(target).hostname==='api.search.brave.com'){externalCalls++;return Response.json({query:{original:'base',more_results_available:true},web:{results:[{title:'External protocol fixture',url:'https://example.test/external',description:'Test-only API transport'}]}});}
    return Response.json({query:'base',total:1,hits:[{title:'Local protocol fixture',url:'https://example.test/local',description:'Test-only index transport'}],has_more:false});
  };
  const app=await createGateway({...stack.config,searchProvider:'hybrid',braveKey:'test-only',braveApproved:true,braveDailyLimit:1,braveRps:100},{pool:stack.pool,searchFetch});const port=await listen(app.server);
  try{
    const first=await api('/search?q=base',undefined,'GET',{},`http://127.0.0.1:${port}`);
    assert.equal(first.status,200);assert.equal(first.data.total,null);assert.equal(first.data.has_more,true);assert.equal(first.data.hits.length,2);
    const second=await api('/search?q=base',undefined,'GET',{},`http://127.0.0.1:${port}`);
    assert.equal(second.status,200);assert.equal(second.data.degraded,true);assert.equal(second.data.hits.length,1);assert.equal(externalCalls,1);
  }finally{await app.close();await resetLimits();}
});
test('hosted signup creates the account and username@domain mailbox together and signs in',async()=>{
  const check=await api('/auth/username?name='+encodeURIComponent('Integration.User'));assert.equal(check.status,200);assert.equal(check.data.available,true);assert.equal(check.data.address,email);
  const result=await api('/auth/signup',profile());assert.equal(result.status,201);assert.deepEqual(result.data,{created:true,email,recoveryEmail:'sent'});
  assert.match(result.response.headers.get('set-cookie'),/HttpOnly/);assert.ok(!('token' in result.data));
  const account=(await stack.pool.query('select * from webapp.accounts where email=$1',[email])).rows[0];
  assert.match(account.password_hash,/^scrypt\$32768\$/);assert.ok(!account.password_hash.includes(password));assert.ok(account.verified_at);
  assert.equal(account.username,username);assert.equal(account.display_name,'테스트통합');assert.equal(account.gender,'custom');assert.equal(account.gender_custom,'Non-binary');assert.equal(account.pronoun,'other');
  assert.equal(account.recovery_phone,'+821012345678');assert.equal(account.recovery_email,recovery);assert.equal(account.recovery_email_verified_at,null);
  assert.equal((await stack.pool.query('select 1 from webapp.mail_connections where account_id=$1',[account.id])).rowCount,1);assert.ok(stack.jmap.mailboxFor(email));
  assert.equal(stack.smtp.messages.length,1);assert.equal(stack.smtp.messages[0].to,recovery);
  const current=await api('/session');csrf=current.data.csrf;assert.equal(current.data.user.email,email);assert.equal(current.data.user.username,username);assert.equal(current.data.user.recovery_email_verified,false);
  const taken=await api('/auth/username?name='+username);assert.equal(taken.data.available,false);assert.equal(taken.data.reason,'taken');assert.ok(taken.data.suggestions.length>0);
  const duplicate=await api('/auth/signup',profile());assert.equal(duplicate.status,409);assert.equal(duplicate.data.code,'USERNAME_TAKEN');
});
test('signup validation: reserved/format usernames, minimum age, consent and recovery domain create nothing',async()=>{
  await resetLimits();const before=(await stack.pool.query('select count(*)::int as n from webapp.accounts')).rows[0].n;
  assert.equal((await api('/auth/username?name=postmaster')).data.reason,'reserved');assert.equal((await api('/auth/username?name=ab')).data.reason,'format');
  assert.equal((await api('/auth/signup',profile({username:'postmaster'}))).data.code,'USERNAME_RESERVED');
  assert.equal((await api('/auth/signup',profile({username:'bad..name'}))).data.code,'INVALID_USERNAME');
  const young=await api('/auth/signup',profile({username:'young.person',birthDate:yearsAgo(13)}));assert.equal(young.status,403);assert.equal(young.data.code,'AGE_REQUIREMENT');
  assert.equal((await api('/auth/signup',profile({username:'no.consent',agree:false}))).data.code,'INVALID_INPUT');
  await resetLimits();
  assert.equal((await api('/auth/signup',profile({username:'same.domain',recoveryEmail:'x@openscanner.test'}))).data.code,'RECOVERY_EMAIL_SAME_DOMAIN');
  assert.equal((await api('/auth/signup',profile({username:'bad.phone',recoveryPhone:'12345'}))).data.code,'INVALID_PHONE');
  assert.equal((await api('/auth/signup',profile({username:'no.gender',gender:'custom',genderCustom:''}))).data.code,'INVALID_INPUT');
  assert.equal((await stack.pool.query('select count(*)::int as n from webapp.accounts')).rows[0].n,before);
});
test('mailbox provisioning failure rolls back the account; no provisioner fails closed',async()=>{
  await resetLimits();stack.provisioner.failNext=true;
  const failed=await api('/auth/signup',profile({username:'rollback.user'}));assert.equal(failed.status,502);assert.equal(failed.data.code,'MAIL_PROVISIONING_FAILED');
  assert.equal((await stack.pool.query('select 1 from webapp.accounts where username=$1',['rollback.user'])).rowCount,0);
  const closed=await createGateway(stack.config,{pool:stack.pool});const port=await listen(closed.server);
  try{
    const response=await api('/auth/signup',profile({username:'closed.user'}),'POST',{},`http://127.0.0.1:${port}`);assert.equal(response.status,503);assert.equal(response.data.code,'MAIL_PROVISIONING_UNAVAILABLE');
    const capability=await api('/auth/username',undefined,'GET',{},`http://127.0.0.1:${port}`);
    assert.equal(capability.data.signupEnabled,false,'the first signup screen must know provisioning is unavailable');
  }
  finally{await closed.close();}
  assert.equal((await stack.pool.query('select 1 from webapp.accounts where username=$1',['closed.user'])).rowCount,0);
});
test('recovery email verification is one-time; login accepts the username or the full address',async()=>{
  await resetLimits();const token=stack.smtp.tokenFor(recovery,'verify');assert.equal((await api('/auth/verify',{token})).status,200);assert.equal((await api('/auth/verify',{token})).status,400);
  assert.ok((await stack.pool.query('select recovery_email_verified_at from webapp.accounts where email=$1',[email])).rows[0].recovery_email_verified_at);
  for(const identifier of [username,email.toUpperCase()]){
    assert.equal((await api('/auth/logout',{})).status,200);
    const login=await api('/auth/login',{email:identifier,password});assert.equal(login.status,200);const cookie=login.response.headers.get('set-cookie');assert.match(cookie,/HttpOnly/);assert.match(cookie,/SameSite=Lax/);
  }
  const current=await api('/session');csrf=current.data.csrf;assert.equal(current.data.user.email,email);assert.equal(current.data.user.recovery_email_verified,true);
});
test('cross-origin mutations are rejected and a missing mail connection does not invent messages',async()=>{
  assert.equal((await api('/auth/logout',{},'POST',{origin:'https://attacker.invalid'})).status,403);
  const account=(await stack.pool.query('select id from webapp.accounts where email=$1',[email])).rows[0];
  const saved=(await stack.pool.query('delete from webapp.mail_connections where account_id=$1 returning token_ciphertext',[account.id])).rows[0];
  try{const result=await api('/mail/mailboxes');assert.equal(result.status,503);assert.equal(result.data.code,'MAIL_NOT_PROVISIONED');}
  finally{await stack.pool.query('insert into webapp.mail_connections(account_id,token_ciphertext) values($1,$2)',[account.id,saved.token_ciphertext]);}
});
test('session rotation revokes the previous cookie and state survives gateway restart',async()=>{
  const old=cookies;assert.equal((await api('/auth/refresh',{})).status,200);assert.notEqual(cookies,old);
  const stale=await fetch(base+'/api/v2/session',{headers:{cookie:old}});assert.equal((await stale.json()).user,null);
  const restarted=await createGateway(stack.config,{pool:stack.pool});const port=await listen(restarted.server);
  try{const response=await fetch(`http://127.0.0.1:${port}/api/v2/session`,{headers:{cookie:cookies}});assert.equal((await response.json()).user.email,email);}finally{await restarted.close();}
});
let provider,messageId;
test('JMAP protocol simulator: folder/read/update/draft/send and HTML isolation',async()=>{
  provider=stack.jmap.mailboxFor(email);messageId=stack.jmap.receive(provider.token,{});
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
test('an unresolved send reservation cannot be bypassed by replacing the draft',async()=>{
  const draft=await api('/mail/drafts',{to:'recipient@example.test',subject:'Uncertain submission',text:'Keep the original draft'});
  assert.equal(draft.status,200);
  const account=(await stack.pool.query('select id from webapp.accounts where email=$1',[email])).rows[0];
  // A reservation with no provider response represents a crash/timeout after
  // submission started. Its outcome must be reconciled, never sent as a new ID.
  await stack.pool.query('insert into webapp.mail_send_requests(account_id,email_id) values($1,$2)',[account.id,draft.data.id]);
  const replacement=await api('/mail/drafts',{id:draft.data.id,to:'recipient@example.test',subject:'Edited retry',text:'Keep the original draft'});
  assert.equal(replacement.status,409);
  assert.equal(replacement.data.code,'SEND_STATUS_UNKNOWN');
  const original=await api('/mail/messages/'+draft.data.id);
  assert.equal(original.status,200);
  assert.equal(original.data.subject,'Uncertain submission');
  assert.equal(original.data.submissionStatus,'uncertain');
  const retry=await api('/mail/send',{id:draft.data.id});
  assert.equal(retry.status,409);
});
test('draft replacement and submission share one account-scoped lock and release it',async()=>{
  const draft=await api('/mail/drafts',{to:'recipient@example.test',subject:'Concurrent operation',text:'Preserve one draft'});
  const account=(await stack.pool.query('select id from webapp.accounts where email=$1',[email])).rows[0];
  await withMessageLock(stack.pool,account.id,draft.data.id,async()=>{
    const send=await api('/mail/send',{id:draft.data.id});
    assert.equal(send.status,409);assert.equal(send.data.code,'MAIL_OPERATION_BUSY');
    const edit=await api('/mail/drafts',{id:draft.data.id,to:'recipient@example.test',subject:'Competing edit'});
    assert.equal(edit.status,409);assert.equal(edit.data.code,'MAIL_OPERATION_BUSY');
    assert.equal((await stack.pool.query('select 1 from webapp.mail_send_requests where account_id=$1 and email_id=$2',[account.id,draft.data.id])).rowCount,0);
  });
  const edit=await api('/mail/drafts',{id:draft.data.id,to:'recipient@example.test',subject:'Edit after lock release'});
  assert.equal(edit.status,200);
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
test('reset goes to the verified recovery email, invalidates sessions and rejects weak passwords',async()=>{
  await resetLimits();assert.equal((await api('/auth/forgot-password',{email:username})).status,202);const token=stack.smtp.tokenFor(recovery,'reset');
  assert.equal((await api('/auth/reset-password',{token,password:'short'})).status,400);
  const next=randomBytes(18).toString('base64url');assert.equal((await api('/auth/reset-password',{token,password:next})).status,200);
  const current=await api('/session');csrf=current.data.csrf;assert.equal(current.data.user,null);
  assert.equal((await api('/auth/login',{email,password})).status,401);
  assert.equal((await api('/auth/login',{email:username,password:next})).status,200);password=next;
  csrf=(await api('/session')).data.csrf;
});
test('generic account responses, login throttling and redirect rejection',async()=>{
  await resetLimits();const sentBefore=stack.smtp.messages.length;
  assert.deepEqual((await api('/auth/forgot-password',{email:'missing@example.test'})).data,{accepted:true});
  assert.deepEqual((await api('/auth/forgot-password',{email:'nobody.here'})).data,{accepted:true});assert.equal(stack.smtp.messages.length,sentBefore);
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
test('responses carry baseline security headers and production adds HSTS',async()=>{
  const headers=(await api('/session')).response.headers;
  assert.equal(headers.get('content-security-policy'),"default-src 'none'; frame-ancestors 'none'");
  assert.equal(headers.get('x-frame-options'),'DENY');
  assert.equal(headers.get('cross-origin-resource-policy'),'same-origin');
  assert.equal(headers.get('permissions-policy'),'camera=(), microphone=(), geolocation=(), payment=()');
  assert.equal(headers.get('strict-transport-security'),null);
  const secret=randomBytes(32).toString('hex');const production=await createGateway({...stack.config,production:true,origin:'https://portal.example.test',ingressSecret:secret},{pool:stack.pool});const port=await listen(production.server);
  try{
    const res=await fetch(`http://127.0.0.1:${port}/api/v2/session`,{headers:{'x-gateway-key':secret}});
    assert.equal(res.status,200);assert.equal(res.headers.get('strict-transport-security'),'max-age=31536000; includeSubDomains');
  }finally{await production.close();}
});
test('admin endpoints require the operator flag and never expose credentials',async()=>{
  await resetLimits();
  assert.equal((await fetch(base+'/api/v2/admin/overview')).status,401);
  const denied=await api('/admin/overview');assert.equal(denied.status,403);assert.equal(denied.data.code,'FORBIDDEN');
  await stack.pool.query('update webapp.accounts set is_admin=true where email=$1',[email]);
  const overview=await api('/admin/overview');assert.equal(overview.status,200);
  for (const key of ['accounts','sessions','mailboxes','signups_24h','recent']) assert.ok(key in overview.data);
  assert.doesNotMatch(JSON.stringify(overview.data),/password_hash|token_hash|token_ciphertext/);
  const list=await api('/admin/accounts?q=integration.user');assert.equal(list.status,200);
  assert.equal(list.data.total,1);assert.equal(list.data.accounts[0].email,email);
  assert.ok(!('password_hash' in list.data.accounts[0]));
  assert.equal((await api('/admin/accounts?page=101')).status,400);
  assert.equal((await api('/admin/nope')).status,404);
  await stack.pool.query('update webapp.accounts set is_admin=false where email=$1',[email]);
});
test('account deletion removes the hosted mailbox through the provisioner',async()=>{
  await resetLimits();const secret=randomBytes(18).toString('base64url');const name='delete.me.now';const address=`${name}@openscanner.test`;
  assert.equal((await api('/auth/signup',profile({username:name,password:secret,recoveryEmail:undefined,recoveryPhone:undefined,gender:'unspecified'}))).status,201);
  csrf=(await api('/session')).data.csrf;
  assert.equal((await api('/account',{password:secret},'DELETE')).status,200);
  assert.ok(stack.provisioner.deleted.includes(address));assert.equal(stack.jmap.mailboxFor(address),null);
  assert.equal((await stack.pool.query('select 1 from webapp.accounts where email=$1',[address])).rowCount,0);
});
