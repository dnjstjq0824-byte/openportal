import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { seed } from '../scripts/seed.mjs';

test('seed rejects mapping errors and incomplete or failed bulk acknowledgements', async () => {
  let createStatus = 200;
  let createBody = {};
  let bulk = { errors: false, items: Array.from({length:3},()=>({index:{status:201}})) };
  let bulkCalls = 0;
  const server = http.createServer((req,res)=>{
    req.resume();
    res.setHeader('Content-Type','application/json');
    if(req.url.startsWith('/_bulk')) {bulkCalls++;res.end(JSON.stringify(bulk));}
    else {res.statusCode=createStatus;res.end(JSON.stringify(createBody));}
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const baseURL=`http://127.0.0.1:${server.address().port}`;
  try {
    await seed({baseURL});
    createStatus=400;
    createBody={error:{type:'mapper_parsing_exception'}};
    const before=bulkCalls;
    await assert.rejects(seed({baseURL}), /Failed to create index/);
    assert.equal(bulkCalls,before);
    createBody={error:{type:'resource_already_exists_exception'}};
    await seed({baseURL});
    for(const invalid of [
      {errors:false}, {errors:false,items:[]},
      {errors:false,items:[{index:{status:201}},{index:{status:201}},{index:{status:400}}]},
      {errors:false,items:[{index:{status:201}},{index:{status:201}},{index:{status:201,error:{type:'failure'}}}]},
      {errors:true,items:[]},
    ]) {bulk=invalid;await assert.rejects(seed({baseURL}), /Bulk indexing had errors/);}
  } finally {await new Promise(resolve=>server.close(resolve));}
});
