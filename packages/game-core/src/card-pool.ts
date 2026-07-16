import type { GameCard, Rarity } from '@chess404/contracts';
import { RARITY_STYLE, RARITY_WEIGHTS } from './constants';
import type { Rng } from './rng';

import rawCards from './cards.json';

export const CARD_POOL: readonly Omit<GameCard, 'id'>[] = (rawCards as any[]).map(({ id: _id, ...rest }) => rest as Omit<GameCard, 'id'>);

export const RARITY_CUMULATIVE = (() => {
  let cumulative = 0;
  return (Object.entries(RARITY_WEIGHTS) as [Rarity, number][]).map(([rarity, weight]) => {
    cumulative += weight;
    return { rarity, threshold: cumulative };
  });
})();

export const drawRandomCard = (rng: Rng, ownerSuffix: string, sequence: number): GameCard => {
  const roll = rng.next() * 100;
  const chosen = RARITY_CUMULATIVE.find(({ threshold }) => roll < threshold);
  const chosenRarity: Rarity = chosen?.rarity ?? 'trash';
  const pool = CARD_POOL.filter((card) => card.rarity === chosenRarity);
  const template = pool[Math.floor(rng.next() * pool.length)] ?? pool[0];
  const style = RARITY_STYLE[chosenRarity];

  return {
    ...template,
    id: `card_${sequence}_${ownerSuffix}`,
    color: style.color,
    accent: style.accent
  };
};
