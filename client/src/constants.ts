import type { PieceType, PieceColor, Rarity, CardMechanic } from './types';

// ─── Constants ────────────────────────────────────────────────────────────────
export const FILES = ['a','b','c','d','e','f','g','h'] as const;
export const RANKS = ['1','2','3','4','5','6','7','8'] as const;
export const SQ    = 68;
export const MAX_HAND_SIZE  = 10;
export const CLOCK_START    = 10 * 60;
export const ABORT_SECS     = 10;
export const DRAW_FROM      = 8;
export const DRAW_EVERY     = 3;
export const INITIAL_DEAL_ROUND = 2;

export const PIECE_VALUE: Readonly<Record<PieceType, number>> = {
  pawn: 1, knight: 3, bishop: 3, rook: 5, queen: 9, king: 99,
};

export const UPGRADE: Readonly<Partial<Record<PieceType, PieceType[]>>> = {
  pawn:   ['knight','bishop','rook','queen'],
  knight: ['bishop','rook','queen'],
  bishop: ['knight','rook','queen'],
  rook:   ['queen'],
};

export const DOWNGRADE: Readonly<Partial<Record<PieceType, PieceType[]>>> = {
  queen:  ['rook','bishop','knight','pawn'],
  rook:   ['bishop','knight','pawn'],
  bishop: ['knight','pawn'],
  knight: ['pawn'],
};

export const OPP: Readonly<Record<PieceColor, PieceColor>> = { white: 'black', black: 'white' };

export const RARITY_WEIGHTS: Readonly<Record<Rarity, number>> = {
  trash: 5, common: 40, rare: 30, epic: 20, legendary: 5,
};


export const RARITY_STYLE: Readonly<Record<Rarity, { color: string; accent: string; glow: string; label: string }>> = {
  trash:     { color:'#1c1c1c', accent:'#6b7280', glow:'rgba(107,114,128,0.4)',  label:'TRASH'     },
  common:    { color:'#1a2e1a', accent:'#4ade80', glow:'rgba(74,222,128,0.4)',   label:'COMMON'    },
  rare:      { color:'#1a2a4a', accent:'#60a5fa', glow:'rgba(96,165,250,0.5)',   label:'RARE'      },
  epic:      { color:'#2d1a4a', accent:'#c084fc', glow:'rgba(192,132,252,0.6)',  label:'EPIC'      },
  legendary: { color:'#4a2a00', accent:'#f59e0b', glow:'rgba(245,158,11,0.8)',   label:'LEGENDARY' },
};


// ─── Target-requiring card mechanics ─────────────────────────────────────────
export const TARGETING_CARDS = new Set<CardMechanic>([
  'freeze', 'shield', 'sniper', 'badsniper',
  'promote', 'demote', 'jump', 'teleport',
  'doublemove_diff', 'doublemove_same',
  'swapme', 'swapus', 'swaphim',
  'clone', 'mindcontrol', 'borrow',
  'demotehim', 'promotehim',
  'smallsacrifice', 'bigsacrifice',
  'lavaground', 'blackhole', 'unabomber',
  'fakepiece', 'parasite', 'fog_village',
  'halffuse', 'fullfusion',
]);

export const CARD_TARGET_MESSAGES: Partial<Record<CardMechanic, string>> = {
  freeze:         '❄️ Click an ENEMY piece (not king) to freeze it',
  shield:         '🛡️ Click YOUR piece (not king) to shield it',
  sniper:         '🎯 Click any piece (not king) to remove it',
  badsniper:      '🔫 Click YOUR piece (not king) to remove...',
  promote:        '⬆️ Click YOUR piece to promote (not king)',
  demote:         '⬇️ Click YOUR piece to demote (not king)',
  jump:           '🦘 Click YOUR piece to jump (not king)',
  teleport:       '✨ Click YOUR piece to teleport (not king)',
  swapme:         '🔄 Click the FIRST of your pieces to swap (not king)',
  swapus:         '↔️ Click YOUR piece to swap with enemy (not king)',
  swaphim:        '🔁 Click FIRST enemy piece to swap (not king)',
  clone:          '🧬 Click YOUR piece to clone (not king)',
  mindcontrol:    '🧠 Click an ENEMY piece to steal permanently (not king)',
  borrow:         '🤏 Click an ENEMY piece to control for 1 turn (not king)',
  demotehim:      '📉 Click ANY piece to demote (not king)',
  promotehim:     '📈 Click an ENEMY piece to promote (not king)',
  smallsacrifice: '🩸 Click YOUR pieces to sacrifice (total 6+ pts). Empty square to confirm.',
  bigsacrifice:   '💎 Click YOUR pieces to sacrifice (total 14+ pts). Empty square to confirm.',
  lavaground:     '🌋 Click any square to place lava trap',
  blackhole:      '🕳️ Click first square for black hole (2 squares total)',
  unabomber:      '💣 Click YOUR piece to attach a bomb (not king)',
  fakepiece:      '👻 Click an empty square to place a fake piece',
  parasite:       '🦠 Click YOUR piece to be the host (not king)',
  fog_village:     '🌫️ Click any square to place the Fog Village (covers 3×3 zone)',
  halffuse:       '⚗️ Click YOUR first piece to fuse (not king)',
  fullfusion:     '🔮 Click YOUR first piece to fuse (not king, no point limit)',
};