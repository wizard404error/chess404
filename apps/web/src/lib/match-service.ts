import type { MatchModeId, MatchPresenceRequest, MatchSnapshotMessage, PlayerIntent } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';

const gatewayBaseUrl = '/api/gateway';
let httpBaseUrl = '/api/realtime';
let wsBaseUrl = '';
const MATCH_POLL_INTERVAL_MS = 750;
const MATCH_POLL_RETRY_INTERVAL_MS = 900;

export interface MatchServiceRuntimeConfig {
  httpBaseUrl?: string;
  wsBaseUrl?: string;
}

export interface CreateMatchInput {
  matchId?: string;
  seed?: number;
  clockSeconds?: number;
  starterHandMode?: 'starter_three' | 'full_catalog';
  queue?: 'casual' | 'rated' | 'direct';
  modeId?: MatchModeId;
  whiteGuestId?: string;
  blackGuestId?: string;
  whiteAccountId?: string;
  blackAccountId?: string;
  whiteName?: string;
  blackName?: string;
  whitePlayerSecret?: string;
  blackPlayerSecret?: string;
  whiteClaimToken?: string;
  blackClaimToken?: string;
}

export interface StoredRoomMeta extends CreateMatchInput {
  viewerSeat?: 'white' | 'black' | null;
  whiteClaimExpiresAt?: string;
  blackClaimExpiresAt?: string;
}

const ROOM_META_PREFIX = 'chess404.room.';

export function configureMatchServiceRuntime(config?: MatchServiceRuntimeConfig): void {
  const nextHttpBase = normalizeBaseUrl(config?.httpBaseUrl);
  if (nextHttpBase) {
    httpBaseUrl = nextHttpBase;
  }

  const nextWsBase = normalizeBaseUrl(config?.wsBaseUrl);
  if (nextWsBase) {
    wsBaseUrl = toWebSocketBaseUrl(nextWsBase);
  }
}

export async function createMatch(input: CreateMatchInput = {}): Promise<MatchSnapshotMessage> {
  const response = await fetch(`${httpBaseUrl}/matches`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(input)
  });

  return unwrapResponse<MatchSnapshotMessage>(response);
}

