/**
 * session-storage.ts
 *
 * Single source of truth for all localStorage key names and the read/write/clear
 * helpers for guest identity, account identity, and active match ID that are
 * consumed by both App.tsx and useMatchEngine.tsx.
 *
 * Also contains pure URL-query helpers and clipboard utilities that have no
 * dependency on React or the platform-service layer.
 */

// ── Storage key names ─────────────────────────────────────────────────────────

export const ACTIVE_MATCH_STORAGE_KEY = 'chess404.activeMatchId';

export const WHITE_GUEST_ID_STORAGE_KEY = 'chess404.guest.white';
export const BLACK_GUEST_ID_STORAGE_KEY = 'chess404.guest.black';
export const WHITE_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.white.secret';
export const BLACK_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.black.secret';
export const WHITE_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.white.token';
export const BLACK_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.black.token';
export const WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY = 'chess404.guest.white.token.expiresAt';
export const BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY = 'chess404.guest.black.token.expiresAt';

export const WHITE_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.white.id';
export const BLACK_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.black.id';
export const WHITE_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.white.token';
export const BLACK_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.black.token';
export const WHITE_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.white.expiresAt';
export const BLACK_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.black.expiresAt';

// ── Timing constants ──────────────────────────────────────────────────────────

export const CLAIM_REFRESH_CHECK_INTERVAL_MS = 30_000;
export const CLAIM_REFRESH_LEAD_MS = 5 * 60 * 1000;
export const MATCH_PRESENCE_HEARTBEAT_INTERVAL_MS = 10_000;

// ── User-facing message constants ─────────────────────────────────────────────

export const STREAM_RECONNECT_MESSAGE = 'Reconnecting to live match stream...';
export const PRESENCE_RETRY_MESSAGE = 'Live match presence sync is delayed.';

// ── Active match ID ───────────────────────────────────────────────────────────

export function readStoredActiveMatchId(): string | null {
  if (typeof window === 'undefined') return null;
  return window.localStorage.getItem(ACTIVE_MATCH_STORAGE_KEY);
}

export function writeStoredActiveMatchId(matchId: string | null): void {
  if (typeof window === 'undefined') return;
  if (matchId) {
    window.localStorage.setItem(ACTIVE_MATCH_STORAGE_KEY, matchId);
  } else {
    window.localStorage.removeItem(ACTIVE_MATCH_STORAGE_KEY);
  }
}

// ── Guest identity ────────────────────────────────────────────────────────────

export function readStoredGuestIdentity(side: 'white' | 'black'): {
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
  sessionExpiresAt?: string;
} {
  if (typeof window === 'undefined') return {};
  const guestId = window.localStorage.getItem(
    side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY,
  ) ?? undefined;
  const sessionSecret = window.localStorage.getItem(
    side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY,
  ) ?? undefined;
  const sessionToken = window.localStorage.getItem(
    side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY,
  ) ?? undefined;
  const sessionExpiresAt = window.localStorage.getItem(
    side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY,
  ) ?? undefined;
  return { guestId, sessionSecret, sessionToken, sessionExpiresAt };
}

export function writeStoredGuestIdentity(
  side: 'white' | 'black',
  guestId: string,
  sessionSecret: string,
  options: { sessionToken?: string | null; sessionExpiresAt?: string | null } = {},
): void {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(
    side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY,
    guestId,
  );
  if (sessionSecret.trim()) {
    window.localStorage.setItem(
      side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY,
      sessionSecret,
    );
  } else {
    window.localStorage.removeItem(
      side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY,
    );
  }
  if (options.sessionToken !== undefined) {
    if ((options.sessionToken ?? '').trim()) {
      window.localStorage.setItem(
        side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY,
        options.sessionToken ?? '',
      );
    } else {
      window.localStorage.removeItem(
        side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY,
      );
    }
  }
  if (options.sessionExpiresAt !== undefined) {
    if ((options.sessionExpiresAt ?? '').trim()) {
      window.localStorage.setItem(
        side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY,
        options.sessionExpiresAt ?? '',
      );
    } else {
      window.localStorage.removeItem(
        side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY,
      );
    }
  }
}

export function clearStoredGuestIdentity(side: 'white' | 'black'): void {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(
    side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY,
  );
  window.localStorage.removeItem(
    side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY,
  );
  window.localStorage.removeItem(
    side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY,
  );
  window.localStorage.removeItem(
    side === 'white' ? WHITE_GUEST_TOKEN_EXPIRY_STORAGE_KEY : BLACK_GUEST_TOKEN_EXPIRY_STORAGE_KEY,
  );
}

// ── Account identity ──────────────────────────────────────────────────────────

