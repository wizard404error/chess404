'use client';

import React from 'react';
import type { PieceColor } from '../types';
import type { SocialAlert } from '../lib/match-labels';
import type { GuestProfile } from '../lib/platform-service';
import type {
  AccountSession as PlatformAccountSession,
  GuestSession as PlatformGuestSession,
  MatchSeatClaim,
} from '../lib/platform-service';
import type { QueueTicket } from '../lib/matchmaking-service';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import type { StoredRoomMeta } from '../lib/match-service';
import {
  readStoredGuestIdentity,
  writeStoredGuestIdentity,
  clearStoredGuestIdentity,
  readStoredAccountIdentity,
  writeStoredAccountIdentity,
  clearStoredAccountIdentity,
  readStoredActiveMatchId,
  writeStoredActiveMatchId,
  clearRequestedMatchQuery,
} from '../lib/session-storage';
import {
  readStoredRoomMeta,
  writeStoredRoomMeta,
} from '../lib/match-service';
import {
  createGuestSession,
  connectAccountNotificationStream,
  fetchDirectChallengeOverview,
  fetchFriendOverview,
  fetchAccountNotificationOverview,
  isAccountRestrictionError,
  formatAccountRestrictionNotice,
  parseAccountRestrictionMessage,
  touchAccountPresence,
} from '../lib/platform-service';
import { fetchGatewayBootstrap } from '../lib/system-service';
import { resolveSeatSecret } from '../lib/match-service';
import { buildSocialAlert } from '../lib/match-labels';

export interface UsePlatformStateProps {
  hostedRuntime: boolean | null;
  setHostedRuntime: React.Dispatch<React.SetStateAction<boolean | null>>;
  activePage: any;
  setActivePage: React.Dispatch<React.SetStateAction<any>>;
  setAccountActionQueryDetected: React.Dispatch<React.SetStateAction<boolean>>;
  setHistoryFocusMatchId: React.Dispatch<React.SetStateAction<string | null>>;
  setHistoryFocusGuestId: React.Dispatch<React.SetStateAction<string | null>>;
  setProfileFocusHandle: React.Dispatch<React.SetStateAction<string | null>>;
  setProfileQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setHistoryQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setMatchQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setFriendsAttentionCount: React.Dispatch<React.SetStateAction<number>>;
  setInboxUnreadCount: React.Dispatch<React.SetStateAction<number>>;
  setSocialAlert: React.Dispatch<React.SetStateAction<SocialAlert | null>>;
  socialAlert: SocialAlert | null;
  setSocialLiveToken: React.Dispatch<React.SetStateAction<number>>;
  socialLiveToken: number;
  setWhiteProfile: React.Dispatch<React.SetStateAction<GuestProfile | null>>;
  setBlackProfile: React.Dispatch<React.SetStateAction<GuestProfile | null>>;
  setViewerSeat: React.Dispatch<React.SetStateAction<PieceColor | null>>;
  viewerSeat: PieceColor | null;
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
  setGuestProfilesReady: React.Dispatch<React.SetStateAction<boolean>>;
  guestProfilesReady: boolean;
  setBootstrapQueueRecovery: React.Dispatch<React.SetStateAction<{ white: QueueTicket | null; black: QueueTicket | null } | null>>;
  setMatchSeatMeta: React.Dispatch<React.SetStateAction<{ whiteGuestId?: string; blackGuestId?: string; whiteName?: string; blackName?: string } | null>>;
  setMatchDestinationNotice: React.Dispatch<React.SetStateAction<string>>;
  openedBoardMatchRef: React.MutableRefObject<string | null>;
  pathname: string;
  profileFocusHandle: string | null;
  historyFocusGuestId: string | null;
  historyFocusMatchId: string | null;
}

