import http from 'k6/http';
import { WebSocket } from 'k6/websockets';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const gameComplete = new Counter('games_completed');
const gameError = new Rate('game_errors');
const gameDuration = new Trend('game_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = __ENV.WS_URL || 'ws://localhost:8082';

const SAMPLE_MOVES = [
  ['e2', 'e4'], ['e7', 'e5'],
  ['g1', 'f3'], ['b8', 'c6'],
  ['f1', 'b5'], ['a7', 'a6'],
  ['b5', 'a4'], ['g8', 'f6'],
  ['d2', 'd3'], ['b7', 'b5'],
  ['a4', 'b3'], ['f8', 'c5'],
  ['0-0', ''], ['0-0', ''],
  ['c2', 'c3'], ['d7', 'd6'],
  ['d3', 'd4'], ['e5', 'd4'],
  ['c3', 'd4'], ['c6', 'd4'],
  ['f3', 'd4'], ['c5', 'd4'],
];

export const options = {
  stages: [
    { duration: '1m', target: 10 },
    { duration: '2m', target: 25 },
    { duration: '2m', target: 50 },
    { duration: '2m', target: 50 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    game_errors: ['rate<0.1'],
    game_duration: ['p(95)<30000'],
  },
};

function createAndJoinMatch() {
  const createRes = http.post(`${BASE_URL}/api/realtime/matches`, JSON.stringify({
    modeId: 'open_cards',
    clockSeconds: 300,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (createRes.status !== 200) return null;

  let matchId;
  try {
    const body = JSON.parse(createRes.body);
    matchId = body.match?.matchId;
  } catch {
    return null;
  }

  if (!matchId) return null;

  const joinRes = http.post(`${BASE_URL}/api/realtime/matches/${matchId}/join`, JSON.stringify({
    preferredSeat: 'black',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (joinRes.status !== 200) return null;

  return matchId;
}

function playGame(matchId) {
  const startTime = Date.now();
  let moveCount = 0;

  return new Promise((resolve) => {
    const ws = new WebSocket(`${WS_URL}/api/realtime/matches/${matchId}/stream`);
    let subscribed = false;

    ws.onopen = () => {
      ws.send(JSON.stringify({
        type: 'subscribe',
        matchId: matchId,
      }));
    };

    ws.onmessage = (e) => {
      let data;
      try {
        data = JSON.parse(e.data);
      } catch {
        return;
      }

      if (data.type === 'snapshot' && data.match?.turn === 'black' && moveCount < SAMPLE_MOVES.length) {
        const move = SAMPLE_MOVES[moveCount];
        moveCount++;

        ws.send(JSON.stringify({
          type: 'intent',
          intent: {
            type: 'make_move',
            matchId: matchId,
            from: { row: parseInt(move[0][1]) - 1, col: move[0].charCodeAt(0) - 97 },
            to: { row: parseInt(move[1][1]) - 1, col: move[1].charCodeAt(0) - 97 },
          },
        }));
      }

      if (data.match?.status === 'finished' || moveCount >= SAMPLE_MOVES.length) {
        ws.close();
        resolve(Date.now() - startTime);
      }
    };

    ws.onerror = () => {
      resolve(Date.now() - startTime);
    };

    setTimeout(() => {
      ws.close();
      resolve(Date.now() - startTime);
    }, 15000);
  });
}

export default async function () {
  const matchId = createAndJoinMatch();
  if (!matchId) {
    gameError.add(1);
    return;
  }

  try {
    const duration = await playGame(matchId);
    gameDuration.add(duration);
    gameComplete.add(1);
  } catch {
    gameError.add(1);
  }

  sleep(0.5);
}
