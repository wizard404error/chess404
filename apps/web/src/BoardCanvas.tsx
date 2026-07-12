'use client';

import React from 'react';
import type { Board, Piece, PieceType, PieceColor, Sq, CardPendingState, BombPiece, LavaSquare, DoubleMove } from './types';
import { SQ as IMPORTED_SQ, FILES } from './constants';
import { SQ, setSQ } from './canvas/animations';
import {
  drawBoardArrow, parseColor, hexToRgb,
  easeOut, easeIn, easeInOut, clamp, lerp,
  ANALYSIS_ARROW_COLOR, TRANSFORM_DURATION, SNIPER_DURATION,
  TELEPORT_DURATION, JUMP_DURATION, REVERSE_DURATION, SACRIFICE_DURATION, MINDCONTROL_DURATION, FUSE_DURATION,
  paintTeleportAnim, paintJumpAnim, paintMindControlAnim,
  paintFuseAnim, paintSacrificeAnim, paintReverseAnim,
  paintSniperAnim, paintTransformAnim, pushParticle,
} from './canvas/animations';
import { isUsableImage, getFusedImage, PIECE_IMAGES } from './canvas/images';
import type { Particle, TransformAnim, SniperAnim, TeleportAnim, JumpAnim, SacrificeAnim, MindControlAnim, FuseAnim, ReverseAnim, BoardArrow } from './canvas/animations';
export type { TransformAnim, SniperAnim, TeleportAnim, JumpAnim, SacrificeAnim, MindControlAnim, FuseAnim, ReverseAnim, BoardArrow };

// ─── BoardCanvas ─────────────────────────────────────────────────────────────
export interface BoardCanvasProps {
  board: Board;
  turn: PieceColor;
  sel: Sq | null;
  hints: Sq[];
  lm: { from: Sq; to: Sq } | null;
  drag: Sq | null;
  dragPos: { x: number; y: number } | null;
  check: boolean;
  kingPos: Sq | null;
  cardHighlight: (r: number, c: number) => string | null;
  doubleMoveHighlight: (r: number, c: number) => string | null;
  bombPieces: BombPiece[];
  bombExploding: Sq[];
  lavaSquares: LavaSquare[];
  lavaExploding: Sq[];
  swapAnim: { sq1: Sq; sq2: Sq; color1: string; color2: string } | null;
  isReviewing: boolean;
  reviewBoard: Board | null;
  cardPending: CardPendingState;
  onClick: (r: number, c: number) => void;
  onDragStart: (e: React.MouseEvent, r: number, c: number) => void;
  onDrop: (r: number, c: number) => void;
  doubleMove: DoubleMove | null;
  transformAnim: TransformAnim | null;
  sniperAnim: SniperAnim | null;
  teleportAnim: TeleportAnim | null;
  jumpAnim: JumpAnim | null;
  reverseAnim: ReverseAnim | null;
  sacrificeAnim: SacrificeAnim | null;
  sacrificeSelectedSquares: { row: number; col: number }[];
  mindControlAnim: MindControlAnim | null;
  mindControlTargetSquare: { row: number; col: number } | null;
  fuseAnim: FuseAnim | null;
  fuseSelectedSq: { row: number; col: number } | null; // step-1 selected square highlight
  fogZones: { centerRow: number; centerCol: number; ownerColor: PieceColor }[];
  viewerColor: PieceColor | null;
  invisibleUnder?: { row: number; col: number; piece: Piece; ownerColor: PieceColor } | null;
  analysisArrows: BoardArrow[];
  onToggleAnalysisArrow: (from: Sq, to: Sq) => void;
  onClearAnalysisArrows: () => void;
  colorBlindMode?: boolean;
  premove?: { from: Sq; to: Sq } | null;
  onPremove?: () => void;
}

