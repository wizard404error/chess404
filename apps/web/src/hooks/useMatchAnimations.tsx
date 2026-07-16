'use client';

import React from 'react';
import type {
  BombPiece,
  CardAnimType,
  PieceColor,
  PieceType,
  Sq,
} from '../types';
import type {
  TransformAnim,
  SniperAnim,
  TeleportAnim,
  JumpAnim,
  SacrificeAnim,
  MindControlAnim,
  FuseAnim,
} from '../BoardCanvas';

export function useMatchAnimations() {
  const [cardAnim,    setCardAnim]    = React.useState<CardAnimType>(null);
  const [cardAnimLbl, setCardAnimLbl] = React.useState('');
  const fireCardAnim = React.useCallback((type: CardAnimType, lbl = '') => {
    setCardAnim(type);
    setCardAnimLbl(lbl);
  }, []);

  const [bombPieces,    setBombPieces]    = React.useState<BombPiece[]>([]);
  const [bombExploding, setBombExploding] = React.useState<Sq[]>([]);
  const bombPiecesRef = React.useRef<BombPiece[]>([]);
  React.useEffect(() => { bombPiecesRef.current = bombPieces; }, [bombPieces]);

  const [swapAnim, setSwapAnim] = React.useState<{
    sq1: Sq; sq2: Sq;
    color1: string; color2: string;
  } | null>(null);
  const swapAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerSwapAnim = React.useCallback((sq1: Sq, sq2: Sq, color1 = '#4ade80', color2 = '#60a5fa') => {
    if (swapAnimTimerRef.current) clearTimeout(swapAnimTimerRef.current);
    setSwapAnim({ sq1, sq2, color1, color2 });
    swapAnimTimerRef.current = setTimeout(() => setSwapAnim(null), 800);
  }, []);

  const [transformAnim, setTransformAnim] = React.useState<TransformAnim | null>(null);
  const transformAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerTransformAnim = React.useCallback((
    sq: Sq,
    direction: 'up' | 'down',
    fromType: PieceType,
    toType: PieceType,
    color: PieceColor,
  ) => {
    if (transformAnimTimerRef.current) clearTimeout(transformAnimTimerRef.current);
    setTransformAnim({ sq, direction, fromType, toType, color, startTime: performance.now() });
    transformAnimTimerRef.current = setTimeout(() => setTransformAnim(null), 1600);
  }, []);

  const [sniperAnim, setSniperAnim]     = React.useState<SniperAnim | null>(null);
  const sniperAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  const [teleportAnim, setTeleportAnim]   = React.useState<TeleportAnim | null>(null);
  const teleportAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  const [jumpAnim, setJumpAnim] = React.useState<JumpAnim | null>(null);
  const jumpAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  const [sacrificeAnim, setSacrificeAnim] = React.useState<SacrificeAnim | null>(null);
  const sacrificeAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerSacrificeAnim = React.useCallback((squares: { row: number; col: number }[]) => {
    if (sacrificeAnimTimerRef.current) clearTimeout(sacrificeAnimTimerRef.current);
    setSacrificeAnim({ squares, startTime: performance.now() });
    sacrificeAnimTimerRef.current = setTimeout(() => setSacrificeAnim(null), 1700);
  }, []);

  const [mindControlAnim, setMindControlAnim] = React.useState<MindControlAnim | null>(null);
  const mindControlAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerMindControlAnim = React.useCallback((
    targetSq: Sq,
    playerColor: PieceColor,
    pieceType: PieceType,
  ) => {
    if (mindControlAnimTimerRef.current) clearTimeout(mindControlAnimTimerRef.current);
    setMindControlAnim({ targetSq, playerColor, pieceType, startTime: performance.now() });
    mindControlAnimTimerRef.current = setTimeout(() => setMindControlAnim(null), 2100);
  }, []);

  const [fuseAnim, setFuseAnim] = React.useState<FuseAnim | null>(null);
  const fuseAnimTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerFuseAnim = React.useCallback((params: Omit<FuseAnim, 'startTime'>) => {
    if (fuseAnimTimerRef.current) clearTimeout(fuseAnimTimerRef.current);
    setFuseAnim({ ...params, startTime: performance.now() });
    fuseAnimTimerRef.current = setTimeout(() => setFuseAnim(null), 1900);
  }, []);

  const triggerJumpAnim = React.useCallback((
    fromSq: Sq,
    toSq: Sq,
    pieceType: PieceType,
    pieceColor: PieceColor,
    captured: boolean,
  ) => {
    if (jumpAnimTimerRef.current) clearTimeout(jumpAnimTimerRef.current);
    setJumpAnim({ fromSq, toSq, pieceType, pieceColor, captured, startTime: performance.now() });
    jumpAnimTimerRef.current = setTimeout(() => setJumpAnim(null), 1200);
  }, []);

  const triggerSniperAnim = React.useCallback((
    sq: Sq, pieceType: PieceType, pieceColor: PieceColor, variant: 'sniper' | 'badsniper'
  ) => {
    if (sniperAnimTimerRef.current) clearTimeout(sniperAnimTimerRef.current);
    setSniperAnim({ sq, pieceType, pieceColor, variant, startTime: performance.now() });
    sniperAnimTimerRef.current = setTimeout(() => setSniperAnim(null), 1200);
  }, []);

  const triggerTeleportAnim = React.useCallback((
    fromSq: { row: number; col: number },
    toSq: { row: number; col: number },
    pieceType: PieceType,
    pieceColor: PieceColor,
  ) => {
    if (teleportAnimTimerRef.current) clearTimeout(teleportAnimTimerRef.current);
    setTeleportAnim({ fromSq, toSq, pieceType, pieceColor, startTime: performance.now() });
    teleportAnimTimerRef.current = setTimeout(() => setTeleportAnim(null), 1400);
  }, []);

  React.useEffect(() => {
    return () => {
      if (swapAnimTimerRef.current) clearTimeout(swapAnimTimerRef.current);
      if (transformAnimTimerRef.current) clearTimeout(transformAnimTimerRef.current);
      if (sniperAnimTimerRef.current) clearTimeout(sniperAnimTimerRef.current);
      if (teleportAnimTimerRef.current) clearTimeout(teleportAnimTimerRef.current);
      if (jumpAnimTimerRef.current) clearTimeout(jumpAnimTimerRef.current);
      if (sacrificeAnimTimerRef.current) clearTimeout(sacrificeAnimTimerRef.current);
      if (mindControlAnimTimerRef.current) clearTimeout(mindControlAnimTimerRef.current);
      if (fuseAnimTimerRef.current) clearTimeout(fuseAnimTimerRef.current);
    };
  }, []);

  return {
    cardAnim, setCardAnim,
    cardAnimLbl, setCardAnimLbl,
    fireCardAnim,
    bombPieces, setBombPieces,
    bombExploding, setBombExploding,
    bombPiecesRef,
    swapAnim, setSwapAnim,
    swapAnimTimerRef,
    triggerSwapAnim,
    transformAnim, setTransformAnim,
    transformAnimTimerRef,
    triggerTransformAnim,
    sniperAnim, setSniperAnim,
    sniperAnimTimerRef,
    teleportAnim, setTeleportAnim,
    teleportAnimTimerRef,
    jumpAnim, setJumpAnim,
    jumpAnimTimerRef,
    sacrificeAnim, setSacrificeAnim,
    sacrificeAnimTimerRef,
    triggerSacrificeAnim,
    mindControlAnim, setMindControlAnim,
    mindControlAnimTimerRef,
    triggerMindControlAnim,
    fuseAnim, setFuseAnim,
    fuseAnimTimerRef,
    triggerFuseAnim,
    triggerJumpAnim,
    triggerSniperAnim,
    triggerTeleportAnim,
  };
}
