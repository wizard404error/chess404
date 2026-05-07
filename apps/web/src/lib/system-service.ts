import type { AccountSession, GuestSession, MatchSeatClaim } from './platform-service';

export interface MatchServiceStatus {
  status: string;
  service: string;
  rulesVersion: string;
  checkedAt: string;
  stats: {
    loadedMatches: number;
    activeMatches: number;
    finishedMatches: number;
    subscriberCount: number;
    bufferedEventSets: number;
  };
}

export interface GatewayServiceHealth {
  url: string;
  healthy: boolean;
  statusCode?: number;
  payload?: unknown;
  error?: string;
}

export interface GatewaySystemStatus {
  status: string;
  service: string;
  checkedAt: string;
  services: {
    match: GatewayServiceHealth;
    platform: GatewayServiceHealth;
    matchmaking: GatewayServiceHealth;
  };
}

export interface PlatformServiceStatus {
  status: string;
  service: string;
  checkedAt: string;
  archiveBackend?: string;
  guestStoreBackend?: string;
  accountStoreBackend?: string;
  claimStoreBackend?: string;
  claimLeaseSeconds?: number;
  archive: {
    totalMatches: number;
    activeMatches: number;
    finishedMatches: number;
    ratedMatches: number;
    casualMatches: number;
    directMatches: number;
  };
  guests: {
    guestCount: number;
    finalizedMatchCount: number;
    rankedPlayers: number;
  };
  accounts: {
    accountCount: number;
    linkedGuestCount: number;
    activeSessionCount: number;
  };
  claims: {
    cachedClaims: number;
  };
}

export interface MatchmakingServiceStatus {
  status: string;
  service: string;
  checkedAt: string;
  stats: {
    backend?: string;
    totalTickets: number;
    casual: {
      queue: string;
      queuedCount: number;
      matchedCount: number;
      cancelledCount: number;
    };
    rated: {
      queue: string;
      queuedCount: number;
      matchedCount: number;
      cancelledCount: number;
    };
  };
}

export interface SystemStatusSnapshot {
  gateway: GatewaySystemStatus;
  match: MatchServiceStatus;
  platform: PlatformServiceStatus;
  matchmaking: MatchmakingServiceStatus;
}

export interface GatewayBootstrapIdentity {
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
}

export interface GatewayAccountIdentity {
  accountId?: string;
  sessionToken?: string;
}

export interface GatewayBootstrapGuestSessions {
  white?: GuestSession;
  black?: GuestSession;
}

export interface GatewayBootstrapMatchClaims {
  white?: MatchSeatClaim;
  black?: MatchSeatClaim;
}

export interface GatewayBootstrapAccountSessions {
  white?: AccountSession;
  black?: AccountSession;
}

export interface GatewayBootstrapErrors {
  white?: string;
  black?: string;
}

export interface GatewayBootstrapPayload {
  status: string;
  realtimeReady: boolean;
  platformReady: boolean;
  matchmakingReady: boolean;
  authoritative: boolean;
  services: GatewaySystemStatus['services'];
  serviceEndpoints: {
    matchServiceUrl: string;
    platformServiceUrl: string;
    matchmakingServiceUrl: string;
  };
  platformCaps?: unknown;
  defaultQueue?: unknown;
  guestSessions?: GatewayBootstrapGuestSessions;
  matchClaims?: GatewayBootstrapMatchClaims;
  accountSessions?: GatewayBootstrapAccountSessions;
  sessionErrors?: GatewayBootstrapErrors;
  claimErrors?: GatewayBootstrapErrors;
  accountErrors?: GatewayBootstrapErrors;
  requestedMatchId?: string;
  bootstrapCheckedAt: string;
  message: string;
}

export async function fetchSystemStatus(): Promise<SystemStatusSnapshot> {
  const [gateway, match, platform, matchmaking] = await Promise.all([
    fetchJSON<GatewaySystemStatus>('/api/gateway/status').catch(err => degradedGatewayStatus(err)),
    fetchJSON<MatchServiceStatus>('/api/realtime/status').catch(err => degradedMatchStatus(err)),
    fetchJSON<PlatformServiceStatus>('/api/platform/status').catch(err => degradedPlatformStatus(err)),
    fetchJSON<MatchmakingServiceStatus>('/api/matchmaking/status').catch(err => degradedMatchmakingStatus(err)),
  ]);

  return { gateway, match, platform, matchmaking };
}

export async function fetchGatewayBootstrap(input: {
  matchId?: string;
  white?: GatewayBootstrapIdentity;
  black?: GatewayBootstrapIdentity;
  whiteAccount?: GatewayAccountIdentity;
  blackAccount?: GatewayAccountIdentity;
}): Promise<GatewayBootstrapPayload> {
  const response = await fetch('/api/gateway/bootstrap', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
    body: JSON.stringify(input),
  });

  const envelope = await unwrapResponse<{ type?: string; payload?: GatewayBootstrapPayload }>(response);
  if (envelope.type !== 'gateway.bootstrap' || !envelope.payload) {
    throw new Error('Invalid gateway bootstrap response');
  }
  return envelope.payload;
}

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload?.error) {
        message = payload.error;
      }
    } catch {
      // Keep fallback message.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
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
      // Keep fallback message.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

function degradedGatewayStatus(error: unknown): GatewaySystemStatus {
  const message = error instanceof Error ? error.message : 'gateway unavailable';
  return {
    status: 'degraded',
    service: 'gateway',
    checkedAt: new Date().toISOString(),
    services: {
      match: { url: '', healthy: false, error: message },
      platform: { url: '', healthy: false, error: message },
      matchmaking: { url: '', healthy: false, error: message },
    },
  };
}

function degradedMatchStatus(error: unknown): MatchServiceStatus {
  void error;
  return {
    status: 'degraded',
    service: 'match-service',
    rulesVersion: 'unknown',
    checkedAt: new Date().toISOString(),
    stats: {
      loadedMatches: 0,
      activeMatches: 0,
      finishedMatches: 0,
      subscriberCount: 0,
      bufferedEventSets: 0,
    },
  };
}

function degradedPlatformStatus(error: unknown): PlatformServiceStatus {
  void error;
  return {
    status: 'degraded',
    service: 'platform-service',
    checkedAt: new Date().toISOString(),
    archiveBackend: 'unknown',
    guestStoreBackend: 'unknown',
    accountStoreBackend: 'unknown',
    claimStoreBackend: 'unknown',
    claimLeaseSeconds: 0,
    archive: {
      totalMatches: 0,
      activeMatches: 0,
      finishedMatches: 0,
      ratedMatches: 0,
      casualMatches: 0,
      directMatches: 0,
    },
    guests: {
      guestCount: 0,
      finalizedMatchCount: 0,
      rankedPlayers: 0,
    },
    accounts: {
      accountCount: 0,
      linkedGuestCount: 0,
      activeSessionCount: 0,
    },
    claims: {
      cachedClaims: 0,
    },
  };
}

function degradedMatchmakingStatus(error: unknown): MatchmakingServiceStatus {
  void error;
  return {
    status: 'degraded',
    service: 'matchmaking-service',
    checkedAt: new Date().toISOString(),
    stats: {
      backend: 'unknown',
      totalTickets: 0,
      casual: {
        queue: 'casual',
        queuedCount: 0,
        matchedCount: 0,
        cancelledCount: 0,
      },
      rated: {
        queue: 'rated',
        queuedCount: 0,
        matchedCount: 0,
        cancelledCount: 0,
      },
    },
  };
}