export const BoardCanvas = React.memo(function BoardCanvas(props: BoardCanvasProps) {
  const {
    board, turn, sel, hints, lm, check, kingPos,
    cardHighlight, doubleMoveHighlight, bombPieces, bombExploding,
    lavaSquares, lavaExploding, swapAnim, isReviewing, reviewBoard,
    cardPending, onClick, onDragStart, onDrop, doubleMove, transformAnim,
    sniperAnim, teleportAnim, jumpAnim, reverseAnim, sacrificeAnim, sacrificeSelectedSquares, mindControlAnim, mindControlTargetSquare, fuseAnim, fuseSelectedSq,     fogZones, viewerColor, invisibleUnder, analysisArrows, onToggleAnalysisArrow, onClearAnalysisArrows, colorBlindMode, premove, onPremove,
  } = props;

  const canvasRef  = React.useRef<HTMLCanvasElement>(null);
  const rafRef     = React.useRef<number>(0);
  const particles  = React.useRef<Particle[]>([]);
  const tRef       = React.useRef(0);
  const swapRef    = React.useRef<typeof swapAnim>(null);
  const swapStartT = React.useRef(0);

  const transformRef     = React.useRef<TransformAnim | null>(null);
  const transformSpawned = React.useRef(false);
  const sniperRef        = React.useRef<SniperAnim | null>(null);
  const teleportRef      = React.useRef<TeleportAnim | null>(null);
  const jumpRef          = React.useRef<JumpAnim | null>(null);
  const reverseRef       = React.useRef<ReverseAnim | null>(null);
  const sacrificeRef     = React.useRef<SacrificeAnim | null>(null);
  const mindControlRef   = React.useRef<MindControlAnim | null>(null);
  const fuseRef          = React.useRef<FuseAnim | null>(null);

  React.useEffect(() => {
    sacrificeRef.current = sacrificeAnim ?? null;
  }, [sacrificeAnim]);

  React.useEffect(() => {
    mindControlRef.current = mindControlAnim ?? null;
  }, [mindControlAnim]);

  React.useEffect(() => {
    fuseRef.current = fuseAnim ?? null;
  }, [fuseAnim]);

  React.useEffect(() => {
    if (transformAnim && transformAnim !== transformRef.current) {
      transformRef.current = transformAnim;
      transformSpawned.current = false;
    }
    if (!transformAnim) {
      transformRef.current = null;
      transformSpawned.current = false;
    }
  }, [transformAnim]);

  React.useEffect(() => {
    sniperRef.current = sniperAnim;
  }, [sniperAnim]);

  React.useEffect(() => {
    teleportRef.current = teleportAnim;
  }, [teleportAnim]);

  React.useEffect(() => {
    jumpRef.current = jumpAnim;
  }, [jumpAnim]);

  React.useEffect(() => {
    reverseRef.current = reverseAnim ?? null;
  }, [reverseAnim]);

  const displayBoard = reviewBoard ?? board;

  // ── Responsive board size ─────────────────────────────────────────────────
  const MAX_BOARD_PX = 8 * IMPORTED_SQ;
  const [boardPx, setBoardPx] = React.useState(MAX_BOARD_PX);

  React.useLayoutEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const parent = canvas.parentElement;
    if (!parent) return;
    const w = parent.clientWidth;
    if (w > 0) {
      setBoardPx(Math.min(MAX_BOARD_PX, Math.floor(w / 8) * 8));
    }
  }, []);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = entry.contentBoxSize?.[0]?.inlineSize ?? entry.contentRect.width;
        if (w > 0) {
          setBoardPx(prev => {
            const next = Math.min(MAX_BOARD_PX, Math.floor(w / 8) * 8);
            return next >= 32 * 8 ? next : prev;
          });
        }
      }
    });
    ro.observe(canvas);
    return () => ro.disconnect();
  }, []);

  setSQ(boardPx / 8);
  const W = boardPx, H = boardPx;
  const dpr = typeof window !== 'undefined' ? (window.devicePixelRatio || 1) : 1;

  React.useEffect(() => {
    if (swapAnim && swapAnim !== swapRef.current) {
      swapRef.current  = swapAnim;
      swapStartT.current = tRef.current;
      const spawn = (sq: Sq, col: string) => {
        const cx = sq.col * SQ + SQ / 2;
        const cy = (7 - sq.row) * SQ + SQ / 2;
        for (let i = 0; i < 20; i++) {
          const a = Math.random() * Math.PI * 2;
          const spd = 1.5 + Math.random() * 3;
          pushParticle(particles.current, {
            x: cx, y: cy,
            vx: Math.cos(a) * spd, vy: Math.sin(a) * spd,
            life: 1, maxLife: 1,
            r: 2 + Math.random() * 3,
            color: col,
            type: 'swap',
          });
        }
      };
      spawn(swapAnim.sq1, swapAnim.color1);
      spawn(swapAnim.sq2, swapAnim.color2);
    }
    if (!swapAnim) swapRef.current = null;
  }, [swapAnim]);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d')!;
    let last = performance.now();

    const spawnBombParticles = (sq: Sq) => {
      const cx = sq.col * SQ + SQ / 2;
      const cy = (7 - sq.row) * SQ + SQ / 2;
      for (let i = 0; i < 40; i++) {
        const a = Math.random() * Math.PI * 2;
        const spd = 2 + Math.random() * 6;
        pushParticle(particles.current, {
          x: cx, y: cy, vx: Math.cos(a)*spd, vy: Math.sin(a)*spd - 2,
          life: 1, maxLife: 1, r: 3 + Math.random() * 5,
          color: i < 20 ? '#ff6600' : i < 32 ? '#ffdd00' : '#ffffff',
          type: 'spark',
        });
      }
      for (let i = 0; i < 12; i++) {
        const a = Math.random() * Math.PI * 2;
        const spd = 0.5 + Math.random() * 2;
        pushParticle(particles.current, {
          x: cx + (Math.random()-0.5)*20, y: cy + (Math.random()-0.5)*20,
          vx: Math.cos(a)*spd, vy: Math.sin(a)*spd - 1.5,
          life: 1, maxLife: 1, r: 8 + Math.random() * 12,
          color: 'rgba(60,60,60,0.8)',
          type: 'smoke',
        });
      }
    };

    const spawnLavaParticles = (sq: Sq) => {
      const cx = sq.col * SQ + SQ / 2;
      const cy = (7 - sq.row) * SQ + SQ / 2;
      for (let i = 0; i < 18; i++) {
        const a = Math.random() * Math.PI * 2;
        const spd = 1.5 + Math.random() * 4;
        pushParticle(particles.current, {
          x: cx, y: cy, vx: Math.cos(a)*spd, vy: Math.sin(a)*spd - 3,
          life: 1, maxLife: 1, r: 3 + Math.random() * 5,
          color: i < 9 ? '#ff4500' : '#ffcc00',
          type: 'lava',
        });
      }
    };

    const spawnTransformParticles = (anim: TransformAnim) => {
      const cx = anim.sq.col * SQ + SQ / 2;
      const cy = (7 - anim.sq.row) * SQ + SQ / 2;
      const isUp = anim.direction === 'up';
      const colors = isUp
        ? ['#ffd700', '#ffffff', '#ffa500', '#fffacd']
        : ['#9333ea', '#c084fc', '#6d28d9', '#e879f9'];
      for (let i = 0; i < 50; i++) {
        const a = Math.random() * Math.PI * 2;
        const spd = 1.5 + Math.random() * 5;
        const biasVy = isUp ? -Math.random() * 3 : Math.random() * 3;
        pushParticle(particles.current, {
          x: cx + (Math.random() - 0.5) * SQ * 0.5,
          y: cy + (Math.random() - 0.5) * SQ * 0.5,
          vx: Math.cos(a) * spd,
          vy: Math.sin(a) * spd + biasVy,
          life: 1, maxLife: 1,
          r: 2 + Math.random() * 5,
          color: colors[Math.floor(Math.random() * colors.length)],
          type: 'spark',
        });
      }
    };

    const spawnedBombs = new Set<string>();
    const spawnedLava  = new Set<string>();

    const draw = (now: number) => {
      const dt = Math.min((now - last) / 16.67, 3);
      last = now;
      tRef.current = now;

      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, W, H);

      // ── Draw squares ──────────────────────────────────────────────────────
      for (let ri = 0; ri < 8; ri++) {
        for (let ci = 0; ci < 8; ci++) {
          const row = 7 - ri;
          const col = ci;
          const light = (row + col) % 2 !== 0;
          const x = col * SQ, y = ri * SQ;

          const isSel = !isReviewing && sel?.row === row && sel?.col === col;
          const isLM  = lm && ((lm.from.row === row && lm.from.col === col) || (lm.to.row === row && lm.to.col === col));
          const isChk = kingPos?.row === row && kingPos?.col === col;
          const isBombRadius = bombPieces.some((b: BombPiece) => Math.abs(row-b.row)<=1 && Math.abs(col-b.col)<=1);
          const isBombCenter = bombPieces.some((b: BombPiece) => b.row===row && b.col===col);
          const isExplodingBomb = bombExploding.some((s: Sq) => s.row===row && s.col===col);

          const cb = colorBlindMode;
          let baseColor: string;
          if (cb) {
            baseColor = light ? '#FFD9B3' : '#8B7D6B';
            if (isLM)  baseColor = light ? '#A3C4F3' : '#6A9BD1';
            if (isSel) baseColor = '#4A90D9';
            if (isChk) baseColor = '#E85D5D';
          } else {
            baseColor = light ? '#F0D9B5' : '#B58863';
            if (isLM)  baseColor = light ? '#cdd26a' : '#aaa23a';
            if (isSel) baseColor = '#FFD700';
            if (isChk) baseColor = '#e85555';
          }

          if (isBombRadius && !isBombCenter) {
            const pulse = 0.4 + Math.sin(now/500 + row * 0.7 + col * 0.3) * 0.15;
            const r2 = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.6);
            r2.addColorStop(0, `rgba(255,100,20,${pulse})`);
            r2.addColorStop(1, 'rgba(0,0,0,0)');
            ctx.fillStyle = baseColor;
            ctx.fillRect(x, y, SQ, SQ);
            ctx.fillStyle = r2;
          } else {
            ctx.fillStyle = baseColor;
          }
          ctx.fillRect(x, y, SQ, SQ);

          // ── Card / double-move highlight with FIXED SYMMETRIC BORDERS ─────
          const chl = cardHighlight(row, col);
          const dhl = doubleMoveHighlight(row, col);
          const hl  = chl || dhl;
          if (hl) {
            const [r2,g2,b2,a2] = parseColor(hl);

            // Fill tint
            ctx.fillStyle = `rgba(${r2},${g2},${b2},${a2 * 0.7})`;
            ctx.fillRect(x, y, SQ, SQ);

            // Only draw a border on sides where the neighbor is NOT also highlighted.
            // This prevents double-borders on shared edges that make them look thinner.
            const noTop    = row < 7 && !!(cardHighlight(row + 1, col) || doubleMoveHighlight(row + 1, col));
            const noBottom = row > 0 && !!(cardHighlight(row - 1, col) || doubleMoveHighlight(row - 1, col));
            const noLeft   = col > 0 && !!(cardHighlight(row, col - 1) || doubleMoveHighlight(row, col - 1));
            const noRight  = col < 7 && !!(cardHighlight(row, col + 1) || doubleMoveHighlight(row, col + 1));

            const bw = 3;
            const borderAlpha = Math.min(1, a2 * 1.8);
            ctx.fillStyle = `rgba(${r2},${g2},${b2},${borderAlpha})`;
            if (!noTop)    ctx.fillRect(x,           y,           SQ, bw); // top
            if (!noBottom) ctx.fillRect(x,           y + SQ - bw, SQ, bw); // bottom
            if (!noLeft)   ctx.fillRect(x,           y,           bw, SQ); // left
            if (!noRight)  ctx.fillRect(x + SQ - bw, y,           bw, SQ); // right
          }

          // ── Pulsing blood highlight for sacrifice-selected squares ──────────
          const isSacSelected = sacrificeSelectedSquares.some(s => s.row === row && s.col === col);
          if (isSacSelected) {
            const pulse = 0.55 + Math.sin(now / 220) * 0.35;
            // Beating blood fill
            const sacGrad = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.65);
            sacGrad.addColorStop(0, `rgba(220,20,20,${pulse * 0.75})`);
            sacGrad.addColorStop(0.6, `rgba(180,10,10,${pulse * 0.45})`);
            sacGrad.addColorStop(1, `rgba(100,0,0,0)`);
            ctx.fillStyle = sacGrad;
            ctx.fillRect(x, y, SQ, SQ);
            // Glowing red border
            ctx.strokeStyle = `rgba(255,${40 + (pulse * 60) | 0},0,${0.8 + pulse * 0.2})`;
            ctx.lineWidth = 3;
            ctx.shadowColor = '#dc1414';
            ctx.shadowBlur = 14 * pulse;
            ctx.strokeRect(x + 1.5, y + 1.5, SQ - 3, SQ - 3);
            ctx.shadowBlur = 0;
            // ✕ mark in center
            ctx.save();
            ctx.font = `bold ${SQ * 0.38}px serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = `rgba(255,80,80,${pulse * 0.85})`;
            ctx.shadowColor = '#ff0000';
            ctx.shadowBlur = 8;
            ctx.fillText('✕', x + SQ/2, y + SQ/2);
            ctx.shadowBlur = 0;
            ctx.restore();
          }

          // ── Pulsing psychic highlight for mindcontrol/borrow target squares ─
          // Detected by the purple highlight color returned by cardHighlight
          if (chl === 'rgba(168,85,247,0.5)') {
            const pulse = 0.55 + Math.sin(now / 200) * 0.38;
            const mcGrad = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.65);
            mcGrad.addColorStop(0, `rgba(139,0,255,${pulse * 0.55})`);
            mcGrad.addColorStop(0.5, `rgba(168,85,247,${pulse * 0.3})`);
            mcGrad.addColorStop(1, `rgba(80,0,160,0)`);
            ctx.fillStyle = mcGrad;
            ctx.fillRect(x, y, SQ, SQ);

            // Spinning arc border
            const arcAngle = (now / 600) % (Math.PI * 2);
            ctx.save();
            ctx.beginPath();
            ctx.arc(x+SQ/2, y+SQ/2, SQ/2 - 2, arcAngle, arcAngle + Math.PI * 1.3);
            ctx.strokeStyle = `rgba(200,100,255,${0.75 + pulse * 0.2})`;
            ctx.lineWidth = 2.5;
            ctx.shadowColor = '#8b00ff';
            ctx.shadowBlur = 12 * pulse;
            ctx.stroke();
            ctx.shadowBlur = 0;
            ctx.beginPath();
            ctx.arc(x+SQ/2, y+SQ/2, SQ/2 - 2, arcAngle + Math.PI, arcAngle + Math.PI * 1.6);
            ctx.strokeStyle = `rgba(255,100,220,${0.5 * pulse})`;
            ctx.lineWidth = 1.5;
            ctx.stroke();
            ctx.restore();
          }

          // ── Dead isMCTarget block (prop is always null, kept for future) ────
          const isMCTarget = mindControlTargetSquare?.row === row && mindControlTargetSquare?.col === col;
          if (isMCTarget) { /* unused */ }

          // ── Fuse step-1 selected square: pulsing gold border ─────────────
          const isFuseSel = fuseSelectedSq?.row === row && fuseSelectedSq?.col === col;
          if (isFuseSel) {
            const fpulse = 0.6 + Math.sin(now / 220) * 0.35;
            ctx.save();
            ctx.beginPath();
            ctx.rect(x + 1, y + 1, SQ - 2, SQ - 2);
            ctx.strokeStyle = `rgba(251,191,36,${fpulse})`;
            ctx.lineWidth = 3;
            ctx.shadowColor = '#fbbf24';
            ctx.shadowBlur = 14 * fpulse;
            ctx.stroke();
            ctx.shadowBlur = 0;
            // Gold radial glow
            const fGrad = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.6);
            fGrad.addColorStop(0, `rgba(251,191,36,${fpulse * 0.35})`);
            fGrad.addColorStop(1, 'rgba(251,191,36,0)');
            ctx.fillStyle = fGrad;
            ctx.fillRect(x, y, SQ, SQ);
            ctx.restore();
          }

          if (isBombCenter && !isExplodingBomb) {
            const bomb = bombPieces.find((b: BombPiece) => b.row===row && b.col===col)!;
            const pulse = 0.6 + Math.sin(now / (bomb.turnsLeft <= 1 ? 200 : 400)) * 0.35;
            const color = bomb.turnsLeft <= 1 ? `rgba(255,40,0,${pulse})` : `rgba(255,120,30,${pulse})`;
            ctx.strokeStyle = color;
            ctx.lineWidth = 3;
            ctx.strokeRect(x + 1.5, y + 1.5, SQ - 3, SQ - 3);
            const g2 = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.6);
            g2.addColorStop(0, `rgba(255,${bomb.turnsLeft<=1?40:120},0,${pulse * 0.5})`);
            g2.addColorStop(1, 'transparent');
            ctx.fillStyle = g2;
            ctx.fillRect(x, y, SQ, SQ);
          }

          const isHint = !isReviewing && hints.some((h: Sq) => h.row===row && h.col===col);
          const piece   = displayBoard[row][col];
          if (isHint) {
            if (!piece) {
              ctx.beginPath();
              ctx.arc(x + SQ/2, y + SQ/2, 10, 0, Math.PI*2);
              ctx.fillStyle = 'rgba(0,0,0,0.22)';
              ctx.fill();
              if (cb) {
                ctx.strokeStyle = '#4A90D9';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.arc(x + SQ/2, y + SQ/2, 14, 0, Math.PI*2);
                ctx.stroke();
              }
            } else {
              ctx.strokeStyle = cb ? '#4A90D9' : 'rgba(0,0,0,0.22)';
              ctx.lineWidth = cb ? 3 : 4;
              ctx.beginPath();
              ctx.arc(x + SQ/2, y + SQ/2, SQ/2 - 4, 0, Math.PI*2);
              ctx.stroke();
            }
          }
          if (cb && isSel) {
            ctx.fillStyle = '#FFFFFF';
            ctx.font = `bold ${SQ * 0.25}px sans-serif`;
            ctx.textAlign = 'right';
            ctx.textBaseline = 'top';
            ctx.fillText('◆', x + SQ - 4, y + 4);
          }
          if (cb && isChk) {
            ctx.fillStyle = '#FF4444';
            ctx.font = `bold ${SQ * 0.35}px sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText('⚠', x + SQ/2, y + SQ/2);
          }
        }
      }

      // ── Rank / file labels ────────────────────────────────────────────────
      ctx.font = 'bold 11px sans-serif';
      ctx.textBaseline = 'top';
      for (let ri = 0; ri < 8; ri++) {
        const row = 7 - ri;
        const lightAtCol0 = (row + 0) % 2 !== 0;
        ctx.fillStyle = lightAtCol0 ? '#B58863' : '#F0D9B5';
        ctx.fillText(String(row + 1), 3, ri * SQ + 3);
      }
      ctx.textBaseline = 'bottom';
      for (let ci = 0; ci < 8; ci++) {
        const lightAtRow0 = (0 + ci) % 2 !== 0;
        ctx.fillStyle = lightAtRow0 ? '#B58863' : '#F0D9B5';
        ctx.fillText(FILES[ci], ci * SQ + SQ - 10, H - 2);
      }
      ctx.textBaseline = 'alphabetic';

      for (const arrow of analysisArrows) {
        drawBoardArrow(ctx, arrow.from, arrow.to, arrow.color);
      }
      if (annotationStartRef.current && annotationTargetRef.current) {
        drawBoardArrow(ctx, annotationStartRef.current, annotationTargetRef.current, ANALYSIS_ARROW_COLOR, { alpha: 0.6, preview: true });
      }

      if (premove) {
        ctx.save();
        ctx.setLineDash([6, 4]);
        drawBoardArrow(ctx, premove.from, premove.to, 'rgba(96,165,250,0.8)', { lineWidth: 3 });
        ctx.setLineDash([]);
        const pmFrom = { row: 7 - premove.from.row, col: premove.from.col };
        const pmTo = { row: 7 - premove.to.row, col: premove.to.col };
        ctx.fillStyle = 'rgba(96,165,250,0.85)';
        ctx.font = `bold ${SQ * 0.2}px sans-serif`;
        ctx.textAlign = 'right';
        ctx.textBaseline = 'top';
        ctx.fillText('P➜', pmFrom.col * SQ + SQ - 3, pmFrom.row * SQ + 3);
        ctx.restore();
      }

      // ── Draw ghost (invisible) piece and any piece on same square ────────
      if (invisibleUnder) {
        const { row: ur, col: uc, piece: up } = invisibleUnder;
        const uri = 7 - ur;
        const ux = uc * SQ, uy = uri * SQ;

        // Draw any real piece occupying the same square at full opacity underneath
        const boardPiece = displayBoard[ur][uc];
        if (boardPiece) {
          const bimg = PIECE_IMAGES[`${boardPiece.color}_${boardPiece.type}`];
          if (bimg && bimg.complete) {
            ctx.save();
            ctx.globalAlpha = 1.0;
            ctx.drawImage(bimg, ux + 3, uy + 3, SQ - 6, SQ - 6);
            ctx.restore();
          }
        }

        // Draw ghost piece on top with ethereal animation
        const gimg = PIECE_IMAGES[`${up.color}_${up.type}`];
        if (gimg && gimg.complete) {
          const cx2 = ux + SQ / 2, cy2 = uy + SQ / 2;

          // Pulsing ghost aura behind piece
          ctx.save();
          const auraA = 0.18 + Math.sin(now / 500) * 0.08;
          const aura = ctx.createRadialGradient(cx2, cy2, 0, cx2, cy2, SQ * 0.6);
          aura.addColorStop(0,   `rgba(180,210,255,${auraA * 1.5})`);
          aura.addColorStop(0.5, `rgba(140,180,255,${auraA})`);
          aura.addColorStop(1,   'rgba(100,150,255,0)');
          ctx.fillStyle = aura;
          ctx.beginPath(); ctx.arc(cx2, cy2, SQ * 0.6, 0, Math.PI * 2); ctx.fill();
          ctx.restore();

          // Ghost piece itself — semi-transparent blue shimmer
          ctx.save();
          ctx.globalAlpha = 0.38 + Math.sin(now / 600) * 0.10;
          ctx.filter = 'brightness(1.6) saturate(0.3) hue-rotate(200deg)';
          ctx.shadowColor = 'rgba(160,200,255,0.9)';
          ctx.shadowBlur = 18 + Math.sin(now / 400) * 8;
          ctx.drawImage(gimg, ux + 3, uy + 3, SQ - 6, SQ - 6);
          ctx.restore();

          // 3 orbiting sparkle dots
          ctx.save();
          for (let sp = 0; sp < 3; sp++) {
            const sAngle = (now / 1200) + sp * (Math.PI * 2 / 3);
            const sR = SQ * 0.38;
            const sx2 = cx2 + Math.cos(sAngle) * sR;
            const sy2 = cy2 + Math.sin(sAngle) * sR;
            const sSize = 2.5 + Math.sin(now / 300 + sp) * 1;
            ctx.beginPath(); ctx.arc(sx2, sy2, sSize, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(200,225,255,${0.8 + Math.sin(now / 200 + sp) * 0.15})`;
            ctx.shadowColor = 'rgba(160,200,255,0.9)';
            ctx.shadowBlur = 8;
            ctx.fill();
            ctx.shadowBlur = 0;
          }
          ctx.restore();

          // 👁️ icon top-left corner
          ctx.save();
          ctx.font = '11px serif';
          ctx.textBaseline = 'top';
          ctx.globalAlpha = 0.7 + Math.sin(now / 700) * 0.2;
          ctx.fillText('👁️', ux + 1, uy + 1);
          ctx.globalAlpha = 1;
          ctx.textBaseline = 'alphabetic';
          ctx.restore();
        }
      }

      const transformSq = transformRef.current?.sq;
      for (let ri = 0; ri < 8; ri++) {
        for (let ci = 0; ci < 8; ci++) {
          const row = 7 - ri;
          const col = ci;
          const p = displayBoard[row][col];
          if (!p) continue;
          if (localDragRef.current?.row === row && localDragRef.current?.col === col) continue;

          if (transformRef.current && transformSq?.row === row && transformSq?.col === col) {
            const elapsed = now - transformRef.current.startTime;
            const t = elapsed / TRANSFORM_DURATION;
            if (t >= 0 && t < 1.0) continue;
          }

          const sa = sniperRef.current;
          if (sa && sa.sq.row === row && sa.sq.col === col) {
            const elapsed = now - sa.startTime;
            const t = elapsed / SNIPER_DURATION;
            if (t >= 0.14 && t < 1.0) continue;
          }

          // Skip rendering enemy pieces that are inside the opponent's fog zone
          // (the fog overlay with "?" will be drawn later)
          const isHiddenByFog = fogZones.some(z =>
            p.color === z.ownerColor &&        // piece belongs to fog owner
            viewerColor !== z.ownerColor &&    // viewer is the opponent
            Math.abs(row - z.centerRow) <= 1 &&
            Math.abs(col - z.centerCol) <= 1
          );
          if (isHiddenByFog) continue;

          // Skip ghost's square — drawn separately above with ghost animation
          if (invisibleUnder && invisibleUnder.row === row && invisibleUnder.col === col) continue;

          const x = col * SQ, y = ri * SQ;
          const img = PIECE_IMAGES[`${p.color}_${p.type}`];
          if (!isUsableImage(img)) continue;

          ctx.save();
          const cx = x + SQ/2, cy = y + SQ/2;

          if (p.frozen) {
            ctx.filter = 'sepia(100%) hue-rotate(185deg) brightness(0.55) saturate(3.0) contrast(1.4)';
            ctx.shadowColor = '#38bdf8';
            ctx.shadowBlur = 18 + Math.sin(now / 380) * 8;
          } else if (p.shielded) {
            ctx.shadowColor = '#4ade80';
            ctx.shadowBlur = 16 + Math.sin(now / 400) * 8;
          } else if (p.parasiteTarget) {
            ctx.shadowColor = '#a855f7';
            ctx.shadowBlur = 14 + Math.sin(now / 300) * 7;
          } else if (p.bomb) {
            ctx.shadowColor = '#ff6600';
            ctx.shadowBlur = 10;
            const tick = Math.sin(now / 133) * 4 * (Math.PI / 180);
            ctx.translate(cx, cy);
            ctx.rotate(tick);
            ctx.translate(-cx, -cy);
          } else if (p.fusedWith) {
            // glow handled in the fused rendering block below
          }

          // Only draw base piece if no custom fused sprite will replace it
          const hasCustomFusedSprite = p.fusedWith ? !!getFusedImage(p.color, p.type, p.fusedWith) : false;
          if (!hasCustomFusedSprite) {
            ctx.drawImage(img, x + 3, y + 3, SQ - 6, SQ - 6);
          }
          ctx.restore();

          // ── FUSED piece rendering ─────────────────────────────────────────
          if (p.fusedWith) {
            const pulse = 0.5 + Math.sin(now / 500) * 0.15;
            const customImg = getFusedImage(p.color, p.type, p.fusedWith);

            if (customImg) {
              // ── Custom sprite — smooth, large, multi-pass glow ───────────
              const pad = -6;
              const size = SQ - pad * 2;
              ctx.save();
              ctx.imageSmoothingEnabled = true;
              ctx.imageSmoothingQuality = 'high';
              // Pass 1: strong white halo
              ctx.shadowColor = 'rgba(255,255,255,1)';
              ctx.shadowBlur = 14;
              ctx.drawImage(customImg, x + pad, y + pad, size, size);
              ctx.restore();
              ctx.save();
              ctx.imageSmoothingEnabled = true;
              ctx.imageSmoothingQuality = 'high';
              // Pass 2: gold glow
              ctx.shadowColor = '#fbbf24';
              ctx.shadowBlur = 18 + pulse * 12;
              ctx.drawImage(customImg, x + pad, y + pad, size, size);
              ctx.restore();
              ctx.save();
              ctx.imageSmoothingEnabled = true;
              ctx.imageSmoothingQuality = 'high';
              // Pass 3: crisp final draw
              ctx.drawImage(customImg, x + pad, y + pad, size, size);
              ctx.restore();
              // Small gold ⚗ badge in corner
              ctx.save();
              ctx.beginPath();
              ctx.arc(x + SQ - 9, y + 9, 6, 0, Math.PI * 2);
              ctx.fillStyle = `rgba(251,191,36,${0.85 + pulse * 0.15})`;
              ctx.shadowColor = '#fbbf24';
              ctx.shadowBlur = 8 + pulse * 6;
              ctx.fill();
              ctx.font = 'bold 7px sans-serif';
              ctx.fillStyle = '#1c1400';
              ctx.textAlign = 'center';
              ctx.textBaseline = 'middle';
              ctx.shadowBlur = 0;
              ctx.fillText('⚗', x + SQ - 9, y + 9);
              ctx.restore();
            } else {
              // ── Fallback: split-diagonal with both piece images ──────────
              const secondImg = PIECE_IMAGES[`${p.color}_${p.fusedWith}`];
              if (secondImg && secondImg.complete) {
                // Clip top-right triangle for the second piece
                ctx.save();
                ctx.beginPath();
                ctx.moveTo(x + SQ * 0.38, y);
                ctx.lineTo(x + SQ, y);
                ctx.lineTo(x + SQ, y + SQ * 0.62);
                ctx.closePath();
                ctx.clip();
                ctx.globalAlpha = 0.88;
                ctx.drawImage(secondImg, x + 3, y + 3, SQ - 6, SQ - 6);
                ctx.restore();
                // Diagonal divider line
                ctx.save();
                ctx.beginPath();
                ctx.moveTo(x + SQ * 0.38, y);
                ctx.lineTo(x + SQ, y + SQ * 0.62);
                ctx.strokeStyle = `rgba(251,191,36,${0.7 + pulse * 0.3})`;
                ctx.lineWidth = 2;
                ctx.shadowColor = '#fbbf24';
                ctx.shadowBlur = 8 + pulse * 6;
                ctx.stroke();
                ctx.restore();
              }
              // Gold ⚗ badge
              ctx.save();
              ctx.beginPath();
              ctx.arc(x + SQ - 9, y + 9, 6, 0, Math.PI * 2);
              ctx.fillStyle = `rgba(251,191,36,${0.85 + pulse * 0.15})`;
              ctx.shadowColor = '#fbbf24';
              ctx.shadowBlur = 10 + pulse * 8;
              ctx.fill();
              ctx.font = 'bold 7px sans-serif';
              ctx.fillStyle = '#1c1400';
              ctx.textAlign = 'center';
              ctx.textBaseline = 'middle';
              ctx.shadowBlur = 0;
              ctx.fillText('⚗', x + SQ - 9, y + 9);
              ctx.restore();
            }
          }
          if (p.frozen) {
            const cx2 = x + SQ / 2, cy2 = y + SQ / 2;
            const iceA = 0.72 + Math.sin(now / 700) * 0.06;
            const iceFill = ctx.createLinearGradient(x, y, x + SQ, y + SQ);
            iceFill.addColorStop(0,   `rgba(30, 100, 200, ${iceA * 0.55})`);
            iceFill.addColorStop(0.35,`rgba(120,190,255, ${iceA * 0.85})`);
            iceFill.addColorStop(0.65,`rgba(200,230,255, ${iceA})`);
            iceFill.addColorStop(1,   `rgba(40, 120, 220, ${iceA * 0.65})`);
            ctx.fillStyle = iceFill;
            ctx.fillRect(x, y, SQ, SQ);

            const spec = ctx.createLinearGradient(x, y, x + SQ * 0.6, y + SQ * 0.6);
            spec.addColorStop(0,   `rgba(255,255,255,${0.45 + Math.sin(now/500)*0.1})`);
            spec.addColorStop(0.4, `rgba(200,235,255,0.20)`);
            spec.addColorStop(1,   'rgba(255,255,255,0)');
            ctx.fillStyle = spec;
            ctx.fillRect(x, y, SQ, SQ);

            const shimmer = (Math.sin(now / 380 + col * 0.6 + row * 0.5) + 1) / 2;
            const sg = ctx.createLinearGradient(x, y, x + SQ, y + SQ);
            sg.addColorStop(Math.max(0, shimmer - 0.15), 'rgba(255,255,255,0)');
            sg.addColorStop(shimmer,                     'rgba(255,255,255,0.55)');
            sg.addColorStop(Math.min(1, shimmer + 0.15), 'rgba(255,255,255,0)');
            ctx.fillStyle = sg;
            ctx.fillRect(x, y, SQ, SQ);

            const bp = 0.85 + Math.sin(now / 420) * 0.12;
            ctx.shadowColor = '#7dd3fc';
            ctx.shadowBlur = 18;
            ctx.strokeStyle = `rgba(147,210,255,${bp})`;
            ctx.lineWidth = 4;
            ctx.strokeRect(x + 2, y + 2, SQ - 4, SQ - 4);
            ctx.strokeStyle = `rgba(224,242,255,${bp * 0.6})`;
            ctx.lineWidth = 1.5;
            ctx.strokeRect(x + 6, y + 6, SQ - 12, SQ - 12);
            ctx.shadowBlur = 0;

            const corners: [number, number, number][] = [
              [x + 10,      y + 10,      0],
              [x + SQ - 10, y + 10,      1],
              [x + 10,      y + SQ - 10, 2],
              [x + SQ - 10, y + SQ - 10, 3],
            ];
            const cA = 0.95 + Math.sin(now / 600 + col + row) * 0.04;
            ctx.shadowColor = '#bae6fd';
            ctx.shadowBlur = 12;
            for (const [crx, cry, ci] of corners) {
              ctx.save();
              ctx.translate(crx, cry);
              ctx.rotate(now / 3500 + ci * Math.PI / 2);
              for (let arm = 0; arm < 6; arm++) {
                const ang   = (arm / 6) * Math.PI * 2;
                const armLen = 8 + Math.sin(now / 300 + arm + ci) * 1.5;
                ctx.beginPath();
                ctx.moveTo(0, 0);
                ctx.lineTo(Math.cos(ang) * armLen, Math.sin(ang) * armLen);
                ctx.strokeStyle = `rgba(220,242,255,${cA})`;
                ctx.lineWidth = 1.5;
                ctx.stroke();
                for (const barFrac of [0.45, 0.75]) {
                  const bx   = Math.cos(ang) * armLen * barFrac;
                  const by   = Math.sin(ang) * armLen * barFrac;
                  const perp = ang + Math.PI / 2;
                  const bLen = 3.5 * (1 - barFrac * 0.4);
                  ctx.beginPath();
                  ctx.moveTo(bx - Math.cos(perp)*bLen, by - Math.sin(perp)*bLen);
                  ctx.lineTo(bx + Math.cos(perp)*bLen, by + Math.sin(perp)*bLen);
                  ctx.strokeStyle = `rgba(186,230,255,${cA * 0.8})`;
                  ctx.lineWidth = 1;
                  ctx.stroke();
                }
              }
              const gg = ctx.createRadialGradient(0,0,0,0,0,3);
              gg.addColorStop(0, `rgba(255,255,255,${cA})`);
              gg.addColorStop(1, `rgba(125,211,252,${cA*0.5})`);
              ctx.beginPath(); ctx.arc(0, 0, 3, 0, Math.PI*2);
              ctx.fillStyle = gg; ctx.fill();
              ctx.restore();
            }
            ctx.shadowBlur = 0;

            const crackSeeds = [0.1, 0.95, 1.8, 2.65, 3.5, 4.35, 5.2];
            for (let ci2 = 0; ci2 < crackSeeds.length; ci2++) {
              const baseAngle = crackSeeds[ci2];
              const crackA2   = 0.55 + Math.sin(now / 900 + ci2 * 1.4) * 0.1;
              const crackLen2 = (0.28 + (ci2 % 3) * 0.14) * SQ * 0.52;
              ctx.save();
              ctx.translate(cx2, cy2);
              ctx.beginPath();
              ctx.moveTo(0, 0);
              let cx3 = 0, cy3 = 0;
              for (let seg = 0; seg < 4; seg++) {
                const jitter = (seg % 2 === 0 ? 0.22 : -0.22);
                const sA = baseAngle + jitter;
                const sL = crackLen2 / 4;
                cx3 += Math.cos(sA) * sL;
                cy3 += Math.sin(sA) * sL;
                ctx.lineTo(cx3, cy3);
                if (seg === 2) {
                  ctx.save();
                  ctx.moveTo(cx3, cy3);
                  ctx.lineTo(
                    cx3 + Math.cos(sA + 0.5) * sL * 0.6,
                    cy3 + Math.sin(sA + 0.5) * sL * 0.6
                  );
                  ctx.strokeStyle = `rgba(147,210,255,${crackA2 * 0.6})`;
                  ctx.lineWidth = 0.8;
                  ctx.stroke();
                  ctx.restore();
                  ctx.moveTo(cx3, cy3);
                }
              }
              ctx.strokeStyle = `rgba(200,235,255,${crackA2})`;
              ctx.lineWidth = 1.2;
              ctx.stroke();
              ctx.restore();
            }

            for (let fi = 0; fi < 10; fi++) {
              const phase = (now / 1200 + fi * 0.1) % 1;
              const fx  = x + 5 + (fi * (SQ - 10) / 9) + Math.sin(now / 450 + fi * 1.2) * 6;
              const fy  = y + SQ - phase * (SQ + 12);
              const fa  = phase < 0.12 ? phase / 0.12 : phase > 0.7 ? (1 - phase) / 0.3 : 1;
              const fr  = 1.8 + Math.sin(now / 260 + fi) * 0.7;
              ctx.beginPath();
              ctx.arc(fx, fy, fr, 0, Math.PI * 2);
              ctx.fillStyle = `rgba(224,242,255,${fa * 0.9})`;
              ctx.fill();
            }

            const sfA = 0.9 + Math.sin(now / 310) * 0.08;
            ctx.save();
            ctx.translate(cx2, cy2);
            ctx.rotate(Math.sin(now / 1800) * 0.12);
            ctx.font = 'bold 20px serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.shadowColor = '#7dd3fc';
            ctx.shadowBlur = 20;
            ctx.fillStyle = `rgba(240,249,255,${sfA})`;
            ctx.fillText('❄', 0, 0);
            ctx.shadowBlur = 0;
            ctx.restore();
          }

          // ── SHIELD overlay ─────────────────────────────────────────────────
          if (p.shielded) {
            const cx2 = x + SQ / 2, cy2 = y + SQ / 2;
            const R = SQ * 0.48;

            const glowA = 0.18 + Math.sin(now / 450) * 0.08;
            const grad = ctx.createRadialGradient(cx2, cy2, R * 0.5, cx2, cy2, R * 1.15);
            grad.addColorStop(0, `rgba(74,222,128,0)`);
            grad.addColorStop(0.6, `rgba(74,222,128,${glowA})`);
            grad.addColorStop(1, `rgba(74,222,128,0)`);
            ctx.fillStyle = grad;
            ctx.beginPath();
            ctx.arc(cx2, cy2, R * 1.15, 0, Math.PI * 2);
            ctx.fill();

            const hexA = 0.7 + Math.sin(now / 500) * 0.2;
            ctx.beginPath();
            for (let hi = 0; hi < 6; hi++) {
              const angle = (hi / 6) * Math.PI * 2 - Math.PI / 6;
              const hx = cx2 + Math.cos(angle) * R;
              const hy = cy2 + Math.sin(angle) * R;
              hi === 0 ? ctx.moveTo(hx, hy) : ctx.lineTo(hx, hy);
            }
            ctx.closePath();
            ctx.strokeStyle = `rgba(74,222,128,${hexA})`;
            ctx.lineWidth = 2;
            ctx.shadowColor = '#4ade80';
            ctx.shadowBlur = 12;
            ctx.stroke();
            ctx.shadowBlur = 0;

            const innerR = R * 0.78;
            ctx.beginPath();
            const innerRot = -now / 3000;
            for (let hi = 0; hi < 6; hi++) {
              const angle = (hi / 6) * Math.PI * 2 - Math.PI / 6 + innerRot;
              const hx = cx2 + Math.cos(angle) * innerR;
              const hy = cy2 + Math.sin(angle) * innerR;
              hi === 0 ? ctx.moveTo(hx, hy) : ctx.lineTo(hx, hy);
            }
            ctx.closePath();
            ctx.strokeStyle = `rgba(134,239,172,${hexA * 0.45})`;
            ctx.lineWidth = 1;
            ctx.stroke();

            const arcAngle = (now / 1200) % (Math.PI * 2);
            ctx.beginPath();
            ctx.arc(cx2, cy2, R, arcAngle, arcAngle + Math.PI * 0.6);
            ctx.strokeStyle = `rgba(167,243,208,0.9)`;
            ctx.lineWidth = 2.5;
            ctx.shadowColor = '#6ee7b7';
            ctx.shadowBlur = 10;
            ctx.stroke();
            ctx.beginPath();
            ctx.arc(cx2, cy2, R, arcAngle + Math.PI, arcAngle + Math.PI * 1.4);
            ctx.strokeStyle = `rgba(74,222,128,0.6)`;
            ctx.lineWidth = 1.5;
            ctx.stroke();
            ctx.shadowBlur = 0;

            for (let hi = 0; hi < 6; hi++) {
              const angle = (hi / 6) * Math.PI * 2 - Math.PI / 6;
              const dotA = 0.6 + Math.sin(now / 350 + hi * 1.05) * 0.35;
              const dotR = 2.5 + Math.sin(now / 280 + hi) * 0.8;
              ctx.beginPath();
              ctx.arc(
                cx2 + Math.cos(angle) * R,
                cy2 + Math.sin(angle) * R,
                dotR, 0, Math.PI * 2
              );
              ctx.fillStyle = `rgba(134,239,172,${dotA})`;
              ctx.shadowColor = '#4ade80';
              ctx.shadowBlur = 8;
              ctx.fill();
              ctx.shadowBlur = 0;
            }

            const runeA = 0.55 + Math.sin(now / 600) * 0.3;
            ctx.font = 'bold 9px serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = `rgba(134,239,172,${runeA})`;
            ctx.shadowColor = '#4ade80';
            ctx.shadowBlur = 6;
            ctx.fillText('✦', x + SQ - 8, y + 8);
            ctx.shadowBlur = 0;
            ctx.textAlign = 'left';
            ctx.textBaseline = 'alphabetic';
          }

          if (p.bomb) {
            const wobble = Math.sin(now / 250) * 2;
            ctx.font = '14px serif';
            ctx.shadowColor = '#ff6600';
            ctx.shadowBlur = 8;
            ctx.fillText('💣', x - 2, y + 12 + wobble);
            ctx.shadowBlur = 0;
            const bomb = bombPieces.find((b: BombPiece) => b.row===row && b.col===col);
            if (bomb) {
              const bx = x + SQ - 10, by = y + 10;
              ctx.beginPath();
              ctx.arc(bx, by, 9, 0, Math.PI*2);
              const pulse = 0.85 + Math.sin(now / 200) * 0.15;
              ctx.fillStyle = bomb.turnsLeft <= 1
                ? `rgba(255,${Math.floor(20+pulse*30)},0,1)`
                : `rgba(255,${Math.floor(80+pulse*50)},0,1)`;
              ctx.fill();
              ctx.strokeStyle = '#fff';
              ctx.lineWidth = 1.5;
              ctx.stroke();
              ctx.fillStyle = '#fff';
              ctx.font = 'bold 9px sans-serif';
              ctx.textAlign = 'center';
              ctx.textBaseline = 'middle';
              ctx.fillText(String(bomb.turnsLeft), bx, by);
              ctx.textAlign = 'left';
              ctx.textBaseline = 'alphabetic';
            }
          }

        }
      }

      // ── Parasite links ───────────────────────────────────────────────────
      for (let pr = 0; pr < 8; pr++) {
        for (let pc = 0; pc < 8; pc++) {
          const pp = displayBoard[pr]?.[pc];
          if (!pp?.parasiteTarget) continue;
          const [tpr, tpc] = (pp.parasiteTarget as string).split(',').map(Number);
          const tp2 = displayBoard[tpr]?.[tpc];
          if (!tp2) continue;
          const x1 = pc  * SQ + SQ / 2, y1 = (7 - pr)  * SQ + SQ / 2;
          const x2 = tpc * SQ + SQ / 2, y2 = (7 - tpr) * SQ + SQ / 2;
          const pulse = 0.55 + Math.sin(now / 220) * 0.35;
          // outer glow line
          ctx.save();
          ctx.beginPath();
          ctx.moveTo(x1, y1);
          ctx.lineTo(x2, y2);
          ctx.strokeStyle = `rgba(168,85,247,${pulse * 0.4})`;
          ctx.lineWidth = 6;
          ctx.shadowColor = '#a855f7';
          ctx.shadowBlur = 12;
          ctx.stroke();
          // inner bright line
          ctx.beginPath();
          ctx.moveTo(x1, y1);
          ctx.lineTo(x2, y2);
          ctx.strokeStyle = `rgba(240,171,252,${pulse * 0.9})`;
          ctx.lineWidth = 2;
          ctx.shadowBlur = 4;
          ctx.setLineDash([6, 5]);
          ctx.lineDashOffset = -(now / 30) % 11;
          ctx.stroke();
          ctx.setLineDash([]);
          // orbs at each end
          for (const [ox, oy] of [[x1,y1],[x2,y2]]) {
            ctx.beginPath();
            ctx.arc(ox, oy, 5 + Math.sin(now / 200) * 2, 0, Math.PI * 2);
            ctx.fillStyle = `rgba(216,180,254,${pulse})`;
            ctx.shadowColor = '#a855f7';
            ctx.shadowBlur = 10;
            ctx.fill();
          }
          ctx.restore();
        }
      }

      // ── Drag ghost ────────────────────────────────────────────────────────
      const activeDrag    = localDragRef.current;
      const activeDragPos = localDragPosRef.current;
      if (activeDrag && activeDragPos) {
        const p = displayBoard[activeDrag.row][activeDrag.col];
        if (p) {
          const img = PIECE_IMAGES[`${p.color}_${p.type}`];
          if (isUsableImage(img)) {
            ctx.save();
            ctx.globalAlpha = 0.85;
            ctx.shadowColor = 'rgba(0,0,0,0.5)';
            ctx.shadowBlur = 12;
            ctx.drawImage(img, activeDragPos.x - SQ/2, activeDragPos.y - SQ/2, SQ, SQ);
            ctx.restore();
            ctx.save();
            ctx.globalAlpha = 0.25;
            ctx.drawImage(img, activeDrag.col*SQ+3, (7-activeDrag.row)*SQ+3, SQ-6, SQ-6);
            ctx.restore();
          }
        }
      }

      // ── Bomb explosion ────────────────────────────────────────────────────
      for (const sq of bombExploding) {
        const key = `${sq.row},${sq.col}`;
        if (!spawnedBombs.has(key)) {
          spawnedBombs.add(key);
          spawnBombParticles(sq);
        }
        const cx = sq.col * SQ + SQ/2;
        const cy = (7 - sq.row) * SQ + SQ/2;
        const tElapsed = now - (swapStartT.current || 0);
        const fb = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 1.8);
        fb.addColorStop(0, 'rgba(255,255,255,0.95)');
        fb.addColorStop(0.15, 'rgba(255,220,0,0.9)');
        fb.addColorStop(0.4, 'rgba(255,80,0,0.7)');
        fb.addColorStop(0.7, 'rgba(150,20,0,0.4)');
        fb.addColorStop(1, 'transparent');
        ctx.fillStyle = fb;
        ctx.beginPath();
        ctx.arc(cx, cy, SQ * 2.2, 0, Math.PI*2);
        ctx.fill();
        const ringR = SQ * (0.5 + (tElapsed % 900) / 900 * 3);
        ctx.strokeStyle = `rgba(255,200,50,${Math.max(0, 0.8 - ringR/(SQ*3))})`;
        ctx.lineWidth = 3;
        ctx.beginPath();
        ctx.arc(cx, cy, ringR, 0, Math.PI*2);
        ctx.stroke();
        const ringR2 = SQ * (0.5 + ((tElapsed + 150) % 900) / 900 * 3);
        ctx.strokeStyle = `rgba(255,150,30,${Math.max(0, 0.6 - ringR2/(SQ*3))})`;
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.arc(cx, cy, ringR2, 0, Math.PI*2);
        ctx.stroke();
      }

      // ── Lava explosion ────────────────────────────────────────────────────
      for (const sq of lavaExploding) {
        const key = `${sq.row},${sq.col}`;
        if (!spawnedLava.has(key)) {
          spawnedLava.add(key);
          spawnLavaParticles(sq);
        }
        const cx = sq.col * SQ + SQ/2;
        const cy = (7 - sq.row) * SQ + SQ/2;
        const lb = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 1.5);
        lb.addColorStop(0, 'rgba(255,200,0,0.9)');
        lb.addColorStop(0.3, 'rgba(255,80,0,0.8)');
        lb.addColorStop(0.7, 'rgba(200,20,0,0.4)');
        lb.addColorStop(1, 'transparent');
        ctx.fillStyle = lb;
        ctx.beginPath();
        ctx.arc(cx, cy, SQ * 1.6, 0, Math.PI*2);
        ctx.fill();
      }

      // ── Lava active squares ───────────────────────────────────────────────
      for (const ls of lavaSquares) {
        if (lavaExploding.some((e: Sq) => e.row===ls.row && e.col===ls.col)) continue;
        const x = ls.col * SQ, y = (7 - ls.row) * SQ;
        const pulse = 0.55 + Math.sin(now / 280 + ls.col + ls.row) * 0.25;
        const lavGrad = ctx.createRadialGradient(x+SQ/2, y+SQ/2, 0, x+SQ/2, y+SQ/2, SQ*0.65);
        lavGrad.addColorStop(0, `rgba(255,${Math.floor(100+pulse*80)},0,${pulse * 0.85})`);
        lavGrad.addColorStop(0.5, `rgba(200,30,0,${pulse * 0.7})`);
        lavGrad.addColorStop(1, `rgba(120,15,0,${pulse * 0.4})`);
        ctx.fillStyle = lavGrad;
        ctx.fillRect(x, y, SQ, SQ);
        ctx.strokeStyle = `rgba(255,${Math.floor(80+pulse*100)},0,${0.8 + pulse * 0.2})`;
        ctx.lineWidth = 2.5;
        ctx.strokeRect(x+2, y+2, SQ-4, SQ-4);
        for (let b2 = 0; b2 < 3; b2++) {
          const bx = x + ((Math.sin(now/400 + ls.col*1.7 + ls.row*2.1 + b2*1.1)*0.5+0.5) * (SQ-14)) + 7;
          const by = y + ((Math.cos(now/350 + ls.row*1.3 + ls.col*0.8 + b2*0.9)*0.5+0.5) * (SQ-14)) + 7;
          const br = 2.5 + Math.sin(now/200 + ls.col + ls.row + b2) * 1.5;
          ctx.beginPath();
          ctx.arc(bx, by, Math.max(1, br), 0, Math.PI*2);
          ctx.fillStyle = `rgba(255,${Math.floor(180+Math.sin(now/300+b2)*60)},0,0.95)`;
          ctx.fill();
        }
        ctx.font = '11px serif';
        ctx.textBaseline = 'top';
        ctx.fillText('🌋', x+2, y+2);
        ctx.textBaseline = 'alphabetic';
      }

      // ── Fog of War zones ──────────────────────────────────────────────────
      for (const zone of fogZones) {
        const isOwner = viewerColor === zone.ownerColor;

        const cr0 = Math.max(0, zone.centerRow - 1);
        const cr1 = Math.min(7, zone.centerRow + 1);
        const cc0 = Math.max(0, zone.centerCol - 1);
        const cc1 = Math.min(7, zone.centerCol + 1);
        const zx  = cc0 * SQ;
        const zy  = (7 - cr1) * SQ;
        const zw  = (cc1 - cc0 + 1) * SQ;
        const zh  = (cr1 - cr0 + 1) * SQ;
        const zcx = zx + zw / 2;
        const zcy = zy + zh / 2;

        // ── OWNER: clearly visible misty veil — pieces show through but fog is obvious
        if (isOwner) {
          ctx.save();
          ctx.beginPath(); ctx.rect(zx, zy, zw, zh); ctx.clip();

          // Strong blue-white mist — very visible but pieces still show through
          ctx.fillStyle = 'rgba(160,200,245,0.42)';
          ctx.fillRect(zx, zy, zw, zh);

          // 5 large drifting cloud blobs — clearly visible
          for (let w = 0; w < 5; w++) {
            const s   = w * 79.3 + zone.centerCol * 7 + zone.centerRow * 13;
            const bx  = zcx + Math.sin(now * 0.000095 + s) * zw * 0.38;
            const by  = zcy + Math.cos(now * 0.000078 + s * 0.6) * zh * 0.32;
            const br  = zw * (0.34 + w * 0.06);
            const bri = 0.38 - w * 0.04;
            const bg  = ctx.createRadialGradient(bx, by - br * 0.12, 0, bx, by, br);
            bg.addColorStop(0,    `rgba(215,232,255,${bri})`);
            bg.addColorStop(0.45, `rgba(185,215,252,${bri * 0.55})`);
            bg.addColorStop(1,    'rgba(150,190,240,0)');
            ctx.fillStyle = bg;
            ctx.beginPath(); ctx.arc(bx, by, br, 0, Math.PI * 2); ctx.fill();
          }

          // Bright lit top edge
          const topG = ctx.createLinearGradient(zx, zy, zx, zy + zh * 0.45);
          topG.addColorStop(0,   'rgba(230,244,255,0.32)');
          topG.addColorStop(0.5, 'rgba(200,228,254,0.14)');
          topG.addColorStop(1,   'rgba(175,210,250,0)');
          ctx.fillStyle = topG;
          ctx.fillRect(zx, zy, zw, zh * 0.45);

          // Solid glowing border
          ctx.strokeStyle = `rgba(120,185,255,${0.70 + Math.sin(now / 1600) * 0.15})`;
          ctx.lineWidth = 2.5;
          ctx.shadowColor = 'rgba(100,170,255,0.6)';
          ctx.shadowBlur = 8;
          ctx.strokeRect(zx + 1.5, zy + 1.5, zw - 3, zh - 3);
          ctx.shadowBlur = 0;
          ctx.restore();

        // ── ENEMY: full realistic cloud-fog
        } else {
          ctx.save();
          ctx.beginPath(); ctx.rect(zx, zy, zw, zh); ctx.clip();

          // ── 1. Neutral mid-grey base so clouds have something to sit on ───
          ctx.fillStyle = 'rgba(80,95,125,0.72)';
          ctx.fillRect(zx, zy, zw, zh);

          // ── 2. Large macro billowing cloud masses ─────────────────────────
          // Big soft ellipses, bright white-grey tops, fade to nothing at edges
          // Each drifts slowly left-right at its own speed
          const clouds = [
            { relX:0.50, relY:0.30, rx:0.78, ry:0.55, spd:0.000055, ph:0.00, br:0.90 },
            { relX:0.20, relY:0.44, rx:0.62, ry:0.46, spd:0.000042, ph:2.10, br:0.78 },
            { relX:0.80, relY:0.40, rx:0.58, ry:0.42, spd:0.000038, ph:4.35, br:0.74 },
            { relX:0.38, relY:0.62, rx:0.65, ry:0.38, spd:0.000063, ph:1.55, br:0.62 },
            { relX:0.65, relY:0.58, rx:0.52, ry:0.35, spd:0.000047, ph:3.20, br:0.58 },
            { relX:0.50, relY:0.78, rx:0.85, ry:0.28, spd:0.000031, ph:5.10, br:0.48 },
          ];

          for (const c of clouds) {
            const driftX = Math.sin(now * c.spd + c.ph) * zw * 0.20;
            const cx2 = zx + c.relX * zw + driftX;
            const cy2 = zy + c.relY * zh;
            const rx  = c.rx * zw;
            const ry  = c.ry * zh;

            ctx.save();
            ctx.translate(cx2, cy2);
            ctx.scale(1, ry / rx);
            // Gradient: bright at top-center, fades to transparent at edge
            const cg = ctx.createRadialGradient(0, -rx * 0.15, 0, 0, rx * 0.05, rx);
            cg.addColorStop(0,    `rgba(225,235,250,${c.br})`);
            cg.addColorStop(0.22, `rgba(200,218,242,${c.br * 0.82})`);
            cg.addColorStop(0.50, `rgba(165,185,225,${c.br * 0.50})`);
            cg.addColorStop(0.78, `rgba(110,135,185,${c.br * 0.20})`);
            cg.addColorStop(1,    'rgba(70,95,150,0)');
            ctx.fillStyle = cg;
            ctx.beginPath(); ctx.arc(0, 0, rx, 0, Math.PI * 2); ctx.fill();
            ctx.restore();
          }

          // ── 3. Mid-size secondary puffs for texture variety ───────────────
          for (let p = 0; p < 9; p++) {
            const s   = p * 41.7 + zone.centerCol * 19 + zone.centerRow * 29;
            const px2 = zx + ((Math.sin(s) * 0.5 + 0.5)) * zw;
            const py2 = zy + ((Math.cos(s * 0.7) * 0.5 + 0.5)) * zh * 0.72;
            const pdx = Math.sin(now * 0.000048 + s) * SQ * 0.60;
            const pr  = SQ * (0.42 + (Math.sin(s * 1.3) * 0.5 + 0.5) * 0.28);
            const pa  = 0.32 + (Math.cos(s * 0.9) * 0.5 + 0.5) * 0.20;
            const pg  = ctx.createRadialGradient(px2 + pdx, py2, 0, px2 + pdx, py2, pr);
            pg.addColorStop(0,   `rgba(215,228,248,${pa})`);
            pg.addColorStop(0.5, `rgba(175,198,232,${pa * 0.48})`);
            pg.addColorStop(1,   'rgba(130,158,208,0)');
            ctx.fillStyle = pg;
            ctx.beginPath(); ctx.arc(px2 + pdx, py2, pr, 0, Math.PI * 2); ctx.fill();
          }

          // ── 4. Bright lit top-edge highlight (sun catching cloud tops) ────
          const ridgeG = ctx.createLinearGradient(zx, zy, zx, zy + zh * 0.38);
          ridgeG.addColorStop(0,   'rgba(240,248,255,0.28)');
          ridgeG.addColorStop(0.35,'rgba(215,232,252,0.12)');
          ridgeG.addColorStop(1,   'rgba(185,210,245,0)');
          ctx.fillStyle = ridgeG;
          ctx.fillRect(zx, zy, zw, zh * 0.38);

          // ── 5. Dark bottom — fog base is always shadowed ──────────────────
          const groundG = ctx.createLinearGradient(zx, zy + zh * 0.50, zx, zy + zh);
          groundG.addColorStop(0,   'rgba(15,22,45,0)');
          groundG.addColorStop(0.55,'rgba(12,18,38,0.42)');
          groundG.addColorStop(1,   'rgba(6,10,24,0.78)');
          ctx.fillStyle = groundG;
          ctx.fillRect(zx, zy + zh * 0.50, zw, zh * 0.50);

          ctx.restore(); // end clip

          // ── 6. Soft spill halo outside zone boundary ───────────────────────
          ctx.save();
          const halo = ctx.createRadialGradient(zcx, zcy + zh * 0.1, zw * 0.34, zcx, zcy + zh * 0.1, zw * 0.34 + SQ * 0.85);
          halo.addColorStop(0,   'rgba(130,158,210,0.13)');
          halo.addColorStop(0.6, 'rgba(100,130,195,0.05)');
          halo.addColorStop(1,   'rgba(80,110,180,0)');
          ctx.fillStyle = halo;
          ctx.beginPath(); ctx.arc(zcx, zcy + zh * 0.1, zw * 0.34 + SQ * 0.85, 0, Math.PI * 2); ctx.fill();
          ctx.restore();

          // ── 7. ? marks — rendered ABOVE fog, bold and clear ───────────────
          for (let dr2 = -1; dr2 <= 1; dr2++) {
            for (let dc2 = -1; dc2 <= 1; dc2++) {
              const fogRow = zone.centerRow + dr2;
              const fogCol = zone.centerCol + dc2;
              if (fogRow < 0 || fogRow > 7 || fogCol < 0 || fogCol > 7) continue;
              const hp = (reviewBoard ?? board)[fogRow][fogCol];
              if (!hp || hp.color !== zone.ownerColor) continue;

              const fx  = fogCol * SQ;
              const fy  = (7 - fogRow) * SQ;
              const bob = Math.sin(now / 1800 + fogRow * 1.1 + fogCol * 0.8) * 2.2;

              ctx.save();
              // Solid dark pill backing
              const pillCx = fx + SQ / 2, pillCy = fy + SQ / 2 + bob;
              ctx.beginPath();
              ctx.arc(pillCx, pillCy, SQ * 0.32, 0, Math.PI * 2);
              ctx.fillStyle = 'rgba(3,8,22,0.82)';
              ctx.fill();
              // Glowing ring
              ctx.beginPath();
              ctx.arc(pillCx, pillCy, SQ * 0.32, 0, Math.PI * 2);
              ctx.strokeStyle = `rgba(155,210,255,${0.70 + Math.sin(now / 1200 + fogCol) * 0.18})`;
              ctx.lineWidth = 2;
              ctx.shadowColor = 'rgba(130,200,255,0.9)';
              ctx.shadowBlur = 14;
              ctx.stroke();
              ctx.shadowBlur = 0;
              // ? glow pass
              ctx.font = `bold ${Math.floor(SQ * 0.50)}px "Segoe UI", sans-serif`;
              ctx.textAlign = 'center';
              ctx.textBaseline = 'middle';
              ctx.shadowColor = 'rgba(160,220,255,0.95)';
              ctx.shadowBlur = 18;
              ctx.fillStyle = 'rgba(200,235,255,0.9)';
              ctx.fillText('?', pillCx, pillCy);
              // ? crisp pass
              ctx.shadowBlur = 3;
              ctx.fillStyle = 'rgba(255,255,255,1.0)';
              ctx.fillText('?', pillCx, pillCy);
              ctx.shadowBlur = 0;
              ctx.restore();
            }
          }

          // ── 8. Zone border glow ────────────────────────────────────────────
          ctx.save();
          ctx.strokeStyle = `rgba(110,155,230,${0.38 + Math.sin(now / 2800) * 0.10})`;
          ctx.lineWidth = 2;
          ctx.shadowColor = 'rgba(100,150,230,0.45)';
          ctx.shadowBlur = 8;
          ctx.strokeRect(zx + 1.5, zy + 1.5, zw - 3, zh - 3);
          ctx.shadowBlur = 0;
          ctx.restore();
        }
      }
      const swapArc = swapRef.current;
      if (swapArc) {
        const sa = swapArc;
        const elapsed = now - swapStartT.current;
        const dur = 650;
        const t = Math.min(1, elapsed / dur);

        const cx1 = sa.sq1.col * SQ + SQ/2, cy1 = (7 - sa.sq1.row) * SQ + SQ/2;
        const cx2 = sa.sq2.col * SQ + SQ/2, cy2 = (7 - sa.sq2.row) * SQ + SQ/2;
        const mx = (cx1+cx2)/2, my = (cy1+cy2)/2;
        const dx = cx2-cx1, dy = cy2-cy1, len = Math.sqrt(dx*dx+dy*dy)||1;
        const bulge = Math.min(80, len*0.5);
        const qx1 = mx - (dy/len)*bulge, qy1 = my + (dx/len)*bulge;
        const qx2 = mx + (dy/len)*bulge, qy2 = my - (dx/len)*bulge;

        const [r1c,g1c,b1c] = hexToRgb(sa.color1.startsWith('#') ? sa.color1 : '#4ade80');
        const [r2c,g2c,b2c] = hexToRgb(sa.color2.startsWith('#') ? sa.color2 : '#60a5fa');

        const alpha = t < 0.8 ? 1 : 1 - (t-0.8)/0.2;

        ctx.fillStyle = `rgba(${r1c},${g1c},${b1c},${0.18 * alpha})`;
        ctx.fillRect(sa.sq1.col*SQ, (7-sa.sq1.row)*SQ, SQ, SQ);
        ctx.strokeStyle = `rgba(${r1c},${g1c},${b1c},${0.9 * alpha})`;
        ctx.lineWidth = 2.5;
        ctx.strokeRect(sa.sq1.col*SQ+1.5, (7-sa.sq1.row)*SQ+1.5, SQ-3, SQ-3);

        ctx.fillStyle = `rgba(${r2c},${g2c},${b2c},${0.18 * alpha})`;
        ctx.fillRect(sa.sq2.col*SQ, (7-sa.sq2.row)*SQ, SQ, SQ);
        ctx.strokeStyle = `rgba(${r2c},${g2c},${b2c},${0.9 * alpha})`;
        ctx.lineWidth = 2.5;
        ctx.strokeRect(sa.sq2.col*SQ+1.5, (7-sa.sq2.row)*SQ+1.5, SQ-3, SQ-3);

        const drawArc = (
          x1: number, y1: number, qx: number, qy: number, x2: number, y2: number,
          color: string, delay: number,
        ) => {
          const arcT = Math.min(1, Math.max(0, (t - delay) / (1 - delay)));
          if (arcT <= 0) return;
          ctx.save();
          ctx.beginPath();
          const steps = 40;
          ctx.moveTo(x1, y1);
          for (let i = 1; i <= Math.floor(steps * arcT); i++) {
            const s = i / steps;
            const bx2 = (1-s)*(1-s)*x1 + 2*(1-s)*s*qx + s*s*x2;
            const by2 = (1-s)*(1-s)*y1 + 2*(1-s)*s*qy + s*s*y2;
            ctx.lineTo(bx2, by2);
          }
          const [rr,gg,bb] = hexToRgb(color.startsWith('#') ? color : '#ffffff');
          ctx.strokeStyle = `rgba(${rr},${gg},${bb},${0.2 * alpha})`;
          ctx.lineWidth = 9;
          ctx.lineCap = 'round';
          ctx.stroke();
          ctx.strokeStyle = `rgba(${rr},${gg},${bb},${0.95 * alpha})`;
          ctx.lineWidth = 2.5;
          ctx.stroke();
          ctx.restore();

          if (arcT < 1) {
            const s = arcT;
            const dotX = (1-s)*(1-s)*x1 + 2*(1-s)*s*qx + s*s*x2;
            const dotY = (1-s)*(1-s)*y1 + 2*(1-s)*s*qy + s*s*y2;
            ctx.beginPath();
            ctx.arc(dotX, dotY, 5, 0, Math.PI*2);
            ctx.fillStyle = color;
            ctx.shadowColor = color;
            ctx.shadowBlur = 10;
            ctx.fill();
            ctx.shadowBlur = 0;
          }
        };

        drawArc(cx1, cy1, qx1, qy1, cx2, cy2, sa.color1, 0);
        drawArc(cx2, cy2, qx2, qy2, cx1, cy1, sa.color2, 0.07);

        for (let ring = 0; ring < 2; ring++) {
          const ringT = Math.min(1, Math.max(0, (t - ring*0.08)));
          if (ringT <= 0) continue;
          const ringR = ringT * SQ * 0.7;
          const ringAlpha = (1 - ringT) * alpha;
          [[cx1, r1c, g1c, b1c], [cx2, r2c, g2c, b2c]].forEach(([cx, r, g, b], i) => {
            const cy = i === 0 ? cy1 : cy2;
            ctx.strokeStyle = `rgba(${r},${g},${b},${ringAlpha})`;
            ctx.lineWidth = 2.5 - ring * 0.8;
            ctx.beginPath();
            ctx.arc(cx as number, cy, ringR, 0, Math.PI*2);
            ctx.stroke();
          });
        }

        const flash = ctx.createRadialGradient(mx, my, 0, mx, my, SQ*0.5);
        flash.addColorStop(0, `rgba(255,255,255,${0.3*alpha})`);
        flash.addColorStop(1, 'transparent');
        ctx.fillStyle = flash;
        ctx.beginPath();
        ctx.arc(mx, my, SQ*0.5, 0, Math.PI*2);
        ctx.fill();
      }

      // ── Transform animation ───────────────────────────────────────────────
      const ta = transformRef.current;
      if (ta) {
        if (!transformSpawned.current) {
          const elapsed = now - ta.startTime;
          if (elapsed >= ta.startTime + 400 || elapsed > 400) {
            spawnTransformParticles(ta);
            transformSpawned.current = true;
          }
        }
        paintTransformAnim(ctx, ta, now);
      }

      // ── Sniper animation ──────────────────────────────────────────────────
      const sa = sniperRef.current;
      if (sa) {
        const elapsed = now - sa.startTime;
        if (elapsed < SNIPER_DURATION) {
          paintSniperAnim(ctx, sa, now);
        } else {
          sniperRef.current = null;
        }
      }

      // ── Teleport animation ───────────────────────────────────────────────
      const tpa = teleportRef.current;
      if (tpa) {
        const elapsed = now - tpa.startTime;
        if (elapsed < TELEPORT_DURATION) {
          paintTeleportAnim(ctx, tpa, now);
        } else {
          teleportRef.current = null;
        }
      }

      // ── Jump animation ────────────────────────────────────────────────────
      const ja = jumpRef.current;
      if (ja) {
        const elapsed = now - ja.startTime;
        if (elapsed < JUMP_DURATION) {
          paintJumpAnim(ctx, ja, now);
        } else {
          jumpRef.current = null;
        }
      }

      // ── Reverse animation ─────────────────────────────────────────────────
      const ra = reverseRef.current;
      if (ra) {
        const elapsed = now - ra.startTime;
        if (elapsed < REVERSE_DURATION) {
          paintReverseAnim(ctx, ra, now, W, H);
        } else {
          reverseRef.current = null;
        }
      }

      // ── Sacrifice animation ───────────────────────────────────────────────
      const sacA = sacrificeRef.current;
      if (sacA) {
        const elapsed = now - sacA.startTime;
        if (elapsed < SACRIFICE_DURATION) {
          paintSacrificeAnim(ctx, sacA, now);
        } else {
          sacrificeRef.current = null;
        }
      }

      // ── Mind control animation ────────────────────────────────────────────
      const mcA = mindControlRef.current;
      if (mcA) {
        const elapsed = now - mcA.startTime;
        if (elapsed < MINDCONTROL_DURATION) {
          paintMindControlAnim(ctx, mcA, now);
        } else {
          mindControlRef.current = null;
        }
      }

      // ── Fuse animation ────────────────────────────────────────────────────
      const fusA = fuseRef.current;
      if (fusA) {
        const elapsed = now - fusA.startTime;
        if (elapsed < FUSE_DURATION) {
          paintFuseAnim(ctx, fusA, now);
        } else {
          fuseRef.current = null;
        }
      }

      // ── Particles ─────────────────────────────────────────────────────────
      particles.current = particles.current.filter(p => p.life > 0);
      for (const p of particles.current) {
        p.x  += p.vx * dt;
        p.y  += p.vy * dt;
        p.vy += 0.12 * dt;
        p.life -= dt / (p.type === 'smoke' ? 25 : p.type === 'swap' ? 20 : 18);

        const alpha = Math.max(0, p.life);
        ctx.save();
        ctx.globalAlpha = alpha;

        if (p.type === 'smoke') {
          ctx.beginPath();
          ctx.arc(p.x, p.y, p.r * (1.5 - alpha * 0.5), 0, Math.PI*2);
          ctx.fillStyle = `rgba(60,60,60,0.6)`;
          ctx.fill();
        } else {
          ctx.beginPath();
          ctx.arc(p.x, p.y, p.r, 0, Math.PI*2);
          ctx.fillStyle = p.color;
          ctx.shadowColor = p.color;
          ctx.shadowBlur = p.type === 'swap' ? 6 : 4;
          ctx.fill();
          ctx.shadowBlur = 0;
        }
        ctx.restore();
      }

      rafRef.current = requestAnimationFrame(draw);
    };
    cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(rafRef.current);
  }, [
    displayBoard, sel, hints, lm, check, kingPos,
    cardHighlight, doubleMoveHighlight, bombPieces, bombExploding,
    lavaSquares, lavaExploding, isReviewing, doubleMove, transformAnim, sniperAnim, reverseAnim, sacrificeAnim, sacrificeSelectedSquares, mindControlAnim, mindControlTargetSquare, fuseAnim, fuseSelectedSq,
    fogZones, viewerColor, analysisArrows, colorBlindMode, premove,
    boardPx,
  ]);

  const [localDrag, setLocalDrag] = React.useState<Sq | null>(null);
  const [localDragPos, setLocalDragPos] = React.useState<{ x: number; y: number } | null>(null);
  const [annotationStart, setAnnotationStart] = React.useState<Sq | null>(null);
  const [annotationTarget, setAnnotationTarget] = React.useState<Sq | null>(null);
  const [focusedSquare, setFocusedSquare] = React.useState<{row: number, col: number} | null>(null);
  const [lastMoveAnnouncement, setLastMoveAnnouncement] = React.useState('');
  const localDragRef    = React.useRef<Sq | null>(null);
  const localDragPosRef = React.useRef<{ x: number; y: number } | null>(null);
  const annotationStartRef = React.useRef<Sq | null>(null);
  const annotationTargetRef = React.useRef<Sq | null>(null);
  React.useEffect(() => { localDragRef.current = localDrag; }, [localDrag]);
  React.useEffect(() => { localDragPosRef.current = localDragPos; }, [localDragPos]);
  React.useEffect(() => { annotationStartRef.current = annotationStart; }, [annotationStart]);
  React.useEffect(() => { annotationTargetRef.current = annotationTarget; }, [annotationTarget]);
  React.useEffect(() => {
    if (lm) {
      const fromFile = FILES[lm.from.col];
      const fromRank = lm.from.row + 1;
      const toFile = FILES[lm.to.col];
      const toRank = lm.to.row + 1;
      setLastMoveAnnouncement(`Move from ${fromFile}${fromRank} to ${toFile}${toRank}`);
    }
  }, [lm]);

  const getSquare = (e: React.MouseEvent): Sq => {
    const rect = canvasRef.current!.getBoundingClientRect();
    const x = e.clientX - rect.left, y = e.clientY - rect.top;
    const effSQ = rect.width / 8;
    return { row: 7 - Math.floor(y / effSQ), col: Math.floor(x / effSQ) };
  };

  const getTouchSquare = (e: TouchEvent): Sq | null => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const rect = canvas.getBoundingClientRect();
    const touch = e.touches[0] || e.changedTouches[0];
    if (!touch) return null;
    const x = touch.clientX - rect.left, y = touch.clientY - rect.top;
    const effSQ = rect.width / 8;
    return { row: 7 - Math.floor(y / effSQ), col: Math.floor(x / effSQ) };
  };

  const touchStartSq = React.useRef<Sq | null>(null);
  const touchMoved = React.useRef(false);

  const handleTouchStart = (e: TouchEvent) => {
    if (e.cancelable) e.preventDefault();
    touchMoved.current = false;
    const sq = getTouchSquare(e);
    if (!sq || sq.row < 0 || sq.row > 7 || sq.col < 0 || sq.col > 7) return;
    touchStartSq.current = sq;
    if (cardPending || isReviewing) return;
    onClearAnalysisArrows();
    const p = displayBoard[sq.row]?.[sq.col];
    if (viewerColor && viewerColor === turn && p?.color === viewerColor) {
      const canvas = canvasRef.current!;
      const rect = canvas.getBoundingClientRect();
      const touch = e.touches[0];
      const pos = { x: touch.clientX - rect.left, y: touch.clientY - rect.top };
      setLocalDrag(sq);
      setLocalDragPos(pos);
      const fakeEvent = { clientX: touch.clientX, clientY: touch.clientY, target: canvas, stopPropagation() {}, preventDefault() {} } as unknown as React.MouseEvent;
      onDragStart(fakeEvent, sq.row, sq.col);
    }
  };

  const handleTouchMove = (e: TouchEvent) => {
    if (e.cancelable) e.preventDefault();
    if (!localDrag) return;
    touchMoved.current = true;
    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    const touch = e.touches[0];
    setLocalDragPos({ x: touch.clientX - rect.left, y: touch.clientY - rect.top });
  };

  const handleTouchEnd = (e: TouchEvent) => {
    e.preventDefault();
    const sq = getTouchSquare(e);
    if (localDrag) {
      if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
        onDrop(sq.row, sq.col);
      }
      setLocalDrag(null);
      setLocalDragPos(null);
    } else if (!touchMoved.current && touchStartSq.current) {
      const start = touchStartSq.current;
      if (sq && sq.row === start.row && sq.col === start.col) {
        onClick(sq.row, sq.col);
      }
    }
    touchStartSq.current = null;
  };

  const handleTouchCancel = () => {
    if (localDragRef.current) {
      setLocalDrag(null);
      setLocalDragPos(null);
    }
    touchStartSq.current = null;
    touchMoved.current = false;
  };



  const handleMouseDown = (e: React.MouseEvent) => {
    const sq = getSquare(e);
    if (!sq || sq.row < 0 || sq.row > 7 || sq.col < 0 || sq.col > 7) return;
    if (e.button === 2) {
      e.preventDefault();
      setAnnotationStart(sq);
      setAnnotationTarget(sq);
      return;
    }
    if (e.button !== 0) return;
    if (cardPending || isReviewing) return;
    onClearAnalysisArrows();
    const p = displayBoard[sq.row]?.[sq.col];
    if (viewerColor && viewerColor === turn && p?.color === viewerColor) {
      setLocalDrag(sq);
      setLocalDragPos({ x: e.clientX - canvasRef.current!.getBoundingClientRect().left, y: e.clientY - canvasRef.current!.getBoundingClientRect().top });
      onDragStart(e, sq.row, sq.col);
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (annotationStart) {
      const sq = getSquare(e);
      if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
        setAnnotationTarget(sq);
      }
      return;
    }
    if (!localDrag) return;
    const rect = canvasRef.current!.getBoundingClientRect();
    setLocalDragPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
  };

  const handleMouseUp = (e: React.MouseEvent) => {
    if (annotationStart) {
      const sq = getSquare(e);
      if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
        onToggleAnalysisArrow(annotationStart, sq);
      }
      setAnnotationStart(null);
      setAnnotationTarget(null);
      return;
    }
    if (localDrag) {
      const sq = getSquare(e);
      if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
        onDrop(sq.row, sq.col);
      }
      setLocalDrag(null);
      setLocalDragPos(null);
    }
  };

  const handleClick = (e: React.MouseEvent) => {
    if (localDrag) return;
    if (annotationStart) return;
    const sq = getSquare(e);
    if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
      onClick(sq.row, sq.col);
    }
  };

  const handleMouseLeave = () => {
    if (annotationStartRef.current) {
      setAnnotationStart(null);
      setAnnotationTarget(null);
    }
    if (localDragRef.current) {
      setLocalDrag(null);
      setLocalDragPos(null);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    let row = focusedSquare?.row ?? 4;
    let col = focusedSquare?.col ?? 4;
    switch (e.key) {
      case 'ArrowUp':
        e.preventDefault();
        row = Math.min(7, row + 1);
        break;
      case 'ArrowDown':
        e.preventDefault();
        row = Math.max(0, row - 1);
        break;
      case 'ArrowLeft':
        e.preventDefault();
        col = Math.max(0, col - 1);
        break;
      case 'ArrowRight':
        e.preventDefault();
        col = Math.min(7, col + 1);
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        if (focusedSquare) onClick(focusedSquare.row, focusedSquare.col);
        return;
      case 'Escape':
        e.preventDefault();
        setFocusedSquare(null);
        return;
      default:
        return;
    }
    setFocusedSquare({ row, col });
  };

  return (
    <>
      <canvas
        ref={canvasRef}
        width={Math.round(W * dpr)}
        height={Math.round(H * dpr)}
        role="application"
        aria-label="Chess board"
        tabIndex={0}
        style={{
          display:'block',
          width: '100%',
          height: '100%',
          touchAction: 'none',
          cursor: annotationStart ? 'crosshair' : (cardPending ? 'crosshair' : (localDrag ? 'grabbing' : 'pointer')),
        }}
        onContextMenu={(e) => e.preventDefault()}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onClick={handleClick}
        onMouseLeave={handleMouseLeave}
        onTouchStart={handleTouchStart as any}
        onTouchMove={handleTouchMove as any}
        onTouchEnd={handleTouchEnd as any}
        onTouchCancel={handleTouchCancel as any}
        onKeyDown={handleKeyDown}
      />
      <div aria-live="polite" className="sr-only">
        {lastMoveAnnouncement}
      </div>
    </>
  );
});
