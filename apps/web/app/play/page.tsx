'use client';

import React from 'react';
import PlayHubPage from '../../src/PlayHubPage';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function PlayRoute() {
  const platform = usePlatform();

  React.useEffect(() => {
    platform.setActivePage('Play');
  }, [platform.setActivePage]);

  return null;
}

