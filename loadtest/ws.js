import { WebSocket } from 'k6/websockets';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const wsConnected = new Rate('ws_connected');
const wsMessages = new Counter('ws_messages');
const wsDuration = new Trend('ws_duration', true);
const wsErrors = new Rate('ws_errors');

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = __ENV.WS_URL || 'ws://localhost:8082';

export const options = {
  stages: [
    { duration: '30s', target: 20 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 100 },
    { duration: '2m', target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    ws_connected: ['rate>0.9'],
    ws_errors: ['rate<0.05'],
  },
};

function createMatch() {
  const res = http.post(`${BASE_URL}/api/realtime/matches`, JSON.stringify({
    modeId: 'open_cards',
    clockSeconds: 600,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status !== 200) return null;

  try {
    const body = JSON.parse(res.body);
    return body.match?.matchId;
  } catch {
    return null;
  }
}

function joinMatch(matchId) {
  const res = http.post(`${BASE_URL}/api/realtime/matches/${matchId}/join`, JSON.stringify({
    preferredSeat: 'black',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });
  return res.status === 200;
}

export default function () {
  const matchId = createMatch();
  if (!matchId) {
    wsErrors.add(1);
    return;
  }

  joinMatch(matchId);

  const startTime = Date.now();
  let messageCount = 0;

  const ws = new WebSocket(`${WS_URL}/api/realtime/matches/${matchId}/stream`);

  ws.onopen = () => {
    wsConnected.add(1);

    ws.send(JSON.stringify({
      type: 'subscribe',
      matchId: matchId,
    }));

    sleep(0.5);

    for (let i = 0; i < 5; i++) {
      ws.send(JSON.stringify({
        type: 'intent',
        intent: {
          type: 'chat',
          matchId: matchId,
          message: `Load test move ${i}`,
        },
      }));
      sleep(0.2);
    }
  };

  ws.onmessage = (e) => {
    messageCount++;
    wsMessages.add(1);
  };

  ws.onerror = (e) => {
    wsErrors.add(1);
  };

  ws.onclose = () => {
    const duration = Date.now() - startTime;
    wsDuration.add(duration);
  };

  sleep(3);

  ws.close();
}
