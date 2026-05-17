import type { MatchFinishReason, MatchModeId, MatchSnapshotMessage } from '@chess404/contracts';

const httpBaseUrl = (
  process.env.NEXT_PUBLIC_PLATFORM_SERVICE_HTTP_BASE ??
  process.env.NEXT_PUBLIC_PLATFORM_SERVICE_URL ??
  '/api/platform'
).replace(/\/$/, '');

export interface MatchArchiveEntry {
  matchId: string;
  status: string;
  winner?: string;
  finishReason?: MatchFinishReason;
  rulesVersion: string;
  queue?: 'casual' | 'rated' | 'direct';
  modeId?: MatchModeId;
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
  queue?: 'casual' | 'rated' | 'direct';
  modeId?: MatchModeId;
  whiteGuestId?: string;
  blackGuestId?: string;
  whiteName?: string;
  blackName?: string;
  status?: string;
}

export interface AccountRatingHistoryEntry {
  matchId: string;
  opponentAccountId?: string;
  queue?: 'casual' | 'rated' | 'direct';
  modeId?: MatchModeId;
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

export interface AccountLeaderboardSpotlight {
  accountId: string;
  handle: string;
  displayName: string;
  rating: number;
  peakRating: number;
  netDelta: number;
  matchesPlayed: number;
  wins: number;
  losses: number;
  draws: number;
}

export interface AccountLeaderboardSummary {
  modeId?: MatchModeId;
  seasonId?: string;
  seasonLabel?: string;
  playerCount: number;
  matchCount: number;
  leader?: AccountLeaderboardSpotlight;
  biggestClimber?: AccountLeaderboardSpotlight;
  highestPeak?: AccountLeaderboardSpotlight;
  mostActive?: AccountLeaderboardSpotlight;
}

export interface AccountProfile {
  accountId: string;
  handle: string;
  primaryGuestId: string;
  linkedGuestIds: string[];
  createdAt: string;
  lastSeenAt: string;
  lastActiveAt?: string;
  presenceStatus?: 'online' | 'recently_active' | 'offline';
  online?: boolean;
  recentlyActive?: boolean;
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

export interface AccountSessionRecord {
  sessionToken: string;
  expiresAt: string;
  createdAt: string;
  lastSeenAt: string;
}

export interface AccountSessionOverview {
  account: AccountProfile;
  sessions: AccountSessionRecord[];
}

export interface AccountAuthOverview {
  accountId: string;
  handle: string;
  email?: string;
  passwordLoginEnabled: boolean;
  emailVerified: boolean;
  emailVerifiedAt?: string;
  pendingEmailVerification: boolean;
  verificationExpiresAt?: string;
}

export interface AccountLoginResult {
  account: AccountSession;
  guest: GuestSession;
}

export interface AccountRegistrationResult extends AccountLoginResult {
  overview: AccountAuthOverview;
  requestedVerification?: boolean;
  expiresAt?: string;
  previewToken?: string;
  delivery?: AccountEmailDelivery;
}

export interface AccountEmailVerificationRequestResult {
  overview: AccountAuthOverview;
  requested: boolean;
  email?: string;
  expiresAt?: string;
  previewToken?: string;
  delivery?: AccountEmailDelivery;
}

export interface AccountPasswordResetRequestResult {
  requested: boolean;
  previewAccountId?: string;
  email?: string;
  expiresAt?: string;
  previewToken?: string;
  delivery?: AccountEmailDelivery;
}

export interface AccountEmailDelivery {
  deliveryId: string;
  accountId: string;
  email: string;
  kind: string;
  subject: string;
  textBody: string;
  htmlBody: string;
  actionUrl?: string;
  status: string;
  provider?: string;
  providerMessageId?: string;
  attemptCount: number;
  lastAttemptAt?: string;
  nextAttemptAt?: string;
  deliveredAt?: string;
  failedAt?: string;
  failureReason?: string;
  createdAt: string;
  updatedAt: string;
}

export interface AccountEmailDeliveryOverview {
  deliveries: AccountEmailDelivery[];
}

export interface AccountSecurityEvent {
  eventId: string;
  accountId: string;
  kind: string;
  detail?: string;
  createdAt: string;
}

export interface AccountSecurityEventOverview {
  events: AccountSecurityEvent[];
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
  summary?: AccountLeaderboardSummary;
  selectedSeasonId?: string;
  selectedModeId?: MatchModeId;
  selectedQuery?: string;
}

export interface FriendshipView {
  friendshipId: string;
  account: AccountProfile;
  createdAt: string;
}

export interface FriendRequestView {
  requestId: string;
  status: string;
  account: AccountProfile;
  createdAt: string;
  updatedAt: string;
}

export interface FriendOverview {
  viewer: AccountProfile;
  friends: FriendshipView[];
  incoming: FriendRequestView[];
  outgoing: FriendRequestView[];
}

export interface DirectChallengeView {
  challengeId: string;
  status: string;
  account: AccountProfile;
  matchId: string;
  modeId?: MatchModeId;
  clockSeconds?: number;
  challengerSeat?: 'white' | 'black';
  viewerSeat?: 'white' | 'black';
  createdAt: string;
  updatedAt: string;
}

export interface DirectChallengeOverview {
  viewer: AccountProfile;
  incoming: DirectChallengeView[];
  outgoing: DirectChallengeView[];
}

export interface AccountNotificationView {
  notificationId: string;
  kind:
    | 'friend_request_received'
    | 'friend_request_accepted'
    | 'direct_challenge_received'
    | 'direct_challenge_accepted'
    | 'direct_challenge_declined'
    | 'direct_challenge_cancelled';
  actor: AccountProfile;
  friendRequestId?: string;
  challengeId?: string;
  matchId?: string;
  modeId?: MatchModeId;
  challengerSeat?: 'white' | 'black';
  createdAt: string;
  updatedAt: string;
  readAt?: string;
}

export interface AccountNotificationOverview {
  viewer: AccountProfile;
  notifications: AccountNotificationView[];
  unreadCount: number;
}

export interface AccountNotificationStreamEvent {
  eventId: string;
  accountId: string;
  kind: 'created' | 'read' | 'bulk_read' | 'purged';
  notificationId?: string;
  unreadCount: number;
  occurredAt: string;
}

export interface AccountBlockView {
  blockId: string;
  direction: 'outgoing' | 'incoming';
  reason?: string;
  account: AccountProfile;
  createdAt: string;
  updatedAt: string;
}

export interface PlayerReportView {
  reportId: string;
  category: string;
  details?: string;
  status: string;
  target: AccountProfile;
  reviewedBy?: AccountProfile;
  reviewedAt?: string;
  resolutionNote?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ModerationOverview {
  viewer: AccountProfile;
  outgoingBlocks: AccountBlockView[];
  incomingBlocks: AccountBlockView[];
  submittedReports: PlayerReportView[];
}

export interface AccountRestrictionView {
  restrictionId: string;
  account: AccountProfile;
  kind: 'suspended' | 'banned' | string;
  reason?: string;
  reportId?: string;
  appliedBy?: AccountProfile;
  createdAt: string;
  updatedAt: string;
}

export interface ModerationAdminReportView {
  reportId: string;
  category: string;
  details?: string;
  status: string;
  reporter: AccountProfile;
  target: AccountProfile;
  targetRestriction?: AccountRestrictionView;
  reviewedBy?: AccountProfile;
  reviewedAt?: string;
  resolutionNote?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ModerationActionAuditView {
  actionId: string;
  reportId: string;
  previousStatus: string;
  nextStatus: string;
  action: string;
  note?: string;
  moderator: AccountProfile;
  reporter: AccountProfile;
  target: AccountProfile;
  createdAt: string;
}

export interface ModerationAdminOverview {
  viewer: AccountProfile;
  selectedStatus?: string;
  reports: ModerationAdminReportView[];
  recentActions: ModerationActionAuditView[];
  activeRestrictions: AccountRestrictionView[];
}

export interface AccountRestrictionState {
  kind: 'suspended' | 'banned' | string;
  reason?: string;
}

export class PlatformAccountRestrictionError extends Error {
  restriction: AccountRestrictionState;

