import React from 'react';
import { OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import {
  claimAccount,
  confirmAccountEmailVerification,
  confirmPasswordReset,
  enableAccountPasswordLogin,
  formatAccountRestrictionNotice,
  fetchAccountEmailOutboxOverview,
  fetchAccountAuthOverview,
  fetchAccountSecurityOverview,
  fetchAccountSessionOverview,
  fetchAccountArchivedMatches,
  fetchAccount,
  loginAccountWithPassword,
  logoutAccountSession,
  registerAccountWithPassword,
  requestAccountEmailVerification,
  requestPasswordReset,
  revokeAccountSessionToken,
  revokeOtherAccountSessions,
  resumeAccountSession,
  isAccountRestrictionError,
  type AccountAuthOverview,
  type AccountEmailDelivery,
  type AccountEmailDeliveryOverview,
  type AccountProfile,
  type AccountRatingHistoryEntry,
  type AccountPasswordResetRequestResult,
  type AccountSecurityEventOverview,
  type AccountSession,
  type AccountSessionOverview,
  type AccountSessionRecord,
  type GuestProfile,
  type GuestSession,
  type MatchArchiveEntry,
} from './lib/platform-service';

const WHITE_GUEST_ID_STORAGE_KEY = 'chess404.guest.white';
const BLACK_GUEST_ID_STORAGE_KEY = 'chess404.guest.black';
const WHITE_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.white.secret';
const BLACK_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.black.secret';
const WHITE_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.white.token';
const BLACK_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.black.token';
const WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY = 'chess404.guest.white.token.expiresAt';
const BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY = 'chess404.guest.black.token.expiresAt';

const WHITE_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.white.id';
const BLACK_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.black.id';
const WHITE_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.white.token';
const BLACK_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.black.token';
const WHITE_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.white.expiresAt';
const BLACK_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.black.expiresAt';

function formatDateTime(value?: string): string {
  if (!value) {
    return 'Unknown';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatRatingDelta(delta: number): string {
  return delta > 0 ? `+${delta}` : `${delta}`;
}

function describeSessionTokenFingerprint(sessionToken: string): string {
  const normalized = sessionToken.trim();
  if (!normalized) {
    return 'unknown-session';
  }
  if (normalized.length <= 12) {
    return normalized;
  }
  return `${normalized.slice(0, 8)}…${normalized.slice(-4)}`;
}

function parseModeFilterValue(value: string): MatchModeId | '' {
  return OFFICIAL_MATCH_MODES.some((mode) => mode.id === value as MatchModeId) ? (value as MatchModeId) : '';
}

function describeAccountEmailDeliveryKind(kind: string): string {
  switch (kind) {
    case 'account_email_verification':
      return 'Verification email';
    case 'account_password_reset':
      return 'Password reset email';
    default:
      return kind.replace(/_/g, ' ');
  }
}

function describeAccountEmailDeliveryStatus(delivery: AccountEmailDelivery): string {
  switch (delivery.status) {
    case 'delivered':
      return delivery.deliveredAt ? `Delivered ${formatDateTime(delivery.deliveredAt)}` : 'Delivered';
    case 'failed':
      return delivery.failedAt ? `Failed ${formatDateTime(delivery.failedAt)}` : 'Failed';
    default:
      return delivery.nextAttemptAt
        ? `Retry scheduled ${formatDateTime(delivery.nextAttemptAt)}`
        : `Queued ${formatDateTime(delivery.createdAt)}`;
  }
}

function describeAccountSecurityEvent(kind: string, detail?: string): string {
  const resolvedDetail = detail?.trim() ?? '';
  switch (kind) {
    case 'account_claimed':
      return resolvedDetail ? `Account claimed as @${resolvedDetail}` : 'Account claimed';
    case 'password_login_enabled':
      return resolvedDetail ? `Password sign-in enabled for ${resolvedDetail}` : 'Password sign-in enabled';
    case 'email_verification_requested':
      return resolvedDetail ? `Verification email queued for ${resolvedDetail}` : 'Verification email queued';
    case 'email_verified':
      return resolvedDetail ? `Email verified for ${resolvedDetail}` : 'Email verified';
    case 'password_login_succeeded':
      return resolvedDetail ? `Signed in with password as @${resolvedDetail}` : 'Signed in with password';
    case 'password_reset_requested':
      return resolvedDetail ? `Password reset requested for ${resolvedDetail}` : 'Password reset requested';
    case 'password_reset_completed':
      return resolvedDetail ? `Password reset completed for @${resolvedDetail}` : 'Password reset completed';
    case 'session_signed_out':
      return resolvedDetail ? `Signed out session ${resolvedDetail}` : 'Signed out active session';
    case 'session_revoked':
      return resolvedDetail ? `Revoked session ${resolvedDetail}` : 'Revoked another device session';
    case 'other_sessions_revoked':
      return resolvedDetail ? `Signed out ${resolvedDetail} other device session(s)` : 'Signed out other devices';
    default:
      return kind.replace(/_/g, ' ');
  }
}

function mergeAccountEmailDelivery(
  current: AccountEmailDeliveryOverview | null,
  delivery: AccountEmailDelivery,
): AccountEmailDeliveryOverview {
  const next = [delivery];
  for (const item of current?.deliveries ?? []) {
    if (item.deliveryId === delivery.deliveryId) {
      continue;
    }
    next.push(item);
  }
  return { deliveries: next.slice(0, 12) };
}

function parseAccountAuthActionUrl(value?: string | null): {
  action: 'verify-email' | 'reset-password' | null;
  accountId: string;
  token: string;
} {
  const resolved = value?.trim() ?? '';
  if (!resolved) {
    return { action: null, accountId: '', token: '' };
  }
  try {
    const url = new URL(resolved, typeof window !== 'undefined' ? window.location.origin : 'http://127.0.0.1');
    const auth = url.searchParams.get('auth');
    const accountId = url.searchParams.get('account')?.trim() ?? '';
    const token = url.searchParams.get('token')?.trim() ?? '';
    if ((auth === 'verify-email' || auth === 'reset-password') && token) {
      return { action: auth, accountId, token };
    }
  } catch {
    return { action: null, accountId: '', token: '' };
  }
  return { action: null, accountId: '', token: '' };
}

function readStoredGuestIdentity(side: 'white' | 'black'): {
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
  sessionExpiresAt?: string;
} {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    guestId: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY) ?? undefined,
    sessionSecret: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY) ?? undefined,
    sessionExpiresAt: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY) ?? undefined,
  };
}

function readStoredAccountSession(side: 'white' | 'black'): {
  accountId?: string;
  sessionToken?: string;
  expiresAt?: string;
} {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    accountId: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY) ?? undefined,
    expiresAt: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY) ?? undefined,
  };
}

function writeStoredAccountSession(side: 'white' | 'black', session: AccountSession | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  const idKey = side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY;
  const tokenKey = side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY;
  const expiryKey = side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY;
  if (!session) {
    window.localStorage.removeItem(idKey);
    window.localStorage.removeItem(tokenKey);
    window.localStorage.removeItem(expiryKey);
    return;
  }
  window.localStorage.setItem(idKey, session.account.accountId);
  window.localStorage.setItem(tokenKey, session.sessionToken);
  window.localStorage.setItem(expiryKey, session.expiresAt);
}

function writeStoredGuestSession(side: 'white' | 'black', session: GuestSession): void {
  if (typeof window === 'undefined') {
    return;
  }
  const idKey = side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY;
  const secretKey = side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY;
  const tokenKey = side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY;
  const expiryKey = side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY;
  window.localStorage.setItem(idKey, session.guest.guestId);
  window.localStorage.setItem(secretKey, session.sessionSecret);
  if ((session.sessionToken ?? '').trim()) {
    window.localStorage.setItem(tokenKey, session.sessionToken ?? '');
  } else {
    window.localStorage.removeItem(tokenKey);
  }
  if ((session.expiresAt ?? '').trim()) {
    window.localStorage.setItem(expiryKey, session.expiresAt ?? '');
  } else {
    window.localStorage.removeItem(expiryKey);
  }
}

function suggestHandle(seed: string): string {
  const normalized = seed
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 24);
  if (normalized.length >= 3) {
    return normalized;
  }
  return 'cardchess_player';
}

function describePerspective(entry: MatchArchiveEntry, accountId: string): 'white' | 'black' | 'unknown' {
  if (entry.whiteAccountId === accountId) {
    return 'white';
  }
  if (entry.blackAccountId === accountId) {
    return 'black';
  }
  return 'unknown';
}

