import { readFileSync, writeFileSync } from 'node:fs';
const permitted=new Set(['MIT','ISC','Apache-2.0','BSD-2-Clause','BSD-3-Clause','0BSD','Python-2.0','CC0-1.0','CC-BY-4.0','(MIT OR CC0-1.0)','(MIT OR Apache-2.0)']);
const inventory=[];
for(const path of ['package-lock.json','apps/web/package-lock.json']){
  const lock=JSON.parse(readFileSync(path));
  for(const [name,pkg] of Object.entries(lock.packages)){
    if(!name) continue;
    inventory.push({lock:path,name,version:pkg.version,license:pkg.license||'UNKNOWN'});
  }
}
writeFileSync('docs/evidence/npm-licenses.json',JSON.stringify(inventory,null,2));
const unknown=inventory.filter(p=>!permitted.has(p.license));
if(unknown.length) { console.error(unknown); process.exitCode=1; }
else console.log(`PASS declared npm licenses: ${inventory.length} entries; legal approval remains separate`);