export async function fetchMatch(matchId: string): Promise<MatchSnapshotMessage> {
  const response = await fetch(`${httpBaseUrl}/matches/${matchId}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json'
    },
    cache: 'no-store'
  });

  return unwrapResponse<MatchSnapshotMessage>(response);
}

export async function ensureMatch(input: CreateMatchInput & { matchId: string }): Promise<MatchSnapshotMessage> {
  try {
    return await fetchMatch(input.matchId);
  } catch (err) {
    if (err instanceof Error && /404|not found/i.test(err.message)) {
      return createMatch(input);
    }
    throw err;
  }
}

export async function applyIntent(matchId: string, intent: Omit<PlayerIntent, 'matchId'>): Promise<MatchSnapshotMessage> {
  const response = await fetch(buildIntentUrl(matchId, intent), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      intent: {
        ...intent,
        matchId
      }
    })
  });

  return unwrapResponse<MatchSnapshotMessage>(response);
}

export async function sendMatchPresenceHeartbeat(
  matchId: string,
  presence: MatchPresenceRequest,
): Promise<void> {
  const response = await fetch(buildPresenceUrl(matchId, presence), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(presence),
  });

  if (!response.ok) {
    await unwrapResponse<never>(response);
  }
}

export function createSeatSecret(): string {
  if (typeof globalThis !== 'undefined' && globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  return `seat_${Date.now()}_${Math.random().toString(36).slice(2, 12)}`;
}

export function resolveSeatSecret(existingSecret?: string | null, guestSessionSecret?: string | null): string {
  const stored = normalizeSecret(existingSecret);
  if (stored) {
    return stored;
  }
  const session = normalizeSecret(guestSessionSecret);
  if (session) {
    return session;
  }
  return createSeatSecret();
}

export function readStoredRoomMeta(matchId: string): StoredRoomMeta | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = window.localStorage.getItem(`${ROOM_META_PREFIX}${matchId}`);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as StoredRoomMeta;
    return {
      ...parsed,
      modeId: parsed.modeId ?? DEFAULT_MATCH_MODE_ID,
    };
  } catch {
    return null;
  }
}

export function writeStoredRoomMeta(matchId: string, meta: StoredRoomMeta | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  const key = `${ROOM_META_PREFIX}${matchId}`;
  if (!meta) {
    window.localStorage.removeItem(key);
    return;
  }
  window.localStorage.setItem(key, JSON.stringify({
    ...meta,
    modeId: meta.modeId ?? DEFAULT_MATCH_MODE_ID,
  }));
}

export function connectToMatchStream(
  matchId: string,
  handlers: {
    onSnapshot: (snapshot: MatchSnapshotMessage) => void;
    onStatusChange?: (status: 'connecting' | 'connected' | 'reconnecting' | 'disconnected') => void;
    onError?: (error: Event) => void;
  },
  playerIdentity?: { playerId: string; playerSecret: string } | null
): () => void {
  let socket: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let pollTimer: number | null = null;
  let disposed = false;
  let reconnectAttempt = 0;

  const clearReconnectTimer = () => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  };

  const clearPollTimer = () => {
    if (pollTimer !== null) {
      window.clearTimeout(pollTimer);
      pollTimer = null;
    }
  };

  const schedulePoll = (delay = MATCH_POLL_INTERVAL_MS) => {
    if (disposed) {
      return;
    }
    clearPollTimer();
    pollTimer = window.setTimeout(async () => {
      pollTimer = null;
      if (disposed) {
        return;
      }
      try {
        const snapshot = await fetchMatch(matchId);
        if (!disposed) {
          handlers.onSnapshot(snapshot);
          handlers.onStatusChange?.('connected');
        }
      } catch {
        if (!disposed) {
          handlers.onStatusChange?.('reconnecting');
        }
      } finally {
        schedulePoll();
      }
    }, delay);
  };

  const scheduleReconnect = () => {
    if (disposed) {
      return;
    }
    clearReconnectTimer();
    handlers.onStatusChange?.('reconnecting');
    const delay = Math.min(5000, 500 * 2 ** Math.min(reconnectAttempt, 4));
    reconnectAttempt += 1;
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  };

  const connect = () => {
    if (disposed) {
      return;
    }
    handlers.onStatusChange?.(reconnectAttempt > 0 ? 'reconnecting' : 'connecting');
    const nextSocketUrl = resolveWebSocketBaseUrl();
    if (!nextSocketUrl) {
      handlers.onStatusChange?.('connected');
      schedulePoll(reconnectAttempt > 0 ? MATCH_POLL_RETRY_INTERVAL_MS : 0);
      return;
    }
    const wsUrl = playerIdentity
      ? `${nextSocketUrl}/api/matches/${matchId}/ws?playerId=${encodeURIComponent(playerIdentity.playerId)}&playerSecret=${encodeURIComponent(playerIdentity.playerSecret)}`
      : `${nextSocketUrl}/api/matches/${matchId}/ws`;
    const nextSocket = new WebSocket(wsUrl);
    socket = nextSocket;

    nextSocket.addEventListener('open', () => {
      reconnectAttempt = 0;
      handlers.onStatusChange?.('connected');
    });

    nextSocket.addEventListener('message', event => {
      try {
        const payload = JSON.parse(event.data) as { type?: string; payload?: MatchSnapshotMessage };
        if (payload.type === 'match.snapshot' && payload.payload) {
          handlers.onSnapshot(payload.payload);
        }
      } catch {
        // Ignore malformed stream payloads for now.
      }
    });

    nextSocket.addEventListener('error', event => {
      handlers.onError?.(event);
      // Close the socket so the close handler triggers reconnection
      if (!disposed) {
        nextSocket.close();
      }
    });

    nextSocket.addEventListener('close', () => {
      if (socket === nextSocket) {
        socket = null;
      }
      if (!disposed) {
        scheduleReconnect();
      }
    });
  };

  connect();

  return () => {
    disposed = true;
    clearReconnectTimer();
    clearPollTimer();
    handlers.onStatusChange?.('disconnected');
    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
      socket.close();
    }
  };
}

async function unwrapResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload?.error) {
        message = payload.error;
      }
    } catch {
      // Ignore parse failures and keep fallback message.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

function toWebSocketBaseUrl(input: string): string {
  if (input.startsWith('https://')) {
    return `wss://${input.slice('https://'.length)}`;
  }
  if (input.startsWith('http://')) {
    return `ws://${input.slice('http://'.length)}`;
  }
  return input;
}

function buildIntentUrl(matchId: string, intent?: Partial<PlayerIntent>): string {
  if (typeof intent?.playerClaimToken === 'string' && intent.playerClaimToken.trim()) {
    return `${gatewayBaseUrl}/matches/${matchId}/intents`;
  }
  if (/\/api\/realtime$/i.test(httpBaseUrl)) {
    return `${httpBaseUrl}/matches/${matchId}`;
  }
  return `${httpBaseUrl}/matches/${matchId}/intents`;
}

function buildPresenceUrl(matchId: string, presence?: Partial<MatchPresenceRequest>): string {
  if (typeof presence?.playerClaimToken === 'string' && presence.playerClaimToken.trim()) {
    return `${gatewayBaseUrl}/matches/${matchId}/presence`;
  }
  if (/\/api\/realtime$/i.test(httpBaseUrl)) {
    return `${httpBaseUrl}/matches/${matchId}/presence`;
  }
  return `${httpBaseUrl}/matches/${matchId}/presence`;
}

function normalizeSecret(value?: string | null): string {
  return typeof value === 'string' ? value.trim() : '';
}

function normalizeBaseUrl(value?: string | null): string {
  return typeof value === 'string' ? value.trim().replace(/\/$/, '') : '';
}

function resolveWebSocketBaseUrl(): string | null {
  if (wsBaseUrl) {
    return wsBaseUrl;
  }

  const derivedFromHttp = deriveWebSocketBaseUrlFromHttpBase(httpBaseUrl);
  if (derivedFromHttp) {
    return derivedFromHttp;
  }

  return null;
}

function deriveWebSocketBaseUrlFromHttpBase(input: string): string | null {
  const normalized = normalizeBaseUrl(input);
  if (!normalized) {
    return null;
  }
  if (normalized.startsWith('https://')) {
    return normalized.replace(/^https:\/\//i, 'wss://').replace(/\/api(?:\/realtime)?$/i, '');
  }
  if (normalized.startsWith('http://')) {
    return normalized.replace(/^http:\/\//i, 'ws://').replace(/\/api(?:\/realtime)?$/i, '');
  }
  return null;
}
