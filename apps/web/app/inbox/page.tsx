'use client';
import React from 'react';
import InboxPage from '../../src/InboxPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function InboxRoute() {
  const p = usePlatform();
  return (
    <InboxPage
      accountId={p.primaryAccountIdentity?.accountId ?? null}
      sessionToken={p.primaryAccountIdentity?.sessionToken ?? null}
      liveRefreshToken={p.socialLiveToken ?? undefined}
      onOpenProfile={p.openProfileHandle}
      onOpenFriends={() => p.setActivePage('Friends')}
      onUnreadCountChange={p.setInboxUnreadCount}
    />
  );
}
