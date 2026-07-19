'use client';

import React from 'react';
import { PlatformContext } from '../contexts/PlatformContext';
import { usePathname, useRouter } from 'next/navigation';
import { PlayerBar } from '../components/match/PlayerBar';
import { GamePanel } from '../components/match/GamePanel';
import type { MatchFinishReason, MatchModeId, MatchSnapshotMessage, MatchState as AuthoritativeMatchState, PlayerIntent } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import { useStockfish } from '../usestockfish';
import type { Board, PieceType, PieceColor, Piece, Sq, GameCard, CardMechanic, CardPendingState, DoubleMove, BombPiece, LavaSquare, Snapshot, Rarity } from '../types';
import { makeBoard, cloneBoard, findKing, isAttacked, inB, legalMoves, gameStatus, insuffMat, positionKey, threefold, toFEN, moveNotation, uciToSan } from '../chessEngine';
import { CARD_POOL, drawRandomCard, incrementCardSeq } from '../cardPool';
import { RARITY_STYLE, RARITY_WEIGHTS, OPP, FILES, RANKS, SQ, MAX_HAND_SIZE, CLOCK_START, ABORT_SECS, DRAW_FROM, DRAW_EVERY, INITIAL_DEAL_ROUND, PIECE_VALUE, UPGRADE, DOWNGRADE, TARGETING_CARDS, CARD_TARGET_MESSAGES } from '../constants';
import { GLOBAL_STYLES } from '../styles';


import { fetchGatewayBootstrap } from '../lib/system-service';
import { joinPrivateMatch } from '../lib/private-match-service';
import {
  applyIntent,
  configureMatchServiceRuntime,
  createMatch,
  ensureMatch,
  fetchMatch,
  readStoredRoomMeta,
  resolveSeatSecret,
  writeStoredRoomMeta,
  type MatchServiceRuntimeConfig,
  type StoredRoomMeta,
} from '../lib/match-service';
import {
  connectAccountNotificationStream,
  createGuestSession,
  formatAccountRestrictionNotice,
  fetchDirectChallengeOverview,
  fetchFriendOverview,
  fetchAccountNotificationOverview,
  isAccountRestrictionError,
  parseAccountRestrictionMessage,
  type AccountNotificationView,
  type AccountSession as PlatformAccountSession,
  type DirectChallengeOverview,
  type FriendOverview,
  type GuestProfile,
  type GuestSession as PlatformGuestSession,
  type MatchSeatClaim,
  touchAccountPresence,
} from '../lib/platform-service';
import type { QueueName } from '../lib/matchmaking-service';

type AppPage =
  | 'Play'
  | 'Match'
  | 'Watch'
  | 'Rankings'
  | 'Profiles'
  | 'Account'
  | 'History'
  | 'Friends'
  | 'Inbox'
  | 'Cards'
  | 'Community'
  | 'Status'
  | 'Admin'
  | 'Modes'
  | 'Queue'
  | 'Lobbies';

// ─── App ──────────────────────────────────────────────────────────────────────
const AUTHORITATIVE_JOKER_MECHANICS = new Set<CardMechanic>([
  'freeze', 'shield', 'sniper', 'badsniper', 'promote', 'demote', 'promotehim', 'demotehim',
  'teleport', 'jump', 'doublemove_diff', 'doublemove_same', 'swapme', 'swapus', 'swaphim',
  'borrow', 'mindcontrol', 'parasite', 'clone', 'fakepiece', 'lavaground', 'blackhole',
  'fortress',
  'fog_village', 'invisible', 'unabomber', 'halffuse', 'fullfusion', 'reverse', 'undo',
  'mirror', 'smallsacrifice', 'bigsacrifice', 'gambler', 'radar', 'cheater'
]);

import {
  ACTIVE_MATCH_STORAGE_KEY,
  readStoredActiveMatchId,
  writeStoredActiveMatchId,
  readStoredGuestIdentity,
  writeStoredGuestIdentity,
  clearStoredGuestIdentity,
  readStoredAccountIdentity,
  writeStoredAccountIdentity,
  clearStoredAccountIdentity,
  clearRequestedMatchQuery,
  syncAllQueries,
  buildLiveMatchUrl,
  buildReplayPageUrl,
  copyTextToClipboard,
} from '../lib/session-storage';
import type { QueueTicket } from '../lib/matchmaking-service';
import {
  modeLabel,
  queueLabel,
  finishReasonLabel,
  buildSocialAlert,
  type SocialAlert,
} from '../lib/match-labels';
import { useMatchTimer } from './useMatchTimer';
import { useMatchReplay } from './useMatchReplay';
import { usePlatformState } from './usePlatformState';
import { useMatchConnection } from './useMatchConnection';
import { useBoardInteraction } from './useBoardInteraction';
import { useMatchNav } from './useMatchNav';
import { useMatchAnimations } from './useMatchAnimations';
import { useCardEngine } from './useCardEngine';
import { useGameState } from './useGameState';

function buildStoredRoomMeta(
  base: StoredRoomMeta | null | undefined,
  whiteProfile: GuestProfile | null,
  blackProfile: GuestProfile | null,
  whiteSessionSecret: string | null,
  blackSessionSecret: string | null,
  options: { ensureSecrets?: boolean } = {},
): StoredRoomMeta {
  return {
    ...base,
    modeId: base?.modeId ?? DEFAULT_MATCH_MODE_ID,
    whiteGuestId: base?.whiteGuestId ?? whiteProfile?.guestId,
    blackGuestId: base?.blackGuestId ?? blackProfile?.guestId,
    whiteAccountId: base?.whiteAccountId ?? readStoredAccountIdentity('white').accountId,
    blackAccountId: base?.blackAccountId ?? readStoredAccountIdentity('black').accountId,
    whiteName: base?.whiteName ?? whiteProfile?.displayName,
    blackName: base?.blackName ?? blackProfile?.displayName,
    whitePlayerSecret: options.ensureSecrets ? resolveSeatSecret(base?.whitePlayerSecret, whiteSessionSecret) : base?.whitePlayerSecret,
    blackPlayerSecret: options.ensureSecrets ? resolveSeatSecret(base?.blackPlayerSecret, blackSessionSecret) : base?.blackPlayerSecret,
  };
}


// ─── UseMatchEngineProps ───────────────────────────────────────────────────────
// Fully-typed boundary between App.tsx and the extracted useMatchEngine hook.
// All props mirror the exact useState<T> types declared in App.tsx.
export interface UseMatchEngineProps {
  // ── Read-only state passed in from App ──────────────────────────────────────
  accountActionQueryDetected: boolean;
  activePage: AppPage;
  authoritativeRematchBusy: boolean;
  blackProfile: GuestProfile | null;
  communityFocusGuestId: string | null;
  friendsAttentionCount: number;
  guestProfilesReady: boolean;
  historyFocusGuestId: string | null;
  historyFocusMatchId: string | null;
  historyQueryReady: boolean;
  hostedRuntime: boolean | null;
  inboxUnreadCount: number;
  matchDestinationNotice: string;
  matchQueryReady: boolean;
  matchSeatMeta: {
    whiteGuestId?: string;
    blackGuestId?: string;
    whiteName?: string;
    blackName?: string;
  } | null;
  openedBoardMatchRef: React.MutableRefObject<string | null>;
  pathname: string;
  profileFocusHandle: string | null;
  profileQueryReady: boolean;
  bootstrapQueueRecovery: {
    white: QueueTicket | null;
    black: QueueTicket | null;
  } | null;
  queueLaunchIntent: { modeId: MatchModeId; queue: QueueName } | null;
  router: ReturnType<typeof useRouter>;
  socialAlert: SocialAlert | null;
  socialLiveToken: number;
  viewerSeat: PieceColor | null;
  whiteProfile: GuestProfile | null;

  // ── Setters delegated to App state ──────────────────────────────────────────
  setAccountActionQueryDetected: React.Dispatch<React.SetStateAction<boolean>>;
  setActivePage: React.Dispatch<React.SetStateAction<AppPage>>;
  setAuthoritativeRematchBusy: React.Dispatch<React.SetStateAction<boolean>>;
  setBlackProfile: React.Dispatch<React.SetStateAction<GuestProfile | null>>;
  setFriendsAttentionCount: React.Dispatch<React.SetStateAction<number>>;
  setGuestProfilesReady: React.Dispatch<React.SetStateAction<boolean>>;
  setHistoryFocusGuestId: React.Dispatch<React.SetStateAction<string | null>>;
  setHistoryFocusMatchId: React.Dispatch<React.SetStateAction<string | null>>;
  setHistoryQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setHostedRuntime: React.Dispatch<React.SetStateAction<boolean | null>>;
  setInboxUnreadCount: React.Dispatch<React.SetStateAction<number>>;
  setMatchDestinationNotice: React.Dispatch<React.SetStateAction<string>>;
  setMatchQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setMatchSeatMeta: React.Dispatch<React.SetStateAction<{
    whiteGuestId?: string;
    blackGuestId?: string;
    whiteName?: string;
    blackName?: string;
  } | null>>;
  setProfileFocusHandle: React.Dispatch<React.SetStateAction<string | null>>;
  setProfileQueryReady: React.Dispatch<React.SetStateAction<boolean>>;
  setBootstrapQueueRecovery: React.Dispatch<React.SetStateAction<{
    white: QueueTicket | null;
    black: QueueTicket | null;
  } | null>>;
  setCommunityFocusGuestId: React.Dispatch<React.SetStateAction<string | null>>;
  setQueueLaunchIntent: React.Dispatch<React.SetStateAction<{ modeId: MatchModeId; queue: QueueName } | null>>;
  setSecondaryMenuOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setSocialAlert: React.Dispatch<React.SetStateAction<SocialAlert | null>>;
  setSocialLiveToken: React.Dispatch<React.SetStateAction<number>>;
  setViewerSeat: React.Dispatch<React.SetStateAction<PieceColor | null>>;
  setWhiteProfile: React.Dispatch<React.SetStateAction<GuestProfile | null>>;
}

const useFocusTrap = (ref: React.RefObject<HTMLElement | null>, active: boolean) => {
  React.useEffect(() => {
    if (!active || !ref.current) return;
    const el = ref.current;
    const focusable = el.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (first) first.focus();
    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    };
    el.addEventListener('keydown', handler);
    return () => el.removeEventListener('keydown', handler);
  }, [active, ref]);
};

