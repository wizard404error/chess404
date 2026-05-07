import type { MatchSnapshotMessage } from '@chess404/contracts';

const httpBaseUrl = (
  process.env.NEXT_PUBLIC_PLATFORM_SERVICE_HTTP_BASE ??
  process.env.NEXT_PUBLIC_PLATFORM_SERVICE_URL ??
  '/api/platform'
).replace(/\/$/, '');

export interface MatchArchiveEntry {
  matchId: string;
  status: string;
  winner?: string;
  rulesVersion: string;
  queue?: 'casual' | 'rated';
  whiteGuestId?: string;
  blackGuestId?: string;
  whiteAccountId?: string;
  blackAccountId?: string;
  whiteAccountHandle?: string;
  blackAccountHandle?: string;
  whiteName?: string;
  blackName?: string;
  createdAt: string;
  updatedAt: string;
  moveCount: number;
  lastMove?: string;
  snapshot: MatchSnapshotMessage;
}

export interface GuestProfile {
  guestId: string;
  displayName: string;
  rating: number;
  matchesPlayed: number;
  wins: number;
  losses: number;
  draws: number;
  createdAt: string;
  lastSeenAt: string;
}

export interface GuestSession {
  guest: GuestProfile;
  sessionSecret: string;
  sessionToken?: string;
  expiresAt?: string;
}

export interface MatchSeatClaim {
  matchId: string;
  guestId: string;
  seatColor: 'white' | 'black';
  playerId: string;
  playerSecret: string;
  claimToken: string;
  expiresAt: string;
  queue?: 'casual' | 'rated';
  whiteGuestId?: string;
  blackGuestId?: string;
  whiteName?: string;
  blackName?: string;
  status?: string;
}

export interface AccountRatingHistoryEntry {
  matchId: string;
  opponentAccountId?: string;
  result: 'win' | 'loss' | 'draw';
  winner: 'white' | 'black' | 'draw';
  delta: number;
  ratingBefore: number;
  ratingAfter: number;
  matchesPlayed: number;
  at: string;
}

export interface AccountSeasonSummary {
  seasonId: string;
  label: string;
  ratingStart: number;
  ratingEnd: number;
  peakRating: number;
  matchesPlayed: number;
  wins: number;
  losses: number;
  draws: number;
  netDelta: number;
  startedAt: string;
  lastPlayedAt: string;
}

export interface SeasonOption {
  seasonId: string;
  label: string;
}

export interface AccountProfile {
  accountId: string;
  handle: string;
  primaryGuestId: string;
  linkedGuestIds: string[];
  createdAt: string;
  lastSeenAt: string;
  displayName?: string;
  rating?: number;
  matchesPlayed?: number;
  wins?: number;
  losses?: number;
  draws?: number;
  guestCount?: number;
  currentSeason?: AccountSeasonSummary;
  selectedSeason?: AccountSeasonSummary;
  ratingHistory?: AccountRatingHistoryEntry[];
  seasonHistory?: AccountSeasonSummary[];
}

export interface AccountSession {
  account: AccountProfile;
  sessionToken: string;
  expiresAt: string;
}

export interface GuestResultResponse {
  changed: boolean;
  white: GuestProfile;
  black: GuestProfile;
}

export interface AccountResultResponse extends GuestResultResponse {
  whiteAccount: AccountProfile;
  blackAccount: AccountProfile;
}

export interface AccountLeaderboardResponse {
  accounts: AccountProfile[];
  seasons: SeasonOption[];
  selectedSeasonId?: string;
}

export async function fetchRankings(limit = 20): Promise<GuestProfile[]> {
  const response = await fetch(`${httpBaseUrl}/rankings?limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ players?: GuestProfile[] }>(response);
  return payload.players ?? [];
}

export async function fetchGuests(limit = 24): Promise<GuestProfile[]> {
  const response = await fetch(`${httpBaseUrl}/guests?limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ guests?: GuestProfile[] }>(response);
  return payload.guests ?? [];
}

export async function fetchGuest(guestId: string): Promise<GuestProfile> {
  const response = await fetch(`${httpBaseUrl}/guests/${guestId}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ guest: GuestProfile }>(response);
  return payload.guest;
}

export async function fetchArchivedMatches(limit = 20): Promise<MatchArchiveEntry[]> {
  const response = await fetch(`${httpBaseUrl}/matches?limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ matches?: MatchArchiveEntry[] }>(response);
  return payload.matches ?? [];
}

