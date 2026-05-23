'use client';
import React from 'react';
import { usePlatform } from '../../src/contexts/PlatformContext';

export default function Route() {
  const platform = usePlatform();
  React.useEffect(() => {
    platform.setActivePage('Cards');
  }, [platform.setActivePage]);
  return null;
}

