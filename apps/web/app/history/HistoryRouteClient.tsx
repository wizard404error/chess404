'use client';

import React from 'react';
import { usePlatform } from '../../src/contexts/PlatformContext';

interface HistoryRouteClientProps {
  replayMatchId: string | null;
  guestId: string | null;
}

export default function HistoryRouteClient({
  replayMatchId,
  guestId,
}: HistoryRouteClientProps) {
  const platform = usePlatform();

  React.useLayoutEffect(() => {
    platform.setHistoryFocusMatchId(replayMatchId);
    platform.setHistoryFocusGuestId(guestId);
    platform.setActivePage('History');
  }, [
    guestId,
    platform.setActivePage,
    platform.setHistoryFocusGuestId,
    platform.setHistoryFocusMatchId,
    replayMatchId,
  ]);

  return null;
}