export async function fetchGuestArchivedMatches(guestId: string, limit = 12): Promise<MatchArchiveEntry[]> {
  const response = await fetch(`${httpBaseUrl}/matches?guestId=${encodeURIComponent(guestId)}&limit=${limit}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ matches?: MatchArchiveEntry[] }>(response);
  return payload.matches ?? [];
}

export async function fetchAccountArchivedMatches(accountId: string, limit = 12, seasonId?: string): Promise<MatchArchiveEntry[]> {
  const params = new URLSearchParams({
    accountId,
    limit: String(limit),
  });
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  const response = await fetch(`${httpBaseUrl}/matches?${params.toString()}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ matches?: MatchArchiveEntry[] }>(response);
  return payload.matches ?? [];
}

export async function fetchArchivedMatch(matchId: string): Promise<MatchArchiveEntry> {
  const response = await fetch(`${httpBaseUrl}/matches/${matchId}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  return unwrapResponse<MatchArchiveEntry>(response);
}

export async function createGuestSession(input: { guestId?: string; sessionSecret?: string; sessionToken?: string } = {}): Promise<GuestSession> {
  const response = await fetch(`${httpBaseUrl}/guest-sessions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      guestId: input.guestId,
      sessionSecret: input.sessionSecret,
      sessionToken: input.sessionToken,
    }),
  });

  return unwrapResponse<GuestSession>(response);
}

export async function claimMatchSeat(input: {
  matchId: string;
  guestId: string;
  sessionSecret?: string;
  sessionToken?: string;
}): Promise<MatchSeatClaim> {
  const response = await fetch(`${httpBaseUrl}/match-claims`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<MatchSeatClaim>(response);
}

export async function claimAccount(input: {
  guestId: string;
  sessionSecret?: string;
  sessionToken?: string;
  handle: string;
}): Promise<AccountSession> {
  const response = await fetch(`${httpBaseUrl}/accounts/claim`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSession>(response);
}

export async function resumeAccountSession(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountSession> {
  const response = await fetch(`${httpBaseUrl}/account-sessions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSession>(response);
}

export async function fetchAccount(accountId: string, seasonId?: string): Promise<AccountProfile> {
  const suffix = seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : '';
  const response = await fetch(`${httpBaseUrl}/accounts/${accountId}${suffix}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ account: AccountProfile }>(response);
  return payload.account;
}

export async function fetchAccounts(limit = 24, sort: 'recent' | 'rating' = 'recent', seasonId?: string): Promise<AccountProfile[]> {
  const payload = await fetchAccountLeaderboard(limit, sort, seasonId);
  return payload.accounts;
}

export async function fetchAccountLeaderboard(limit = 24, sort: 'recent' | 'rating' = 'recent', seasonId?: string): Promise<AccountLeaderboardResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    sort,
  });
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  const response = await fetch(`${httpBaseUrl}/accounts?${params.toString()}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ accounts?: AccountProfile[]; seasons?: SeasonOption[]; selectedSeasonId?: string }>(response);
  return {
    accounts: payload.accounts ?? [],
    seasons: payload.seasons ?? [],
    selectedSeasonId: payload.selectedSeasonId,
  };
}

export async function fetchGuestAccount(guestId: string, seasonId?: string): Promise<AccountProfile> {
  const suffix = seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : '';
  const response = await fetch(`${httpBaseUrl}/accounts/by-guest/${encodeURIComponent(guestId)}${suffix}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ account: AccountProfile }>(response);
  return payload.account;
}

export async function finalizeGuestMatch(input: {
  matchId: string;
  whiteGuestId: string;
  blackGuestId: string;
  winner: 'white' | 'black' | 'draw';
}): Promise<GuestResultResponse> {
  const response = await fetch(`${httpBaseUrl}/guest-results`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<GuestResultResponse>(response);
}

export async function finalizeAccountMatch(input: {
  matchId: string;
  whiteAccountId: string;
  blackAccountId: string;
  winner: 'white' | 'black' | 'draw';
}): Promise<AccountResultResponse> {
  const response = await fetch(`${httpBaseUrl}/account-results`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountResultResponse>(response);
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
