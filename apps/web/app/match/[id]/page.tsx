'use client';

import React from 'react';
import { usePlatform } from '../../../src/contexts/PlatformContext';

export default function MatchRoute({ params }: { params: Promise<{ id: string }> }) {
  const platform = usePlatform();
  const resolvedParams = React.use(params);
  const id = resolvedParams.id;
  const prevIdRef = React.useRef<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    // Avoid re-processing the same match ID
    if (prevIdRef.current === id) return;
    prevIdRef.current = id;

    platform.requestedMatchIdRef.current = id;
    if (!platform.authoritativeMatchId) {
      // If not already in a match, force the ID change so App.tsx loads it
      platform.setAuthoritativeMatchId(id);
    }
    platform.setActivePage('Match');
  }, [id, platform.authoritativeMatchId, platform.requestedMatchIdRef, platform.setActivePage, platform.setAuthoritativeMatchId]);

  return null;
}