function describeOutcome(entry: MatchArchiveEntry, accountId: string): string {
  const perspective = describePerspective(entry, accountId);
  if (entry.winner === 'draw') {
    return 'Draw';
  }
  if (perspective === 'white') {
    return entry.winner === 'white' ? 'Win' : 'Loss';
  }
  if (perspective === 'black') {
    return entry.winner === 'black' ? 'Win' : 'Loss';
  }
  return entry.winner ? `Winner: ${entry.winner}` : 'Unknown';
}

function describeOpponent(entry: MatchArchiveEntry, accountId: string): string {
  const perspective = describePerspective(entry, accountId);
  if (perspective === 'white') {
    return entry.blackAccountHandle ? `@${entry.blackAccountHandle}` : (entry.blackName ?? entry.blackGuestId ?? 'Unknown');
  }
  if (perspective === 'black') {
    return entry.whiteAccountHandle ? `@${entry.whiteAccountHandle}` : (entry.whiteName ?? entry.whiteGuestId ?? 'Unknown');
  }
  return entry.whiteName ?? entry.blackName ?? entry.matchId;
}

interface AccountPageProps {
  whiteProfile?: GuestProfile | null;
  blackProfile?: GuestProfile | null;
  externalNotice?: string | null;
  onOpenProfile?: (handle: string) => void;
  onSeatAuthenticated?: (side: 'white' | 'black', guestSession: GuestSession, accountSession: AccountSession) => void;
  onAuthStateChange?: () => void;
}

interface AccountSeatPanelProps {
  side: 'white' | 'black';
  label: string;
  accent: string;
  guestProfile?: GuestProfile | null;
  externalNotice?: string | null;
  onOpenProfile?: (handle: string) => void;
  onSeatAuthenticated?: (side: 'white' | 'black', guestSession: GuestSession, accountSession: AccountSession) => void;
  onAuthStateChange?: () => void;
}

