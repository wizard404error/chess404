import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const httpDuration = new Trend('http_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '1m', target: 100 },
    { duration: '30s', target: 200 },
    { duration: '1m', target: 200 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.1'],
  },
};

export default function () {
  const scenario = Math.random();

  if (scenario < 0.3) {
    testGatewayHealth();
  } else if (scenario < 0.5) {
    testMatchServiceHealth();
  } else if (scenario < 0.7) {
    testPlatformServiceHealth();
  } else if (scenario < 0.85) {
    testCreateMatch();
  } else {
    testListGuests();
  }

  sleep(0.1);
}

function testGatewayHealth() {
  const res = http.get(`${BASE_URL}/readyz`);
  const success = check(res, {
    'gateway health 200': (r) => r.status === 200,
    'gateway health < 200ms': (r) => r.timings.duration < 200,
  });
  errorRate.add(!success);
  httpDuration.add(res.timings.duration);
}

function testMatchServiceHealth() {
  const res = http.get(`${BASE_URL}/api/realtime/readyz`);
  const success = check(res, {
    'match health 200': (r) => r.status === 200,
    'match health < 200ms': (r) => r.timings.duration < 200,
  });
  errorRate.add(!success);
  httpDuration.add(res.timings.duration);
}

function testPlatformServiceHealth() {
  const res = http.get(`${BASE_URL}/api/platform/readyz`);
  const success = check(res, {
    'platform health 200': (r) => r.status === 200,
    'platform health < 200ms': (r) => r.timings.duration < 200,
  });
  errorRate.add(!success);
  httpDuration.add(res.timings.duration);
}

function testCreateMatch() {
  const payload = JSON.stringify({
    modeId: 'open_cards',
    clockSeconds: 600,
  });

  const res = http.post(`${BASE_URL}/api/realtime/matches`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const success = check(res, {
    'create match 200': (r) => r.status === 200,
    'create match < 500ms': (r) => r.timings.duration < 500,
    'create match has matchId': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.match && body.match.matchId;
      } catch {
        return false;
      }
    },
  });
  errorRate.add(!success);
  httpDuration.add(res.timings.duration);
}

function testListGuests() {
  const res = http.get(`${BASE_URL}/api/platform/guests?limit=10`);
  const success = check(res, {
    'list guests 200': (r) => r.status === 200,
    'list guests < 300ms': (r) => r.timings.duration < 300,
  });
  errorRate.add(!success);
  httpDuration.add(res.timings.duration);
}
