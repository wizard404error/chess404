import type { CardMechanic, PieceColor, PieceType, Rarity } from '@chess404/contracts';

export const RULES_VERSION = 'v1-alpha-foundation';
export const FILES = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'] as const;
export const RANKS = ['1', '2', '3', '4', '5', '6', '7', '8'] as const;
export const SQ = 68;
export const MAX_HAND_SIZE = 10;
export const CLOCK_START = 10 * 60;
export const ABORT_SECS = 10;
export const DRAW_FROM = 8;
export const DRAW_EVERY = 3;
export const INITIAL_DEAL_ROUND = 2;

export const PIECE_VALUE: Readonly<Record<PieceType, number>> = {
  pawn: 1,
  knight: 3,
  bishop: 3,
  rook: 5,
  queen: 9,
  king: 99
};

export const UPGRADE: Readonly<Partial<Record<PieceType, PieceType[]>>> = {
  pawn: ['knight', 'bishop', 'rook', 'queen'],
  knight: ['bishop', 'rook', 'queen'],
  bishop: ['knight', 'rook', 'queen'],
  rook: ['queen']
};

export const DOWNGRADE: Readonly<Partial<Record<PieceType, PieceType[]>>> = {
  queen: ['rook', 'bishop', 'knight', 'pawn'],
  rook: ['bishop', 'knight', 'pawn'],
  bishop: ['knight', 'pawn'],
  knight: ['pawn']
};

export const OPP: Readonly<Record<PieceColor, PieceColor>> = { white: 'black', black: 'white' };

export const RARITY_WEIGHTS: Readonly<Record<Rarity, number>> = {
  trash: 5,
  common: 40,
  rare: 30,
  epic: 20,
  legendary: 5
};

export const RARITY_STYLE: Readonly<Record<Rarity, { color: string; accent: string; glow: string; label: string }>> = {
  trash: { color: '#1c1c1c', accent: '#6b7280', glow: 'rgba(107,114,128,0.4)', label: 'TRASH' },
  common: { color: '#1a2e1a', accent: '#4ade80', glow: 'rgba(74,222,128,0.4)', label: 'COMMON' },
  rare: { color: '#1a2a4a', accent: '#60a5fa', glow: 'rgba(96,165,250,0.5)', label: 'RARE' },
  epic: { color: '#2d1a4a', accent: '#c084fc', glow: 'rgba(192,132,252,0.6)', label: 'EPIC' },
  legendary: { color: '#4a2a00', accent: '#f59e0b', glow: 'rgba(245,158,11,0.8)', label: 'LEGENDARY' }
};

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
  'halffuse', 'fullfusion'
]);

export const CARD_TARGET_MESSAGES: Partial<Record<CardMechanic, string>> = {
  freeze: 'Click an enemy piece (not king) to freeze it',
  shield: 'Click your piece (not king) to shield it',
  sniper: 'Click any piece (not king) to remove it',
  badsniper: 'Click your piece (not king) to remove it',
  promote: 'Click your piece to promote (not king)',
  demote: 'Click your piece to demote (not king)',
  jump: 'Click your piece to jump (not king)',
  teleport: 'Click your piece to teleport (not king)',
  swapme: 'Click the first of your pieces to swap (not king)',
  swapus: 'Click your piece to swap with enemy (not king)',
  swaphim: 'Click first enemy piece to swap (not king)',
  clone: 'Click your piece to clone (not king)',
  mindcontrol: 'Click an enemy piece to steal permanently (not king)',
  borrow: 'Click an enemy piece to control for 1 turn (not king)',
  demotehim: 'Click any piece to demote (not king)',
  promotehim: 'Click an enemy piece to promote (not king)',
  smallsacrifice: 'Click your pieces to sacrifice, then an empty square to confirm',
  bigsacrifice: 'Click your pieces to sacrifice, then an empty square to confirm',
  lavaground: 'Click any square to place lava trap',
  blackhole: 'Click first square for black hole',
  unabomber: 'Click your piece to attach a bomb (not king)',
  fakepiece: 'Click an empty square to place a fake piece',
  parasite: 'Click your piece to be the host (not king)',
  fog_village: 'Click any square to place the Fog Village',
  halffuse: 'Click your first piece to fuse (not king)',
  fullfusion: 'Click your first piece to fuse (not king)'
};