function AccountSeatPanel({ side, label, accent, guestProfile = null, externalNotice = null, onOpenProfile, onSeatAuthenticated, onAuthStateChange }: AccountSeatPanelProps): React.ReactElement {
  const [guestIdentity, setGuestIdentity] = React.useState(() => readStoredGuestIdentity(side));
  const [accountSession, setAccountSession] = React.useState<AccountSession | null>(null);
  const [accountProfile, setAccountProfile] = React.useState<AccountProfile | null>(null);
  const [handle, setHandle] = React.useState('');
  const [authEmail, setAuthEmail] = React.useState('');
  const [authPassword, setAuthPassword] = React.useState('');
  const [loginIdentifier, setLoginIdentifier] = React.useState('');
  const [loginPassword, setLoginPassword] = React.useState('');
  const [loading, setLoading] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');
  const [selectedSeasonId, setSelectedSeasonId] = React.useState('');
  const [selectedModeId, setSelectedModeId] = React.useState<MatchModeId | ''>('');
  const [recentMatches, setRecentMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [recentMatchesLoading, setRecentMatchesLoading] = React.useState(false);
  const [recentMatchesError, setRecentMatchesError] = React.useState('');
  const [sessionOverview, setSessionOverview] = React.useState<AccountSessionOverview | null>(null);
  const [sessionOverviewLoading, setSessionOverviewLoading] = React.useState(false);
  const [sessionOverviewError, setSessionOverviewError] = React.useState('');
  const [authOverview, setAuthOverview] = React.useState<AccountAuthOverview | null>(null);
  const [authOverviewLoading, setAuthOverviewLoading] = React.useState(false);
  const [authOverviewError, setAuthOverviewError] = React.useState('');
  const [emailOutboxOverview, setEmailOutboxOverview] = React.useState<AccountEmailDeliveryOverview | null>(null);
  const [emailOutboxLoading, setEmailOutboxLoading] = React.useState(false);
  const [emailOutboxError, setEmailOutboxError] = React.useState('');
  const [securityOverview, setSecurityOverview] = React.useState<AccountSecurityEventOverview | null>(null);
  const [securityOverviewLoading, setSecurityOverviewLoading] = React.useState(false);
  const [securityOverviewError, setSecurityOverviewError] = React.useState('');
  const [verificationAccountId, setVerificationAccountId] = React.useState('');
  const [verificationToken, setVerificationToken] = React.useState('');
  const [verificationPreviewToken, setVerificationPreviewToken] = React.useState('');
  const [verificationPreviewExpiry, setVerificationPreviewExpiry] = React.useState('');
  const [resetPreview, setResetPreview] = React.useState<AccountPasswordResetRequestResult | null>(null);
  const [resetAccountId, setResetAccountId] = React.useState('');
  const [resetToken, setResetToken] = React.useState('');
  const [resetPassword, setResetPassword] = React.useState('');
  const appliedAuthQueryRef = React.useRef<string | null>(null);

  React.useEffect(() => {
    setGuestIdentity(readStoredGuestIdentity(side));
  }, [side, guestProfile?.guestId]);

  React.useEffect(() => {
    setHandle(current => {
      if (current.trim()) {
        return current;
      }
      if (accountProfile?.handle) {
        return accountProfile.handle;
      }
      if (guestProfile?.displayName) {
        return suggestHandle(guestProfile.displayName);
      }
      if (guestIdentity.guestId) {
        return suggestHandle(guestIdentity.guestId);
      }
      return current;
    });
  }, [accountProfile?.handle, guestIdentity.guestId, guestProfile?.displayName]);

  React.useEffect(() => {
    setLoginIdentifier(current => current.trim() ? current : (accountProfile?.handle ?? current));
  }, [accountProfile?.handle]);

  const refreshStoredAccount = React.useCallback(async () => {
    setLoading(true);
    setError('');
    const nextGuestIdentity = readStoredGuestIdentity(side);
    setGuestIdentity(nextGuestIdentity);
    const storedAccount = readStoredAccountSession(side);
    if (!storedAccount.accountId || !storedAccount.sessionToken) {
      setAccountSession(null);
      setAccountProfile(null);
      setLoading(false);
      return;
    }

    try {
      const session = await resumeAccountSession({
        accountId: storedAccount.accountId,
        sessionToken: storedAccount.sessionToken,
      });
      writeStoredAccountSession(side, session);
      onAuthStateChange?.();
      setAccountSession(session);
      setAccountProfile(current => current?.accountId === session.account.accountId ? current : session.account);
      setNotice('');
    } catch (err) {
      writeStoredAccountSession(side, null);
      onAuthStateChange?.();
      setAccountSession(null);
      setAuthOverview(null);
      try {
        const publicProfile = await fetchAccount(storedAccount.accountId);
        setAccountProfile(publicProfile);
        setHandle(publicProfile.handle);
        setNotice(isAccountRestrictionError(err) ? formatAccountRestrictionNotice(err.restriction) : 'Stored account session expired. Claim again to renew it.');
      } catch {
        setAccountProfile(null);
        setNotice(isAccountRestrictionError(err) ? formatAccountRestrictionNotice(err.restriction) : '');
      }
      if (isAccountRestrictionError(err)) {
        setError('');
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to resume account session.');
      }
    } finally {
      setLoading(false);
    }
  }, [onAuthStateChange, side]);

  React.useEffect(() => {
    void refreshStoredAccount();
  }, [refreshStoredAccount]);

  React.useEffect(() => {
    if (!accountSession) {
      setSessionOverview(null);
      setSessionOverviewLoading(false);
      setSessionOverviewError('');
      return;
    }

    let cancelled = false;
    setSessionOverviewLoading(true);
    setSessionOverviewError('');
    void fetchAccountSessionOverview({
      accountId: accountSession.account.accountId,
      sessionToken: accountSession.sessionToken,
    })
      .then((overview) => {
        if (cancelled) {
          return;
        }
        setSessionOverview(overview);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setSessionOverview(null);
        setSessionOverviewError(err instanceof Error ? err.message : 'Failed to load active account sessions.');
      })
      .finally(() => {
        if (!cancelled) {
          setSessionOverviewLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountSession?.account.accountId, accountSession?.sessionToken]);

  React.useEffect(() => {
    if (!accountSession) {
      setAuthOverview(null);
      setAuthOverviewLoading(false);
      setAuthOverviewError('');
      return;
    }

    let cancelled = false;
    setAuthOverviewLoading(true);
    setAuthOverviewError('');
    void fetchAccountAuthOverview({
      accountId: accountSession.account.accountId,
      sessionToken: accountSession.sessionToken,
    })
      .then((overview) => {
        if (cancelled) {
          return;
        }
        setAuthOverview(overview);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setAuthOverview(null);
        if (isAccountRestrictionError(err)) {
          writeStoredAccountSession(side, null);
          onAuthStateChange?.();
          setAccountSession(null);
          setNotice(formatAccountRestrictionNotice(err.restriction));
          setAuthOverviewError('');
        } else {
          setAuthOverviewError(err instanceof Error ? err.message : 'Failed to load account authentication status.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setAuthOverviewLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountSession?.account.accountId, accountSession?.sessionToken]);

  React.useEffect(() => {
    if (!accountSession) {
      setEmailOutboxOverview(null);
      setEmailOutboxLoading(false);
      setEmailOutboxError('');
      return;
    }

    let cancelled = false;
    setEmailOutboxLoading(true);
    setEmailOutboxError('');
    void fetchAccountEmailOutboxOverview({
      accountId: accountSession.account.accountId,
      sessionToken: accountSession.sessionToken,
      limit: 8,
    })
      .then((overview) => {
        if (cancelled) {
          return;
        }
        setEmailOutboxOverview(overview);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setEmailOutboxOverview(null);
        setEmailOutboxError(err instanceof Error ? err.message : 'Failed to load account email deliveries.');
      })
      .finally(() => {
        if (!cancelled) {
          setEmailOutboxLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountSession?.account.accountId, accountSession?.sessionToken]);

  React.useEffect(() => {
    if (!accountSession) {
      setSecurityOverview(null);
      setSecurityOverviewLoading(false);
      setSecurityOverviewError('');
      return;
    }

    let cancelled = false;
    setSecurityOverviewLoading(true);
    setSecurityOverviewError('');
    void fetchAccountSecurityOverview({
      accountId: accountSession.account.accountId,
      sessionToken: accountSession.sessionToken,
      limit: 10,
    })
      .then((overview) => {
        if (cancelled) {
          return;
        }
        setSecurityOverview(overview);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setSecurityOverview(null);
        setSecurityOverviewError(err instanceof Error ? err.message : 'Failed to load account security activity.');
      })
      .finally(() => {
        if (!cancelled) {
          setSecurityOverviewLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountSession?.account.accountId, accountSession?.sessionToken]);

  React.useEffect(() => {
    const nextAccountId = authOverview?.accountId ?? accountSession?.account.accountId ?? '';
    setVerificationAccountId((current) => current.trim() ? current : nextAccountId);
    if (authOverview?.email) {
      setAuthEmail((current) => current.trim() ? current : authOverview.email ?? current);
    }
  }, [accountSession?.account.accountId, authOverview?.accountId, authOverview?.email]);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const url = new URL(window.location.href);
    const auth = url.searchParams.get('auth');
    const accountId = url.searchParams.get('account')?.trim() ?? '';
    const token = url.searchParams.get('token')?.trim() ?? '';
    if ((auth !== 'verify-email' && auth !== 'reset-password') || !token) {
      return;
    }

    const activeAccountId = accountSession?.account.accountId ?? accountProfile?.accountId ?? '';
    if (accountId && activeAccountId && accountId !== activeAccountId) {
      return;
    }
    if (!accountId && !activeAccountId && side !== 'white') {
      return;
    }

    const queryKey = `${side}:${auth}:${accountId}:${token}`;
    if (appliedAuthQueryRef.current === queryKey) {
      return;
    }
    appliedAuthQueryRef.current = queryKey;

    if (auth === 'verify-email') {
      setVerificationAccountId(accountId || activeAccountId);
      setVerificationToken(token);
      setNotice('Verification link loaded. Confirm verification to finish the email check for this account.');
      setError('');
      return;
    }

    setResetAccountId(accountId || activeAccountId);
    setResetToken(token);
    setNotice('Password reset link loaded. Enter a new password to finish account recovery.');
    setError('');
  }, [accountProfile?.accountId, accountSession?.account.accountId, side]);

  React.useEffect(() => {
    const accountId = accountSession?.account.accountId ?? accountProfile?.accountId;
    if (!accountId) {
      return;
    }

    let cancelled = false;
    void fetchAccount(accountId, selectedSeasonId || undefined, selectedModeId || undefined)
      .then(profile => {
        if (cancelled) {
          return;
        }
        setAccountProfile(profile);
        if (selectedSeasonId && !profile.selectedSeason) {
          setSelectedSeasonId('');
        }
      })
      .catch(() => {
        // Keep the session-backed profile if the detailed fetch fails.
      });

    return () => {
      cancelled = true;
    };
  }, [accountProfile?.accountId, accountSession?.account.accountId, selectedModeId, selectedSeasonId]);

  React.useEffect(() => {
    const accountId = accountSession?.account.accountId ?? accountProfile?.accountId;
    if (!accountId) {
      setRecentMatches([]);
      setRecentMatchesLoading(false);
      setRecentMatchesError('');
      return;
    }

    let cancelled = false;
    setRecentMatchesLoading(true);
    setRecentMatchesError('');

    void fetchAccountArchivedMatches(accountId, 6, selectedSeasonId || undefined, selectedModeId || undefined)
      .then(matches => {
        if (cancelled) {
          return;
        }
        setRecentMatches(matches);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setRecentMatches([]);
        setRecentMatchesError(err instanceof Error ? err.message : 'Failed to load recent account matches.');
      })
      .finally(() => {
        if (!cancelled) {
          setRecentMatchesLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountProfile?.accountId, accountSession?.account.accountId, selectedModeId, selectedSeasonId]);

  const submitClaim = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const liveGuestIdentity = readStoredGuestIdentity(side);
      setGuestIdentity(liveGuestIdentity);
      if (!liveGuestIdentity.guestId) {
        throw new Error('No guest session is available for this seat yet.');
      }
      const desiredHandle = (accountProfile?.handle ?? handle).trim();
      const session = await claimAccount({
        guestId: liveGuestIdentity.guestId,
        sessionSecret: liveGuestIdentity.sessionSecret,
        sessionToken: liveGuestIdentity.sessionToken,
        handle: desiredHandle,
      });
      writeStoredAccountSession(side, session);
      onAuthStateChange?.();
      setAccountSession(session);
      setAccountProfile(session.account);
      setHandle(session.account.handle);
      setSelectedSeasonId('');
      setSelectedModeId('');
      setNotice('Account session is active on this device.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to claim account.');
    } finally {
      setBusy(false);
    }
  }, [accountProfile?.handle, handle, onAuthStateChange, side]);

  const renewSession = React.useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      await refreshStoredAccount();
      setNotice('Account session refreshed.');
    } finally {
      setBusy(false);
    }
  }, [refreshStoredAccount]);

  const enableLogin = React.useCallback(async () => {
    if (!accountSession) {
      setError('Claim or refresh this account before enabling sign-in.');
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const nextSession = await enableAccountPasswordLogin({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
        email: authEmail,
        password: authPassword,
      });
      writeStoredAccountSession(side, nextSession);
      onAuthStateChange?.();
      setAccountSession(nextSession);
      setAccountProfile(nextSession.account);
      setHandle(nextSession.account.handle);
      setLoginIdentifier(current => current.trim() ? current : nextSession.account.handle);
      setAuthPassword('');
      setNotice(`Password sign-in is enabled for ${authEmail.trim().toLowerCase()}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to enable account sign-in.');
    } finally {
      setBusy(false);
    }
  }, [accountSession, authEmail, authPassword, onAuthStateChange, side]);

  const registerWithPassword = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const liveGuestIdentity = readStoredGuestIdentity(side);
      const result = await registerAccountWithPassword({
        handle,
        email: authEmail,
        password: authPassword,
        guestId: liveGuestIdentity.guestId,
        sessionSecret: liveGuestIdentity.sessionSecret,
        sessionToken: liveGuestIdentity.sessionToken,
      });
      writeStoredGuestSession(side, result.guest);
      writeStoredAccountSession(side, result.account);
      onAuthStateChange?.();
      setGuestIdentity({
        guestId: result.guest.guest.guestId,
        sessionSecret: result.guest.sessionSecret,
        sessionToken: result.guest.sessionToken,
        sessionExpiresAt: result.guest.expiresAt,
      });
      setAccountSession(result.account);
      setAccountProfile(result.account.account);
      setAuthOverview(result.overview);
      setHandle(result.account.account.handle);
      setLoginIdentifier(result.account.account.handle);
      setAuthPassword('');
      setLoginPassword('');
      setVerificationAccountId(result.overview.accountId);
      setVerificationPreviewToken(result.previewToken ?? '');
      setVerificationPreviewExpiry(result.expiresAt ?? '');
      setVerificationToken(result.previewToken ?? '');
      setSelectedSeasonId('');
      setSelectedModeId('');
      if (result.delivery) {
        setEmailOutboxOverview((current) => mergeAccountEmailDelivery(current, result.delivery!));
      }
      onSeatAuthenticated?.(side, result.guest, result.account);
      setNotice(
        result.requestedVerification
          ? `Account created as @${result.account.account.handle}. Verification is queued for ${result.overview.email ?? authEmail.trim().toLowerCase()}.`
          : `Account created as @${result.account.account.handle}.`
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create account.');
    } finally {
      setBusy(false);
    }
  }, [authEmail, authPassword, handle, onAuthStateChange, onSeatAuthenticated, side]);

  const requestVerification = React.useCallback(async () => {
    if (!accountSession) {
      setError('Claim or refresh this account before requesting verification.');
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await requestAccountEmailVerification({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      setAuthOverview(result.overview);
      setVerificationAccountId(result.overview.accountId);
      if (result.delivery) {
        setEmailOutboxOverview((current) => mergeAccountEmailDelivery(current, result.delivery!));
      }
      setVerificationPreviewToken(result.previewToken ?? '');
      setVerificationPreviewExpiry(result.expiresAt ?? '');
      setVerificationToken(result.previewToken ?? verificationToken);
      setNotice(result.previewToken
        ? `Verification preview generated for ${result.email ?? authEmail.trim().toLowerCase()}.`
        : 'Verification request queued in the account email outbox.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to request email verification.');
    } finally {
      setBusy(false);
    }
  }, [accountSession, authEmail, verificationToken]);

  const confirmVerification = React.useCallback(async () => {
    const accountId = accountSession?.account.accountId ?? verificationAccountId ?? authOverview?.accountId ?? '';
    if (!accountId) {
      setError('Account verification needs an active account first.');
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const overview = await confirmAccountEmailVerification({
        accountId,
        token: verificationToken,
      });
      setAuthOverview(overview);
      setVerificationAccountId(overview.accountId);
      setVerificationPreviewToken('');
      setVerificationPreviewExpiry('');
      setNotice(`Email verified for ${overview.email ?? authEmail.trim().toLowerCase()}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to verify email.');
    } finally {
      setBusy(false);
    }
  }, [accountSession?.account.accountId, authEmail, authOverview?.accountId, verificationToken]);

  const loginWithPassword = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await loginAccountWithPassword({
        identifier: loginIdentifier,
        password: loginPassword,
      });
      writeStoredAccountSession(side, result.account);
      writeStoredGuestSession(side, result.guest);
      onAuthStateChange?.();
      setAccountSession(result.account);
      setAccountProfile(result.account.account);
      setHandle(result.account.account.handle);
      setLoginIdentifier(result.account.account.handle);
      setGuestIdentity({
        guestId: result.guest.guest.guestId,
        sessionSecret: result.guest.sessionSecret,
        sessionToken: result.guest.sessionToken,
      });
      onSeatAuthenticated?.(side, result.guest, result.account);
      setLoginPassword('');
      setNotice(`Signed in as @${result.account.account.handle} on this seat.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sign in.');
    } finally {
      setBusy(false);
    }
  }, [loginIdentifier, loginPassword, onAuthStateChange, onSeatAuthenticated, side]);

  const requestReset = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await requestPasswordReset({
        identifier: loginIdentifier,
      });
      setResetPreview(result);
      setResetAccountId(result.previewAccountId ?? resetAccountId);
      setResetToken(result.previewToken ?? resetToken);
      if (result.delivery) {
        setEmailOutboxOverview((current) => mergeAccountEmailDelivery(current, result.delivery!));
      }
      setNotice(result.previewToken
        ? `Password reset preview generated for ${result.email ?? loginIdentifier.trim()}.`
        : 'If this account has a verified email, a reset request has been queued in the account email outbox.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to request password reset.');
    } finally {
      setBusy(false);
    }
  }, [loginIdentifier, resetAccountId, resetToken]);

  const completeReset = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const result = await confirmPasswordReset({
        accountId: resetAccountId,
        token: resetToken,
        password: resetPassword,
      });
      writeStoredAccountSession(side, result.account);
      writeStoredGuestSession(side, result.guest);
      onAuthStateChange?.();
      setAccountSession(result.account);
      setAccountProfile(result.account.account);
      setGuestIdentity({
        guestId: result.guest.guest.guestId,
        sessionSecret: result.guest.sessionSecret,
        sessionToken: result.guest.sessionToken,
      });
      setLoginIdentifier(result.account.account.handle);
      setLoginPassword('');
      setResetPassword('');
      setNotice(`Password reset complete for @${result.account.account.handle}.`);
      onSeatAuthenticated?.(side, result.guest, result.account);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to complete password reset.');
    } finally {
      setBusy(false);
    }
  }, [onAuthStateChange, onSeatAuthenticated, resetAccountId, resetPassword, resetToken, side]);

  const activeAccount = accountProfile ?? accountSession?.account ?? null;

  const logoutSession = React.useCallback(async () => {
    if (!accountSession) {
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await logoutAccountSession({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      writeStoredAccountSession(side, null);
      onAuthStateChange?.();
      setAccountSession(null);
      setNotice('Account session signed out on this device.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sign out.');
    } finally {
      setBusy(false);
    }
  }, [accountSession, onAuthStateChange, side]);

  const signOutOtherSessions = React.useCallback(async () => {
    if (!accountSession) {
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await revokeOtherAccountSessions({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      const overview = await fetchAccountSessionOverview({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      setSessionOverview(overview);
      setSessionOverviewError('');
      setNotice('Signed out every other active device for this account.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sign out other devices.');
    } finally {
      setBusy(false);
    }
  }, [accountSession]);

  const revokeOtherSession = React.useCallback(async (revokeToken: string) => {
    if (!accountSession) {
      return;
    }
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await revokeAccountSessionToken({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
        revokeToken,
      });
      const overview = await fetchAccountSessionOverview({
        accountId: accountSession.account.accountId,
        sessionToken: accountSession.sessionToken,
      });
      setSessionOverview(overview);
      setSessionOverviewError('');
      setNotice(`Signed out session ${describeSessionTokenFingerprint(revokeToken)}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke the selected session.');
    } finally {
      setBusy(false);
    }
  }, [accountSession]);

  const copyProfileLink = React.useCallback(async (handle: string) => {
    if (typeof window === 'undefined') {
      return;
    }
    const profileUrl = `${window.location.origin}/?profile=${encodeURIComponent(handle)}`;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(profileUrl);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = profileUrl;
        textArea.style.position = 'fixed';
        textArea.style.opacity = '0';
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
      }
      setNotice(`Copied ${profileUrl}`);
    } catch {
      setNotice(profileUrl);
    }
  }, []);

  const copyTextValue = React.useCallback(async (value: string) => {
    if (typeof window === 'undefined') {
      return;
    }
    const resolved = value.trim();
    if (!resolved) {
      return;
    }
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(resolved);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = resolved;
        textArea.style.position = 'fixed';
        textArea.style.opacity = '0';
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
      }
      setNotice(`Copied ${resolved}`);
    } catch {
      setNotice(resolved);
    }
  }, []);

  const loadEmailAction = React.useCallback((delivery: AccountEmailDelivery) => {
    const parsed = parseAccountAuthActionUrl(delivery.actionUrl);
    if (parsed.action === 'verify-email') {
      setVerificationAccountId(parsed.accountId || verificationAccountId || activeAccount?.accountId || '');
      setVerificationToken(parsed.token);
      setNotice(`Loaded verification action from ${delivery.email}.`);
      return;
    }
    if (parsed.action === 'reset-password') {
      setResetAccountId(parsed.accountId || activeAccount?.accountId || '');
      setResetToken(parsed.token);
      setNotice(`Loaded password reset action from ${delivery.email}.`);
    }
  }, [activeAccount?.accountId, verificationAccountId]);

  const profile = guestProfile;
  const activeSessions = sessionOverview?.sessions ?? [];
  const currentSessionToken = accountSession?.sessionToken ?? '';
  const currentSessionRecord = activeSessions.find((session) => session.sessionToken === currentSessionToken) ?? null;
  const otherSessions = activeSessions.filter((session) => session.sessionToken !== currentSessionToken);
  const displayedRatingHistory = [...(activeAccount?.ratingHistory ?? [])].slice(-6).reverse();
  const displayedSeasonHistory = activeAccount?.seasonHistory?.slice(0, 4) ?? [];
  const highlightedSeason = activeAccount?.selectedSeason ?? activeAccount?.currentSeason;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '14px',
        minWidth: 0,
        background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
        border: `1px solid ${accent}`,
        borderRadius: '18px',
        padding: '20px',
        boxShadow: '0 16px 42px rgba(0,0,0,0.32)',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '16px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div>
          <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.8px', textTransform: 'uppercase', marginBottom: '6px' }}>{label}</div>
          <div style={{ color: '#fff4d6', fontSize: '24px', fontWeight: 800 }}>
            {profile?.displayName ?? guestIdentity.guestId ?? 'Guest seat'}
          </div>
          <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px', marginTop: '6px' }}>
            {profile ? `Guest rating ${profile.rating} · ${profile.wins}W ${profile.losses}L ${profile.draws}D` : 'Waiting for guest profile bootstrap.'}
          </div>
        </div>
        <div style={{
          padding: '8px 12px',
          borderRadius: '999px',
          border: `1px solid ${accent}`,
          color: '#ffe4a3',
          fontSize: '12px',
          fontWeight: 700,
          background: 'rgba(255,255,255,0.04)',
        }}>
          {accountSession ? 'Signed in locally' : activeAccount ? 'Claimed account' : 'Guest-only'}
        </div>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: '10px',
      }}>
        <MetaTile label="Guest ID" value={guestIdentity.guestId ?? 'Not available'} />
        <MetaTile label="Guest token" value={guestIdentity.sessionToken ? 'Active' : guestIdentity.sessionSecret ? 'Secret only' : 'Missing'} />
        <MetaTile label="Account handle" value={activeAccount?.handle ?? 'Unclaimed'} />
        <MetaTile label="Account session" value={accountSession?.expiresAt ? `Expires ${formatDateTime(accountSession.expiresAt)}` : 'No active session'} />
      </div>

      {error && (
        <div style={{
          padding: '12px 14px',
          borderRadius: '12px',
          background: 'rgba(120,18,18,0.34)',
          border: '1px solid rgba(220,80,80,0.45)',
          color: '#ffd6d6',
          fontSize: '13px',
          fontWeight: 700,
        }}>
          {error}
        </div>
      )}

      {(externalNotice || notice) && (
        <div style={{
          padding: '12px 14px',
          borderRadius: '12px',
          background: 'rgba(28,64,42,0.34)',
          border: '1px solid rgba(88,180,126,0.35)',
          color: '#d8ffe7',
          fontSize: '13px',
          fontWeight: 700,
        }}>
          {externalNotice || notice}
        </div>
      )}

      {loading ? (
        <div style={{ color: 'rgba(244,232,200,0.7)', fontSize: '13px' }}>Checking stored account session...</div>
      ) : (
        <>
          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '8px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '18px', fontWeight: 800 }}>@{activeAccount.handle}</div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Account ID: <span style={{ color: '#ffe9b1', fontFamily: 'monospace' }}>{activeAccount.accountId}</span>
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Primary guest: <span style={{ color: '#ffe9b1', fontFamily: 'monospace' }}>{activeAccount.primaryGuestId}</span>
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Linked guests: {activeAccount.linkedGuestIds.length}
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Ladder: {activeAccount.rating ?? 1200} · {activeAccount.matchesPlayed ?? 0} matches · {activeAccount.wins ?? 0}W {activeAccount.losses ?? 0}L {activeAccount.draws ?? 0}D
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Last seen: {formatDateTime(activeAccount.lastSeenAt)}
              </div>
              <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '4px' }}>
                <button
                  onClick={() => onOpenProfile?.(activeAccount.handle)}
                  style={{
                    padding: '8px 10px',
                    borderRadius: '999px',
                    border: '1px solid rgba(255,180,60,0.26)',
                    background: 'rgba(255,180,60,0.08)',
                    color: '#ffe9b1',
                    fontSize: '11px',
                    fontWeight: 800,
                    cursor: onOpenProfile ? 'pointer' : 'default',
                  }}
                >
                  Open Public Profile
                </button>
                <button
                  onClick={() => void copyProfileLink(activeAccount.handle)}
                  style={{
                    padding: '8px 10px',
                    borderRadius: '999px',
                    border: '1px solid rgba(255,180,60,0.16)',
                    background: 'rgba(255,255,255,0.03)',
                    color: '#fff2c8',
                    fontSize: '11px',
                    fontWeight: 800,
                    cursor: 'pointer',
                  }}
                >
                  Copy Profile Link
                </button>
              </div>
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '12px',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
                <div>
                  <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Active sessions</div>
                  <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                    This account can now stay signed in on multiple devices. The current seat keeps one active session token, and you can revoke any of the others here.
                  </div>
                </div>
                {accountSession && otherSessions.length > 0 && (
                  <button
                    onClick={() => void signOutOtherSessions()}
                    disabled={busy}
                    style={{
                      padding: '9px 12px',
                      borderRadius: '999px',
                      border: '1px solid rgba(255,180,60,0.24)',
                      background: 'rgba(255,255,255,0.03)',
                      color: '#ffe9b1',
                      fontSize: '11px',
                      fontWeight: 800,
                      cursor: busy ? 'not-allowed' : 'pointer',
                    }}
                  >
                    Sign Out Other Devices
                  </button>
                )}
              </div>
              {sessionOverviewLoading ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>Loading active account sessions...</div>
              ) : sessionOverviewError ? (
                <div style={{ color: '#ffd6d6', fontSize: '13px', fontWeight: 700 }}>{sessionOverviewError}</div>
              ) : !accountSession ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  Claim or sign in to this account to inspect active devices.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '10px' }}>
                  <AccountSessionCard
                    label="This device"
                    accent="rgba(88,180,126,0.28)"
                    record={currentSessionRecord}
                    fallbackExpiresAt={accountSession.expiresAt}
                    fallbackSessionToken={accountSession.sessionToken}
                    actionLabel="Current session"
                    disabledAction
                  />
                  {otherSessions.length === 0 ? (
                    <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                      No other active devices are signed in right now.
                    </div>
                  ) : (
                    otherSessions.map((record, index) => (
                      <AccountSessionCard
                        key={record.sessionToken}
                        label={`Other device ${index + 1}`}
                        accent="rgba(255,180,60,0.2)"
                        record={record}
                        actionLabel="Sign out"
                        onAction={() => void revokeOtherSession(record.sessionToken)}
                        busy={busy}
                      />
                    ))
                  )}
                </div>
              )}
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Season view</div>
              <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                <select
                  aria-label="Season view mode"
                  value={selectedModeId}
                  onChange={(event) => setSelectedModeId(parseModeFilterValue(event.target.value))}
                  style={{
                    padding: '8px 10px',
                    borderRadius: '8px',
                    border: '1px solid rgba(255,180,60,0.24)',
                    background: 'rgba(255,255,255,0.04)',
                    color: '#fff2c8',
                    fontSize: '12px',
                    fontWeight: 700,
                  }}
                >
                  <option value="">All official modes</option>
                  {OFFICIAL_MATCH_MODES.map((mode) => (
                    <option key={mode.id} value={mode.id}>
                      {mode.label}
                    </option>
                  ))}
                </select>
              </div>
              {highlightedSeason ? (
                <>
                  <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800 }}>
                    {highlightedSeason.label}
                  </div>
                  <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                    {highlightedSeason.matchesPlayed} matches - {highlightedSeason.wins}W {highlightedSeason.losses}L {highlightedSeason.draws}D - peak {highlightedSeason.peakRating}
                  </div>
                  <div style={{ color: '#7dffb4', fontSize: '13px', fontWeight: 700 }}>
                    Season delta {formatRatingDelta(highlightedSeason.netDelta)} to {highlightedSeason.ratingEnd}
                  </div>
                </>
              ) : (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No season summary exists yet for the selected official mode because this account has not completed an account-owned rated result there.
                </div>
              )}
              {displayedSeasonHistory.length > 0 && (
                <div style={{ display: 'grid', gap: '8px', marginTop: '4px' }}>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button
                      onClick={() => setSelectedSeasonId('')}
                      style={{
                        padding: '7px 10px',
                        borderRadius: '999px',
                        border: selectedSeasonId === '' ? '1px solid rgba(255,180,60,0.38)' : '1px solid rgba(255,180,60,0.18)',
                        background: selectedSeasonId === '' ? 'rgba(255,180,60,0.14)' : 'rgba(255,255,255,0.03)',
                        color: '#ffe9b1',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: 'pointer',
                      }}
                    >
                      All seasons
                    </button>
                    {displayedSeasonHistory.map((season) => (
                      <button
                        key={`season-chip-${season.seasonId}`}
                        onClick={() => setSelectedSeasonId(season.seasonId)}
                        style={{
                          padding: '7px 10px',
                          borderRadius: '999px',
                          border: selectedSeasonId === season.seasonId ? '1px solid rgba(255,180,60,0.38)' : '1px solid rgba(255,180,60,0.18)',
                          background: selectedSeasonId === season.seasonId ? 'rgba(255,180,60,0.14)' : 'rgba(255,255,255,0.03)',
                          color: '#ffe9b1',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        {season.label}
                      </button>
                    ))}
                  </div>
                  {displayedSeasonHistory.map((season) => (
                    <SeasonSummaryRow key={season.seasonId} summary={season} />
                  ))}
                </div>
              )}
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Rating history</div>
              {displayedRatingHistory.length === 0 ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No account-owned rating changes have been recorded yet.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '8px' }}>
                  {displayedRatingHistory.map((entry) => (
                    <RatingHistoryRow key={`${entry.matchId}-${entry.at}`} entry={entry} />
                  ))}
                </div>
              )}
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>
                {selectedSeasonId || selectedModeId ? 'Filtered account matches' : 'Recent account matches'}
              </div>
              {recentMatchesLoading ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>Loading archived matches...</div>
              ) : recentMatchesError ? (
                <div style={{ color: '#ffd6d6', fontSize: '13px', fontWeight: 700 }}>{recentMatchesError}</div>
              ) : recentMatches.length === 0 ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No archived matches are linked to this account yet.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '8px' }}>
                  {recentMatches.map((entry) => (
                    <div
                      key={entry.matchId}
                      style={{
                        borderRadius: '12px',
                        border: '1px solid rgba(255,180,60,0.12)',
                        background: 'rgba(255,255,255,0.022)',
                        padding: '12px 14px',
                        display: 'grid',
                        gap: '5px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
                        <div style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>
                          {describeOutcome(entry, activeAccount.accountId)} vs {describeOpponent(entry, activeAccount.accountId)}
                        </div>
                        <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
                          {entry.queue ?? 'direct'} · {entry.status}
                        </div>
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px' }}>
                        {entry.moveCount} moves · updated {formatDateTime(entry.updatedAt)}
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px', fontFamily: 'monospace' }}>
                        {entry.matchId}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <div style={{ display: 'grid', gap: '10px' }}>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                {activeAccount ? 'Renew using handle' : 'Claim handle'}
              </span>
              <input
                value={handle}
                onChange={(event) => setHandle(event.target.value)}
                placeholder="aurora_fox"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
              Handles are lowercase, 3-24 characters, and can use letters, numbers, `_`, or `-`.
            </div>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              <button
                onClick={() => void submitClaim()}
                disabled={busy || !handle.trim()}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.48)',
                  background: busy || !handle.trim()
                    ? 'rgba(120,90,35,0.38)'
                    : 'linear-gradient(180deg, rgba(200,134,10,0.92) 0%, rgba(122,79,8,0.95) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !handle.trim() ? 'not-allowed' : 'pointer',
                }}
              >
                {busy ? 'Working...' : activeAccount ? 'Renew Account Session' : 'Claim Account'}
              </button>
              {accountSession && (
                <button
                  onClick={() => void renewSession()}
                  disabled={busy}
                  style={{
                    padding: '11px 16px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.3)',
                    background: 'transparent',
                    color: '#ffe9b1',
                    fontSize: '13px',
                    fontWeight: 700,
                    cursor: busy ? 'not-allowed' : 'pointer',
                  }}
                >
                  Refresh Session
                </button>
              )}
            </div>
          </div>

          <div style={{
            borderRadius: '14px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
            padding: '16px',
            display: 'grid',
            gap: '12px',
          }}>
            <div>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Enable account sign-in</div>
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                Create a full reusable account directly, or add sign-in credentials to a handle you already claimed on this seat.
              </div>
            </div>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                Email
              </span>
              <input
                value={authEmail}
                onChange={(event) => setAuthEmail(event.target.value)}
                placeholder="player@example.com"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                Password
              </span>
              <input
                type="password"
                value={authPassword}
                onChange={(event) => setAuthPassword(event.target.value)}
                placeholder="at least 8 characters"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              <button
                onClick={() => void (accountSession ? enableLogin() : registerWithPassword())}
                disabled={busy || !handle.trim() || !authEmail.trim() || !authPassword.trim()}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.38)',
                  background: busy || !handle.trim() || !authEmail.trim() || !authPassword.trim()
                    ? 'rgba(120,90,35,0.28)'
                    : 'linear-gradient(180deg, rgba(120,84,255,0.88) 0%, rgba(76,51,166,0.96) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !handle.trim() || !authEmail.trim() || !authPassword.trim() ? 'not-allowed' : 'pointer',
                }}
              >
                {accountSession ? 'Enable Sign-In' : 'Create Account'}
              </button>
              {!accountSession && (
                <div style={{ color: 'rgba(244,232,200,0.56)', fontSize: '12px', alignSelf: 'center' }}>
                  This path creates the account immediately and signs this seat in, while still bridging through the seat guest identity underneath.
                </div>
              )}
            </div>
          </div>

          <div style={{
            borderRadius: '14px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
            padding: '16px',
            display: 'grid',
            gap: '12px',
          }}>
            <div>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Email verification</div>
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                Verified email is the gate for real recovery. Requests now queue into a durable account email outbox so verification and reset actions survive across sessions and devices.
              </div>
            </div>
            {authOverviewLoading ? (
              <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>Loading auth status...</div>
            ) : authOverview ? (
              <div style={{ display: 'grid', gap: '8px' }}>
                <div style={{ color: '#fff4d6', fontSize: '13px', fontWeight: 700 }}>
                  {authOverview.email ? authOverview.email : 'No account email configured yet'}
                </div>
                <div style={{ color: authOverview.emailVerified ? '#7dffb4' : '#ffd0a8', fontSize: '12px', fontWeight: 700 }}>
                  {authOverview.emailVerified
                    ? `Verified ${formatDateTime(authOverview.emailVerifiedAt)}`
                    : authOverview.pendingEmailVerification
                      ? `Pending verification until ${formatDateTime(authOverview.verificationExpiresAt)}`
                      : 'Not verified yet'}
                </div>
              </div>
            ) : (
              <div style={{ color: 'rgba(244,232,200,0.56)', fontSize: '12px' }}>
                {authOverviewError || 'Claim and enable account sign-in to manage verification.'}
              </div>
            )}
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              <button
                onClick={() => void requestVerification()}
                disabled={busy || !accountSession || !authOverview || authOverview.emailVerified || !authOverview.email}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.38)',
                  background: busy || !accountSession || !authOverview || authOverview.emailVerified || !authOverview.email
                    ? 'rgba(120,90,35,0.28)'
                    : 'linear-gradient(180deg, rgba(71,127,240,0.9) 0%, rgba(33,77,153,0.96) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !accountSession || !authOverview || authOverview.emailVerified || !authOverview.email ? 'not-allowed' : 'pointer',
                }}
              >
                {authOverview?.emailVerified ? 'Email Verified' : 'Request Verification'}
              </button>
            </div>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                Verification token
              </span>
              <input
                value={verificationToken}
                onChange={(event) => setVerificationToken(event.target.value)}
                placeholder="acctverify_..."
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            {verificationPreviewToken && (
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px', display: 'grid', gap: '4px' }}>
                <div>Preview token: <span style={{ fontFamily: 'monospace' }}>{verificationPreviewToken}</span></div>
                {verificationPreviewExpiry && <div>Expires {formatDateTime(verificationPreviewExpiry)}</div>}
              </div>
            )}
            {verificationAccountId && (
              <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
                Verification account: <span style={{ fontFamily: 'monospace' }}>{verificationAccountId}</span>
              </div>
            )}
            <div>
              <button
                onClick={() => void confirmVerification()}
                disabled={busy || !(verificationToken.trim())}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.38)',
                  background: busy || !verificationToken.trim()
                    ? 'rgba(120,90,35,0.28)'
                    : 'linear-gradient(180deg, rgba(74,173,120,0.92) 0%, rgba(27,108,67,0.96) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !verificationToken.trim() ? 'not-allowed' : 'pointer',
                }}
              >
                Confirm Verification
              </button>
            </div>
          </div>

          <div style={{
            borderRadius: '14px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
            padding: '16px',
            display: 'grid',
            gap: '12px',
          }}>
            <div>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Sign in on this seat</div>
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                Use a handle or email. Successful sign-in also restores the playable guest identity for this seat.
              </div>
            </div>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                Handle or email
              </span>
              <input
                value={loginIdentifier}
                onChange={(event) => setLoginIdentifier(event.target.value)}
                placeholder="aurora_fox or player@example.com"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                Password
              </span>
              <input
                type="password"
                value={loginPassword}
                onChange={(event) => setLoginPassword(event.target.value)}
                placeholder="your password"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              <button
                onClick={() => void loginWithPassword()}
                disabled={busy || !loginIdentifier.trim() || !loginPassword.trim()}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.38)',
                  background: busy || !loginIdentifier.trim() || !loginPassword.trim()
                    ? 'rgba(120,90,35,0.28)'
                    : 'linear-gradient(180deg, rgba(64,154,108,0.92) 0%, rgba(29,97,62,0.96) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !loginIdentifier.trim() || !loginPassword.trim() ? 'not-allowed' : 'pointer',
                }}
              >
                Sign In
              </button>
              {accountSession && (
                <button
                  onClick={() => void logoutSession()}
                  disabled={busy}
                  style={{
                    padding: '11px 16px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,120,120,0.28)',
                    background: 'rgba(90,24,24,0.32)',
                    color: '#ffd7d7',
                    fontSize: '13px',
                    fontWeight: 800,
                    cursor: busy ? 'not-allowed' : 'pointer',
                  }}
                >
                  Sign Out
                </button>
              )}
            </div>
            <div style={{
              marginTop: '4px',
              paddingTop: '10px',
              borderTop: '1px solid rgba(255,180,60,0.12)',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800 }}>Password reset preview</div>
              <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
                Verified accounts can request a reset token. If preview mode is enabled, the token also appears here so recovery can still be tested locally while the real delivery pipeline runs.
              </div>
              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                <button
                  onClick={() => void requestReset()}
                  disabled={busy || !loginIdentifier.trim()}
                  style={{
                    padding: '10px 14px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,190,70,0.3)',
                    background: busy || !loginIdentifier.trim()
                      ? 'rgba(120,90,35,0.28)'
                      : 'rgba(255,255,255,0.04)',
                    color: '#ffe9b1',
                    fontSize: '12px',
                    fontWeight: 800,
                    cursor: busy || !loginIdentifier.trim() ? 'not-allowed' : 'pointer',
                  }}
                >
                  Request Reset
                </button>
              </div>
              {resetPreview?.previewToken && (
                <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px', display: 'grid', gap: '4px' }}>
                  <div>Preview account: <span style={{ fontFamily: 'monospace' }}>{resetPreview.previewAccountId}</span></div>
                  <div>Preview token: <span style={{ fontFamily: 'monospace' }}>{resetPreview.previewToken}</span></div>
                  {resetPreview.expiresAt && <div>Expires {formatDateTime(resetPreview.expiresAt)}</div>}
                </div>
              )}
              <label style={{ display: 'grid', gap: '6px' }}>
                <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                  Reset account id
                </span>
                <input
                  value={resetAccountId}
                  onChange={(event) => setResetAccountId(event.target.value)}
                  placeholder="acct_..."
                  style={{
                    width: '100%',
                    padding: '12px 14px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.28)',
                    background: 'rgba(8,10,18,0.92)',
                    color: '#fff6df',
                    fontSize: '14px',
                    outline: 'none',
                  }}
                />
              </label>
              <label style={{ display: 'grid', gap: '6px' }}>
                <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                  Reset token
                </span>
                <input
                  value={resetToken}
                  onChange={(event) => setResetToken(event.target.value)}
                  placeholder="acctreset_..."
                  style={{
                    width: '100%',
                    padding: '12px 14px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.28)',
                    background: 'rgba(8,10,18,0.92)',
                    color: '#fff6df',
                    fontSize: '14px',
                    outline: 'none',
                  }}
                />
              </label>
              <label style={{ display: 'grid', gap: '6px' }}>
                <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                  New password
                </span>
                <input
                  type="password"
                  value={resetPassword}
                  onChange={(event) => setResetPassword(event.target.value)}
                  placeholder="new password"
                  style={{
                    width: '100%',
                    padding: '12px 14px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.28)',
                    background: 'rgba(8,10,18,0.92)',
                    color: '#fff6df',
                    fontSize: '14px',
                    outline: 'none',
                  }}
                />
              </label>
              <div>
                <button
                  onClick={() => void completeReset()}
                  disabled={busy || !resetAccountId.trim() || !resetToken.trim() || !resetPassword.trim()}
                  style={{
                    padding: '11px 16px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,190,70,0.38)',
                    background: busy || !resetAccountId.trim() || !resetToken.trim() || !resetPassword.trim()
                      ? 'rgba(120,90,35,0.28)'
                      : 'linear-gradient(180deg, rgba(206,110,40,0.9) 0%, rgba(150,66,10,0.96) 100%)',
                    color: '#fff7e3',
                    fontSize: '13px',
                    fontWeight: 800,
                    cursor: busy || !resetAccountId.trim() || !resetToken.trim() || !resetPassword.trim() ? 'not-allowed' : 'pointer',
                  }}
                >
                  Complete Reset
                </button>
              </div>
            </div>
          </div>

          <div style={{
            borderRadius: '14px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
            padding: '16px',
            display: 'grid',
            gap: '12px',
          }}>
            <div>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Recent auth emails</div>
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                This durable auth outbox tracks verification and password-reset delivery activity, including queued, delivered, retrying, and failed messages. You can still load the action link directly into this page or copy it for another device.
              </div>
            </div>
            {emailOutboxLoading ? (
              <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>Loading account email activity...</div>
            ) : emailOutboxError ? (
              <div style={{ color: '#ffd6d6', fontSize: '12px', fontWeight: 700 }}>{emailOutboxError}</div>
            ) : !accountSession ? (
              <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px' }}>
                Sign in on this seat to inspect the durable auth outbox for this account.
              </div>
            ) : (emailOutboxOverview?.deliveries.length ?? 0) === 0 ? (
              <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px' }}>
                No verification or reset emails have been queued for this account yet.
              </div>
              ) : (
                <div style={{ display: 'grid', gap: '10px' }}>
                  {(emailOutboxOverview?.deliveries ?? []).map((delivery) => (
                  <div
                    key={delivery.deliveryId}
                    style={{
                      borderRadius: '12px',
                      border: '1px solid rgba(255,180,60,0.12)',
                      background: 'rgba(255,255,255,0.022)',
                      padding: '12px 14px',
                      display: 'grid',
                      gap: '7px',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
                      <div style={{ color: '#fff4d6', fontSize: '13px', fontWeight: 800 }}>
                        {describeAccountEmailDeliveryKind(delivery.kind)}
                      </div>
                      <div style={{ color: delivery.status === 'failed' ? '#ffd3d3' : delivery.status === 'delivered' ? '#d7ffd4' : '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
                        {delivery.status}
                      </div>
                    </div>
                    <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px' }}>
                      {delivery.email} | {describeAccountEmailDeliveryStatus(delivery)}
                    </div>
                    <div style={{ color: 'rgba(244,232,200,0.82)', fontSize: '12px', fontWeight: 700 }}>
                      {delivery.subject}
                    </div>
                    <div style={{ color: 'rgba(244,232,200,0.56)', fontSize: '11px' }}>
                      Attempts {delivery.attemptCount}
                      {delivery.provider ? ` | provider ${delivery.provider}` : ''}
                      {delivery.providerMessageId ? ` | id ${delivery.providerMessageId}` : ''}
                    </div>
                    {delivery.failureReason && (
                      <div style={{ color: '#ffd6d6', fontSize: '11px', fontWeight: 700 }}>
                        Last failure: {delivery.failureReason}
                      </div>
                    )}
                    {delivery.actionUrl && (
                      <div style={{ color: 'rgba(244,232,200,0.58)', fontSize: '12px', fontFamily: 'monospace', wordBreak: 'break-all' }}>
                        {delivery.actionUrl}
                      </div>
                    )}
                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                      {delivery.actionUrl && (
                        <>
                          <button
                            onClick={() => loadEmailAction(delivery)}
                            style={{
                              padding: '8px 10px',
                              borderRadius: '999px',
                              border: '1px solid rgba(255,180,60,0.26)',
                              background: 'rgba(255,180,60,0.08)',
                              color: '#ffe9b1',
                              fontSize: '11px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Load Action
                          </button>
                          <button
                            onClick={() => void copyTextValue(delivery.actionUrl ?? '')}
                            style={{
                              padding: '8px 10px',
                              borderRadius: '999px',
                              border: '1px solid rgba(255,180,60,0.16)',
                              background: 'rgba(255,255,255,0.03)',
                              color: '#fff2c8',
                              fontSize: '11px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Copy Link
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div style={{
            borderRadius: '14px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
            padding: '16px',
            display: 'grid',
            gap: '12px',
          }}>
            <div>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Security activity</div>
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                This audit trail tracks account-sensitive actions like sign-in, verification, password recovery, and device session changes.
              </div>
            </div>
            {securityOverviewLoading ? (
              <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>Loading security activity...</div>
            ) : securityOverviewError ? (
              <div style={{ color: '#ffd6d6', fontSize: '12px', fontWeight: 700 }}>{securityOverviewError}</div>
            ) : !accountSession ? (
              <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px' }}>
                Sign in on this seat to inspect the account security activity feed.
              </div>
            ) : (securityOverview?.events.length ?? 0) === 0 ? (
              <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px' }}>
                No recorded security activity exists for this account yet.
              </div>
            ) : (
              <div style={{ display: 'grid', gap: '10px' }}>
                {(securityOverview?.events ?? []).map((event) => (
                  <div
                    key={event.eventId}
                    style={{
                      borderRadius: '12px',
                      border: '1px solid rgba(255,180,60,0.12)',
                      background: 'rgba(255,255,255,0.022)',
                      padding: '12px 14px',
                      display: 'grid',
                      gap: '6px',
                    }}
                  >
                    <div style={{ color: '#fff4d6', fontSize: '13px', fontWeight: 800 }}>
                      {describeAccountSecurityEvent(event.kind, event.detail)}
                    </div>
                    <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
                      {formatDateTime(event.createdAt)}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function MetaTile({ label, value }: { label: string; value: string }): React.ReactElement {
  return (
    <div style={{
      padding: '12px 14px',
      borderRadius: '12px',
      border: '1px solid rgba(255,180,60,0.12)',
      background: 'rgba(255,255,255,0.025)',
      minWidth: 0,
    }}>
      <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase', marginBottom: '6px' }}>{label}</div>
      <div style={{ color: '#fff3d1', fontSize: '13px', fontWeight: 700, wordBreak: 'break-word' }}>{value}</div>
    </div>
  );
}

function AccountSessionCard({
  label,
  accent,
  record,
  fallbackExpiresAt,
  fallbackSessionToken,
  actionLabel,
  onAction,
  busy = false,
  disabledAction = false,
}: {
  label: string;
  accent: string;
  record: AccountSessionRecord | null;
  fallbackExpiresAt?: string;
  fallbackSessionToken?: string;
  actionLabel: string;
  onAction?: () => void;
  busy?: boolean;
  disabledAction?: boolean;
}): React.ReactElement {
  const sessionToken = record?.sessionToken ?? fallbackSessionToken ?? '';
  const createdAt = record?.createdAt;
  const lastSeenAt = record?.lastSeenAt;
  const expiresAt = record?.expiresAt ?? fallbackExpiresAt;

  return (
    <div
      style={{
        borderRadius: '12px',
        border: `1px solid ${accent}`,
        background: 'rgba(255,255,255,0.022)',
        padding: '12px 14px',
        display: 'grid',
        gap: '8px',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <div style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>{label}</div>
        <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
          {describeSessionTokenFingerprint(sessionToken)}
        </div>
      </div>
      <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px' }}>
        Expires {formatDateTime(expiresAt)}
      </div>
      <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
        Last active {formatDateTime(lastSeenAt)}
        {createdAt ? ` · Created ${formatDateTime(createdAt)}` : ''}
      </div>
      <div>
        <button
          onClick={onAction}
          disabled={busy || disabledAction || !onAction}
          style={{
            padding: '8px 11px',
            borderRadius: '999px',
            border: '1px solid rgba(255,180,60,0.22)',
            background: disabledAction ? 'rgba(255,255,255,0.03)' : 'rgba(90,24,24,0.28)',
            color: disabledAction ? '#e9ddb8' : '#ffd7d7',
            fontSize: '11px',
            fontWeight: 800,
            cursor: busy || disabledAction || !onAction ? 'not-allowed' : 'pointer',
          }}
        >
          {actionLabel}
        </button>
      </div>
    </div>
  );
}

function RatingHistoryRow({ entry }: { entry: AccountRatingHistoryEntry }): React.ReactElement {
  const badgeColor =
    entry.result === 'win'
      ? 'rgba(88,180,126,0.22)'
      : entry.result === 'loss'
        ? 'rgba(220,80,80,0.22)'
        : 'rgba(140,160,190,0.18)';
  const badgeBorder =
    entry.result === 'win'
      ? 'rgba(88,180,126,0.35)'
      : entry.result === 'loss'
        ? 'rgba(220,80,80,0.35)'
        : 'rgba(160,176,204,0.26)';
  const deltaColor = entry.delta > 0 ? '#7dffb4' : entry.delta < 0 ? '#ff9d9d' : '#e5d9b7';

  return (
    <div
      style={{
        borderRadius: '12px',
        border: '1px solid rgba(255,180,60,0.12)',
        background: 'rgba(255,255,255,0.022)',
        padding: '12px 14px',
        display: 'grid',
        gap: '6px',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
          <span
            style={{
              padding: '4px 8px',
              borderRadius: '999px',
              border: `1px solid ${badgeBorder}`,
              background: badgeColor,
              color: '#fff4d6',
              fontSize: '11px',
              fontWeight: 800,
              letterSpacing: '0.8px',
            }}
          >
            {entry.result.toUpperCase()}
          </span>
          <span style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>
            {formatRatingDelta(entry.delta)} to {entry.ratingAfter}
          </span>
        </div>
        <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px' }}>
          {formatDateTime(entry.at)}
        </div>
      </div>
      <div style={{ color: 'rgba(244,232,200,0.74)', fontSize: '12px' }}>
        Started at {entry.ratingBefore}, now {entry.ratingAfter}, match #{entry.matchesPlayed}
      </div>
      <div style={{ color: deltaColor, fontSize: '12px', fontWeight: 700 }}>
        Match {entry.matchId}
        {entry.opponentAccountId ? ` - opponent ${entry.opponentAccountId}` : ''}
      </div>
    </div>
  );
}

function SeasonSummaryRow({ summary }: { summary: NonNullable<AccountProfile['currentSeason']> }): React.ReactElement {
  return (
    <div
      style={{
        borderRadius: '12px',
        border: '1px solid rgba(255,180,60,0.12)',
        background: 'rgba(255,255,255,0.022)',
        padding: '12px 14px',
        display: 'grid',
        gap: '6px',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <div style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>{summary.label}</div>
        <div style={{ color: '#7dffb4', fontSize: '12px', fontWeight: 700 }}>
          {formatRatingDelta(summary.netDelta)} to {summary.ratingEnd}
        </div>
      </div>
      <div style={{ color: 'rgba(244,232,200,0.74)', fontSize: '12px' }}>
        {summary.matchesPlayed} matches - {summary.wins}W {summary.losses}L {summary.draws}D - peak {summary.peakRating}
      </div>
      <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
        {formatDateTime(summary.startedAt)} to {formatDateTime(summary.lastPlayedAt)}
      </div>
    </div>
  );
}

export default function AccountPage({
  whiteProfile = null,
  blackProfile = null,
  externalNotice = null,
  onOpenProfile,
  onSeatAuthenticated,
  onAuthStateChange,
}: AccountPageProps): React.ReactElement {
  return (
    <div style={{
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: '24px 28px 30px',
      color: '#f4e8c8',
    }}>
      <div style={{ marginBottom: '22px' }}>
        <div style={{ color: '#ffb830', fontSize: '11px', fontWeight: 800, letterSpacing: '2px', textTransform: 'uppercase', marginBottom: '6px' }}>Account Layer</div>
        <h2 style={{ margin: 0, fontSize: '30px', color: '#fff4d6' }}>Accounts and identity</h2>
        <div style={{ color: 'rgba(222, 210, 180, 0.72)', fontSize: '13px', marginTop: '8px', maxWidth: '760px' }}>
          Chess404 still bridges through seat guest identity for live play, but this page now supports direct account creation, cross-device sign-in, verification, and recovery so the platform can move toward real account-first onboarding without breaking hosted matches.
        </div>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))',
        gap: '18px',
      }}>
        <AccountSeatPanel side="white" label="White Seat Account" accent="rgba(255,210,120,0.34)" guestProfile={whiteProfile} externalNotice={externalNotice} onOpenProfile={onOpenProfile} onSeatAuthenticated={onSeatAuthenticated} onAuthStateChange={onAuthStateChange} />
        <AccountSeatPanel side="black" label="Black Seat Account" accent="rgba(158,120,255,0.34)" guestProfile={blackProfile} onOpenProfile={onOpenProfile} onSeatAuthenticated={onSeatAuthenticated} onAuthStateChange={onAuthStateChange} />
      </div>
    </div>
  );
}
