'use client';

import React from 'react';
import type {
  Board, PieceColor, PieceType, Piece, Sq, GameCard, CardMechanic, CardPendingState,
  DoubleMove, LavaSquare, BombPiece, Rarity, Snapshot,
} from '../types';
import { drawRandomCard } from '../cardPool';
import { OPP, MAX_HAND_SIZE, INITIAL_DEAL_ROUND, DRAW_FROM, DRAW_EVERY } from '../constants';
import { findKing, isAttacked, legalMoves, cloneBoard } from '../chessEngine';

export function useCardEngine(
  board: Board,
  turn: PieceColor,
  moved: Set<string>,
  lm: { from: Sq; to: Sq } | null,
  fmn: number,
  fmnRef: React.MutableRefObject<number>,
  boardRef: React.MutableRefObject<Board>,
  turnRef: React.MutableRefObject<PieceColor>,
  authoritativeMatchId: string | null,
  hostedRuntime: boolean | null,
  viewerSeatRef: React.MutableRefObject<PieceColor | null>,
) {
  const [whiteHand,    setWhiteHand]    = React.useState<GameCard[]>([]);
  const [blackHand,    setBlackHand]    = React.useState<GameCard[]>([]);
  const [selectedCard, setSelectedCard] = React.useState<GameCard | null>(null);
  const [dealPhase,    setDealPhase]    = React.useState<'idle'|'dealing'|'done'>('idle');
  const [lastDrawAnim, setLastDrawAnim] = React.useState<{ color: PieceColor; rarity: Rarity } | null>(null);
  const [cardPending,  setCardPending]  = React.useState<CardPendingState>(null);
  const [cardMsg,      setCardMsg]      = React.useState<string>('');
  const [promoPicker,  setPromoPicker]  = React.useState<{ sq: Sq; options: PieceType[]; mechanic: CardMechanic } | null>(null);
  const [cardPromo, setCardPromo] = React.useState<{ sq: Sq; color: PieceColor } | null>(null);
  const [cardUsedBy,   setCardUsedBy]   = React.useState<{ white: boolean; black: boolean }>({ white: false, black: false });

  const [jokerPicker, setJokerPicker] = React.useState<{
    card: GameCard;
    playerColor: PieceColor;
    filterRarity: Rarity | 'all';
    transforming: boolean;
  } | null>(null);

  const pendingCardUseRef = React.useRef<Set<string>>(new Set());
  const cardUsedByRef     = React.useRef<{ white: boolean; black: boolean }>({ white: false, black: false });
  const cardMsgTimerRef   = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  React.useEffect(() => {
    return () => {
      if (cardMsgTimerRef.current) clearTimeout(cardMsgTimerRef.current);
    };
  }, []);

  const lastDrawRound = React.useRef(0);
  const roundNumber   = React.useMemo(() => Math.floor(fmn), [fmn]);

  // ── Card draw logic ──────────────────────────────────────────────────────
  React.useEffect(() => {
    if (dealPhase !== 'done') return;
    if (authoritativeMatchId) return;
    if (lastDrawRound.current === roundNumber) return;

    if (roundNumber === INITIAL_DEAL_ROUND) {
      lastDrawRound.current = roundNumber;
      setWhiteHand(Array.from({ length: 3 }, () => drawRandomCard('w')));
      setBlackHand(Array.from({ length: 3 }, () => drawRandomCard('b')));
      setLastDrawAnim({ color: 'white', rarity: 'trash' });
      setTimeout(() => setLastDrawAnim(null), 3000);
      return;
    }

    if (roundNumber < DRAW_FROM) return;
    if ((roundNumber - DRAW_FROM) % DRAW_EVERY !== 0) return;

    lastDrawRound.current = roundNumber;
    const hasFused = board.some(r => r.some(p => p?.fusedWith));
    const drawSafe = (side: string) => {
      let card = drawRandomCard(side);
      let attempts = 0;
      while (hasFused && card.mechanic === 'cheater' && attempts < 20) {
        card = drawRandomCard(side); attempts++;
      }
      return card;
    };
    const wCard = drawSafe('w');
    const bCard = drawSafe('b');
    setWhiteHand(h => h.length < MAX_HAND_SIZE ? [...h, wCard] : h);
    setBlackHand(h => h.length < MAX_HAND_SIZE ? [...h, bCard] : h);
    setLastDrawAnim({ color: 'white', rarity: wCard.rarity });
    setTimeout(() => setLastDrawAnim(null), 2000);
  }, [roundNumber, dealPhase, authoritativeMatchId, board]);

  const resetCardUsed = React.useCallback((nextTurn: PieceColor) => {
    cardUsedByRef.current = { ...cardUsedByRef.current, [nextTurn]: false };
    setCardUsedBy(prev => ({ ...prev, [nextTurn]: false }));
  }, []);

  const removeCardFromHand = React.useCallback((card: GameCard, color: PieceColor) => {
    if (color === 'white') setWhiteHand(h => h.filter(c => c.id !== card.id));
    else setBlackHand(h => h.filter(c => c.id !== card.id));
  }, []);

  const finishCardUse = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    removeCardFromHand(card, playerColor);
    cardUsedByRef.current = { ...cardUsedByRef.current, [playerColor]: true };
    setCardUsedBy(prev => ({ ...prev, [playerColor]: true }));
    setSelectedCard(null);
    pendingCardUseRef.current.delete(card.id);
  }, [removeCardFromHand]);

  const cancelCard = React.useCallback(() => {
    setCardPending(null);
    setPromoPicker(null);
    setCardMsg('');
    if (selectedCard) pendingCardUseRef.current.delete(selectedCard.id);
    setSelectedCard(null);
  }, [selectedCard]);

  const isAttackedWithFusion = React.useCallback((b: Board, row: number, col: number, byColor: PieceColor): boolean => {
    if (isAttacked(b, row, col, byColor)) return true;
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = b[r][c];
        if (!p || p.color !== byColor || !p.fusedWith) continue;
        const tempBoard: Board = b.map(row2 => row2.map(p2 => p2 ? { ...p2 } : null));
        tempBoard[r][c] = { ...p, type: p.fusedWith, fusedWith: undefined };
        if (isAttacked(tempBoard, row, col, byColor)) return true;
      }
    }
    return false;
  }, []);

  const getFusedMoves = React.useCallback((
    b: Board, r: number, c: number, typeA: PieceType, typeB: PieceType,
  ): Sq[] => {
    const b1 = b.map(row => row.map(p => p ? { ...p } : null));
    b1[r][c] = { ...b1[r][c]!, type: typeA, fusedWith: undefined };
    const m1 = legalMoves(b1, r, c, lm, moved);
    const b2 = b.map(row => row.map(p => p ? { ...p } : null));
    b2[r][c] = { ...b2[r][c]!, type: typeB, fusedWith: undefined };
    const m2 = legalMoves(b2, r, c, lm, moved);
    const seen = new Set<string>();
    return [...m1, ...m2].filter(sq => {
      const key = `${sq.row},${sq.col}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }, [lm, moved]);

  const checkFusionRedundancy = React.useCallback((
    typeA: PieceType, typeB: PieceType, sqA: Sq, sqB: Sq,
  ): string => {
    if (typeA === typeB && (sqA.row !== sqB.row || sqA.col !== sqB.col)) return 'duplicate';
    return 'none';
  }, []);

  const getSafeTransforms = React.useCallback((
    b: Board, target: Sq, mechanic: CardMechanic, ownerColor: PieceColor,
  ): PieceType[] => {
    const piece = b[target.row][target.col];
    if (!piece) return [];
    const isPromotion = mechanic === 'promote' || mechanic === 'promotehim';
    const options = isPromotion
      ? ['queen', 'rook', 'bishop', 'knight'] as PieceType[]
      : ['pawn'] as PieceType[];
    return options.filter(t => {
      const nb = cloneBoard(b);
      nb[target.row][target.col] = { ...piece, type: t };
      const kp = findKing(nb, ownerColor);
      if (!kp) return true;
      const opp = OPP[ownerColor];
      return !isAttackedWithFusion(nb, kp.row, kp.col, opp);
    });
  }, [isAttackedWithFusion]);

  const [doubleMove, setDoubleMove] = React.useState<DoubleMove | null>(null);
  const doubleMoveRef = React.useRef<DoubleMove | null>(null);
  React.useEffect(() => { doubleMoveRef.current = doubleMove; }, [doubleMove]);

  const activateDoubleMove = React.useCallback((type: 'diff' | 'same') => {
    const newDm: DoubleMove = { type, movesLeft: 2, trackedSq: null };
    doubleMoveRef.current = newDm;
    setDoubleMove(newDm);
    setCardMsg(type === 'diff'
      ? 'Twin active! Make your first move, then move a different piece.'
      : 'Solo active! Make your first move, then move the same piece again.'
    );
    setTimeout(() => setCardMsg(''), 2000);
  }, []);

  const openJokerPicker = React.useCallback((card: GameCard, playerColor: PieceColor) => {
    setJokerPicker({
      card,
      playerColor: playerColor,
      filterRarity: 'all',
      transforming: false,
    });
  }, []);

  const applyJokerTransform = React.useCallback((
    target: GameCard, jokerCard: GameCard, playerColor: PieceColor,
  ) => {
    const transformed: GameCard = { ...jokerCard, ...target, id: jokerCard.id };
    if (playerColor === 'white') setWhiteHand(h => h.map(c => c.id === jokerCard.id ? transformed : c));
    else setBlackHand(h => h.map(c => c.id === jokerCard.id ? transformed : c));
    setJokerPicker(null);
    setSelectedCard(null);
  }, []);

  return {
    whiteHand, setWhiteHand,
    blackHand, setBlackHand,
    selectedCard, setSelectedCard,
    dealPhase, setDealPhase,
    lastDrawAnim, setLastDrawAnim,
    cardPending, setCardPending,
    cardMsg, setCardMsg,
    promoPicker, setPromoPicker,
    cardPromo, setCardPromo,
    cardUsedBy, setCardUsedBy,
    jokerPicker, setJokerPicker,
    doubleMove, setDoubleMove,
    doubleMoveRef,
    cardUsedByRef,
    pendingCardUseRef,
    lastDrawRound,
    roundNumber,
    resetCardUsed,
    removeCardFromHand,
    finishCardUse,
    cancelCard,
    isAttackedWithFusion,
    getFusedMoves,
    checkFusionRedundancy,
    getSafeTransforms,
    activateDoubleMove,
    openJokerPicker,
    applyJokerTransform,
  };
}
