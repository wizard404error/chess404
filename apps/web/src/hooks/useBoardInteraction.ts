'use client';

import React from 'react';
import type { Board, PieceType, PieceColor, Sq, Snapshot } from '../types';
import type { BoardArrow } from '../BoardCanvas';
import { makeBoard } from '../chessEngine';
import type { MatchFinishReason } from '@chess404/contracts';

export function useBoardInteraction() {
  const [board,     setBoard]     = React.useState<Board>(makeBoard);
  const [turn,      setTurn]      = React.useState<PieceColor>('white');
  const [sel,       setSel]       = React.useState<Sq | null>(null);
  const [hints,     setHints]     = React.useState<Sq[]>([]);
  const [premove,   setPremove]   = React.useState<{ from: Sq; to: Sq } | null>(null);
  const [moved,     setMoved]     = React.useState<Set<string>>(new Set());
  const [lm,        setLm]        = React.useState<{ from: Sq; to: Sq } | null>(null);
  const [drag,      setDrag]      = React.useState<Sq | null>(null);
  const [dragPos,   setDragPos]   = React.useState<{ x: number; y: number } | null>(null);
  const [promo,     setPromo]     = React.useState<{
    row: number; col: number; color: PieceColor; fromCol?: number;
    from?: Sq; to?: Sq; authoritativeMatchId?: string;
    turn?: PieceColor; note?: string; moved: Set<string>;
    lm: { from: Sq; to: Sq } | null; hmc: number; fmn: number;
  } | null>(null);
  const [check,     setCheck]     = React.useState(false);
  const [mate,      setMate]      = React.useState(false);
  const [stale,     setStale]     = React.useState(false);
  const [insuf,     setInsuf]     = React.useState(false);
  const [hmc,       setHmc]       = React.useState(0);
  const [fmn,       setFmn]       = React.useState(1);
  const [posHist,   setPosHist]   = React.useState<string[]>([]);
  const [drawOffer, setDrawOffer] = React.useState<PieceColor | null>(null);
  const [over,      setOver]      = React.useState(false);
  const [winner,    setWinner]    = React.useState<PieceColor | 'draw' | 'aborted' | null>(null);
  const [authoritativeFinishReason, setAuthoritativeFinishReason] = React.useState<MatchFinishReason | null>(null);
  const [movHist,   setMovHist]   = React.useState<{ n: string; w?: string; b?: string }[]>([]);
  const [snapshots, setSnapshots] = React.useState<Snapshot[]>([]);
  const [analysisArrows, setAnalysisArrows] = React.useState<BoardArrow[]>([]);

  const boardRef   = React.useRef(board);
  const turnRef    = React.useRef(turn);
  const movedRef   = React.useRef(moved);
  const lmRef      = React.useRef(lm);
  const hmcRef     = React.useRef(hmc);
  const fmnRef     = React.useRef(fmn);
  const posHistRef = React.useRef(posHist);
  const overRef    = React.useRef(over);
  const premoveRef = React.useRef<{ from: Sq; to: Sq } | null>(null);

  React.useEffect(() => { boardRef.current      = board;      }, [board]);
  React.useEffect(() => { turnRef.current       = turn;       }, [turn]);
  React.useEffect(() => { movedRef.current      = moved;      }, [moved]);
  React.useEffect(() => { lmRef.current         = lm;         }, [lm]);
  React.useEffect(() => { hmcRef.current        = hmc;        }, [hmc]);
  React.useEffect(() => { fmnRef.current        = fmn;        }, [fmn]);
  React.useEffect(() => { posHistRef.current    = posHist;    }, [posHist]);
  React.useEffect(() => { overRef.current       = over;       }, [over]);
  React.useEffect(() => { premoveRef.current    = premove;    }, [premove]);

  return {
    board, setBoard,
    turn, setTurn,
    sel, setSel,
    hints, setHints,
    premove, setPremove,
    moved, setMoved,
    lm, setLm,
    drag, setDrag,
    dragPos, setDragPos,
    promo, setPromo,
    check, setCheck,
    mate, setMate,
    stale, setStale,
    insuf, setInsuf,
    hmc, setHmc,
    fmn, setFmn,
    posHist, setPosHist,
    drawOffer, setDrawOffer,
    over, setOver,
    winner, setWinner,
    authoritativeFinishReason, setAuthoritativeFinishReason,
    movHist, setMovHist,
    snapshots, setSnapshots,
    analysisArrows, setAnalysisArrows,
    boardRef, turnRef, movedRef, lmRef, hmcRef, fmnRef, posHistRef, overRef, premoveRef,
  };
}
