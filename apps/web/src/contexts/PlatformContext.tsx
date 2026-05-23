'use client';

import React, { createContext, useContext } from 'react';

export interface PlatformContextShape {
  hostedRuntime: boolean;
  setHostedRuntime: (v: boolean) => void;
  whiteProfile: unknown;
  blackProfile: unknown;
  queueLaunchIntent: unknown;
  activeMatchRoomMeta: unknown;
  authoritativeMatchId: string | null;
  setAuthoritativeMatchId: (id: string | null) => void;
  primaryAccountIdentity: unknown;
  boardStatusLabel: string;
  viewerSeat: string | null;
  matchDestinationNotice: unknown;
  setActivePage: (page: any) => void;
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
  socialLiveToken: unknown;
  setInboxUnreadCount: (count: number) => void;
  profileFocusHandle: string | null;
  shellAccountNotice: unknown;
  hasPrimaryAccountSession: boolean;
  accountActionQueryDetected: boolean;
  handlePrimaryShellAuthenticated: (...args: any[]) => void;
  handleSeatAuthenticated: (...args: any[]) => void;
  syncPrimaryAccountIdentity: (...args: any[]) => void;
  writeStoredActiveMatchId: (id: string) => void;
  clearRequestedMatchQuery: () => void;
  requestedMatchIdRef: { current: string | null };
  readStoredGuestIdentity: (color: 'white' | 'black') => unknown;
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
