import type { MatchState, PieceColor, PlayerIntent, ResolvedEvent, Sq } from '@chess404/contracts';
import { RULES_VERSION, CLOCK_START, OPP } from './constants';
import { createMulberry32 } from './rng';
import { cloneBoard, legalMoves, makeBoard, moveNotation } from './chess-engine';

export interface ApplyIntentResult {
  match: MatchState;
  events: ResolvedEvent[];
}

export const createMatchState = (params?: {
  matchId?: string;
  seed?: number;
  clockSeconds?: number;
  startedAtMs?: number;
}): MatchState => {
  const startedAtMs = params?.startedAtMs ?? Date.now();
  const seed = params?.seed ?? startedAtMs;
  const rng = createMulberry32(seed);
  const matchId = params?.matchId ?? `match_${startedAtMs}`;
  const clockMs = (params?.clockSeconds ?? CLOCK_START) * 1000;

  return {
    matchId,
    rulesVersion: RULES_VERSION,
    rngSeed: Math.floor(rng.next() * 1_000_000_000),
    board: makeBoard(),
    turn: 'white',
    moved: [],
    lastMove: null,
    halfMoveClock: 0,
    fullMoveNumber: 1,
    whiteHand: [],
    blackHand: [],
    moveHistory: [],
    chatMessages: [],
    clock: {
      whiteMs: clockMs,
      blackMs: clockMs,
      runningFor: 'white',
      startedAtMs
    },
    status: 'active',
    winner: null,
    drawOfferedBy: null
  };
};

export const applyPlayerIntent = (match: MatchState, intent: PlayerIntent, now = new Date()): ApplyIntentResult => {
  switch (intent.type) {
    case 'make_move':
      return applyMoveIntent(match, intent, now);
    case 'send_chat':
      return applyChatIntent(match, intent, now);
    case 'offer_draw':
      return applyOfferDrawIntent(match, intent, now);
    case 'respond_draw':
      return applyRespondDrawIntent(match, intent, now);
    case 'resign':
      return applyResignIntent(match, intent, now);
    default:
      return {
        match,
        events: [makeEvent(match.matchId, 'snapshot_checkpoint', now, intent.playerId, { ignoredIntent: intent.type })]
      };
  }
};

const applyMoveIntent = (
  match: MatchState,
  intent: Extract<PlayerIntent, { type: 'make_move' }>,
  now: Date
): ApplyIntentResult => {
  ensureActive(match);
  const piece = match.board[intent.from.row]?.[intent.from.col];
  if (!piece) {
    throw new Error('No piece on source square');
  }
  if (piece.color !== match.turn) {
    throw new Error('Cannot move out of turn');
  }

  const legal = legalMoves(
    match.board,
    intent.from.row,
    intent.from.col,
    match.lastMove,
    new Set(match.moved)
  );

  const isLegal = legal.some((move) => move.row === intent.to.row && move.col === intent.to.col);
  if (!isLegal) {
    throw new Error('Illegal move');
  }

  const nextBoard = cloneBoard(match.board);
  const movingPiece = nextBoard[intent.from.row][intent.from.col];
  const capturedPiece = nextBoard[intent.to.row][intent.to.col];
  nextBoard[intent.to.row][intent.to.col] = movingPiece;
  nextBoard[intent.from.row][intent.from.col] = null;

  if (movingPiece?.type === 'pawn' && intent.to.col !== intent.from.col && !capturedPiece) {
    nextBoard[intent.from.row][intent.to.col] = null;
  }

  const notation = moveNotation(
    match.board,
    intent.from.row,
    intent.from.col,
    intent.to.row,
    intent.to.col,
    piece,
    Boolean(capturedPiece)
  );

  const nextTurn: PieceColor = OPP[match.turn];
  const nextHalfMoveClock = piece.type === 'pawn' || capturedPiece ? 0 : match.halfMoveClock + 1;
  const nextFullMoveNumber = match.turn === 'black' ? match.fullMoveNumber + 1 : match.fullMoveNumber;

  const nextMatch: MatchState = {
    ...match,
    board: nextBoard,
    turn: nextTurn,
    moved: [...match.moved, `${intent.from.row}-${intent.from.col}`],
    lastMove: { from: intent.from, to: intent.to },
    halfMoveClock: nextHalfMoveClock,
    fullMoveNumber: nextFullMoveNumber,
    moveHistory: [...match.moveHistory, notation],
    drawOfferedBy: null,
    clock: {
      ...match.clock,
      runningFor: nextTurn,
      startedAtMs: now.getTime()
    }
  };

  return {
    match: nextMatch,
    events: [
      makeEvent(match.matchId, 'move_applied', now, intent.playerId, {
        from: intent.from,
        to: intent.to,
        notation,
        nextTurn
      }),
      makeEvent(match.matchId, 'clock_updated', now, intent.playerId, {
        runningFor: nextTurn
      })
    ]
  };
};

