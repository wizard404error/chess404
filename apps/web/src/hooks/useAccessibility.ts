'use client';

import React from 'react';

const COLOR_BLIND_KEY = 'chess404_colorblind';

function getStored(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return localStorage.getItem(COLOR_BLIND_KEY) === 'true';
  } catch {
    return false;
  }
}

export function useAccessibility() {
  const [colorBlindMode, setColorBlindMode] = React.useState(getStored);

  React.useEffect(() => {
    try {
      localStorage.setItem(COLOR_BLIND_KEY, colorBlindMode ? 'true' : 'false');
    } catch {}
    document.documentElement.dataset.colorblind = colorBlindMode ? 'true' : 'false';
  }, [colorBlindMode]);

  const toggleColorBlind = React.useCallback(() => {
    setColorBlindMode(prev => !prev);
  }, []);

  return { colorBlindMode, toggleColorBlind };
}
