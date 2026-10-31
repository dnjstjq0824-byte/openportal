import {test} from 'node:test';
import assert from 'node:assert/strict';
import worker from '../apps/web/public/_worker.js';
const request=(path='/api/v1/search?q=test',method='GET',host='openportal.test')=>new Request(`https://${host}${path}`,{method});
test('missing backend returns JSON 503 on every host, never fixtures or SPA HTML',async()=>{
  for(const host of ['openportal.test','openportal.pages.dev']){const response=await worker.fetch(request(undefined,'GET',host),{});assert.equal(response.status,503);assert.equal((await response.json()).code,'SEARCH_BACKEND_UNAVAILABLE');}
});
test('HTML, invalid JSON and network failures become sanitized JSON',async()=>{
  for(const fetch of [async()=>new Response('<!doctype html>'),async()=>new Response('{',{headers:{'content-type':'application/json'}}),async()=>{throw Error('secret upstream');}]){const response=await worker.fetch(request(),{SEARCH_API:{fetch}});assert.equal(response.status,502);assert.doesNotMatch(await response.text(),/secret|doctype|SyntaxError/);}
});
test('search contract, query forwarding and request correlation',async()=>{
  const response=await worker.fetch(request(),{SEARCH_API:{async fetch(req){assert.equal(new URL(req.url).searchParams.get('q'),'test');assert.ok(req.headers.get('x-request-id'));return Response.json({query:'test',total:0,hits:[]});}}});assert.equal(response.status,200);assert.deepEqual((await response.json()).hits,[]);
});
test('privileged and old prototype endpoints are denied before upstream',async()=>{
  assert.equal((await worker.fetch(request('/api/v1/search','POST'),{})).status,405);
  for(const path of ['/api/v1/auth/register','/api/v1/auth/verify','/api/v1/admin','/api/v1/organizations/x','/api/v1/superplatform/mail','/api/v1/vertical/search'])assert.equal((await worker.fetch(request(path),{})).status,404);
});
test('unavailable gateway is explicit; authenticated cookies survive a trusted service binding',async()=>{
  assert.equal((await worker.fetch(request('/api/v2/session'),{})).status,503);
  const req=new Request('https://openportal.pages.dev/api/v2/session',{headers:{cookie:'op_session=test',authorization:'Bearer discard'}});
  const response=await worker.fetch(req,{WEB_GATEWAY:{async fetch(forwarded){assert.equal(forwarded.headers.get('cookie'),'op_session=test');assert.equal(forwarded.headers.get('authorization'),null);return Response.json({user:null},{headers:{'Set-Cookie':'op_csrf=opaque; HttpOnly; Secure; SameSite=Lax'}});}}});assert.equal(response.status,200);assert.match(response.headers.get('set-cookie'),/HttpOnly/);
});
test('upstream request limits do not become success responses',async()=>{const response=await worker.fetch(request(),{SEARCH_API:{fetch:async()=>Response.json({error:'secret'},{status:429})}});assert.equal(response.status,429);assert.equal((await response.json()).code,'RATE_LIMITED');});
test('sitemap includes only public locale homes and robots excludes private paths',async()=>{
  const sitemap=await worker.fetch(request('/sitemap.xml'),{});const xml=await sitemap.text();assert.match(xml,/https:\/\/openportal.test\/th/);assert.doesNotMatch(xml,/\/mail|\/search|internal/);
  const robots=await worker.fetch(request('/robots.txt'),{});assert.match(await robots.text(),/Disallow: \/\*\/mail/);
});
test('private SPA pages receive noindex response headers',async()=>{const response=await worker.fetch(request('/ko/mail'),{ASSETS:{fetch:async()=>new Response('home',{headers:{'content-type':'text/html'}})}});assert.equal(response.headers.get('x-robots-tag'),'noindex,nofollow');assert.equal(await response.text(),'home');});
test('gateway POST retains CSRF and cookies but replaces spoofed ingress identity',async()=>{
  const req=new Request('https://openportal.pages.dev/api/v2/auth/login',{method:'POST',headers:{'content-type':'application/json',origin:'https://openportal.pages.dev','x-csrf-token':'opaque',cookie:'op_csrf=opaque','x-gateway-key':'attacker','x-gateway-client-ip':'spoofed','cf-connecting-ip':'192.0.2.1'},body:'{}'});
  const response=await worker.fetch(req,{GATEWAY_SHARED_SECRET:'operator-secret',WEB_GATEWAY:{async fetch(forwarded){assert.equal(forwarded.headers.get('x-gateway-key'),'operator-secret');assert.equal(forwarded.headers.get('x-gateway-client-ip'),'192.0.2.1');assert.equal(forwarded.headers.get('x-csrf-token'),'opaque');assert.equal(await forwarded.text(),'{}');return Response.json({authenticated:true});}}});assert.equal(response.status,200);
});
