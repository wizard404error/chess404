import type { GameCard, Rarity } from './types';
import { RARITY_WEIGHTS, RARITY_STYLE } from './constants';

export const CARD_POOL: readonly Omit<GameCard, 'id'>[] = [
  // TRASH
  { name:'Bad Sniper',     mechanic:'badsniper',       type:'spell', rarity:'trash',     color:'#1c1c1c', accent:'#6b7280', icon:'🔫', desc:'Remove one of YOUR own pieces (not king).' },
  { name:'Demote',         mechanic:'demote',          type:'spell', rarity:'trash',     color:'#1c1c1c', accent:'#6b7280', icon:'⬇️', desc:'Lower one of YOUR pieces to a weaker type. Not king.' },
  { name:'Gambler',        mechanic:'gambler',         type:'spell', rarity:'trash',     color:'#1c1c1c', accent:'#6b7280', icon:'🎲', desc:'50% steal a card from opponent. 50% give one of yours away.' },
  { name:'Promote Him',    mechanic:'promotehim',      type:'spell', rarity:'trash',      color:'#1c1c1c', accent:'#6b7280', icon:'📈', desc:"Promote enemy piece to higher value. Not king." },

  // COMMON
  { name:'Half Fuse',      mechanic:'halffuse',        type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'⚗️', desc:'Fuse two adjacent pieces (value ≤5 each) into one. Not king.' },
  { name:'Swap Me',        mechanic:'swapme',          type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🔄', desc:'Exchange positions of two of YOUR pieces. No check. No king.' },
  { name:'Jump',           mechanic:'jump',            type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🦘', desc:'Move one of your pieces to any empty square in YOUR half. Not king.' },
  { name:'Small Sacrifice',mechanic:'smallsacrifice',  type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🩸', desc:'Sacrifice one or more of your pieces totaling 6+ points to draw 2 cards. Click pieces one by one, then click an empty square to confirm.' },
  { name:'Freeze',         mechanic:'freeze',          type:'trap',  rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🧊', desc:'Freeze one enemy piece — it cannot move for 1 turn. Not king.' },
  { name:'Promote',        mechanic:'promote',         type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'⬆️', desc:'Promote one of your pieces to a higher value type. Not king.' },
  { name:'Shield',         mechanic:'shield',          type:'trap',  rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🛡️', desc:'Protect one of your pieces from capture for 1 turn.' },
  { name:'Fog Village',    mechanic:'fog_village',     type:'spell', rarity:'common',    color:'#1a2e1a', accent:'#4ade80', icon:'🌫️', desc:'3×3 zone — opponent cannot see pieces inside.' },

  // RARE
  { name:'Full Fusion',    mechanic:'fullfusion',      type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'⚡', desc:'Merge two adjacent pieces into one with combined movement. No point limit. Not king.' },
  { name:'Swap Us',        mechanic:'swapus',          type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'↔️', desc:'Swap one of YOUR pieces with one ENEMY piece. No check. No kings.' },
  { name:'Swap Him',       mechanic:'swaphim',         type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'🔁', desc:"Swap 2 of your opponent's pieces. No kings." },
  { name:'Double Move (Twin)', mechanic:'doublemove_diff', type:'spell', rarity:'rare', color:'#1a2a4a', accent:'#60a5fa', icon:'👥', desc:'Move TWO DIFFERENT pieces this turn. First must not cause check.' },
  { name:'Double Move (Solo)', mechanic:'doublemove_same', type:'spell', rarity:'rare', color:'#1a2a4a', accent:'#60a5fa', icon:'🏃', desc:'Move the SAME piece TWICE this turn. First move must not cause check.' },
  { name:'Demote Him',     mechanic:'demotehim',       type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'📉', desc:"Lower any piece (yours or enemy) to weaker type. Not king." },
  { name:'Fake Piece',     mechanic:'fakepiece',       type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'👻', desc:'Place an illusion piece. Opponent cannot tell if real until captured.' },
  { name:'Teleport',       mechanic:'teleport',        type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'🌀', desc:'Move one of your pieces to any empty square. No check. Not king.' },
  { name:'Lava Ground',    mechanic:'lavaground',      type:'trap',  rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'🌋', desc:'Mark 1 square. Any piece there next turn is destroyed (not king).' },
  { name:'Radar',          mechanic:'radar',           type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'📡', desc:"See all opponent's cards for 1 turn." },
  { name:'Mirror',         mechanic:'mirror',          type:'trap',  rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'🪞', desc:"Copy opponent's last move with your equivalent piece, if legal." },
  { name:'Cheater',        mechanic:'cheater',         type:'spell', rarity:'rare',      color:'#1a2a4a', accent:'#60a5fa', icon:'💡', desc:'Engine reveals the best move to you.' },
  { name:'Invisible',      mechanic:'invisible',       type:'spell', rarity:'common',      color:'#1a2a4a', accent:'#60a5fa', icon:'👁️', desc:'One of your pieces becomes invisible for 1 round. It can jump to any square, even occupied ones. Giving check breaks invisibility. If it expires while on an enemy square, it dies.' },

  // EPIC
  { name:'Sniper',         mechanic:'sniper',          type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🎯', desc:'Remove ANY piece from the board (even yours). Not king. No check.' },
  { name:'Fortress',       mechanic:'fortress',        type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🏰', desc:'2×2 zone — enemies cannot enter for 2 turns.' },
  { name:'Clone',          mechanic:'clone',           type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🧬', desc:'Copy one of your pieces onto an adjacent empty square. No check. Not king.' },
  { name:'Borrow',         mechanic:'borrow',          type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🤏', desc:'Control one enemy piece for 1 turn. Not king. No check.' },
  { name:'Parasite',       mechanic:'parasite',        type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🦠', desc:'Link your piece to enemy piece. If yours dies → theirs dies too.' },
  { name:'Black Hole',     mechanic:'blackhole',       type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'🕳️', desc:'Choose 2 squares. After 2 turns all adjacent pieces explode. Kings immune.' },
  { name:'Big Sacrifice',  mechanic:'bigsacrifice',    type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'💎', desc:'Sacrifice one or more of your pieces totaling 14+ points to draw 3 cards. Click pieces then empty square to confirm.' },
  { name:'Undo',           mechanic:'undo',            type:'trap',  rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'↩️', desc:"Cancel the last card your opponent played." },
  { name:'Reverse',        mechanic:'reverse',         type:'trap',  rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'⏪', desc:"Undo opponent's last move." },
  { name:'Unabomber',      mechanic:'unabomber',       type:'spell', rarity:'epic',      color:'#2d1a4a', accent:'#c084fc', icon:'💣', desc:'Attach bomb to your piece. Next round it explodes destroying all adjacent pieces (kings immune). The piece itself is also destroyed.' },

  // LEGENDARY
  { name:'Mind Control',   mechanic:'mindcontrol',     type:'spell', rarity:'legendary', color:'#4a2a00', accent:'#f59e0b', icon:'🧠', desc:'Permanently steal one enemy piece. Not king. No check.' },
  { name:'Joker',          mechanic:'joker',           type:'spell', rarity:'common', color:'#4a2a00', accent:'#f59e0b', icon:'🃏', desc:'Choose any card from the full card pool — the Joker becomes it instantly.' },
];

export const RARITY_CUMULATIVE = (() => {
  let cum = 0;
  return (Object.entries(RARITY_WEIGHTS) as [Rarity, number][]).map(([rarity, weight]) => {
    cum += weight;
    return { rarity, threshold: cum };
  });
})();

export let _cardSeq = 0;
export const drawRandomCard = (ownerSuffix: string): GameCard => {
  const roll = Math.random() * 100;
  const chosen = RARITY_CUMULATIVE.find(({ threshold }) => roll < threshold);
  const chosenRarity: Rarity = chosen?.rarity ?? 'trash';
  const pool = CARD_POOL.filter(c => c.rarity === chosenRarity);
  const template = pool[Math.floor(Math.random() * pool.length)];
  const style = RARITY_STYLE[chosenRarity];
  _cardSeq++;
  return {
    ...template,
    id: `card_${_cardSeq}_${ownerSuffix}_${Date.now()}`,
    color: style.color,
    accent: style.accent,
  };
};

export const incrementCardSeq = (): number => { _cardSeq++; return _cardSeq; };