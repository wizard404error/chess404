const https = require('https');

const GATEWAY = 'gateway-production-1340.up.railway.app';

function fetch(path) {
  return new Promise((resolve) => {
    const start = Date.now();
    const req = https.get({ hostname: GATEWAY, path, port: 443, rejectUnauthorized: false }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, dur: Date.now() - start, body: data.substring(0, 200) }));
    });
    req.on('error', (err) => resolve({ status: 0, dur: Date.now() - start, body: err.message }));
    req.setTimeout(10000, () => { req.destroy(); resolve({ status: 0, dur: 10000, body: 'timeout' }); });
  });
}

async function main() {
  // First, wait 1s to let any previous rate limit windows expire
  await new Promise(r => setTimeout(r, 1000));
  
  for (let i = 0; i < 5; i++) {
    const r = await fetch('/readyz');
    console.log(`Request ${i+1}: status=${r.status} dur=${r.dur}ms body=${r.body}`);
    await new Promise(r => setTimeout(r, 100));
  }
}

main().catch(console.error);
