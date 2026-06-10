'use client';

import React, { createContext, useContext } from 'react';
import type { GuestProfile } from '../lib/platform-service';
import type { QueueTicket } from '../lib/matchmaking-service';
import type { StoredRoomMeta } from '../lib/match-service';

export type AppPage = 'Play' | 'Watch' | 'Rankings' | 'History' | 'Cards' | 'Friends' | 'Inbox' | 'Community' | 'Account' | 'Status';

export interface PlatformContextShape {
  hostedRuntime: boolean | null;
  setHostedRuntime: (v: boolean | null) => void;
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
  queueLaunchIntent: { modeId: string; queue: string } | null;
  activeMatchRoomMeta: StoredRoomMeta | null;
  authoritativeMatchId: string | null;
  setAuthoritativeMatchId: (id: string | null) => void;
  primaryAccountIdentity: { accountId: string; sessionToken: string; handle: string } | null;
  boardStatusLabel: string;
  viewerSeat: 'white' | 'black' | null;
  matchDestinationNotice: string | null;
  setActivePage: (page: AppPage) => void;
  openLiveMatch: (id: string) => void;
  openReplayMatch: (id: string) => void;
  openProfileHandle: (handle: string) => void;
  openGuestHistory: (guestId: string) => void;
  historyFocusMatchId: string | null;
  setHistoryFocusMatchId: (id: string | null) => void;
  historyFocusGuestId: string | null;
  setHistoryFocusGuestId: (id: string | null) => void;
  communityFocusGuestId: string | null;
  setCommunityFocusGuestId: (id: string | null) => void;
  socialLiveToken: string | null;
  setInboxUnreadCount: (count: number) => void;
  profileFocusHandle: string | null;
  shellAccountNotice: string | null;
  hasPrimaryAccountSession: boolean;
  accountActionQueryDetected: boolean;
  handlePrimaryShellAuthenticated: (handle: string, token: string) => void;
  handleSeatAuthenticated: (seat: 'white' | 'black', token: string) => void;
  syncPrimaryAccountIdentity: () => void;
  writeStoredActiveMatchId: (id: string) => void;
  clearRequestedMatchQuery: () => void;
  requestedMatchIdRef: { current: string | null };
  readStoredGuestIdentity: (color: 'white' | 'black') => { guestId: string; sessionSecret: string } | null;
  copyLiveMatchLink: (matchId: string) => void;
}

export const PlatformContext = createContext<PlatformContextShape | null>(null);

export function usePlatform(): PlatformContextShape {
  const context = useContext(PlatformContext);
  if (!context) {
    throw new Error('usePlatform must be used within a PlatformContext.Provider');
  }
  return context;
}