const applyChatIntent = (
  match: MatchState,
  intent: Extract<PlayerIntent, { type: 'send_chat' }>,
  now: Date
): ApplyIntentResult => {
  const sender = inferColor(intent.playerId, match.turn);
  const nextMatch: MatchState = {
    ...match,
    chatMessages: [
      ...match.chatMessages,
      {
        sender,
        text: intent.text.trim(),
        sentAt: now.toISOString()
      }
    ]
  };

  return {
    match: nextMatch,
    events: [
      makeEvent(match.matchId, 'chat_sent', now, intent.playerId, {
        sender,
        text: intent.text.trim()
      })
    ]
  };
};

const applyOfferDrawIntent = (
  match: MatchState,
  intent: Extract<PlayerIntent, { type: 'offer_draw' }>,
  now: Date
): ApplyIntentResult => {
  ensureActive(match);
  const offeredBy = inferColor(intent.playerId, match.turn);
  const nextMatch: MatchState = {
    ...match,
    drawOfferedBy: offeredBy
  };

  return {
    match: nextMatch,
    events: [
      makeEvent(match.matchId, 'draw_offered', now, intent.playerId, {
        offeredBy
      })
    ]
  };
};

const applyRespondDrawIntent = (
  match: MatchState,
  intent: Extract<PlayerIntent, { type: 'respond_draw' }>,
  now: Date
): ApplyIntentResult => {
  ensureActive(match);
  if (!match.drawOfferedBy) {
    throw new Error('No draw offer to respond to');
  }

  if (intent.accept) {
    const nextMatch: MatchState = {
      ...match,
      status: 'finished',
      winner: 'draw',
      drawOfferedBy: null,
      clock: {
        ...match.clock,
        runningFor: null,
        startedAtMs: null
      }
    };

    return {
      match: nextMatch,
      events: [
        makeEvent(match.matchId, 'draw_resolved', now, intent.playerId, { accept: true }),
        makeEvent(match.matchId, 'match_finished', now, intent.playerId, { result: 'draw' })
      ]
    };
  }

  const nextMatch: MatchState = {
    ...match,
    drawOfferedBy: null
  };

  return {
    match: nextMatch,
    events: [
      makeEvent(match.matchId, 'draw_resolved', now, intent.playerId, { accept: false })
    ]
  };
};

const applyResignIntent = (
  match: MatchState,
  intent: Extract<PlayerIntent, { type: 'resign' }>,
  now: Date
): ApplyIntentResult => {
  ensureActive(match);
  const resigningColor = inferColor(intent.playerId, match.turn);
  const winner: PieceColor = OPP[resigningColor];

  const nextMatch: MatchState = {
    ...match,
    status: 'finished',
    winner,
    drawOfferedBy: null,
    clock: {
      ...match.clock,
      runningFor: null,
      startedAtMs: null
    }
  };

  return {
    match: nextMatch,
    events: [
      makeEvent(match.matchId, 'match_finished', now, intent.playerId, {
        result: 'resign',
        winner
      })
    ]
  };
};

const makeEvent = (
  matchId: string,
  type: ResolvedEvent['type'],
  now: Date,
  actorId?: string,
  payload: Record<string, unknown> = {}
): ResolvedEvent => ({
  id: `${matchId}_${type}_${now.getTime()}`,
  matchId,
  type,
  actorId,
  at: now.toISOString(),
  payload
});

const inferColor = (playerId: string, fallback: PieceColor): PieceColor => {
  if (playerId.toLowerCase().includes('black')) return 'black';
  if (playerId.toLowerCase().includes('white')) return 'white';
  return fallback;
};

const ensureActive = (match: MatchState) => {
  if (match.status !== 'active') {
    throw new Error('Match is not active');
  }
};

export const sameSquare = (a: Sq, b: Sq): boolean => a.row === b.row && a.col === b.col;
