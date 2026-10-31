import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import ts from '../apps/web/node_modules/typescript/lib/typescript.js';
import {emailText} from '../gateway/translations.mjs';
test('all supported product and error dictionaries have exactly the same nonempty keys',async()=>{
  const source=await readFile('apps/web/src/i18n.ts','utf8');const js=ts.transpile(source,{module:ts.ModuleKind.ESNext});
  const {messages,errors,locales,folderLabels}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
  for(const group of [messages,errors,emailText,folderLabels])for(const locale of locales){assert.deepEqual(Object.keys(group[locale]).sort(),Object.keys(group.en).sort());for(const value of Object.values(group[locale]))assert.equal(typeof value==='string'&&value.trim().length>0,true);}
});
