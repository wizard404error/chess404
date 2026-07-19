const https = require('https');

const GATEWAY = 'gateway-production-1340.up.railway.app';
const MATCH_SVC = 'match-service-production-9f8b.up.railway.app';
const WEB = 'web-production-ddc27.up.railway.app';

async function fetch(opts) {
  return new Promise((resolve) => {
    const start = Date.now();
    const req = https.get({ hostname: opts.host, path: opts.path, port: 443, rejectUnauthorized: false }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({ status: res.statusCode, dur: Date.now() - start, body: data.substring(0, 200) });
      });
    });
    req.on('error', (err) => resolve({ status: 0, dur: Date.now() - start, body: err.message }));
    req.setTimeout(10000, () => { req.destroy(); resolve({ status: 0, dur: 10000, body: 'timeout' }); });
  });
}

async function main() {
  // Test each endpoint sequentially first to see status codes
  const checks = [
    { host: GATEWAY, path: '/readyz', name: 'gateway readyz' },
    { host: GATEWAY, path: '/api/system/status', name: 'gateway status' },
    { host: MATCH_SVC, path: '/readyz', name: 'match readyz' },
    { host: MATCH_SVC, path: '/api/system/status', name: 'match status' },
    { host: MATCH_SVC, path: '/api/matches', name: 'match list' },
  ];

  for (const ep of checks) {
    const r = await fetch(ep);
    console.log(`${ep.name}: status=${r.status} dur=${r.dur}ms body=${r.body}`);
  }

  // Then hammer gateway readyz with 50 concurrent to see behavior
  console.log('\nHammering gateway readyz with 50 concurrent...');
  const results = [];
  const start = Date.now();
  const workers = [];
  for (let i = 0; i < 50; i++) {
    workers.push(fetch({ host: GATEWAY, path: '/readyz' }));
  }
  const responses = await Promise.all(workers);
  const dur = Date.now() - start;
  const statusMap = {};
  for (const r of responses) {
    statusMap[r.status] = (statusMap[r.status] || 0) + 1;
  }
  console.log(`Total time: ${dur}ms`);
  for (const [code, count] of Object.entries(statusMap)) {
    console.log(`  Status ${code}: ${count} responses`);
  }
}

main().catch(console.error);