  constructor(message: string, restriction: AccountRestrictionState) {
    super(message);
    this.name = 'PlatformAccountRestrictionError';
    this.restriction = restriction;
  }
}

export function isAccountRestrictionError(error: unknown): error is PlatformAccountRestrictionError {
  return error instanceof PlatformAccountRestrictionError;
}

export function parseAccountRestrictionMessage(message?: string | null): AccountRestrictionState | null {
  const resolved = message?.trim().toLowerCase() ?? '';
  if (!resolved) {
    return null;
  }
  if (resolved.includes('account suspended')) {
    return { kind: 'suspended' };
  }
  if (resolved.includes('account banned')) {
    return { kind: 'banned' };
  }
  return null;
}

export function formatAccountRestrictionNotice(
  restriction: AccountRestrictionState,
  fallback = 'This account cannot use the platform right now.',
): string {
  const base = restriction.kind === 'banned'
    ? 'This account has been banned from Chess404.'
    : restriction.kind === 'suspended'
      ? 'This account is currently suspended from Chess404.'
      : fallback;
  const reason = restriction.reason?.trim();
  return reason ? `${base} Reason: ${reason}` : base;
}

export type PublicMatchStatusFilter = 'active' | 'finished';

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

export async function fetchArchivedMatches(limit = 20, modeId?: MatchModeId, status?: PublicMatchStatusFilter): Promise<MatchArchiveEntry[]> {
  const params = new URLSearchParams({
    limit: String(limit),
  });
  if (modeId) {
    params.set('modeId', modeId);
  }
  if (status) {
    params.set('status', status);
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

export async function fetchGuestArchivedMatches(guestId: string, limit = 12, modeId?: MatchModeId): Promise<MatchArchiveEntry[]> {
  const params = new URLSearchParams({
    guestId,
    limit: String(limit),
  });
  if (modeId) {
    params.set('modeId', modeId);
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

export async function fetchAccountArchivedMatches(accountId: string, limit = 12, seasonId?: string, modeId?: MatchModeId): Promise<MatchArchiveEntry[]> {
  const params = new URLSearchParams({
    accountId,
    limit: String(limit),
  });
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  if (modeId) {
    params.set('modeId', modeId);
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

export async function touchAccountPresence(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountSession> {
  const response = await fetch(`${httpBaseUrl}/account-presence`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSession>(response);
}

export async function fetchAccountSessionOverview(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountSessionOverview> {
  const response = await fetch(`${httpBaseUrl}/account-sessions/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSessionOverview>(response);
}

export async function revokeAccountSessionToken(input: {
  accountId: string;
  sessionToken: string;
  revokeToken: string;
}): Promise<void> {
  const response = await fetch(`${httpBaseUrl}/account-sessions/revoke`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  if (!response.ok) {
    await unwrapResponse(response);
  }
}

export async function revokeOtherAccountSessions(input: {
  accountId: string;
  sessionToken: string;
}): Promise<void> {
  const response = await fetch(`${httpBaseUrl}/account-sessions/revoke-others`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  if (!response.ok) {
    await unwrapResponse(response);
  }
}

export async function enableAccountPasswordLogin(input: {
  accountId: string;
  sessionToken: string;
  email: string;
  password: string;
}): Promise<AccountSession> {
  const response = await fetch(`${httpBaseUrl}/account-auth/credentials`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSession>(response);
}

export async function fetchAccountAuthOverview(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountAuthOverview> {
  const response = await fetch(`${httpBaseUrl}/account-auth/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountAuthOverview>(response);
}

export async function requestAccountEmailVerification(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountEmailVerificationRequestResult> {
  const response = await fetch(`${httpBaseUrl}/account-auth/email-verification/request`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountEmailVerificationRequestResult>(response);
}

export async function confirmAccountEmailVerification(input: {
  accountId: string;
  token: string;
}): Promise<AccountAuthOverview> {
  const response = await fetch(`${httpBaseUrl}/account-auth/email-verification/confirm`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountAuthOverview>(response);
}

export async function loginAccountWithPassword(input: {
  identifier: string;
  password: string;
}): Promise<AccountLoginResult> {
  const response = await fetch(`${httpBaseUrl}/account-auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountLoginResult>(response);
}

export async function registerAccountWithPassword(input: {
  handle: string;
  email: string;
  password: string;
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
}): Promise<AccountRegistrationResult> {
  const response = await fetch(`${httpBaseUrl}/account-auth/register`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountRegistrationResult>(response);
}

export async function requestPasswordReset(input: {
  identifier: string;
}): Promise<AccountPasswordResetRequestResult> {
  const response = await fetch(`${httpBaseUrl}/account-auth/password-reset/request`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountPasswordResetRequestResult>(response);
}

export async function fetchAccountEmailOutboxOverview(input: {
  accountId: string;
  sessionToken: string;
  limit?: number;
}): Promise<AccountEmailDeliveryOverview> {
  const response = await fetch(`${httpBaseUrl}/email-outbox/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountEmailDeliveryOverview>(response);
}

export async function fetchAccountSecurityOverview(input: {
  accountId: string;
  sessionToken: string;
  limit?: number;
}): Promise<AccountSecurityEventOverview> {
  const response = await fetch(`${httpBaseUrl}/account-security/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountSecurityEventOverview>(response);
}

export async function confirmPasswordReset(input: {
  accountId: string;
  token: string;
  password: string;
}): Promise<AccountLoginResult> {
  const response = await fetch(`${httpBaseUrl}/account-auth/password-reset/confirm`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountLoginResult>(response);
}

export async function logoutAccountSession(input: {
  accountId: string;
  sessionToken: string;
}): Promise<void> {
  const response = await fetch(`${httpBaseUrl}/account-auth/logout`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  if (!response.ok) {
    await unwrapResponse(response);
  }
}

export async function fetchFriendOverview(input: {
  accountId: string;
  sessionToken: string;
}): Promise<FriendOverview> {
  const response = await fetch(`${httpBaseUrl}/friends/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<FriendOverview>(response);
}

export async function sendFriendRequest(input: {
  accountId: string;
  sessionToken: string;
  targetHandle: string;
}): Promise<FriendOverview> {
  const response = await fetch(`${httpBaseUrl}/friends/requests`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<FriendOverview>(response);
}

export async function respondToFriendRequest(input: {
  accountId: string;
  sessionToken: string;
  requestId: string;
  accept: boolean;
}): Promise<FriendOverview> {
  const response = await fetch(`${httpBaseUrl}/friends/requests/${encodeURIComponent(input.requestId)}/respond`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      accountId: input.accountId,
      sessionToken: input.sessionToken,
      accept: input.accept,
    }),
  });

  return unwrapResponse<FriendOverview>(response);
}

export async function removeFriend(input: {
  accountId: string;
  sessionToken: string;
  friendAccountId: string;
}): Promise<FriendOverview> {
  const response = await fetch(`${httpBaseUrl}/friends/remove`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<FriendOverview>(response);
}

export async function fetchModerationOverview(input: {
  accountId: string;
  sessionToken: string;
}): Promise<ModerationOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationOverview>(response);
}

export async function blockAccount(input: {
  accountId: string;
  sessionToken: string;
  targetAccountId: string;
  reason?: string;
}): Promise<ModerationOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/blocks`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationOverview>(response);
}

export async function unblockAccount(input: {
  accountId: string;
  sessionToken: string;
  targetAccountId: string;
}): Promise<ModerationOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/blocks/remove`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationOverview>(response);
}

export async function submitPlayerReport(input: {
  accountId: string;
  sessionToken: string;
  targetAccountId: string;
  category: string;
  details?: string;
}): Promise<ModerationOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/reports`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationOverview>(response);
}

export async function fetchModerationAdminOverview(input: {
  accountId: string;
  sessionToken: string;
  limit?: number;
  status?: string;
}): Promise<ModerationAdminOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/admin/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationAdminOverview>(response);
}

export async function resolveModerationReport(input: {
  accountId: string;
  sessionToken: string;
  reportId: string;
  action: 'under_review' | 'resolved_actioned' | 'resolved_dismissed';
  restriction?: 'suspended' | 'banned' | 'clear';
  note?: string;
  limit?: number;
  status?: string;
}): Promise<ModerationAdminOverview> {
  const response = await fetch(`${httpBaseUrl}/moderation/admin/reports/resolve`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<ModerationAdminOverview>(response);
}

export async function fetchDirectChallengeOverview(input: {
  accountId: string;
  sessionToken: string;
}): Promise<DirectChallengeOverview> {
  const response = await fetch(`${httpBaseUrl}/challenges/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<DirectChallengeOverview>(response);
}

export async function fetchAccountNotificationOverview(input: {
  accountId: string;
  sessionToken: string;
  limit?: number;
}): Promise<AccountNotificationOverview> {
  const response = await fetch(`${httpBaseUrl}/inbox/overview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountNotificationOverview>(response);
}

export async function markAccountNotificationRead(input: {
  accountId: string;
  sessionToken: string;
  notificationId: string;
}): Promise<AccountNotificationOverview> {
  const response = await fetch(`${httpBaseUrl}/inbox/read`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountNotificationOverview>(response);
}

export async function markAllAccountNotificationsRead(input: {
  accountId: string;
  sessionToken: string;
}): Promise<AccountNotificationOverview> {
  const response = await fetch(`${httpBaseUrl}/inbox/read-all`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });

  return unwrapResponse<AccountNotificationOverview>(response);
}

export function connectAccountNotificationStream(input: {
  accountId: string;
  sessionToken: string;
  onEvent: (event: AccountNotificationStreamEvent) => void;
  onError?: (error: Error) => void;
}): () => void {
  let cancelled = false;
  let activeController: AbortController | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const clearReconnectTimer = () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  };

  const scheduleReconnect = () => {
    if (cancelled || reconnectTimer) {
      return;
    }
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      void connect();
    }, 3_000);
  };

  const processEventBlock = (block: string) => {
    const lines = block.split(/\r?\n/);
    let eventName = 'message';
    const dataLines: string[] = [];
    for (const line of lines) {
      if (!line) {
        continue;
      }
      if (line.startsWith(':')) {
        continue;
      }
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim();
        continue;
      }
      if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trimStart());
      }
    }
    if (eventName !== 'notification' || dataLines.length === 0) {
      return;
    }
    try {
      input.onEvent(JSON.parse(dataLines.join('\n')) as AccountNotificationStreamEvent);
    } catch (error) {
      input.onError?.(error instanceof Error ? error : new Error('Failed to parse notification stream event'));
    }
  };

  const connect = async () => {
    if (cancelled) {
      return;
    }
    activeController = new AbortController();
    let shouldReconnect = true;
    try {
      const response = await fetch(`${httpBaseUrl}/inbox/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'text/event-stream',
        },
        body: JSON.stringify({
          accountId: input.accountId,
          sessionToken: input.sessionToken,
        }),
        cache: 'no-store',
        signal: activeController.signal,
      });

      if (!response.ok) {
        if (response.status === 400 || response.status === 401 || response.status === 403) {
          shouldReconnect = false;
        }
        throw new Error(`Notification stream request failed with status ${response.status}`);
      }
      if (!response.body) {
        throw new Error('Notification stream response body missing');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (!cancelled) {
        const { value, done } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        let separatorIndex = buffer.indexOf('\n\n');
        while (separatorIndex >= 0) {
          const block = buffer.slice(0, separatorIndex).trim();
          buffer = buffer.slice(separatorIndex + 2);
          if (block) {
            processEventBlock(block);
          }
          separatorIndex = buffer.indexOf('\n\n');
        }
      }
    } catch (error) {
      if (!cancelled && !(error instanceof DOMException && error.name === 'AbortError')) {
        input.onError?.(error instanceof Error ? error : new Error('Notification stream failed'));
      }
    } finally {
      activeController = null;
      if (!cancelled && shouldReconnect) {
        scheduleReconnect();
      }
    }
  };

  void connect();

  return () => {
    cancelled = true;
    clearReconnectTimer();
    activeController?.abort();
  };
}

export async function declineDirectChallenge(input: {
  accountId: string;
  sessionToken: string;
  challengeId: string;
}): Promise<DirectChallengeView> {
  const response = await fetch(`${httpBaseUrl}/challenges/${encodeURIComponent(input.challengeId)}/respond`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      accountId: input.accountId,
      sessionToken: input.sessionToken,
      accept: false,
    }),
  });

  return unwrapResponse<DirectChallengeView>(response);
}

export async function cancelDirectChallenge(input: {
  accountId: string;
  sessionToken: string;
  challengeId: string;
}): Promise<DirectChallengeView> {
  const response = await fetch(`${httpBaseUrl}/challenges/${encodeURIComponent(input.challengeId)}/cancel`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      accountId: input.accountId,
      sessionToken: input.sessionToken,
    }),
  });

  return unwrapResponse<DirectChallengeView>(response);
}

export async function fetchAccount(accountId: string, seasonId?: string, modeId?: MatchModeId): Promise<AccountProfile> {
  const params = new URLSearchParams();
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  if (modeId) {
    params.set('modeId', modeId);
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
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

export async function fetchAccountByHandle(handle: string, seasonId?: string, modeId?: MatchModeId): Promise<AccountProfile> {
  const params = new URLSearchParams();
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  if (modeId) {
    params.set('modeId', modeId);
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
  const response = await fetch(`${httpBaseUrl}/accounts/by-handle/${encodeURIComponent(handle)}${suffix}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ account: AccountProfile }>(response);
  return payload.account;
}

export async function fetchAccounts(limit = 24, sort: 'recent' | 'rating' = 'recent', seasonId?: string, modeId?: MatchModeId, query?: string): Promise<AccountProfile[]> {
  const payload = await fetchAccountLeaderboard(limit, sort, seasonId, modeId, query);
  return payload.accounts;
}

export async function fetchAccountLeaderboard(limit = 24, sort: 'recent' | 'rating' = 'recent', seasonId?: string, modeId?: MatchModeId, query?: string): Promise<AccountLeaderboardResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    sort,
  });
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  if (modeId) {
    params.set('modeId', modeId);
  }
  if (query?.trim()) {
    params.set('query', query.trim());
  }
  const response = await fetch(`${httpBaseUrl}/accounts?${params.toString()}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });

  const payload = await unwrapResponse<{ accounts?: AccountProfile[]; seasons?: SeasonOption[]; summary?: AccountLeaderboardSummary; selectedSeasonId?: string; selectedModeId?: MatchModeId; selectedQuery?: string }>(response);
  return {
    accounts: payload.accounts ?? [],
    seasons: payload.seasons ?? [],
    summary: payload.summary,
    selectedSeasonId: payload.selectedSeasonId,
    selectedModeId: payload.selectedModeId,
    selectedQuery: payload.selectedQuery,
  };
}

export async function fetchGuestAccount(guestId: string, seasonId?: string, modeId?: MatchModeId): Promise<AccountProfile> {
  const params = new URLSearchParams();
  if (seasonId) {
    params.set('seasonId', seasonId);
  }
  if (modeId) {
    params.set('modeId', modeId);
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
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
    let restriction: AccountRestrictionState | null = null;
    try {
      const payload = (await response.json()) as {
        error?: string;
        restrictionKind?: string;
        restrictionReason?: string;
      };
      if (payload?.error) {
        message = payload.error;
      }
      if (payload?.restrictionKind) {
        restriction = {
          kind: payload.restrictionKind,
          reason: payload.restrictionReason?.trim() || undefined,
        };
      } else {
        restriction = parseAccountRestrictionMessage(payload?.error);
      }
    } catch {
      // Ignore parse failures and keep fallback message.
    }
    if (restriction) {
      throw new PlatformAccountRestrictionError(message, restriction);
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}
