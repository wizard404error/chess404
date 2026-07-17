'use client';
import { useMatchEngine, type UseMatchEngineProps } from './hooks/useMatchEngine';

import React from 'react';
import { PlatformContext } from './contexts/PlatformContext';
import { usePathname, useRouter } from 'next/navigation';
import type { MatchFinishReason, MatchModeId, MatchSnapshotMessage, MatchState as AuthoritativeMatchState, PlayerIntent } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import { useStockfish } from './usestockfish';
import type { Board, PieceType, PieceColor, Piece, Sq, GameCard, CardMechanic, CardPendingState, DoubleMove, BombPiece, LavaSquare, Snapshot, Rarity, CardAnimType } from './types';
import { makeBoard, cloneBoard, findKing, isAttacked, inB, legalMoves, gameStatus, insuffMat, positionKey, threefold, toFEN, moveNotation, uciToSan } from './chessEngine';
import { CARD_POOL, drawRandomCard, incrementCardSeq } from './cardPool';
import { RARITY_STYLE, RARITY_WEIGHTS, OPP, FILES, RANKS, SQ, MAX_HAND_SIZE, CLOCK_START, ABORT_SECS, DRAW_FROM, DRAW_EVERY, INITIAL_DEAL_ROUND, PIECE_VALUE, UPGRADE, DOWNGRADE, TARGETING_CARDS, CARD_TARGET_MESSAGES } from './constants';
import { GLOBAL_STYLES } from './styles';
import { MatchBoardView } from './components/match/MatchBoardView';
import MatchEngineProvider from './contexts/MatchEngineProvider';
import { CardAnimOverlay } from './CardAnimOverlay';
import AdminModerationPage from './AdminModerationPage';
import AuthPage from './AuthPage';
import CardsPage from './CardsPage';
import FriendsPage from './FriendsPage';
import HistoryPage from './HistoryPage';
import InboxPage from './InboxPage';
import PlayHubPage from './PlayHubPage';
import ProfilesPage from './ProfilesPage';
import WatchPage from './WatchPage';
import RankingsPage from './RankingsPage';
import CommunityPage from './CommunityPage';
import StatusPage from './StatusPage';
import AccountPage from './AccountPage';
import AppShell, { type ShellNavGroup, type ShellNavItem, type ShellPageMeta } from './components/layout/AppShell';
import { ErrorBoundary } from './components/ErrorBoundary';
import { useTutorial } from './hooks/useTutorial';
import { OnboardingTutorial } from './components/OnboardingTutorial';
import { NavBar } from './components/layout/NavBar';
import { SocialAlertBanner } from './components/SocialAlertBanner';
import { useSound, playSound } from './hooks/useSound';
import { useAccessibility } from './hooks/useAccessibility';
import { useToast } from './hooks/useToast';
import { ToastContainer } from './components/Toast';
import { useOnlineStatus } from './hooks/useOnlineStatus';
import {
  AdminIcon,
  CardsIcon,
  CommunityIcon,
  FriendsIcon,
  HistoryIcon,
  InboxIcon,
  PlayIcon,
  ProfileIcon,
  StatusIcon,
  TrophyIcon,
  WatchIcon,
} from './components/layout/icons';
import { fetchGatewayBootstrap } from './lib/system-service';
import { joinPrivateMatch, rematchPrivateMatch } from './lib/private-match-service';
import {
  applyIntent,
  configureMatchServiceRuntime,
  connectToMatchStream,
  createMatch,
  ensureMatch,
  fetchMatch,
  readStoredRoomMeta,
  resolveSeatSecret,
  sendMatchPresenceHeartbeat,
  writeStoredRoomMeta,
  type MatchServiceRuntimeConfig,
  type StoredRoomMeta,
} from './lib/match-service';
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
} from './lib/platform-service';
import type { QueueName, QueueTicket } from './lib/matchmaking-service';
import {
  modeLabel,
  queueLabel,
  finishReasonLabel,
  buildSocialAlert,
  type SocialAlert,
} from './lib/match-labels';
import {
  ACTIVE_MATCH_STORAGE_KEY,
  CLAIM_REFRESH_CHECK_INTERVAL_MS,
  CLAIM_REFRESH_LEAD_MS,
  MATCH_PRESENCE_HEARTBEAT_INTERVAL_MS,
  STREAM_RECONNECT_MESSAGE,
  PRESENCE_RETRY_MESSAGE,
  readStoredActiveMatchId,
  writeStoredActiveMatchId,
  readStoredGuestIdentity,
  writeStoredGuestIdentity,
  clearStoredGuestIdentity,
  readStoredAccountIdentity,
  writeStoredAccountIdentity,
  clearStoredAccountIdentity,
  clearRequestedMatchQuery,
  syncRequestedProfileQuery,
  syncRequestedHistoryQuery,
  syncRequestedMatchQuery,
  buildLiveMatchUrl,
  buildReplayPageUrl,
  copyTextToClipboard,
} from './lib/session-storage';

