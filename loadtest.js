import http from 'k6/http';
import { check, sleep, group } from 'k6';

const GATEWAY = 'https://gateway-production-1340.up.railway.app';
const MATCH_SVC = 'https://match-service-production-9f8b.up.railway.app';
const WEB = 'https://web-production-ddc27.up.railway.app';

export const options = {
  stages: [
    { target: 20, duration: '15s' },   // ramp up to 20
    { target: 100, duration: '30s' },  // ramp to 100
    { target: 200, duration: '20s' },  // ramp to 200
    { target: 200, duration: '30s' },  // hold at 200
    { target: 0, duration: '15s' },    // ramp down
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],       // <5% failure rate
    http_req_duration: ['p(95)<2000'],    // 95% under 2s
    'http_req_duration{gatetype:gateway}': ['p(95)<1000'],
    'http_req_duration{gatetype:match}': ['p(95)<1000'],
    'http_req_duration{gatetype:web}': ['p(95)<3000'],
  },
};

function gatewayHealth() {
  group('gateway health', () => {
    for (const path of ['/', '/healthz', '/readyz', '/livez', '/api/system/status']) {
      const r = http.get(`${GATEWAY}${path}`, { tags: { gatetype: 'gateway' } });
      check(r, {
        [`gateway ${path} status 2xx`]: (res) => res.status >= 200 && res.status < 300,
      });
      sleep(0.1);
    }
  });
}

function matchServiceHealth() {
  group('match-service health', () => {
    for (const path of ['/', '/healthz', '/readyz', '/api/system/status']) {
      const r = http.get(`${MATCH_SVC}${path}`, { tags: { gatetype: 'match' } });
      check(r, {
        [`match ${path} status 2xx`]: (res) => res.status >= 200 && res.status < 300,
      });
      sleep(0.1);
    }
  });
}

function webFrontend() {
  group('web frontend', () => {
    const r = http.get(`${WEB}/`, { tags: { gatetype: 'web' } });
    check(r, {
      'web / status 2xx': (res) => res.status >= 200 && res.status < 300,
    });
  });
}

function matchCreateFlow() {
  group('match creation flow', () => {
    // POST create match
    const createRes = http.post(`${MATCH_SVC}/api/matches`, JSON.stringify({
      mode_id: 'standard',
    }), {
      headers: { 'Content-Type': 'application/json' },
      tags: { gatetype: 'match' },
    });
    check(createRes, {
      'create match status 2xx': (r) => r.status >= 200 && r.status < 300,
    });

    if (createRes.status === 200) {
      sleep(0.5);
      // GET matches list
      const listRes = http.get(`${MATCH_SVC}/api/matches`, {
        tags: { gatetype: 'match' },
      });
      check(listRes, {
        'list matches status 2xx': (r) => r.status >= 200 && r.status < 300,
      });
    }
  });
}

export default function () {
  gatewayHealth();
  sleep(0.3);
  matchServiceHealth();
  sleep(0.3);
  webFrontend();
  sleep(0.3);
  matchCreateFlow();
  sleep(0.5);
}
