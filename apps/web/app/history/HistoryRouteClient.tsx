'use client';

import React from 'react';
import HistoryPage from '../../src/HistoryPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

interface HistoryRouteClientProps {
  replayMatchId: string | null;
  guestId: string | null;
}

export default function HistoryRouteClient({
  replayMatchId,
  guestId,
}: HistoryRouteClientProps) {
  const p = usePlatform();

  React.useLayoutEffect(() => {
    p.setHistoryFocusMatchId(replayMatchId);
    p.setHistoryFocusGuestId(guestId);
  }, [guestId, p.setHistoryFocusGuestId, p.setHistoryFocusMatchId, replayMatchId]);

  return (
    <HistoryPage
      focusMatchId={p.historyFocusMatchId}
      focusGuestId={p.historyFocusGuestId}
      onSelectMatchId={p.setHistoryFocusMatchId}
      onOpenGuest={(id) => {
        p.setCommunityFocusGuestId(id);
        p.setActivePage('Community');
      }}
      onClearGuestFocus={() => p.setHistoryFocusGuestId(null)}
      onWatchLiveMatch={p.openLiveMatch}
    />
  );
}
