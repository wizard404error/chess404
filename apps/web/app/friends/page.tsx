'use client';
import React from 'react';
import FriendsPage from '../../src/FriendsPage';
import { usePlatform } from '../../src/contexts/PlatformContext';
import { readStoredGuestIdentity } from '../../src/lib/session-storage';

export default function FriendsRoute() {
  const p = usePlatform();
  const whiteId = readStoredGuestIdentity('white');
  const identity = {
    guestId: whiteId?.guestId,
    sessionSecret: whiteId?.sessionSecret,
    sessionToken: whiteId?.sessionToken,
    accountId: p.primaryAccountIdentity?.accountId,
    accountSessionToken: p.primaryAccountIdentity?.sessionToken,
  };
  return (
    <FriendsPage
      identity={identity}
      accountId={p.primaryAccountIdentity?.accountId ?? null}
      sessionToken={p.primaryAccountIdentity?.sessionToken ?? null}
      liveRefreshToken={p.socialLiveToken ?? undefined}
      onOpenProfile={p.openProfileHandle}
      onOpenAccount={() => p.setActivePage('Account')}
    />
  );
}
