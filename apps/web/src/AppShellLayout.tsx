'use client';

import React from 'react';
import { PlatformContext } from './contexts/PlatformContext';
import { useMatchEngineContext, useMatchEnginePropsContext } from './contexts/MatchEngineProvider';
import { MatchBoardView } from './components/match/MatchBoardView';
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
import AppShell from './components/layout/AppShell';
import { ErrorBoundary } from './components/ErrorBoundary';
import { useTutorial } from './hooks/useTutorial';
import { OnboardingTutorial } from './components/OnboardingTutorial';
// NavBar removed — keeping only the sidebar in AppShell
import { SocialAlertBanner } from './components/SocialAlertBanner';
import { useSound, playSound } from './hooks/useSound';
import { useAccessibility } from './hooks/useAccessibility';
import { useToast } from './hooks/useToast';
import { ToastContainer } from './components/Toast';
import { useOnlineStatus } from './hooks/useOnlineStatus';
import { GLOBAL_STYLES } from './styles';
import {
  readStoredGuestIdentity,
  writeStoredActiveMatchId,
  clearRequestedMatchQuery,
} from './lib/session-storage';
import type { AppPage } from './App';

const DRAW_COOLDOWN_MS = 15000;

export default function AppShellLayout({ children }: { children?: React.ReactNode }) {
  const engine = useMatchEngineContext();
  const engineProps = useMatchEnginePropsContext();
  const tutorial = useTutorial();

  const { soundEnabled, toggleSound } = useSound();
  const { colorBlindMode, toggleColorBlind } = useAccessibility();
  const { messages: toastMessages, toast: showToast, dismiss: dismissToast } = useToast();
  const online = useOnlineStatus();

  // ── Values from engine & props ─────────────────────────────────────────────
  const hostedRuntime = engineProps.hostedRuntime;
  const viewerSeat = engineProps.viewerSeat;
  const authoritativeRematchBusy = engineProps.authoritativeRematchBusy;
  const router = engineProps.router;
  const activePage = engineProps.activePage;
  const setActivePage = engineProps.setActivePage;

  const {
    topSeat, bottomSeat,
    check, over, movHist, chatMessages,
    timeW, timeB, tickingState, clockActive, authoritativeLive,
    cardAnim, cardAnimLbl, setCardAnim,
    renderJokerPicker,
    primaryNavItems, secondaryNavItems, shellPageMeta, utilityGroups,
    activeSecondaryNav, showReturnToMatch, hasPrimaryAccountSession,
    visibleSocialAlert, handleSocialAlertAction, dismissSocialAlert,
    showPlayHub, showBoardSurface, authoritativeMatchId, boardStatusLabel,
    openLiveMatch, copyLiveMatchLink,
    snapshots, reviewIdx, reviewPrev, reviewNext,
    setSel, setHints, setDrag, setDragPos, setPromo, setCardPromo,
    setEngineOn, engineOn,
  } = engine;

  // ── Keyboard handler ───────────────────────────────────────────────────────
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
  }, [reviewIdx, snapshots.length, over, reviewPrev, reviewNext, setSel, setHints, setDrag, setDragPos, setPromo, setCardPromo, setEngineOn]);

  // ── Sound effects ──────────────────────────────────────────────────────────
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

  // ── Platform context ───────────────────────────────────────────────────────
  const platformContextValue = React.useMemo(() => ({
    hostedRuntime,
    setHostedRuntime: engineProps.setHostedRuntime,
    whiteProfile: engineProps.whiteProfile,
    blackProfile: engineProps.blackProfile,
    queueLaunchIntent: engineProps.queueLaunchIntent,
    activeMatchRoomMeta: engine.activeMatchRoomMeta,
    authoritativeMatchId,
    setAuthoritativeMatchId: engine.setAuthoritativeMatchId,
    primaryAccountIdentity: engine.primaryAccountIdentity,
    boardStatusLabel,
    viewerSeat,
    matchDestinationNotice: engineProps.matchDestinationNotice,
    activePage,
    setActivePage,
    openLiveMatch: engine.openLiveMatch,
    openReplayMatch: engine.openReplayMatch,
    openProfileHandle: engine.openProfileHandle,
    openGuestHistory: engine.openGuestHistory,
    historyFocusMatchId: engineProps.historyFocusMatchId,
    setHistoryFocusMatchId: engineProps.setHistoryFocusMatchId,
    historyFocusGuestId: engineProps.historyFocusGuestId,
    setHistoryFocusGuestId: engineProps.setHistoryFocusGuestId,
    communityFocusGuestId: engineProps.communityFocusGuestId,
    setCommunityFocusGuestId: engineProps.setCommunityFocusGuestId,
    socialLiveToken: engineProps.socialLiveToken,
    setInboxUnreadCount: engineProps.setInboxUnreadCount,
    setFriendsAttentionCount: engineProps.setFriendsAttentionCount,
    profileFocusHandle: engineProps.profileFocusHandle,
    shellAccountNotice: engine.shellAccountNotice,
    setShellAccountNotice: engine.setShellAccountNotice,
    hasPrimaryAccountSession,
    accountActionQueryDetected: engineProps.accountActionQueryDetected,
    handlePrimaryShellAuthenticated: engine.handlePrimaryShellAuthenticated,
    handleSeatAuthenticated: engine.handleSeatAuthenticated,
    syncPrimaryAccountIdentity: engine.syncPrimaryAccountIdentity,
    writeStoredActiveMatchId,
    clearRequestedMatchQuery,
    requestedMatchIdRef: engine.requestedMatchIdRef,
    readStoredGuestIdentity,
    copyLiveMatchLink: engine.copyLiveMatchLink,
    showReturnToMatch,
    activeMatchQueue: engine.activeMatchRoomMeta?.queue ?? null,
    activeMatchModeId: engine.activeMatchRoomMeta?.modeId ?? null,
    setQueueLaunchIntent: engineProps.setQueueLaunchIntent,
    setMatchDestinationNotice: engineProps.setMatchDestinationNotice,
    setBootstrapQueueRecovery: engineProps.setBootstrapQueueRecovery,
    openAuthoritativeMatch: engine.openLiveMatch,
  }), [
    hostedRuntime, engineProps, engine,
    authoritativeMatchId, boardStatusLabel, viewerSeat, activePage,
    setActivePage, engine.primaryAccountIdentity, hasPrimaryAccountSession,
    showReturnToMatch,
  ]);

  // ── Loading skeleton ───────────────────────────────────────────────────────
  if (hostedRuntime === null) {
    return (
      <div style={{
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        minHeight: '100vh', background: '#0a0d16', color: '#ffbe5a', gap: '16px'
      }}>
        <div style={{ fontSize: '28px', fontWeight: 800, letterSpacing: '2px' }}>♟ CHESS404</div>
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

  // ── Render ─────────────────────────────────────────────────────────────────
  return (
    <PlatformContext.Provider value={platformContextValue}>
    {!online && (
      <div style={{
        position: 'fixed', top: 0, left: 0, right: 0, zIndex: 9999,
        background: '#dc2626', color: '#fff', textAlign: 'center',
        padding: '10px 16px', fontWeight: 700, fontSize: '14px'
      }}>
        🔴 You are offline — some features may not work
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
      <div style={{ position:'fixed', inset:0, background:'linear-gradient(160deg, rgba(8,4,20,0.45) 0%, rgba(15,6,30,0.35) 50%, rgba(5,2,15,0.50) 100%)', pointerEvents:'none', zIndex:0 }} />
      <style>{GLOBAL_STYLES}</style>

      <CardAnimOverlay anim={cardAnim} label={cardAnimLbl} onDone={() => setCardAnim(null)} />

      {renderJokerPicker()}

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
          whiteProfile={engineProps.whiteProfile}
          blackProfile={engineProps.blackProfile}
          preferredQueue={engineProps.queueLaunchIntent?.queue}
          preferredModeId={engineProps.queueLaunchIntent?.modeId}
          queueRecovery={engineProps.bootstrapQueueRecovery}
          displayName={engineProps.whiteProfile?.displayName ?? null}
          identity={{
            guestId: readStoredGuestIdentity('white').guestId,
            sessionSecret: readStoredGuestIdentity('white').sessionSecret,
            sessionToken: readStoredGuestIdentity('white').sessionToken,
            accountId: engine.primaryAccountIdentity.accountId,
            accountSessionToken: engine.primaryAccountIdentity.sessionToken,
          }}
          activeMatchId={authoritativeMatchId}
          activeMatchQueue={engine.activeMatchRoomMeta?.queue ?? null}
          activeMatchModeId={engine.activeMatchRoomMeta?.modeId ?? null}
          boardStatusLabel={boardStatusLabel}
          viewerSeat={viewerSeat}
          matchDestinationNotice={engineProps.matchDestinationNotice}
          onReturnToMatch={() => {
            if (authoritativeMatchId) {
              void engine.openLiveMatch(authoritativeMatchId);
            }
          }}
          onCopyMatchLink={(matchId) => { void engine.copyLiveMatchLink(matchId); }}
          tutorialActive={tutorial.active}
        />
        </ErrorBoundary>
      ) : activePage === 'Account' ? (
        !hasPrimaryAccountSession && !engineProps.accountActionQueryDetected ? (
          <AuthPage
            hostedRuntime={hostedRuntime}
            guestProfile={engineProps.whiteProfile}
            externalNotice={engine.shellAccountNotice}
            onAuthenticated={engine.handlePrimaryShellAuthenticated}
            onOpenAccount={() => setActivePage('Account')}
            onContinue={() => setActivePage('Play')}
            onAuthStateChange={engine.syncPrimaryAccountIdentity}
          />
        ) : (
          <AccountPage
            whiteProfile={engineProps.whiteProfile}
            blackProfile={engineProps.blackProfile}
            externalNotice={engine.shellAccountNotice}
            onOpenProfile={engine.openProfileHandle}
            onSeatAuthenticated={engine.handleSeatAuthenticated}
            onAuthStateChange={engine.syncPrimaryAccountIdentity}
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
                  engine.requestedMatchIdRef.current = null;
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
        <MatchBoardView />
        </ErrorBoundary>
      ) : null}
      </AppShell>
    </main>
    <ToastContainer messages={toastMessages} onDismiss={dismissToast} />
    <OnboardingTutorial tutorial={tutorial} activePage={activePage} />
    </PlatformContext.Provider>
  );
}
