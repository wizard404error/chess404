'use client';
import { useMatchEngine, type UseMatchEngineProps } from './hooks/useMatchEngine';

import React from 'react';
import { PlatformContext } from './contexts/PlatformContext';
import { usePathname, useRouter } from 'next/navigation';
import { PlayerBar } from './components/match/PlayerBar';
import { GamePanel } from './components/match/GamePanel';
import type { MatchFinishReason, MatchModeId, MatchSnapshotMessage, MatchState as AuthoritativeMatchState, PlayerIntent } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import { useStockfish } from './usestockfish';
import type { Board, PieceType, PieceColor, Piece, Sq, GameCard, CardMechanic, CardPendingState, DoubleMove, BombPiece, LavaSquare, Snapshot, Rarity, CardAnimType } from './types';
import { makeBoard, cloneBoard, findKing, isAttacked, inB, legalMoves, gameStatus, insuffMat, positionKey, threefold, toFEN, moveNotation, uciToSan } from './chessEngine';
import { CARD_POOL, drawRandomCard, incrementCardSeq } from './cardPool';
import { RARITY_STYLE, RARITY_WEIGHTS, OPP, FILES, RANKS, SQ, MAX_HAND_SIZE, CLOCK_START, ABORT_SECS, DRAW_FROM, DRAW_EVERY, INITIAL_DEAL_ROUND, PIECE_VALUE, UPGRADE, DOWNGRADE, TARGETING_CARDS, CARD_TARGET_MESSAGES } from './constants';
import { GLOBAL_STYLES } from './styles';
import { BoardCanvas, type TransformAnim, type SniperAnim, type TeleportAnim, type JumpAnim, type SacrificeAnim, type MindControlAnim, type FuseAnim, type BoardArrow } from './BoardCanvas';
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
  finalizeAccountMatch,
  finalizeGuestMatch,
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
  const [hostedRuntime, setHostedRuntime] = React.useState(false);
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
    clockRef,
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
    renderHand,
    renderPlayerCard,
    renderJokerPicker
  } = matchEngine;

  // ── Render ──────────────────────────────────────────────────────────────────
  return (
    <>
    <PlatformContext.Provider value={React.useMemo(() => ({
      hostedRuntime, setHostedRuntime,
      whiteProfile, blackProfile,
      queueLaunchIntent,
            activeMatchRoomMeta,
    authoritativeMatchId, setAuthoritativeMatchId,
    primaryAccountIdentity,
      boardStatusLabel,
      viewerSeat,
      matchDestinationNotice,
      setActivePage,
      openLiveMatch,
      openReplayMatch,
      openProfileHandle,
      openGuestHistory,
      historyFocusMatchId, setHistoryFocusMatchId,
      historyFocusGuestId, setHistoryFocusGuestId,
      communityFocusGuestId, setCommunityFocusGuestId,
      socialLiveToken,
      setInboxUnreadCount,
      profileFocusHandle,
      shellAccountNotice,
      hasPrimaryAccountSession,
      accountActionQueryDetected,
      handlePrimaryShellAuthenticated,
      handleSeatAuthenticated,
      syncPrimaryAccountIdentity,
      writeStoredActiveMatchId,
      clearRequestedMatchQuery,
      requestedMatchIdRef,
      readStoredGuestIdentity,
      copyLiveMatchLink: (matchId: string) => { void copyLiveMatchLink(matchId); },
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
      setActivePage,
      openLiveMatch,
      openReplayMatch,
      openProfileHandle,
      openGuestHistory,
      historyFocusMatchId, setHistoryFocusMatchId,
      historyFocusGuestId, setHistoryFocusGuestId,
      communityFocusGuestId, setCommunityFocusGuestId,
      socialLiveToken,
      setInboxUnreadCount,
      profileFocusHandle,
      shellAccountNotice,
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
    ])}>
    <ErrorBoundary>
    <div style={{
      display:'flex', flexDirection:'column', height:'100vh', overflow:'hidden',
      fontFamily:"'Segoe UI', sans-serif",
      backgroundImage:'url(/background.png)',
      backgroundSize:'cover',
      backgroundPosition:'center',
      backgroundRepeat:'no-repeat',
      backgroundAttachment:'fixed',
      position:'relative',
    }}>
      {/* Cinematic overlay — lighter so background shows */}
      <div style={{ position:'fixed', inset:0, background:'linear-gradient(160deg, rgba(8,4,20,0.45) 0%, rgba(15,6,30,0.35) 50%, rgba(5,2,15,0.50) 100%)', pointerEvents:'none', zIndex:0 }} />
      <style>{GLOBAL_STYLES}</style>
      <style>{`
        @keyframes sacrificePulse {
          0%, 100% { filter: drop-shadow(0 0 4px rgba(220,20,20,0.6)); transform: scale(1); }
          50% { filter: drop-shadow(0 0 14px rgba(255,60,60,0.95)); transform: scale(1.12); }
        }
        @keyframes mindControlPulse {
          0%, 100% { filter: drop-shadow(0 0 4px rgba(139,0,255,0.5)); transform: scale(1) rotate(0deg); }
          33% { filter: drop-shadow(0 0 16px rgba(200,0,255,0.95)); transform: scale(1.1) rotate(-6deg); }
          66% { filter: drop-shadow(0 0 12px rgba(255,0,200,0.8)); transform: scale(1.08) rotate(6deg); }
        }
        @keyframes fusePulse {
          0%, 100% { filter: drop-shadow(0 0 4px rgba(251,191,36,0.5)); transform: scale(1); }
          50% { filter: drop-shadow(0 0 18px rgba(251,191,36,1)) drop-shadow(0 0 8px rgba(167,139,250,0.8)); transform: scale(1.15); }
        }
        .glass-card {
          background: rgba(255,255,255,0.05);
          backdrop-filter: blur(16px);
          -webkit-backdrop-filter: blur(16px);
        }
        .card-hand-slot:hover {
          transform: translateY(-8px) scale(1.04);
          filter: drop-shadow(0 12px 24px rgba(255,160,40,0.5));
        }
      `}</style>

      {/* ── CARD ANIMATION OVERLAY ── */}
      <CardAnimOverlay anim={cardAnim} label={cardAnimLbl} onDone={() => setCardAnim(null)} />

      {/* ── JOKER PICKER PORTAL ── */}
      {renderJokerPicker()}

      {/* TOP NAV */}
      <nav style={{
        display:'none', alignItems:'center', justifyContent:'space-between',
        padding:'0 28px', minHeight:'62px', flexShrink:0,
        background:'rgba(8,4,20,0.82)',
        backdropFilter:'blur(20px)',
        WebkitBackdropFilter:'blur(20px)',
        borderBottom:'1px solid rgba(255,165,40,0.25)',
        boxShadow:'0 4px 32px rgba(0,0,0,0.5), inset 0 -1px 0 rgba(255,140,0,0.1)',
        position:'relative', zIndex:100,
        gap:'18px',
        flexWrap:'wrap',
      }}>
        <div style={{ display:'flex', alignItems:'center', gap:'12px', minWidth:'180px' }}>
          <div style={{ width:'38px', height:'38px', borderRadius:'8px', background:'linear-gradient(135deg, #c8860a 0%, #8b5e0a 100%)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'20px', boxShadow:'0 0 18px rgba(200,134,10,0.6)', border:'1px solid rgba(255,180,60,0.5)' }}>♛</div>
          <span style={{ fontSize:'22px', fontWeight:800, letterSpacing:'1px', color:'#fff1c7' }}>CardChess</span>
        </div>
        <div style={{ display:'flex', alignItems:'center', gap:'6px', flex:'1 1 auto', flexWrap:'wrap' }}>
          {primaryNavItems.map((item, i) => (
            <button key={item.key} onClick={() => setActivePage(item.key as AppPage)} style={{
              padding:'8px 16px', fontSize:'13px', fontWeight: i===0?700:600,
              background: activePage===item.key?'linear-gradient(180deg, rgba(200,134,10,0.35) 0%, rgba(139,94,10,0.4) 100%)':'transparent',
              color: activePage===item.key?'#ffd700':'rgba(200,185,140,0.8)',
              border: activePage===item.key?'1px solid rgba(200,134,10,0.6)':'1px solid transparent',
              borderRadius:'6px', cursor:'pointer',
              borderBottom: activePage===item.key?'2px solid #c8860a':'2px solid transparent',
              transition:'all 0.15s ease',
              display:'flex',
              alignItems:'center',
              gap:'8px',
            }}
              onMouseEnter={e => { if (activePage!==item.key) { (e.target as HTMLButtonElement).style.color='#ffd700'; }}}
              onMouseLeave={e => { if (activePage!==item.key) (e.target as HTMLButtonElement).style.color='rgba(200,185,140,0.8)'; }}
            >
              <span>{item.label}</span>
            </button>
          ))}
        </div>
        <div style={{ display:'flex', gap:'10px', minWidth:'180px', justifyContent:'flex-end', alignItems:'center', marginLeft:'auto' }}>
          {showReturnToMatch ? (
            <button
              onClick={() => setActivePage('Match')}
              style={{
                padding:'8px 16px',
                fontSize:'12px',
                fontWeight:800,
                background: activePage === 'Match'
                  ? 'linear-gradient(180deg, rgba(58,110,210,0.9) 0%, rgba(28,54,112,0.95) 100%)'
                  : 'rgba(58,110,210,0.12)',
                color:'#eff6ff',
                border:'1px solid rgba(122,166,255,0.34)',
                borderRadius:'8px',
                cursor:'pointer',
              }}
            >
              Return To Match
            </button>
          ) : null}
          <button
            onClick={() => setSecondaryMenuOpen(current => !current)}
            style={{
              padding:'8px 14px',
              fontSize:'12px',
              fontWeight:700,
              background: activeSecondaryNav || secondaryMenuOpen ? 'rgba(255,255,255,0.08)' : 'transparent',
              color:'rgba(220,200,150,0.9)',
              border:'1px solid rgba(180,130,60,0.3)',
              borderRadius:'8px',
              cursor:'pointer',
            }}
          >
            More
          </button>
          <button onClick={() => setActivePage('Account')} style={{ padding:'8px 18px', fontSize:'13px', fontWeight:700, background:'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)', color:'#fff8e0', border:'1px solid rgba(255,180,60,0.5)', borderRadius:'8px', cursor:'pointer', boxShadow:'0 2px 14px rgba(200,134,10,0.5)' }}>{hasPrimaryAccountSession ? 'Account' : 'Sign In'}</button>
        </div>
        {secondaryMenuOpen ? (
          <div style={{
            position:'absolute',
            top:'calc(100% + 10px)',
            right:'28px',
            width:'min(320px, calc(100vw - 32px))',
            padding:'12px',
            borderRadius:'16px',
            background:'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.99) 100%)',
            border:'1px solid rgba(255,165,40,0.18)',
            boxShadow:'0 18px 48px rgba(0,0,0,0.35)',
            display:'grid',
            gap:'8px',
          }}>
            <div style={{ color:'#ffcf72', fontSize:'11px', fontWeight:800, letterSpacing:'1.2px', textTransform:'uppercase', padding:'4px 6px 8px' }}>
              Secondary Surfaces
            </div>
            {secondaryNavItems.map((item) => (
              <button
                key={item.key}
                      onClick={() => setActivePage(item.key as AppPage)}
                style={{
                  padding:'10px 12px',
                  borderRadius:'10px',
                  border: activePage === item.key ? '1px solid rgba(255,165,40,0.24)' : '1px solid rgba(255,255,255,0.06)',
                  background: activePage === item.key ? 'rgba(200,134,10,0.14)' : 'rgba(255,255,255,0.03)',
                  color: activePage === item.key ? '#fff2c8' : 'rgba(244,232,200,0.82)',
                  display:'flex',
                  alignItems:'center',
                  justifyContent:'space-between',
                  gap:'10px',
                  cursor:'pointer',
                  fontWeight:700,
                  fontSize:'12px',
                  textAlign:'left',
                }}
              >
                <span>{item.label}</span>
                {item.badge ? (
                  <span style={{
                    minWidth:'18px',
                    padding:'1px 6px',
                    borderRadius:'999px',
                    background:'rgba(255,215,0,0.18)',
                    border:'1px solid rgba(255,215,0,0.22)',
                    color:'#fff3cf',
                    fontSize:'11px',
                    fontWeight:800,
                    lineHeight:1.4,
                  }}>{item.badge}</span>
                ) : null}
              </button>
            ))}
          </div>
        ) : null}
      </nav>

      {visibleSocialAlert ? (
        <div style={{
          display:'flex',
          alignItems:'center',
          justifyContent:'space-between',
          gap:'16px',
          padding:'12px 22px',
          background: visibleSocialAlert.action === 'match'
            ? 'linear-gradient(90deg, rgba(22,64,40,0.92) 0%, rgba(16,42,32,0.95) 100%)'
            : 'linear-gradient(90deg, rgba(70,44,12,0.92) 0%, rgba(32,22,10,0.95) 100%)',
          borderBottom:'1px solid rgba(255,180,60,0.18)',
          boxShadow:'0 8px 24px rgba(0,0,0,0.18)',
          position:'relative',
          zIndex:90,
        }}>
          <div style={{ display:'grid', gap:'4px', minWidth:0 }}>
            <div style={{ color:'#fff5d6', fontSize:'15px', fontWeight:800, whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>
              {visibleSocialAlert.title}
            </div>
            <div style={{ color:'rgba(255,236,194,0.78)', fontSize:'12px', lineHeight:1.5 }}>
              {visibleSocialAlert.detail}
            </div>
          </div>
          <div style={{ display:'flex', gap:'10px', flexWrap:'wrap', flexShrink:0 }}>
            <button
              onClick={handleSocialAlertAction}
              style={{
                padding:'9px 14px',
                borderRadius:'10px',
                border:'1px solid rgba(255,215,120,0.26)',
                background:'rgba(255,255,255,0.08)',
                color:'#fff8de',
                fontSize:'12px',
                fontWeight:800,
                cursor:'pointer',
              }}
            >
              {visibleSocialAlert.actionLabel}
            </button>
            <button
              onClick={dismissSocialAlert}
              style={{
                padding:'9px 14px',
                borderRadius:'10px',
                border:'1px solid rgba(255,255,255,0.10)',
                background:'rgba(255,255,255,0.03)',
                color:'rgba(255,236,194,0.82)',
                fontSize:'12px',
                fontWeight:700,
                cursor:'pointer',
              }}
            >
              Dismiss
            </button>
          </div>
        </div>
      ) : null}

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
      <div style={{ display: 'none' }}>{children}</div>
      {showPlayHub ? (
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
        />
      ) : activePage === 'History' ? (
        <HistoryPage
          focusMatchId={historyFocusMatchId}
          focusGuestId={historyFocusGuestId}
          onSelectMatchId={setHistoryFocusMatchId}
          onOpenGuest={(guestId) => {
            setCommunityFocusGuestId(guestId);
            setActivePage('Community');
          }}
          onClearGuestFocus={() => setHistoryFocusGuestId(null)}
          onWatchLiveMatch={openLiveMatch}
        />
      ) : activePage === 'Friends' ? (
        <FriendsPage
          identity={{
            guestId: readStoredGuestIdentity('white').guestId,
            sessionSecret: readStoredGuestIdentity('white').sessionSecret,
            sessionToken: readStoredGuestIdentity('white').sessionToken,
            accountId: primaryAccountIdentity.accountId,
            accountSessionToken: primaryAccountIdentity.sessionToken,
          }}
          accountId={primaryAccountIdentity.accountId ?? null}
          sessionToken={primaryAccountIdentity.sessionToken ?? null}
          liveRefreshToken={socialLiveToken}
          onOpenProfile={openProfileHandle}
          onOpenAccount={() => setActivePage('Account')}
        />
      ) : activePage === 'Inbox' ? (
        <InboxPage
          accountId={primaryAccountIdentity.accountId ?? null}
          sessionToken={primaryAccountIdentity.sessionToken ?? null}
          liveRefreshToken={socialLiveToken}
          onOpenProfile={openProfileHandle}
          onOpenFriends={() => setActivePage('Friends')}
          onUnreadCountChange={setInboxUnreadCount}
        />
      ) : activePage === 'Profiles' ? (
        <ProfilesPage
          focusHandle={profileFocusHandle}
          viewerHandle={null}
          accountId={primaryAccountIdentity.accountId ?? null}
          sessionToken={primaryAccountIdentity.sessionToken ?? null}
          onSelectHandle={openProfileHandle}
          onOpenAccount={() => setActivePage('Account')}
          onOpenReplay={openReplayMatch}
        />
      ) : activePage === 'Watch' ? (
        <WatchPage
          onWatchMatch={openLiveMatch}
          onOpenReplay={openReplayMatch}
        />
      ) : activePage === 'Cards' ? (
        <CardsPage embedded onNavigate={(page: string) => setActivePage(page as AppPage)} />
      ) : activePage === 'Rankings' ? (
        <RankingsPage
          onViewGuest={(guestId) => {
            setCommunityFocusGuestId(guestId);
            setActivePage('Community');
          }}
          onViewAccount={openProfileHandle}
        />
      ) : activePage === 'Community' ? (
        <CommunityPage
          whiteProfile={whiteProfile}
          blackProfile={blackProfile}
          focusGuestId={communityFocusGuestId}
          onOpenAccount={openProfileHandle}
          onOpenMatch={openReplayMatch}
          onOpenGuestHistory={openGuestHistory}
        />
      ) : activePage === 'Admin' ? (
        <AdminModerationPage
          accountId={primaryAccountIdentity.accountId ?? null}
          sessionToken={primaryAccountIdentity.sessionToken ?? null}
          onOpenProfile={openProfileHandle}
        />
      ) : activePage === 'Status' ? (
        <StatusPage />
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
      ) : showBoardSurface ? (
      <div className="match-layout">

        {/* ── Left column ── */}
        <div className="match-layout__left">
          {renderPlayerCard(topSeat)}
          {false && (
          <div style={{
            background: topSeat === 'white' ? 'rgba(8,45,18,0.50)' : 'rgba(40,10,80,0.50)',
            backdropFilter:'blur(16px)', WebkitBackdropFilter:'blur(16px)',
            border: topSeat === 'white' ? '1px solid rgba(60,220,110,0.45)' : '1px solid rgba(200,120,255,0.45)',
            borderRadius:'16px', padding:'12px 16px',
            display:'flex', alignItems:'center', gap:'12px',
            boxShadow: topSeat === 'white'
              ? '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(80,240,130,0.2), 0 0 30px rgba(30,180,70,0.2)'
              : '0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(220,140,255,0.2), 0 0 30px rgba(160,60,240,0.2)',
          }}>
            <div style={{ width:'58px', height:'58px', borderRadius:'50%', flexShrink:0, background:'linear-gradient(135deg, #1a0a30, #0d0520)', border:'2px solid rgba(150,100,220,0.7)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'28px', boxShadow:'0 0 20px rgba(150,100,220,0.5)' }}>🕵️</div>
            <div style={{ flex:1, minWidth:0 }}>
              <div style={{ display:'flex', alignItems:'center', gap:'8px' }}>
                <div style={{ color: topSeat === 'white' ? '#d0fce8' : '#e8d8ff', fontWeight:700, fontSize:'16px', letterSpacing:'0.3px' }}>{topPlayerName}</div>
                {topSeatBadge && (
                  <span style={{ padding:'2px 7px', borderRadius:'999px', background: topSeatBadge === 'You' ? (topSeat === 'white' ? 'rgba(74,222,128,0.16)' : 'rgba(96,165,250,0.18)') : 'rgba(255,255,255,0.06)', border: topSeatBadge === 'You' ? (topSeat === 'white' ? '1px solid rgba(74,222,128,0.32)' : '1px solid rgba(96,165,250,0.35)') : '1px solid rgba(255,255,255,0.10)', color: topSeatBadge === 'You' ? (topSeat === 'white' ? '#86efac' : '#93c5fd') : 'rgba(255,255,255,0.6)', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px' }}>
                    {topSeatBadge}
                  </span>
                )}
              </div>
              <div style={{ display:'flex', alignItems:'center', gap:'10px', marginTop:'5px' }}>
                <span style={{ color:'#b088f0', fontSize:'12px', fontWeight:600 }}>♟ {displayedBlackRating}</span>
                <span style={{ color: timeB <= 30 ? '#ff5555' : '#f0a030', fontSize:'13px', fontFamily:'monospace', fontWeight:700, background: tickingState==='black'&&clockActive&&!over ? 'rgba(240,160,48,0.18)' : 'rgba(0,0,0,0.3)', padding:'2px 8px', borderRadius:'5px', border:'1px solid rgba(240,160,48,0.2)' }}>⏱ {fmtClock(timeB)}</span>
              </div>
            </div>
            <div style={{ width:'10px', height:'10px', borderRadius:'50%', background:'#2ecc71', boxShadow:'0 0 12px #2ecc71', flexShrink:0 }} />
          </div>
          )}

          {/* Card preview panel */}
          <div style={{
            border:'1px solid rgba(180,130,60,0.4)',
            borderRadius:'14px', overflow:'hidden',
            background: selectedCard ? selectedCard.color : 'rgba(8,4,20,0.88)',
            backdropFilter:'blur(20px)', WebkitBackdropFilter:'blur(20px)',
            boxShadow:'0 4px 30px rgba(0,0,0,0.8)',
            flex:1, minHeight:0, display:'flex', flexDirection:'column',
          }}>
            {cardPending ? (
              <div style={{ flex:1, display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', gap:'12px', padding:'16px' }}>
                <div style={{ fontSize:'32px', animation: (cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'sacrificePulse 1.2s ease-in-out infinite' : cardPending.mechanic === 'mindcontrol' ? 'mindControlPulse 1.5s ease-in-out infinite' : (cardPending.mechanic === 'halffuse' || cardPending.mechanic === 'fullfusion') ? 'fusePulse 1.0s ease-in-out infinite' : 'none' }}>{cardPending.card.icon}</div>
                <div style={{ color:'#fff', fontWeight:700, fontSize:'13px', textAlign:'center' }}>{cardPending.card.name}</div>
                {cardPending.mechanic === 'mindcontrol' && (
                  <div style={{ width:'100%' }}>
                    <div style={{ padding:'8px 10px', background:'rgba(139,0,255,0.12)', border:'1px solid rgba(168,85,247,0.4)', borderRadius:'8px', marginBottom:'6px' }}>
                      <div style={{ fontSize:'9px', fontWeight:700, color:'rgba(200,100,255,0.9)', textTransform:'uppercase', letterSpacing:'1px', marginBottom:'4px' }}>⚡ Psychic Scan Active</div>
                      <div style={{ fontSize:'10px', color:'rgba(180,140,255,0.8)', lineHeight:'1.5' }}>All enemy pieces are highlighted. Click any non-king enemy piece to permanently take control of it.</div>
                    </div>
                    <div style={{ display:'flex', gap:'6px', justifyContent:'center', flexWrap:'wrap' }}>
                      {(['queen','rook','bishop','knight','pawn'] as const).map(pt => {
                        const oppColor = OPP[cardPending.playerColor];
                        const hasPiece = board.some(r => r.some(p => p?.type === pt && p.color === oppColor));
                        return hasPiece ? (
                          <span key={pt} style={{ padding:'2px 7px', background:'rgba(139,0,255,0.18)', border:'1px solid rgba(168,85,247,0.4)', borderRadius:'4px', fontSize:'9px', color:'#d8b4fe', fontWeight:700, textTransform:'capitalize' }}>
                            🎯 {pt}
                          </span>
                        ) : null;
                      })}
                    </div>
                  </div>
                )}
                {(cardPending.mechanic === 'halffuse' || cardPending.mechanic === 'fullfusion') && (() => {
                  const type1 = cardPending.data?.type1 as PieceType | undefined;
                  const val1  = cardPending.data?.val1 as number | undefined;
                  const isHalf = cardPending.mechanic === 'halffuse';
                  const HALF_CAP = 6;
                  const accentColor = isHalf ? '#fbbf24' : '#a78bfa';
                  const accentRgb   = isHalf ? '251,191,36' : '167,139,250';
                  return (
                    <div style={{ width:'100%' }}>
                      <div style={{ padding:'8px 10px', background:`rgba(${accentRgb},0.10)`, border:`1px solid rgba(${accentRgb},0.4)`, borderRadius:'8px', marginBottom:'6px' }}>
                        <div style={{ fontSize:'9px', fontWeight:700, color:`rgba(${accentRgb},0.9)`, textTransform:'uppercase', letterSpacing:'1px', marginBottom:'4px' }}>
                          {isHalf ? '⚗️ Half Fuse' : '🔮 Full Fusion'} — Step {cardPending.step}/2
                        </div>
                        {cardPending.step === 1 && (
                          <div style={{ fontSize:'10px', color:`rgba(${accentRgb},0.8)`, lineHeight:'1.5' }}>
                            {isHalf
                              ? `Pick a piece to sacrifice. Combined value with absorber must be ≤${HALF_CAP}pts. Must be adjacent.`
                              : 'Pick any piece to sacrifice. No value cap, no distance restriction.'}
                          </div>
                        )}
                        {cardPending.step === 2 && type1 && val1 !== undefined && (
                          <div>
                            <div style={{ display:'flex', alignItems:'center', gap:'6px', marginBottom:'6px' }}>
                              <span style={{ padding:'2px 8px', background:`rgba(${accentRgb},0.2)`, border:`1px solid rgba(${accentRgb},0.5)`, borderRadius:'4px', fontSize:'10px', color: accentColor, fontWeight:700, textTransform:'capitalize' }}>
                                {'💀 '}{type1}{' ('}{val1}{'pt)'}
                              </span>
                              <span style={{ color:`rgba(${accentRgb},0.6)`, fontSize:'12px' }}>{'→'}</span>
                              <span style={{ padding:'2px 8px', background:'rgba(255,255,255,0.07)', border:'1px solid rgba(255,255,255,0.2)', borderRadius:'4px', fontSize:'10px', color:'#cbd5e1', fontWeight:700 }}>
                                {'? absorber'}
                              </span>
                            </div>
                            {isHalf && (
                              <div style={{ marginBottom:'4px' }}>
                                <div style={{ display:'flex', justifyContent:'space-between', marginBottom:'3px' }}>
                                  <span style={{ fontSize:'9px', color:'#fbbf24', fontWeight:700 }}>{'Value budget'}</span>
                                  <span style={{ fontSize:'9px', fontWeight:800, color:'#fbbf24' }}>{val1}{' / '}{HALF_CAP}{' pts'}</span>
                                </div>
                                <div style={{ height:'5px', background:'rgba(0,0,0,0.4)', borderRadius:'3px', overflow:'hidden', border:'1px solid rgba(251,191,36,0.3)' }}>
                                  <div style={{ height:'100%', borderRadius:'3px', width:`${Math.min(100,(val1/HALF_CAP)*100)}%`, background:'linear-gradient(90deg,#92400e,#fbbf24)' }} />
                                </div>
                              </div>
                            )}
                            <div style={{ fontSize:'9px', color:`rgba(${accentRgb},0.7)`, marginTop:'4px' }}>
                              {isHalf ? 'Adjacent only. Red-tinted squares exceed the 6pt cap.' : 'Pick any piece — it absorbs the sacrifice and gains both movement types.'}
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })()}
                {(cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') && (() => {
                  const selected = (cardPending.data?.selected as Sq[] | undefined) ?? [];
                  const totalVal = selected.reduce((sum, sq) => sum + PIECE_VALUE[board[sq.row][sq.col]?.type ?? 'pawn'], 0);
                  const goal = cardPending.mechanic === 'smallsacrifice' ? 6 : 14;
                  const pct = Math.min(100, (totalVal / goal) * 100);
                  const ready = totalVal >= goal;
                  return (
                    <div style={{ width:'100%' }}>
                      <div style={{ display:'flex', justifyContent:'space-between', marginBottom:'5px' }}>
                        <span style={{ fontSize:'10px', color: ready ? '#ef4444' : '#a0b8d8', fontWeight:700 }}>Blood Price</span>
                        <span style={{ fontSize:'11px', fontWeight:800, color: ready ? '#ef4444' : '#a0b8d8' }}>{totalVal} / {goal} pts</span>
                      </div>
                      <div style={{ height:'8px', background:'rgba(0,0,0,0.5)', borderRadius:'4px', overflow:'hidden', border:'1px solid rgba(220,20,20,0.3)' }}>
                        <div style={{
                          height:'100%', borderRadius:'4px',
                          width:`${pct}%`,
                          background: ready
                            ? 'linear-gradient(90deg, #dc1414, #ff4444)'
                            : 'linear-gradient(90deg, #7f1d1d, #dc2626)',
                          boxShadow: ready ? '0 0 8px rgba(220,20,20,0.9)' : 'none',
                          transition:'width 0.3s ease, background 0.3s ease',
                        }} />
                      </div>
                      {selected.length > 0 && (
                        <div style={{ marginTop:'6px', display:'flex', flexWrap:'wrap', gap:'3px', justifyContent:'center' }}>
                          {selected.map((sq, i) => {
                            const p = board[sq.row][sq.col];
                            return p ? (
                              <span key={i} style={{ padding:'2px 6px', background:'rgba(220,20,20,0.25)', border:'1px solid rgba(220,20,20,0.5)', borderRadius:'4px', fontSize:'9px', color:'#fca5a5', fontWeight:700, textTransform:'capitalize' }}>
                                🩸 {p.type} ({PIECE_VALUE[p.type]}pt)
                              </span>
                            ) : null;
                          })}
                        </div>
                      )}
                      {ready && (
                        <div style={{ marginTop:'8px', padding:'5px 10px', background:'rgba(220,20,20,0.2)', border:'1px solid rgba(220,20,20,0.5)', borderRadius:'6px', fontSize:'10px', color:'#fca5a5', textAlign:'center', fontWeight:700 }}>
                          ✅ Click empty square to confirm sacrifice
                        </div>
                      )}
                    </div>
                  );
                })()}
                <div style={{ color:'#a0b8d8', fontSize:'11px', textAlign:'center', lineHeight:1.5, padding:'8px 10px', background: (cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'rgba(220,20,20,0.08)' : cardPending.mechanic === 'mindcontrol' ? 'rgba(139,0,255,0.08)' : (cardPending.mechanic === 'halffuse') ? 'rgba(251,191,36,0.08)' : (cardPending.mechanic === 'fullfusion') ? 'rgba(167,139,250,0.08)' : 'rgba(74,144,210,0.1)', border:`1px solid ${(cardPending.mechanic === 'smallsacrifice' || cardPending.mechanic === 'bigsacrifice') ? 'rgba(220,20,20,0.3)' : cardPending.mechanic === 'mindcontrol' ? 'rgba(139,0,255,0.35)' : (cardPending.mechanic === 'halffuse') ? 'rgba(251,191,36,0.35)' : (cardPending.mechanic === 'fullfusion') ? 'rgba(167,139,250,0.35)' : 'rgba(74,144,210,0.3)'}`, borderRadius:'8px' }}>{cardMsg || 'Click a square on the board...'}</div>
                <button onClick={cancelCard} style={{ padding:'7px 18px', background:'rgba(231,76,60,0.2)', color:'#e74c3c', border:'1px solid rgba(231,76,60,0.4)', borderRadius:'6px', cursor:'pointer', fontSize:'12px', fontWeight:700 }}>✕ Cancel</button>
                {promoPicker && (
                  <div style={{ display:'flex', gap:'6px', flexWrap:'wrap', justifyContent:'center', marginTop:'4px' }}>
                    {promoPicker.options.map(t => (
                      <button key={t} onClick={() => handlePromoPick(t)}
                        style={{ padding:'6px 10px', background:'rgba(245,158,11,0.2)', color:'#f59e0b', border:'1px solid rgba(245,158,11,0.5)', borderRadius:'6px', cursor:'pointer', fontSize:'11px', fontWeight:700, textTransform:'capitalize' }}>
                        {t}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            ) : selectedCard ? (() => {
              const ownerColor: PieceColor = whiteHand.some(c => c.id === selectedCard.id) ? 'white' : 'black';
              const isViewerOwner = (hostedRuntime || authoritativeMatchId) ? viewerSeat === ownerColor : true;
              const canUse = canUseCard(selectedCard, ownerColor) && isViewerOwner;
              const usedThisTurn = cardUsedBy[ownerColor];
              let blockReason = '';
              if (over)           blockReason = 'Game is over';
              else if (!isViewerOwner) blockReason = "Not your card to use";
              else if (usedThisTurn) blockReason = 'Already used a card this turn';
              else if (selectedCard.type !== 'trap' && turn !== ownerColor) blockReason = `Only usable on ${ownerColor}'s turn`;
              return (
                <div style={{ display:'flex', flexDirection:'column', background: selectedCard.color, animation:'cardReveal 0.22s cubic-bezier(0.34,1.56,0.64,1)', flex:1, overflow:'hidden' }}>
                  <div style={{ padding:'10px 14px 8px', display:'flex', justifyContent:'space-between', alignItems:'center', borderBottom:`1px solid ${selectedCard.accent}55` }}>
                    <div>
                      <div style={{ color:'#fff', fontWeight:800, fontSize:'14px', textShadow:'0 1px 6px rgba(0,0,0,0.9)' }}>{selectedCard.name}</div>
                      <div style={{ marginTop:'3px', display:'inline-block', padding:'2px 8px', borderRadius:'4px', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px', color: RARITY_STYLE[selectedCard.rarity].accent, background:`${RARITY_STYLE[selectedCard.rarity].accent}33`, border:`1px solid ${RARITY_STYLE[selectedCard.rarity].accent}88` }}>
                        {RARITY_STYLE[selectedCard.rarity].label} · {RARITY_WEIGHTS[selectedCard.rarity]}% drop
                      </div>
                    </div>
                    <div style={{ width:'26px', height:'26px', borderRadius:'6px', background:`${selectedCard.accent}44`, border:`1px solid ${selectedCard.accent}88`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'14px' }}>地</div>
                  </div>
                  <div style={{ height:'150px', margin:'10px 12px', borderRadius:'10px', background:`radial-gradient(ellipse at 50% 40%, ${selectedCard.accent}66 0%, rgba(0,0,0,0.8) 70%)`, border:`2px solid ${selectedCard.accent}55`, display:'flex', alignItems:'center', justifyContent:'center', fontSize:'68px', position:'relative', overflow:'hidden', boxShadow:`0 0 24px ${selectedCard.accent}33` }}>
                    <div style={{ animation: selectedCard.mechanic === 'joker' ? 'jokerFloat 2s ease-in-out infinite' : 'none', filter:`drop-shadow(0 0 10px ${selectedCard.accent})` }}>
                      {selectedCard.icon}
                    </div>
                    {selectedCard.mechanic === 'joker' && (
                      <>
                        {[0,1,2,3,4].map(j => (
                          <div key={j} style={{
                            position:'absolute',
                            top:`${10+j*18}%`, left:`${5+j*22}%`,
                            fontSize:'10px',
                            animation:`jokerGlitter ${1+j*0.3}s ease-in-out infinite`,
                            animationDelay:`${j*0.2}s`,
                            pointerEvents:'none',
                          }}>✦</div>
                        ))}
                      </>
                    )}
                  </div>
                  <div style={{ margin:'0 14px 8px', padding:'4px 10px', background:`${selectedCard.accent}28`, border:`1px solid ${selectedCard.accent}55`, borderRadius:'5px', fontSize:'10px', color:selectedCard.accent, fontWeight:700, textTransform:'uppercase', letterSpacing:'1px', display:'inline-flex', alignSelf:'flex-start' }}>[{selectedCard.type === 'spell' ? 'Spell Card' : 'Trap Card'}]</div>
                  <div style={{ margin:'0 14px 12px', fontSize:'11px', color:'rgba(235,225,210,0.95)', lineHeight:'1.65', fontWeight:500 }}>{selectedCard.desc}</div>
                  {blockReason && <div style={{ margin:'0 14px 8px', padding:'7px 10px', background:'rgba(200,40,40,0.2)', border:'1px solid rgba(220,60,60,0.5)', borderRadius:'6px', fontSize:'10px', color:'#ff8080', fontWeight:600, textAlign:'center' }}>🔒 {blockReason}</div>}
                  <div style={{ flex:1 }} />
                  <div style={{ padding:'4px 14px 16px' }}>
                    <button onClick={() => applyCard(selectedCard, ownerColor)} disabled={!canUse}
                      style={{ width:'100%', padding:'11px', borderRadius:'22px', border:'none', background: canUse ? (selectedCard.mechanic === 'joker' ? 'linear-gradient(135deg, #f59e0b, #b45309)' : 'linear-gradient(135deg, #3b9edd, #1a5fa8)') : 'rgba(40,40,60,0.8)', color: canUse ? '#fff' : 'rgba(255,255,255,0.25)', fontWeight:700, fontSize:'13px', cursor: canUse ? 'pointer' : 'not-allowed', boxShadow: canUse ? (selectedCard.mechanic === 'joker' ? '0 4px 16px rgba(245,158,11,0.55)' : '0 4px 16px rgba(26,111,196,0.55)') : 'none', letterSpacing:'0.3px' }}>
                      {canUse ? (selectedCard.mechanic === 'joker' ? '🃏 Choose Transformation' : 'use card') : '🔒 blocked'}
                    </button>
                  </div>
                </div>
              );
            })() : (
              <div style={{ flex:1, display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', gap:'10px' }}>
                <div style={{ fontSize:'40px', opacity:0.15 }}>🃏</div>
                <div style={{ color:'rgba(180,150,100,0.4)', fontSize:'12px', letterSpacing:'0.5px' }}>Click a card to preview</div>
                {cardMsg && <div style={{ color:'#f59e0b', fontSize:'11px', textAlign:'center', padding:'6px 10px', background:'rgba(245,158,11,0.1)', border:'1px solid rgba(245,158,11,0.3)', borderRadius:'6px', maxWidth:'180px' }}>{cardMsg}</div>}
                {doubleMove && (
                  <div style={{ padding:'8px 12px', background: doubleMove.type === 'same' ? 'rgba(74,222,128,0.12)' : 'rgba(96,165,250,0.12)', border:`1px solid ${doubleMove.type === 'same' ? 'rgba(74,222,128,0.5)' : 'rgba(96,165,250,0.5)'}`, borderRadius:'8px', fontSize:'10px', color: doubleMove.type === 'same' ? '#4ade80' : '#60a5fa', fontWeight:700, textAlign:'center', maxWidth:'180px', animation:'pulse 1.5s ease infinite' }}>
                    {doubleMove.type === 'same' ? '🏃 SOLO' : '👥 TWIN'} — Move {3 - doubleMove.movesLeft}/2
                    {doubleMove.trackedSq && doubleMove.type === 'same' && <div style={{ fontSize:'9px', marginTop:'3px', opacity:0.8 }}>Move piece at {FILES[doubleMove.trackedSq.col]}{RANKS[doubleMove.trackedSq.row]}</div>}
                    {doubleMove.trackedSq && doubleMove.type === 'diff' && <div style={{ fontSize:'9px', marginTop:'3px', opacity:0.8 }}>Don't move {FILES[doubleMove.trackedSq.col]}{RANKS[doubleMove.trackedSq.row]} again</div>}
                  </div>
                )}
                {/* Bomb status display */}
                {bombPieces.length > 0 && (
                  <div style={{ padding:'8px 12px', background:'rgba(255,80,0,0.12)', border:'1px solid rgba(255,80,0,0.5)', borderRadius:'8px', fontSize:'10px', color:'#ff6030', fontWeight:700, textAlign:'center', maxWidth:'180px', animation:'bombGlow 1s ease-in-out infinite' }}>
                    💣 {bombPieces.length} BOMB{bombPieces.length > 1 ? 'S' : ''} ACTIVE
                    {bombPieces.map((b, i) => (
                      <div key={i} style={{ fontSize:'9px', marginTop:'2px', opacity:0.8 }}>
                        {FILES[b.col]}{RANKS[b.row]} — {b.turnsLeft} turn{b.turnsLeft !== 1 ? 's' : ''} left
                      </div>
                    ))}
                  </div>
                )}
                <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:'4px', marginTop:'8px' }}>
                  {(['white','black'] as PieceColor[]).map(color => (
                    <div key={color} style={{ display:'flex', alignItems:'center', gap:'6px', fontSize:'10px' }}>
                      <span style={{ color: color==='white' ? '#e8eaf0' : '#7ab8f5' }}>{color==='white' ? '⚪' : '⚫'}</span>
                      <span style={{ color: cardUsedBy[color] ? '#e74c3c' : '#2ecc71', fontWeight:600 }}>{cardUsedBy[color] ? '✓ Card used' : '○ Card available'}</span>
                    </div>
                  ))}
                  <div style={{ marginTop:'6px', fontSize:'9px', color:'rgba(160,184,216,0.35)', textAlign:'center' }}>Max {MAX_HAND_SIZE} cards per player</div>
                </div>
              </div>
            )}
          </div>

          {renderPlayerCard(bottomSeat)}
          {false && (
          <div style={{
            background:'rgba(8,45,18,0.50)',
            backdropFilter:'blur(16px)', WebkitBackdropFilter:'blur(16px)',
            border:'1px solid rgba(60,220,110,0.45)',
            borderRadius:'16px', padding:'12px 16px',
            display:'flex', alignItems:'center', gap:'12px',
            boxShadow:'0 8px 32px rgba(0,0,0,0.35), inset 0 1px 0 rgba(80,240,130,0.2), 0 0 30px rgba(30,180,70,0.2)',
          }}>
            <div style={{ width:'58px', height:'58px', borderRadius:'50%', flexShrink:0, background:'linear-gradient(135deg, #0a200f, #051208)', border:'2px solid rgba(46,180,90,0.7)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'28px', boxShadow:'0 0 20px rgba(46,180,90,0.4)' }}>🧑‍💻</div>
            <div style={{ flex:1, minWidth:0 }}>
              <div style={{ display:'flex', alignItems:'center', gap:'8px' }}>
                <div style={{ color:'#d0fce8', fontWeight:700, fontSize:'16px', letterSpacing:'0.3px' }}>{displayedWhiteName}</div>
                {whiteSeatBadge && (
                  <span style={{ padding:'2px 7px', borderRadius:'999px', background:whiteSeatBadge === 'You' ? 'rgba(74,222,128,0.16)' : 'rgba(255,255,255,0.06)', border:whiteSeatBadge === 'You' ? '1px solid rgba(74,222,128,0.32)' : '1px solid rgba(255,255,255,0.10)', color:whiteSeatBadge === 'You' ? '#86efac' : 'rgba(255,255,255,0.6)', fontSize:'9px', fontWeight:800, textTransform:'uppercase', letterSpacing:'0.8px' }}>
                    {whiteSeatBadge}
                  </span>
                )}
              </div>
              <div style={{ display:'flex', alignItems:'center', gap:'10px', marginTop:'5px' }}>
                <span style={{ color:'#52c77a', fontSize:'12px', fontWeight:600 }}>♟ {displayedWhiteRating}</span>
                <span style={{ color: timeW <= 30 ? '#ff5555' : '#f0a030', fontSize:'13px', fontFamily:'monospace', fontWeight:700, background: tickingState==='white'&&clockActive&&!over ? 'rgba(240,160,48,0.18)' : 'rgba(0,0,0,0.3)', padding:'2px 8px', borderRadius:'5px', border:'1px solid rgba(240,160,48,0.2)' }}>⏱ {fmtClock(timeW)}</span>
              </div>
            </div>
            <div style={{ width:'10px', height:'10px', borderRadius:'50%', background:'#2ecc71', boxShadow:'0 0 12px #2ecc71', flexShrink:0 }} />
          </div>
          )}
        </div>

        {/* ── Board column ── */}
      <div className="match-layout__center">

          {authoritativeLive && authoritativeStatus === 'waiting' && authoritativeMatchId ? (
            <div style={{ marginBottom:'8px', padding:'9px 14px', background:'rgba(255,212,135,0.10)', border:'1px solid rgba(255,212,135,0.28)', borderRadius:'8px', color:'#ffd487', fontSize:'11px', fontWeight:700, textAlign:'center' }}>
              Private room is waiting for the second player to open the invite link. This seat is reserved, but the game will only start once both seats are claimed.
            </div>
          ) : null}
          {showHostedSoloBanner && (
            <div style={{ marginBottom:'8px', padding:'8px 14px', background:'rgba(96,165,250,0.10)', border:'1px solid rgba(96,165,250,0.28)', borderRadius:'8px', color:'#93c5fd', fontSize:'11px', fontWeight:700, textAlign:'center' }}>
              Solo board: use the Queue tab to find a real online opponent.
            </div>
          )}
          {cardPending && (
            <div style={{ marginBottom:'6px', padding:'8px 18px', background:'rgba(245,158,11,0.15)', border:'1px solid rgba(245,158,11,0.5)', borderRadius:'8px', color:'#f59e0b', fontSize:'12px', fontWeight:700, textAlign:'center', animation:'pulse 1.5s ease infinite' }}>
              🃏 {cardMsg} &nbsp;<span onClick={cancelCard} style={{ cursor:'pointer', color:'#e74c3c', marginLeft:'8px' }}>✕ cancel</span>
            </div>
          )}
          {doubleMove && !cardPending && (
            <div style={{ marginBottom:'6px', padding:'7px 16px', background: doubleMove.type === 'same' ? 'rgba(74,222,128,0.12)' : 'rgba(96,165,250,0.12)', border:`1px solid ${doubleMove.type === 'same' ? 'rgba(74,222,128,0.5)' : 'rgba(96,165,250,0.5)'}`, borderRadius:'8px', color: doubleMove.type === 'same' ? '#4ade80' : '#60a5fa', fontSize:'12px', fontWeight:700, textAlign:'center', animation:'pulse 1.5s ease infinite' }}>
              {doubleMove.movesLeft === 2
                ? (doubleMove.type === 'same' ? '🏃 Solo: Make your first move!' : '👥 Twin: Make your first move!')
                : doubleMove.type === 'same'
                  ? `🏃 Solo: Now move the SAME piece at ${doubleMove.trackedSq ? FILES[doubleMove.trackedSq.col]+RANKS[doubleMove.trackedSq.row] : '?'} again!`
                  : `👥 Twin: Now move a DIFFERENT piece! (not ${doubleMove.trackedSq ? FILES[doubleMove.trackedSq.col]+RANKS[doubleMove.trackedSq.row] : '?'})`
              }
            </div>
          )}
          {radarActive && (
            <div style={{ marginBottom:'4px', padding:'5px 14px', background:'rgba(96,165,250,0.15)', border:'1px solid rgba(96,165,250,0.5)', borderRadius:'8px', color:'#60a5fa', fontSize:'11px', fontWeight:700, textAlign:'center', display:'flex', alignItems:'center', gap:'8px', justifyContent:'center' }}>
              <span style={{ animation:'radarSweep 1.5s linear infinite', display:'inline-block' }}>📡</span>
              RADAR ACTIVE — Enemy hand revealed!
              <span style={{ animation:'radarPing 1s ease-out infinite', display:'inline-block', width:'8px', height:'8px', borderRadius:'50%', background:'#60a5fa' }} />
            </div>
          )}
          {/* Bomb warning banner */}
          {bombPieces.length > 0 && (
            <div style={{ marginBottom:'4px', padding:'5px 14px', background:'rgba(255,80,0,0.12)', border:'1px solid rgba(255,80,0,0.45)', borderRadius:'8px', color:'#ff7040', fontSize:'11px', fontWeight:700, textAlign:'center', display:'flex', alignItems:'center', gap:'8px', justifyContent:'center', animation:'bombGlow 1s ease-in-out infinite' }}>
              <span style={{ animation:'bombTick 0.8s ease-in-out infinite' }}>💣</span>
              BOMB{bombPieces.length > 1 ? 'S' : ''} ACTIVE — {bombPieces.map(b => `${FILES[b.col]}${RANKS[b.row]}(${b.turnsLeft}t)`).join(', ')}
            </div>
          )}

          {renderHand(topHand, topSeat, 'top')}

          <div style={{ display:'flex', alignItems:'flex-start' }}>
            <div style={{ position:'relative', border:'2px solid rgba(220,160,40,0.8)', borderRadius:'4px', display:'inline-block', boxShadow:'0 0 0 1px rgba(255,200,60,0.2), 0 0 60px rgba(200,100,10,0.5), 0 0 120px rgba(180,60,0,0.25)' }}>
              <BoardCanvas
                reverseAnim={null}
                board={board}
                turn={turn}
                sel={sel}
                hints={hints}
                lm={lm}
                drag={drag}
                dragPos={dragPos}
                check={check}
                kingPos={kingPos}
                cardHighlight={getCardHighlight}
                doubleMoveHighlight={getDoubleMoveHighlight}
                bombPieces={bombPieces}
                bombExploding={bombExploding}
                lavaSquares={lavaSquares}
                lavaExploding={lavaExploding}
                swapAnim={swapAnim}
                isReviewing={isReviewing}
                reviewBoard={reviewBoard}
                cardPending={cardPending}
                onClick={clickSq}
                onDragStart={(e, r, c) => {
                  if (cardPending || isReviewing || over || promo || (hostedRuntime && authoritativeStatus !== 'active')) return;
                  const p = board[r][c];
                  const ghostDs = ghostRef.current;
                  const actingColor = (hostedRuntime || authoritativeMatchId) ? viewerSeat : turn;
                  const isGhostDs = ghostDs && actingColor && ghostDs.ownerColor === actingColor && turn === actingColor && ghostDs.row === r && ghostDs.col === c;
                  if (!actingColor) return;
                  if (!isGhostDs && (!p || p.color !== actingColor || turn !== actingColor)) return;
                  setDrag({ row: r, col: c });
                  setSel({ row: r, col: c });
                  setHints(getMoves(r, c));
                  const rect = (e.target as HTMLElement).getBoundingClientRect();
                  setDragPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
                }}
                onDrop={(r, c) => {
                  if (!drag || isReviewing || cardPending) { setDrag(null); setDragPos(null); setSel(null); setHints([]); return; }
                  const mv = getMoves(drag.row, drag.col);
                  if (mv.some(m => m.row === r && m.col === c)) doMove(drag.row, drag.col, r, c);
                  setDrag(null); setDragPos(null); setSel(null); setHints([]);
                }}
                doubleMove={doubleMove}
                transformAnim={transformAnim}
                sniperAnim={sniperAnim}
                teleportAnim={teleportAnim}
                jumpAnim={jumpAnim}
                sacrificeAnim={sacrificeAnim}
                sacrificeSelectedSquares={
                  cardPending?.mechanic === 'smallsacrifice' || cardPending?.mechanic === 'bigsacrifice'
                    ? ((cardPending.data?.selected as Sq[] | undefined) ?? [])
                    : []
                }
                mindControlAnim={mindControlAnim}
                mindControlTargetSquare={null}
                fuseAnim={fuseAnim}
                fuseSelectedSq={
                  (cardPending?.mechanic === 'halffuse' || cardPending?.mechanic === 'fullfusion') && cardPending.step === 2
                    ? (cardPending.data?.sq1 as { row: number; col: number } | undefined) ?? null
                    : null
                }
                fogZones={fogZones}
                viewerColor={hostedRuntime ? viewerSeat : turn}
                invisibleUnder={ghostPiece}
                analysisArrows={analysisArrows}
                onToggleAnalysisArrow={toggleAnalysisArrow}
                onClearAnalysisArrows={clearAnalysisArrows}
              />

              {/* Promotion overlay */}
              {promo && (() => {
                const order: PieceType[] = promo.color === 'white'
                  ? ['queen','knight','rook','bishop']
                  : ['bishop','rook','knight','queen'];
                const left = promo.col * SQ + 4;
                const top  = promo.color === 'white' ? 4 : 4 * SQ + 4;
                return (
                  <>
                    <div style={{ position:'absolute', inset:0, background:'rgba(0,0,0,0.45)', zIndex:50 }} />
                    <div style={{ position:'absolute', left:`${left}px`, top:`${top}px`, zIndex:51, display:'flex', flexDirection:'column' }}>
                      {order.map(t => (
                        <div key={t} onClick={() => doPromo(t)}
                          style={{ width:`${SQ}px`, height:`${SQ}px`, background:'#c8c8c8', display:'flex', alignItems:'center', justifyContent:'center', cursor:'pointer', boxShadow:'0 2px 8px rgba(0,0,0,0.6)', userSelect:'none' }}
                          onMouseEnter={e => { (e.currentTarget as HTMLDivElement).style.background = '#e8a020'; }}
                          onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = '#c8c8c8'; }}
                        >
                          <div style={{ width:'50px', height:'50px', borderRadius:'50%', background:'rgba(255,255,255,0.18)', display:'flex', alignItems:'center', justifyContent:'center' }}>
                            <img src={`/pieces/${promo.color}_${t}.svg`} alt={`${promo.color} ${t}`} style={{ width:'40px', height:'40px', objectFit:'contain', pointerEvents:'none' }} draggable={false} />
                          </div>
                        </div>
                      ))}
                    </div>
                  </>
                );
              })()}
              {cardPromo && (() => {
                const order: PieceType[] = cardPromo.color === 'white'
                  ? ['queen','knight','rook','bishop']
                  : ['bishop','rook','knight','queen'];
                const left = cardPromo.sq.col * SQ + 4;
                const top  = cardPromo.color === 'white' ? 4 : 4 * SQ + 4;
                return (
                  <>
                    <div style={{ position:'absolute', inset:0, background:'rgba(0,0,0,0.45)', zIndex:50 }} />
                    <div style={{ position:'absolute', left:`${left}px`, top:`${top}px`, zIndex:51, display:'flex', flexDirection:'column' }}>
                      {order.map(t => (
                        <div key={t} onClick={() => {
                          const nb = board.map(r => r.map(p => p ? { ...p } : null));
                          nb[cardPromo.sq.row][cardPromo.sq.col] = { type: t, color: cardPromo.color };
                          const myKp = findKing(nb, OPP[turn]);
                          if (myKp && isAttackedWithFusion(nb, myKp.row, myKp.col, turn)) {
                            setCardMsg('❌ That promotion would leave your king in check!');
                            setTimeout(() => setCardMsg(''), 2000);
                            return;
                          }
                          setBoard(nb);
                          setCardPromo(null);
                          const posKey = positionKey(nb, turn, moved, lm);
                          const newPh = [...posHist, posKey];
                          setPosHist(newPh);
                          checkEndGame(nb, turn, moved, lm, hmc, newPh, posKey, toFEN(nb, turn, moved, lm, hmc, fmn), OPP[turn]);
                        }}
                          style={{ width:`${SQ}px`, height:`${SQ}px`, background:'#c8c8c8', display:'flex', alignItems:'center', justifyContent:'center', cursor:'pointer', boxShadow:'0 2px 8px rgba(0,0,0,0.6)', userSelect:'none' }}
                          onMouseEnter={e => { (e.currentTarget as HTMLDivElement).style.background = '#e8a020'; }}
                          onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = '#c8c8c8'; }}
                        >
                          <div style={{ width:'50px', height:'50px', borderRadius:'50%', background:'rgba(255,255,255,0.18)', display:'flex', alignItems:'center', justifyContent:'center' }}>
                            <img src={`/pieces/${cardPromo.color}_${t}.svg`} alt={`${cardPromo.color} ${t}`} style={{ width:'40px', height:'40px', objectFit:'contain', pointerEvents:'none' }} draggable={false} />
                          </div>
                        </div>
                      ))}
                    </div>
                  </>
                );
              })()}
            </div>
          </div>

          {renderHand(bottomHand, bottomSeat, 'bottom')}
        </div>

        {/* ── Right panel ── */}
        <div style={{
          flex:1, minWidth:0,
          background:'rgba(20,8,45,0.45)',
          backdropFilter:'blur(24px)', WebkitBackdropFilter:'blur(24px)',
          border:'1px solid rgba(255,165,40,0.3)',
          borderRadius:'16px', padding:'12px',
          display:'flex', flexDirection:'column', gap:'8px',
          boxShadow:'0 8px 40px rgba(0,0,0,0.35), inset 0 1px 0 rgba(255,165,40,0.12), 0 0 40px rgba(200,80,10,0.12)',
          overflow:'auto', margin:'10px 0',
        }}>

          {/* Round + rarity panel */}
          <div style={{
            display:'flex', flexDirection:'column', gap:'6px',
            padding:'10px 14px',
            background:'rgba(255,140,0,0.06)',
            border:'1px solid rgba(255,165,40,0.18)',
            borderRadius:'12px', flexShrink:0,
          }}>
            <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', gap:'8px' }}>
              <div style={{ color: authoritativeLive ? '#4ade80' : (hostedRuntime ? '#f59e0b' : 'rgba(160,184,216,0.55)'), fontSize:'9px', fontWeight:700, textTransform:'uppercase', letterSpacing:'1px' }}>
                {boardStatusLabel}
              </div>
              <div style={{ color:'rgba(160,184,216,0.4)', fontSize:'8px' }}>
                {authoritativeMatchIdRef.current ? `match ${authoritativeMatchIdRef.current.slice(-6)}` : 'no match'}
              </div>
            </div>
            {showHostedReconnectWarning && (
              <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', gap:'10px', padding:'7px 9px', borderRadius:'8px', background:'rgba(245,158,11,0.10)', border:'1px solid rgba(245,158,11,0.28)' }}>
                <div style={{ color:'#fcd34d', fontSize:'10px', lineHeight:1.35 }}>
                  Live match sync is reconnecting, so updates may briefly fall back to slower refreshes until the stream is back.
                </div>
                <button
                  onClick={() => { void bootstrapAuthoritativeMatch(); }}
                  style={{ padding:'6px 10px', background:'linear-gradient(180deg,#d97706,#92400e)', color:'#fff', border:'1px solid rgba(251,191,36,0.35)', borderRadius:'7px', cursor:'pointer', fontSize:'10px', fontWeight:800, whiteSpace:'nowrap' }}
                >
                  Retry Sync
                </button>
              </div>
            )}
            <div style={{ display:'flex', alignItems:'center', gap:'10px' }}>
              <div style={{ flex:1 }}>
                <div style={{ color:'#a0b8d8', fontSize:'9px', fontWeight:600, textTransform:'uppercase', letterSpacing:'0.8px' }}>Round</div>
                <div style={{ color: roundNumber >= 7 ? '#f39c12' : '#fff', fontSize:'20px', fontWeight:800, lineHeight:1.1 }}>{roundNumber}</div>
                <div style={{ color:'#4a6080', fontSize:'9px', marginTop:'1px' }}>
                  {roundNumber < INITIAL_DEAL_ROUND ? `cards dealt at r${INITIAL_DEAL_ROUND}`
                  : roundNumber < DRAW_FROM        ? `next draw at r${DRAW_FROM}`
                  : (roundNumber - DRAW_FROM) % DRAW_EVERY === 0 ? '🃏 draw now!'
                  : `next draw r${DRAW_FROM + Math.ceil((roundNumber - DRAW_FROM) / DRAW_EVERY) * DRAW_EVERY}`}
                </div>
              </div>
              <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:'3px' }}>
                <div style={{ width:'30px', height:'30px', borderRadius:'50%', background: turn==='white' ? 'radial-gradient(circle, #fff 0%, #ccc 100%)' : 'radial-gradient(circle, #444 0%, #111 100%)', border: turn==='white' ? '2px solid rgba(255,255,255,0.6)' : '2px solid rgba(74,144,210,0.5)', display:'flex', alignItems:'center', justifyContent:'center', fontSize:'15px' }}>{turn==='white'?'♔':'♚'}</div>
                <div style={{ color:'rgba(200,215,235,0.55)', fontSize:'8px', fontWeight:600 }}>{turn==='white'?'WHITE':'BLACK'}</div>
              </div>
              <div style={{ flex:1, textAlign:'right' }}>
                <div style={{ color:'#a0b8d8', fontSize:'9px', fontWeight:600, textTransform:'uppercase', letterSpacing:'0.8px', marginBottom:'2px' }}>∞ Card Pool</div>
                <div style={{ color:'rgba(160,184,216,0.5)', fontSize:'8px', lineHeight:1.4 }}>Infinite draws<br/>Rarity weighted</div>
              </div>
            </div>
            <div style={{ display:'flex', flexDirection:'column', gap:'3px' }}>
              <div style={{ color:'rgba(160,184,216,0.5)', fontSize:'8px', fontWeight:700, textTransform:'uppercase', letterSpacing:'1px', marginBottom:'1px' }}>Drop Rates</div>
              {(Object.entries(RARITY_WEIGHTS) as [Rarity, number][]).map(([rarity, weight]) => {
                const style = RARITY_STYLE[rarity];
                return (
                  <div key={rarity} style={{ display:'flex', alignItems:'center', gap:'5px' }}>
                    <div style={{ width:'52px', fontSize:'8px', fontWeight:700, color: style.accent, textTransform:'uppercase', letterSpacing:'0.5px' }}>{style.label}</div>
                    <div style={{ flex:1, height:'5px', borderRadius:'3px', background:'rgba(255,255,255,0.06)', overflow:'hidden' }}>
                      <div style={{ height:'100%', width:`${weight}%`, borderRadius:'3px', background:`linear-gradient(90deg, ${style.accent}99, ${style.accent})`, boxShadow:`0 0 4px ${style.glow}` }} />
                    </div>
                    <div style={{ width:'28px', fontSize:'8px', fontWeight:700, color:'rgba(200,215,235,0.5)', textAlign:'right' }}>{weight}%</div>
                  </div>
                );
              })}
            </div>
            {lastDrawAnim && (
              roundNumber === INITIAL_DEAL_ROUND ? (
                <div style={{ textAlign:'center', padding:'4px 8px', borderRadius:'5px', background:'rgba(245,158,11,0.15)', border:'1px solid rgba(245,158,11,0.5)', animation:'pulse 0.5s ease infinite' }}>
                  <span style={{ fontSize:'10px', fontWeight:800, color:'#f59e0b' }}>🃏 Both players received 3 starter cards!</span>
                </div>
              ) : (
                <div style={{ textAlign:'center', padding:'4px 8px', borderRadius:'5px', background:`${RARITY_STYLE[lastDrawAnim.rarity].accent}22`, border:`1px solid ${RARITY_STYLE[lastDrawAnim.rarity].accent}66`, animation:'pulse 0.5s ease infinite' }}>
                  <span style={{ fontSize:'10px', fontWeight:800, color:RARITY_STYLE[lastDrawAnim.rarity].accent }}>🃏 Both players drew a {RARITY_STYLE[lastDrawAnim.rarity].label} card!</span>
                </div>
              )
            )}
          </div>

          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <GamePanel
              chatMessages={chatMessages}
              onSendMessage={(text) => {
                if (authoritativeMatchIdRef.current) {
                  void submitAuthoritativeIntent({ type: 'send_chat', ...authoritativeActorForColor(controlSender), text });
                } else {
                  setChatMessages(prev => [...prev, { sender: controlSender, text }]);
                }
              }}
              isChatDisabled={hostedActionLocked}
              movHist={movHist}
              engineNode={
                engineOn && ev ? (
                  <div style={{ padding: '12px', fontFamily: 'monospace' }}>
                    <div style={{ fontSize: '22px', fontWeight: 'bold', color: ev.score > 0 ? '#2ecc71' : ev.score < 0 ? '#e74c3c' : '#ecf0f1', textAlign: 'center', marginBottom: '8px' }}>
                      {ev.mate != null ? (ev.mate === 0 ? 'Mate' : `M${Math.abs(ev.mate)}`) : (ev.score / 100).toFixed(2)}
                    </div>
                    {ev.best && (
                      <div style={{ color: '#f39c12', textAlign: 'center', fontSize: '13px' }}>
                        Best: {uciToSan(ev.best, reviewIdx >= 0 ? (reviewBoard ?? board) : board)} <span style={{ color: '#7f8c8d', fontSize: '10px' }}>({ev.best})</span>
                      </div>
                    )}
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '8px' }}>
                    <div style={{ color: 'rgba(255,255,255,0.3)', fontSize: '12px' }}>Engine {engineOn ? 'calculating...' : 'off'}</div>
                    {over && (
                      <button onClick={() => setEngineOn(v => !v)} style={{ padding: '6px 14px', fontSize: '11px', background: engineOn ? 'linear-gradient(180deg,#1a6fc4,#0d4a8a)' : 'rgba(60,70,90,0.6)', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold' }}>
                        {engineOn ? 'ENGINE ON' : 'ENGINE OFF'}
                      </button>
                    )}
                  </div>
                )
              }
            />
          </div>
          <div style={{ marginTop:'7px', textAlign:'center', color:'rgba(160,184,216,0.3)', fontSize:'8px' }}>&nbsp;</div>

          {/* Game controls */}
          <div style={{ display:'flex', flexDirection:'column', gap:'6px', flexShrink:0 }}>
            {drawOffer && drawOffer !== turn && canRespondToDrawOffer && (
              <div style={{ background:'rgba(243,156,18,0.12)', border:'1px solid rgba(243,156,18,0.45)', borderRadius:'7px', padding:'8px 10px' }}>
                <div style={{ marginBottom:'6px', fontWeight:'bold', fontSize:'12px', color:'#f39c12' }}>{drawOffer==='white'?'⚪ White':'⚫ Black'} offers a draw</div>
                <div style={{ display:'flex', gap:'6px' }}>
                  <button onClick={() => {
                    if (authoritativeMatchIdRef.current) {
                      void submitAuthoritativeIntent({ type: 'respond_draw', ...authoritativeActorForColor(controlSender), accept: true });
                      return;
                    }
                    finalPositionRef.current = { fen: toFEN(board, turn, moved, lm, hmc, fmn), turn };
                    setOver(true);
                    setWinner('draw');
                    setDrawOffer(null);
                  }} style={{ flex:1, padding:'5px', background:'linear-gradient(180deg,#27ae60,#1e8449)', color:'#fff', border:'none', borderRadius:'5px', cursor:'pointer', fontWeight:'bold', fontSize:'12px' }}>✓ Accept</button>
                  <button onClick={() => {
                    if (authoritativeMatchIdRef.current) {
                      void submitAuthoritativeIntent({ type: 'respond_draw', ...authoritativeActorForColor(controlSender), accept: false });
                      return;
                    }
                    setDrawOffer(null);
                  }} style={{ flex:1, padding:'5px', background:'linear-gradient(180deg,#c0392b,#96281b)', color:'#fff', border:'none', borderRadius:'5px', cursor:'pointer', fontWeight:'bold', fontSize:'12px' }}>✕ Decline</button>
                </div>
              </div>
            )}

            {abortActive && !over && !authoritativeMatchId && (
              <div style={{ padding:'8px 12px', borderRadius:'6px', textAlign:'center', background:'linear-gradient(90deg, rgba(180,30,20,0.95) 0%, rgba(220,50,35,0.95) 100%)', border:'1px solid rgba(231,76,60,0.5)', boxShadow:'0 0 14px rgba(231,76,60,0.3)' }}>
                <div style={{ color:'#fff', fontWeight:800, fontSize:'13px', marginBottom:'6px' }}>⚡ {movHist.length===0?'White':'Black'} must move — {abortCountdown}s left</div>
                <div style={{ height:'5px', borderRadius:'3px', background:'rgba(0,0,0,0.35)', overflow:'hidden' }}>
                  <div style={{ height:'100%', borderRadius:'3px', width:`${(abortCountdown/ABORT_SECS)*100}%`, background: abortCountdown<=3?'#ff4444':abortCountdown<=6?'#f39c12':'#2ecc71', transition:'width 0.9s linear, background 0.3s' }} />
                </div>
              </div>
            )}

            <div style={{ padding:'8px 10px', background:'rgba(0,0,0,0.2)', border:'1px solid rgba(255,165,40,0.12)', borderRadius:'8px', textAlign:'center', fontSize:'12px', fontWeight:'bold', color:'#fff' }}>
              {over ? (
                <div>
                  <div style={{ fontSize:'13px', marginBottom:'2px' }}>
                    {winner==='aborted' ? <span style={{ color:'#e74c3c' }}>Game Aborted 🚫</span>
                    : winner==='draw'   ? <span style={{ color:'#f39c12' }}>Draw! 🤝</span>
                    : <span style={{ color:'#2ecc71' }}>{winner==='white'?'⚪ White':'⚫ Black'} Wins! 🏆</span>}
                  </div>
                  {activeFinishReasonLabel ? (
                    <div style={{ fontSize:'10px', color: winner === 'draw' || winner === 'aborted' ? '#f39c12' : '#e8f7cf' }}>
                      by {activeFinishReasonLabel}
                    </div>
                  ) : null}
                </div>
              ) : (
                <div>
                  {check
                    ? <span style={{ color:'#ffaa00', fontSize:'11px' }}>⚠️ CHECK! {turn==='white'?'⚪ White':'⚫ Black'} to move</span>
                    : <span style={{ color:'#a0b8d8', fontSize:'11px' }}>Turn: {turn==='white'?'⚪ White':'⚫ Black'}</span>}
                  {hmc > 40 && <div style={{ fontSize:'10px', color:'#f39c12', marginTop:'2px' }}>50-move rule: {50-Math.floor(hmc/2)} left</div>}
                </div>
              )}
            </div>

            <div style={{ display:'flex', gap:'6px' }}>
              {over ? (
                <>
                  {winner !== 'aborted' && (
                    <button
                      disabled={authoritativeRematchBusy}
                      onClick={() => {
                        if (canCreateDirectRematch) {
                          void createAuthoritativeRematchRoom();
                          return;
                        }
                        if (canQueueSameLane) {
                          returnToSameQueueLane();
                          return;
                        }
                        if (hostedRuntime) {
                          returnToQueueHome();
                          return;
                        }
                        newGame();
                      }}
                      style={{
                        flex:1,
                        padding:'9px',
                        fontSize:'12px',
                        background:'linear-gradient(180deg,#7b2fd4,#4a1a8a)',
                        color:'#fff',
                        border:'1px solid rgba(150,80,255,0.5)',
                        borderRadius:'7px',
                        cursor: authoritativeRematchBusy ? 'default' : 'pointer',
                        fontWeight:'bold',
                        boxShadow:'0 2px 12px rgba(120,50,220,0.4)',
                        opacity: authoritativeRematchBusy ? 0.75 : 1,
                      }}
                    >
                      {finishedPrimaryActionLabel}
                    </button>
                  )}
                  <button
                    onClick={() => {
                      if (hostedRuntime) {
                        returnToQueueHome();
                        return;
                      }
                      newGame();
                    }}
                    style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#1a8a40,#0f5a28)', color:'#fff', border:'1px solid rgba(46,204,113,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(30,140,70,0.4)' }}
                  >
                    {finishedSecondaryActionLabel}
                  </button>
                </>
              ) : (
                <>
                  {movHist.length === 0 || (movHist.length === 1 && !movHist[0].b) ? (
                    <button disabled={hostedActionLocked} onClick={() => {
                      if (hostedActionLocked) {
                        return;
                      }
                      if (authoritativeMatchIdRef.current) {
                        stopAbortCountdown();
                        void submitAuthoritativeIntent({ type: 'abort', ...authoritativeActorForColor(controlSender) });
                        return;
                      }
                      stopAbortCountdown();
                      setWinner('aborted');
                      setOver(true);
                    }}
                      style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#3a4055,#222638)', color:'#ccc', border:'1px solid rgba(255,255,255,0.1)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold' }}>✕ Abort</button>
                  ) : (
                    <button onClick={newGame} style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#1a8a40,#0f5a28)', color:'#fff', border:'1px solid rgba(46,204,113,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(30,140,70,0.4)' }}>♟ New Game</button>
                  )}
            <button disabled={hostedActionLocked} onClick={() => {
              if (hostedActionLocked) {
                return;
              }
              if (confirmResign === 'prompting') {
                setConfirmResign('idle');
                if (authoritativeMatchIdRef.current) {
                  void submitAuthoritativeIntent({ type: 'resign', ...authoritativeActorForColor(controlSender) });
                  return;
                }
                finalPositionRef.current = { fen: toFEN(board, turn, moved, lm, hmc, fmn), turn };
                setOver(true);
                setWinner(OPP[turn]);
                return;
              }
              setConfirmResign('prompting');
              setTimeout(() => setConfirmResign('idle'), 3000);
            }}
            style={{
              flex:1, padding:'9px', fontSize:'12px',
              background: confirmResign === 'prompting' ? 'linear-gradient(180deg,#cc3300,#991100)' : 'linear-gradient(180deg,#8a1a1a,#5a0f0f)',
              color:'#fff',
              border: confirmResign === 'prompting' ? '2px solid #ff4444' : '1px solid rgba(220,60,60,0.4)',
              borderRadius:'7px', cursor:'pointer', fontWeight:'bold',
              boxShadow: confirmResign === 'prompting' ? '0 0 16px rgba(255,60,60,0.6)' : '0 2px 12px rgba(180,30,30,0.4)'
            }}>{confirmResign === 'prompting' ? '⚠ Confirm Resign?' : '🏳 Resign'}</button>
            {!drawOffer
              ? <button disabled={hostedActionLocked} onClick={() => {
                if (hostedActionLocked) return;
                const now = Date.now();
                if (now - lastDrawOfferTime.current < DRAW_COOLDOWN_MS) return;
                lastDrawOfferTime.current = now;
                if (authoritativeMatchIdRef.current) {
                void submitAuthoritativeIntent({ type: 'offer_draw', ...authoritativeActorForColor(controlSender) });
                  return;
                }
                setDrawOffer(turn);
              }} style={{ flex:1, padding:'9px', fontSize:'12px', background:'linear-gradient(180deg,#8a6010,#5a3e08)', color:'#fff', border:'1px solid rgba(240,160,30,0.4)', borderRadius:'7px', cursor:'pointer', fontWeight:'bold', boxShadow:'0 2px 12px rgba(180,120,20,0.4)' }}>🤝 Draw</button>
              : <button disabled style={{ flex:1, padding:'9px', fontSize:'12px', background:'rgba(60,60,80,0.35)', color:'rgba(255,255,255,0.3)', border:'1px solid rgba(255,255,255,0.08)', borderRadius:'7px', fontWeight:'bold', cursor:'not-allowed' }}>Draw sent…</button>
            }
                </>
              )}
            </div>
          </div>


        </div>
      </div>
      ) : null}
      </AppShell>
    </div>
    </ErrorBoundary>
    </PlatformContext.Provider>
    </>
  );
}
