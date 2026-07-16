'use client';

import React from 'react';
import type { PieceColor, GameCard } from '../types';
import type { MatchFinishReason } from '@chess404/contracts';
import type { GuestProfile } from '../lib/platform-service';
import type { ShellNavGroup, ShellNavItem, ShellPageMeta } from '../components/layout/AppShell';
import type { SocialAlert } from '../lib/match-labels';
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
} from '../components/layout/icons';
import { modeLabel, queueLabel, finishReasonLabel } from '../lib/match-labels';
import { buildLiveMatchUrl, buildReplayPageUrl } from '../lib/session-storage';
import { readStoredRoomMeta } from '../lib/match-service';

interface UseMatchNavProps {
  authoritativeMatchId: string | null;
  authoritativeStatus: 'waiting' | 'active' | 'finished' | null;
  hostedRuntime: boolean | null;
  viewerSeat: PieceColor | null;
  authoritativeLive: boolean;
  authoritativeRematchBusy: boolean;
  primaryAccountIdentity: { accountId?: string; sessionToken?: string };
  activePage: string;
  friendsAttentionCount: number;
  inboxUnreadCount: number;
  whiteHand: GameCard[];
  blackHand: GameCard[];
  over: boolean;
  winner: PieceColor | 'draw' | 'aborted' | null;
  authoritativeFinishReason: MatchFinishReason | null;
  hmc: number;
  stale: boolean;
  insuf: boolean;
  mate: boolean;
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
  matchSeatMeta: {
    whiteGuestId?: string;
    blackGuestId?: string;
    whiteName?: string;
    blackName?: string;
  } | null;
  authoritativeDisconnectGraceFor: PieceColor | null;
  authoritativeDisconnectGraceDeadline: string | null;
  authoritativeWhiteConnected: boolean;
  authoritativeBlackConnected: boolean;
  timeW: number;
  timeB: number;
  tickingState: PieceColor | null;
  clockActive: boolean;
  socialAlert: SocialAlert | null;
  dismissedSocialAlertIdsRef: React.MutableRefObject<Set<string>>;
  turn: PieceColor;
  openLiveMatch: (matchId: string) => void;
  setActivePage: React.Dispatch<React.SetStateAction<any>>;
  setSocialAlert: React.Dispatch<React.SetStateAction<SocialAlert | null>>;
}