export type AppPage =
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

// â”€â”€â”€ App â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
const AUTHORITATIVE_JOKER_MECHANICS = new Set<CardMechanic>([
  'freeze', 'shield', 'sniper', 'badsniper', 'promote', 'demote', 'promotehim', 'demotehim',
  'teleport', 'jump', 'doublemove_diff', 'doublemove_same', 'swapme', 'swapus', 'swaphim',
  'borrow', 'mindcontrol', 'parasite', 'clone', 'fakepiece', 'lavaground', 'blackhole',
  'fortress',
  'fog_village', 'invisible', 'unabomber', 'halffuse', 'fullfusion', 'reverse', 'undo',
  'mirror', 'smallsacrifice', 'bigsacrifice', 'gambler', 'radar', 'cheater'
]);


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


export default function App({ runtimeConfig, children }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string }, children?: React.ReactNode }) {
  configureMatchServiceRuntime({
    httpBaseUrl: runtimeConfig?.matchServiceHttpBase,
    wsBaseUrl: runtimeConfig?.matchServiceWsBase,
  } satisfies MatchServiceRuntimeConfig);
  const [hostedRuntime, setHostedRuntime] = React.useState<boolean | null>(null);
  const [activePage, setActivePage] = React.useState<AppPage>('Play');
  
  // App Router pathname -> activePage bridge
  const router = useRouter();
  const pathname = usePathname();
  React.useEffect(() => {
    if (!pathname) return;
    if (pathname === '/' || pathname === '/play') setActivePage('Play');
    else if (pathname === '/watch') setActivePage('Watch');
    else if (pathname === '/history') setActivePage('History');
    else if (pathname === '/friends') setActivePage('Friends');
    else if (pathname === '/inbox') setActivePage('Inbox');
    else if (pathname === '/profiles') setActivePage('Profiles');
    else if (pathname === '/cards') setActivePage('Cards');
    else if (pathname === '/rankings') setActivePage('Rankings');
    else if (pathname === '/community') setActivePage('Community');
    else if (pathname === '/status') setActivePage('Status');
    else if (pathname === '/account') setActivePage('Account');
    else if (pathname === '/admin') setActivePage('Admin');
    else if (pathname === '/lobbies') setActivePage('Lobbies');
    else if (pathname.startsWith('/match/')) setActivePage('Match');
  }, [pathname]);

  const [secondaryMenuOpen, setSecondaryMenuOpen] = React.useState(false);
  const [friendsAttentionCount, setFriendsAttentionCount] = React.useState(0);
  const [inboxUnreadCount, setInboxUnreadCount] = React.useState(0);
  const [socialAlert, setSocialAlert] = React.useState<SocialAlert | null>(null);
  const [socialLiveToken, setSocialLiveToken] = React.useState(0);
  const [profileFocusHandle, setProfileFocusHandle] = React.useState<string | null>(null);
  const [profileQueryReady, setProfileQueryReady] = React.useState(false);
  const [historyQueryReady, setHistoryQueryReady] = React.useState(false);
  const [matchQueryReady, setMatchQueryReady] = React.useState(false);
  const [accountActionQueryDetected, setAccountActionQueryDetected] = React.useState(false);
  const [matchDestinationNotice, setMatchDestinationNotice] = React.useState('');
  const [queueLaunchIntent, setQueueLaunchIntent] = React.useState<{ modeId: MatchModeId; queue: QueueName } | null>(null);
  const [bootstrapQueueRecovery, setBootstrapQueueRecovery] = React.useState<{
    white: QueueTicket | null;
    black: QueueTicket | null;
  } | null>(null);
  const openedBoardMatchRef = React.useRef<string | null>(null);
  const [communityFocusGuestId, setCommunityFocusGuestId] = React.useState<string | null>(null);
  const [historyFocusMatchId, setHistoryFocusMatchId] = React.useState<string | null>(null);
  const [historyFocusGuestId, setHistoryFocusGuestId] = React.useState<string | null>(null);
  const tutorial = useTutorial();
  const [authoritativeRematchBusy, setAuthoritativeRematchBusy] = React.useState(false);
  const [whiteProfile, setWhiteProfile] = React.useState<GuestProfile | null>(null);
  const [blackProfile, setBlackProfile] = React.useState<GuestProfile | null>(null);
  const [viewerSeat, setViewerSeat] = React.useState<PieceColor | null>(null);
  const [matchSeatMeta, setMatchSeatMeta] = React.useState<{
    whiteGuestId?: string;
    blackGuestId?: string;
    whiteName?: string;
    blackName?: string;
  } | null>(null);
  const [guestProfilesReady, setGuestProfilesReady] = React.useState(false);
  const [confirmResign, setConfirmResign] = React.useState<'idle' | 'prompting'>('idle');
  const lastDrawOfferTime = React.useRef(0);
  const DRAW_COOLDOWN_MS = 15000;
  const boardWrapperRef = React.useRef<HTMLDivElement>(null);
  const [boardWrapperPx, setBoardWrapperPx] = React.useState(SQ * 8);

  React.useEffect(() => {
    const el = boardWrapperRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = entry.contentBoxSize?.[0]?.inlineSize ?? entry.contentRect.width;
        if (w > 0) setBoardWrapperPx(Math.round(w));
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const matchEngine = useMatchEngine({
    accountActionQueryDetected,
    activePage,
    authoritativeRematchBusy,
    blackProfile,
    communityFocusGuestId,
    friendsAttentionCount,
    guestProfilesReady,
    historyFocusGuestId,
    historyFocusMatchId,
    historyQueryReady,
    hostedRuntime,
    inboxUnreadCount,
    matchDestinationNotice,
    matchQueryReady,
    matchSeatMeta,
    openedBoardMatchRef,
    pathname,
    profileFocusHandle,
    profileQueryReady,
    bootstrapQueueRecovery,
    queueLaunchIntent,
    router,
    setAccountActionQueryDetected,
    setActivePage,
    setAuthoritativeRematchBusy,
    setBlackProfile,
    setFriendsAttentionCount,
    setGuestProfilesReady,
    setHistoryFocusGuestId,
    setHistoryFocusMatchId,
    setHistoryQueryReady,
    setHostedRuntime,
    setInboxUnreadCount,
    setMatchDestinationNotice,
    setMatchQueryReady,
    setMatchSeatMeta,
    setProfileFocusHandle,
    setProfileQueryReady,
    setBootstrapQueueRecovery,
    setQueueLaunchIntent,
    setSecondaryMenuOpen,
    setSocialAlert,
    setSocialLiveToken,
    setViewerSeat,
    setWhiteProfile,
    socialAlert,
    socialLiveToken,
    viewerSeat,
    whiteProfile
  });
  const {
    authoritativeMatchId, setAuthoritativeMatchId, primaryAccountIdentity,
    primaryNavItems, utilityGroups, topSeat, bottomSeat,
    shellPageMeta,
    actorSeatForHostedControls, controlSender,
    sfReady, isThinking, ev, sfErr, analyse, stop, resetEval,
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
    authoritativeLive,
    setAuthoritativeLive,
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
    premove,
    setPremove,
    clickSq,
    getCardHighlight,
    getDoubleMoveHighlight,
    fmtClock,
    evalStr,
    evalLabel,
    renderJokerPicker
  } = matchEngine;

  const { soundEnabled, toggleSound } = useSound();
  const { colorBlindMode, toggleColorBlind } = useAccessibility();
  const { messages: toastMessages, toast: showToast, dismiss: dismissToast } = useToast();
  const online = useOnlineStatus();

  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (target.closest?.('button') || target.closest?.('a') || target.closest?.('textarea') || target.closest?.('input') || target.closest?.('select')) return;
      if (e.key === 'ArrowLeft' && reviewIdx > 0) {
        e.preventDefault();
        reviewPrev();
      }
      if (e.key === 'ArrowRight' && reviewIdx < snapshots.length - 1) {
        e.preventDefault();
        reviewNext();
      }
      if (e.key === 'Escape') {
        setSel(null); setHints([]); setDrag(null); setDragPos(null);
        setPromo(null); setCardPromo(null);
      }
      if (e.key === ' ' && over) {
        e.preventDefault();
        setEngineOn(v => !v);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [reviewIdx, snapshots.length, over, reviewPrev, reviewNext]);

  const prevCheckRef = React.useRef(check);
  const prevOverRef = React.useRef(over);
  const prevMoveLenRef = React.useRef(movHist.length);
  const prevChatLenRef = React.useRef(chatMessages.length);

  React.useEffect(() => {
    if (check && check !== prevCheckRef.current) {
      playSound('check');
    }
    prevCheckRef.current = check;
  }, [check]);

  React.useEffect(() => {
    if (over && over !== prevOverRef.current) {
      playSound('game_over');
    }
    prevOverRef.current = over;
  }, [over]);

  React.useEffect(() => {
    if (movHist.length > prevMoveLenRef.current) {
      playSound('move');
    }
    prevMoveLenRef.current = movHist.length;
  }, [movHist.length]);

  React.useEffect(() => {
    if (chatMessages.length > prevChatLenRef.current) {
      playSound('chat');
    }
    prevChatLenRef.current = chatMessages.length;
  }, [chatMessages.length]);

  const timerWarningPlayedRef = React.useRef<boolean>(false);
  React.useEffect(() => {
    if (tickingState && clockActive && !over && authoritativeLive) {
      const warned = tickingState === 'white' ? timeW <= 15000 : timeB <= 15000;
      if (warned && !timerWarningPlayedRef.current) {
        playSound('timer_warning');
        timerWarningPlayedRef.current = true;
      } else if (!warned) {
        timerWarningPlayedRef.current = false;
      }
    }
  }, [timeW, timeB, tickingState, clockActive, over, authoritativeLive]);

  const platformContextValue = React.useMemo(() => ({
    hostedRuntime, setHostedRuntime,
    whiteProfile, blackProfile,
    queueLaunchIntent,
    activeMatchRoomMeta,
    authoritativeMatchId, setAuthoritativeMatchId,
    primaryAccountIdentity,
    boardStatusLabel,
    viewerSeat,
    matchDestinationNotice,
    activePage, setActivePage,
    openLiveMatch, openReplayMatch,
    openProfileHandle, openGuestHistory,
    historyFocusMatchId, setHistoryFocusMatchId,
    historyFocusGuestId, setHistoryFocusGuestId,
    communityFocusGuestId, setCommunityFocusGuestId,
    socialLiveToken,
    setInboxUnreadCount, setFriendsAttentionCount,
    profileFocusHandle,
    shellAccountNotice, setShellAccountNotice,
    hasPrimaryAccountSession,
    accountActionQueryDetected,
    handlePrimaryShellAuthenticated,
    handleSeatAuthenticated,
    syncPrimaryAccountIdentity,
    writeStoredActiveMatchId,
    clearRequestedMatchQuery,
    requestedMatchIdRef,
    readStoredGuestIdentity,
    copyLiveMatchLink,
    showReturnToMatch,
    activeMatchQueue: activeMatchRoomMeta?.queue ?? null,
    activeMatchModeId: activeMatchRoomMeta?.modeId ?? null,
    setQueueLaunchIntent,
    setMatchDestinationNotice,
    setBootstrapQueueRecovery,
    openAuthoritativeMatch: openLiveMatch,
  }), [
    hostedRuntime, setHostedRuntime,
    whiteProfile, blackProfile,
    queueLaunchIntent,
    activeMatchRoomMeta,
    authoritativeMatchId, setAuthoritativeMatchId,
    primaryAccountIdentity,
    boardStatusLabel,
    viewerSeat,
    matchDestinationNotice,
    activePage, setActivePage,
    openLiveMatch, openReplayMatch,
    openProfileHandle, openGuestHistory,
    historyFocusMatchId, setHistoryFocusMatchId,
    historyFocusGuestId, setHistoryFocusGuestId,
    communityFocusGuestId, setCommunityFocusGuestId,
    socialLiveToken,
    setInboxUnreadCount, setFriendsAttentionCount,
    profileFocusHandle,
    shellAccountNotice, setShellAccountNotice,
    hasPrimaryAccountSession,
    accountActionQueryDetected,
    handlePrimaryShellAuthenticated,
    handleSeatAuthenticated,
    syncPrimaryAccountIdentity,
    writeStoredActiveMatchId,
    clearRequestedMatchQuery,
    requestedMatchIdRef,
    readStoredGuestIdentity,
    copyLiveMatchLink,
    showReturnToMatch,
    activeMatchRoomMeta,
    setQueueLaunchIntent,
    setMatchDestinationNotice,
    setBootstrapQueueRecovery,
  ]);

  // â”€â”€ Loading skeleton â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
  if (hostedRuntime === null) {
    return (
      <div style={{
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        minHeight: '100vh', background: '#0a0d16', color: '#ffbe5a', gap: '16px'
      }}>
        <div style={{ fontSize: '28px', fontWeight: 800, letterSpacing: '2px' }}>â™Ÿ CHESS404</div>
        <div style={{
          width: '200px', height: '4px', borderRadius: '2px',
          background: 'rgba(255,190,90,0.15)', overflow: 'hidden'
        }}>
          <div style={{
            width: '40%', height: '100%', borderRadius: '2px',
            background: '#ffbe5a', animation: 'loadingSlide 1.2s ease-in-out infinite'
          }} />
        </div>
      </div>
    );
  }

  // â”€â”€ Render â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
  return (
    <PlatformContext.Provider value={platformContextValue}>
    {!online && (
      <div style={{
        position: 'fixed', top: 0, left: 0, right: 0, zIndex: 9999,
        background: '#dc2626', color: '#fff', textAlign: 'center',
        padding: '10px 16px', fontWeight: 700, fontSize: '14px'
      }}>
        ðŸ”´ You are offline â€” some features may not work
      </div>
    )}

    <main id="main-content" style={{
      display:'flex', flexDirection:'column', height:'100vh', overflow:'hidden',
      fontFamily:"'Segoe UI', sans-serif",
      backgroundImage:'url(/background.png)',
      backgroundSize:'cover',
      backgroundPosition:'center',
      backgroundRepeat:'no-repeat',
      backgroundAttachment:'fixed',
      position:'relative',
    }}>
      {/* Cinematic overlay â€” lighter so background shows */}
      <div style={{ position:'fixed', inset:0, background:'linear-gradient(160deg, rgba(8,4,20,0.45) 0%, rgba(15,6,30,0.35) 50%, rgba(5,2,15,0.50) 100%)', pointerEvents:'none', zIndex:0 }} />
      <style>{GLOBAL_STYLES}</style>

      {/* â”€â”€ CARD ANIMATION OVERLAY â”€â”€ */}
      <CardAnimOverlay anim={cardAnim} label={cardAnimLbl} onDone={() => setCardAnim(null)} />

      {/* â”€â”€ JOKER PICKER PORTAL â”€â”€ */}
      {renderJokerPicker()}

      <NavBar
        activePage={activePage}
        primaryNavItems={primaryNavItems}
        secondaryNavItems={secondaryNavItems}
        showReturnToMatch={showReturnToMatch}
        hasPrimaryAccountSession={hasPrimaryAccountSession}
        activeSecondaryNav={activeSecondaryNav}
        secondaryMenuOpen={secondaryMenuOpen}
        setSecondaryMenuOpen={setSecondaryMenuOpen}
        setActivePage={setActivePage}
        accountLabel={hasPrimaryAccountSession ? 'Account' : 'Sign In'}
      />

      <SocialAlertBanner
        visible={visibleSocialAlert}
        onAction={handleSocialAlertAction}
        onDismiss={dismissSocialAlert}
      />

      <AppShell
        brandTitle="Chess404"
        brandSubtitle="Card Chess"
        pageMeta={shellPageMeta}
        primaryItems={primaryNavItems}
        utilityGroups={utilityGroups}
        accountLabel={hasPrimaryAccountSession ? 'Account' : 'Sign In'}
        activeKey={activePage}
        onNavigate={(key) => {
          const k = key as string;
          if (k === 'Play') router.push('/play');
          else if (k === 'Watch') router.push('/watch');
          else if (k === 'History') router.push('/history');
          else if (k === 'Friends') router.push('/friends');
          else if (k === 'Inbox') router.push('/inbox');
          else if (k === 'Profiles') router.push('/profiles');
          else if (k === 'Cards') router.push('/cards');
          else if (k === 'Rankings') router.push('/rankings');
          else if (k === 'Community') router.push('/community');
          else if (k === 'Status') router.push('/status');
          else if (k === 'Admin') router.push('/admin');
          else setActivePage(key as any);
        }}
        onOpenAccount={() => router.push('/account')}
        showReturnToMatch={showReturnToMatch}
        onReturnToMatch={() => {
          if (authoritativeMatchId) {
            router.push(`/match/${authoritativeMatchId}`);
          } else {
            setActivePage('Match');
          }
        }}
      >
      {children}
      {showPlayHub ? (
        <ErrorBoundary>
        <PlayHubPage
          hostedRuntime={hostedRuntime}
          whiteProfile={whiteProfile}
          blackProfile={blackProfile}
          preferredQueue={queueLaunchIntent?.queue}
          preferredModeId={queueLaunchIntent?.modeId}
          queueRecovery={bootstrapQueueRecovery}
          displayName={whiteProfile?.displayName ?? null}
          identity={{
            guestId: readStoredGuestIdentity('white').guestId,
            sessionSecret: readStoredGuestIdentity('white').sessionSecret,
            sessionToken: readStoredGuestIdentity('white').sessionToken,
            accountId: primaryAccountIdentity.accountId,
            accountSessionToken: primaryAccountIdentity.sessionToken,
          }}
          activeMatchId={authoritativeMatchId}
          activeMatchQueue={activeMatchRoomMeta?.queue ?? null}
          activeMatchModeId={activeMatchRoomMeta?.modeId ?? null}
          boardStatusLabel={boardStatusLabel}
          viewerSeat={viewerSeat}
          matchDestinationNotice={matchDestinationNotice}
          onReturnToMatch={() => {
            if (authoritativeMatchId) {
              void openLiveMatch(authoritativeMatchId);
            }
          }}
          onCopyMatchLink={(matchId) => { void copyLiveMatchLink(matchId); }}
          tutorialActive={tutorial.active}
        />
        </ErrorBoundary>
      ) : activePage === 'Account' ? (
        !hasPrimaryAccountSession && !accountActionQueryDetected ? (
          <AuthPage
            hostedRuntime={hostedRuntime}
            guestProfile={whiteProfile}
            externalNotice={shellAccountNotice}
            onAuthenticated={handlePrimaryShellAuthenticated}
            onOpenAccount={() => setActivePage('Account')}
            onContinue={() => setActivePage('Play')}
            onAuthStateChange={syncPrimaryAccountIdentity}
          />
        ) : (
          <AccountPage
            whiteProfile={whiteProfile}
            blackProfile={blackProfile}
            externalNotice={shellAccountNotice}
            onOpenProfile={openProfileHandle}
            onSeatAuthenticated={handleSeatAuthenticated}
            onAuthStateChange={syncPrimaryAccountIdentity}
          />
        )
      ) : showBoardSurface && hostedRuntime && !authoritativeMatchId ? (
        <ErrorBoundary>
        <div style={{ display:'flex', flex:1, minHeight:0, alignItems:'center', justifyContent:'center', padding:'28px' }}>
          <div style={{
            width:'min(720px, 100%)',
            padding:'28px 30px',
            borderRadius:'20px',
            background:'linear-gradient(180deg, rgba(14,18,30,0.96) 0%, rgba(9,12,20,0.98) 100%)',
            border:'1px solid rgba(255,165,40,0.18)',
            boxShadow:'0 18px 60px rgba(0,0,0,0.35)',
            textAlign:'center',
          }}>
            <div style={{ fontSize:'14px', fontWeight:800, letterSpacing:'1.5px', textTransform:'uppercase', color:'#ffcf72', marginBottom:'10px' }}>
              No Active Online Match
            </div>
            <div style={{ color:'#f3e6bf', fontSize:'28px', fontWeight:800, marginBottom:'10px' }}>
              Return to the play hub
            </div>
            <div style={{ color:'rgba(255,232,180,0.72)', fontSize:'14px', lineHeight:1.6, maxWidth:'560px', margin:'0 auto 20px' }}>
              On the hosted site, online play starts from the Play hub. Open quick pair or create a private invite room there, then come back once a real room exists.
            </div>
            <div style={{ display:'flex', gap:'12px', justifyContent:'center', flexWrap:'wrap' }}>
              <button
                onClick={() => setActivePage('Play')}
                style={{
                  padding:'12px 22px',
                  background:'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)',
                  color:'#fff8e0',
                  border:'1px solid rgba(255,180,60,0.45)',
                  borderRadius:'10px',
                  cursor:'pointer',
                  fontSize:'13px',
                  fontWeight:800,
                  boxShadow:'0 6px 20px rgba(200,134,10,0.35)',
                }}
              >
                Go To Play
              </button>
              <button
                onClick={() => {
                  writeStoredActiveMatchId(null);
                  clearRequestedMatchQuery();
                  requestedMatchIdRef.current = null;
                  setActivePage('Play');
                }}
                style={{
                  padding:'12px 22px',
                  background:'rgba(255,255,255,0.03)',
                  color:'rgba(255,232,180,0.82)',
                  border:'1px solid rgba(255,255,255,0.10)',
                  borderRadius:'10px',
                  cursor:'pointer',
                  fontSize:'13px',
                  fontWeight:700,
                }}
              >
                Clear Stale Match State
              </button>
            </div>
          </div>
        </div>
        </ErrorBoundary>
      ) : showBoardSurface ? (
        <ErrorBoundary>
      <MatchEngineProvider engine={matchEngine} hostedRuntime={hostedRuntime} viewerSeat={viewerSeat} authoritativeRematchBusy={authoritativeRematchBusy}>
      <MatchBoardView
        board={board}
        turn={turn}
        sel={sel}
        hints={hints}
        lm={lm}
        drag={drag}
        dragPos={dragPos}
        check={check}
        kingPos={kingPos}
        over={over}
        winner={winner}
        topSeat={topSeat}
        bottomSeat={bottomSeat}
        topPlayerName={topPlayerName}
        bottomPlayerName={bottomPlayerName}
        topSeatBadge={topSeatBadge}
        bottomSeatBadge={bottomSeatBadge}
        displayedWhiteRating={displayedWhiteRating}
        displayedBlackRating={displayedBlackRating}
        displayedWhiteName={displayedWhiteName}
        displayedBlackName={displayedBlackName}
        whiteSeatBadge={whiteSeatBadge}
        blackSeatBadge={blackSeatBadge}
        timeW={timeW}
        timeB={timeB}
        clockActive={clockActive}
        tickingState={tickingState}
        fmtClock={fmtClock}
        hostedRuntime={hostedRuntime}
        authoritativeMatchId={authoritativeMatchId}
        authoritativeMatchIdRef={authoritativeMatchIdRef}
        viewerSeat={viewerSeat}
        controlSender={controlSender}
        authoritativeLive={authoritativeLive}
        authoritativeStatus={authoritativeStatus}
        engineOn={engineOn}
        setEngineOn={setEngineOn}
        ev={ev}
        selectedCard={selectedCard}
        cardPending={cardPending}
        whiteHand={whiteHand}
        blackHand={blackHand}
        topHand={topHand}
        bottomHand={bottomHand}
        cardUsedBy={cardUsedBy}
        canUseCard={canUseCard}
        applyCard={applyCard}
        cancelCard={cancelCard}
        cardMsg={cardMsg}
        setCardMsg={setCardMsg}
        streamDisconnected={matchEngine.streamDisconnected}
        onReconnect={matchEngine.onStreamReconnect}
        clickSq={clickSq}
        getMoves={getMoves}
        doMove={doMove}
        promo={promo}
        doPromo={doPromo}
        promoPicker={promoPicker}
        handlePromoPick={handlePromoPick}
        cardPromo={cardPromo}
        setCardPromo={setCardPromo}
        getCardHighlight={getCardHighlight}
        getDoubleMoveHighlight={getDoubleMoveHighlight}
        bombPieces={bombPieces}
        bombExploding={bombExploding}
        lavaSquares={lavaSquares}
        lavaExploding={lavaExploding}
        swapAnim={swapAnim}
        transformAnim={transformAnim}
        sniperAnim={sniperAnim}
        teleportAnim={teleportAnim}
        jumpAnim={jumpAnim}
        sacrificeAnim={sacrificeAnim}
        mindControlAnim={mindControlAnim}
        fuseAnim={fuseAnim}
        fogZones={fogZones}
        ghostPiece={ghostPiece}
        ghostRef={ghostRef}
        analysisArrows={analysisArrows}
        toggleAnalysisArrow={toggleAnalysisArrow}
        clearAnalysisArrows={clearAnalysisArrows}
        premove={premove}
        setPremove={setPremove}
        isReviewing={isReviewing}
        reviewBoard={reviewBoard}
        reviewIdx={reviewIdx}
        chatMessages={chatMessages}
        setChatMessages={setChatMessages}
        movHist={movHist}
        submitAuthoritativeIntent={submitAuthoritativeIntent}
        authoritativeActorForColor={authoritativeActorForColor}
        createAuthoritativeRematchRoom={createAuthoritativeRematchRoom}
        hostedActionLocked={hostedActionLocked}
        drawOffer={drawOffer}
        canRespondToDrawOffer={canRespondToDrawOffer}
        setDrawOffer={setDrawOffer}
        abortActive={abortActive}
        abortCountdown={abortCountdown}
        stopAbortCountdown={stopAbortCountdown}
        activeFinishReasonLabel={activeFinishReasonLabel}
        authoritativeRematchBusy={authoritativeRematchBusy}
        canCreateDirectRematch={canCreateDirectRematch}
        canQueueSameLane={canQueueSameLane}
        returnToSameQueueLane={returnToSameQueueLane}
        returnToQueueHome={returnToQueueHome}
        newGame={newGame}
        finishedPrimaryActionLabel={finishedPrimaryActionLabel}
        finishedSecondaryActionLabel={finishedSecondaryActionLabel}
        boardStatusLabel={boardStatusLabel}
        roundNumber={roundNumber}
        lastDrawAnim={lastDrawAnim}
        soundEnabled={soundEnabled}
        toggleSound={toggleSound}
        colorBlindMode={colorBlindMode}
        toggleColorBlind={toggleColorBlind}
        showHostedReconnectWarning={showHostedReconnectWarning}
        intentInFlight={intentInFlight}
        activeDisconnectGraceFor={activeDisconnectGraceFor}
        bootstrapAuthoritativeMatch={bootstrapAuthoritativeMatch}
        showHostedSoloBanner={showHostedSoloBanner}
        isAttackedWithFusion={isAttackedWithFusion}
        checkEndGame={checkEndGame}
        setSel={setSel}
        setHints={setHints}
        setDrag={setDrag}
        setDragPos={setDragPos}
        setBoard={setBoard}
        setPosHist={setPosHist}
        setOver={setOver}
        setWinner={setWinner}
        moved={moved}
        hmc={hmc}
        fmn={fmn}
        posHist={posHist}
        doubleMove={doubleMove}
        radarActive={radarActive}
        finalPositionRef={finalPositionRef}
      />
      </MatchEngineProvider>
        </ErrorBoundary>
      ) : null}
      </AppShell>
    </main>
    <ToastContainer messages={toastMessages} onDismiss={dismissToast} />
    <OnboardingTutorial tutorial={tutorial} activePage={activePage} />
    </PlatformContext.Provider>
  );
}
