'use client';

import React, { createContext, useContext } from 'react';
import type { Sq } from '@chess404/contracts';

export interface MatchAnimContextShape {
  cardAnim: any | null;
  cardAnimLbl: string;
  bombPieces: any[];
  bombExploding: Sq[];
  swapAnim: any | null;
  transformAnim: any | null;
  sniperAnim: any | null;
  teleportAnim: any | null;
  jumpAnim: any | null;
  sacrificeAnim: any | null;
  mindControlAnim: any | null;
  fuseAnim: any | null;
}

const MatchAnimContext = createContext<MatchAnimContextShape | null>(null);

export function useMatchAnim(): MatchAnimContextShape {
  const ctx = useContext(MatchAnimContext);
  if (!ctx) throw new Error('useMatchAnim must be used within MatchEngineProvider');
  return ctx;
}

export default MatchAnimContext;
