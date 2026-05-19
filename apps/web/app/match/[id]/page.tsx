'use client';

import React from 'react';
import { usePlatform } from '../../../src/contexts/PlatformContext';

export default function MatchRoute({ params }: { params: Promise<{ id: string }> }) {
  const platform = usePlatform();

  React.useEffect(() => {
    // Un-wrap Next.js 15 async params in client via React.use if needed, but since it's just a simple id we can use it directly
    // Wait, params is a Promise in Next.js 15 Page components? Let's use React.use.
    const resolveParams = async () => {
      const p = await params;
      platform.requestedMatchIdRef.current = p.id;
      if (!platform.authoritativeMatchId) {
        // If not already in a match, force the ID change so App.tsx loads it
        platform.setAuthoritativeMatchId(p.id);
      }
      platform.setActivePage('Match');
    };
    resolveParams();
  }, [platform, params]);

  return null;
}