export function useMatchNav(props: UseMatchNavProps) {
  const {
    authoritativeMatchId, authoritativeStatus, hostedRuntime, viewerSeat, authoritativeLive,
    authoritativeRematchBusy,
    primaryAccountIdentity, activePage, friendsAttentionCount, inboxUnreadCount,
    whiteHand, blackHand, over, winner, authoritativeFinishReason, hmc, stale, insuf, mate,
    whiteProfile, blackProfile, matchSeatMeta,
    authoritativeDisconnectGraceFor, authoritativeDisconnectGraceDeadline,
    authoritativeWhiteConnected, authoritativeBlackConnected,
    timeW, timeB, tickingState, clockActive,
    socialAlert, dismissedSocialAlertIdsRef, turn,
    openLiveMatch, setActivePage, setSocialAlert,
  } = props;

  const boardStatusLabel = authoritativeMatchId
    ? authoritativeStatus === 'waiting'
      ? 'Private Match Waiting Room'
      : hostedRuntime && !viewerSeat
        ? (authoritativeLive ? 'Spectating Live Match' : 'Spectator Sync Reconnecting')
        : (authoritativeLive ? 'Online Match Live' : 'Match Sync Reconnecting')
    : hostedRuntime
      ? 'Competitive Match Destination'
      : 'Local Play Sandbox';
  const hasPrimaryAccountSession = Boolean((primaryAccountIdentity.accountId ?? '').trim() && (primaryAccountIdentity.sessionToken ?? '').trim());
  const showSocialNav = hasPrimaryAccountSession || activePage === 'Friends' || activePage === 'Inbox';
  const showAdminNav = activePage === 'Admin' || activePage === 'Status';
  const primaryNavItems: ShellNavItem[] = [
    { key: 'Play', label: 'Play', icon: <PlayIcon /> },
    { key: 'Watch', label: 'Watch', icon: <WatchIcon /> },
    { key: 'Rankings', label: 'Rankings', icon: <TrophyIcon /> },
    { key: 'Profiles', label: 'Profiles', icon: <ProfileIcon /> },
  ];
  const utilityGroups: ShellNavGroup[] = [
    {
      label: 'Library',
      items: [
        { key: 'History', label: 'History', icon: <HistoryIcon /> },
        { key: 'Cards', label: 'Cards', icon: <CardsIcon /> },
        { key: 'Community', label: 'Community', icon: <CommunityIcon /> },
      ],
    },
    ...(showSocialNav
      ? [{
          label: 'Social',
          items: [
            { key: 'Friends', label: 'Friends', icon: <FriendsIcon />, badge: friendsAttentionCount > 0 ? friendsAttentionCount : null },
            { key: 'Inbox', label: 'Inbox', icon: <InboxIcon />, badge: inboxUnreadCount > 0 ? inboxUnreadCount : null },
          ],
        } satisfies ShellNavGroup]
      : []),
    ...(showAdminNav
      ? [{
          label: 'Admin',
          items: [
            { key: 'Admin', label: 'Moderation', icon: <AdminIcon /> },
            { key: 'Status', label: 'Status', icon: <StatusIcon /> },
          ],
        } satisfies ShellNavGroup]
      : []),
  ];
  const secondaryNavItems = utilityGroups.flatMap((group) =>
    group.items.map((item) => ({
      key: item.key as string,
      label: item.label,
      badge: item.badge,
    }))
  );
  const activeSecondaryNav = secondaryNavItems.some((item) => item.key === activePage);
  const showReturnToMatch = !!hostedRuntime && Boolean(authoritativeMatchId);
  const showPlayHub = hostedRuntime
    ? (activePage === 'Play' || activePage === 'Modes' || activePage === 'Queue' || activePage === 'Lobbies')
    : (activePage === 'Modes' || activePage === 'Queue' || activePage === 'Lobbies');
  const showBoardSurface = activePage === 'Match' || (!hostedRuntime && activePage === 'Play');
  const controlledSeat = hostedRuntime ? viewerSeat : null;
  const topSeat: PieceColor = controlledSeat === 'black' ? 'white' : 'black';
  const bottomSeat: PieceColor = controlledSeat === 'black' ? 'black' : 'white';
  const topHand = topSeat === 'white' ? whiteHand : blackHand;
  const bottomHand = bottomSeat === 'white' ? whiteHand : blackHand;
  const whiteSeatBadge = hostedRuntime
    ? viewerSeat === 'white'
      ? 'You'
      : viewerSeat === 'black'
        ? 'Opponent'
        : 'Spectator'
    : null;
  const blackSeatBadge = hostedRuntime
    ? viewerSeat === 'black'
      ? 'You'
      : viewerSeat === 'white'
        ? 'Opponent'
        : 'Spectator'
    : null;
  const showHostedSoloBanner = hostedRuntime && !authoritativeMatchId;
  const showHostedReconnectWarning = false;
  const activeDisconnectGraceFor = authoritativeStatus === 'active' ? authoritativeDisconnectGraceFor : null;
  const disconnectGraceDeadlineLabel = authoritativeDisconnectGraceDeadline
    ? new Date(authoritativeDisconnectGraceDeadline).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : null;
  const whitePresenceLabel = authoritativeMatchId ? (authoritativeWhiteConnected ? 'Online' : 'Disconnected') : null;
  const blackPresenceLabel = authoritativeMatchId ? (authoritativeBlackConnected ? 'Online' : 'Disconnected') : null;
  const activeFinishReason = over
    ? authoritativeFinishReason
      ?? (winner === 'aborted'
        ? 'abort'
        : winner === 'draw'
          ? (hmc >= 100 ? 'fifty_move_rule' : stale ? 'stalemate' : insuf ? 'insufficient_material' : null)
          : mate
            ? 'checkmate'
            : null)
    : null;
  const activeFinishReasonLabel = finishReasonLabel(activeFinishReason);
  const displayedWhiteName = hostedRuntime && authoritativeMatchId
    ? (matchSeatMeta?.whiteName ?? (viewerSeat === 'white' ? whiteProfile?.displayName : undefined) ?? 'White')
    : (whiteProfile?.displayName ?? 'Player');
  const displayedBlackName = hostedRuntime && authoritativeMatchId
    ? (matchSeatMeta?.blackName ?? (viewerSeat === 'black' ? whiteProfile?.displayName : undefined) ?? 'Black')
    : (blackProfile?.displayName ?? 'Opponent');
  const disconnectGraceBanner = activeDisconnectGraceFor
    ? viewerSeat === activeDisconnectGraceFor
      ? `Your seat is in reconnect grace. Rejoin before ${disconnectGraceDeadlineLabel ?? 'the timer expires'} or the match will be forfeited.`
      : `${activeDisconnectGraceFor === 'white' ? displayedWhiteName : displayedBlackName} disconnected. The match will forfeit if they do not return by ${disconnectGraceDeadlineLabel ?? 'the end of the grace window'}.`
    : null;
  const displayedWhiteRating = hostedRuntime && authoritativeMatchId
    ? ((viewerSeat === 'white' ? whiteProfile?.rating : blackProfile?.rating) ?? 1200)
    : (whiteProfile?.rating ?? 1200);
  const displayedBlackRating = hostedRuntime && authoritativeMatchId
    ? ((viewerSeat === 'black' ? whiteProfile?.rating : blackProfile?.rating) ?? 1200)
    : (blackProfile?.rating ?? 1200);
  const activeMatchRoomMeta = authoritativeMatchId ? readStoredRoomMeta(authoritativeMatchId) : null;
  const activeMatchModeLabel = modeLabel(activeMatchRoomMeta?.modeId);
  const activeMatchQueueLabel = queueLabel(activeMatchRoomMeta?.queue);
  const canCreateDirectRematch = Boolean(authoritativeMatchId && activeMatchRoomMeta?.queue === 'direct');
  const canQueueSameLane = Boolean(
    authoritativeMatchId &&
    hostedRuntime &&
    (activeMatchRoomMeta?.queue === 'casual' || activeMatchRoomMeta?.queue === 'rated'),
  );
  const activeMatchRoleLabel = !authoritativeMatchId
    ? null
    : authoritativeStatus === 'waiting'
      ? 'Reserved seat waiting for the second player'
      : !hostedRuntime
        ? 'Local operator view'
        : viewerSeat === 'white'
          ? 'Playing as White'
          : viewerSeat === 'black'
            ? 'Playing as Black'
            : 'Spectating read-only';
  const activeMatchRouteLabel = authoritativeStatus === 'finished' || over ? 'Replay page' : 'Archive detail';
  const finishedPrimaryActionLabel = canCreateDirectRematch
    ? (authoritativeRematchBusy ? 'Creating...' : '🔄 Rematch Room')
    : canQueueSameLane
      ? '↩ Play Same Lane'
      : authoritativeMatchId
        ? (hostedRuntime ? '↩ Queue' : '♟ New Game')
        : '🔄 Rematch';
  const finishedSecondaryActionLabel = hostedRuntime
    ? '↩ Queue'
    : '♟ New Game';
  const activeLiveMatchUrl = authoritativeMatchId ? buildLiveMatchUrl(authoritativeMatchId) : null;
  const activeReplayPageUrl = authoritativeMatchId ? buildReplayPageUrl(authoritativeMatchId) : null;
  const topSeatBadge = topSeat === 'white' ? whiteSeatBadge : blackSeatBadge;
  const bottomSeatBadge = bottomSeat === 'white' ? whiteSeatBadge : blackSeatBadge;
  const topPlayerName = topSeat === 'white' ? displayedWhiteName : displayedBlackName;
  const bottomPlayerName = bottomSeat === 'white' ? displayedWhiteName : displayedBlackName;
  const topPlayerRating = topSeat === 'white' ? displayedWhiteRating : displayedBlackRating;
  const bottomPlayerRating = bottomSeat === 'white' ? displayedWhiteRating : displayedBlackRating;
  const topPlayerClock = topSeat === 'white' ? timeW : timeB;
  const bottomPlayerClock = bottomSeat === 'white' ? timeW : timeB;
  const topClockTicking = tickingState === topSeat && clockActive && !over;
  const bottomClockTicking = tickingState === bottomSeat && clockActive && !over;
  const shellPageMeta: ShellPageMeta = (() => {
    switch (activePage) {
      case 'Match':
        return {
          eyebrow: 'Match',
          title: authoritativeMatchId ? boardStatusLabel : 'Live match',
          description: authoritativeMatchId
            ? `${activeMatchQueueLabel} · ${activeMatchModeLabel}${activeMatchRoleLabel ? ` · ${activeMatchRoleLabel}` : ''}`
            : 'Live matches open here once a real room exists.',
        };
      case 'Watch':
        return {
          eyebrow: 'Watch',
          title: 'Live games and replays',
          description: 'Spectate active public matches, browse recent replays, and jump into stable match destinations.',
        };
      case 'Rankings':
        return {
          eyebrow: 'Rankings',
          title: 'Competitive ladders',
          description: 'Track official mode leaders, seasonal momentum, and player progression across the platform.',
        };
      case 'Profiles':
        return {
          eyebrow: 'Profiles',
          title: 'Public player identity',
          description: 'Search claimed handles, inspect competitive snapshots, and open shareable profile destinations.',
        };
      case 'History':
        return {
          eyebrow: 'History',
          title: 'Replay archive',
          description: 'Review finished games through curated player-facing summaries instead of raw platform state.',
        };
      case 'Friends':
        return {
          eyebrow: 'Friends',
          title: 'Friend graph',
          description: 'Manage persistent friendships and direct challenge relationships tied to your account.',
        };
      case 'Inbox':
        return {
          eyebrow: 'Inbox',
          title: 'Account inbox',
          description: 'See social notifications, challenge updates, and unread account events in one place.',
        };
      case 'Cards':
        return {
          eyebrow: 'Cards',
          title: 'Card compendium',
          description: 'Browse curated card powers and learn the mechanics that make Chess404 different from standard chess.',
        };
      case 'Community':
        return {
          eyebrow: 'Community',
          title: 'Player activity',
          description: 'Explore guest and account activity across the broader Chess404 platform.',
        };
      case 'Admin':
        return {
          eyebrow: 'Admin',
          title: 'Moderation console',
          description: 'Review reports and trust actions through the internal moderation workflow.',
        };
      case 'Status':
        return {
          eyebrow: 'Status',
          title: 'Operational status',
          description: 'Internal backend health and routing visibility for signed-in operators.',
        };
      case 'Account':
        return {
          eyebrow: hasPrimaryAccountSession ? 'Account' : 'Sign In',
          title: hasPrimaryAccountSession ? 'Account security and identity' : 'Create your Chess404 account',
          description: 'Chess404 is competitive online chess with curated card powers. Sign in once, recover easily, and carry your identity across devices.',
        };
      case 'Modes':
      case 'Queue':
      case 'Lobbies':
      case 'Play':
      default:
        return {
          eyebrow: 'Play',
          title: 'Competitive play hub',
          description: 'Quick pair into official modes or create a private invite room. The board opens only when a real match exists.',
        };
    }
  })();
  const actorSeatForHostedControls: PieceColor | null = hostedRuntime ? viewerSeat : turn;
  const actorSeatPlainLabel = actorSeatForHostedControls
    ? (actorSeatForHostedControls === 'white' ? 'White' : 'Black')
    : 'Spectator';
  const controlSender: PieceColor = actorSeatForHostedControls ?? turn;
  const hostedActionLocked = !!hostedRuntime && !viewerSeat;
  const canRespondToDrawOffer = !hostedRuntime || (!!viewerSeat && viewerSeat === turn);
  const actorSeatLabel = actorSeatForHostedControls
    ? (actorSeatForHostedControls === 'white' ? '⚪ White' : '⚫ Black')
    : 'Spectator';
  const hostedSpectator = hostedRuntime && !viewerSeat;
  const visibleSocialAlert = socialAlert && !(socialAlert.action === 'friends' && (activePage === 'Friends' || activePage === 'Inbox'))
    ? socialAlert
    : null;

  const dismissSocialAlert = React.useCallback(() => {
    if (!socialAlert) {
      return;
    }
    dismissedSocialAlertIdsRef.current.add(socialAlert.id);
    setSocialAlert(null);
  }, [socialAlert]);

  const handleSocialAlertAction = React.useCallback(() => {
    if (!socialAlert) {
      return;
    }
    dismissedSocialAlertIdsRef.current.add(socialAlert.id);
    if (socialAlert.action === 'match' && socialAlert.matchId && typeof window !== 'undefined') {
      openLiveMatch(socialAlert.matchId);
      return;
    }
    setActivePage('Friends');
    setSocialAlert(null);
  }, [openLiveMatch, socialAlert]);

  return {
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
  };
}
