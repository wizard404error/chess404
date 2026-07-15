'use client';
import React from 'react';
import CommunityPage from '../../src/CommunityPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function CommunityRoute() {
  const p = usePlatform();
  return (
    <CommunityPage
      whiteProfile={p.whiteProfile}
      blackProfile={p.blackProfile}
      focusGuestId={p.communityFocusGuestId}
      onOpenAccount={p.openProfileHandle}
      onOpenMatch={p.openReplayMatch}
      onOpenGuestHistory={p.openGuestHistory}
    />
  );
}