export function readStoredAccountIdentity(side: 'white' | 'black'): {
  accountId?: string;
  sessionToken?: string;
  expiresAt?: string;
} {
  if (typeof window === 'undefined') return {};
  return {
    accountId: window.localStorage.getItem(
      side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY,
    ) ?? undefined,
    sessionToken: window.localStorage.getItem(
      side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY,
    ) ?? undefined,
    expiresAt: window.localStorage.getItem(
      side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY,
    ) ?? undefined,
  };
}

export function writeStoredAccountIdentity(
  side: 'white' | 'black',
  account: { accountId: string },
  options: { sessionToken?: string | null; expiresAt?: string | null } = {},
): void {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(
    side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY,
    account.accountId,
  );
  if (options.sessionToken !== undefined) {
    if ((options.sessionToken ?? '').trim()) {
      window.localStorage.setItem(
        side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY,
        options.sessionToken ?? '',
      );
    } else {
      window.localStorage.removeItem(
        side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY,
      );
    }
  }
  if (options.expiresAt !== undefined) {
    if ((options.expiresAt ?? '').trim()) {
      window.localStorage.setItem(
        side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY,
        options.expiresAt ?? '',
      );
    } else {
      window.localStorage.removeItem(
        side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY,
      );
    }
  }
}

export function clearStoredAccountIdentity(side: 'white' | 'black'): void {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(
    side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY,
  );
  window.localStorage.removeItem(
    side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY,
  );
  window.localStorage.removeItem(
    side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY,
  );
}

// ── URL-query sync helpers ────────────────────────────────────────────────────

export function clearRequestedMatchQuery(): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (!url.searchParams.has('match')) return;
  url.searchParams.delete('match');
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

export function syncRequestedProfileQuery(handle: string | null): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (handle && handle.trim()) {
    url.searchParams.set('profile', handle.trim().toLowerCase());
  } else {
    url.searchParams.delete('profile');
  }
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

export function syncRequestedHistoryQuery(matchId: string | null, guestId: string | null): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (matchId && matchId.trim()) {
    url.searchParams.set('replay', matchId.trim());
  } else {
    url.searchParams.delete('replay');
  }
  if (guestId && guestId.trim()) {
    url.searchParams.set('guest', guestId.trim());
  } else {
    url.searchParams.delete('guest');
  }
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

export function syncRequestedMatchQuery(matchId: string | null): void {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (matchId && matchId.trim()) {
    url.searchParams.set('match', matchId.trim());
    url.searchParams.delete('replay');
    url.searchParams.delete('guest');
  } else {
    url.searchParams.delete('match');
  }
  window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);
}

// ── URL builders ──────────────────────────────────────────────────────────────

export function buildLiveMatchUrl(matchId: string): string | null {
  if (typeof window === 'undefined') return null;
  const normalizedMatchId = matchId.trim();
  if (!normalizedMatchId) return null;
  const url = new URL(window.location.href);
  url.searchParams.set('match', normalizedMatchId);
  url.searchParams.delete('replay');
  url.searchParams.delete('guest');
  url.searchParams.delete('profile');
  return `${url.origin}${url.pathname}${url.search}${url.hash}`;
}

export function buildReplayPageUrl(matchId: string): string | null {
  if (typeof window === 'undefined') return null;
  const normalizedMatchId = matchId.trim();
  if (!normalizedMatchId) return null;
  const url = new URL(window.location.href);
  url.searchParams.set('replay', normalizedMatchId);
  url.searchParams.delete('match');
  url.searchParams.delete('guest');
  url.searchParams.delete('profile');
  return `${url.origin}${url.pathname}${url.search}${url.hash}`;
}

// ── Clipboard ─────────────────────────────────────────────────────────────────

export async function copyTextToClipboard(value: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return true;
  }
  return false;
}

// ── Guest history URL helpers ─────────────────────────────────────────────────

/** Like buildReplayPageUrl but also sets/clears a guest query param. */
export function buildReplayPageUrlWithGuest(
  matchId: string,
  guestId?: string | null,
): string | null {
  if (typeof window === 'undefined') return null;
  const normalizedMatchId = matchId.trim();
  if (!normalizedMatchId) return null;
  const url = new URL(window.location.href);
  url.searchParams.delete('match');
  url.searchParams.delete('profile');
  url.searchParams.set('replay', normalizedMatchId);
  if (guestId?.trim()) {
    url.searchParams.set('guest', guestId.trim());
  } else {
    url.searchParams.delete('guest');
  }
  return `${url.origin}${url.pathname}${url.search}${url.hash}`;
}

/** Builds a URL pointing at the guest's full history by guestId. */
export function buildGuestHistoryUrl(guestId: string): string | null {
  if (typeof window === 'undefined') return null;
  const normalizedGuestId = guestId.trim();
  if (!normalizedGuestId) return null;
  const url = new URL(window.location.href);
  url.searchParams.delete('match');
  url.searchParams.delete('profile');
  url.searchParams.delete('replay');
  url.searchParams.set('guest', normalizedGuestId);
  return `${url.origin}${url.pathname}${url.search}${url.hash}`;
}
