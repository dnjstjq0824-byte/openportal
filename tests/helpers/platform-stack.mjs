// Test-only infrastructure. SMTP is a real loopback protocol server; JMAP and
// ClamAV below are protocol simulators, never production service implementations.
import EmbeddedPostgres from 'embedded-postgres';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { randomBytes } from 'node:crypto';
import net from 'node:net';
import http from 'node:http';
import pg from 'pg';
import { migrate } from '../../scripts/migrate.mjs';
import { configuration,createGateway } from '../../gateway/server.mjs';
import { seal } from '../../gateway/security.mjs';

export async function listen(server,port=0){await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(port,'127.0.0.1',resolve);});return server.address().port;}
export async function smtpFixture(){
  const messages=[];const sockets=new Set();
  const server=net.createServer(socket=>{sockets.add(socket);socket.on('close',()=>sockets.delete(socket));socket.setEncoding('utf8');let buffer='',inData=false,lines=[],to='';socket.write('220 localhost test SMTP\r\n');socket.on('data',chunk=>{buffer+=chunk;let index;while((index=buffer.indexOf('\r\n'))>=0){const line=buffer.slice(0,index);buffer=buffer.slice(index+2);if(inData){if(line==='.') {messages.push({to,raw:lines.join('\r\n')});inData=false;socket.write('250 queued\r\n');}else lines.push(line);continue;}if(/^EHLO|^HELO/.test(line))socket.write('250-localhost\r\n250 SIZE 1000000\r\n');else if(/^RCPT TO:/i.test(line)){to=line.slice(8).replace(/[<>]/g,'').trim();socket.write('250 OK\r\n');}else if(/^DATA/.test(line)){inData=true;lines=[];socket.write('354 send data\r\n');}else if(/^QUIT/.test(line)){socket.end('221 bye\r\n');}else socket.write('250 OK\r\n');}});});
  const port=await listen(server);return {port,messages,close:async()=>{for(const socket of sockets)socket.destroy();await new Promise(resolve=>server.close(resolve));},tokenFor(email,purpose){const msg=[...messages].reverse().find(x=>x.to.toLowerCase()===email.toLowerCase()&&x.raw.includes(purpose==='verify'?'verify-email':'reset-password'));if(!msg)throw Error('No SMTP verification message received');const decoded=msg.raw.replace(/=\r\n/g,'').replace(/=3D/g,'=');const token=decoded.match(/#token=([\w-]+)/)?.[1];if(!token)throw Error('No token in SMTP message');return token;}};
}
export async function jmapFixture(){
  const accounts=new Map();const blobs=new Map();let counter=0;let calls=0;
  const server=http.createServer(async(req,res)=>{const token=String(req.headers.authorization||'').replace('Bearer ','');const owner=accounts.get(token);const output=body=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify(body));};if(!owner){res.statusCode=401;return output({});}calls++;const base=`http://127.0.0.1:${server.address().port}`;
    if(req.url==='/session')return output({primaryAccounts:{'urn:ietf:params:jmap:mail':owner.id},apiUrl:base+'/api',uploadUrl:base+'/upload/{accountId}',downloadUrl:base+'/download/{accountId}/{blobId}/{name}?type={type}'});
    if(req.url.startsWith('/upload/')){const chunks=[];for await(const chunk of req)chunks.push(chunk);const blobId=`blob-${++counter}`;blobs.set(blobId,{account:owner.id,bytes:Buffer.concat(chunks)});return output({accountId:owner.id,blobId,type:req.headers['content-type'],size:Buffer.concat(chunks).length});}
    if(req.url.startsWith('/download/')){const blob=blobs.get(req.url.split('/')[3]);if(!blob||blob.account!==owner.id){res.statusCode=404;return output({});}return res.end(blob.bytes);}
    const chunks=[];for await(const chunk of req)chunks.push(chunk);const request=JSON.parse(Buffer.concat(chunks));const [method,args,tag]=request.methodCalls[0];let data={accountId:owner.id};
    if(args.accountId!==owner.id){res.statusCode=403;return output({});}
    if(method==='Identity/get')data.list=[{id:'identity',email:owner.email}];
    else if(method==='Mailbox/get')data.list=[...['inbox','sent','drafts','trash','junk','archive'].map(role=>({id:`${owner.id}-${role}`,role,name:role,unreadEmails:0})),...(owner.folders||[])];
    else if(method==='Mailbox/set'){
      owner.folders ||= [];data.created={};data.updated={};data.destroyed=[];
      for(const [key,value]of Object.entries(args.create||{})){const id=`folder-${++counter}`;owner.folders.push({id,name:value.name,role:null});data.created[key]={id};}
      for(const [id,value]of Object.entries(args.update||{})){const box=owner.folders.find(x=>x.id===id);if(box){box.name=value.name;data.updated[id]=null;}}
      for(const id of args.destroy||[]){owner.folders=owner.folders.filter(x=>x.id!==id);data.destroyed.push(id);}
    }
    else if(method==='Email/query'){const messages=[...owner.messages.values()].filter(x=>x.mailboxIds[args.filter.inMailbox]&&(!args.filter.text||`${x.subject} ${x.preview}`.includes(args.filter.text)));data.ids=messages.slice(args.position,args.position+args.limit).map(x=>x.id);data.total=messages.length;}
    else if(method==='Email/get')data.list=args.ids.map(id=>owner.messages.get(id)).filter(Boolean);
    else if(method==='Email/set'){
      data.created={};data.updated={};data.notUpdated={};data.destroyed=[];
      for(const [key,value] of Object.entries(args.create||{})){const id=`message-${++counter}`;const entry={...value,id,receivedAt:new Date().toISOString(),preview:value.bodyValues?.text?.value||''};owner.messages.set(id,entry);data.created[key]={id};}
      for(const [id,patch] of Object.entries(args.update||{})){const entry=owner.messages.get(id);if(!entry){data.notUpdated[id]={type:'notFound'};continue;}for(const [key,value]of Object.entries(patch)){if(key.startsWith('keywords/')){if(value===null)delete entry.keywords[key.slice(9)];else entry.keywords[key.slice(9)]=value;}else entry[key]=value;}data.updated[id]=null;}
      for(const id of args.destroy||[]){if(owner.messages.delete(id))data.destroyed.push(id);}
    }else if(method==='EmailSubmission/set'){
      const message=owner.messages.get(args.create.submission.emailId);if(!message){data.notCreated={submission:{type:'notFound'}};}else{message.mailboxIds={[`${owner.id}-sent`]:true};delete message.keywords.$draft;data.created={submission:{id:`submission-${++counter}`}};}
    }else return output({methodResponses:[['error',{type:'unknownMethod'},tag]]});
    output({methodResponses:[[method,data,tag]]});
  });
  const port=await listen(server);return {url:`http://127.0.0.1:${port}/session`,get calls(){return calls;},accounts,provision(email){const token=randomBytes(24).toString('hex');const id=`account-${++counter}`;accounts.set(token,{id,email,messages:new Map()});return {token,id};},receive(token,message){const account=accounts.get(token);const id=`message-${++counter}`;account.messages.set(id,{id,from:[{email:'sender@example.test'}],to:[{email:account.email}],subject:'Protocol fixture',receivedAt:new Date().toISOString(),mailboxIds:{[`${account.id}-inbox`]:true},keywords:{},textBody:[{partId:'text',type:'text/plain'}],htmlBody:[{partId:'html',type:'text/html'}],bodyValues:{text:{value:'Protocol simulator message'},html:{value:'<p>Hello</p><img src="https://tracker.invalid/pixel"><script>alert(1)</script>'}},attachments:[],...message});return id;},close:()=>new Promise(resolve=>server.close(resolve))};
}
export async function startStack({origin='http://127.0.0.1:5173',gatewayPort=8090,pgPort=55436,searchURL='http://127.0.0.1:8080'}={}){
  const directory=resolve('work',`platform-test-${Date.now()}`);await mkdir(directory,{recursive:true});const password=randomBytes(24).toString('hex');
  const postgres=new EmbeddedPostgres({databaseDir:directory,user:'postgres',password,port:pgPort,persistent:true,initdbFlags:['--encoding=UTF8','--locale=C'],postgresFlags:['-c','listen_addresses=127.0.0.1','-c','log_statement=none','-c','log_min_error_statement=panic']});
  await postgres.initialise();await postgres.start();const pool=new pg.Pool({host:'127.0.0.1',port:pgPort,user:'postgres',password,database:'postgres'});const client=await pool.connect();try{await migrate(client);}finally{client.release();}
  const smtp=await smtpFixture(),jmap=await jmapFixture();
  const scanner=net.createServer(socket=>{socket.on('data',chunk=>{if(chunk.subarray(-4).equals(Buffer.alloc(4)))socket.end('stream: OK\0');});});const clamPort=await listen(scanner);
  const config=configuration({NODE_ENV:'test',PUBLIC_ORIGIN:origin,SMTP_HOST:'127.0.0.1',SMTP_PORT:String(smtp.port),SMTP_FROM:'OpenPortal <no-reply@example.test>',MAIL_JMAP_URL:jmap.url,MAIL_ENCRYPTION_KEY:randomBytes(32).toString('base64'),CLAMAV_HOST:'127.0.0.1',CLAMAV_PORT:String(clamPort)});
  config.searchURL=searchURL;const gateway=await createGateway(config,{pool});await listen(gateway.server,gatewayPort);
  return {pool,smtp,jmap,gateway,config,async provision(email){const account=(await pool.query('select id from webapp.accounts where email=$1',[email])).rows[0];const provider=jmap.provision(email);await pool.query('insert into webapp.mail_connections(account_id,token_ciphertext) values($1,$2)',[account.id,seal(provider.token,config.mailKey)]);return provider;},async close(){await gateway.close();await smtp.close();await jmap.close();await new Promise(resolve=>scanner.close(resolve));await pool.end();await postgres.stop();}};
}
