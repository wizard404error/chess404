// ─── Chess Types ──────────────────────────────────────────────────────────────
export type PieceType  = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn';
export type PieceColor = 'white' | 'black';

export interface Piece {
  type: PieceType;
  color: PieceColor;
  shielded?: boolean;
  shieldTurn?: number;
  frozen?: boolean;
  borrowed?: boolean;
  parasiteTarget?: string;
  bomb?: boolean;
  invisible?: boolean;
  invisibleTurn?: number; // fmn value when invisibility was granted — expires after 1 full round
  invisibleOver?: boolean; // true if invisible piece landed on enemy square — will die on expiry
  fusedWith?: PieceType;  // second piece type merged into this one (moves as union of both)
}

export interface Sq { row: number; col: number }
export type Board = (Piece | null)[][];

// ─── Card Types ───────────────────────────────────────────────────────────────
export type Rarity = 'trash' | 'common' | 'rare' | 'epic' | 'legendary';

export type CardMechanic =
  | 'halffuse' | 'fullfusion' | 'swapme' | 'swapus' | 'swaphim'
  | 'sniper' | 'badsniper' | 'promote' | 'demote' | 'shield'
  | 'doublemove_diff' | 'doublemove_same' | 'fortress' | 'fog_village'
  | 'freeze' | 'jump' | 'teleport' | 'clone' | 'demotehim' | 'promotehim'
  | 'borrow' | 'mindcontrol' | 'parasite' | 'lavaground' | 'blackhole'
  | 'fakepiece' | 'gambler' | 'bigsacrifice' | 'smallsacrifice'
  | 'mirror' | 'radar' | 'undo' | 'joker' | 'reverse' | 'cheater' | 'unabomber' | 'invisible';

export interface GameCard {
  id: string;
  name: string;
  mechanic: CardMechanic;
  type: 'spell' | 'trap';
  rarity: Rarity;
  color: string;
  accent: string;
  icon: string;
  desc: string;
}

export type CardPendingState = {
  card: GameCard;
  playerColor: PieceColor;
  mechanic: CardMechanic;
  step: number;
  data: Record<string, unknown>;
} | null;

export interface Snapshot {
  board: Board;
  turn: PieceColor;
  moved: Set<string>;
  lm: { from: Sq; to: Sq } | null;
  hmc: number;
  fmn: number;
  fen: string;
}

export interface DoubleMove {
  type: 'diff' | 'same';
  movesLeft: number;
  trackedSq: Sq | null;
  firstNote?: string;
}

export interface LavaSquare { row: number; col: number; movesLeft: number }

export interface BombPiece {
  row: number;
  col: number;
  turnsLeft: number;
  ownerColor: PieceColor;
}

export interface FogZone {
  squares: Array<{ row: number; col: number }>;
  turnsLeft: number;
  ownerColor: PieceColor;
}

// ─── Card Animation ───────────────────────────────────────────────────────────
export type CardAnimType =
  | 'shield'
  | 'gambler_win'
  | 'gambler_lose'
  | 'reverse'
  | 'freeze'
  | 'bomb_explode'
  | 'lava_kill'
  | 'swap'
  | 'teleport'
  | 'mindcontrol'
  | 'clone'
  | 'sniper'
  | 'bigsacrifice'
  | 'smallsacrifice'
  | 'blackhole'
  | 'fullfusion'
  | null;