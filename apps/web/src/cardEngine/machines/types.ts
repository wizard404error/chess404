import type { Board, PieceColor, PieceType, Sq, CardMechanic, GameCard } from '../../types';

export type CardContext = {
  board: Board;
  turn: PieceColor;
  playerColor: PieceColor;
  card: GameCard;
  row: number;
  col: number;
  piece: { type: PieceType; color: PieceColor } | null;
  step: number;
  data: Record<string, unknown>;
};

export type CardResult = {
  board?: Board;
  cardMsg?: string;
  cardMsgTimeout?: number;
  anim?: string;
  animLabel?: string;
  finishCard?: boolean;
};

export type CardHandler = (ctx: CardContext) => CardResult | null | undefined;
