'use client';
import React from 'react';
import WatchPage from '../../src/WatchPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function WatchRoute() {
  const p = usePlatform();
  return <WatchPage onWatchMatch={p.openLiveMatch} onOpenReplay={p.openReplayMatch} />;
}
