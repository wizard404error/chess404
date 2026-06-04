import type { GameCard, Rarity } from '@chess404/contracts';
import { RARITY_STYLE, RARITY_WEIGHTS } from './constants';
import type { Rng } from './rng';

export const CARD_POOL: readonly Omit<GameCard, 'id'>[] = [
  { name: 'Bad Sniper', mechanic: 'badsniper', type: 'spell', rarity: 'trash', color: '#1c1c1c', accent: '#6b7280', icon: '🔫', desc: 'Remove one of your own pieces (not king).' },
  { name: 'Demote', mechanic: 'demote', type: 'spell', rarity: 'trash', color: '#1c1c1c', accent: '#6b7280', icon: '⬇️', desc: 'Lower one of your pieces to a weaker type. Not king.' },
  { name: 'Gambler', mechanic: 'gambler', type: 'spell', rarity: 'trash', color: '#1c1c1c', accent: '#6b7280', icon: '🎲', desc: '50% steal a card from opponent. 50% give one of yours away.' },
  { name: 'Promote Him', mechanic: 'promotehim', type: 'spell', rarity: 'trash', color: '#1c1c1c', accent: '#6b7280', icon: '📈', desc: 'Promote enemy piece to higher value. Not king.' },
  { name: 'Half Fuse', mechanic: 'halffuse', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '⚗️', desc: 'Fuse two adjacent pieces into one. Not king.' },
  { name: 'Swap Me', mechanic: 'swapme', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🔄', desc: 'Exchange positions of two of your pieces. No check. No king.' },
  { name: 'Jump', mechanic: 'jump', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🦘', desc: 'Move one of your pieces to any empty square in your half. Not king.' },
  { name: 'Small Sacrifice', mechanic: 'smallsacrifice', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🩸', desc: 'Sacrifice pieces totaling 6+ points to draw 2 cards.' },
  { name: 'Freeze', mechanic: 'freeze', type: 'trap', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🧊', desc: 'Freeze one enemy piece for 1 turn. Not king.' },
  { name: 'Promote', mechanic: 'promote', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '⬆️', desc: 'Promote one of your pieces. Not king.' },
  { name: 'Shield', mechanic: 'shield', type: 'trap', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🛡️', desc: 'Protect one of your pieces from capture for 1 turn.' },
  { name: 'Fog Village', mechanic: 'fog_village', type: 'spell', rarity: 'common', color: '#1a2e1a', accent: '#4ade80', icon: '🌫️', desc: '3x3 zone: opponent cannot see pieces inside.' },
  { name: 'Full Fusion', mechanic: 'fullfusion', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '⚡', desc: 'Merge two adjacent pieces into one with combined movement. Not king.' },
  { name: 'Swap Us', mechanic: 'swapus', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '↔️', desc: 'Swap one of your pieces with one enemy piece. No kings.' },
  { name: 'Swap Him', mechanic: 'swaphim', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '🔁', desc: 'Swap two enemy pieces. No kings.' },
  { name: 'Double Move (Twin)', mechanic: 'doublemove_diff', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '👥', desc: 'Move two different pieces this turn.' },
  { name: 'Double Move (Solo)', mechanic: 'doublemove_same', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '🏃', desc: 'Move the same piece twice this turn.' },
  { name: 'Demote Him', mechanic: 'demotehim', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '📉', desc: 'Lower any piece to weaker type. Not king.' },
  { name: 'Fake Piece', mechanic: 'fakepiece', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '👻', desc: 'Place an illusion piece.' },
  { name: 'Teleport', mechanic: 'teleport', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '🌀', desc: 'Move one of your pieces to any empty square. Not king.' },
  { name: 'Lava Ground', mechanic: 'lavaground', type: 'trap', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '🌋', desc: 'Mark one square. Any piece there next turn is destroyed.' },
  { name: 'Radar', mechanic: 'radar', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '📡', desc: 'See all opponent cards for 1 turn.' },
  { name: 'Mirror', mechanic: 'mirror', type: 'trap', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '🪞', desc: 'Copy opponent last move with your equivalent piece, if legal.' },
  { name: 'Cheater', mechanic: 'cheater', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '💡', desc: 'Reveal the best move to you.' },
  { name: 'Invisible', mechanic: 'invisible', type: 'spell', rarity: 'rare', color: '#1a2a4a', accent: '#60a5fa', icon: '👁️', desc: 'One of your pieces becomes invisible for 1 round.' },
  { name: 'Sniper', mechanic: 'sniper', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🎯', desc: 'Remove any piece from the board. Not king.' },
  { name: 'Fortress', mechanic: 'fortress', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🏰', desc: '2x2 zone: enemies cannot enter for 2 turns.' },
  { name: 'Clone', mechanic: 'clone', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🧬', desc: 'Copy one of your pieces onto an adjacent empty square. Not king.' },
  { name: 'Borrow', mechanic: 'borrow', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🤏', desc: 'Control one enemy piece for 1 turn. Not king.' },
  { name: 'Parasite', mechanic: 'parasite', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🦠', desc: 'Link your piece to enemy piece. If yours dies, theirs dies too.' },
  { name: 'Black Hole', mechanic: 'blackhole', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '🕳️', desc: 'Choose 2 squares. After 2 turns all adjacent pieces explode.' },
  { name: 'Big Sacrifice', mechanic: 'bigsacrifice', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '💎', desc: 'Sacrifice pieces totaling 14+ points to draw 3 cards.' },
  { name: 'Undo', mechanic: 'undo', type: 'trap', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '↩️', desc: 'Cancel the last card your opponent played.' },
  { name: 'Reverse', mechanic: 'reverse', type: 'trap', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '⏪', desc: 'Undo opponent last move.' },
  { name: 'Unabomber', mechanic: 'unabomber', type: 'spell', rarity: 'epic', color: '#2d1a4a', accent: '#c084fc', icon: '💣', desc: 'Attach bomb to your piece. Next round it explodes.' },
  { name: 'Mind Control', mechanic: 'mindcontrol', type: 'spell', rarity: 'legendary', color: '#4a2a00', accent: '#f59e0b', icon: '🧠', desc: 'Permanently steal one enemy piece. Not king.' },
  { name: 'Joker', mechanic: 'joker', type: 'spell', rarity: 'rare', color: '#4a2a00', accent: '#f59e0b', icon: '🃏', desc: 'Choose any card from the full card pool instantly.' }
];

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
