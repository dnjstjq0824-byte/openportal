import { spawn } from 'node:child_process';
import http from 'node:http';
import fs from 'node:fs';

const osBin = 'C:\\Users\\dnjst\\Documents\\Codex\\2026-10-20\\openportal-search-agents-md-codex-master\\work\\search-runtime\\opensearch-2.19.0\\bin\\opensearch.bat';
const outLog = fs.openSync('work/os-stdout.log', 'a');
const errLog = fs.openSync('work/os-stderr.log', 'a');

console.log('Spawning OpenSearch 2.19.0...');
const child = spawn(osBin, [], {
  detached: true,
  windowsHide: true,
  stdio: ['ignore', outLog, errLog],
  env: { ...process.env, OPENSEARCH_JAVA_OPTS: '-Xms512m -Xmx512m' },
  shell: true
});
child.unref();
console.log('OpenSearch process spawned.');

async function checkHealth() {
  for (let i = 0; i < 40; i++) {
    await new Promise(r => setTimeout(r, 2000));
    try {
      const data = await new Promise((resolve, reject) => {
        const req = http.get('http://127.0.0.1:9200/_cluster/health?wait_for_status=yellow&timeout=1s', (res) => {
          let buf = '';
          res.on('data', chunk => buf += chunk);
          res.on('end', () => {
            if (res.statusCode === 200) resolve(buf);
            else reject(new Error('Status ' + res.statusCode));
          });
        });
        req.on('error', reject);
        req.setTimeout(2000, () => req.destroy(new Error('timeout')));
      });
      console.log('OpenSearch is ready: ' + data);
      return true;
    } catch (e) {
      process.stdout.write('.');
    }
  }
  throw new Error('OpenSearch did not become healthy in time');
}

checkHealth().then(() => {
  console.log('\nOpenSearch started successfully.');
  process.exit(0);
}).catch(err => {
  console.error('\n' + err.message);
  process.exit(1);
});

