'use client';
import React from 'react';
import ProfilesPage from '../../src/ProfilesPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function ProfilesRoute() {
  const p = usePlatform();
  return (
    <ProfilesPage
      focusHandle={p.profileFocusHandle}
      viewerHandle={null}
      accountId={p.primaryAccountIdentity?.accountId ?? null}
      sessionToken={p.primaryAccountIdentity?.sessionToken ?? null}
      onSelectHandle={p.openProfileHandle}
      onOpenAccount={() => p.setActivePage('Account')}
      onOpenReplay={p.openReplayMatch}
    />
  );
}
