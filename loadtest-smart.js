const http = require('http');
const https = require('https');

const GATEWAY = 'gateway-production-1340.up.railway.app';
const MATCH_SVC = 'match-service-production-9f8b.up.railway.app';
const WEB = 'web-production-ddc27.up.railway.app';

// Stay within 60 req/min per endpoint to avoid rate limiting
const RATE_LIMIT_BUDGET_PER_ENDPOINT = 60;
const CONCURRENT = 200;

function fetch(opts) {
  return new Promise((resolve) => {
    const start = Date.now();
    const req = https.get({ hostname: opts.host, path: opts.path, port: 443, rejectUnauthorized: false }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, dur: Date.now() - start, err: null }));
    });
    req.on('error', (err) => resolve({ status: 0, dur: Date.now() - start, err: err.message }));
    req.setTimeout(10000, () => { req.destroy(); resolve({ status: 0, dur: 10000, err: 'timeout' }); });
  });
}

async function main() {
  // Phase 1: Sequential warmup (1 request each endpoint)
  console.log('=== Phase 1: Warmup ===');
  const warmup = [
    { host: GATEWAY, path: '/readyz', name: 'gateway readyz' },
    { host: MATCH_SVC, path: '/readyz', name: 'match readyz' },
    { host: WEB, path: '/', name: 'web /' },
  ];
  for (const ep of warmup) {
    const r = await fetch(ep);
    console.log(`  ${ep.name}: ${r.status} (${r.dur}ms)`);
  }

  // Phase 2: Burst test — fire all 200 concurrent users at the web frontend (no rate limit issues)
  console.log('\n=== Phase 2: 200 concurrent web frontend requests ===');
  const burstStart = Date.now();
  const burst = [];
  for (let i = 0; i < CONCURRENT; i++) {
    burst.push(fetch({ host: WEB, path: '/', name: 'web /' }));
  }
  const burstResults = await Promise.all(burst);
  const burstDur = Date.now() - burstStart;
  const burstOk = burstResults.filter(r => r.status >= 200 && r.status < 400).length;
  const times = burstResults.map(r => r.dur).sort((a,b) => a-b);
  const avg = times.reduce((a,b) => a+b, 0) / times.length;
  const p95 = times[Math.floor(times.length * 0.95)];
  console.log(`  Total time: ${burstDur}ms`);
  console.log(`  OK: ${burstOk}/${CONCURRENT}`);
  console.log(`  Avg: ${avg.toFixed(0)}ms  P95: ${p95}ms`);
  console.log(`  Throughput: ${(CONCURRENT / (burstDur/1000)).toFixed(1)} req/s`);

  // Phase 3: Sustained load at 10 req/s for 30s (under rate limit)
  console.log('\n=== Phase 3: Sustained load (10 req/s, 30s) ===');
  const results = { 'gateway readyz': [], 'match readyz': [], 'web /': [] };
  const endpoints = [
    { host: GATEWAY, path: '/readyz', name: 'gateway readyz' },
    { host: MATCH_SVC, path: '/readyz', name: 'match readyz' },
    { host: WEB, path: '/', name: 'web /' },
  ];
  const sustainStart = Date.now();
  let sustained = 0;
  while (Date.now() - sustainStart < 30000) {
    const batch = endpoints.map(ep => fetch(ep));
    const batchRes = await Promise.all(batch);
    sustained += batchRes.length;
    for (let i = 0; i < batchRes.length; i++) {
      results[endpoints[i].name].push(batchRes[i]);
    }
    await new Promise(r => setTimeout(r, 300)); // ~10 req/s
  }
  const sustainDur = (Date.now() - sustainStart) / 1000;
  console.log(`  Duration: ${sustainDur.toFixed(1)}s`);
  console.log(`  Total requests: ${sustained}`);
  console.log(`  Avg throughput: ${(sustained / sustainDur).toFixed(1)} req/s`);
  
  for (const [name, res] of Object.entries(results)) {
    const ok = res.filter(r => r.status >= 200 && r.status < 400).length;
    const fail = res.filter(r => r.status === 429).length;
    const err = res.filter(r => r.status === 0).length;
    const times = res.map(r => r.dur).sort((a,b) => a-b);
    const avg = times.length > 0 ? (times.reduce((a,b) => a+b, 0) / times.length).toFixed(0) : 0;
    const p95 = times.length > 0 ? times[Math.floor(times.length * 0.95)] : 0;
    const p99 = times.length > 0 ? times[Math.floor(times.length * 0.99)] : 0;
    console.log(`  ${name}: ok=${ok} rate-limited=${fail} err=${err} avg=${avg}ms p95=${p95}ms p99=${p99}ms`);
  }
}

main().catch(console.error);
