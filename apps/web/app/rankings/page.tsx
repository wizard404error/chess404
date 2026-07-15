'use client';
import React from 'react';
import RankingsPage from '../../src/RankingsPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function RankingsRoute() {
  const p = usePlatform();
  return (
    <RankingsPage
      onViewGuest={(guestId) => {
        p.setCommunityFocusGuestId(guestId);
        p.setActivePage('Community');
      }}
      onViewAccount={p.openProfileHandle}
    />
  );
}
