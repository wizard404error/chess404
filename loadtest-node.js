const http = require('http');
const https = require('https');

const GATEWAY = 'gateway-production-1340.up.railway.app';
const MATCH_SVC = 'match-service-production-9f8b.up.railway.app';
const WEB = 'web-production-ddc27.up.railway.app';

const endpoints = [
  { host: GATEWAY, path: '/', name: 'gateway /' },
  { host: GATEWAY, path: '/healthz', name: 'gateway healthz' },
  { host: GATEWAY, path: '/readyz', name: 'gateway readyz' },
  { host: GATEWAY, path: '/api/system/status', name: 'gateway status' },
  { host: MATCH_SVC, path: '/', name: 'match /' },
  { host: MATCH_SVC, path: '/healthz', name: 'match healthz' },
  { host: MATCH_SVC, path: '/readyz', name: 'match readyz' },
  { host: MATCH_SVC, path: '/api/system/status', name: 'match status' },
  { host: MATCH_SVC, path: '/api/matches', name: 'match list' },
  { host: WEB, path: '/', name: 'web /' },
];

let success = 0;
let failure = 0;
const results = {};

function fetch(opts) {
  return new Promise((resolve) => {
    const start = Date.now();
    const req = https.get({ hostname: opts.host, path: opts.path, port: 443, rejectUnauthorized: false }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        const dur = Date.now() - start;
        resolve({ status: res.statusCode, dur, err: null });
      });
    });
    req.on('error', (err) => {
      resolve({ status: 0, dur: Date.now() - start, err: err.message });
    });
    req.setTimeout(10000, () => { req.destroy(); resolve({ status: 0, dur: 10000, err: 'timeout' }); });
  });
}

async function worker(id, iterations) {
  for (let i = 0; i < iterations; i++) {
    for (const ep of endpoints) {
      const r = await fetch(ep);
      if (!results[ep.name]) results[ep.name] = { ok: 0, fail: 0, times: [] };
      if (r.status >= 200 && r.status < 400) {
        results[ep.name].ok++;
        success++;
      } else {
        results[ep.name].fail++;
        failure++;
      }
      results[ep.name].times.push(r.dur);
    }
  }
}

async function main() {
  const CONCURRENT = 200;
  const ITERATIONS_PER_WORKER = 5;
  const workers = [];
  console.log(`Starting load test: ${CONCURRENT} concurrent users, ${ITERATIONS_PER_WORKER} cycles each`);
  console.log(`Total requests: ${CONCURRENT * ITERATIONS_PER_WORKER * endpoints.length}\n`);
  const startAll = Date.now();

  for (let i = 0; i < CONCURRENT; i++) {
    workers.push(worker(i, ITERATIONS_PER_WORKER));
  }
  await Promise.all(workers);

  const totalTime = (Date.now() - startAll) / 1000;
  const totalReq = success + failure;

  console.log(`\n=== RESULTS ===`);
  console.log(`Duration: ${totalTime.toFixed(1)}s`);
  console.log(`Total requests: ${totalReq}`);
  console.log(`Success: ${success} (${(success/totalReq*100).toFixed(1)}%)`);
  console.log(`Failure: ${failure} (${(failure/totalReq*100).toFixed(1)}%)`);
  console.log(`Throughput: ${(totalReq / totalTime).toFixed(1)} req/s`);
  console.log(`Concurrent users: ${CONCURRENT}\n`);

  console.log(`Endpoint breakdown:`);
  for (const [name, r] of Object.entries(results)) {
    const total = r.ok + r.fail;
    const avg = r.times.length > 0 ? (r.times.reduce((a,b) => a+b, 0) / r.times.length).toFixed(0) : 0;
    const sorted = [...r.times].sort((a,b) => a-b);
    const p95 = sorted.length > 0 ? sorted[Math.floor(sorted.length * 0.95)] : 0;
    console.log(`  ${name}: ok=${r.ok} fail=${r.fail} avg=${avg}ms p95=${p95}ms`);
  }
}

main().catch(console.error);
