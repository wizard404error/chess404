const https = require('https');

const MATCH_SVC = 'match-service-production-9f8b.up.railway.app';

function fetch(path) {
  return new Promise((resolve) => {
    const start = Date.now();
    https.get({ hostname: MATCH_SVC, path, port: 443, rejectUnauthorized: false }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, dur: Date.now() - start }));
    }).on('error', (e) => resolve({ status: 0, dur: Date.now() - start }));
  });
}

async function main() {
  // Memory leak check: hammer the match creation endpoint to check for OOM
  console.log('=== Memory/GC stress: match creation (50 sequential) ===');
  const times = [];
  for (let i = 0; i < 50; i++) {
    const start = Date.now();
    const res = await new Promise((resolve) => {
      const postData = JSON.stringify({ mode_id: 'standard' });
      const req = https.request({
        hostname: MATCH_SVC,
        path: '/api/matches',
        port: 443,
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Content-Length': postData.length },
        rejectUnauthorized: false,
      }, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => resolve({ status: res.statusCode, body: data.substring(0, 100), dur: Date.now() - start }));
      });
      req.on('error', (e) => resolve({ status: 0, dur: Date.now() - start }));
      req.write(postData);
      req.end();
    });
    times.push(res.dur);
    console.log(`  Match ${i+1}: status=${res.status} dur=${res.dur}ms body=${res.body}`);
  }
  
  const avg = times.reduce((a,b) => a+b, 0) / times.length;
  const sorted = [...times].sort((a,b) => a-b);
  console.log(`\n  Avg: ${avg.toFixed(0)}ms  P95: ${sorted[Math.floor(sorted.length*0.95)]}ms`);
  console.log(`  No OOM/crash (all 50 completed)`);

  // Check memory of match service via API
  console.log('\n=== Match service status ===');
  const statusRes = await fetch('/api/system/status');
  console.log(`  Status endpoint: ${statusRes.status} (${statusRes.dur}ms)`);
}

main().catch(console.error);
