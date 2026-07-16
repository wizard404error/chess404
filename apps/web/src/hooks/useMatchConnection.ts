'use client';

import React from 'react';
import type { MatchModeId, MatchSnapshotMessage, PieceColor } from '@chess404/contracts';
import type { GatewayBootstrapGuestSessions, GatewayBootstrapMatchClaims, GatewayBootstrapAccountSessions } from '../lib/system-service';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';
import {
  connectToMatchStream,
  fetchMatch,
  readStoredRoomMeta,
  sendMatchPresenceHeartbeat,
  writeStoredRoomMeta,
} from '../lib/match-service';
import { rematchPrivateMatch } from '../lib/private-match-service';
import { fetchGatewayBootstrap } from '../lib/system-service';
import {
  CLAIM_REFRESH_CHECK_INTERVAL_MS,
  CLAIM_REFRESH_LEAD_MS,
  MATCH_PRESENCE_HEARTBEAT_INTERVAL_MS,
  PRESENCE_RETRY_MESSAGE,
  STREAM_RECONNECT_MESSAGE,
  readStoredGuestIdentity,
  writeStoredActiveMatchId,
} from '../lib/session-storage';

export interface UseMatchConnectionProps {
  sets: {
    setAuthoritativeMatchId: React.Dispatch<React.SetStateAction<string | null>>;
    setAuthoritativeLive: React.Dispatch<React.SetStateAction<boolean>>;
    setStreamDisconnected: React.Dispatch<React.SetStateAction<boolean>>;
    setAuthoritativeStatus: React.Dispatch<React.SetStateAction<'waiting' | 'active' | 'finished' | null>>;
    setAuthoritativeWhiteConnected: React.Dispatch<React.SetStateAction<boolean>>;
    setAuthoritativeBlackConnected: React.Dispatch<React.SetStateAction<boolean>>;
    setAuthoritativeDisconnectGraceFor: React.Dispatch<React.SetStateAction<PieceColor | null>>;
    setAuthoritativeDisconnectGraceDeadline: React.Dispatch<React.SetStateAction<string | null>>;
    setViewerSeat: React.Dispatch<React.SetStateAction<PieceColor | null>>;
    setMatchSeatMeta: React.Dispatch<React.SetStateAction<{
      whiteGuestId?: string;
      blackGuestId?: string;
      whiteName?: string;
      blackName?: string;
    } | null>>;
    setCardMsg: React.Dispatch<React.SetStateAction<string>>;
    setAuthoritativeRematchBusy: React.Dispatch<React.SetStateAction<boolean>>;
    setMatchDestinationNotice: React.Dispatch<React.SetStateAction<string>>;
    setActivePage: React.Dispatch<React.SetStateAction<any>>;
  };

  authoritativeMatchIdRef: React.MutableRefObject<string | null>;
  authoritativeClaimTokensRef: React.MutableRefObject<{ white: string | null; black: string | null }>;
  authoritativeClaimExpiresAtRef: React.MutableRefObject<{ white: string | null; black: string | null }>;

  hostedRuntime: boolean | null;
  viewerSeat: PieceColor | null;
  over: boolean;
  primaryAccountIdentity: { accountId?: string; sessionToken?: string; expiresAt?: string };
  openLiveMatch: (matchId: string) => void;

  buildGatewayBootstrapRequest: (matchId?: string | null) => {
    matchId?: string;
    white: { guestId?: string; sessionSecret?: string; sessionToken?: string; sessionExpiresAt?: string };
    black: { guestId?: string; sessionSecret?: string; sessionToken?: string; sessionExpiresAt?: string };
    whiteAccount: { accountId?: string; sessionToken?: string; expiresAt?: string };
    blackAccount: { accountId?: string; sessionToken?: string; expiresAt?: string };
  };
  applyGatewayGuestSessions: (sessions?: GatewayBootstrapGuestSessions) => void;
  applyGatewayMatchClaims: (matchId: string | null | undefined, claims?: GatewayBootstrapMatchClaims | null) => void;
  applyGatewayAccountSessions: (sessions?: GatewayBootstrapAccountSessions) => void;

  onSnapshot: (snapshot: MatchSnapshotMessage) => void;
  stopAbortCountdown: (manual?: boolean) => void;
  authoritativeActorForColor: (color: PieceColor) => {
    playerId: string;
    playerSecret?: string;
    playerClaimToken?: string;
  };
}

