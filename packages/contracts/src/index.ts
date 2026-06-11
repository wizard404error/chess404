export type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn';
export type PieceColor = 'white' | 'black';
export type Rarity = 'trash' | 'common' | 'rare' | 'epic' | 'legendary';
export type MatchModeId = 'open_cards' | 'hidden_cards' | 'computer';

export interface MatchModeDefinition {
  id: MatchModeId;
  label: string;
  shortLabel: string;
  rulesSummary: string;
}

export const DEFAULT_MATCH_MODE_ID: MatchModeId = 'open_cards';

export const OFFICIAL_MATCH_MODES: readonly MatchModeDefinition[] = [
  {
    id: 'open_cards',
    label: 'Open Cards',
    shortLabel: 'Open',
    rulesSummary: 'Both players can see the public state and card information.',
  },
  {
    id: 'hidden_cards',
    label: 'Hidden Cards',
    shortLabel: 'Hidden',
    rulesSummary: 'Card plans stay concealed to create a bluff-heavy competitive mode.',
  },
  {
    id: 'computer',
    label: 'Play vs Computer',
    shortLabel: 'Computer',
    rulesSummary: 'Play against the built-in chess engine with card effects.',
  },
] as const;

export interface Piece {
  type: PieceType;
  color: PieceColor;
  fake?: boolean;
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

export type MatchFinishReason =
  | 'checkmate'
  | 'stalemate'
  | 'insufficient_material'
  | 'threefold_repetition'
  | 'fifty_move_rule'
  | 'timeout'
  | 'abandon'
  | 'resign'
  | 'abort'
  | 'draw_agreement';

export interface MatchState {
  matchId: string;
  rulesVersion: string;
  rngSeed: number;
  queue?: 'casual' | 'rated' | 'direct';
  modeId?: MatchModeId;
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
  whiteConnected: boolean;
  blackConnected: boolean;
  disconnectGraceFor?: PieceColor | null;
  disconnectGraceDeadline?: string | null;
  status: 'waiting' | 'active' | 'finished';
  winner: PieceColor | 'draw' | 'aborted' | null;
  finishReason?: MatchFinishReason | null;
  drawOfferedBy?: PieceColor | null;
  pendingCard?: PendingCardState | null;
}

export interface MatchPresenceRequest {
  playerId: string;
  playerSecret?: string;
  playerClaimToken?: string;
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
  | { type: 'make_move'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; from: Sq; to: Sq; promotion?: PieceType; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'play_card'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; cardId: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'select_target'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; target?: Sq; selectionId?: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'offer_draw'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'respond_draw'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; accept: boolean; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'abort'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'resign'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'request_rematch'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; clientMoveId?: string; expectedSeqNum?: number }
  | { type: 'send_chat'; matchId: string; playerId: string; playerSecret?: string; playerClaimToken?: string; text: string; clientMoveId?: string; expectedSeqNum?: number };

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
  seqNum?: number;
}

export interface IntentRejectedMessage {
  intentType: PlayerIntent['type'];
  reason: string;
}

export interface QueueStatusMessage {
  queue: 'casual' | 'rated';
  modeId?: MatchModeId;
  status: 'queued' | 'matched' | 'cancelled';
  ticketId?: string;
}
