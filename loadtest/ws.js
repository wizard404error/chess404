import { WebSocket } from 'k6/websockets';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import http from 'k6/http';

const wsConnected = new Rate('ws_connected');
const wsMessages = new Counter('ws_messages');
const wsDuration = new Trend('ws_duration', true);
const wsErrors = new Rate('ws_errors');

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = __ENV.WS_URL || 'ws://localhost:8081';

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

function createMatch(vu) {
  const guestId = `loadtest_white_${vu}`;
  const res = http.post(`${BASE_URL}/api/matches`, JSON.stringify({
    modeId: 'open_cards',
    clockSeconds: 600,
    whiteGuestId: guestId,
    whiteName: `White Player ${vu}`,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status !== 201) return null;

  try {
    const body = JSON.parse(res.body);
    return {
      matchId: body.match?.matchId,
      playerId: guestId,
      playerSecret: body.match?.whitePlayerSecret,
    };
  } catch {
    return null;
  }
}

export default function () {
  const vu = __VU;
  const match = createMatch(vu);
  if (!match) {
    wsErrors.add(1);
    return;
  }

  const startTime = Date.now();
  let messageCount = 0;
  let authed = false;

  const ws = new WebSocket(`${WS_URL}/api/matches/${match.matchId}/ws`);

  ws.onopen = () => {
    wsConnected.add(1);

    ws.send(JSON.stringify({
      playerId: match.playerId,
      playerSecret: match.playerSecret,
    }));

    sleep(0.5);

    for (let i = 0; i < 5; i++) {
      ws.send(JSON.stringify({
        type: 'intent',
        intent: {
          type: 'chat',
          matchId: match.matchId,
          playerId: match.playerId,
          playerSecret: match.playerSecret,
          text: `Load test message ${i}`,
        },
      }));
      sleep(0.2);
    }
  };

  ws.onmessage = (e) => {
    messageCount++;
    wsMessages.add(1);
    try {
      const msg = JSON.parse(e.data);
      if (msg.type === 'auth.success') {
        authed = true;
      }
    } catch {}
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