export function useMatchConnection(props: UseMatchConnectionProps) {
  const {
    sets,
    authoritativeMatchIdRef,
    authoritativeClaimTokensRef,
    authoritativeClaimExpiresAtRef,
    hostedRuntime,
    viewerSeat,
    over,
    primaryAccountIdentity,
    openLiveMatch,
    buildGatewayBootstrapRequest,
    applyGatewayGuestSessions,
    applyGatewayMatchClaims,
    applyGatewayAccountSessions,
    onSnapshot,
    stopAbortCountdown,
    authoritativeActorForColor,
  } = props;

  const {
    setAuthoritativeMatchId,
    setAuthoritativeLive,
    setStreamDisconnected,
    setAuthoritativeStatus,
    setAuthoritativeWhiteConnected,
    setAuthoritativeBlackConnected,
    setAuthoritativeDisconnectGraceFor,
    setAuthoritativeDisconnectGraceDeadline,
    setViewerSeat,
    setMatchSeatMeta,
    setCardMsg,
    setAuthoritativeRematchBusy,
    setMatchDestinationNotice,
    setActivePage,
  } = sets;

  const manualRetryRef = React.useRef<(() => void) | null>(null);

  const createAuthoritativeRematchRoom = React.useCallback(async () => {
    const matchId = authoritativeMatchIdRef.current;
    if (!matchId) {
      return;
    }
    const roomMeta = readStoredRoomMeta(matchId);
    if (roomMeta?.queue !== 'direct') {
      return;
    }
    const guestIdentity = readStoredGuestIdentity('white');
    if (!guestIdentity.guestId) {
      setMatchDestinationNotice('Hosted player session is still loading, so rematch room creation is not ready yet.');
      return;
    }

    setAuthoritativeRematchBusy(true);
    setMatchDestinationNotice('');
    try {
      const result = await rematchPrivateMatch({
        matchId,
        identity: {
          guestId: guestIdentity.guestId,
          sessionSecret: guestIdentity.sessionSecret,
          sessionToken: guestIdentity.sessionToken,
          accountId: primaryAccountIdentity.accountId,
          accountSessionToken: primaryAccountIdentity.sessionToken,
        },
        clockSeconds: roomMeta?.clockSeconds ?? 600,
      });
      writeStoredRoomMeta(result.matchId, {
        queue: 'direct',
        modeId: result.snapshot.match.modeId ?? roomMeta?.modeId,
        clockSeconds: roomMeta?.clockSeconds ?? 600,
        viewerSeat: result.seatColor,
        whiteGuestId: result.snapshot.match.whiteGuestId,
        blackGuestId: result.snapshot.match.blackGuestId,
        whiteAccountId: result.snapshot.match.whiteAccountId,
        blackAccountId: result.snapshot.match.blackAccountId,
        whiteName: result.snapshot.match.whiteName,
        blackName: result.snapshot.match.blackName,
        whitePlayerSecret: result.seatColor === 'white' ? result.claim?.playerSecret : undefined,
        blackPlayerSecret: result.seatColor === 'black' ? result.claim?.playerSecret : undefined,
        whiteClaimToken: result.seatColor === 'white' ? result.claim?.claimToken : undefined,
        blackClaimToken: result.seatColor === 'black' ? result.claim?.claimToken : undefined,
        whiteClaimExpiresAt: result.seatColor === 'white' ? result.claim?.expiresAt : undefined,
        blackClaimExpiresAt: result.seatColor === 'black' ? result.claim?.expiresAt : undefined,
      });
      writeStoredActiveMatchId(result.matchId);
      setMatchDestinationNotice('Rematch room created. Opening it now...');
      openLiveMatch(result.matchId);
    } catch (err) {
      setMatchDestinationNotice(err instanceof Error ? err.message : 'Failed to create private rematch room.');
    } finally {
      setAuthoritativeRematchBusy(false);
    }
  }, [authoritativeMatchIdRef, openLiveMatch, primaryAccountIdentity.accountId, primaryAccountIdentity.sessionToken]);

  // ── Stream connection effect ──────────────────────────────────────────────
  React.useEffect(() => {
    const matchId = authoritativeMatchIdRef.current;
    if (!matchId) {
      setAuthoritativeLive(false);
      setAuthoritativeWhiteConnected(false);
      setAuthoritativeBlackConnected(false);
      setAuthoritativeDisconnectGraceFor(null);
      setAuthoritativeDisconnectGraceDeadline(null);
      setViewerSeat(null);
      setMatchSeatMeta(null);
      return;
    }

    stopAbortCountdown(true);
    const streamIdentity = hostedRuntime && viewerSeat ? authoritativeActorForColor(viewerSeat) : null;
    if (hostedRuntime && viewerSeat && !streamIdentity?.playerSecret && !streamIdentity?.playerClaimToken) {
      setAuthoritativeLive(false);
      setStreamDisconnected(true);
      setCardMsg('Cannot connect: missing player credentials. Try re-entering the match.');
      return;
    }

    const { disconnect, retry } = connectToMatchStream(matchId, {
      onSnapshot: (snapshot) => {
        setCardMsg(prev => prev === STREAM_RECONNECT_MESSAGE ? '' : prev);
        onSnapshot(snapshot);
      },
      onStatusChange: (status) => {
        if (status === 'connected') {
          setCardMsg(prev => prev === STREAM_RECONNECT_MESSAGE ? '' : prev);
          setStreamDisconnected(false);
          return;
        }
        if (status === 'reconnecting') {
          setAuthoritativeLive(false);
          setStreamDisconnected(false);
          setCardMsg(STREAM_RECONNECT_MESSAGE);
        }
        if (status === 'disconnected') {
          setAuthoritativeLive(false);
          setStreamDisconnected(true);
          setCardMsg('Live match stream lost. Click "Reconnect" to try again.');
        }
      },
      onError: () => {
        setAuthoritativeLive(false);
      }
    }, streamIdentity);

    manualRetryRef.current = retry;

    return () => {
      disconnect();
    };
  }, [authoritativeActorForColor, authoritativeMatchIdRef, onSnapshot, hostedRuntime, stopAbortCountdown, viewerSeat]);

  // ── Presence heartbeat effect ─────────────────────────────────────────────
  React.useEffect(() => {
    if (!hostedRuntime || !authoritativeMatchIdRef.current || !viewerSeat || over) {
      return;
    }

    let cancelled = false;
    const sendHeartbeat = async () => {
      try {
        await sendMatchPresenceHeartbeat(authoritativeMatchIdRef.current!, authoritativeActorForColor(viewerSeat));
        if (!cancelled) {
          setCardMsg(prev => prev === PRESENCE_RETRY_MESSAGE ? '' : prev);
        }
      } catch {
        if (!cancelled) {
          setCardMsg(prev => prev || PRESENCE_RETRY_MESSAGE);
        }
      }
    };

    void sendHeartbeat();
    const interval = window.setInterval(() => {
      void sendHeartbeat();
    }, MATCH_PRESENCE_HEARTBEAT_INTERVAL_MS);
    const handleFocus = () => {
      void sendHeartbeat();
    };
    const handleVisibility = () => {
      if (!document.hidden) {
        void sendHeartbeat();
      }
    };
    window.addEventListener('focus', handleFocus);
    document.addEventListener('visibilitychange', handleVisibility);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener('focus', handleFocus);
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  }, [authoritativeActorForColor, authoritativeMatchIdRef, hostedRuntime, over, viewerSeat]);

  // ── Fallback polling fetch effect ─────────────────────────────────────────
  React.useEffect(() => {
    if (!authoritativeMatchIdRef.current || over) {
      return;
    }

    const interval = window.setInterval(() => {
      void fetchMatch(authoritativeMatchIdRef.current!).then(snapshot => {
        onSnapshot(snapshot);
      }).catch(() => {});
    }, 5000);

    return () => window.clearInterval(interval);
  }, [authoritativeMatchIdRef, over, onSnapshot]);

  // ── Claim refresh effect ──────────────────────────────────────────────────
  React.useEffect(() => {
    if (!authoritativeMatchIdRef.current || over) {
      return;
    }

    let cancelled = false;
    let refreshInFlight = false;

    const claimNeedsRefresh = (token?: string | null, expiresAt?: string | null): boolean => {
      if (!token || !expiresAt) {
        return true;
      }
      const expiryMs = Date.parse(expiresAt);
      if (Number.isNaN(expiryMs)) {
        return true;
      }
      return expiryMs - Date.now() <= CLAIM_REFRESH_LEAD_MS;
    };

    const maybeRefreshClaims = async () => {
      if (refreshInFlight) {
        return;
      }

      const storedWhite = readStoredGuestIdentity('white');
      const storedBlack = readStoredGuestIdentity('black');
      if ((!storedWhite.guestId && !storedWhite.sessionSecret) && (!storedBlack.guestId && !storedBlack.sessionSecret)) {
        return;
      }

      const roomMeta = readStoredRoomMeta(authoritativeMatchIdRef.current!);
      const whiteNeedsRefresh = claimNeedsRefresh(
        authoritativeClaimTokensRef.current.white ?? roomMeta?.whiteClaimToken,
        authoritativeClaimExpiresAtRef.current.white ?? roomMeta?.whiteClaimExpiresAt,
      );
      const blackNeedsRefresh = claimNeedsRefresh(
        authoritativeClaimTokensRef.current.black ?? roomMeta?.blackClaimToken,
        authoritativeClaimExpiresAtRef.current.black ?? roomMeta?.blackClaimExpiresAt,
      );

      if (!whiteNeedsRefresh && !blackNeedsRefresh) {
        return;
      }

      refreshInFlight = true;
      try {
        const bootstrap = await fetchGatewayBootstrap(buildGatewayBootstrapRequest(authoritativeMatchIdRef.current!));
        if (cancelled) {
          return;
        }
        applyGatewayGuestSessions(bootstrap.guestSessions);
        applyGatewayMatchClaims(authoritativeMatchIdRef.current!, bootstrap.matchClaims);
        applyGatewayAccountSessions(bootstrap.accountSessions);
      } catch {
      } finally {
        refreshInFlight = false;
      }
    };

    void maybeRefreshClaims();
    const interval = window.setInterval(() => {
      void maybeRefreshClaims();
    }, CLAIM_REFRESH_CHECK_INTERVAL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [authoritativeMatchIdRef, over, applyGatewayGuestSessions, applyGatewayMatchClaims, applyGatewayAccountSessions, buildGatewayBootstrapRequest, authoritativeClaimTokensRef, authoritativeClaimExpiresAtRef]);

  const onStreamReconnect = React.useCallback(() => {
    manualRetryRef.current?.();
  }, []);

  return {
    manualRetryRef,
    createAuthoritativeRematchRoom,
    onStreamReconnect,
  };
}
