'use client';

import React from 'react';
import type { PieceColor, PieceType, Piece, Sq, Board, BombPiece, LavaSquare, FogZone, CardAnimType } from '../types';
import { cloneBoard, inB } from '../chessEngine';
import { FILES, RANKS } from '../constants';

interface UseMatchBoardEffectsParams {
  setBoard: React.Dispatch<React.SetStateAction<Board>>;
  setCardMsg: (msg: string) => void;
  fireCardAnim: (type: CardAnimType, lbl?: string) => void;
  bombPiecesRef: React.MutableRefObject<BombPiece[]>;
  setBombPieces: React.Dispatch<React.SetStateAction<BombPiece[]>>;
  setBombExploding: React.Dispatch<React.SetStateAction<Sq[]>>;
}

interface GhostPiece {
  row: number;
  col: number;
  piece: Piece;
  ownerColor: PieceColor;
  roundsLeft: number;
}

export function useMatchBoardEffects({
  setBoard,
  setCardMsg,
  fireCardAnim,
  bombPiecesRef,
  setBombPieces,
  setBombExploding,
}: UseMatchBoardEffectsParams) {
  const [lavaSquares, setLavaSquares] = React.useState<LavaSquare[]>([]);
  const [lavaExploding, setLavaExploding] = React.useState<Sq[]>([]);
  const lavaSquaresRef = React.useRef(lavaSquares);
  React.useEffect(() => { lavaSquaresRef.current = lavaSquares; }, [lavaSquares]);

  const [ghostPiece, setGhostPiece] = React.useState<GhostPiece | null>(null);
  const ghostRef = React.useRef<GhostPiece | null>(null);
  React.useEffect(() => { ghostRef.current = ghostPiece; }, [ghostPiece]);

  const [fogZones, setFogZones] = React.useState<FogZone[]>([]);

  const processBombs = React.useCallback((currentTurn: PieceColor, currentBoard: Board) => {
    const bombs = bombPiecesRef.current;
    if (bombs.length === 0) return currentBoard;

    const shouldDecrement = currentTurn === 'white';

    const updatedBombs: BombPiece[] = [];
    const nb = currentBoard;
    const newExplodingSqs: Sq[] = [];

    for (const bomb of bombs) {
      const p = nb[bomb.row]?.[bomb.col];
      const hasBombAtTracked = p?.bomb === true;

      let foundRow = -1, foundCol = -1;
      if (hasBombAtTracked) {
        foundRow = bomb.row; foundCol = bomb.col;
      } else {
        outer: for (let r = 0; r < 8; r++) {
          for (let c = 0; c < 8; c++) {
            if (nb[r][c]?.bomb === true && nb[r][c]?.color === bomb.ownerColor) {
              foundRow = r; foundCol = c; break outer;
            }
          }
        }
      }

      const newTurnsLeft = shouldDecrement ? bomb.turnsLeft - 1 : bomb.turnsLeft;

      if (newTurnsLeft <= 0 && foundRow >= 0) {
        const explodeCenter = { row: foundRow, col: foundCol };
        newExplodingSqs.push(explodeCenter);

        for (let dr = -1; dr <= 1; dr++) {
          for (let dc = -1; dc <= 1; dc++) {
            const r = foundRow + dr, c = foundCol + dc;
            if (!inB(r, c)) continue;
            const target = nb[r][c];
            if (target && target.type !== 'king') {
              newExplodingSqs.push({ row: r, col: c });
            }
          }
        }
      } else if (foundRow >= 0) {
        updatedBombs.push({ ...bomb, row: foundRow, col: foundCol, turnsLeft: newTurnsLeft });
      }
    }

    if (newExplodingSqs.length > 0) {
      const uniqueSqs = newExplodingSqs.filter((s, i, arr) =>
        arr.findIndex(x => x.row === s.row && x.col === s.col) === i
      );
      setBombExploding(uniqueSqs);
      fireCardAnim('bomb_explode', '💥 Bomb detonated!');

      setTimeout(() => {
        setBoard(b2 => {
          const explodedBoard = cloneBoard(b2);
          for (const sq of uniqueSqs) {
            const tp = explodedBoard[sq.row]?.[sq.col];
            if (tp && tp.type !== 'king') {
              explodedBoard[sq.row][sq.col] = null;
            }
          }
          return explodedBoard;
        });
        setBombPieces(updatedBombs);
        setBombExploding([]);
        setCardMsg(`💥 BOMB EXPLODED! ${uniqueSqs.length} pieces destroyed!`);
        setTimeout(() => setCardMsg(''), 3000);
      }, 900);
    } else {
      setBombPieces(updatedBombs);
    }

    return currentBoard;
  }, []);

  const handleLavaLanding = React.useCallback((tr: number, tc: number, pieceType: PieceType | undefined) => {
    const lava = lavaSquaresRef.current.find(l => l.row === tr && l.col === tc);
    if (lava && pieceType !== 'king') {
      setLavaExploding(prev => [...prev, { row: tr, col: tc }]);
      fireCardAnim('lava_kill', `Piece incinerated on ${FILES[tc]}${RANKS[tr]}`);
      setTimeout(() => {
        setBoard(b2 => { const nb2 = cloneBoard(b2); nb2[tr][tc] = null; return nb2; });
        setLavaSquares(prev =>
          prev
            .filter(l => !(l.row === tr && l.col === tc))
            .map(l => ({ ...l, movesLeft: l.movesLeft - 1 }))
            .filter((l): l is LavaSquare => l.movesLeft > 0)
        );
        setLavaExploding(prev => prev.filter(l => !(l.row === tr && l.col === tc)));
      }, 700);
      return true;
    }
    return false;
  }, []);

  const processTurnEffects = React.useCallback((turn: PieceColor, authoritativeLive: boolean, boardRef: React.MutableRefObject<Board>) => {
    if (authoritativeLive) return;

    const ghost = ghostRef.current;
    if (ghost) {
      if (ghost.ownerColor === turn) {
        const updated = { ...ghost, roundsLeft: ghost.roundsLeft - 1 };
        setGhostPiece(updated);
        ghostRef.current = updated;
      } else if (ghost.roundsLeft <= 0) {
        setGhostPiece(null);
        ghostRef.current = null;
        setBoard(prev => {
          const nb = prev.map(r => r.map(p => p ? { ...p } : null)) as Board;
          const occupant = nb[ghost.row][ghost.col];
          if (occupant) {
            setCardMsg(`👁️ ${ghost.piece.type} reappeared on an occupied square and was destroyed!`);
            setTimeout(() => setCardMsg(''), 3000);
            return nb;
          } else {
            nb[ghost.row][ghost.col] = { ...ghost.piece };
            setCardMsg(`👁️ ${ghost.piece.type} reappears!`);
            setTimeout(() => setCardMsg(''), 2000);
            return nb;
          }
        });
      }
    }

    processBombs(turn, boardRef.current);

    if (turn === 'white') {
      setFogZones(prev => {
        const next = prev
          .map(z => ({ ...z, turnsLeft: z.turnsLeft - 1 }))
          .filter(z => z.turnsLeft > 0);
        if (next.length < prev.length) {
          setCardMsg('🌤️ Fog of War lifted!');
          setTimeout(() => setCardMsg(''), 2500);
        }
        return next;
      });
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [processBombs]);

  const resetBoardEffects = React.useCallback(() => {
    setLavaSquares([]);
    setLavaExploding([]);
    setFogZones([]);
    setGhostPiece(null);
    ghostRef.current = null;
  }, []);

  return {
    lavaSquares, setLavaSquares,
    lavaExploding, setLavaExploding,
    lavaSquaresRef,
    ghostPiece, setGhostPiece,
    ghostRef,
    fogZones, setFogZones,
    processBombs,
    handleLavaLanding,
    processTurnEffects,
    resetBoardEffects,
  };
}
