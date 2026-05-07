export type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn';
export type PieceColor = 'white' | 'black';
export type Rarity = 'trash' | 'common' | 'rare' | 'epic' | 'legendary';

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
  invisibleTurn?: number;
  invisibleOver?: boolean;
  fusedWith?: PieceType;
}

export interface Sq {
  row: number;
  col: number;
}

export type Board = (Piece | null)[][];

export type CardMechanic =
  | 'halffuse' | 'fullfusion' | 'swapme' | 'swapus' | 'swaphim'
  | 'sniper' | 'badsniper' | 'promote' | 'demote' | 'shield'
  | 'doublemove_diff' | 'doublemove_same' | 'fortress' | 'fog_village'
  | 'freeze' | 'jump' | 'teleport' | 'clone' | 'demotehim' | 'promotehim'
  | 'borrow' | 'mindcontrol' | 'parasite' | 'lavaground' | 'blackhole'
  | 'fakepiece' | 'gambler' | 'bigsacrifice' | 'smallsacrifice'
  | 'mirror' | 'radar' | 'undo' | 'joker' | 'reverse' | 'cheater' | 'unabomber' | 'invisible';

export interface CardDefinition {
  name: string;
  mechanic: CardMechanic;
  type: 'spell' | 'trap';
  rarity: Rarity;
  color: string;
  accent: string;
  icon: string;
  desc: string;
}

export interface GameCard extends CardDefinition {
  id: string;
}

export interface PendingCardState {
  cardId: string;
  mechanic: CardMechanic;
  ownerColor: PieceColor;
  target?: Sq | null;
  options?: string[] | null;
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

export interface LavaSquare {
  row: number;
  col: number;
  movesLeft: number;
}

export interface InvisiblePieceState {
  row: number;
  col: number;
  piece: Piece;
  ownerColor: PieceColor;
  roundsLeft: number;
}

export interface CheaterState {
  ownerColor: PieceColor;
  turnsLeft: number;
}

export interface BombPiece {
  row: number;
  col: number;
  turnsLeft: number;
  ownerColor: PieceColor;
}

export interface BlackHoleZone {
  sq1: Sq;
  sq2: Sq;
  turnsLeft: number;
  ownerColor: PieceColor;
}

export interface FogZone {
  centerRow: number;
  centerCol: number;
  turnsLeft: number;
  ownerColor: PieceColor;
}

export interface FortressZone {
  topRow: number;
  leftCol: number;
  turnsLeft: number;
  ownerColor: PieceColor;
}

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

export interface MatchClock {
  whiteMs: number;
  blackMs: number;
  runningFor: PieceColor | null;
  startedAtMs: number | null;
}

export interface ChatMessage {
  sender: PieceColor;
  text: string;
  sentAt: string;
}

export interface MatchState {
  matchId: string;
  rulesVersion: string;
  rngSeed: number;
  queue?: 'casual' | 'rated';
  whiteGuestId?: string;
  blackGuestId?: string;
  whiteAccountId?: string;
  blackAccountId?: string;
  whiteName?: string;
  blackName?: string;
  board: Board;
  lavaSquares?: LavaSquare[];
  bombPieces?: BombPiece[];
  blackHoles?: BlackHoleZone[];
  fogZones?: FogZone[];
  fortressZones?: FortressZone[];
  invisiblePiece?: InvisiblePieceState | null;
  cheaterState?: CheaterState | null;
  radarRevealFor?: PieceColor | null;
  doubleMove?: DoubleMove | null;
  undoAgainst?: PieceColor | null;
  turn: PieceColor;
  moved: string[];
  lastMove: { from: Sq; to: Sq } | null;
  halfMoveClock: number;
  fullMoveNumber: number;
  whiteHand: GameCard[];
  blackHand: GameCard[];
  moveHistory: string[];
  chatMessages: ChatMessage[];
  clock: MatchClock;
  status: 'waiting' | 'active' | 'finished';
  winner: PieceColor | 'draw' | 'aborted' | null;
  drawOfferedBy?: PieceColor | null;
  pendingCard?: PendingCardState | null;
}

export interface ReplayFrame {
  index: number;
  turn: PieceColor;
  board: Board;
  lastMove: { from: Sq; to: Sq } | null;
  halfMoveClock: number;
  fullMoveNumber: number;
  moveHistory: string[];
}

export interface ReplayLog {
  matchId: string;
  rulesVersion: string;
  rngSeed: number;
  createdAt: string;
  events: ResolvedEvent[];
}

export type PlayerIntent =
  | { type: 'make_move'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; from: Sq; to: Sq; promotion?: PieceType }
  | { type: 'play_card'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; cardId: string }
  | { type: 'select_target'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; target?: Sq; selectionId?: string }
  | { type: 'offer_draw'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string }
  | { type: 'respond_draw'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; accept: boolean }
  | { type: 'abort'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string }
  | { type: 'resign'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string }
  | { type: 'request_rematch'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string }
  | { type: 'send_chat'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; text: string };

export type MatchEventType =
  | 'match_started'
  | 'move_applied'
  | 'card_drawn'
  | 'card_played'
  | 'target_selected'
  | 'clock_updated'
  | 'chat_sent'
  | 'draw_offered'
  | 'draw_resolved'
  | 'match_finished'
  | 'snapshot_checkpoint';

export interface ResolvedEvent {
  id: string;
  matchId: string;
  type: MatchEventType;
  actorId?: string;
  at: string;
  payload: Record<string, unknown>;
}

export interface ClientEnvelope<T = unknown> {
  type: string;
  payload: T;
}

export interface MatchSnapshotMessage {
  match: MatchState;
  replayHead: number;
  replayFrames?: ReplayFrame[];
  events?: ResolvedEvent[];
}

export interface IntentRejectedMessage {
  intentType: PlayerIntent['type'];
  reason: string;
}

export interface QueueStatusMessage {
  queue: 'casual' | 'rated';
  status: 'queued' | 'matched' | 'cancelled';
  ticketId?: string;
}