export function useMatchEngineFacade(props: UseMatchEngineProps) {
  const { accountActionQueryDetected, activePage, authoritativeRematchBusy, blackProfile, communityFocusGuestId, friendsAttentionCount, guestProfilesReady, historyFocusGuestId, historyFocusMatchId, historyQueryReady, hostedRuntime, inboxUnreadCount, matchDestinationNotice, matchQueryReady, matchSeatMeta, openedBoardMatchRef, pathname, profileFocusHandle, profileQueryReady, queueLaunchIntent, router, setAccountActionQueryDetected, setActivePage, setAuthoritativeRematchBusy, setBlackProfile, setFriendsAttentionCount, setGuestProfilesReady, setHistoryFocusGuestId, setHistoryFocusMatchId, setHistoryQueryReady, setHostedRuntime, setInboxUnreadCount, setMatchDestinationNotice, setMatchQueryReady, setMatchSeatMeta, setProfileFocusHandle, setProfileQueryReady, setBootstrapQueueRecovery, setQueueLaunchIntent, setSecondaryMenuOpen, setSocialAlert, setSocialLiveToken, setViewerSeat, setWhiteProfile, socialAlert, socialLiveToken, viewerSeat, whiteProfile } = props;

  const platformState = usePlatformState({
    hostedRuntime, setHostedRuntime,
    activePage, setActivePage,
    setAccountActionQueryDetected,
    setHistoryFocusMatchId,
    setHistoryFocusGuestId,
    setProfileFocusHandle,
    setProfileQueryReady, setHistoryQueryReady, setMatchQueryReady,
    setFriendsAttentionCount, setInboxUnreadCount,
    setSocialAlert, socialAlert,
    setSocialLiveToken, socialLiveToken,
    setWhiteProfile, setBlackProfile, setViewerSeat, viewerSeat,
    whiteProfile, blackProfile,
    setGuestProfilesReady, guestProfilesReady,
    setBootstrapQueueRecovery,
    setMatchSeatMeta,
    setMatchDestinationNotice,
    openedBoardMatchRef,
    pathname,
    profileFocusHandle,
    historyFocusGuestId, historyFocusMatchId,
  });
  const {
    primaryAccountIdentity,
    setPrimaryAccountIdentity,
    shellAccountNotice,
    setShellAccountNotice,
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
  } = platformState;

  const {
    board, setBoard,
    turn, setTurn,
    sel, setSel,
    hints, setHints,
    premove, setPremove,
    moved, setMoved,
    lm, setLm,
    drag, setDrag,
    dragPos, setDragPos,
    promo, setPromo,
    check, setCheck,
    mate, setMate,
    stale, setStale,
    insuf, setInsuf,
    hmc, setHmc,
    fmn, setFmn,
    posHist, setPosHist,
    drawOffer, setDrawOffer,
    over, setOver,
    winner, setWinner,
    authoritativeFinishReason, setAuthoritativeFinishReason,
    movHist, setMovHist,
    snapshots, setSnapshots,
    analysisArrows, setAnalysisArrows,
    boardRef, turnRef, movedRef, lmRef, hmcRef, fmnRef, posHistRef, overRef, premoveRef,
  } = useBoardInteraction();

  const openProfileHandle = React.useCallback((handle: string) => {
    const normalized = handle.trim().toLowerCase();
    if (!normalized) return;
    setProfileFocusHandle(normalized);
    router.push('/profiles');
  }, [router]);

  const openReplayMatch = React.useCallback((matchId: string, guestId: string | null = null) => {
    const normalizedMatchId = matchId.trim();
    if (!normalizedMatchId) return;
    setHistoryFocusMatchId(normalizedMatchId);
    setHistoryFocusGuestId(guestId);
    router.push('/history');
  }, [router]);

  const openGuestHistory = React.useCallback((guestId: string) => {
    const normalizedGuestId = guestId.trim();
    if (!normalizedGuestId) return;
    setHistoryFocusGuestId(normalizedGuestId);
    setHistoryFocusMatchId(null);
    router.push('/history');
  }, [router]);

  const openLiveMatch = React.useCallback((matchId: string) => {
    const normalizedMatchId = matchId.trim();
    if (!normalizedMatchId) return;
    router.push(`/match/${normalizedMatchId}`);
  }, [router]);

  React.useEffect(() => {
    setSecondaryMenuOpen(false);
  }, [activePage]);

  const copyLiveMatchLink = React.useCallback(async (matchId: string) => {
    const matchUrl = buildLiveMatchUrl(matchId);
    if (!matchUrl) {
      return;
    }
    try {
      const copied = await copyTextToClipboard(matchUrl);
      setMatchDestinationNotice(copied ? 'Live match link copied.' : matchUrl);
    } catch {
      setMatchDestinationNotice(matchUrl);
    }
  }, []);

  const copyReplayPageLink = React.useCallback(async (matchId: string) => {
    const replayUrl = buildReplayPageUrl(matchId);
    if (!replayUrl) {
      return;
    }
    try {
      const copied = await copyTextToClipboard(replayUrl);
      setMatchDestinationNotice(copied ? 'Replay page link copied.' : replayUrl);
    } catch {
      setMatchDestinationNotice(replayUrl);
    }
  }, []);

  const [authoritativeMatchId, setAuthoritativeMatchId] = React.useState<string | null>(null);

  // ── Card engine: owns all card + joker + doubleMove state ───────────────────
  const {
    whiteHand, setWhiteHand,
    blackHand, setBlackHand,
    selectedCard, setSelectedCard,
    dealPhase, setDealPhase,
    lastDrawAnim, setLastDrawAnim,
    cardPending, setCardPending,
    cardMsg, setCardMsg,
    promoPicker, setPromoPicker,
    cardPromo, setCardPromo,
    cardUsedBy, setCardUsedBy,
    jokerPicker, setJokerPicker,
    doubleMove, setDoubleMove,
    doubleMoveRef,
    cardUsedByRef,
    pendingCardUseRef,
    lastDrawRound,
    roundNumber,
  } = useCardEngine(board, turn, moved, lm, fmn, fmnRef, boardRef, turnRef, authoritativeMatchId, hostedRuntime, viewerSeatRef);
  const jokerRef = React.useRef<HTMLDivElement>(null);
  useFocusTrap(jokerRef, jokerPicker !== null);

  const {
    cardAnim, setCardAnim,
    cardAnimLbl, setCardAnimLbl,
    fireCardAnim,
    bombPieces, setBombPieces,
    bombExploding, setBombExploding,
    bombPiecesRef,
    swapAnim, setSwapAnim,
    swapAnimTimerRef,
    triggerSwapAnim,
    transformAnim, setTransformAnim,
    transformAnimTimerRef,
    triggerTransformAnim,
    sniperAnim, setSniperAnim,
    sniperAnimTimerRef,
    teleportAnim, setTeleportAnim,
    teleportAnimTimerRef,
    jumpAnim, setJumpAnim,
    jumpAnimTimerRef,
    sacrificeAnim, setSacrificeAnim,
    sacrificeAnimTimerRef,
    triggerSacrificeAnim,
    mindControlAnim, setMindControlAnim,
    mindControlAnimTimerRef,
    triggerMindControlAnim,
    fuseAnim, setFuseAnim,
    fuseAnimTimerRef,
    triggerFuseAnim,
    triggerJumpAnim,
    triggerSniperAnim,
    triggerTeleportAnim,
  } = useMatchAnimations();

  const cardMsgTimerRef   = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  React.useEffect(() => {
    return () => {
      if (cardMsgTimerRef.current) clearTimeout(cardMsgTimerRef.current);
    };
  }, []);

  const resetCardUsed = React.useCallback((nextTurn: PieceColor) => {
    cardUsedByRef.current = { ...cardUsedByRef.current, [nextTurn]: false };
    setCardUsedBy(prev => ({ ...prev, [nextTurn]: false }));
    // Decrement cheater when the opponent's turn starts (cheater just finished their turn)
    if (cheaterColorRef.current !== null && nextTurn !== cheaterColorRef.current) {
      setCheaterTurnsLeft(prev => {
        const next = prev - 1;
        if (next <= 0) { cheaterColorRef.current = null; setCheaterColor(null); }
        return Math.max(0, next);
      });
    }
    setRadarActive(false);
  }, []);

  const [chatMessages, setChatMessages] = React.useState<{ sender: 'white' | 'black'; text: string }[]>([]);
  const [chatInput,    setChatInput]    = React.useState('');
  const chatRef = React.useRef<HTMLDivElement>(null);


  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const matchId = authoritativeMatchIdRef.current;
    if (!matchId || !over || !winner || winner === 'aborted') return;
    if (finalizedResultRef.current === matchId) return;

    const roomMeta = readStoredRoomMeta(matchId);
    if (roomMeta?.queue !== 'rated') return;

    finalizedResultRef.current = matchId;
  }, [over, winner]);
  const blackMovedRef  = React.useRef(false);

  const [engineOn,    setEngineOn]    = React.useState(false);
  const [authoritativeLive, setAuthoritativeLive] = React.useState(false);
  const [streamDisconnected, setStreamDisconnected] = React.useState(false);

  const {
    timeW, setTimeW,
    timeB, setTimeB,
    clockActive, setClockActive,
    tickingState, tickingRef, setTicking, setTickingState,
    abortCountdown, setAbortCountdown,
    abortActive, setAbortActive,
    startAbortCountdown, stopAbortCountdown, resetTimer,
    abortRef,
  } = useMatchTimer({
    initialClockStart: CLOCK_START,
    initialAbortSecs: ABORT_SECS,
    over,
    authoritativeLive,
    onTimeout: (loser) => {
      setOver(true);
      setWinner(loser === 'white' ? 'black' : 'white');
    },
    onAbort: () => {
      setWinner('aborted');
      setOver(true);
    },
  });
  React.useEffect(() => { stopAbortRef.current = stopAbortCountdown; });

  const allSyncReady = profileQueryReady && historyQueryReady && matchQueryReady;
  React.useEffect(() => {
    if (!allSyncReady) return;
    console.log('[DEBUG] syncAllQueries: activePage=', activePage, 'authoritativeMatchId=', authoritativeMatchId, 'hostedRuntime=', hostedRuntime);
    syncAllQueries({
      profileHandle: activePage === 'Profiles' ? profileFocusHandle : null,
      historyMatchId: activePage === 'History' ? historyFocusMatchId : null,
      historyGuestId: activePage === 'History' ? historyFocusGuestId : null,
      matchId: (activePage === 'Match' || (!hostedRuntime && activePage === 'Play')) ? authoritativeMatchId : null,
    });
  }, [allSyncReady, activePage, profileFocusHandle, historyFocusGuestId, historyFocusMatchId, authoritativeMatchId, hostedRuntime]);
  const [authoritativeStatus, setAuthoritativeStatus] = React.useState<'waiting' | 'active' | 'finished' | null>(null);
  const [authoritativeWhiteConnected, setAuthoritativeWhiteConnected] = React.useState(false);
  const [authoritativeBlackConnected, setAuthoritativeBlackConnected] = React.useState(false);
  const [authoritativeDisconnectGraceFor, setAuthoritativeDisconnectGraceFor] = React.useState<PieceColor | null>(null);
  const [authoritativeDisconnectGraceDeadline, setAuthoritativeDisconnectGraceDeadline] = React.useState<string | null>(null);

  // Refs for late-bound callbacks used by useMatchConnection
  const onSnapshotRef = React.useRef<((snapshot: MatchSnapshotMessage) => void) | null>(null);
  const stopAbortRef = React.useRef<((manual?: boolean) => void) | null>(null);
  const actorForColorRef = React.useRef<((color: PieceColor) => {
    playerId: string;
    playerSecret?: string;
    playerClaimToken?: string;
  }) | null>(null);

  const matchConnection = useMatchConnection({
    sets: {
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
    },
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
    onSnapshot: (snapshot) => onSnapshotRef.current?.(snapshot),
    stopAbortCountdown: (manual) => stopAbortRef.current?.(manual),
    authoritativeActorForColor: (color) => actorForColorRef.current!(color),
  });
  const {
    manualRetryRef,
    createAuthoritativeRematchRoom,
    onStreamReconnect,
  } = matchConnection;

  const [cheaterTurnsLeft, setCheaterTurnsLeft] = React.useState(0);
  const [cheaterColor,     setCheaterColor]     = React.useState<PieceColor | null>(null);
  const cheaterColorRef = React.useRef<PieceColor | null>(null);
  const cheaterActive = cheaterTurnsLeft > 0;
  const [radarActive,   setRadarActive]   = React.useState(false);


  const [lavaSquares,   setLavaSquares]   = React.useState<LavaSquare[]>([]);
  const [lavaExploding, setLavaExploding] = React.useState<Sq[]>([]);

  // Ghost piece: invisible piece lives OUTSIDE the board entirely
  // { row, col } = where it currently is; piece = what it is; ownerColor = who owns it; roundsLeft = owner turns before expiry
  const [ghostPiece, setGhostPiece] = React.useState<{ row: number; col: number; piece: Piece; ownerColor: PieceColor; roundsLeft: number } | null>(null);
  const ghostRef = React.useRef<{ row: number; col: number; piece: Piece; ownerColor: PieceColor; roundsLeft: number } | null>(null);
  React.useEffect(() => { ghostRef.current = ghostPiece; }, [ghostPiece]);

  // ── Fog of War zones ───────────────────────────────────────────────────────
  // Each zone: a 3×3 area centered on (centerRow,centerCol) owned by ownerColor
  // turnsLeft counts down each time white is about to move (= 1 full round passed)
  const [fogZones, setFogZones] = React.useState<{ centerRow: number; centerCol: number; ownerColor: PieceColor; turnsLeft: number }[]>([]);

  const { isReady: sfReady, isThinking, ev, sfErr, analyse, stop, resetEval } = useStockfish(engineOn);

  const {
    reviewIdx,
    setReviewIdx,
    reviewBoard,
    setReviewBoard,
    isReviewing,
    goToSnap,
    reviewFirst,
    reviewPrev,
    reviewNext,
    reviewLast,
  } = useMatchReplay({
    snapshots,
    over,
    resetEval,
  });
  const movRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => { movRef.current?.scrollTo({ top: movRef.current.scrollHeight }); }, [movHist]);
  React.useEffect(() => { chatRef.current?.scrollTo({ top: chatRef.current.scrollHeight }); }, [chatMessages]);



  const lavaSquaresRef = React.useRef(lavaSquares);
  React.useEffect(() => { lavaSquaresRef.current = lavaSquares; }, [lavaSquares]);

  const isMountedRef = React.useRef(true);
  React.useEffect(() => {
    return () => {
      isMountedRef.current = false;
      setAuthoritativeMatchId(null);
    };
  }, []);

  const gameIdRef = React.useRef(0);
  const [gameKey, setGameKey] = React.useState(0);
  // ── Game state: authoritative snapshot / bootstrap / submit / player auth ────
  const {
    finalizedResultRef,
    finalPositionRef,
    lastAppliedSeqNumRef,
    authoritativeBootstrapRef,
    buildMoveRows,
    buildPendingCardFromSnapshot,
    applyAuthoritativeSnapshot,
    bootstrapAuthoritativeMatch,
    submitAuthoritativeIntent: _submitGameIntent,
    authoritativePlayerIdForColor,
    authoritativePlayerSecretForColor,
    authoritativePlayerClaimTokenForColor,
    authoritativeActorForColor,
  } = useGameState(
    { setBoard, setTurn, setMoved, setLm, setHmc, setFmn, setCheck, setMate, setStale, setInsuf, setOver, setWinner, setDrawOffer, setMovHist, setPosHist, setSnapshots, setChatMessages, setReviewIdx, setReviewBoard, setTimeW, setTimeB, setClockActive, setTicking },
    { setWhiteHand, setBlackHand, setSelectedCard, setLastDrawAnim, setCardPending, setPromoPicker, setCardUsedBy, setDealPhase },
    { setViewerSeat, setMatchSeatMeta },
    { setLavaSquares, setBombPieces, setFogZones, setDoubleMove, setGhostPiece, setRadarActive, setCheaterTurnsLeft, setCheaterColor },
    { setAuthoritativeMatchId, setAuthoritativeLive, setAuthoritativeStatus, setAuthoritativeFinishReason, setAuthoritativeWhiteConnected, setAuthoritativeBlackConnected, setAuthoritativeDisconnectGraceFor, setAuthoritativeDisconnectGraceDeadline },
    { turnRef, cheaterColorRef, authoritativeSeatIdsRef, authoritativeSeatSecretsRef, authoritativeClaimTokensRef, cardUsedByRef, pendingCardUseRef },
    hostedRuntime,
    whiteProfileRef,
    blackProfileRef,
    requestedMatchIdRef,
    gatewayRecoveredMatchIdRef,
  );

  const submitAuthoritativeIntent = React.useCallback(async (intent: any) => {
    const matchId = authoritativeMatchIdRef.current;
    if (!matchId) return false;
    if (hostedRuntime && !viewerSeatRef.current) {
      setCardMsg('Spectators cannot control this hosted match.');
      setTimeout(() => setCardMsg(''), 2500);
      return false;
    }
    try {
      setIntentInFlight(true);
      const snapshot = await _submitGameIntent(intent, matchId);
      setIntentInFlight(false);
      return true;
    } catch (err) {
      setIntentInFlight(false);
      const message = err instanceof Error ? err.message : 'Backend request failed';
      setCardMsg(`Backend action failed: ${message}`);
      setTimeout(() => setCardMsg(''), 2500);
      return false;
    }
  }, [_submitGameIntent, hostedRuntime]);

  React.useEffect(() => { onSnapshotRef.current = applyAuthoritativeSnapshot; });

  const prevTurnRef = React.useRef<PieceColor>('white');
  const premoveActorRef = React.useRef<(color: PieceColor) => { playerId: string; playerSecret?: string; playerClaimToken?: string }>(() => ({ playerId: '' }));
  const premoveCanSubmitRef = React.useRef<(fr: number, fc: number, tr: number, tc: number) => boolean>(() => false);
  React.useEffect(() => {
    if (prevTurnRef.current !== turn && turn === viewerSeat && premove && !over && authoritativeMatchIdRef.current) {
      const pm = premoveRef.current;
      if (pm && premoveCanSubmitRef.current(pm.from.row, pm.from.col, pm.to.row, pm.to.col)) {
        const backendMoveIntent: Omit<Extract<PlayerIntent, { type: 'make_move' }>, 'matchId'> = {
          type: 'make_move',
          ...premoveActorRef.current(turnRef.current),
          from: { row: pm.from.row, col: pm.from.col },
          to: { row: pm.to.row, col: pm.to.col },
        };
        void applyIntent(authoritativeMatchIdRef.current!, backendMoveIntent).then(snapshot => {
          applyAuthoritativeSnapshot(snapshot);
        }).catch(() => {});
      }
      setPremove(null);
    }
    prevTurnRef.current = turn;
  }, [turn, viewerSeat, premove, over, applyAuthoritativeSnapshot]);

  const authoritativeGuestSessionSecretForColor = React.useCallback((color: PieceColor): string | undefined => {
    const seatGuestId = authoritativeSeatIdsRef.current[color];
    if (!seatGuestId) {
      return undefined;
    }
    if (whiteProfileRef.current?.guestId === seatGuestId) {
      return guestSessionSecretsRef.current.white ?? undefined;
    }
    if (blackProfileRef.current?.guestId === seatGuestId) {
      return guestSessionSecretsRef.current.black ?? undefined;
    }
    const storedWhite = readStoredGuestIdentity('white');
    if (storedWhite.guestId === seatGuestId) {
      return storedWhite.sessionSecret;
    }
    const storedBlack = readStoredGuestIdentity('black');
    if (storedBlack.guestId === seatGuestId) {
      return storedBlack.sessionSecret;
    }
    return undefined;
  }, []);
  premoveActorRef.current = authoritativeActorForColor;
  React.useEffect(() => { actorForColorRef.current = authoritativeActorForColor; });

  // ── NEW: Bomb explosion logic ──────────────────────────────────────────────
  // Called at start of each turn.
  // Countdown ticks ONLY when white is about to move (= black just finished = 1 full round passed).
  const processBombs = React.useCallback((currentTurn: PieceColor, currentBoard: Board) => {
    const bombs = bombPiecesRef.current;
    if (bombs.length === 0) return currentBoard;

    // Only decrement once per FULL round (after black moves)
    const shouldDecrement = currentTurn === 'white';

    const updatedBombs: BombPiece[] = [];
    const nb = currentBoard;
    const newExplodingSqs: Sq[] = [];

    for (const bomb of bombs) {
      const p = nb[bomb.row]?.[bomb.col];
      const hasBombAtTracked = p?.bomb === true;

      let foundRow = -1, foundCol = -1;
      if (hasBombAtTracked) {
        foundRow = bomb.row; foundCol = bomb.col;
      } else {
        outer: for (let r = 0; r < 8; r++) {
          for (let c = 0; c < 8; c++) {
            if (nb[r][c]?.bomb === true && nb[r][c]?.color === bomb.ownerColor) {
              foundRow = r; foundCol = c; break outer;
            }
          }
        }
      }

      const newTurnsLeft = shouldDecrement ? bomb.turnsLeft - 1 : bomb.turnsLeft;

      if (newTurnsLeft <= 0 && foundRow >= 0) {
        // EXPLODE! Destroy all adjacent pieces (kings immune) + the bomb piece itself
        const explodeCenter = { row: foundRow, col: foundCol };
        newExplodingSqs.push(explodeCenter);

        // Collect adjacent squares
        for (let dr = -1; dr <= 1; dr++) {
          for (let dc = -1; dc <= 1; dc++) {
            const r = foundRow + dr, c = foundCol + dc;
            if (!inB(r, c)) continue;
            const target = nb[r][c];
            if (target && target.type !== 'king') {
              newExplodingSqs.push({ row: r, col: c });
            }
          }
        }

        // Apply destruction after animation frame
        // (we'll do the actual board update in setTimeout below)
      } else if (foundRow >= 0) {
        updatedBombs.push({ ...bomb, row: foundRow, col: foundCol, turnsLeft: newTurnsLeft });
      }
    }

    if (newExplodingSqs.length > 0) {
      // Deduplicate
      const uniqueSqs = newExplodingSqs.filter((s, i, arr) =>
        arr.findIndex(x => x.row === s.row && x.col === s.col) === i
      );
      setBombExploding(uniqueSqs);
      fireCardAnim('bomb_explode', '💥 Bomb detonated!');

      setTimeout(() => {
        setBoard(b2 => {
          const explodedBoard = cloneBoard(b2);
          for (const sq of uniqueSqs) {
            const tp = explodedBoard[sq.row]?.[sq.col];
            if (tp && tp.type !== 'king') {
              explodedBoard[sq.row][sq.col] = null;
            }
          }
          return explodedBoard;
        });
        setBombPieces(updatedBombs);
        setBombExploding([]);
        setCardMsg(`💥 BOMB EXPLODED! ${uniqueSqs.length} pieces destroyed!`);
        setTimeout(() => setCardMsg(''), 3000);
      }, 900);
    } else {
      setBombPieces(updatedBombs);
    }

    return currentBoard;
  }, []);



  // Engine analysis trigger
  React.useEffect(() => {
    if (!over || !engineOn) { if (!engineOn) { stop(); resetEval(); } return; }
    const src = reviewIdx >= 0 && snapshots[reviewIdx] ? snapshots[reviewIdx] : finalPositionRef.current;
    if (!src) return;
    analyse(src.fen, src.turn);
  }, [engineOn, over, sfReady, reviewIdx, snapshots, analyse, stop, resetEval]);

  React.useEffect(() => { if (!over) { stop(); setEngineOn(false); } }, [over, stop]);

  // Re-run engine analysis on each cheater turn
  React.useEffect(() => {
    if (cheaterActive && cheaterColor === turn && !over) {
      analyse(toFEN(board, turn, moved, lm, hmc, fmn), turn);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [turn, cheaterActive]);

  // Game init
  React.useEffect(() => {
    setWhiteHand([]);
    setBlackHand([]);
    lastDrawRound.current = 0;
    setDealPhase('done');
    setAnalysisArrows([]);
    if (gameKey === 0) startAbortCountdown(() => { blackMovedRef.current = false; });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameKey]);

  React.useEffect(() => {
    if (!guestProfilesReady) return;
    void bootstrapAuthoritativeMatch();
  }, [guestProfilesReady, bootstrapAuthoritativeMatch]);

  React.useEffect(() => {
    writeStoredActiveMatchId(authoritativeMatchId);
    setAnalysisArrows([]);
  }, [authoritativeMatchId]);

  React.useEffect(() => {
    if (!authoritativeMatchId) {
      openedBoardMatchRef.current = null;
      return;
    }
    if (openedBoardMatchRef.current === authoritativeMatchId) {
      return;
    }
    openedBoardMatchRef.current = authoritativeMatchId;
    const boardRouteRequested = pathname.startsWith('/match/') || Boolean(requestedMatchIdRef.current);
    console.log('[DEBUG] effect2212: authoritativeMatchId=', authoritativeMatchId, 'pathname=', pathname, 'boardRouteRequested=', boardRouteRequested, 'hostedRuntime=', hostedRuntime, 'setting activePage to', hostedRuntime ? 'Match' : 'Play');
    if (!hostedRuntime || boardRouteRequested) {
      setActivePage(hostedRuntime ? 'Match' : 'Play');
    }
  }, [authoritativeMatchId, hostedRuntime, pathname, setActivePage]);

  React.useEffect(() => {
    setMatchDestinationNotice('');
  }, [activePage, authoritativeMatchId, authoritativeStatus, viewerSeat]);

  const {
    boardStatusLabel,
    hasPrimaryAccountSession,
    showSocialNav,
    showAdminNav,
    primaryNavItems,
    utilityGroups,
    secondaryNavItems,
    activeSecondaryNav,
    showReturnToMatch,
    showPlayHub,
    showBoardSurface,
    controlledSeat,
    topSeat,
    bottomSeat,
    topHand,
    bottomHand,
    whiteSeatBadge,
    blackSeatBadge,
    showHostedSoloBanner,
    showHostedReconnectWarning,
    activeDisconnectGraceFor,
    disconnectGraceDeadlineLabel,
    whitePresenceLabel,
    blackPresenceLabel,
    activeFinishReason,
    activeFinishReasonLabel,
    displayedWhiteName,
    displayedBlackName,
    disconnectGraceBanner,
    displayedWhiteRating,
    displayedBlackRating,
    activeMatchRoomMeta,
    activeMatchModeLabel,
    activeMatchQueueLabel,
    canCreateDirectRematch,
    canQueueSameLane,
    activeMatchRoleLabel,
    activeMatchRouteLabel,
    finishedPrimaryActionLabel,
    finishedSecondaryActionLabel,
    activeLiveMatchUrl,
    activeReplayPageUrl,
    topSeatBadge,
    bottomSeatBadge,
    topPlayerName,
    bottomPlayerName,
    topPlayerRating,
    bottomPlayerRating,
    topPlayerClock,
    bottomPlayerClock,
    topClockTicking,
    bottomClockTicking,
    shellPageMeta,
    actorSeatForHostedControls,
    actorSeatPlainLabel,
    controlSender,
    hostedActionLocked,
    canRespondToDrawOffer,
    actorSeatLabel,
    hostedSpectator,
    visibleSocialAlert,
    dismissSocialAlert,
    handleSocialAlertAction,
  } = useMatchNav({
    authoritativeMatchId,
    authoritativeStatus,
    hostedRuntime,
    viewerSeat,
    authoritativeLive,
    authoritativeRematchBusy,
    primaryAccountIdentity,
    activePage,
    friendsAttentionCount,
    inboxUnreadCount,
    whiteHand,
    blackHand,
    over,
    winner,
    authoritativeFinishReason,
    hmc,
    stale,
    insuf,
    mate,
    whiteProfile,
    blackProfile,
    matchSeatMeta,
    authoritativeDisconnectGraceFor,
    authoritativeDisconnectGraceDeadline,
    authoritativeWhiteConnected,
    authoritativeBlackConnected,
    timeW,
    timeB,
    tickingState,
    clockActive,
    socialAlert,
    dismissedSocialAlertIdsRef,
    turn,
    openLiveMatch,
    setActivePage,
    setSocialAlert,
  });

  // Card draw logic
  React.useEffect(() => {
    if (dealPhase !== 'done') return;
    if (authoritativeMatchId) return;
    if (lastDrawRound.current === roundNumber) return;

    if (roundNumber === INITIAL_DEAL_ROUND) {
      lastDrawRound.current = roundNumber;
      setWhiteHand(Array.from({ length: 3 }, () => drawRandomCard('w')));
      setBlackHand(Array.from({ length: 3 }, () => drawRandomCard('b')));
      setLastDrawAnim({ color: 'white', rarity: 'trash' });
      setTimeout(() => setLastDrawAnim(null), 3000);
      return;
    }

    if (roundNumber < DRAW_FROM) return;
    if ((roundNumber - DRAW_FROM) % DRAW_EVERY !== 0) return;

    lastDrawRound.current = roundNumber;
    const hasFused = board.some(r => r.some(p => p?.fusedWith));
    const drawSafe = (side: string) => {
      let card = drawRandomCard(side);
      let attempts = 0;
      while (hasFused && card.mechanic === 'cheater' && attempts < 20) {
        card = drawRandomCard(side); attempts++;
      }
      return card;
    };
    const wCard = drawSafe('w');
    const bCard = drawSafe('b');
    setWhiteHand(h => h.length < MAX_HAND_SIZE ? [...h, wCard] : h);
    setBlackHand(h => h.length < MAX_HAND_SIZE ? [...h, bCard] : h);
    setLastDrawAnim({ color: 'white', rarity: wCard.rarity });
    setTimeout(() => setLastDrawAnim(null), 2000);
  }, [roundNumber, dealPhase, authoritativeMatchId]);

  // Unfreeze/un-borrow/un-shield/un-invisible pieces when their turn ends
  React.useEffect(() => {
    if (!authoritativeLive) {
      const justMovedColor: PieceColor = OPP[turn];
      setBoard(prev => {
      let changed = false;
      const nb: Board = prev.map(row => row.map(p => p ? { ...p } : null));

      for (let r = 0; r < 8; r++) {
        for (let c = 0; c < 8; c++) {
          const p = nb[r][c];
          if (!p) continue;
          if (p.frozen && p.color === justMovedColor) {
            nb[r][c] = { ...p, frozen: false }; changed = true;
          }
          if (p.borrowed && p.color !== turn) {
            nb[r][c] = { type: p.type, color: turn } as Piece; changed = true;
          }
          if (p.shielded && p.shieldTurn !== undefined && fmnRef.current >= p.shieldTurn) {
            nb[r][c] = { ...p, shielded: false, shieldTurn: undefined }; changed = true;
          }
        }
      }
      return changed ? nb : prev;
    });

    // Ghost piece expiry:
    // activate -> owner plays (move 1) -> opponent plays -> owner plays (move 2) + piece reappears
    // roundsLeft=1: decrements when owner's turn starts; expires on opponent's turn after roundsLeft<=0
    const ghost = ghostRef.current;
    if (ghost) {
      if (ghost.ownerColor === turn) {
        // Owner's turn starting - decrement counter
        const updated = { ...ghost, roundsLeft: ghost.roundsLeft - 1 };
        setGhostPiece(updated);
        ghostRef.current = updated;
      } else if (ghost.roundsLeft <= 0) {
        // Opponent's turn starting after owner played - expire now
        setGhostPiece(null);
        ghostRef.current = null;
        setBoard(prev => {
          const nb = prev.map(r => r.map(p => p ? { ...p } : null)) as Board;
          const occupant = nb[ghost.row][ghost.col];
          if (occupant) {
            setCardMsg(`👁️ ${ghost.piece.type} reappeared on an occupied square and was destroyed!`);
            setTimeout(() => setCardMsg(''), 3000);
            return nb;
          } else {
            nb[ghost.row][ghost.col] = { ...ghost.piece };
            setCardMsg(`👁️ ${ghost.piece.type} reappears!`);
            setTimeout(() => setCardMsg(''), 2000);
            return nb;
          }
        });
      }
    }
    }

    // Process bombs at start of each turn
    if (!authoritativeLive) {
      processBombs(turn, boardRef.current);
    }

    // Countdown fog zones — decrement once per full round (when white is about to move)
      if (!authoritativeLive && turn === 'white') {
        setFogZones(prev => {
          const next = prev
            .map(z => ({ ...z, turnsLeft: z.turnsLeft - 1 }))
          .filter(z => z.turnsLeft > 0);
        if (next.length < prev.length) {
          setCardMsg('🌤️ Fog of War lifted!');
          setTimeout(() => setCardMsg(''), 2500);
        }
        return next;
      });
    }
  }, [turn, processBombs, authoritativeLive]);

  // ── Fusion attack helper ───────────────────────────────────────────────────
  const isAttackedWithFusion = React.useCallback((b: Board, row: number, col: number, byColor: PieceColor): boolean => {
    // Standard attack check first
    if (isAttacked(b, row, col, byColor)) return true;
    // Also check fused pieces: temporarily treat them as their secondary type
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = b[r][c];
        if (!p || p.color !== byColor || !p.fusedWith) continue;
        // Try secondary type
        const tempBoard: Board = b.map(row2 => row2.map(p2 => p2 ? { ...p2 } : null));
        tempBoard[r][c] = { ...p, type: p.fusedWith, fusedWith: undefined };
        if (isAttacked(tempBoard, row, col, byColor)) return true;
      }
    }
    return false;
  }, []);

  // ── End-game helper ────────────────────────────────────────────────────────
  const checkEndGame = React.useCallback((
    nb: Board,
    next: PieceColor,
    newMv: Set<string>,
    newLm: { from: Sq; to: Sq } | null,
    newHmc: number,
    newPh: string[],
    posKey: string,
    fen: string,
    t: PieceColor,
  ) => {
    const st = gameStatus(nb, next, newLm, newMv);
    // Override check/mate/stale to account for fused pieces
    const kp = findKing(nb, next);
    const opp2 = next === 'white' ? 'black' : 'white';
    const fusionCheck = kp ? isAttackedWithFusion(nb, kp.row, kp.col, opp2) : false;
    const isCheck = st.isCheck || fusionCheck;

    // Fusion-aware mate/stale: check if ANY move exists that escapes check
    let fusionHasLegal = false;
    if (fusionCheck && !st.isMate) {
      // Engine thinks there are legal moves but fusion check may invalidate them all
      outer: for (let r = 0; r < 8; r++) {
        for (let c = 0; c < 8; c++) {
          const p = nb[r][c];
          if (!p || p.color !== next) continue;
          let moves: Sq[];
          if (p.fusedWith) {
            const b1 = nb.map(row => row.map(p2 => p2 ? { ...p2 } : null));
            b1[r][c] = { ...p, type: p.type, fusedWith: undefined };
            const b2 = nb.map(row => row.map(p2 => p2 ? { ...p2 } : null));
            b2[r][c] = { ...p, type: p.fusedWith, fusedWith: undefined };
            const m1 = legalMoves(b1, r, c, newLm, newMv);
            const m2 = legalMoves(b2, r, c, newLm, newMv);
            const seen = new Set<string>();
            moves = [...m1, ...m2].filter(sq => {
              const key = `${sq.row},${sq.col}`;
              if (seen.has(key)) return false;
              seen.add(key); return true;
            });
          } else {
            moves = legalMoves(nb, r, c, newLm, newMv);
          }
          for (const sq of moves) {
            const test = nb.map(row => row.map(p2 => p2 ? { ...p2 } : null));
            test[sq.row][sq.col] = test[r][c];
            test[r][c] = null;
            const myKp = findKing(test, next);
            if (myKp && !isAttackedWithFusion(test, myKp.row, myKp.col, opp2)) {
              fusionHasLegal = true;
              break outer;
            }
          }
        }
      }
    } else {
      fusionHasLegal = !fusionCheck;
    }

    const isFusionMate  = isCheck && !st.isMate && !fusionHasLegal;
    const isMate  = st.isMate  || isFusionMate;
    const isStale = st.isStale;

    setCheck(isCheck);
    setMate(isMate);
    setStale(isStale);
    const im = insuffMat(nb);
    setInsuf(im);
    const isGameOver =
      newHmc >= 100 ||
      threefold(newPh, posKey) ||
      isMate || isStale || im;
    if (isGameOver) {
      finalPositionRef.current = { fen, turn: next };
      setOver(true);
      if      (newHmc >= 100)            setWinner('draw');
      else if (threefold(newPh, posKey)) setWinner('draw');
      else if (isMate)                   setWinner(t);
      else if (isStale || im)            setWinner('draw');
    }
  }, [isAttackedWithFusion]);

  // ── Handle lava landing ────────────────────────────────────────────────────
  const handleLavaLanding = React.useCallback((tr: number, tc: number, pieceType: PieceType | undefined) => {
    const lava = lavaSquaresRef.current.find(l => l.row === tr && l.col === tc);
    if (lava && pieceType !== 'king') {
      setLavaExploding(prev => [...prev, { row: tr, col: tc }]);
      fireCardAnim('lava_kill', `Piece incinerated on ${FILES[tc]}${RANKS[tr]}`);
      setTimeout(() => {
        setBoard(b2 => { const nb2 = cloneBoard(b2); nb2[tr][tc] = null; return nb2; });
        setLavaSquares(prev =>
          prev
            .filter(l => !(l.row === tr && l.col === tc))
            .map(l => ({ ...l, movesLeft: l.movesLeft - 1 }))
            .filter((l): l is LavaSquare => l.movesLeft > 0)
        );
        setLavaExploding(prev => prev.filter(l => !(l.row === tr && l.col === tc)));
      }, 700);
      return true;
    }
    return false;
  }, []);

  const canSubmitAuthoritativeMove = React.useCallback((fr: number, fc: number, tr: number, tc: number) => {
    const matchId = authoritativeMatchIdRef.current;
    if (!matchId) return false;
    if (cardPending || selectedCard || promo || promoPicker || cardPromo || jokerPicker) return false;
      if (ghostRef.current) return false;

    const piece = boardRef.current[fr]?.[fc];
    const target = boardRef.current[tr]?.[tc];
    if (!piece) return false;
    if (hostedRuntime) {
      if (viewerSeatRef.current !== piece.color) return false;
      if (turnRef.current !== piece.color) return false;
    }
    if (piece.fusedWith || piece.invisible || piece.shielded || piece.frozen) return false;
    if (target?.fusedWith || target?.shielded || target?.invisible) return false;
    return true;
    }, [cardPending, selectedCard, promo, promoPicker, cardPromo, jokerPicker, hostedRuntime]);
  premoveCanSubmitRef.current = canSubmitAuthoritativeMove;

  // ── doMove ─────────────────────────────────────────────────────────────────
  const doMove = React.useCallback((fr: number, fc: number, tr: number, tc: number, forcePromo?: PieceType) => {
    if (overRef.current) return;
    const matchId = authoritativeMatchIdRef.current;
    const liveBoard = boardRef.current;
    const livePiece = liveBoard[fr]?.[fc];
    const liveGhost = ghostRef.current;
    const isAuthoritativePromotion =
      !!matchId &&
      !!livePiece &&
      livePiece.type === 'pawn' &&
      (tr === 0 || tr === 7);

    if (
      matchId &&
      liveGhost &&
      liveGhost.row === fr &&
      liveGhost.col === fc &&
      (!hostedRuntime || (viewerSeatRef.current === liveGhost.ownerColor && turnRef.current === liveGhost.ownerColor))
    ) {
      const backendMoveIntent: Omit<Extract<PlayerIntent, { type: 'make_move' }>, 'matchId'> = {
        type: 'make_move',
        ...authoritativeActorForColor(turnRef.current),
        from: { row: fr, col: fc },
        to: { row: tr, col: tc },
      };

      void applyIntent(matchId, backendMoveIntent).then(snapshot => {
        applyAuthoritativeSnapshot(snapshot);
      }).catch(err => {
        const message = err instanceof Error ? err.message : 'Backend rejected invisible move';
        setCardMsg(`Backend invisible move failed: ${message}`);
        setTimeout(() => setCardMsg(''), 2500);
      });
      return;
    }

    if (isAuthoritativePromotion && !forcePromo) {
      setPromo({
        row: tr,
        col: tc,
        color: livePiece.color,
        fromCol: fc,
        from: { row: fr, col: fc },
        to: { row: tr, col: tc },
        authoritativeMatchId: matchId,
        moved: movedRef.current,
        lm: { from: { row: fr, col: fc }, to: { row: tr, col: tc } },
        hmc: hmcRef.current,
        fmn: fmnRef.current,
        turn: turnRef.current,
      });
      return;
    }

    if (canSubmitAuthoritativeMove(fr, fc, tr, tc)) {
      if (matchId) {
        const backendMoveIntent: Omit<Extract<PlayerIntent, { type: 'make_move' }>, 'matchId'> = {
          type: 'make_move',
          ...authoritativeActorForColor(turnRef.current),
          from: { row: fr, col: fc },
          to: { row: tr, col: tc },
          promotion: forcePromo,
        };

        void applyIntent(matchId, backendMoveIntent).then(snapshot => {
          applyAuthoritativeSnapshot(snapshot);
        }).catch(err => {
          const message = err instanceof Error ? err.message : 'Backend rejected move';
          setCardMsg(`Backend move failed: ${message}`);
          setTimeout(() => setCardMsg(''), 2500);
        });
        return;
      }
    }
    const b    = boardRef.current;
    const t    = turnRef.current;
    const mv   = movedRef.current;
    const h    = hmcRef.current;
    const f    = fmnRef.current;
    const ph   = posHistRef.current;
    const dm   = doubleMoveRef.current;

    const nb = cloneBoard(b);

    // Ghost piece moves — it's NOT on the board, just update ghostRef position
    const ghost = ghostRef.current;
    if (ghost && ghost.ownerColor === t && ghost.row === fr && ghost.col === fc) {
      // Move the ghost to new position (board unchanged — ghost is never on it)
      const newGhost = { ...ghost, row: tr, col: tc };
      setGhostPiece(newGhost);
      ghostRef.current = newGhost;

      // Materialise if: giving check OR (move 2 and capturing an enemy piece)
      const testBoard = cloneBoard(b);
      testBoard[tr][tc] = ghost.piece; // temporarily place to test check
      const oppKp = findKing(testBoard, OPP[t]);
      const givesCheck = !!(oppKp && isAttackedWithFusion(testBoard, oppKp.row, oppKp.col, t));
      const targetPiece = nb[tr][tc];
      const isCapture = !!(targetPiece && targetPiece.color !== t);
      const isMove2 = ghost.roundsLeft <= 0;
      if (givesCheck || (isMove2 && isCapture)) {
        const captured = targetPiece;
        nb[tr][tc] = { ...ghost.piece };
        setGhostPiece(null); ghostRef.current = null;
        const reason = givesCheck ? 'giving check' : `captured ${captured?.type}`;
        setCardMsg(`👁️ ${ghost.piece.type} materialised (${reason})!`);
        setTimeout(() => setCardMsg(''), 2500);
      }

      const note2  = `${FILES[fc]}${RANKS[fr]}→${FILES[tc]}${RANKS[tr]}`;
      const newMv2 = new Set(mv).add(`${fr}-${fc}`);
      const newLm2 = { from: { row: fr, col: fc }, to: { row: tr, col: tc } };
      const newFmn2 = t === 'black' ? f + 1 : f;
      const next2: PieceColor = OPP[t];
      setBoard(nb); setMoved(newMv2); setLm(newLm2);
      setFmn(newFmn2); setHmc(h);
      setMovHist(prev => {
        const nx = [...prev];
        if (t === 'white') nx.push({ n: `${nx.length + 1}.`, w: note2 });
        else { const last = nx[nx.length - 1]; if (last && !last.b) nx[nx.length - 1] = { ...last, b: note2 }; }
        return nx;
      });
      resetCardUsed(next2);
      setTurn(next2); setTicking(next2);
      setSel(null); setHints([]);
      return;
    }

    const piece = nb[fr][fc];
    if (!piece) return;

    const cap  = !!nb[tr][tc];
    const isEP = piece.type === 'pawn' && tc !== fc && !nb[tr][tc];

    if (cap && nb[tr][tc]?.shielded) {
      nb[tr][tc] = { ...nb[tr][tc]!, shielded: false };
      setCardMsg('🛡️ Shield blocked the capture!');
      setTimeout(() => setCardMsg(''), 2000);
      return;
    }

    const note = moveNotation(nb, fr, fc, tr, tc, piece, cap || isEP);

    if (piece.type === 'king' && Math.abs(tc - fc) === 2) {
      if (tc === 6) { nb[tr][5] = nb[tr][7]; nb[tr][7] = null; }
      else          { nb[tr][3] = nb[tr][0]; nb[tr][0] = null; }
    }

    if (isEP) nb[fr][tc] = null;
    nb[tr][tc] = { ...piece };
    nb[fr][fc] = null;

    // ── Parasite: if a captured piece was linked, kill the host too ──────────
    // Also: if the host piece was captured, kill the linked target
    if (cap || isEP) {
      // piece moved TO (tr,tc) killing whatever was there — check if killed piece had a parasiteTarget
      const killedPiece = b[tr][tc];
      if (killedPiece?.parasiteTarget) {
        const [pr, pc] = killedPiece.parasiteTarget.split(',').map(Number);
        if (nb[pr]?.[pc] && nb[pr][pc]!.type !== 'king') {
          // Check if removing the linked piece would leave its own king in check
          const linkedColor = nb[pr][pc]!.color;
          const testNb = cloneBoard(nb);
          testNb[pr][pc] = null;
          const linkedKp = findKing(testNb, linkedColor);
          const linkedOpp = OPP[linkedColor];
          if (linkedKp && isAttackedWithFusion(testNb, linkedKp.row, linkedKp.col, linkedOpp)) {
            // Block the capture — parasite would cause illegal check
            setBoard(b); // revert
            setCardMsg(`🦠 Cannot capture — parasite would leave a king in check!`);
            setTimeout(() => setCardMsg(''), 2500);
            setSel(null); setHints([]);
            return;
          }
          nb[pr][pc] = null;
          setCardMsg(`🦠 Parasite triggered! ${killedPiece.type} died → linked piece destroyed too!`);
          setTimeout(() => setCardMsg(''), 3000);
        }
      }
      // Check if any surviving piece has parasiteTarget pointing to (tr,tc) — the square just captured
      for (let pr = 0; pr < 8; pr++) {
        for (let pc = 0; pc < 8; pc++) {
          const pp = nb[pr][pc];
          if (pp?.parasiteTarget) {
            const [tpr, tpc] = pp.parasiteTarget.split(',').map(Number);
            if (tpr === tr && tpc === tc) {
              if (pp.type !== 'king') {
                // Check if removing the host would leave its king in check
                const testNb2 = cloneBoard(nb);
                testNb2[pr][pc] = null;
                const hostKp = findKing(testNb2, pp.color);
                const hostOpp = OPP[pp.color];
                if (hostKp && isAttackedWithFusion(testNb2, hostKp.row, hostKp.col, hostOpp)) {
                  setBoard(b);
                  setCardMsg(`🦠 Cannot capture — parasite would leave a king in check!`);
                  setTimeout(() => setCardMsg(''), 2500);
                  setSel(null); setHints([]);
                  return;
                }
                nb[pr][pc] = null;
                setCardMsg(`🦠 Parasite triggered! Linked enemy piece died → your host piece destroyed too!`);
                setTimeout(() => setCardMsg(''), 3000);
              }
            }
          }
        }
      }
    }

    // ── Parasite: update parasiteTarget coords when the linked piece moves ────
    // Any host piece pointing to (fr,fc) must update its target to (tr,tc)
    for (let pr = 0; pr < 8; pr++) {
      for (let pc = 0; pc < 8; pc++) {
        const pp = nb[pr][pc];
        if (pp?.parasiteTarget) {
          const [tpr, tpc] = (pp.parasiteTarget as string).split(',').map(Number);
          if (tpr === fr && tpc === fc) {
            nb[pr][pc] = { ...pp, parasiteTarget: `${tr},${tc}` };
          }
        }
      }
    }

    if (dm?.movesLeft === 2) {
      const oppKp = findKing(nb, OPP[t]);
      if (oppKp && isAttackedWithFusion(nb, oppKp.row, oppKp.col, t)) {
        setCardMsg('🚫 First double move cannot put enemy king in check!');
        setTimeout(() => setCardMsg(''), 2500);
        setSel(null); setHints([]);
        return;
      }
    }

    const newMv  = new Set(mv).add(`${fr}-${fc}`);
    const newLm  = { from: { row: fr, col: fc }, to: { row: tr, col: tc } };
    const newHmc = (piece.type === 'pawn' || cap || isEP) ? 0 : h + 1;
    const newFmn = t === 'black' ? f + 1 : f;

    if (piece.type === 'pawn' && (tr === 7 || tr === 0)) {
      if (forcePromo) {
        nb[tr][tc] = { type: forcePromo, color: piece.color };
      } else {
        setBoard(nb);
        setPromo({ row: tr, col: tc, color: piece.color, fromCol: fc, turn: t, note, moved: newMv, lm: newLm, hmc: newHmc, fmn: newFmn });
        return;
      }
    }

    if (dm?.movesLeft === 1 && dm.trackedSq) {
      const ts = dm.trackedSq;
      if (dm.type === 'same' && (fr !== ts.row || fc !== ts.col)) {
        setCardMsg(`🏃 Solo: must move the SAME piece at ${FILES[ts.col]}${RANKS[ts.row]}!`);
        setTimeout(() => setCardMsg(''), 2500);
        setSel(null); setHints([]);
        return;
      }
      if (dm.type === 'diff' && fr === ts.row && fc === ts.col) {
        setCardMsg('👥 Twin: must move a DIFFERENT piece!');
        setTimeout(() => setCardMsg(''), 2500);
        setSel(null); setHints([]);
        return;
      }
    }

    if (dm && dm.movesLeft > 0) {
      const newMovesLeft = dm.movesLeft - 1;

      if (newMovesLeft > 0) {
        setBoard(nb);
        setMoved(newMv);
        setLm(newLm);
        setHmc(newHmc);
        setFmn(newFmn);

        handleLavaLanding(tr, tc, piece.type);

        const newDm: DoubleMove = { ...dm, movesLeft: newMovesLeft, trackedSq: { row: tr, col: tc }, firstNote: note };
        doubleMoveRef.current = newDm;
        setDoubleMove(newDm);

        setCardMsg(
          dm.type === 'same'
            ? `🏃 Solo: now move the SAME piece again! (${FILES[tc]}${RANKS[tr]})`
            : `👥 Twin: now move a DIFFERENT piece! (not ${FILES[tc]}${RANKS[tr]})`
        );
        setTimeout(() => setCardMsg(''), 4000);
        setSel(null); setHints([]);
        return;
      }

      const firstNote = dm.firstNote ?? '?';
      setMovHist(prev => {
        const nx = [...prev];
        const combined = `${firstNote}+${note}`;
        if (t === 'white') {
          nx.push({ n: `${nx.length + 1}.`, w: combined });
        } else {
          const last = nx[nx.length - 1];
          if (last && !last.b) nx[nx.length - 1] = { ...last, b: combined };
          else nx.push({ n: `${nx.length + 1}.`, b: combined });
        }
        return nx;
      });
      doubleMoveRef.current = null;
      setDoubleMove(null);
    }

    // ── Shield breaks if the moved piece gives check ─────────────────────────
    // A shielded piece that delivers check loses its shield (it can now be captured)
    const movedPiece = nb[tr][tc];
    if (movedPiece?.shielded) {
      const oppKingPos = findKing(nb, OPP[t]);
      if (oppKingPos && isAttackedWithFusion(nb, oppKingPos.row, oppKingPos.col, t)) {
        nb[tr][tc] = { ...movedPiece, shielded: false, shieldTurn: undefined };
        setCardMsg('🛡️ Shield shattered — giving check broke the protection!');
        setTimeout(() => setCardMsg(''), 2500);
      }
    }

    const wasDoubleMoveFinal = dm !== null && dm.movesLeft === 1;
    const next: PieceColor = OPP[t];
    resetCardUsed(next);

    const posKey = positionKey(nb, next, newMv, newLm);
    const newPh  = [...ph, posKey];
    const fen    = toFEN(nb, next, newMv, newLm, newHmc, newFmn);
    const snap: Snapshot = { board: nb.map(r => [...r]), turn: next, moved: newMv, lm: newLm, hmc: newHmc, fmn: newFmn, fen };

    setSnapshots(prev => [...prev, snap]);
    setBoard(nb);
    setMoved(newMv);
    setLm(newLm);
    setHmc(newHmc);
    setFmn(newFmn);
    setPosHist(newPh);

    if (!wasDoubleMoveFinal) {
      setMovHist(prev => {
        const nx = [...prev];
        if (t === 'white') nx.push({ n: `${nx.length + 1}.`, w: note });
        else {
          const last = nx[nx.length - 1];
          if (last && !last.b) nx[nx.length - 1] = { ...last, b: note };
        }
        return nx;
      });
    }

    handleLavaLanding(tr, tc, piece.type);

    if (t === 'white' && !blackMovedRef.current) {
      startAbortCountdown();
      setTicking(null);
    } else if (t === 'black' && !blackMovedRef.current) {
      stopAbortCountdown();
      blackMovedRef.current = true;
      setClockActive(true);
      setTicking(next);
    } else {
      setTicking(next);
    }

    setTurn(next);
    checkEndGame(nb, next, newMv, newLm, newHmc, newPh, posKey, fen, t);
    setDrawOffer(null);
  }, [resetCardUsed, startAbortCountdown, stopAbortCountdown, setTicking, checkEndGame, handleLavaLanding, canSubmitAuthoritativeMove, applyAuthoritativeSnapshot, authoritativeActorForColor, hostedRuntime]);

  const doPromo = React.useCallback((type: PieceType) => {
    if (!promo) return;
    if (promo.authoritativeMatchId && promo.from && promo.to) {
      const backendMoveIntent: Omit<Extract<PlayerIntent, { type: 'make_move' }>, 'matchId'> = {
        type: 'make_move',
        ...authoritativeActorForColor(promo.turn ?? turn),
        from: promo.from,
        to: promo.to,
        promotion: type,
      };

      void applyIntent(promo.authoritativeMatchId, backendMoveIntent).then(snapshot => {
        setPromo(null);
        applyAuthoritativeSnapshot(snapshot);
      }).catch(err => {
        const message = err instanceof Error ? err.message : 'Backend promotion failed';
        setCardMsg(`Backend promotion failed: ${message}`);
        setTimeout(() => setCardMsg(''), 2500);
      });
      return;
    }

    const nb = cloneBoard(board);
    nb[promo.row][promo.col] = { type, color: promo.color };
    const newMv  = promo.moved;
    const newLm  = promo.lm;
    const newHmc = promo.hmc;
    const t      = promo.turn ?? turn;
    const newFmn = t === 'black' ? promo.fmn + 1 : promo.fmn;
    const PROMO_CHAR: Record<PieceType, string> = { queen:'Q', rook:'R', bishop:'B', knight:'N', king:'', pawn:'' };
    const fullNote = `${promo.note ?? (FILES[promo.col] + RANKS[promo.row])}=${PROMO_CHAR[type]}`;

    setBoard(nb);
    setMoved(newMv);
    setLm(newLm);
    setHmc(newHmc);
    setFmn(newFmn);
    setPromo(null);

    setMovHist(prev => {
      const nx = [...prev];
      if (t === 'white') nx.push({ n: `${nx.length + 1}.`, w: fullNote });
      else {
        const last = nx[nx.length - 1];
        if (last && !last.b) nx[nx.length - 1] = { ...last, b: fullNote };
        else nx.push({ n: `${nx.length + 1}.`, b: fullNote });
      }
      return nx;
    });

    const next: PieceColor = OPP[t];
    resetCardUsed(next);
    const posKey = positionKey(nb, next, newMv, newLm);
    const newPh  = [...posHist, posKey];
    setPosHist(newPh);
    setTicking(next);
    setTurn(next);
    setClockActive(true);

    const fen  = toFEN(nb, next, newMv, newLm, newHmc, newFmn);
    const snap: Snapshot = { board: nb.map(r => [...r]), turn: next, moved: newMv, lm: newLm, hmc: newHmc, fmn: newFmn, fen };
    setSnapshots(prev => [...prev, snap]);

    checkEndGame(nb, next, newMv, newLm, newHmc, newPh, posKey, fen, t);
    if (!over) finalPositionRef.current = { fen, turn: next };
  }, [promo, board, turn, posHist, resetCardUsed, setTicking, checkEndGame, over, applyAuthoritativeSnapshot, authoritativeActorForColor]);

  // ── Card helpers ───────────────────────────────────────────────────────────
  const removeCardFromHand = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    if (playerColor === 'white') setWhiteHand(h => h.filter(c => c.id !== card.id));
    else                         setBlackHand(h => h.filter(c => c.id !== card.id));
  }, []);

  const finishCardUse = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    removeCardFromHand(card, playerColor);
    cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
    setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
    setCardPending(null);
    setSelectedCard(null);
    pendingCardUseRef.current.delete(card.id);
  }, [removeCardFromHand]);

  const jokerPickerRef = React.useRef<typeof jokerPicker>(null);
  React.useEffect(() => { jokerPickerRef.current = jokerPicker; }, [jokerPicker]);

  const cancelCard = React.useCallback(() => {
    if (cardPending) pendingCardUseRef.current.delete(cardPending.card.id);
    // Always clean up joker lock using the ref (works even without cardPending)
    const jp = jokerPickerRef.current;
    if (jp) pendingCardUseRef.current.delete(jp.card.id);
    setJokerPicker(null);
    setCardPending(null);
    setCardMsg('');
    setPromoPicker(null);
    setCardPromo(null);
    setSelectedCard(null);
  }, [cardPending]);

  const getSafeTransforms = React.useCallback((
    b: Board,
    row: number,
    col: number,
    transforms: PieceType[],
    playerColor: PieceColor,
  ): PieceType[] => {
    const opp = OPP[playerColor];
    const piece = b[row][col]!;
    return transforms.filter(t => {
      const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
      nb[row][col] = { ...piece, type: t };
      const kp  = findKing(nb, playerColor);
      const okp = findKing(nb, opp);
      return (
        !(kp  && isAttackedWithFusion(nb, kp.row,  kp.col,  opp))        &&
        !(okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor))
      );
    });
  }, []);

  // Returns union of legal moves for a fused piece (type1 moves + type2 moves)
  const getFusedMoves = React.useCallback((
    b: Board,
    row: number,
    col: number,
    type1: PieceType,
    type2: PieceType,
  ): Sq[] => {
    const piece = b[row][col]!;
    const boardAs1: Board = b.map(r => r.map(p => p ? { ...p } : null));
    boardAs1[row][col] = { ...piece, type: type1, fusedWith: undefined };
    const boardAs2: Board = b.map(r => r.map(p => p ? { ...p } : null));
    boardAs2[row][col] = { ...piece, type: type2, fusedWith: undefined };
    const moves1 = legalMoves(boardAs1, row, col, lm, moved);
    const moves2 = legalMoves(boardAs2, row, col, lm, moved);
    const seen = new Set<string>();
    return [...moves1, ...moves2].filter(sq => {
      const key = `${sq.row},${sq.col}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }, [lm, moved]);

  // isAttacked that accounts for fusedWith — checks both piece types attack the king sq
  // Returns null if fusion is valid, or an error string if it's redundant/useless
  const checkFusionRedundancy = React.useCallback((
    typeA: PieceType, // piece being consumed (step 1)
    typeB: PieceType, // piece surviving (step 2)
  ): string | null => {
    // Same type always redundant
    if (typeA === typeB) return `⚗️ Can't fuse two ${typeA}s — same piece type adds nothing!`;
    // Queen already moves like rook and bishop
    if ((typeA === 'queen' && typeB === 'rook') || (typeA === 'rook' && typeB === 'queen'))
      return '⚗️ Queen already moves like a rook — nothing to gain!';
    if ((typeA === 'queen' && typeB === 'bishop') || (typeA === 'bishop' && typeB === 'queen'))
      return '⚗️ Queen already moves like a bishop — nothing to gain!';
    // Queen + pawn: queen already outclasses pawn movement entirely
    if ((typeA === 'queen' && typeB === 'pawn') || (typeA === 'pawn' && typeB === 'queen'))
      return '⚗️ Queen already outclasses pawn movement — nothing to gain!';
    // Bishop + bishop: locked to same color — useless
    if (typeA === 'bishop' && typeB === 'bishop')
      return '⚗️ Bishops are locked to their square color — fusing them adds no new movement!';
    return null; // valid (bishop+rook handled separately as queen promotion)
  }, []);

  const activateDoubleMove = React.useCallback((type: 'diff' | 'same', card: GameCard, playerColor: PieceColor) => {
    const newDm: DoubleMove = { type, movesLeft: 2, trackedSq: null };
    doubleMoveRef.current = newDm;
    setDoubleMove(newDm);
    setCardMsg(
      type === 'diff'
        ? '👥 Twin active! Make your first move with any piece, then move a DIFFERENT piece.'
        : '🏃 Solo active! Make your first move, then move the SAME piece again.'
    );
    setTimeout(() => setCardMsg(''), 4000);
    finishCardUse(card, playerColor);
  }, [finishCardUse]);

  // ── NEW: Joker picker handlers ─────────────────────────────────────────────
  const openJokerPicker = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    setJokerPicker({ card, playerColor, filterRarity: 'all', transforming: false });
    setSelectedCard(null);
    pendingCardUseRef.current.add(card.id);
  }, []);

  const applyJokerTransform = React.useCallback((jokerCard: GameCard, playerColor: PieceColor, chosenTemplate: Omit<GameCard, 'id'>) => {
    // Animate transform then replace card
    setJokerPicker(prev => prev ? { ...prev, transforming: true } : null);
    setTimeout(() => {
      if (authoritativeMatchIdRef.current) {
        const transformIntent: Omit<Extract<PlayerIntent, { type: 'select_target' }>, 'matchId'> = {
          type: 'select_target',
          ...authoritativeActorForColor(playerColor),
          selectionId: chosenTemplate.mechanic,
        };
        void applyIntent(authoritativeMatchIdRef.current, transformIntent).then(snapshot => {
          applyAuthoritativeSnapshot(snapshot);
          cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: false };
          setCardUsedBy(prev => ({ ...prev, [playerColor]: false }));
          pendingCardUseRef.current.delete(jokerCard.id);
          setJokerPicker(null);
          setCardMsg(`🃏 Joker transformed into ${chosenTemplate.name} ${chosenTemplate.icon}!`);
          setTimeout(() => setCardMsg(''), 3000);
        }).catch(err => {
          pendingCardUseRef.current.delete(jokerCard.id);
          setJokerPicker(null);
          const message = err instanceof Error ? err.message : 'Joker transform failed';
          setCardMsg(message);
          setTimeout(() => setCardMsg(''), 2500);
        });
        return;
      }
      const style = RARITY_STYLE[chosenTemplate.rarity];
      const newCard: GameCard = {
        ...chosenTemplate,
        id: `joker_transformed_${incrementCardSeq()}_${Date.now()}`,
        color: style.color,
        accent: style.accent,
      };
      // Replace Joker in hand with the chosen card
      if (playerColor === 'white') {
        setWhiteHand(h => h.map(c => c.id === jokerCard.id ? newCard : c));
      } else {
        setBlackHand(h => h.map(c => c.id === jokerCard.id ? newCard : c));
      }
      cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: false }; // allow using the new card
      setCardUsedBy(prev => ({ ...prev, [playerColor]: false }));
      pendingCardUseRef.current.delete(jokerCard.id);
      setJokerPicker(null);
      setCardMsg(`🃏 Joker transformed into ${chosenTemplate.name} ${chosenTemplate.icon}!`);
      setTimeout(() => setCardMsg(''), 3000);
    }, 800);
  }, [applyAuthoritativeSnapshot, authoritativeActorForColor, openJokerPicker]);

  // ── handleCardClick ────────────────────────────────────────────────────────
  const handleCardClick = React.useCallback((row: number, col: number) => {
    if (!cardPending) return;
    const { card, playerColor, mechanic, step, data } = cardPending;
    const b = board;
    const piece = b[row][col];
    const opp   = OPP[playerColor];

    if (authoritativeMatchIdRef.current && (mechanic === 'freeze' || mechanic === 'shield' || mechanic === 'sniper' || mechanic === 'badsniper' || mechanic === 'promote' || mechanic === 'demote' || mechanic === 'promotehim' || mechanic === 'demotehim' || mechanic === 'teleport' || mechanic === 'jump' || mechanic === 'swapme' || mechanic === 'swapus' || mechanic === 'swaphim' || mechanic === 'borrow' || mechanic === 'mindcontrol' || mechanic === 'parasite' || mechanic === 'clone' || mechanic === 'fakepiece' || mechanic === 'smallsacrifice' || mechanic === 'bigsacrifice' || mechanic === 'lavaground' || mechanic === 'blackhole' || mechanic === 'fortress' || mechanic === 'fog_village' || mechanic === 'invisible' || mechanic === 'unabomber' || mechanic === 'halffuse' || mechanic === 'fullfusion')) {
      const targetIntent: Omit<Extract<PlayerIntent, { type: 'select_target' }>, 'matchId'> = {
        type: 'select_target',
        ...authoritativeActorForColor(playerColor),
        target: { row, col }
      };

      void applyIntent(authoritativeMatchIdRef.current, targetIntent).then(snapshot => {
        applyAuthoritativeSnapshot(snapshot);
        if (mechanic === 'freeze') {
          setCardMsg(`Freeze applied at ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'shield') {
          setCardMsg(`Shield applied at ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'sniper') {
          triggerSniperAnim({ row, col }, piece!.type, piece!.color, 'sniper');
          setCardMsg(`Sniper removed ${piece!.type} on ${FILES[col]}${RANKS[row]}`);
          fireCardAnim('sniper', `${piece!.type} eliminated`);
        } else if (mechanic === 'badsniper') {
          triggerSniperAnim({ row, col }, piece!.type, piece!.color, 'badsniper');
          setCardMsg(`Bad Sniper removed your ${piece!.type} on ${FILES[col]}${RANKS[row]}`);
          fireCardAnim('sniper', `${piece!.type} eliminated`);
        } else if (mechanic === 'promote') {
          setCardMsg(`Choose promotion for ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'promotehim') {
          setCardMsg(`Choose enemy promotion for ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'demotehim') {
          setCardMsg(`Choose demotion for ${FILES[col]}${RANKS[row]}`);
          } else if (mechanic === 'lavaground') {
            setCardMsg(`Lava trap placed on ${FILES[col]}${RANKS[row]}`);
          } else if (mechanic === 'fortress') {
            setCardMsg(`Fortress placed with top-left at ${FILES[Math.min(col, 6)]}${RANKS[Math.min(row, 6)]}`);
          } else if (mechanic === 'fog_village') {
            setCardMsg(`Fog Village placed around ${FILES[col]}${RANKS[row]}`);
          } else if (mechanic === 'invisible') {
            setCardMsg(`Invisible applied to ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'unabomber') {
          setCardMsg(`Bomb attached on ${FILES[col]}${RANKS[row]}`);
        } else if (mechanic === 'halffuse' || mechanic === 'fullfusion') {
          const sq1 = step === 2 ? (data.sq1 as Sq | undefined) : undefined;
          const type1 = data.type1 as PieceType | undefined;
          if (sq1 && type1 && piece) {
            triggerFuseAnim({ sq1, sq2: { row, col }, type1, type2: piece.type, color: playerColor });
            setCardMsg(`${mechanic === 'halffuse' ? 'Half Fuse' : 'Full Fusion'} applied to ${FILES[col]}${RANKS[row]}`);
          } else {
            setCardMsg('Now click an adjacent own piece to fuse');
          }
        } else if (mechanic === 'teleport') {
          const from = step === 2 ? (data.from as Sq | undefined) : undefined;
          if (from && piece) {
            triggerTeleportAnim(from, { row, col }, board[from.row][from.col]?.type ?? 'pawn', board[from.row][from.col]?.color ?? playerColor);
            setCardMsg(`Teleported to ${FILES[col]}${RANKS[row]}`);
          } else {
            setCardMsg('Now click any empty square on the board');
          }
        } else if (mechanic === 'jump') {
          const from = step === 2 ? (data.from as Sq | undefined) : undefined;
          const jumper = from ? board[from.row][from.col] : null;
          if (from && jumper) {
            triggerJumpAnim(from, { row, col }, jumper.type, jumper.color, !!piece);
            setCardMsg(piece ? `Jump captured on ${FILES[col]}${RANKS[row]}` : `Jumped to ${FILES[col]}${RANKS[row]}`);
          } else {
            setCardMsg('Now click a square to jump to');
          }
        } else if (mechanic === 'swapme') {
          const sq1 = step === 2 ? (data.sq1 as Sq | undefined) : undefined;
          const firstPiece = sq1 ? board[sq1.row][sq1.col] : null;
          if (sq1 && firstPiece && piece) {
            triggerSwapAnim(sq1, { row, col }, '#4ade80', '#4ade80');
            setCardMsg(`Swapped ${firstPiece.type} and ${piece.type}!`);
          } else {
            setCardMsg('Now click the second of your pieces to swap with');
          }
        } else if (mechanic === 'swapus') {
          const sq1 = step === 2 ? (data.sq1 as Sq | undefined) : undefined;
          const firstPiece = sq1 ? board[sq1.row][sq1.col] : null;
          if (sq1 && firstPiece && piece) {
            triggerSwapAnim(sq1, { row, col }, '#4ade80', '#f87171');
            setCardMsg(`Swapped ${firstPiece.type} with enemy ${piece.type}!`);
          } else {
            setCardMsg('Now click an enemy piece to swap with');
          }
        } else if (mechanic === 'swaphim') {
          const sq1 = step === 2 ? (data.sq1 as Sq | undefined) : undefined;
          const firstPiece = sq1 ? board[sq1.row][sq1.col] : null;
          if (sq1 && firstPiece && piece) {
            triggerSwapAnim(sq1, { row, col }, '#f87171', '#f87171');
            setCardMsg(`Swapped enemy ${firstPiece.type} and ${piece.type}!`);
          } else {
            setCardMsg('Now click the second enemy piece to swap with');
          }
        } else if (mechanic === 'borrow') {
          if (piece) {
            setCardMsg(`Borrowed enemy ${piece.type} for this turn!`);
          } else {
            setCardMsg('Click an enemy piece to control for 1 turn');
          }
        } else if (mechanic === 'mindcontrol') {
          if (piece) {
            triggerMindControlAnim({ row, col }, playerColor, piece.type);
            fireCardAnim('mindcontrol', `${piece.type} permanently stolen`);
            setCardMsg(`Stole enemy ${piece.type}! It's yours now.`);
          } else {
            setCardMsg('Click an enemy piece to steal permanently');
          }
        } else if (mechanic === 'parasite') {
          const hostSq = step === 2 ? (data.hostSq as Sq | undefined) : undefined;
          const host = hostSq ? board[hostSq.row][hostSq.col] : null;
          if (hostSq && host && piece) {
            setCardMsg(`Parasite linked! If your ${host.type} dies, their ${piece.type} dies too!`);
          } else {
            setCardMsg('Now click an enemy piece with the same value');
          }
        } else if (mechanic === 'clone') {
          const from = step === 2 ? (data.from as Sq | undefined) : undefined;
          const source = from ? board[from.row][from.col] : null;
          if (from && source) {
            setCardMsg(`Cloned ${source.type} to ${FILES[col]}${RANKS[row]}!`);
            fireCardAnim('clone', `${source.type} duplicated`);
          } else {
            setCardMsg('Now click an adjacent empty square to place the clone');
          }
        } else if (mechanic === 'fakepiece') {
          setCardMsg(`Fake pawn placed on ${FILES[col]}${RANKS[row]}!`);
        } else if (mechanic === 'blackhole') {
          if (step === 2) {
            const sq1 = data.sq1 as Sq | undefined;
            if (sq1) {
              setCardMsg(`Black hole set between ${FILES[sq1.col]}${RANKS[sq1.row]} and ${FILES[col]}${RANKS[row]}!`);
              fireCardAnim('blackhole', 'Gravity trap armed — 2 turns');
            } else {
              setCardMsg('Now click the second square for the black hole');
            }
          } else {
            setCardMsg('Now click the second square for the black hole');
          }
        } else if (mechanic === 'smallsacrifice' || mechanic === 'bigsacrifice') {
          const selected = (data.selected as Sq[] | undefined) ?? [];
          const updated = piece
            ? (selected.some(sq => sq.row === row && sq.col === col)
                ? selected.filter(sq => !(sq.row === row && sq.col === col))
                : [...selected, { row, col }])
            : selected;
          const total = updated.reduce((sum, sq) => sum + PIECE_VALUE[board[sq.row][sq.col]?.type ?? 'pawn'], 0);
          const goal = mechanic === 'smallsacrifice' ? 6 : 14;
          if (!piece) {
            if (total < goal) {
              setCardMsg(`Need at least ${goal} pts, only have ${total}. Select more pieces.`);
              setTimeout(() => setCardMsg(''), 2500);
              return;
            }
            triggerSacrificeAnim(selected);
            const rewardCount = mechanic === 'smallsacrifice' ? 2 : 3;
            setCardMsg(`Sacrificed ${selected.length} piece(s) (${total} pts)! Drew ${rewardCount} cards.`);
            fireCardAnim(mechanic, `Sacrificed ${total} pts`);
          } else {
            setCardMsg(`Selected ${updated.length} piece(s) = ${total} pts (need ${goal}+). Click empty square to confirm.`);
          }
        } else {
          setCardMsg(`Choose demotion for ${FILES[col]}${RANKS[row]}`);
        }
        setTimeout(() => setCardMsg(''), 2000);
        finishCardUse(card, playerColor);
      }).catch(err => {
        const message = err instanceof Error ? err.message : 'Card target failed';
        setCardMsg(message);
        setTimeout(() => setCardMsg(''), 2000);
      });
      return;
    }

    switch (mechanic) {

      case 'freeze': {
        if (!piece || piece.color !== opp || piece.type === 'king') {
          setCardMsg('❄️ Click an ENEMY piece (not king) to freeze it!'); return;
        }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = { ...piece, frozen: true };
        setBoard(nb);
        setCardMsg(`❄️ ${FILES[col]}${RANKS[row]} is frozen for 1 turn!`);
        setTimeout(() => setCardMsg(''), 2000);
        fireCardAnim('freeze', `${FILES[col]}${RANKS[row]} cannot move for 1 turn`);
        finishCardUse(card, playerColor);
        return;
      }

      case 'shield': {
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('🛡️ Click YOUR piece (not king) to shield it!'); return;
        }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        // Shield lasts 1 full round: expires at start of player's NEXT turn (fmn + 1)
        nb[row][col] = { ...piece, shielded: true, shieldTurn: fmnRef.current + 1 };
        setBoard(nb);
        setCardMsg(`🛡️ ${FILES[col]}${RANKS[row]} is shielded for 1 full round!`);
        setTimeout(() => setCardMsg(''), 2000);
        fireCardAnim('shield', `${piece.type} on ${FILES[col]}${RANKS[row]} protected!`);
        finishCardUse(card, playerColor);
        return;
      }

      case 'sniper': {
        if (!piece || piece.type === 'king' || piece.color !== opp) {
          setCardMsg('🎯 Click an ENEMY piece (not king) to remove it!'); return;
        }
        const nb = cloneBoard(b);
        nb[row][col] = null;
        const kp  = findKing(nb, playerColor);
        const okp = findKing(nb, opp);
        if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) {
          setCardMsg('🎯 Cannot remove that piece — it would leave your king in check!'); return;
        }
        if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) {
          setCardMsg('🎯 Cannot remove that piece — it would put the enemy king in check!'); return;
        }
        // Delay board update so piece stays visible during sniper animation
        triggerSniperAnim({ row, col }, piece.type, piece.color, 'sniper');
        setTimeout(() => setBoard(nb), 450);
        setCardMsg(`🎯 ${piece.color} ${piece.type} on ${FILES[col]}${RANKS[row]} eliminated!`);
        setTimeout(() => setCardMsg(''), 2500);
        fireCardAnim('sniper', `${piece.type} eliminated`);
        finishCardUse(card, playerColor);
        return;
      }

      case 'badsniper': {
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('🔫 Click YOUR piece (not king) to remove it...'); return;
        }
        const nb = cloneBoard(b);
        nb[row][col] = null;
        const kp  = findKing(nb, playerColor);
        const okp = findKing(nb, opp);
        if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) {
          setCardMsg('🔫 Cannot — that piece is protecting your king!'); return;
        }
        if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) {
          setCardMsg('🔫 Cannot — removing that piece would put the enemy king in check!'); return;
        }
        // Delay board update so piece stays visible during animation
        triggerSniperAnim({ row, col }, piece.type, piece.color, 'badsniper');
        setTimeout(() => setBoard(nb), 450);
        setCardMsg(`🔫 You removed your own ${piece.type}... why?`);
        setTimeout(() => setCardMsg(''), 2500);
        finishCardUse(card, playerColor);
        return;
      }

      case 'promote': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('⬆️ Click YOUR piece to promote (not king)'); return;
          }
          const upgrades = UPGRADE[piece.type];
          if (!upgrades?.length) { setCardMsg('⬆️ That piece cannot be promoted further!'); return; }
          const safe = getSafeTransforms(b, row, col, upgrades, playerColor);
          if (!safe.length) { setCardMsg('⬆️ No safe promotion — all options would cause check!'); return; }
          setPromoPicker({ sq: { row, col }, options: safe, mechanic: 'promote' });
          setCardMsg('⬆️ Choose what to promote it to:');
          setCardPending({ ...cardPending, step: 2, data: { sq: { row, col } } });
        }
        return;
      }

      case 'demote': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('⬇️ Click YOUR piece to demote (not king)'); return;
          }
          const downgrades = DOWNGRADE[piece.type];
          if (!downgrades?.length) { setCardMsg('⬇️ That piece cannot be demoted further (already a pawn)!'); return; }
          const safe = getSafeTransforms(b, row, col, downgrades, playerColor);
          if (!safe.length) { setCardMsg('⬇️ No safe demotion — all options would cause check!'); return; }
          setPromoPicker({ sq: { row, col }, options: safe, mechanic: 'demote' });
          setCardMsg('⬇️ Choose what to demote it to:');
          setCardPending({ ...cardPending, step: 2, data: { sq: { row, col } } });
        }
        return;
      }

      case 'jump': {
        const jumpValid = (fr: number, fc: number, tr: number, tc: number, board2: Board): boolean => {
          const dr = tr - fr, dc = tc - fc;
          if (dr === 0 && dc === 0) return false;
          const sr = Math.sign(dr), sc = Math.sign(dc);
          let count = 0;
          let r = fr + sr, c = fc + sc;
          while (r !== tr || c !== tc) {
            if (board2[r][c]) count++;
            r += sr; c += sc;
          }
          return count === 1;
        };
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king' || piece.type === 'knight') {
            setCardMsg('🦘 Click YOUR piece to jump (not king or knight)'); return;
          }
          setCardMsg('🦘 Now click a square to jump to — must have exactly 1 piece in between');
          setCardPending({ ...cardPending, step: 2, data: { from: { row, col }, pieceType: piece.type, pieceColor: piece.color } });
          return;
        }
        if (step === 2) {
          const from = data.from as Sq;
          const pt = data.pieceType as PieceType;
          const pc = data.pieceColor as PieceColor;
          if (piece && piece.color === playerColor) { setCardMsg('🦘 Cannot land on your own piece!'); return; }
          if (piece && piece.type === 'king') { setCardMsg('🦘 Cannot jump onto the king!'); return; }
          const dr = row - from.row, dc = col - from.col;
          const diag = Math.abs(dr) === Math.abs(dc), straight = dr === 0 || dc === 0;
          let dirOk = false;
          if (pt === 'bishop') dirOk = diag;
          else if (pt === 'rook') dirOk = straight;
          else if (pt === 'queen') dirOk = diag || straight;
          else if (pt === 'pawn') {
            const fwd = pc === 'white' ? 1 : -1;
            // straight forward 1 or 2 squares, OR diagonal forward 2 squares (need room for 1 piece in between)
            dirOk = (dc === 0 && (dr === fwd || dr === fwd * 2)) || (Math.abs(dc) === 2 && dr === fwd * 2);
          }
          if (!dirOk) { setCardMsg(`🦘 That direction is invalid for a ${pt}!`); return; }
          if (!jumpValid(from.row, from.col, row, col, b)) { setCardMsg('🦘 Must have exactly 1 piece in between!'); return; }
          // Pawn straight jump must land on empty square
          if (pt === 'pawn' && dc === 0 && piece) { setCardMsg('🦘 Pawn can only jump straight to an empty square!'); return; }
          const nb = cloneBoard(b);
          const mp = nb[from.row][from.col]!;
          nb[row][col] = mp; nb[from.row][from.col] = null;
          // End the turn properly
          const t = playerColor;
          const next: PieceColor = OPP[t];
          const newMv  = new Set(movedRef.current).add(`${from.row}-${from.col}`);
          const newLm  = { from: { row: from.row, col: from.col }, to: { row, col } };
          const newHmc = (pt === 'pawn' || !!piece) ? 0 : hmcRef.current + 1;
          const newFmn = t === 'black' ? fmnRef.current + 1 : fmnRef.current;
          const posKey = positionKey(nb, next, newMv, newLm);
          const newPh  = [...posHistRef.current, posKey];
          const fen    = toFEN(nb, next, newMv, newLm, newHmc, newFmn);
          const snap: Snapshot = { board: nb.map(r => [...r]), turn: next, moved: newMv, lm: newLm, hmc: newHmc, fmn: newFmn, fen };
          setSnapshots(prev => [...prev, snap]);
          setBoard(nb);
          setMoved(newMv);
          setLm(newLm);
          setHmc(newHmc);
          setFmn(newFmn);
          setPosHist(newPh);
          setMovHist(prev => {
            const nx = [...prev];
            const note = `🦘${FILES[from.col]}${RANKS[from.row]}→${FILES[col]}${RANKS[row]}`;
            if (t === 'white') nx.push({ n: `${nx.length + 1}.`, w: note });
            else { const last = nx[nx.length - 1]; if (last && !last.b) nx[nx.length - 1] = { ...last, b: note }; }
            return nx;
          });
          resetCardUsed(next);
          setTurn(next);
          setTicking(next);
          setSel(null); setHints([]);
          checkEndGame(nb, next, newMv, newLm, newHmc, newPh, posKey, fen, t);
          const action = piece ? `captured ${piece.type} on` : 'jumped to';
          setCardMsg(`🦘 ${mp.type} ${action} ${FILES[col]}${RANKS[row]}!`);
          setTimeout(() => setCardMsg(''), 2000);
          triggerJumpAnim(from, { row, col }, pt, pc, !!piece);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'teleport': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🌀 Click YOUR piece to teleport (not king)'); return;
          }
          setCardMsg('🌀 Now click any empty square on the board');
          setCardPending({ ...cardPending, step: 2, data: { from: { row, col } } });
          return;
        }
        if (step === 2) {
          const from = data.from as Sq;
          if (piece) { setCardMsg('🌀 Target square must be EMPTY!'); return; }
          const nb = cloneBoard(b);
          const mp = nb[from.row][from.col]!;
          nb[row][col] = mp; nb[from.row][from.col] = null;
          const kp  = findKing(nb, playerColor);
          const okp = findKing(nb, opp);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('🌀 That teleport would leave your king in check!'); return; }
          if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('🌀 Cannot teleport there — would put the enemy king in check!'); return; }
          setBoard(nb);
          setCardMsg(`🌀 Teleported ${mp.type} to ${FILES[col]}${RANKS[row]}!`);
          setTimeout(() => setCardMsg(''), 2000);
          triggerTeleportAnim(from, { row, col }, mp.type, mp.color);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'doublemove_diff':
        activateDoubleMove('diff', card, playerColor);
        return;

      case 'doublemove_same':
        activateDoubleMove('same', card, playerColor);
        return;

      case 'swapme': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🔄 Click the FIRST of your pieces to swap (not king)'); return;
          }
          setCardMsg('🔄 Now click the SECOND of your pieces to swap with');
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col } } });
          return;
        }
        if (step === 2) {
          const sq1 = data.sq1 as Sq;
          if (!piece || piece.color !== playerColor || piece.type === 'king') { setCardMsg('🔄 Must pick YOUR piece (not king)!'); return; }
          if (row === sq1.row && col === sq1.col) { setCardMsg('🔄 Pick a different piece!'); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          const p1 = nb[sq1.row][sq1.col]!;
          const p2 = nb[row][col]!;
          nb[sq1.row][sq1.col] = p2; nb[row][col] = p1;
          triggerSwapAnim(sq1, { row, col }, '#4ade80', '#4ade80');
          fireCardAnim('swap', 'Positions exchanged');
          setBoard(nb);
          const next = opp;
          resetCardUsed(next); setTurn(next); setTicking(next); setClockActive(true); setSel(null); setHints([]);
          const posKey = positionKey(nb, next, moved, lm);
          const newPh = [...posHist, posKey];
          setPosHist(newPh);
          checkEndGame(nb, next, moved, lm, hmc, newPh, posKey, toFEN(nb, next, moved, lm, hmc, fmn), playerColor);
          const promoSq = (nb[row][col]?.type === 'pawn' && ((nb[row][col]!.color === 'white' && row === 7) || (nb[row][col]!.color === 'black' && row === 0))) ? { row, col }
            : (nb[sq1.row][sq1.col]?.type === 'pawn' && ((nb[sq1.row][sq1.col]!.color === 'white' && sq1.row === 7) || (nb[sq1.row][sq1.col]!.color === 'black' && sq1.row === 0))) ? sq1 : null;
          if (promoSq) setCardPromo({ sq: promoSq, color: nb[promoSq.row][promoSq.col]!.color });
          setCardMsg(`🔄 Swapped ${p1.type} and ${p2.type}!`);
          setTimeout(() => setCardMsg(''), 2000);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'swapus': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('↔️ Click YOUR piece to swap with enemy (not king)'); return;
          }
          setCardMsg('↔️ Now click an ENEMY piece to swap with (not king)');
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col } } });
          return;
        }
        if (step === 2) {
          const sq1 = data.sq1 as Sq;
          if (!piece || piece.color !== opp || piece.type === 'king') { setCardMsg('↔️ Must pick an ENEMY piece (not king)!'); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          const p1 = nb[sq1.row][sq1.col]!;
          const p2 = nb[row][col]!;
          nb[sq1.row][sq1.col] = p2; nb[row][col] = p1;
          const kp = findKing(nb, playerColor);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('↔️ That swap would leave your king in check!'); return; }
          triggerSwapAnim(sq1, { row, col }, '#4ade80', '#f87171');
          fireCardAnim('swap', 'Positions exchanged');
          setBoard(nb);
          const next = opp;
          resetCardUsed(next); setTurn(next); setTicking(next); setClockActive(true); setSel(null); setHints([]);
          const posKey = positionKey(nb, next, moved, lm);
          const newPh = [...posHist, posKey];
          setPosHist(newPh);
          checkEndGame(nb, next, moved, lm, hmc, newPh, posKey, toFEN(nb, next, moved, lm, hmc, fmn), playerColor);
          const promoSq = (nb[row][col]?.type === 'pawn' && ((nb[row][col]!.color === 'white' && row === 7) || (nb[row][col]!.color === 'black' && row === 0))) ? { row, col }
            : (nb[sq1.row][sq1.col]?.type === 'pawn' && ((nb[sq1.row][sq1.col]!.color === 'white' && sq1.row === 7) || (nb[sq1.row][sq1.col]!.color === 'black' && sq1.row === 0))) ? sq1 : null;
          if (promoSq) setCardPromo({ sq: promoSq, color: nb[promoSq.row][promoSq.col]!.color });
          setCardMsg(`↔️ Swapped ${p1.type} with enemy ${p2.type}!`);
          setTimeout(() => setCardMsg(''), 2000);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'swaphim': {
        if (step === 1) {
          if (!piece || piece.color !== opp || piece.type === 'king') {
            setCardMsg('🔁 Click FIRST enemy piece to swap (not king)'); return;
          }
          setCardMsg('🔁 Now click the SECOND enemy piece to swap with (not king)');
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col } } });
          return;
        }
        if (step === 2) {
          const sq1 = data.sq1 as Sq;
          if (!piece || piece.color !== opp || piece.type === 'king') { setCardMsg('🔁 Must pick an ENEMY piece (not king)!'); return; }
          if (row === sq1.row && col === sq1.col) { setCardMsg('🔁 Pick a different piece!'); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          const p1 = nb[sq1.row][sq1.col]!;
          const p2 = nb[row][col]!;
          nb[sq1.row][sq1.col] = p2; nb[row][col] = p1;
          const kp = findKing(nb, playerColor);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('🔁 That swap would leave your king in check!'); return; }
          triggerSwapAnim(sq1, { row, col }, '#f87171', '#f87171');
          fireCardAnim('swap', 'Opponent pieces swapped');
          setBoard(nb);
          const next = opp;
          resetCardUsed(next); setTurn(next); setTicking(next); setClockActive(true); setSel(null); setHints([]);
          const posKey = positionKey(nb, next, moved, lm);
          const newPh = [...posHist, posKey];
          setPosHist(newPh);
          checkEndGame(nb, next, moved, lm, hmc, newPh, posKey, toFEN(nb, next, moved, lm, hmc, fmn), playerColor);
          const promoSq = (nb[row][col]?.type === 'pawn' && ((nb[row][col]!.color === 'white' && row === 7) || (nb[row][col]!.color === 'black' && row === 0))) ? { row, col }
            : (nb[sq1.row][sq1.col]?.type === 'pawn' && ((nb[sq1.row][sq1.col]!.color === 'white' && sq1.row === 7) || (nb[sq1.row][sq1.col]!.color === 'black' && sq1.row === 0))) ? sq1 : null;
          if (promoSq) setCardPromo({ sq: promoSq, color: nb[promoSq.row][promoSq.col]!.color });
          setCardMsg(`🔁 Swapped enemy ${p1.type} and ${p2.type}!`);
          setTimeout(() => setCardMsg(''), 2000);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'clone': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🧬 Click YOUR piece to clone (not king)'); return;
          }
          setCardMsg('🧬 Now click an adjacent empty square to place the clone');
          setCardPending({ ...cardPending, step: 2, data: { from: { row, col } } });
          return;
        }
        if (step === 2) {
          const from = data.from as Sq;
          if (piece) { setCardMsg('🧬 Target square must be EMPTY!'); return; }
          if (Math.abs(row - from.row) > 1 || Math.abs(col - from.col) > 1) { setCardMsg('🧬 Must be an ADJACENT square!'); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          const src = nb[from.row][from.col]!;
          nb[row][col] = { ...src };
          const kp  = findKing(nb, playerColor);
          const okp = findKing(nb, opp);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp))         { setCardMsg('🧬 Clone would leave your king in check!'); return; }
          if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('🧬 Cannot clone there — would put enemy king in check!'); return; }
          setBoard(nb);
          setCardMsg(`🧬 Cloned ${src.type} to ${FILES[col]}${RANKS[row]}!`);
          setTimeout(() => setCardMsg(''), 2000);
          fireCardAnim('clone', `${src.type} duplicated`);
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'mindcontrol': {
        if (!piece || piece.color !== opp || piece.type === 'king') {
          setCardMsg('🧠 Click an ENEMY piece to steal (not king)'); return;
        }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = { ...piece, color: playerColor };
        const kp  = findKing(nb, playerColor);
        const okp = findKing(nb, opp);
        if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp))         { setCardMsg('🧠 Cannot steal — would leave your king in check!'); return; }
        if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('🧠 Cannot steal — would put enemy king in check!'); return; }
        setBoard(nb);
        setCardMsg(`🧠 Stole enemy ${piece.type}! It's yours now.`);
        setTimeout(() => setCardMsg(''), 2500);
        fireCardAnim('mindcontrol', `${piece.type} permanently stolen`);
        triggerMindControlAnim({ row, col }, playerColor, piece.type);
        finishCardUse(card, playerColor);
        return;
      }

      case 'borrow': {
        if (!piece || piece.color !== opp || piece.type === 'king') {
          setCardMsg('🤏 Click an ENEMY piece to borrow for 1 turn (not king)'); return;
        }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = { ...piece, color: playerColor, borrowed: true };
        const kp  = findKing(nb, playerColor);
        const okp = findKing(nb, opp);
        if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp))         { setCardMsg('🤏 Cannot borrow — would leave your king in check!'); return; }
        if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('🤏 Cannot borrow — would put enemy king in check!'); return; }
        setBoard(nb);
        setCardMsg(`🤏 Borrowed enemy ${piece.type} for this turn!`);
        setTimeout(() => setCardMsg(''), 2500);
        finishCardUse(card, playerColor);
        return;
      }

      case 'demotehim': {
        if (step === 1) {
          if (!piece || piece.type === 'king') { setCardMsg('📉 Click ANY piece to demote (not king)'); return; }
          const downgrades = DOWNGRADE[piece.type];
          if (!downgrades?.length) { setCardMsg('📉 That piece is already a pawn — cannot demote further!'); return; }
          const targetColor  = piece.color;
          const targetOpp    = OPP[targetColor];
          const safe = downgrades.filter(t => {
            const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
            nb[row][col] = { ...piece, type: t };
            const tkp  = findKing(nb, targetColor);
            const mykp = findKing(nb, playerColor);
            return (
              !(tkp  && isAttackedWithFusion(nb, tkp.row,  tkp.col,  targetOpp))  &&
              !(mykp && isAttackedWithFusion(nb, mykp.row, mykp.col, opp))
            );
          });
          if (!safe.length) { setCardMsg('📉 No safe demotion available!'); return; }
          setPromoPicker({ sq: { row, col }, options: safe, mechanic: 'demote' });
          setCardMsg('📉 Choose what to demote it to:');
          setCardPending({ ...cardPending, step: 2, data: { sq: { row, col } } });
        }
        return;
      }

      case 'promotehim': {
        if (step === 1) {
          if (!piece || piece.color !== opp || piece.type === 'king') {
            setCardMsg('📈 Click an ENEMY piece to promote (not king)'); return;
          }
          const upgrades = UPGRADE[piece.type];
          if (!upgrades?.length) { setCardMsg('📈 That piece cannot be promoted further!'); return; }
          const safe = getSafeTransforms(b, row, col, upgrades, playerColor);
          if (!safe.length) { setCardMsg('📈 No safe promotion available!'); return; }
          setPromoPicker({ sq: { row, col }, options: safe, mechanic: 'promote' });
          setCardMsg('📈 Choose what to promote enemy piece to:');
          setCardPending({ ...cardPending, step: 2, data: { sq: { row, col } } });
        }
        return;
      }

      case 'smallsacrifice': {
        const selected = (data.selected as Sq[] | undefined) ?? [];
        const totalVal = selected.reduce((sum, sq) => sum + PIECE_VALUE[b[sq.row][sq.col]?.type ?? 'pawn'], 0);
        if (!piece) {
          if (totalVal < 6) { setCardMsg(`🩸 Total value: ${totalVal}/6. Keep clicking YOUR pieces to add more!`); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          for (const sq of selected) nb[sq.row][sq.col] = null;
          const kp = findKing(nb, playerColor);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('🩸 Cannot sacrifice — would leave your king in check!'); return; }
          setBoard(nb);
          triggerSacrificeAnim(selected);
          const [c1, c2] = [drawRandomCard(playerColor[0]), drawRandomCard(playerColor[0])];
          const addCards = (h: GameCard[]) => {
            let nh = h.length < MAX_HAND_SIZE ? [...h, c1] : h;
            return nh.length < MAX_HAND_SIZE ? [...nh, c2] : nh;
          };
          if (playerColor === 'white') setWhiteHand(addCards);
          else                         setBlackHand(addCards);
          setCardMsg(`🩸 Sacrificed ${selected.length} piece(s) (${totalVal} pts)! Drew: ${c1.name} ${c1.icon} + ${c2.name} ${c2.icon}`);
          setTimeout(() => setCardMsg(''), 3500);
          fireCardAnim('smallsacrifice', `Sacrificed ${totalVal} pts — drew 2 cards`);
          finishCardUse(card, playerColor);
          return;
        }
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('🩸 Click YOUR pieces to sacrifice (not king). Click empty square when done.'); return;
        }
        const alreadyIdx = selected.findIndex(s => s.row === row && s.col === col);
        const newSelected = alreadyIdx >= 0
          ? selected.filter((_, i) => i !== alreadyIdx)
          : [...selected, { row, col }];
        const newTotal = newSelected.reduce((sum, sq) => sum + PIECE_VALUE[b[sq.row][sq.col]?.type ?? 'pawn'], 0);
        setCardPending({ ...cardPending, data: { selected: newSelected } });
        setCardMsg(`🩸 Selected ${newSelected.length} piece(s) = ${newTotal} pts (need 6+). Click empty square to confirm.`);
        return;
      }

      case 'bigsacrifice': {
        const selected = (data.selected as Sq[] | undefined) ?? [];
        const totalVal = selected.reduce((sum, sq) => sum + PIECE_VALUE[b[sq.row][sq.col]?.type ?? 'pawn'], 0);
        if (!piece) {
          if (totalVal < 14) { setCardMsg(`💎 Total value: ${totalVal}/14. Keep clicking YOUR pieces to add more!`); return; }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          for (const sq of selected) nb[sq.row][sq.col] = null;
          const kp = findKing(nb, playerColor);
          if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('💎 Cannot sacrifice — would leave your king in check!'); return; }
          setBoard(nb);
          triggerSacrificeAnim(selected);
          const [c1, c2, c3] = [drawRandomCard(playerColor[0]), drawRandomCard(playerColor[0]), drawRandomCard(playerColor[0])];
          const addCards = (h: GameCard[]) => {
            let nh = h.length < MAX_HAND_SIZE ? [...h, c1] : h;
            nh = nh.length < MAX_HAND_SIZE ? [...nh, c2] : nh;
            return nh.length < MAX_HAND_SIZE ? [...nh, c3] : nh;
          };
          if (playerColor === 'white') setWhiteHand(addCards);
          else                         setBlackHand(addCards);
          setCardMsg(`💎 Sacrificed ${selected.length} piece(s) (${totalVal} pts)! Drew: ${c1.name} ${c1.icon} + ${c2.name} ${c2.icon} + ${c3.name} ${c3.icon}`);
          setTimeout(() => setCardMsg(''), 4000);
          fireCardAnim('bigsacrifice', `Sacrificed ${totalVal} pts — drew 3 cards`);
          finishCardUse(card, playerColor);
          return;
        }
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('💎 Click YOUR pieces to sacrifice (not king). Click empty square when done.'); return;
        }
        const alreadyIdx = selected.findIndex(s => s.row === row && s.col === col);
        const newSelected = alreadyIdx >= 0
          ? selected.filter((_, i) => i !== alreadyIdx)
          : [...selected, { row, col }];
        const newTotal = newSelected.reduce((sum, sq) => sum + PIECE_VALUE[b[sq.row][sq.col]?.type ?? 'pawn'], 0);
        setCardPending({ ...cardPending, data: { selected: newSelected } });
        setCardMsg(`💎 Selected ${newSelected.length} piece(s) = ${newTotal} pts (need 14+). Click empty square to confirm.`);
        return;
      }

      case 'lavaground': {
        if (piece) { setCardMsg('🌋 Lava must be placed on an EMPTY square!'); return; }
        setLavaSquares(prev => [...prev, { row, col, movesLeft: 2 }]);
        setCardMsg(`🌋 Lava trap placed on ${FILES[col]}${RANKS[row]}! Any piece stepping there will be destroyed.`);
        setTimeout(() => setCardMsg(''), 3000);
        finishCardUse(card, playerColor);
        return;
      }

      case 'fog_village': {
        // Click any square — that becomes the CENTER of the 3×3 fog zone
        // Clamp center so full 3×3 stays in bounds (rows/cols 1–6)
        const clampedRow = Math.max(1, Math.min(6, row));
        const clampedCol = Math.max(1, Math.min(6, col));
        // Remove previous fog zone owned by this player (only 1 active at a time)
        setFogZones(prev => [
          ...prev.filter(z => z.ownerColor !== playerColor),
          { centerRow: clampedRow, centerCol: clampedCol, ownerColor: playerColor, turnsLeft: 2 },
        ]);
        setCardMsg(`🌫️ Fog placed at ${FILES[clampedCol]}${RANKS[clampedRow]}! Your pieces in that 3×3 are hidden from the enemy.`);
        setTimeout(() => setCardMsg(''), 3500);
        finishCardUse(card, playerColor);
        return;
      }

      case 'fakepiece': {
        if (piece) { setCardMsg('👻 Must place fake piece on an EMPTY square!'); return; }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = { type: 'pawn', color: playerColor };
        const kp = findKing(nb, playerColor);
        if (kp && isAttackedWithFusion(nb, kp.row, kp.col, opp)) { setCardMsg('👻 Placing fake piece there would expose your king!'); return; }
        setBoard(nb);
        setCardMsg(`👻 Fake pawn placed on ${FILES[col]}${RANKS[row]}! Opponent can't tell if it's real.`);
        setTimeout(() => setCardMsg(''), 3000);
        finishCardUse(card, playerColor);
        return;
      }

      case 'parasite': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🦠 Click YOUR piece to be the host (not king)'); return;
          }
          setCardMsg(`🦠 Now click an ENEMY piece with the SAME value as your ${piece.type} (${PIECE_VALUE[piece.type]} pts)`);
          setCardPending({ ...cardPending, step: 2, data: { hostSq: { row, col }, hostValue: PIECE_VALUE[piece.type] } });
          return;
        }
        if (step === 2) {
          if (!piece || piece.color !== opp || piece.type === 'king') {
            setCardMsg('🦠 Must pick an ENEMY piece (not king)!'); return;
          }
          const hostValue = data.hostValue as number;
          if (PIECE_VALUE[piece.type] !== hostValue) {
            setCardMsg(`🦠 Must pick an enemy piece with the SAME value (${hostValue} pts)! ${piece.type} = ${PIECE_VALUE[piece.type]} pts.`); return;
          }
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          const hostSq = data.hostSq as Sq;
          nb[hostSq.row][hostSq.col] = { ...nb[hostSq.row][hostSq.col]!, parasiteTarget: `${row},${col}` };
          setBoard(nb);
          setCardMsg(`🦠 Parasite linked! If your ${nb[hostSq.row][hostSq.col]?.type} dies, their ${piece.type} dies too!`);
          setTimeout(() => setCardMsg(''), 3000);
          finishCardUse(card, playerColor);
        }
        return;
      }

      // ── UNABOMBER: Attach bomb to piece ───────────────────────────────────
      case 'unabomber': {
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('💣 Click YOUR piece to attach a bomb (not king)'); return;
        }
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = { ...piece, bomb: true };
        setBoard(nb);
        // turnsLeft:2 = explodes after 2 full rounds (countdown only ticks after black moves)
        setBombPieces(prev => [...prev, { row, col, turnsLeft: 2, ownerColor: playerColor }]);
        setCardMsg(`💣 Bomb attached to ${piece.type} on ${FILES[col]}${RANKS[row]}! It explodes in 2 turns destroying everything adjacent.`);
        setTimeout(() => setCardMsg(''), 3500);
        finishCardUse(card, playerColor);
        return;
      }

      case 'blackhole': {
        if (step === 1) {
          setCardMsg('🕳️ Now click the SECOND square for the black hole');
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col } } });
          return;
        }
        if (step === 2) {
          const sq1 = data.sq1 as Sq;
          setCardMsg(`🕳️ Black hole set! Pieces adjacent to ${FILES[sq1.col]}${RANKS[sq1.row]} and ${FILES[col]}${RANKS[row]} will explode in 2 turns!`);
          sessionStorage.setItem('blackhole', JSON.stringify({ sq1, sq2: { row, col }, turnsLeft: 2 }));
          setTimeout(() => setCardMsg(''), 4000);
          fireCardAnim('blackhole', 'Gravity trap armed — 2 turns');
          finishCardUse(card, playerColor);
        }
        return;
      }

      case 'invisible': {
        if (!piece || piece.color !== playerColor || piece.type === 'king') {
          setCardMsg('👁️ Click YOUR piece to make invisible (not king)!'); return;
        }
        // Remove the piece from the board entirely — it lives in ghostPiece state
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[row][col] = null;
        setBoard(nb);
        const newGhost = { row, col, piece: { ...piece }, ownerColor: playerColor, roundsLeft: 1 };
        setGhostPiece(newGhost);
        ghostRef.current = newGhost;
        setCardMsg(`👁️ ${piece.type} is invisible for 1 round! Move it anywhere.`);
        setTimeout(() => setCardMsg(''), 3000);
        finishCardUse(card, playerColor);
        return;
      }

      // ── HALF FUSE: adjacent pieces only, combined value ≤ 6pts ──────────────
      case 'halffuse': {
        const HALF_FUSE_CAP = 6;
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('⚗️ Click YOUR first piece to fuse (not king)'); return;
          }
          if (piece.fusedWith) {
            setCardMsg('⚗️ That piece is already fused!'); return;
          }
          const val = PIECE_VALUE[piece.type];
          if (val > HALF_FUSE_CAP - 1) {
            setCardMsg(`⚗️ Half Fuse cap is ${HALF_FUSE_CAP}pts total — ${piece.type} alone is ${val}pts, leaving no room!`); return;
          }
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col }, type1: piece.type, val1: val } });
          setCardMsg(`⚗️ ${piece.type} (${val}pt) selected — click an ADJACENT own piece to absorb it (combined ≤ ${HALF_FUSE_CAP}pts, not king)`);
          return;
        }
        if (step === 2) {
          const sq1   = data.sq1 as Sq;
          const type1 = data.type1 as PieceType;
          const val1  = data.val1 as number;
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('⚗️ Must click YOUR piece (not king)'); return;
          }
          if (row === sq1.row && col === sq1.col) {
            setCardMsg('⚗️ Must click a DIFFERENT piece'); return;
          }
          if (Math.abs(row - sq1.row) > 1 || Math.abs(col - sq1.col) > 1) {
            setCardMsg(`⚗️ Half Fuse requires ADJACENT pieces! (${FILES[sq1.col]}${RANKS[sq1.row]} and ${FILES[col]}${RANKS[row]} are too far apart)`); return;
          }
          if (piece.fusedWith) {
            setCardMsg('⚗️ That piece is already fused!'); return;
          }
          const val2 = PIECE_VALUE[piece.type];
          const isBishopRookCombo = (type1 === 'bishop' && piece.type === 'rook') || (type1 === 'rook' && piece.type === 'bishop');
          if (!isBishopRookCombo && val1 + val2 > HALF_FUSE_CAP) {
            setCardMsg(`⚗️ Combined value ${val1 + val2}pts exceeds Half Fuse cap of ${HALF_FUSE_CAP}pts! Try smaller pieces.`); return;
          }
          const redundancy = checkFusionRedundancy(type1, piece.type);
          if (redundancy) { setCardMsg(redundancy); return; }
          const isBishopRook = (type1 === 'bishop' && piece.type === 'rook') || (type1 === 'rook' && piece.type === 'bishop');
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          nb[sq1.row][sq1.col] = null;
          nb[row][col] = isBishopRook ? { ...piece, type: 'queen', fusedWith: undefined } : { ...piece, fusedWith: type1 };
          const kp  = findKing(nb, playerColor);
          const okp = findKing(nb, opp);
          if (kp  && isAttackedWithFusion(nb, kp.row,  kp.col,  opp))        { setCardMsg('⚗️ Fusion would leave your king in check!'); return; }
          if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('⚗️ Fusion would put enemy king in check!'); return; }
          // Play animation first, apply board after animation finishes
          triggerFuseAnim({ sq1, sq2: { row, col }, type1, type2: piece.type, color: playerColor });
          if (isBishopRook) {
            setCardMsg('⚗️ Bishop + Rook = QUEEN! The pieces merged into royalty.');
          } else {
            setCardMsg(`⚗️ ${piece.type}+${type1} fused! (${val1 + val2}pts) Moves as both.`);
          }
          setTimeout(() => {
            setBoard(nb);
            setCardMsg('');
            // Replace cheater card in both hands with a random non-cheater card
            const safeCard = (side: string) => {
              let c = drawRandomCard(side); let att = 0;
              while (c.mechanic === 'cheater' && att < 20) { c = drawRandomCard(side); att++; }
              return c;
            };
            setWhiteHand(h => h.map(c => c.mechanic === 'cheater' ? safeCard('w') : c));
            setBlackHand(h => h.map(c => c.mechanic === 'cheater' ? safeCard('b') : c));
          }, 1800);
          finishCardUse(card, playerColor);
        }
        return;
      }

      // ── FULL FUSION: any 2 own pieces, no value cap, no adjacency ────────────
      case 'fullfusion': {
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🔮 Click YOUR first piece to fuse (not king)'); return;
          }
          if (piece.fusedWith) {
            setCardMsg('🔮 That piece is already fused!'); return;
          }
          setCardPending({ ...cardPending, step: 2, data: { sq1: { row, col }, type1: piece.type } });
          setCardMsg(`🔮 ${piece.type} (${PIECE_VALUE[piece.type]}pt) selected — click an ADJACENT own piece to fuse (not king, no point limit)`);
          return;
        }
        if (step === 2) {
          const sq1   = data.sq1 as Sq;
          const type1 = data.type1 as PieceType;
          if (!piece || piece.color !== playerColor || piece.type === 'king') {
            setCardMsg('🔮 Must click YOUR piece (not king)'); return;
          }
          if (row === sq1.row && col === sq1.col) {
            setCardMsg('🔮 Must click a DIFFERENT piece'); return;
          }
          if (Math.abs(row - sq1.row) > 1 || Math.abs(col - sq1.col) > 1) {
            setCardMsg(`🔮 Full Fusion requires ADJACENT pieces! (${FILES[sq1.col]}${RANKS[sq1.row]} and ${FILES[col]}${RANKS[row]} are too far apart)`); return;
          }
          if (piece.fusedWith) {
            setCardMsg('🔮 That piece is already fused!'); return;
          }
          const redundancy = checkFusionRedundancy(type1, piece.type);
          if (redundancy) { setCardMsg(redundancy.replace('⚗️', '🔮')); return; }
          const isBishopRook = (type1 === 'bishop' && piece.type === 'rook') || (type1 === 'rook' && piece.type === 'bishop');
          const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
          nb[sq1.row][sq1.col] = null;
          nb[row][col] = isBishopRook ? { ...piece, type: 'queen', fusedWith: undefined } : { ...piece, fusedWith: type1 };
          const kp  = findKing(nb, playerColor);
          const okp = findKing(nb, opp);
          if (kp  && isAttackedWithFusion(nb, kp.row,  kp.col,  opp))        { setCardMsg('🔮 Fusion would leave your king in check!'); return; }
          if (okp && isAttackedWithFusion(nb, okp.row, okp.col, playerColor)) { setCardMsg('🔮 Fusion would put enemy king in check!'); return; }
          // Play animation first, apply board after animation finishes
          triggerFuseAnim({ sq1, sq2: { row, col }, type1, type2: piece.type, color: playerColor });
          if (isBishopRook) {
            setCardMsg('🔮 Bishop + Rook = QUEEN! The ultimate fusion — pieces transformed into royalty.');
          } else {
            setCardMsg(`🔮 ${piece.type}+${type1} FULL FUSION! (${PIECE_VALUE[type1] + PIECE_VALUE[piece.type]}pts) Unstoppable.`);
          }
          setTimeout(() => {
            setBoard(nb);
            setCardMsg('');
            // Replace cheater card in both hands with a random non-cheater card
            const safeCard = (side: string) => {
              let c = drawRandomCard(side); let att = 0;
              while (c.mechanic === 'cheater' && att < 20) { c = drawRandomCard(side); att++; }
              return c;
            };
            setWhiteHand(h => h.map(c => c.mechanic === 'cheater' ? safeCard('w') : c));
            setBlackHand(h => h.map(c => c.mechanic === 'cheater' ? safeCard('b') : c));
          }, 1800);
          finishCardUse(card, playerColor);
        }
        return;
      }

      default:
        return;
    }
  }, [cardPending, board, finishCardUse, getSafeTransforms, activateDoubleMove, triggerSwapAnim, triggerTeleportAnim, triggerSacrificeAnim, triggerMindControlAnim, triggerFuseAnim, triggerSniperAnim, triggerJumpAnim, checkFusionRedundancy, isAttackedWithFusion, applyAuthoritativeSnapshot, authoritativeActorForColor]);

  const handlePromoPick = React.useCallback((type: PieceType) => {
    if (!cardPending || !promoPicker) return;
    const { card, playerColor, mechanic } = cardPending;
    const sq = promoPicker.sq;
    const oldType = board[sq.row][sq.col]?.type ?? 'pawn';
    // Use piece's actual color (not playerColor) — for promotehim/demotehim this is the enemy color
    const pieceColor = board[sq.row][sq.col]?.color ?? playerColor;
    if (authoritativeMatchIdRef.current && (mechanic === 'promote' || mechanic === 'demote' || mechanic === 'promotehim' || mechanic === 'demotehim')) {
      const targetIntent: Omit<Extract<PlayerIntent, { type: 'select_target' }>, 'matchId'> = {
        type: 'select_target',
        ...authoritativeActorForColor(playerColor),
        selectionId: type,
      };

      void applyIntent(authoritativeMatchIdRef.current, targetIntent).then(snapshot => {
        triggerTransformAnim(sq, (mechanic === 'promote' || mechanic === 'promotehim') ? 'up' : 'down', oldType, type, pieceColor);
        applyAuthoritativeSnapshot(snapshot);
        setCardMsg(`⬆️ ${FILES[sq.col]}${RANKS[sq.row]} ${(mechanic === 'promote' || mechanic === 'promotehim') ? 'promoted' : 'demoted'} to ${type}!`);
        setTimeout(() => setCardMsg(''), 2000);
        finishCardUse(card, playerColor);
      }).catch(err => {
        const message = err instanceof Error ? err.message : 'Transform selection failed';
        setCardMsg(message);
        setTimeout(() => setCardMsg(''), 2000);
        finishCardUse(card, playerColor);
      });
      return;
    }
    setPromoPicker(null);
    // promote + promotehim = gold/up; demote + demotehim = purple/down
    const isPromotion = mechanic === 'promote' || mechanic === 'promotehim';
    triggerTransformAnim(sq, isPromotion ? 'up' : 'down', oldType, type, pieceColor);
    // Delay board update until animation swap phase completes (~850ms into 1400ms anim)
    // so the old piece shows during gather/flash, new piece appears via animation
    setTimeout(() => {
      setBoard(b => {
        const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
        nb[sq.row][sq.col] = { ...nb[sq.row][sq.col]!, type };
        return nb;
      });
    }, 850);
    const verb = isPromotion ? 'promoted' : 'demoted';
    setCardMsg(`${isPromotion ? '⬆️' : '⬇️'} ${FILES[sq.col]}${RANKS[sq.row]} ${verb} to ${type}!`);
    setTimeout(() => setCardMsg(''), 2000);
    finishCardUse(card, playerColor);
  }, [cardPending, promoPicker, board, finishCardUse, triggerTransformAnim, applyAuthoritativeSnapshot, authoritativeActorForColor]);

  const canUseCard = React.useCallback((card: GameCard, playerColor: PieceColor): boolean => {
    if (over) return false;
    if (hostedRuntime && viewerSeatRef.current !== playerColor) return false;
    if (card.type !== 'trap' && turn !== playerColor) return false;
    return !cardUsedByRef.current[playerColor];
  }, [over, turn, hostedRuntime]);

  const applyCard = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    if (!canUseCard(card, playerColor)) return;
    if (pendingCardUseRef.current.has(card.id)) return;

    const opp = OPP[playerColor];

    // ── JOKER: open picker overlay (openJokerPicker adds to pendingCardUseRef itself)
    if (card.mechanic === 'joker') {
      if (authoritativeMatchIdRef.current) {
        pendingCardUseRef.current.add(card.id);
        const jokerIntent: Omit<Extract<PlayerIntent, { type: 'play_card' }>, 'matchId'> = {
          type: 'play_card',
          ...authoritativeActorForColor(playerColor),
          cardId: card.id,
        };
        void applyIntent(authoritativeMatchIdRef.current, jokerIntent).then(snapshot => {
          applyAuthoritativeSnapshot(snapshot);
          openJokerPicker(card, playerColor);
          setCardMsg('ðŸƒ Choose a backend-supported transformation for Joker.');
        }).catch(err => {
          pendingCardUseRef.current.delete(card.id);
          const message = err instanceof Error ? err.message : 'Joker activation failed';
          setCardMsg(message);
          setTimeout(() => setCardMsg(''), 2500);
        });
        return;
      }
      openJokerPicker(card, playerColor);
      return;
    }

    pendingCardUseRef.current.add(card.id);

    if (authoritativeMatchIdRef.current && (card.mechanic === 'freeze' || card.mechanic === 'shield' || card.mechanic === 'sniper' || card.mechanic === 'badsniper' || card.mechanic === 'promote' || card.mechanic === 'demote' || card.mechanic === 'promotehim' || card.mechanic === 'demotehim' || card.mechanic === 'teleport' || card.mechanic === 'jump' || card.mechanic === 'doublemove_diff' || card.mechanic === 'doublemove_same' || card.mechanic === 'swapme' || card.mechanic === 'swapus' || card.mechanic === 'swaphim' || card.mechanic === 'borrow' || card.mechanic === 'mindcontrol' || card.mechanic === 'parasite' || card.mechanic === 'clone' || card.mechanic === 'fakepiece' || card.mechanic === 'smallsacrifice' || card.mechanic === 'bigsacrifice' || card.mechanic === 'gambler' || card.mechanic === 'radar' || card.mechanic === 'cheater' || card.mechanic === 'lavaground' || card.mechanic === 'blackhole' || card.mechanic === 'fortress' || card.mechanic === 'fog_village' || card.mechanic === 'invisible' || card.mechanic === 'unabomber' || card.mechanic === 'halffuse' || card.mechanic === 'fullfusion' || card.mechanic === 'reverse' || card.mechanic === 'undo' || card.mechanic === 'mirror')) {
      const playCardIntent: Omit<Extract<PlayerIntent, { type: 'play_card' }>, 'matchId'> = {
        type: 'play_card',
        ...authoritativeActorForColor(playerColor),
        cardId: card.id
      };

        void applyIntent(authoritativeMatchIdRef.current, playCardIntent).then(snapshot => {
          applyAuthoritativeSnapshot(snapshot);
          if (card.mechanic === 'doublemove_diff') {
            setCardMsg('Twin active! Make your first move, then move a different piece.');
          } else if (card.mechanic === 'doublemove_same') {
            setCardMsg('Solo active! Make your first move, then move the same piece again.');
          } else if (card.mechanic === 'reverse') {
            setCardMsg("Reversed opponent's last move!");
            fireCardAnim('reverse', "Opponent's last move undone");
          } else if (card.mechanic === 'undo') {
            setCardMsg("Undo armed! Opponent's next card will be nullified.");
          } else if (card.mechanic === 'mirror') {
            setCardMsg('Mirror resolved.');
          } else if (card.mechanic === 'gambler') {
            const eventList = snapshot.events ?? [];
            const lastEvent = [...eventList].reverse().find(event => event.type === 'card_played') ?? eventList[eventList.length - 1];
            const outcome = lastEvent?.payload?.outcome;
            const affectedCard = lastEvent?.payload?.card as GameCard | undefined;
            if (outcome === 'win' && affectedCard) {
              setCardMsg(`🎲 WIN! Stole "${affectedCard.name}" from opponent!`);
              fireCardAnim('gambler_win', `Stole "${affectedCard.name}" ${affectedCard.icon}`);
            } else if (outcome === 'lose' && affectedCard) {
              setCardMsg(`🎲 LOSE! Gave "${affectedCard.name}" to opponent...`);
              fireCardAnim('gambler_lose', `Gave away "${affectedCard.name}" ${affectedCard.icon}`);
            } else {
              setCardMsg('🎲 Gambler had no effect.');
            }
          } else if (card.mechanic === 'radar') {
            setCardMsg('📡 Radar active! Enemy hand revealed for this turn.');
          } else if (card.mechanic === 'cheater') {
            analyse(
              toFEN(snapshot.match.board as Board, snapshot.match.turn, new Set(snapshot.match.moved), snapshot.match.lastMove, snapshot.match.halfMoveClock, snapshot.match.fullMoveNumber),
              snapshot.match.turn,
            );
            setCardMsg('💡 Cheater active for 3 turns! Engine panel shows best move.');
          } else if (card.mechanic === 'fortress') {
            setCardMsg('🏰 Fortress ready. Click the board to place the 2x2 zone.');
          } else {
            setCardMsg(CARD_TARGET_MESSAGES[card.mechanic] ?? 'Click a square...');
          }
        }).catch(err => {
        pendingCardUseRef.current.delete(card.id);
        const message = err instanceof Error ? err.message : 'Card play failed';
        setCardMsg(message);
        setTimeout(() => setCardMsg(''), 2000);
      });
      setSelectedCard(null);
      return;
    }

    // Cards requiring board clicks
    if (TARGETING_CARDS.has(card.mechanic)) {
      if (card.mechanic === 'doublemove_diff') { activateDoubleMove('diff', card, playerColor); return; }
      if (card.mechanic === 'doublemove_same') { activateDoubleMove('same', card, playerColor); return; }
      setCardPending({ card, playerColor, mechanic: card.mechanic, step: 1, data: {} });
      setCardMsg(CARD_TARGET_MESSAGES[card.mechanic] ?? 'Click a square...');
      setSelectedCard(null);
      return;
    }

    // fog_village: always needs a board click (even if not in TARGETING_CARDS constant)
    if (card.mechanic === 'fog_village' || card.mechanic === 'fortress') {
      setCardPending({ card, playerColor, mechanic: card.mechanic, step: 1, data: {} });
      setCardMsg('🌫️ Click any square to center your 3×3 Fog of War zone there!');
      setSelectedCard(null);
      return;
    }

    // invisible: needs a board click to pick which piece to make invisible
    if (card.mechanic === 'invisible') {
      setCardPending({ card, playerColor, mechanic: card.mechanic, step: 1, data: {} });
      setCardMsg('👁️ Click YOUR piece to make invisible for 1 round (not king)!');
      setSelectedCard(null);
      return;
    }

    // Self-harm cards that need confirmation before execution
    const selfHarmMechanics = new Set(['gambler', 'badsniper', 'unabomber']);
    if (selfHarmMechanics.has(card.mechanic) && !window.confirm(`⚠️ Play "${card.name}"? ${card.desc}\n\nClick OK to confirm, Cancel to back out.`)) {
      setSelectedCard(null);
      pendingCardUseRef.current.delete(card.id);
      return;
    }

    // Instant cards
    const finishInstant = () => {
      removeCardFromHand(card, playerColor);
      cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
      setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
      setSelectedCard(null);
      pendingCardUseRef.current.delete(card.id);
    };

    switch (card.mechanic) {

      case 'gambler': {
        const oppHand = opp === 'white' ? whiteHand : blackHand;
        const myHand  = playerColor === 'white' ? whiteHand : blackHand;
        const win     = Math.random() < 0.5;
        if (win && oppHand.length > 0) {
          const stolen = oppHand[Math.floor(Math.random() * oppHand.length)];
          if (playerColor === 'white') {
            setBlackHand(h => h.filter(c => c.id !== stolen.id));
            setWhiteHand(h => h.length < MAX_HAND_SIZE ? [...h.filter(c => c.id !== card.id), stolen] : h.filter(c => c.id !== card.id));
          } else {
            setWhiteHand(h => h.filter(c => c.id !== stolen.id));
            setBlackHand(h => h.length < MAX_HAND_SIZE ? [...h.filter(c => c.id !== card.id), stolen] : h.filter(c => c.id !== card.id));
          }
          setCardMsg(`🎲 WIN! Stole "${stolen.name}" from opponent!`);
          fireCardAnim('gambler_win', `Stole "${stolen.name}" ${stolen.icon}`);
        } else if (!win && myHand.length > 1) {
          const candidates = myHand.filter(c => c.id !== card.id);
          const given = candidates[Math.floor(Math.random() * candidates.length)];
          if (playerColor === 'white') {
            setWhiteHand(h => h.filter(c => c.id !== card.id && c.id !== given.id));
            setBlackHand(h => h.length < MAX_HAND_SIZE ? [...h, given] : h);
          } else {
            setBlackHand(h => h.filter(c => c.id !== card.id && c.id !== given.id));
            setWhiteHand(h => h.length < MAX_HAND_SIZE ? [...h, given] : h);
          }
          setCardMsg(`🎲 LOSE! Gave "${given.name}" to opponent...`);
          fireCardAnim('gambler_lose', `Gave away "${given.name}" ${given.icon}`);
        } else {
          setCardMsg('🎲 Gambler had no effect (empty hands)');
          removeCardFromHand(card, playerColor);
        }
        setTimeout(() => setCardMsg(''), 3000);
        cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
        setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
        setSelectedCard(null);
        pendingCardUseRef.current.delete(card.id);
        return;
      }

      case 'radar':
        setRadarActive(true);
        setCardMsg('📡 Radar active! Enemy hand revealed for this turn.');
        setTimeout(() => setCardMsg(''), 3000);
        finishInstant();
        return;

      case 'reverse': {
        if (snapshots.length < 2) {
          setCardMsg('⏪ No move to reverse yet!');
          setTimeout(() => setCardMsg(''), 2000);
          pendingCardUseRef.current.delete(card.id);
          removeCardFromHand(card, playerColor);
          cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
          setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
          setSelectedCard(null);
          return;
        }
        const prevSnap = snapshots[snapshots.length - 2];
        const restored = prevSnap.board.map(r => [...r]);
        const rkp  = findKing(restored, playerColor);
        const rokp = findKing(restored, opp);
        if (rkp && isAttackedWithFusion(restored, rkp.row, rkp.col, opp)) {
          setCardMsg('⏪ Cannot reverse — would leave your king in check!');
          setTimeout(() => setCardMsg(''), 2500);
          pendingCardUseRef.current.delete(card.id);
          return;
        }
        if (rokp && isAttackedWithFusion(restored, rokp.row, rokp.col, playerColor)) {
          setCardMsg('⏪ Cannot reverse — would put enemy king in check!');
          setTimeout(() => setCardMsg(''), 2500);
          pendingCardUseRef.current.delete(card.id);
          return;
        }
        setBoard(restored);
        setTurn(prevSnap.turn);
        setMoved(prevSnap.moved);
        setLm(prevSnap.lm);
        setHmc(prevSnap.hmc);
        setFmn(prevSnap.fmn);
        setSnapshots(prev => prev.slice(0, -1));
        setMovHist(prev => {
          const nx = [...prev];
          const last = nx[nx.length - 1];
          if (last?.b) nx[nx.length - 1] = { ...last, b: undefined };
          else nx.pop();
          return nx;
        });
        setCardMsg("⏪ Reversed opponent's last move!");
        setTimeout(() => setCardMsg(''), 2500);
        fireCardAnim('reverse', "Opponent's last move undone");
        finishInstant();
        return;
      }

      case 'undo':
        setCardMsg("↩️ Undo used! Opponent's last card effect is cancelled (next card they play is nullified).");
        setTimeout(() => setCardMsg(''), 3000);
        finishInstant();
        return;

      case 'mirror': {
        if (!lm) {
          setCardMsg('🪞 No move to mirror yet!');
          setTimeout(() => setCardMsg(''), 2000);
          pendingCardUseRef.current.delete(card.id);
          removeCardFromHand(card, playerColor);
          cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
          setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
          setSelectedCard(null);
          return;
        }
        const dr = lm.to.row - lm.from.row;
        const dc = lm.to.col - lm.from.col;
        const movedType = board[lm.to.row][lm.to.col]?.type;
        const nb: Board = board.map(r => r.map(p => p ? { ...p } : null));
        let mirrored = false;
        outer: for (let r = 0; r < 8; r++) {
          for (let c = 0; c < 8; c++) {
            const p = nb[r][c];
            if (!p || p.color !== playerColor || p.type !== movedType) continue;
            const [tr, tc] = [r + dr, c + dc];
            if (!inB(tr, tc) || nb[tr][tc]?.color === playerColor) continue;
            const test = cloneBoard(nb);
            test[tr][tc] = test[r][c]; test[r][c] = null;
            const kp = findKing(test, playerColor);
            if (kp && isAttackedWithFusion(test, kp.row, kp.col, opp)) continue;
            nb[tr][tc] = nb[r][c]; nb[r][c] = null;
            mirrored = true;
            break outer;
          }
        }
        setBoard(nb);
        setCardMsg(mirrored ? "🪞 Mirrored opponent's last move!" : '🪞 No valid mirror move found — card wasted!');
        setTimeout(() => setCardMsg(''), 2500);
        finishInstant();
        return;
      }

      case 'cheater':
        setCheaterTurnsLeft(3);
        setCheaterColor(playerColor);
        cheaterColorRef.current = playerColor;
        analyse(toFEN(board, turn, moved, lm, hmc, fmn), turn);
        setCardMsg('💡 Cheater active for 3 turns! Engine panel shows best move.');
        setTimeout(() => setCardMsg(''), 4000);
        finishInstant();
        return;

      default:
        finishInstant();
    }
  }, [
    canUseCard, removeCardFromHand, finishCardUse, activateDoubleMove,
    whiteHand, blackHand, snapshots, lm, board, turn, moved, hmc, fmn,
    analyse, openJokerPicker, applyAuthoritativeSnapshot, authoritativeActorForColor,
  ]);

  // ── New game ───────────────────────────────────────────────────────────────
  const newGame = React.useCallback(() => {
    gameIdRef.current += 1;
    stop();
    setBoard(makeBoard());
    setTurn('white');
    setSel(null);
    setHints([]);
    setMoved(new Set());
    setLm(null);
    setDrag(null);
    setPromo(null);
    setCheck(false);
    setMate(false);
    setStale(false);
    setInsuf(false);
    setHmc(0);
    setFmn(1);
    setPosHist([]);
    setDrawOffer(null);
    setOver(false);
    setWinner(null);
    setMovHist([]);
    setSnapshots([]);
    setReviewIdx(-1);
    setReviewBoard(null);
    setEngineOn(false);
    setChatMessages([]);
    setChatInput('');
    resetTimer();
    blackMovedRef.current = false;
    finalPositionRef.current = null;
    cardUsedByRef.current = { white: false, black: false };
    setCardUsedBy({ white: false, black: false });
    pendingCardUseRef.current = new Set();
    setSelectedCard(null);
    setWhiteHand([]);
    setBlackHand([]);
    setLastDrawAnim(null);
    setDealPhase('idle');
    setCardPending(null);
    setCardMsg('');
    setPromoPicker(null);
    setLavaSquares([]);
    setLavaExploding([]);
    setFogZones([]);
    setBombPieces([]);
    setBombExploding([]);
    setGhostPiece(null);
    ghostRef.current = null;
    setSwapAnim(null);
    setJokerPicker(null);
    setRadarActive(false);
    setCheaterTurnsLeft(0);
    setCheaterColor(null);
    cheaterColorRef.current = null;
    setCardPromo(null);
    setDoubleMove(null);
    setCardAnim(null);
    setViewerSeat(null);
    viewerSeatRef.current = null;
    setMatchSeatMeta(null);
      setAuthoritativeLive(false);
      setAuthoritativeMatchId(null);
      setAuthoritativeStatus(null);
      setAuthoritativeFinishReason(null);
      setAuthoritativeWhiteConnected(false);
      setAuthoritativeBlackConnected(false);
      setAuthoritativeDisconnectGraceFor(null);
      setAuthoritativeDisconnectGraceDeadline(null);
      authoritativeMatchIdRef.current = null;
      authoritativeSeatSecretsRef.current = { white: null, black: null };
      authoritativeClaimExpiresAtRef.current = { white: null, black: null };
    authoritativeClaimTokensRef.current = { white: null, black: null };
    gatewayBootstrapClaimsRef.current = { matchId: null, whiteSecret: null, blackSecret: null, whiteToken: null, blackToken: null, whiteExpiresAt: null, blackExpiresAt: null };
    gatewayRecoveredMatchIdRef.current = null;
    requestedMatchIdRef.current = null;
    finalizedResultRef.current = null;
    writeStoredActiveMatchId(null);
    clearRequestedMatchQuery();
    setGameKey(k => k + 1);
    if (hostedRuntime) {
      setActivePage('Play');
      return;
    }
    setTimeout(() => startAbortCountdown(), 0);
    void bootstrapAuthoritativeMatch();
  }, [stop, setTicking, startAbortCountdown, bootstrapAuthoritativeMatch, hostedRuntime]);

  const returnToQueueHome = React.useCallback(() => {
    setQueueLaunchIntent(null);
    newGame();
  }, [newGame]);

  const returnToSameQueueLane = React.useCallback(() => {
    if (!authoritativeMatchId) {
      returnToQueueHome();
      return;
    }
    const roomMeta = readStoredRoomMeta(authoritativeMatchId);
    if (roomMeta?.queue === 'casual' || roomMeta?.queue === 'rated') {
      setQueueLaunchIntent({
        queue: roomMeta.queue,
        modeId: roomMeta.modeId ?? DEFAULT_MATCH_MODE_ID,
      });
      newGame();
      return;
    }
    returnToQueueHome();
  }, [authoritativeMatchId, newGame, returnToQueueHome]);

  // ── Computed values ─────────────────────────────────────────────────────────
  const kingPos      = check && !isReviewing ? findKing(board, turn) : null;

  // Filter moves that would leave own king attacked by any fused enemy piece
  const filterFusionChecks = React.useCallback((
    b: Board,
    moves: Sq[],
    fromRow: number,
    fromCol: number,
    playerColor: PieceColor,
  ): Sq[] => {
    const opp = playerColor === 'white' ? 'black' : 'white';
    return moves.filter(sq => {
      const nb: Board = b.map(r => r.map(p => p ? { ...p } : null));
      nb[sq.row][sq.col] = nb[fromRow][fromCol];
      nb[fromRow][fromCol] = null;
      const kp = findKing(nb, playerColor);
      if (!kp) return true;
      return !isAttackedWithFusion(nb, kp.row, kp.col, opp);
    });
  }, [isAttackedWithFusion]);

  const getMoves = React.useCallback(
    (r: number, c: number) => {
      const ghost = ghostRef.current;
      // Ghost piece — use normal legal moves but on a board where the ghost is placed at its position
      if (ghost && ghost.ownerColor === turn && ghost.row === r && ghost.col === c) {
        const ghostBoard: Board = board.map(row => row.map(p => p ? { ...p } : null));
        ghostBoard[r][c] = { ...ghost.piece };
        const moves = legalMoves(ghostBoard, r, c, lm, moved);
        return filterFusionChecks(ghostBoard, moves, r, c, turn);
      }
      // Fused piece — return union of both piece types' legal moves, filtered for fusion checks
      const p = board[r][c];
      if (p?.fusedWith) {
        const moves = getFusedMoves(board, r, c, p.type, p.fusedWith);
        return filterFusionChecks(board, moves, r, c, p.color);
      }
      // Normal piece — filter for enemy fused piece checks too
      const moves = legalMoves(board, r, c, lm, moved);
      return filterFusionChecks(board, moves, r, c, turn);
    },
    [board, lm, moved, turn, getFusedMoves, filterFusionChecks],
  );

  const canControlColor = React.useCallback((color: PieceColor): boolean => {
    if (!hostedRuntime) {
      return true;
    }
    return viewerSeatRef.current === color;
  }, [hostedRuntime]);

  const canActWithColor = React.useCallback((color: PieceColor): boolean => (
    canControlColor(color) && turnRef.current === color
  ), [canControlColor]);

  const canSelectPiece = React.useCallback((row: number, col: number): boolean => {
    const dm = doubleMoveRef.current;
    if (!dm || dm.movesLeft === 2) return true;
    const ts = dm.trackedSq;
    if (!ts) return true;
    if (dm.type === 'same' && (row !== ts.row || col !== ts.col)) {
      setCardMsg(`🏃 Solo: move the SAME piece at ${FILES[ts.col]}${RANKS[ts.row]}!`);
      return false;
    }
    if (dm.type === 'diff' && row === ts.row && col === ts.col) {
      setCardMsg('👥 Twin: move a DIFFERENT piece!');
      return false;
    }
    return true;
  }, []);

  const toggleAnalysisArrow = React.useCallback((from: Sq, to: Sq) => {
    setAnalysisArrows((current: any) => {
      const existingIndex = current.findIndex(
        (arrow: any) =>
          arrow.from.row === from.row &&
          arrow.from.col === from.col &&
          arrow.to.row === to.row &&
          arrow.to.col === to.col,
      );
      if (existingIndex >= 0) {
        return current.filter((_: any, index: number) => index !== existingIndex);
      }
      return [...current, { from, to }];
    });
  }, []);

  const clearAnalysisArrows = React.useCallback(() => {
    setAnalysisArrows((current: any) => (current.length ? [] : current));
  }, []);

  const clickSq = React.useCallback((r: number, c: number) => {
    if (cardPending) { handleCardClick(r, c); return; }
    if (isReviewing || over || promo) return;
    const p = board[r][c];
    const ghost = ghostRef.current;
    const isGhostSq = ghost && canActWithColor(ghost.ownerColor) && ghost.row === r && ghost.col === c;
    const myColor = hostedRuntime ? viewerSeatRef.current : turnRef.current;
    const isMyTurn = turnRef.current === myColor;
    const canPremove = hostedRuntime && authoritativeMatchIdRef.current && myColor && !isMyTurn && !overRef.current;

    if (canPremove && !sel) {
      if (p && p.color === myColor && canSelectPiece(r, c)) {
        setSel({ row: r, col: c });
        setHints(getMoves(r, c));
        setCardMsg('🔄 Premove set: click destination');
      }
      return;
    }

    if (canPremove && sel && hints.some(m => m.row === r && m.col === c)) {
      setPremove({ from: sel, to: { row: r, col: c } });
      setSel(null);
      setHints([]);
      setCardMsg('✔ Premove queued');
      setTimeout(() => { if (premoveRef.current) setCardMsg('⏳ Premove will fire when turn starts'); }, 1200);
      return;
    }

    if (!sel) {
      if (isGhostSq || (p && canActWithColor(p.color) && canSelectPiece(r, c))) {
        setSel({ row: r, col: c });
        setHints(getMoves(r, c));
      }
      return;
    }

    if (isGhostSq || (p && canControlColor(p.color))) {
      if (sel.row === r && sel.col === c) { setSel(null); setHints([]); }
      else if ((isGhostSq || canSelectPiece(r, c)) && (!p || canActWithColor(p.color))) { setSel({ row: r, col: c }); setHints(getMoves(r, c)); }
      return;
    }

    if (!hints.some(m => m.row === r && m.col === c)) { setSel(null); setHints([]); return; }

    doMove(sel.row, sel.col, r, c);
    setSel(null);
    setHints([]);
  }, [cardPending, isReviewing, over, promo, board, sel, hints, canSelectPiece, getMoves, doMove, handleCardClick, canActWithColor, canControlColor, hostedRuntime, viewerSeatRef]);

  // ── Board highlight helpers ─────────────────────────────────────────────────
  const getCardHighlight = React.useCallback((row: number, col: number): string | null => {
    if (!cardPending) return null;
    const { mechanic, step, playerColor, data } = cardPending;
    const piece = board[row][col];
    const opp   = OPP[playerColor];
    switch (mechanic) {
      case 'freeze':     return piece?.color === opp && piece.type !== 'king' ? 'rgba(96,165,250,0.55)' : null;
      case 'shield':     return piece?.color === playerColor && piece.type !== 'king' ? 'rgba(74,222,128,0.55)' : null;
      case 'sniper':     return piece && piece.type !== 'king' && piece.color !== playerColor ? 'rgba(192,132,252,0.55)' : null;
      case 'badsniper':  return piece?.color === playerColor && piece.type !== 'king' ? 'rgba(107,114,128,0.55)' : null;
      case 'mindcontrol':
      case 'borrow':     return piece?.color === opp && piece.type !== 'king' ? 'rgba(168,85,247,0.5)' : null;
      case 'promote':
      case 'demote':     return step === 1 && piece?.color === playerColor && piece.type !== 'king' ? 'rgba(245,158,11,0.55)' : null;
      case 'jump': {
        if (step === 1 && piece?.color === playerColor && piece.type !== 'king' && piece.type !== 'knight') return 'rgba(74,222,128,0.55)';
        if (step === 2) {
          const from = data.from as Sq | undefined;
          const pt = data.pieceType as PieceType | undefined;
          const pc = data.pieceColor as PieceColor | undefined;
          if (from && row === from.row && col === from.col) return 'rgba(245,158,11,0.6)';
          if (from && pt && pc && piece?.color !== playerColor) {
            const dr = row - from.row, dc = col - from.col;
            if (dr === 0 && dc === 0) return null;
            const diag = Math.abs(dr) === Math.abs(dc), straight = dr === 0 || dc === 0;
            let dirOk = false;
            if (pt === 'bishop') dirOk = diag;
            else if (pt === 'rook') dirOk = straight;
            else if (pt === 'queen') dirOk = diag || straight;
            else if (pt === 'pawn') {
              const fwd = pc === 'white' ? 1 : -1;
              dirOk = (dc === 0 && (dr === fwd || dr === fwd * 2)) || (Math.abs(dc) === 2 && dr === fwd * 2);
            }
            if (!dirOk) return null;
            const sr = Math.sign(dr), sc = Math.sign(dc);
            let count = 0;
            let r = from.row + sr, c = from.col + sc;
            while (r !== row || c !== col) {
              if (board[r][c]) count++;
              r += sr; c += sc;
            }
            if (count === 1) {
              if (pt === 'pawn' && dc === 0) return !piece ? 'rgba(74,222,128,0.45)' : null;
              if (pt === 'pawn' && Math.abs(dc) === 2) return piece?.type === 'king' ? null : (piece ? 'rgba(248,113,113,0.6)' : 'rgba(74,222,128,0.45)');
              if (piece?.type === 'king') return null;
              return piece ? 'rgba(248,113,113,0.6)' : 'rgba(74,222,128,0.45)';
            }
          }
        }
        return null;
      }
      case 'teleport': {
        if (step === 1 && piece?.color === playerColor && piece.type !== 'king') return 'rgba(192,132,252,0.55)';
        if (step === 2 && !piece) return 'rgba(192,132,252,0.35)';
        if (step === 2) {
          const from = data.from as Sq | undefined;
          if (from && row === from.row && col === from.col) return 'rgba(245,158,11,0.6)';
        }
        return null;
      }
      case 'smallsacrifice':
      case 'bigsacrifice': {
        const selected = (data.selected as Sq[] | undefined) ?? [];
        if (selected.some(s => s.row === row && s.col === col)) return 'rgba(231,76,60,0.7)';
        if (piece?.color === playerColor && piece.type !== 'king') return 'rgba(231,76,60,0.25)';
        return null;
      }
      case 'swapme': {
        const sq1s = data.sq1 as Sq | undefined;
        if (sq1s && row === sq1s.row && col === sq1s.col) return 'rgba(74,222,128,0.85)'; // selected piece
        if (step === 1 && piece?.color === playerColor && piece.type !== 'king') return 'rgba(74,222,128,0.4)';
        if (step === 2 && piece?.color === playerColor && piece.type !== 'king') return 'rgba(74,222,128,0.5)';
        return null;
      }
      case 'swapus': {
        const sq1s = data.sq1 as Sq | undefined;
        if (sq1s && row === sq1s.row && col === sq1s.col) return 'rgba(74,222,128,0.85)';
        if (step === 1 && piece?.color === playerColor && piece.type !== 'king') return 'rgba(74,222,128,0.4)';
        if (step === 2 && piece?.color === opp && piece.type !== 'king') return 'rgba(248,113,113,0.5)';
        return null;
      }
      case 'swaphim': {
        const sq1s = data.sq1 as Sq | undefined;
        if (sq1s && row === sq1s.row && col === sq1s.col) return 'rgba(248,113,113,0.85)';
        if (step === 1 && piece?.color === opp && piece.type !== 'king') return 'rgba(248,113,113,0.4)';
        if (step === 2 && piece?.color === opp && piece.type !== 'king') return 'rgba(248,113,113,0.5)';
        return null;
      }
      case 'parasite': {
        const hostSq2 = data.hostSq as Sq | undefined;
        const hostVal = data.hostValue as number | undefined;
        if (step === 1 && piece?.color === playerColor && piece.type !== 'king') return 'rgba(168,85,247,0.5)';
        if (step === 2) {
          if (hostSq2 && row === hostSq2.row && col === hostSq2.col) return 'rgba(168,85,247,0.85)';
          if (piece?.color === opp && piece.type !== 'king' && hostVal !== undefined && PIECE_VALUE[piece.type] === hostVal) return 'rgba(168,85,247,0.5)';
        }
        return null;
      }
      case 'lavaground': return !piece ? 'rgba(255,80,0,0.45)' : null;
      case 'fog_village': return 'rgba(100,180,255,0.22)';
      case 'unabomber':  return step === 1 && piece?.color === playerColor && piece.type !== 'king' ? 'rgba(255,120,30,0.55)' : null;
      case 'invisible':  return piece?.color === playerColor && piece.type !== 'king' ? 'rgba(200,200,255,0.50)' : null;
      case 'halffuse': {
        const HALF_CAP = 6;
        const sq1  = data.sq1 as Sq | undefined;
        const val1 = data.val1 as number | undefined;
        if (step === 1) {
          if (!piece || piece.color !== playerColor || piece.type === 'king' || piece.fusedWith) return null;
          const v = PIECE_VALUE[piece.type];
          return v < HALF_CAP ? 'rgba(251,191,36,0.55)' : 'rgba(251,191,36,0.18)'; // dim if too expensive alone
        }
        if (step === 2) {
          if (sq1 && row === sq1.row && col === sq1.col) return 'rgba(251,191,36,0.85)'; // selected piece glow
          if (piece?.color === playerColor && piece.type !== 'king' && !piece.fusedWith) {
            const adjacent = sq1 && Math.abs(row - sq1.row) <= 1 && Math.abs(col - sq1.col) <= 1;
            if (!adjacent) return 'rgba(251,191,36,0.12)'; // too far away
            const combined = (val1 ?? 0) + PIECE_VALUE[piece.type];
            return combined <= HALF_CAP ? 'rgba(251,191,36,0.55)' : 'rgba(248,113,113,0.35)'; // red if over cap
          }
        }
        return null;
      }
      case 'fullfusion': {
        const sq1 = data.sq1 as Sq | undefined;
        if (step === 1) return piece?.color === playerColor && piece.type !== 'king' && !piece.fusedWith ? 'rgba(167,139,250,0.55)' : null;
        if (step === 2) {
          if (sq1 && row === sq1.row && col === sq1.col) return 'rgba(167,139,250,0.85)'; // selected piece
          if (piece?.color === playerColor && piece.type !== 'king' && !piece.fusedWith) {
            const adjacent = sq1 && Math.abs(row - sq1.row) <= 1 && Math.abs(col - sq1.col) <= 1;
            if (!adjacent) return 'rgba(167,139,250,0.12)'; // too far — dimmed
            return 'rgba(167,139,250,0.55)';
          }
        }
        return null;
      }
      default: return null;
    }
  }, [cardPending, board]);

  const getDoubleMoveHighlight = React.useCallback((row: number, col: number): string | null => {
    if (!doubleMove?.trackedSq || doubleMove.movesLeft !== 1) return null;
    const ts = doubleMove.trackedSq;
    if (doubleMove.type === 'same' && row === ts.row && col === ts.col) return 'rgba(74,222,128,0.7)';
    if (doubleMove.type === 'diff' && row === ts.row && col === ts.col) return 'rgba(231,76,60,0.6)';
    return null;
  }, [doubleMove]);

  // ── Formatting helpers ──────────────────────────────────────────────────────
  const fmtClock = (s: number): string => `${Math.floor(s / 60)}:${(s % 60).toString().padStart(2, '0')}`;

  const evalStr = (score: number, m: number | null): string => {
    if (m !== null) return m > 0 ? `M${Math.abs(m)}` : `-M${Math.abs(m)}`;
    return `${score > 0 ? '+' : ''}${score.toFixed(2)}`;
  };

  const evalLabel = (score: number, m: number | null): string => {
    if (m !== null) return m > 0 ? 'White forces checkmate' : 'Black forces checkmate';
    if (score >  2) return 'White is winning';
    if (score >  0.5) return 'White is better';
    if (score < -2) return 'Black is winning';
    if (score < -0.5) return 'Black is better';
    return 'Equal position';
  };

  // ── Bomb radius highlight (shows which squares would be destroyed) ────────
  const renderPlayerCard = (seat: PieceColor): React.ReactElement => {
    const isWhiteSeat = seat === 'white';
    const seatName = isWhiteSeat ? displayedWhiteName : displayedBlackName;
    const seatRating = isWhiteSeat ? displayedWhiteRating : displayedBlackRating;
    const seatTime = isWhiteSeat ? timeW : timeB;
    const seatBadge = isWhiteSeat ? whiteSeatBadge : blackSeatBadge;
    const seatTicking = tickingState === seat && clockActive && !over;

    return (
      <PlayerBar
        seat={seat}
        playerName={seatName}
        rating={seatRating}
        timeMs={seatTime * 1000} // Assuming timeW/timeB are in seconds
        isClockActive={seatTicking}
        seatBadge={seatBadge ?? undefined}
      />
    );
  };

  // ── JOKER PICKER OVERLAY ───────────────────────────────────────────────────
  const renderJokerPicker = () => {
    if (!jokerPicker) return null;
    const { card: jokerCard, playerColor, filterRarity, transforming } = jokerPicker;
    const rarities: (Rarity | 'all')[] = ['all', 'trash', 'common', 'rare', 'epic', 'legendary'];
    const filteredPool = (filterRarity === 'all'
      ? CARD_POOL
      : CARD_POOL.filter(c => c.rarity === filterRarity))
      .filter(c => !authoritativeMatchIdRef.current || AUTHORITATIVE_JOKER_MECHANICS.has(c.mechanic));

    return (
      <div ref={jokerRef} role="dialog" aria-modal="true" aria-label="Select card" style={{
        position:'fixed', inset:0, zIndex:1000,
        background:'rgba(0,0,0,0.88)',
        backdropFilter:'blur(8px)',
        display:'flex', alignItems:'center', justifyContent:'center',
      }} onClick={e => { if (e.target === e.currentTarget && !transforming) cancelCard(); }}>
        <div style={{
          background:'linear-gradient(160deg, #1a0a2e 0%, #0d0a1e 50%, #0a1020 100%)',
          border:'2px solid rgba(245,158,11,0.6)',
          borderRadius:'20px', padding:'24px',
          width:'680px', maxWidth:'95vw',
          maxHeight:'85vh', overflow:'hidden',
          display:'flex', flexDirection:'column', gap:'16px',
          boxShadow:'0 0 60px rgba(245,158,11,0.3), 0 20px 60px rgba(0,0,0,0.8)',
          animation:'jokerPickerReveal 0.35s cubic-bezier(0.34,1.56,0.64,1)',
        }}>
          {/* Header */}
          <div style={{ display:'flex', alignItems:'center', gap:'14px', borderBottom:'1px solid rgba(245,158,11,0.25)', paddingBottom:'14px' }}>
            <div style={{ fontSize:'40px', animation: transforming ? 'jokerTransform 0.8s ease-in-out' : 'jokerFloat 3s ease-in-out infinite' }}>🃏</div>
            <div style={{ flex:1 }}>
              <div style={{ color:'#f59e0b', fontWeight:800, fontSize:'20px', letterSpacing:'1px' }}>JOKER — Choose Your Transformation</div>
              <div style={{ color:'rgba(200,180,120,0.7)', fontSize:'12px', marginTop:'3px' }}>
                {transforming
                  ? '✨ Transforming...'
                  : `Pick any card from the pool — the Joker will become it instantly.`
                }
              </div>
            </div>
            {!transforming && (
              <button onClick={cancelCard} style={{ width:'32px', height:'32px', borderRadius:'50%', background:'rgba(231,76,60,0.2)', border:'1px solid rgba(231,76,60,0.5)', color:'#e74c3c', fontSize:'16px', cursor:'pointer', display:'flex', alignItems:'center', justifyContent:'center', fontWeight:700 }}>✕</button>
            )}
          </div>

          {/* Rarity filter tabs */}
          <div style={{ display:'flex', gap:'6px', flexWrap:'wrap' }}>
            {rarities.map(r => {
              const style = r === 'all' ? { accent: '#a0b8d8', glow: 'rgba(160,184,216,0.3)', label: 'ALL' } : RARITY_STYLE[r as Rarity];
              const isActive = filterRarity === r;
              const count = r === 'all' ? CARD_POOL.length : CARD_POOL.filter(c => c.rarity === r).length;
              return (
                <button key={r} onClick={() => setJokerPicker(prev => prev ? { ...prev, filterRarity: r } : null)}
                  style={{
                    padding:'4px 12px', borderRadius:'20px', fontSize:'10px', fontWeight:800,
                    cursor:'pointer', border: isActive ? `1px solid ${style.accent}` : '1px solid rgba(255,255,255,0.1)',
                    background: isActive ? `${style.accent}22` : 'rgba(255,255,255,0.03)',
                    color: isActive ? style.accent : 'rgba(200,215,235,0.45)',
                    textTransform:'uppercase', letterSpacing:'0.8px',
                    boxShadow: isActive ? `0 0 10px ${style.glow}` : 'none',
                    transition:'all 0.15s ease',
                  }}>
                  {style.label} ({count})
                </button>
              );
            })}
          </div>

          {/* Card grid */}
          <div style={{ overflowY:'auto', flex:1 }}>
            <div style={{ display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(90px, 1fr))', gap:'10px', paddingRight:'4px' }}>
              {filteredPool.map((template, idx) => {
                const style = RARITY_STYLE[template.rarity];
                return (
                  <div key={`${template.mechanic}-${idx}`}
                    onClick={() => !transforming && applyJokerTransform(jokerCard, playerColor, template)}
                    style={{
                      background:`linear-gradient(160deg, ${template.color || style.color} 0%, #050810 100%)`,
                      border:`1px solid ${style.accent}44`,
                      borderRadius:'10px', padding:'10px 8px',
                      cursor: transforming ? 'wait' : 'pointer',
                      display:'flex', flexDirection:'column', alignItems:'center', gap:'5px',
                      transition:'all 0.18s cubic-bezier(0.34,1.56,0.64,1)',
                      opacity: transforming ? 0.5 : 1,
                    }}
                    onMouseEnter={e => {
                      if (transforming) return;
                      const el = e.currentTarget as HTMLDivElement;
                      el.style.transform = 'scale(1.1) translateY(-4px)';
                      el.style.border = `1px solid ${style.accent}cc`;
                      el.style.boxShadow = `0 8px 24px rgba(0,0,0,0.5), 0 0 16px ${style.glow}`;
                    }}
                    onMouseLeave={e => {
                      const el = e.currentTarget as HTMLDivElement;
                      el.style.transform = 'scale(1) translateY(0)';
                      el.style.border = `1px solid ${style.accent}44`;
                      el.style.boxShadow = 'none';
                    }}
                  >
                    <div style={{ fontSize:'24px', lineHeight:1 }}>{template.icon}</div>
                    <div style={{ color:'#fff', fontWeight:700, fontSize:'7.5px', textAlign:'center', lineHeight:'1.3' }}>{template.name}</div>
                    <div style={{
                      padding:'1px 6px', borderRadius:'3px', fontSize:'6px', fontWeight:800,
                      color: style.accent, background:`${style.accent}22`,
                      border:`1px solid ${style.accent}44`,
                      textTransform:'uppercase', letterSpacing:'0.5px',
                    }}>{style.label}</div>
                    <div style={{ fontSize:'6px', color:'rgba(160,184,216,0.5)', textAlign:'center', lineHeight:'1.3', maxHeight:'24px', overflow:'hidden' }}>{template.desc.slice(0, 50)}{template.desc.length > 50 ? '…' : ''}</div>
                  </div>
                );
              })}
            </div>
          </div>

          {transforming && (
            <div style={{ textAlign:'center', padding:'12px', background:'rgba(245,158,11,0.1)', border:'1px solid rgba(245,158,11,0.4)', borderRadius:'10px' }}>
              <div style={{ fontSize:'28px', animation:'jokerSpin 0.8s linear' }}>🃏</div>
              <div style={{ color:'#f59e0b', fontWeight:700, fontSize:'13px', marginTop:'6px' }}>✨ Transforming the Joker...</div>
            </div>
          )}
        </div>
      </div>
    );
  };


  return {
    sfReady, isThinking, ev, sfErr, analyse, stop, resetEval,
    actorSeatForHostedControls, controlSender,
    shellPageMeta,
    primaryNavItems, utilityGroups, topSeat, bottomSeat,
    authoritativeMatchId, setAuthoritativeMatchId, primaryAccountIdentity,
    board,
    setBoard,
    turn,
    setTurn,
    sel,
    setSel,
    hints,
    setHints,
    moved,
    setMoved,
    lm,
    setLm,
    drag,
    setDrag,
    dragPos,
    setDragPos,
    promo,
    setPromo,
    check,
    setCheck,
    mate,
    setMate,
    stale,
    setStale,
    insuf,
    setInsuf,
    hmc,
    setHmc,
    fmn,
    setFmn,
    posHist,
    setPosHist,
    drawOffer,
    setDrawOffer,
    over,
    setOver,
    winner,
    setWinner,
    authoritativeFinishReason,
    setAuthoritativeFinishReason,
    movHist,
    setMovHist,
    snapshots,
    setSnapshots,
    reviewIdx,
    setReviewIdx,
    analysisArrows,
    setAnalysisArrows,
    openProfileHandle,
    openReplayMatch,
    openGuestHistory,
    openLiveMatch,
    copyLiveMatchLink,
    copyReplayPageLink,
    dismissedSocialAlertIdsRef,
        setPrimaryAccountIdentity,
    shellAccountNotice,
    setShellAccountNotice,
    syncPrimaryAccountIdentity,
    clearPrimaryAccountRestriction,
    pulseSocialLive,
    handleSeatAuthenticated,
    handlePrimaryShellAuthenticated,
    whiteHand,
    setWhiteHand,
    blackHand,
    setBlackHand,
    selectedCard,
    setSelectedCard,
    dealPhase,
    setDealPhase,
    lastDrawAnim,
    setLastDrawAnim,
    cardPending,
    setCardPending,
    cardMsg,
    setCardMsg,
    promoPicker,
    setPromoPicker,
    cardPromo,
    setCardPromo,
    cardUsedBy,
    setCardUsedBy,
    jokerPicker,
    setJokerPicker,
    cardAnim,
    setCardAnim,
    cardAnimLbl,
    setCardAnimLbl,
    fireCardAnim,
    bombPieces,
    setBombPieces,
    bombExploding,
    setBombExploding,
    bombPiecesRef,
    swapAnim,
    setSwapAnim,
    swapAnimTimerRef,
    triggerSwapAnim,
    transformAnim,
    setTransformAnim,
    transformAnimTimerRef,
    triggerTransformAnim,
    sniperAnim,
    setSniperAnim,
    sniperAnimTimerRef,
    teleportAnim,
    setTeleportAnim,
    teleportAnimTimerRef,
    jumpAnim,
    setJumpAnim,
    jumpAnimTimerRef,
    sacrificeAnim,
    setSacrificeAnim,
    sacrificeAnimTimerRef,
    triggerSacrificeAnim,
    mindControlAnim,
    setMindControlAnim,
    mindControlAnimTimerRef,
    triggerMindControlAnim,
    fuseAnim,
    setFuseAnim,
    fuseAnimTimerRef,
    triggerFuseAnim,
    triggerJumpAnim,
    triggerSniperAnim,
    triggerTeleportAnim,
    pendingCardUseRef,
    cardUsedByRef,
    resetCardUsed,
    lastDrawRound,
    roundNumber,
    chatMessages,
    setChatMessages,
    chatInput,
    setChatInput,
    chatRef,
    timeW,
    setTimeW,
    timeB,
    setTimeB,
    clockActive,
    setClockActive,
    abortCountdown,
    setAbortCountdown,
    abortActive,
    setAbortActive,
    abortRef,
    whiteProfileRef,
    blackProfileRef,
    viewerSeatRef,
    guestSessionSecretsRef,
    authoritativeSeatIdsRef,
    authoritativeSeatSecretsRef,
    authoritativeClaimExpiresAtRef,
    authoritativeClaimTokensRef,
    gatewayBootstrapClaimsRef,
    applyGatewayGuestSessions,
    applyGatewayAccountSessions,
    buildGatewayBootstrapRequest,
    applyGatewayMatchClaims,
    blackMovedRef,
    reviewBoard,
    setReviewBoard,
    engineOn,
    setEngineOn,
    hostedRuntime,
    viewerSeat,
    authoritativeRematchBusy,
    authoritativeLive,
    setAuthoritativeLive,
    streamDisconnected,
    onStreamReconnect,
            authoritativeStatus,
    setAuthoritativeStatus,
    authoritativeWhiteConnected,
    setAuthoritativeWhiteConnected,
    authoritativeBlackConnected,
    setAuthoritativeBlackConnected,
    authoritativeDisconnectGraceFor,
    setAuthoritativeDisconnectGraceFor,
    authoritativeDisconnectGraceDeadline,
    setAuthoritativeDisconnectGraceDeadline,
    createAuthoritativeRematchRoom,
    cheaterTurnsLeft,
    setCheaterTurnsLeft,
    cheaterColor,
    setCheaterColor,
    cheaterColorRef,
    cheaterActive,
    radarActive,
    setRadarActive,
    doubleMove,
    setDoubleMove,
    doubleMoveRef,
    lavaSquares,
    setLavaSquares,
    lavaExploding,
    setLavaExploding,
    ghostPiece,
    setGhostPiece,
    ghostRef,
    fogZones,
    setFogZones,
    movRef,
    finalPositionRef,
    tickingRef,
    tickingState,
    setTickingState,
    setTicking,
    boardRef,
    lavaSquaresRef,
    turnRef,
    movedRef,
    lmRef,
    hmcRef,
    fmnRef,
    posHistRef,
    overRef,
    gameIdRef,
    gameKey,
    setGameKey,
    authoritativeMatchIdRef,
    authoritativeBootstrapRef,
    requestedMatchIdRef,
    finalizedResultRef,
    buildMoveRows,
    buildPendingCardFromSnapshot,
    applyAuthoritativeSnapshot,
    bootstrapAuthoritativeMatch,
    submitAuthoritativeIntent,
    authoritativePlayerIdForColor,
    authoritativeGuestSessionSecretForColor,
    authoritativePlayerSecretForColor,
    authoritativePlayerClaimTokenForColor,
    authoritativeActorForColor,
    processBombs,
    stopAbortCountdown,
    startAbortCountdown,
    boardStatusLabel,
    hasPrimaryAccountSession,
    showSocialNav,
    showAdminNav,
    secondaryNavItems,
    activeSecondaryNav,
    showReturnToMatch,
    showPlayHub,
    showBoardSurface,
    controlledSeat,
    topHand,
    bottomHand,
    whiteSeatBadge,
    blackSeatBadge,
    showHostedSoloBanner,
    showHostedReconnectWarning,
    intentInFlight,
    activeDisconnectGraceFor,
    disconnectGraceDeadlineLabel,
    whitePresenceLabel,
    blackPresenceLabel,
    activeFinishReason,
    activeFinishReasonLabel,
    displayedWhiteName,
    displayedBlackName,
    disconnectGraceBanner,
    displayedWhiteRating,
    displayedBlackRating,
    activeMatchRoomMeta,
    activeMatchModeLabel,
    activeMatchQueueLabel,
    canCreateDirectRematch,
    canQueueSameLane,
    activeMatchRoleLabel,
    activeMatchRouteLabel,
    finishedPrimaryActionLabel,
    finishedSecondaryActionLabel,
    activeLiveMatchUrl,
    activeReplayPageUrl,
    topSeatBadge,
    bottomSeatBadge,
    topPlayerName,
    bottomPlayerName,
    topPlayerRating,
    bottomPlayerRating,
    topPlayerClock,
    bottomPlayerClock,
    topClockTicking,
    bottomClockTicking,
    actorSeatPlainLabel,
    hostedActionLocked,
    canRespondToDrawOffer,
    actorSeatLabel,
    hostedSpectator,
    visibleSocialAlert,
    dismissSocialAlert,
    handleSocialAlertAction,
    isAttackedWithFusion,
    checkEndGame,
    handleLavaLanding,
    canSubmitAuthoritativeMove,
    doMove,
    doPromo,
    removeCardFromHand,
    finishCardUse,
    jokerPickerRef,
    cancelCard,
    getSafeTransforms,
    getFusedMoves,
    checkFusionRedundancy,
    activateDoubleMove,
    openJokerPicker,
    applyJokerTransform,
    handleCardClick,
    handlePromoPick,
    canUseCard,
    applyCard,
    newGame,
    returnToQueueHome,
    returnToSameQueueLane,
    goToSnap,
    reviewFirst,
    reviewPrev,
    reviewNext,
    reviewLast,
    isReviewing,
    kingPos,
    filterFusionChecks,
    getMoves,
    canControlColor,
    canActWithColor,
    canSelectPiece,
    toggleAnalysisArrow,
    clearAnalysisArrows,
    clickSq,
    getCardHighlight,
    getDoubleMoveHighlight,
    fmtClock,
    evalStr,
    evalLabel,
    renderJokerPicker,
    premove,
    setPremove,
    premoveRef,
  };
}