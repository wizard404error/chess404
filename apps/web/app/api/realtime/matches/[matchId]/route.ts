import { proxyRealtime } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

const matchServiceBaseUrl = resolveBackendBaseUrl(
  process.env.MATCH_SERVICE_INTERNAL_URL,
  'http://match-service.railway.internal:8080',
);

const platformServiceBaseUrl = resolveBackendBaseUrl(
  process.env.PLATFORM_SERVICE_INTERNAL_URL,
  'http://platform-service.railway.internal:8080',
);

export async function GET(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  const upstream = await fetch(`${matchServiceBaseUrl}/api/matches/${encodeURIComponent(matchId)}`, {
    method: 'GET',
    headers: filterHeaders(request.headers),
    cache: 'no-store',
  });
  const body = await upstream.text();
  if (!upstream.ok) {
    return new Response(body, {
      status: upstream.status,
      headers: filterResponseHeaders(upstream.headers),
    });
  }

  let snapshot: MatchSnapshotResponse;
  try {
    snapshot = JSON.parse(body) as MatchSnapshotResponse;
  } catch {
    return Response.json({ error: 'invalid match service response' }, { status: 502 });
  }

  if (isLocalRequest(request) || await requestOwnsMatchSeat(request, matchId)) {
    return Response.json(snapshot, { status: 200, headers: noStoreHeaders() });
  }

  if (!isPublicSpectatorReadable(snapshot)) {
    return Response.json({ error: 'match is not public' }, { status: 404, headers: noStoreHeaders() });
  }

  return Response.json(buildPublicSpectatorSnapshot(snapshot), {
    status: 200,
    headers: noStoreHeaders(),
  });
}

export async function POST(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  if (!isLocalRequest(request)) {
    return Response.json({
      error: 'direct match intents are not public; use the gateway match flow',
    }, { status: 404 });
  }
  return proxyRealtime(request, `/api/matches/${matchId}/intents`);
}

interface MatchSnapshotResponse {
  match: Record<string, any>;
  replayHead?: number;
  replayFrames?: any[];
  events?: Array<Record<string, any>>;
  seqNum?: number;
}

interface MatchClaimResponse {
  matchId?: string;
  guestId?: string;
  status?: string;
}

function isPublicSpectatorReadable(snapshot: MatchSnapshotResponse): boolean {
  const match = snapshot.match ?? {};
  const status = normalize(match.status);
  if (status !== 'active') {
    return false;
  }
  if (normalize(match.queue) === 'direct') {
    return false;
  }
  if (normalize(match.winner) || normalize(match.finishReason)) {
    return false;
  }
  return true;
}

function buildPublicSpectatorSnapshot(snapshot: MatchSnapshotResponse): MatchSnapshotResponse {
  const match = { ...(snapshot.match ?? {}) };
  delete match.whiteGuestId;
  delete match.blackGuestId;
  delete match.whiteAccountId;
  delete match.blackAccountId;
  delete match.seenClientMoveIds;
  delete match.whiteHand;
  delete match.blackHand;
  delete match.chatMessages;
  delete match.invisiblePiece;
  delete match.cheaterState;
  delete match.radarRevealFor;
  delete match.drawOfferTime;

  return {
    match: {
      ...match,
      whiteHand: [],
      blackHand: [],
      chatMessages: [],
    },
    replayHead: snapshot.replayHead ?? 0,
    replayFrames: [],
    events: (snapshot.events ?? []).map((event) => ({
      id: event.id,
      matchId: event.matchId,
      type: event.type,
      at: event.at,
    })),
    seqNum: snapshot.seqNum,
  };
}

async function requestOwnsMatchSeat(request: Request, matchId: string): Promise<boolean> {
  const candidates = readGuestSessionCandidates(request.headers);
  for (const candidate of candidates) {
    const payload: Record<string, string> = {
      matchId,
      guestId: candidate.guestId,
    };
    if (candidate.sessionToken) {
      payload.sessionToken = candidate.sessionToken;
    } else if (candidate.sessionSecret) {
      payload.sessionSecret = candidate.sessionSecret;
    }

    try {
      const response = await fetch(`${platformServiceBaseUrl}/api/platform/match-claims`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        cache: 'no-store',
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        continue;
      }
      const claim = await response.json() as MatchClaimResponse;
      if (normalize(claim.matchId) === normalize(matchId) && normalize(claim.guestId) === normalize(candidate.guestId) && isRecoverableClaimStatus(claim.status)) {
        return true;
      }
    } catch {
      continue;
    }
  }
  return false;
}

function readGuestSessionCandidates(headers: Headers): Array<{ guestId: string; sessionToken?: string; sessionSecret?: string }> {
  const sides = ['white', 'black'] as const;
  const candidates = sides.map((side) => ({
    guestId: normalize(headers.get(`x-chess404-${side}-guest-id`)),
    sessionToken: normalize(headers.get(`x-chess404-${side}-session-token`)) || undefined,
    sessionSecret: normalize(headers.get(`x-chess404-${side}-session-secret`)) || undefined,
  })).filter((candidate) => candidate.guestId && (candidate.sessionToken || candidate.sessionSecret));

  const generic = {
    guestId: normalize(headers.get('x-chess404-guest-id')),
    sessionToken: normalize(headers.get('x-chess404-session-token')) || undefined,
    sessionSecret: normalize(headers.get('x-chess404-session-secret')) || undefined,
  };
  if (generic.guestId && (generic.sessionToken || generic.sessionSecret)) {
    candidates.push(generic);
  }
  return candidates;
}

function isRecoverableClaimStatus(status?: string): boolean {
  const value = normalize(status);
  return value === 'waiting' || value === 'active';
}

function isLocalRequest(request: Request): boolean {
  if (process.env.NODE_ENV === 'production') {
    return false;
  }
  const host = request.headers.get('host')?.toLowerCase() ?? '';
  return host.startsWith('localhost') || host.startsWith('127.0.0.1');
}

function normalize(value: unknown): string {
  return typeof value === 'string' ? value.trim().toLowerCase() : '';
}

function noStoreHeaders(): Headers {
  const headers = new Headers();
  headers.set('Cache-Control', 'no-store');
  return headers;
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
  next.set('Cache-Control', 'no-store');
  return next;
}

function resolveBackendBaseUrl(explicit: string | undefined, fallback: string): string {
  const value = explicit?.trim().replace(/\/$/, '');
  if (!value || value.includes('${{') || /:\s*$/.test(value)) {
    return fallback;
  }
  return value;
}