export function usePlatformState(props: UsePlatformStateProps) {
  const {
    hostedRuntime,
    setHostedRuntime,
    activePage,
    setActivePage,
    setAccountActionQueryDetected,
    setHistoryFocusMatchId,
    setHistoryFocusGuestId,
    setProfileFocusHandle,
    setProfileQueryReady,
    setHistoryQueryReady,
    setMatchQueryReady,
    setFriendsAttentionCount,
    setInboxUnreadCount,
    setSocialAlert,
    socialAlert,
    setSocialLiveToken,
    socialLiveToken,
    setWhiteProfile,
    setBlackProfile,
    setViewerSeat,
    viewerSeat,
    whiteProfile,
    blackProfile,
    setGuestProfilesReady,
    guestProfilesReady,
    setBootstrapQueueRecovery,
    setMatchSeatMeta,
    setMatchDestinationNotice,
    openedBoardMatchRef,
    pathname,
    profileFocusHandle,
    historyFocusGuestId,
    historyFocusMatchId,
  } = props;

  // ── Identity state ──────────────────────────────────────────────────────────
  const dismissedSocialAlertIdsRef = React.useRef<Set<string>>(new Set());
  const [primaryAccountIdentity, setPrimaryAccountIdentity] = React.useState(() => readStoredAccountIdentity('white'));
  const [shellAccountNotice, setShellAccountNotice] = React.useState('');

  const syncPrimaryAccountIdentity = React.useCallback(() => {
    React.startTransition(() => {
      setPrimaryAccountIdentity(readStoredAccountIdentity('white'));
    });
  }, []);

  const clearPrimaryAccountRestriction = React.useCallback((message: string) => {
    clearStoredAccountIdentity('white');
    React.startTransition(() => {
      setPrimaryAccountIdentity({});
      setShellAccountNotice(message);
      setFriendsAttentionCount(0);
      setInboxUnreadCount(0);
      setSocialAlert(null);
      if (hostedRuntime) {
        setActivePage('Account');
      }
    });
  }, [hostedRuntime]);

  const pulseSocialLive = React.useCallback(() => {
    React.startTransition(() => {
      setSocialLiveToken((current: number) => current + 1);
    });
  }, []);

  // ── Platform refs ───────────────────────────────────────────────────────────
  const whiteProfileRef = React.useRef<GuestProfile | null>(null);
  const blackProfileRef = React.useRef<GuestProfile | null>(null);
  const viewerSeatRef = React.useRef<PieceColor | null>(null);
  const guestSessionSecretsRef = React.useRef<{ white: string | null; black: string | null }>({ white: null, black: null });
  const authoritativeSeatIdsRef = React.useRef<{ white: string | null; black: string | null }>({ white: null, black: null });
  const authoritativeSeatSecretsRef = React.useRef<{ white: string | null; black: string | null }>({ white: null, black: null });
  const authoritativeClaimExpiresAtRef = React.useRef<{ white: string | null; black: string | null }>({ white: null, black: null });
  const authoritativeClaimTokensRef = React.useRef<{ white: string | null; black: string | null }>({ white: null, black: null });
  const [intentInFlight, setIntentInFlight] = React.useState(false);
  const gatewayRecoveredMatchIdRef = React.useRef<string | null>(null);
  const gatewayBootstrapClaimsRef = React.useRef<{
    matchId: string | null;
    whiteSecret: string | null;
    blackSecret: string | null;
    whiteToken: string | null;
    blackToken: string | null;
    whiteExpiresAt: string | null;
    blackExpiresAt: string | null;
  }>({
    matchId: null,
    whiteSecret: null,
    blackSecret: null,
    whiteToken: null,
    blackToken: null,
    whiteExpiresAt: null,
    blackExpiresAt: null,
  });

  const authoritativeMatchIdRef = React.useRef<string | null>(null);
  const requestedMatchIdRef = React.useRef<string | null>(null);

  // ── Authentication callbacks ────────────────────────────────────────────────
  const handleSeatAuthenticated = React.useCallback((side: 'white' | 'black', guestSession: PlatformGuestSession, accountSession: PlatformAccountSession) => {
    guestSessionSecretsRef.current[side] = guestSession.sessionSecret;
    writeStoredGuestIdentity(side, guestSession.guest.guestId, guestSession.sessionSecret, {
      sessionToken: guestSession.sessionToken ?? null,
      sessionExpiresAt: guestSession.expiresAt ?? null,
    });
    writeStoredAccountIdentity(side, { accountId: accountSession.account.accountId }, {
      sessionToken: accountSession.sessionToken,
      expiresAt: accountSession.expiresAt,
    });
    if (side === 'white') {
      syncPrimaryAccountIdentity();
      setShellAccountNotice('');
    }
    if (side === 'white') {
      setWhiteProfile(guestSession.guest);
      if (hostedRuntime) {
        setViewerSeat('white');
      }
    } else {
      setBlackProfile(guestSession.guest);
    }
    setGuestProfilesReady(true);
  }, [hostedRuntime, syncPrimaryAccountIdentity]);

  const handlePrimaryShellAuthenticated = React.useCallback((guestSession: PlatformGuestSession, accountSession: PlatformAccountSession) => {
    handleSeatAuthenticated('white', guestSession, accountSession);
    setActivePage('Play');
  }, [handleSeatAuthenticated, hostedRuntime]);

  // ── Gateway helper functions ────────────────────────────────────────────────
  const applyGatewayGuestSessions = React.useCallback((guestSessions?: {
    white?: { guest: GuestProfile; sessionSecret: string; sessionToken?: string; expiresAt?: string };
    black?: { guest: GuestProfile; sessionSecret: string; sessionToken?: string; expiresAt?: string };
  }) => {
    if (guestSessions?.white) {
      guestSessionSecretsRef.current.white = guestSessions.white.sessionSecret;
      writeStoredGuestIdentity('white', guestSessions.white.guest.guestId, guestSessions.white.sessionSecret, {
        sessionToken: guestSessions.white.sessionToken ?? null,
        sessionExpiresAt: guestSessions.white.expiresAt ?? null,
      });
      setWhiteProfile(guestSessions.white.guest);
    }
    if (guestSessions?.black) {
      if (hostedRuntime) {
        setBlackProfile((current: any) => current ?? guestSessions.black?.guest ?? null);
      } else {
        guestSessionSecretsRef.current.black = guestSessions.black.sessionSecret;
        writeStoredGuestIdentity('black', guestSessions.black.guest.guestId, guestSessions.black.sessionSecret, {
          sessionToken: guestSessions.black.sessionToken ?? null,
          sessionExpiresAt: guestSessions.black.expiresAt ?? null,
        });
        setBlackProfile(guestSessions.black.guest);
      }
    }
  }, [hostedRuntime]);

  const applyGatewayAccountSessions = React.useCallback((accountSessions?: {
    white?: { account: { accountId: string }; sessionToken: string; expiresAt?: string };
    black?: { account: { accountId: string }; sessionToken: string; expiresAt?: string };
  }) => {
    if (accountSessions?.white) {
      writeStoredAccountIdentity('white', accountSessions.white.account, {
        sessionToken: accountSessions.white.sessionToken,
        expiresAt: accountSessions.white.expiresAt ?? null,
      });
      setShellAccountNotice('');
      syncPrimaryAccountIdentity();
    }
    if (accountSessions?.black) {
      if (!hostedRuntime) {
        writeStoredAccountIdentity('black', accountSessions.black.account, {
          sessionToken: accountSessions.black.sessionToken,
          expiresAt: accountSessions.black.expiresAt ?? null,
        });
      }
    }
  }, [hostedRuntime, syncPrimaryAccountIdentity]);

  const buildGatewayBootstrapRequest = React.useCallback((matchId?: string | null) => ({
    matchId: matchId ?? undefined,
    white: readStoredGuestIdentity('white'),
    black: readStoredGuestIdentity('black'),
    whiteAccount: readStoredAccountIdentity('white'),
    blackAccount: readStoredAccountIdentity('black'),
  }), []);

  const applyGatewayMatchClaims = React.useCallback((matchId: string | null | undefined, matchClaims?: {
    white?: MatchSeatClaim;
    black?: MatchSeatClaim;
  } | null) => {
    if (!matchId || !matchClaims) {
      return;
    }

    const storedRoomMeta = readStoredRoomMeta(matchId);
    const whiteClaim = [matchClaims.white, matchClaims.black].find(claim => claim?.seatColor === 'white');
    const blackClaim = [matchClaims.white, matchClaims.black].find(claim => claim?.seatColor === 'black');
    const whiteIdentity = readStoredGuestIdentity('white');
    const blackIdentity = readStoredGuestIdentity('black');
    const ownedClaim =
      (whiteClaim && whiteClaim.guestId === whiteIdentity.guestId ? whiteClaim : null) ??
      (blackClaim && blackClaim.guestId === blackIdentity.guestId ? blackClaim : null) ??
      whiteClaim ?? blackClaim ?? null;
    const isCurrentMatch = authoritativeMatchIdRef.current === matchId;
    const currentBootstrapClaims = gatewayBootstrapClaimsRef.current.matchId === matchId
      ? gatewayBootstrapClaimsRef.current
      : null;

    const nextWhiteSecret =
      whiteClaim?.playerSecret ??
      storedRoomMeta?.whitePlayerSecret ??
      currentBootstrapClaims?.whiteSecret ??
      (isCurrentMatch ? authoritativeSeatSecretsRef.current.white : null);
    const nextBlackSecret =
      blackClaim?.playerSecret ??
      storedRoomMeta?.blackPlayerSecret ??
      currentBootstrapClaims?.blackSecret ??
      (isCurrentMatch ? authoritativeSeatSecretsRef.current.black : null);
    const nextWhiteToken =
      whiteClaim?.claimToken ??
      storedRoomMeta?.whiteClaimToken ??
      currentBootstrapClaims?.whiteToken ??
      (isCurrentMatch ? authoritativeClaimTokensRef.current.white : null);
    const nextBlackToken =
      blackClaim?.claimToken ??
      storedRoomMeta?.blackClaimToken ??
      currentBootstrapClaims?.blackToken ??
      (isCurrentMatch ? authoritativeClaimTokensRef.current.black : null);
    const nextWhiteExpiresAt =
      whiteClaim?.expiresAt ??
      storedRoomMeta?.whiteClaimExpiresAt ??
      currentBootstrapClaims?.whiteExpiresAt ??
      (isCurrentMatch ? authoritativeClaimExpiresAtRef.current.white : null);
    const nextBlackExpiresAt =
      blackClaim?.expiresAt ??
      storedRoomMeta?.blackClaimExpiresAt ??
      currentBootstrapClaims?.blackExpiresAt ??
      (isCurrentMatch ? authoritativeClaimExpiresAtRef.current.black : null);

    authoritativeSeatSecretsRef.current = {
      white: nextWhiteSecret ?? null,
      black: nextBlackSecret ?? null,
    };
    authoritativeClaimTokensRef.current = {
      white: nextWhiteToken ?? null,
      black: nextBlackToken ?? null,
    };
    authoritativeClaimExpiresAtRef.current = {
      white: nextWhiteExpiresAt ?? null,
      black: nextBlackExpiresAt ?? null,
    };
    gatewayBootstrapClaimsRef.current = {
      matchId,
      whiteSecret: nextWhiteSecret ?? null,
      blackSecret: nextBlackSecret ?? null,
      whiteToken: nextWhiteToken ?? null,
      blackToken: nextBlackToken ?? null,
      whiteExpiresAt: nextWhiteExpiresAt ?? null,
      blackExpiresAt: nextBlackExpiresAt ?? null,
    };
    if (ownedClaim?.seatColor) {
      setViewerSeat(ownedClaim.seatColor);
    }

    const nextRoomMeta: StoredRoomMeta = {
      ...storedRoomMeta,
      viewerSeat: ownedClaim?.seatColor ?? storedRoomMeta?.viewerSeat ?? null,
    };

    if (!hostedRuntime || (ownedClaim?.seatColor ?? storedRoomMeta?.viewerSeat) === 'white') {
      nextRoomMeta.whitePlayerSecret = nextWhiteSecret ?? storedRoomMeta?.whitePlayerSecret;
      nextRoomMeta.whiteClaimToken = nextWhiteToken ?? storedRoomMeta?.whiteClaimToken;
      nextRoomMeta.whiteClaimExpiresAt = nextWhiteExpiresAt ?? storedRoomMeta?.whiteClaimExpiresAt;
    } else {
      delete nextRoomMeta.whitePlayerSecret;
      delete nextRoomMeta.whiteClaimToken;
      delete nextRoomMeta.whiteClaimExpiresAt;
    }

    if (!hostedRuntime || (ownedClaim?.seatColor ?? storedRoomMeta?.viewerSeat) === 'black') {
      nextRoomMeta.blackPlayerSecret = nextBlackSecret ?? storedRoomMeta?.blackPlayerSecret;
      nextRoomMeta.blackClaimToken = nextBlackToken ?? storedRoomMeta?.blackClaimToken;
      nextRoomMeta.blackClaimExpiresAt = nextBlackExpiresAt ?? storedRoomMeta?.blackClaimExpiresAt;
    } else {
      delete nextRoomMeta.blackPlayerSecret;
      delete nextRoomMeta.blackClaimToken;
      delete nextRoomMeta.blackClaimExpiresAt;
    }

    writeStoredRoomMeta(matchId, nextRoomMeta);
  }, [hostedRuntime]);

  const applyGatewayRecoveredMatch = React.useCallback((input?: {
    matchId: string;
    queue?: string;
    modeId?: MatchModeId;
    viewerSeat?: PieceColor | null;
    whiteGuestId?: string;
    blackGuestId?: string;
    whiteName?: string;
    blackName?: string;
    claims?: {
      white?: MatchSeatClaim;
      black?: MatchSeatClaim;
    };
  } | null) => {
    if (!input?.matchId) {
      return;
    }

    const storedRoomMeta = readStoredRoomMeta(input.matchId);
    const nextRoomMeta: StoredRoomMeta = {
      ...storedRoomMeta,
      queue: input.queue === 'rated' || input.queue === 'casual' || input.queue === 'direct'
        ? input.queue
        : storedRoomMeta?.queue,
      modeId: input.modeId ?? storedRoomMeta?.modeId ?? DEFAULT_MATCH_MODE_ID,
      viewerSeat: input.viewerSeat ?? storedRoomMeta?.viewerSeat ?? null,
      whiteGuestId: input.whiteGuestId ?? storedRoomMeta?.whiteGuestId,
      blackGuestId: input.blackGuestId ?? storedRoomMeta?.blackGuestId,
      whiteName: input.whiteName ?? storedRoomMeta?.whiteName,
      blackName: input.blackName ?? storedRoomMeta?.blackName,
    };

    writeStoredActiveMatchId(input.matchId);
    writeStoredRoomMeta(input.matchId, nextRoomMeta);
    gatewayRecoveredMatchIdRef.current = input.matchId;
    if (input.viewerSeat) {
      setViewerSeat(input.viewerSeat);
    }
    if (input.claims) {
      applyGatewayMatchClaims(input.matchId, input.claims);
    }
  }, [applyGatewayMatchClaims]);

  const applyGatewayQueueRecovery = React.useCallback((input: {
    queueTickets?: {
      white?: QueueTicket;
      black?: QueueTicket;
    };
    recoveredMatch?: {
      matchId: string;
      queue?: string;
      modeId?: MatchModeId;
      viewerSeat?: PieceColor | null;
      whiteGuestId?: string;
      blackGuestId?: string;
      whiteName?: string;
      blackName?: string;
      claims?: {
        white?: MatchSeatClaim;
        black?: MatchSeatClaim;
      };
    } | null;
  } | undefined, options: { hosted: boolean; requestedMatchId?: string | null }) => {
    if (!input) {
      return;
    }

    if (input.recoveredMatch?.matchId) {
      applyGatewayRecoveredMatch(input.recoveredMatch);
      return;
    }

    if (options.hosted && !options.requestedMatchId) {
      gatewayRecoveredMatchIdRef.current = null;
      writeStoredActiveMatchId(null);
    }
  }, [applyGatewayRecoveredMatch, setBootstrapQueueRecovery]);

  // ── Effects ─────────────────────────────────────────────────────────────────

  // Initialization: detect hosted runtime
  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const hostname = window.location.hostname.toLowerCase();
    const nextHosted = hostname !== 'localhost' && hostname !== '127.0.0.1';
    setHostedRuntime(nextHosted);
    if (nextHosted) {
      setActivePage((current: any) => {
        if (current !== 'Play') {
          return current;
        }
        const identity = readStoredAccountIdentity('white');
        return identity.accountId && identity.sessionToken ? 'Play' : 'Account';
      });
    }
  }, []);

  // Clear black identity on hosted runtime
  React.useEffect(() => {
    if (!hostedRuntime) {
      return;
    }
    clearStoredGuestIdentity('black');
    clearStoredAccountIdentity('black');
    guestSessionSecretsRef.current.black = null;
    authoritativeSeatSecretsRef.current.black = null;
    authoritativeClaimTokensRef.current.black = null;
    authoritativeClaimExpiresAtRef.current.black = null;
    setBlackProfile(null);
  }, [hostedRuntime]);

  // Social live pulse interval
  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    let cancelled = false;

    const emitRefresh = () => {
      if (cancelled || document.visibilityState === 'hidden') {
        return;
      }
      pulseSocialLive();
    };

    const handleWake = () => {
      if (document.visibilityState === 'visible') {
        emitRefresh();
      }
    };

    emitRefresh();
    const intervalId = window.setInterval(() => {
      emitRefresh();
    }, activePage === 'Friends' || activePage === 'Inbox' ? 15_000 : 45_000);

    window.addEventListener('focus', handleWake);
    document.addEventListener('visibilitychange', handleWake);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
      window.removeEventListener('focus', handleWake);
      document.removeEventListener('visibilitychange', handleWake);
    };
  }, [activePage, pulseSocialLive]);

  // Account notification stream
  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!primaryAccountIdentity.accountId || !primaryAccountIdentity.sessionToken) {
      return;
    }

    return connectAccountNotificationStream({
      accountId: primaryAccountIdentity.accountId,
      sessionToken: primaryAccountIdentity.sessionToken,
      onEvent: () => {
        pulseSocialLive();
      },
    });
  }, [primaryAccountIdentity.accountId, primaryAccountIdentity.sessionToken, pulseSocialLive]);

  // Presence heartbeat
  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    let cancelled = false;
    let heartbeatInFlight = false;

    const sendPresenceHeartbeat = async () => {
      if (cancelled || heartbeatInFlight || document.visibilityState === 'hidden') {
        return;
      }
      const identity = readStoredAccountIdentity('white');
      if (!identity.accountId || !identity.sessionToken) {
        return;
      }

      heartbeatInFlight = true;
      try {
        const session = await touchAccountPresence({
          accountId: identity.accountId,
          sessionToken: identity.sessionToken,
        });
        if (cancelled) {
          return;
        }
        writeStoredAccountIdentity('white', session.account, {
          sessionToken: session.sessionToken,
          expiresAt: session.expiresAt,
        });
        syncPrimaryAccountIdentity();
      } catch (err) {
        if (isAccountRestrictionError(err)) {
          clearPrimaryAccountRestriction(formatAccountRestrictionNotice(err.restriction));
        }
      } finally {
        heartbeatInFlight = false;
      }
    };

    const handleWake = () => {
      void sendPresenceHeartbeat();
    };

    void sendPresenceHeartbeat();
    const intervalId = window.setInterval(() => {
      void sendPresenceHeartbeat();
    }, 45_000);
    window.addEventListener('focus', handleWake);
    document.addEventListener('visibilitychange', handleWake);

    return () => {
      cancelled = true;
      heartbeatInFlight = false;
      window.clearInterval(intervalId);
      window.removeEventListener('focus', handleWake);
      document.removeEventListener('visibilitychange', handleWake);
    };
  }, [clearPrimaryAccountRestriction, syncPrimaryAccountIdentity]);

  // Social signals refresh
  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    let cancelled = false;

    const refreshSocialSignals = async () => {
      if (cancelled || document.visibilityState === 'hidden') {
        return;
      }
      const identity = readStoredAccountIdentity('white');
      if (!identity.accountId || !identity.sessionToken) {
        setFriendsAttentionCount(0);
        setInboxUnreadCount(0);
        setSocialAlert(null);
        return;
      }

      try {
        const [notificationOverview, friendOverview, challengeOverview]: [
          Awaited<ReturnType<typeof fetchAccountNotificationOverview>>,
          Awaited<ReturnType<typeof fetchFriendOverview>>,
          Awaited<ReturnType<typeof fetchDirectChallengeOverview>>,
        ] = await Promise.all([
          fetchAccountNotificationOverview({
            accountId: identity.accountId,
            sessionToken: identity.sessionToken,
            limit: 12,
          }),
          fetchFriendOverview({
            accountId: identity.accountId,
            sessionToken: identity.sessionToken,
          }),
          fetchDirectChallengeOverview({
            accountId: identity.accountId,
            sessionToken: identity.sessionToken,
          }),
        ]);
        if (!cancelled) {
          setInboxUnreadCount(notificationOverview.unreadCount);
          setFriendsAttentionCount(friendOverview.incoming.length + challengeOverview.incoming.length);
          const nextAlert = notificationOverview.notifications
            .filter((notification) => !notification.readAt)
            .map(buildSocialAlert)
            .find((candidate) => {
              if (!candidate) {
                return false;
              }
              if (dismissedSocialAlertIdsRef.current.has(candidate.id)) {
                return false;
              }
              if (candidate.action === 'match' && candidate.matchId && candidate.matchId === authoritativeMatchIdRef.current) {
                return false;
              }
              return true;
            }) ?? null;
          setSocialAlert(nextAlert);
        }
      } catch (err) {
        if (!cancelled) {
          setFriendsAttentionCount(0);
          setInboxUnreadCount(0);
          setSocialAlert(null);
          if (isAccountRestrictionError(err)) {
            clearPrimaryAccountRestriction(formatAccountRestrictionNotice(err.restriction));
          }
        }
      }
    };

    void refreshSocialSignals();

    return () => {
      cancelled = true;
    };
  }, [clearPrimaryAccountRestriction, socialLiveToken]);

  // Initial bootstrap fetch
  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const hostname = window.location.hostname.toLowerCase();
    const nextHosted = hostname !== 'localhost' && hostname !== '127.0.0.1';
    const pathMatch = window.location.pathname.match(/^\/match\/([^/]+)$/);
    const requestedPathMatchId = pathMatch?.[1] ? decodeURIComponent(pathMatch[1]) : null;
    const query = new URLSearchParams(window.location.search);
    const requestedMatchId = requestedPathMatchId ?? query.get('match');
    const requestedReplayMatchId = query.get('replay');
    const requestedGuestId = query.get('guest');
    const requestedProfileHandle = query.get('profile');
    const requestedAuthAction = query.get('auth');
    const requestedAuthToken = query.get('token');
    const requestedAuthLink =
      (requestedAuthAction === 'verify-email' || requestedAuthAction === 'reset-password') &&
      (requestedAuthToken?.trim() ?? '') !== '';
    setAccountActionQueryDetected(requestedAuthLink);
    requestedMatchIdRef.current = requestedMatchId ?? (nextHosted ? null : readStoredActiveMatchId());
    if (requestedAuthLink) {
      setActivePage('Account');
    } else if (requestedMatchId?.trim()) {
      setActivePage(nextHosted || requestedPathMatchId ? 'Match' : 'Play');
    } else if (!requestedMatchId && ((requestedReplayMatchId?.trim() ?? '') || (requestedGuestId?.trim() ?? ''))) {
      setHistoryFocusMatchId(requestedReplayMatchId?.trim() ? requestedReplayMatchId.trim() : null);
      setHistoryFocusGuestId(requestedGuestId?.trim() ? requestedGuestId.trim() : null);
      setActivePage('History');
    } else if (!requestedMatchId && requestedProfileHandle?.trim()) {
      setProfileFocusHandle(requestedProfileHandle.trim().toLowerCase());
      setActivePage('Profiles');
    }
    setProfileQueryReady(true);
    setHistoryQueryReady(true);
    setMatchQueryReady(true);

    let cancelled = false;

    void fetchGatewayBootstrap({
      matchId: requestedMatchIdRef.current ?? undefined,
      white: readStoredGuestIdentity('white'),
      black: readStoredGuestIdentity('black'),
      whiteAccount: readStoredAccountIdentity('white'),
      blackAccount: readStoredAccountIdentity('black'),
    })
      .then(bootstrap => {
        if (cancelled) return;
        const bootstrapRestriction = parseAccountRestrictionMessage(bootstrap.accountErrors?.white);
        if (nextHosted && bootstrapRestriction) {
          clearPrimaryAccountRestriction(formatAccountRestrictionNotice(bootstrapRestriction));
          return;
        }
        applyGatewayGuestSessions(bootstrap.guestSessions);
        applyGatewayMatchClaims(bootstrap.requestedMatchId ?? requestedMatchIdRef.current, bootstrap.matchClaims);
        applyGatewayAccountSessions(bootstrap.accountSessions);
        applyGatewayQueueRecovery({
          queueTickets: bootstrap.queueTickets,
          recoveredMatch: bootstrap.recoveredMatch ?? null,
        }, {
          hosted: nextHosted,
          requestedMatchId: requestedMatchIdRef.current,
        });
      })
      .catch(() => {
        // Keep fallback labels if the platform or gateway service is unavailable.
      })
      .finally(() => {
        if (!cancelled) {
          setGuestProfilesReady(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [applyGatewayGuestSessions, applyGatewayMatchClaims, applyGatewayAccountSessions, applyGatewayQueueRecovery, clearPrimaryAccountRestriction]);

  // White profile ref sync
  React.useEffect(() => {
    whiteProfileRef.current = whiteProfile;
  }, [whiteProfile]);

  // Black profile ref sync
  React.useEffect(() => {
    blackProfileRef.current = blackProfile;
  }, [blackProfile]);

  // Viewer seat ref sync
  React.useEffect(() => {
    viewerSeatRef.current = viewerSeat;
  }, [viewerSeat]);

  // Ensure guest sessions after profiles ready
  React.useEffect(() => {
    if (!guestProfilesReady) return;

    let cancelled = false;

    const ensureGuestSeat = async (side: 'white' | 'black') => {
      const stored = readStoredGuestIdentity(side);
      const session = await createGuestSession({
        guestId: stored.guestId,
        sessionSecret: stored.sessionSecret,
        sessionToken: stored.sessionToken,
      });
      if (cancelled) return;
      guestSessionSecretsRef.current[side] = session.sessionSecret;
      writeStoredGuestIdentity(side, session.guest.guestId, session.sessionSecret, {
        sessionToken: session.sessionToken ?? null,
        sessionExpiresAt: session.expiresAt ?? null,
      });
      if (side === 'white') {
        setWhiteProfile(session.guest);
      } else {
        setBlackProfile(session.guest);
      }
    };

    const tasks: Promise<void>[] = [];
    if (!whiteProfile) {
      tasks.push(ensureGuestSeat('white'));
    }
    if (!hostedRuntime && !blackProfile) {
      tasks.push(ensureGuestSeat('black'));
    }
    if (tasks.length === 0) {
      return;
    }
    void Promise.all(tasks).catch(() => {
      // Keep the UI usable; queue page will surface join failures if platform calls still fail.
    });

    return () => {
      cancelled = true;
    };
  }, [guestProfilesReady, hostedRuntime, whiteProfile, blackProfile]);

  // ── Return ──────────────────────────────────────────────────────────────────
  return {
    // State
    primaryAccountIdentity,
    setPrimaryAccountIdentity,
    shellAccountNotice,
    setShellAccountNotice,

    // Refs
    whiteProfileRef,
    blackProfileRef,
    viewerSeatRef,
    guestSessionSecretsRef,
    authoritativeSeatIdsRef,
    authoritativeSeatSecretsRef,
    authoritativeClaimExpiresAtRef,
    authoritativeClaimTokensRef,
    gatewayBootstrapClaimsRef,
    gatewayRecoveredMatchIdRef,
    requestedMatchIdRef,
    authoritativeMatchIdRef,
    dismissedSocialAlertIdsRef,
    intentInFlight,
    setIntentInFlight,

    // Callbacks
    syncPrimaryAccountIdentity,
    clearPrimaryAccountRestriction,
    pulseSocialLive,
    handleSeatAuthenticated,
    handlePrimaryShellAuthenticated,
    applyGatewayGuestSessions,
    applyGatewayAccountSessions,
    buildGatewayBootstrapRequest,
    applyGatewayMatchClaims,
    applyGatewayRecoveredMatch,
    applyGatewayQueueRecovery,
  } as const;
}
