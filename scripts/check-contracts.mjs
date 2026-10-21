import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
import YAML from 'yaml';
import Ajv from 'ajv/dist/2020.js';

const apiPath='docs/openapi.yaml', eventPath='docs/events/index-event.v1.schema.json';
const api=YAML.parse(readFileSync(apiPath,'utf8'));
const event=JSON.parse(readFileSync(eventPath,'utf8'));
const validate=new Ajv().compile(event);
const valid={event_id:'event-1',document_id:'doc-1',operation:'upsert',version:1,occurred_at:'2026-10-20T00:00:00Z'};
assert(validate(valid));
for(const invalid of [{...valid,version:2},{...valid,operation:'crawl'},{...valid,query:'private search'},{...valid,occurred_at:'yesterday'}])
  assert(!validate(invalid));
// Existing contracts are immutable. Additive versions/endpoints remain allowed.
export function compatible(previous,current) {
  for(const [path,methods] of Object.entries(previous.paths))
    for(const [method,contract] of Object.entries(methods))
      assert.deepEqual(current.paths?.[path]?.[method],contract,`changed existing API ${method} ${path}`);
  for(const [name,schema] of Object.entries(previous.components.schemas))
    assert.deepEqual(current.components?.schemas?.[name],schema,`changed schema ${name}`);
}
compatible(api,structuredClone(api));
const broken=structuredClone(api); delete broken.paths['/api/v1/search'];
assert.throws(()=>compatible(api,broken));
if(process.env.CONTRACT_BASE_REF) {
  const old=path=>execFileSync('git',['show',`${process.env.CONTRACT_BASE_REF}:${path}`],{encoding:'utf8'});
  compatible(YAML.parse(old(apiPath)),api);
  assert.deepEqual(JSON.parse(old(eventPath)),event,'event v1 changed; introduce v2');
  console.log('PASS compatibility against',process.env.CONTRACT_BASE_REF);
}
console.log('PASS event valid/invalid fixtures and breaking detector negative control');
