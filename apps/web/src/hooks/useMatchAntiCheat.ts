'use client';

import React from 'react';
import type { PieceColor } from '../types';

export function useMatchAntiCheat() {
  const [cheaterTurnsLeft, setCheaterTurnsLeft] = React.useState(0);
  const [cheaterColor,     setCheaterColor]     = React.useState<PieceColor | null>(null);
  const cheaterColorRef = React.useRef<PieceColor | null>(null);
  const cheaterActive = cheaterTurnsLeft > 0;
  const [radarActive,   setRadarActive]   = React.useState(false);

  React.useEffect(() => { cheaterColorRef.current = cheaterColor; }, [cheaterColor]);

  const resetAntiCheat = React.useCallback(() => {
    setCheaterTurnsLeft(0);
    setCheaterColor(null);
    setRadarActive(false);
  }, []);

  return {
    cheaterTurnsLeft, setCheaterTurnsLeft,
    cheaterColor, setCheaterColor,
    cheaterColorRef,
    cheaterActive,
    radarActive, setRadarActive,
    resetAntiCheat,
  };
}
