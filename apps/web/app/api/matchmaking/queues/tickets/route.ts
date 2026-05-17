import { proxyMatchmaking } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

const matchmakingBaseUrl = resolveBackendBaseUrl(
  process.env.MATCHMAKING_SERVICE_INTERNAL_URL,
  'http://matchmaking-service.railway.internal:8080',
);

const platformBaseUrl = resolveBackendBaseUrl(
  process.env.PLATFORM_SERVICE_INTERNAL_URL,
  'http://platform-service.railway.internal:8080',
);

interface QueueTicketCreatePayload {
  guestId?: string;
  queue?: 'casual' | 'rated';
  modeId?: string;
  rating?: number;
  displayName?: string;
  accountId?: string;
  accountSessionToken?: string;
}

interface PlatformAccountSessionPayload {
  account: {
    accountId: string;
    primaryGuestId: string;
    linkedGuestIds?: string[];
  };
}

export async function GET(request: Request): Promise<Response> {
  const { search } = new URL(request.url);
  return proxyMatchmaking(request, `/api/queues/tickets${search}`);
}

export async function POST(request: Request): Promise<Response> {
  let payload: QueueTicketCreatePayload;
  try {
    payload = (await request.json()) as QueueTicketCreatePayload;
  } catch {
    return jsonError('invalid queue ticket payload', 400);
  }

  const queue = payload.queue === 'rated' ? 'rated' : 'casual';
  const guestId = payload.guestId?.trim() ?? '';
  if (!guestId) {
    return jsonError('guestId is required', 400);
  }

  if (queue === 'rated') {
    const accountId = payload.accountId?.trim() ?? '';
    const sessionToken = payload.accountSessionToken?.trim() ?? '';
    if (!accountId || !sessionToken) {
      return jsonError('Rated queue requires a signed-in Chess404 account.', 401);
    }

    const session = await validateRatedAccountSession(request, accountId, sessionToken);
    if (session instanceof Response) {
      return session;
    }

    const linkedGuestIds = new Set<string>([
      session.account.primaryGuestId,
      ...(session.account.linkedGuestIds ?? []),
    ].map(value => value.trim()).filter(Boolean));

    if (!linkedGuestIds.has(guestId)) {
      return jsonError('Rated queue must use a guest linked to the signed-in Chess404 account.', 403);
    }
  }

  return forwardMatchmaking(request, {
    guestId,
    accountId: payload.accountId?.trim() ?? undefined,
    queue,
    modeId: payload.modeId,
    rating: payload.rating,
    displayName: payload.displayName,
  });
}

async function validateRatedAccountSession(
  request: Request,
  accountId: string,
  sessionToken: string,
): Promise<PlatformAccountSessionPayload | Response> {
  const upstream = await fetch(`${platformBaseUrl}/api/platform/account-sessions`, {
    method: 'POST',
    headers: ensureJSONHeaders(filterHeaders(request.headers)),
    cache: 'no-store',
    body: JSON.stringify({ accountId, sessionToken }),
  });

  const body = await upstream.text();
  if (!upstream.ok) {
    const headers = filterResponseHeaders(upstream.headers);
    const fallbackStatus = upstream.status === 403 ? 403 : 401;
    const parsed = tryParseErrorPayload(body);
    if (upstream.status === 403 && parsed?.restrictionKind) {
      return new Response(body, {
        status: upstream.status,
        headers,
      });
    }
    return jsonError('Rated queue requires a signed-in Chess404 account.', fallbackStatus, headers);
  }

  try {
    return JSON.parse(body) as PlatformAccountSessionPayload;
  } catch {
    return jsonError('failed to validate rated account session', 502);
  }
}

async function forwardMatchmaking(request: Request, payload: QueueTicketCreatePayload): Promise<Response> {
  const upstream = await fetch(`${matchmakingBaseUrl}/api/queues/tickets`, {
    method: 'POST',
    headers: ensureJSONHeaders(filterHeaders(request.headers)),
    cache: 'no-store',
    body: JSON.stringify(payload),
  });

  const body = await upstream.text();
  return new Response(body, {
    status: upstream.status,
    headers: filterResponseHeaders(upstream.headers),
  });
}

function jsonError(message: string, status: number, headers?: Headers): Response {
  const nextHeaders = headers ? new Headers(headers) : new Headers();
  nextHeaders.set('Content-Type', 'application/json');
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: nextHeaders,
  });
}

function tryParseErrorPayload(body: string): { error?: string; restrictionKind?: string; restrictionReason?: string } | null {
  try {
    return JSON.parse(body) as { error?: string; restrictionKind?: string; restrictionReason?: string };
  } catch {
    return null;
  }
}

function ensureJSONHeaders(headers: Headers): Headers {
  const next = new Headers(headers);
  if (!next.has('Content-Type')) {
    next.set('Content-Type', 'application/json');
  }
  if (!next.has('Accept')) {
    next.set('Accept', 'application/json');
  }
  return next;
}

function filterHeaders(headers: Headers): Headers {
  const next = new Headers();
  headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (lower === 'host' || lower === 'connection' || lower === 'content-length') {
      return;
    }
    next.set(key, value);
  });
  return next;
}

function filterResponseHeaders(headers: Headers): Headers {
  const next = new Headers();
  headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (lower === 'content-length' || lower === 'connection' || lower === 'transfer-encoding') {
      return;
    }
    next.set(key, value);
  });
  return next;
}

function resolveBackendBaseUrl(explicit: string | undefined, fallback: string): string {
  const value = explicit?.trim().replace(/\/$/, '');
  if (!value || value.includes('${{') || /:\s*$/.test(value)) {
    return fallback;
  }
  return value;
}
