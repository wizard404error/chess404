'use client';

import React from 'react';
import { usePathname, useRouter } from 'next/navigation';
import MatchEngineProvider from './contexts/MatchEngineProvider';
import AppShellLayout from './AppShellLayout';
import { configureMatchServiceRuntime, type MatchServiceRuntimeConfig } from './lib/match-service';
import type { GuestProfile } from './lib/platform-service';
import type { SocialAlert } from './lib/match-labels';
import type { QueueName, QueueTicket } from './lib/matchmaking-service';
import type { MatchModeId } from '@chess404/contracts';
import type { PieceColor } from './types';

export type AppPage =
  | 'Play' | 'Match' | 'Watch' | 'Rankings' | 'Profiles' | 'Account'
  | 'History' | 'Friends' | 'Inbox' | 'Cards' | 'Community' | 'Status'
  | 'Admin' | 'Modes' | 'Queue' | 'Lobbies';

export default function App({ runtimeConfig, children }: { runtimeConfig?: { matchServiceHttpBase?: string; matchServiceWsBase?: string }, children?: React.ReactNode }) {
  configureMatchServiceRuntime({
    httpBaseUrl: runtimeConfig?.matchServiceHttpBase,
    wsBaseUrl: runtimeConfig?.matchServiceWsBase,
  } satisfies MatchServiceRuntimeConfig);

  const [hostedRuntime, setHostedRuntime] = React.useState<boolean | null>(null);
  const [activePage, setActivePage] = React.useState<AppPage>('Play');
  const router = useRouter();
  const pathname = usePathname();
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
  const [bootstrapQueueRecovery, setBootstrapQueueRecovery] = React.useState<{ white: QueueTicket | null; black: QueueTicket | null } | null>(null);
  const openedBoardMatchRef = React.useRef<string | null>(null);
  const [communityFocusGuestId, setCommunityFocusGuestId] = React.useState<string | null>(null);
  const [historyFocusMatchId, setHistoryFocusMatchId] = React.useState<string | null>(null);
  const [historyFocusGuestId, setHistoryFocusGuestId] = React.useState<string | null>(null);
  const [authoritativeRematchBusy, setAuthoritativeRematchBusy] = React.useState(false);
  const [whiteProfile, setWhiteProfile] = React.useState<GuestProfile | null>(null);
  const [blackProfile, setBlackProfile] = React.useState<GuestProfile | null>(null);
  const [viewerSeat, setViewerSeat] = React.useState<PieceColor | null>(null);
  const [matchSeatMeta, setMatchSeatMeta] = React.useState<{ whiteGuestId?: string; blackGuestId?: string; whiteName?: string; blackName?: string } | null>(null);
  const [guestProfilesReady, setGuestProfilesReady] = React.useState(false);

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

  return (
    <MatchEngineProvider
      accountActionQueryDetected={accountActionQueryDetected}
      activePage={activePage}
      authoritativeRematchBusy={authoritativeRematchBusy}
      blackProfile={blackProfile}
      communityFocusGuestId={communityFocusGuestId}
      friendsAttentionCount={friendsAttentionCount}
      guestProfilesReady={guestProfilesReady}
      historyFocusGuestId={historyFocusGuestId}
      historyFocusMatchId={historyFocusMatchId}
      historyQueryReady={historyQueryReady}
      hostedRuntime={hostedRuntime}
      inboxUnreadCount={inboxUnreadCount}
      matchDestinationNotice={matchDestinationNotice}
      matchQueryReady={matchQueryReady}
      matchSeatMeta={matchSeatMeta}
      openedBoardMatchRef={openedBoardMatchRef}
      pathname={pathname}
      profileFocusHandle={profileFocusHandle}
      profileQueryReady={profileQueryReady}
      bootstrapQueueRecovery={bootstrapQueueRecovery}
      queueLaunchIntent={queueLaunchIntent}
      router={router}
      setAccountActionQueryDetected={setAccountActionQueryDetected}
      setActivePage={setActivePage}
      setAuthoritativeRematchBusy={setAuthoritativeRematchBusy}
      setBlackProfile={setBlackProfile}
      setFriendsAttentionCount={setFriendsAttentionCount}
      setGuestProfilesReady={setGuestProfilesReady}
      setHistoryFocusGuestId={setHistoryFocusGuestId}
      setHistoryFocusMatchId={setHistoryFocusMatchId}
      setHistoryQueryReady={setHistoryQueryReady}
      setHostedRuntime={setHostedRuntime}
      setInboxUnreadCount={setInboxUnreadCount}
      setMatchDestinationNotice={setMatchDestinationNotice}
      setMatchQueryReady={setMatchQueryReady}
      setMatchSeatMeta={setMatchSeatMeta}
      setProfileFocusHandle={setProfileFocusHandle}
      setProfileQueryReady={setProfileQueryReady}
      setBootstrapQueueRecovery={setBootstrapQueueRecovery}
      setCommunityFocusGuestId={setCommunityFocusGuestId}
      setQueueLaunchIntent={setQueueLaunchIntent}
      setSecondaryMenuOpen={setSecondaryMenuOpen}
      setSocialAlert={setSocialAlert}
      setSocialLiveToken={setSocialLiveToken}
      setViewerSeat={setViewerSeat}
      setWhiteProfile={setWhiteProfile}
      socialAlert={socialAlert}
      socialLiveToken={socialLiveToken}
      viewerSeat={viewerSeat}
      whiteProfile={whiteProfile}
    >
      <AppShellLayout>{children}</AppShellLayout>
    </MatchEngineProvider>
  );
}
