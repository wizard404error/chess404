'use client';

import React, { createContext, useContext } from 'react';
import type { GameCard, CardPendingState, PieceColor, Rarity } from '../types';

export interface MatchCardContextShape {
  selectedCard: GameCard | null;
  setSelectedCard: (card: GameCard | null) => void;
  cardPending: CardPendingState;
  whiteHand: any[];
  blackHand: any[];
  topHand: any[];
  bottomHand: any[];
  cardUsedBy: { white: boolean; black: boolean };
  canUseCard: (card: GameCard, playerColor: PieceColor) => boolean;
  lastDrawAnim: { color: PieceColor; rarity: Rarity } | null;
  dealPhase: 'idle' | 'dealing' | 'done';
}

const MatchCardContext = createContext<MatchCardContextShape | null>(null);

export function useMatchCard(): MatchCardContextShape {
  const ctx = useContext(MatchCardContext);
  if (!ctx) throw new Error('useMatchCard must be used within MatchEngineProvider');
  return ctx;
}

export default MatchCardContext;
