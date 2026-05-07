import { CARD_POOL, createMulberry32, drawRandomCard as drawCardFromRng, RARITY_CUMULATIVE } from '@chess404/game-core';
import type { GameCard } from './types';

export { CARD_POOL, RARITY_CUMULATIVE };

let cardSeq = 0;
const clientRng = createMulberry32(404);

export const drawRandomCard = (ownerSuffix: string): GameCard => {
  cardSeq += 1;
  return drawCardFromRng(clientRng, ownerSuffix, cardSeq);
};

export const incrementCardSeq = (): number => {
  cardSeq += 1;
  return cardSeq;
};
