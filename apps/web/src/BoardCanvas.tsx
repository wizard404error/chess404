import React from 'react';
import type { Board, Piece, PieceType, PieceColor, Sq, CardPendingState, BombPiece, LavaSquare, DoubleMove } from './types';
import { SQ, FILES } from './constants';

// ─── BoardCanvas ─────────────────────────────────────────────────────────────
export interface TransformAnim {
  sq: Sq;
  direction: 'up' | 'down';       // up = promote, down = demote
  fromType: PieceType;
  toType: PieceType;
  color: PieceColor;
  startTime: number;
}

export interface SniperAnim {
  sq: Sq;
  pieceType: PieceType;
  pieceColor: PieceColor;
  variant: 'sniper' | 'badsniper'; // red laser vs grey corrupt
  startTime: number;
}

export interface TeleportAnim {
  fromSq: Sq;
  toSq: Sq;
  pieceType: PieceType;
  pieceColor: PieceColor;
  startTime: number;
}

export interface JumpAnim {
  fromSq: Sq;
  toSq: Sq;
  pieceType: PieceType;
  pieceColor: PieceColor;
  captured: boolean; // true if landing on an enemy piece
  startTime: number;
}

export interface SacrificeAnim {
  squares: { row: number; col: number }[]; // all sacrificed squares
  startTime: number;
}

export interface MindControlAnim {
  targetSq: Sq;                // enemy piece being stolen
  playerColor: PieceColor;     // the player casting the spell
  pieceType: PieceType;
  startTime: number;
}

export interface FuseAnim {
  sq1: Sq;            // consumed piece (flies toward sq2)
  sq2: Sq;            // surviving piece (absorbs)
  type1: PieceType;   // consumed type
  type2: PieceType;   // surviving type
  resultType?: PieceType; // final merged piece type (e.g. 'queen' for bishop+rook)
  color: PieceColor;
  startTime: number;
}

export interface ReverseAnim {
  startTime: number;
}

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
  viewerColor: PieceColor;
  invisibleUnder?: { row: number; col: number; piece: Piece; ownerColor: PieceColor } | null;
}

// Preload all piece images once
const PIECE_IMAGES: Partial<Record<string, HTMLImageElement>> = {};
const PIECE_KEYS = (['white','black'] as PieceColor[]).flatMap(color =>
  (['king','queen','rook','bishop','knight','pawn'] as PieceType[]).map(type => `${color}_${type}`)
);
if (typeof Image !== 'undefined') {
  PIECE_KEYS.forEach(key => {
    const img = new Image();
    img.onerror = () => {
      delete PIECE_IMAGES[key];
    };
    img.src = `/pieces/${key}.svg`;
    PIECE_IMAGES[key] = img;
  });
}

// ─── Fused piece images ───────────────────────────────────────────────────────
// Key format: `${color}_${typeA}_${typeB}` where typeA < typeB alphabetically
// so knight+rook and rook+knight both resolve to the same image key
const FUSED_IMAGES: Partial<Record<string, HTMLImageElement>> = {};
const KNOWN_FUSED_COMBOS: [PieceType, PieceType][] = [
  ['knight', 'rook'],
  // add more here as sprites are added e.g. ['knight', 'bishop'], ['pawn', 'rook']
];
if (typeof Image !== 'undefined') {
  (['white','black'] as PieceColor[]).forEach(color => {
    KNOWN_FUSED_COMBOS.forEach(([a, b]) => {
      const key = `${color}_${a}_${b}`; // always alphabetical: a < b
      const img = new Image();
      img.onerror = () => {
        delete FUSED_IMAGES[key];
      };
      img.src = `/pieces/${key}.png`;
      FUSED_IMAGES[key] = img;
    });
  });
}

function isUsableImage(img: HTMLImageElement | null | undefined): img is HTMLImageElement {
  return Boolean(img && img.complete && img.naturalWidth > 0 && img.naturalHeight > 0);
}

// Returns the canonical fused image key (alphabetical type order) or null if not loaded
function getFusedImage(color: PieceColor, typeA: PieceType, typeB: PieceType): HTMLImageElement | null {
  const [t1, t2] = [typeA, typeB].sort() as [PieceType, PieceType];
  const key = `${color}_${t1}_${t2}`;
  const img = FUSED_IMAGES[key];
  return isUsableImage(img) ? img : null;
}

function parseColor(css: string): [number, number, number, number] {
  const m = css.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/);
  if (m) return [+m[1], +m[2], +m[3], m[4] !== undefined ? +m[4] : 1];
  const h = css.replace('#','');
  if (h.length === 6) {
    return [parseInt(h.slice(0,2),16), parseInt(h.slice(2,4),16), parseInt(h.slice(4,6),16), 1];
  }
  return [255,255,255,1];
}

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#','');
  return [parseInt(h.slice(0,2),16), parseInt(h.slice(2,4),16), parseInt(h.slice(4,6),16)];
}

const easeOut  = (t: number) => 1 - Math.pow(1 - t, 3);
const easeIn   = (t: number) => t * t * t;
const easeInOut = (t: number) => t < 0.5 ? 4*t*t*t : 1 - Math.pow(-2*t+2,3)/2;
const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
const lerp  = (a: number, b: number, t: number) => a + (b - a) * t;


// ─── Teleport Animation ───────────────────────────────────────────────────────
const TELEPORT_DURATION = 1400;

function paintTeleportAnim(
  ctx: CanvasRenderingContext2D,
  anim: TeleportAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / TELEPORT_DURATION, 1);
  if (t >= 1) return;

  const img = PIECE_IMAGES[`${anim.pieceColor}_${anim.pieceType}`];

  const fx = anim.fromSq.col * SQ, fy = (7 - anim.fromSq.row) * SQ;
  const tx = anim.toSq.col   * SQ, ty = (7 - anim.toSq.row)   * SQ;
  const fcx = fx + SQ / 2,        fcy = fy + SQ / 2;
  const tcx = tx + SQ / 2,        tcy = ty + SQ / 2;

  // colors: electric purple/cyan
  const r1 = 180, g1 = 60,  b1 = 255; // purple
  const r2 = 0,   g2 = 220, b2 = 255; // cyan

  // ── Phase 0–0.30: piece dissolves at source ──────────────────────────────
  if (t < 0.35) {
    const pt = t / 0.35;
    if (img && img.complete) {
      ctx.save();
      ctx.globalAlpha = Math.max(0, 1 - pt * 1.1);
      ctx.drawImage(img, fx, fy, SQ, SQ);
      ctx.restore();
    }
    // swirling particle ring at source
    const numP = 16;
    for (let i = 0; i < numP; i++) {
      const angle = (i / numP) * Math.PI * 2 + pt * Math.PI * 4;
      const radius = easeOut(pt) * SQ * 0.6;
      const px = fcx + Math.cos(angle) * radius;
      const py = fcy + Math.sin(angle) * radius;
      const pa = (1 - pt) * 0.9;
      const sz = lerp(4, 1, pt);
      ctx.save();
      ctx.beginPath();
      ctx.arc(px, py, sz, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${lerp(r1,r2,i/numP)|0},${lerp(g1,g2,i/numP)|0},${lerp(b1,b2,i/numP)|0},${pa})`;
      ctx.shadowColor = `rgba(${r1},${g1},${b1},0.8)`;
      ctx.shadowBlur = 8;
      ctx.fill();
      ctx.restore();
    }
    // flash ring at source
    const flashA = Math.sin(pt * Math.PI) * 0.6;
    ctx.save();
    const gFrom = ctx.createRadialGradient(fcx, fcy, 0, fcx, fcy, SQ * 0.7);
    gFrom.addColorStop(0, `rgba(255,255,255,${flashA})`);
    gFrom.addColorStop(0.4, `rgba(${r1},${g1},${b1},${flashA * 0.6})`);
    gFrom.addColorStop(1, `rgba(${r1},${g1},${b1},0)`);
    ctx.fillStyle = gFrom;
    ctx.beginPath();
    ctx.arc(fcx, fcy, SQ * 0.7, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  // ── Phase 0.20–0.60: energy streak travels from source to dest ───────────
  if (t >= 0.20 && t < 0.65) {
    const st = (t - 0.20) / 0.45;
    const streakX = lerp(fcx, tcx, easeInOut(st));
    const streakY = lerp(fcy, tcy, easeInOut(st));

    // draw trail
    const trailSteps = 12;
    for (let i = 0; i < trailSteps; i++) {
      const tp = i / trailSteps;
      const trailT = Math.max(0, st - tp * 0.3);
      const trailX = lerp(fcx, tcx, easeInOut(trailT));
      const trailY = lerp(fcy, tcy, easeInOut(trailT));
      const ta2 = (1 - tp) * 0.7 * Math.sin(st * Math.PI);
      const tr2 = lerp(3, 0.5, tp);
      ctx.save();
      ctx.beginPath();
      ctx.arc(trailX, trailY, tr2, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${lerp(r1,r2,tp)|0},${lerp(g1,g2,tp)|0},${lerp(b1,b2,tp)|0},${ta2})`;
      ctx.shadowColor = `rgba(${r2},${g2},${b2},0.9)`;
      ctx.shadowBlur = 10;
      ctx.fill();
      ctx.restore();
    }

    // bright orb at front
    ctx.save();
    const orbGrad = ctx.createRadialGradient(streakX, streakY, 0, streakX, streakY, SQ * 0.25);
    orbGrad.addColorStop(0, `rgba(255,255,255,${Math.sin(st * Math.PI) * 0.95})`);
    orbGrad.addColorStop(0.4, `rgba(${r2},${g2},${b2},${Math.sin(st * Math.PI) * 0.8})`);
    orbGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = orbGrad;
    ctx.beginPath();
    ctx.arc(streakX, streakY, SQ * 0.25, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  // ── Phase 0.55–1.0: piece materialises at destination ───────────────────
  if (t >= 0.55) {
    const mt = (t - 0.55) / 0.45;
    if (img && img.complete) {
      ctx.save();
      ctx.globalAlpha = Math.min(1, easeOut(mt));
      ctx.drawImage(img, tx, ty, SQ, SQ);
      ctx.restore();
    }
    // swirling ring collapses inward at dest
    const numP = 16;
    for (let i = 0; i < numP; i++) {
      const angle = (i / numP) * Math.PI * 2 - mt * Math.PI * 3;
      const radius = (1 - easeOut(mt)) * SQ * 0.65;
      const px = tcx + Math.cos(angle) * radius;
      const py = tcy + Math.sin(angle) * radius;
      const pa = (1 - mt) * 0.85;
      const sz = lerp(4, 1, mt);
      ctx.save();
      ctx.beginPath();
      ctx.arc(px, py, sz, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${lerp(r2,r1,i/numP)|0},${lerp(g2,g1,i/numP)|0},${lerp(b2,b1,i/numP)|0},${pa})`;
      ctx.shadowColor = `rgba(${r2},${g2},${b2},0.8)`;
      ctx.shadowBlur = 8;
      ctx.fill();
      ctx.restore();
    }
    // flash ring at dest
    const flashA = Math.sin(mt * Math.PI) * 0.5;
    ctx.save();
    const gTo = ctx.createRadialGradient(tcx, tcy, 0, tcx, tcy, SQ * 0.7);
    gTo.addColorStop(0, `rgba(255,255,255,${flashA})`);
    gTo.addColorStop(0.4, `rgba(${r2},${g2},${b2},${flashA * 0.6})`);
    gTo.addColorStop(1, `rgba(${r2},${g2},${b2},0)`);
    ctx.fillStyle = gTo;
    ctx.beginPath();
    ctx.arc(tcx, tcy, SQ * 0.7, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }
}

// ─── Jump Card Animation ──────────────────────────────────────────────────────
const JUMP_DURATION = 1100;

function paintJumpAnim(
  ctx: CanvasRenderingContext2D,
  anim: JumpAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / JUMP_DURATION, 1);
  if (t >= 1) return;

  const img = PIECE_IMAGES[`${anim.pieceColor}_${anim.pieceType}`];

  const fx = anim.fromSq.col * SQ, fy = (7 - anim.fromSq.row) * SQ;
  const tx = anim.toSq.col   * SQ, ty = (7 - anim.toSq.row)   * SQ;
  const fcx = fx + SQ / 2,         fcy = fy + SQ / 2;
  const tcx = tx + SQ / 2,         tcy = ty + SQ / 2;

  // Jump arc height — scales with distance, capped generously
  const dist = Math.hypot(tcx - fcx, tcy - fcy);
  const arcHeight = Math.min(dist * 0.75, SQ * 2.8);

  // Kangaroo green/gold palette
  const r1 = 74,  g1 = 222, b1 = 128;  // vivid green
  const r2 = 250, g2 = 204, b2 = 21;   // gold

  // ── Phase 0–0.12: Coil / launch flash at source ───────────────────────────
  if (t < 0.16) {
    const pt = t / 0.16;
    // Compress ring
    const ringR = (1 - easeOut(pt)) * SQ * 0.5 + 4;
    ctx.save();
    ctx.beginPath();
    ctx.arc(fcx, fcy, ringR, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${r1},${g1},${b1},${(1-pt)*0.9})`;
    ctx.lineWidth = 3;
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 14;
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.restore();

    // Dust puff at launch
    const numDust = 8;
    for (let i = 0; i < numDust; i++) {
      const angle = (i / numDust) * Math.PI * 2;
      const dustR = easeOut(pt) * SQ * 0.45;
      const dx2 = fcx + Math.cos(angle) * dustR;
      const dy2 = fcy + Math.sin(angle) * dustR;
      const da = (1 - pt) * 0.6;
      ctx.save();
      ctx.beginPath();
      ctx.arc(dx2, dy2, lerp(4, 1, pt), 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${r1},${g1},${b1},${da})`;
      ctx.fill();
      ctx.restore();
    }
  }

  // ── Phase 0.08–0.78: Piece arcs through air ───────────────────────────────
  if (t >= 0.08 && t < 0.82) {
    const at = (t - 0.08) / 0.74;
    const easedT = easeInOut(at);

    // Parabolic arc: linear x, quadratic y
    const px2 = lerp(fcx, tcx, easedT);
    const py2 = lerp(fcy, tcy, easedT) - arcHeight * 4 * easedT * (1 - easedT);

    // Motion trail (green → gold along the arc)
    const trailSteps = 14;
    for (let i = 0; i < trailSteps; i++) {
      const ti = Math.max(0, at - (i / trailSteps) * 0.35);
      const eti = easeInOut(ti);
      const trailX = lerp(fcx, tcx, eti);
      const trailY = lerp(fcy, tcy, eti) - arcHeight * 4 * eti * (1 - eti);
      const ta2 = ((1 - i / trailSteps) * 0.55) * Math.sin(at * Math.PI);
      const tr2 = lerp(3.5, 0.5, i / trailSteps);
      const frac = i / trailSteps;
      ctx.save();
      ctx.beginPath();
      ctx.arc(trailX, trailY, tr2, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${lerp(r2,r1,frac)|0},${lerp(g2,g1,frac)|0},${lerp(b2,b1,frac)|0},${ta2})`;
      ctx.shadowColor = `rgba(${r1},${g1},${b1},0.6)`;
      ctx.shadowBlur = 8;
      ctx.fill();
      ctx.restore();
    }

    // Draw the piece, rotated slightly to follow trajectory
    if (img && img.complete) {
      // Tangent rotation: derivative of arc position
      const dEased = easeInOut(Math.min(at + 0.01, 1)) - easedT;
      const dPx = (tcx - fcx) * dEased / 0.01;
      const dPy = (tcy - fcy) * dEased / 0.01 - arcHeight * 4 * (1 - 2 * easedT) * dEased / 0.01;
      const angle = Math.atan2(dPy, dPx) * 0.25; // subtle tilt

      // Shadow beneath piece (on the board baseline)
      const shadowY = lerp(fcy, tcy, easedT);
      const shadowScale = lerp(0.55, 0.95, 1 - Math.abs(easedT - 0.5) * 2);
      ctx.save();
      ctx.globalAlpha = 0.22 * shadowScale;
      ctx.scale(1, 0.3);
      ctx.beginPath();
      ctx.ellipse(px2, shadowY / 0.3, SQ * 0.38 * shadowScale, SQ * 0.18, 0, 0, Math.PI * 2);
      ctx.fillStyle = 'rgba(0,0,0,0.7)';
      ctx.fill();
      ctx.restore();

      ctx.save();
      ctx.translate(px2, py2);
      ctx.rotate(angle);
      ctx.globalAlpha = 1;
      // Piece glow at apex
      const apexGlow = Math.sin(at * Math.PI);
      if (apexGlow > 0.3) {
        ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
        ctx.shadowBlur = 18 * apexGlow;
      }
      ctx.drawImage(img, -SQ / 2 + 2, -SQ / 2 + 2, SQ - 4, SQ - 4);
      ctx.shadowBlur = 0;
      ctx.restore();
    }
  }

  // ── Phase 0.74–1.0: Landing impact ───────────────────────────────────────
  if (t >= 0.74) {
    const lt = (t - 0.74) / 0.26;

    // Shockwave ring
    const shockR = easeOut(lt) * SQ * 0.65;
    const shockA = (1 - lt) * 0.9;
    ctx.save();
    ctx.beginPath();
    ctx.arc(tcx, tcy, shockR, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${r2},${g2},${b2},${shockA})`;
    ctx.lineWidth = 3.5 - lt * 2;
    ctx.shadowColor = `rgb(${r2},${g2},${b2})`;
    ctx.shadowBlur = 16 * (1 - lt);
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.restore();

    // Second softer ring
    const shock2R = easeOut(Math.min(lt * 1.4, 1)) * SQ * 0.9;
    const shock2A = Math.max(0, (1 - lt * 1.4)) * 0.55;
    ctx.save();
    ctx.beginPath();
    ctx.arc(tcx, tcy, shock2R, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${r1},${g1},${b1},${shock2A})`;
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.restore();

    // Capture burst
    if (anim.captured && lt < 0.6) {
      const numSparks = 10;
      for (let i = 0; i < numSparks; i++) {
        const angle = (i / numSparks) * Math.PI * 2;
        const spd = easeOut(lt) * SQ * 0.65;
        const sx2 = tcx + Math.cos(angle) * spd;
        const sy2 = tcy + Math.sin(angle) * spd;
        const sa2 = (1 - lt / 0.6) * 0.85;
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(tcx + Math.cos(angle) * spd * 0.5, tcy + Math.sin(angle) * spd * 0.5);
        ctx.lineTo(sx2, sy2);
        ctx.strokeStyle = i % 2 === 0
          ? `rgba(255,80,80,${sa2})`
          : `rgba(255,220,80,${sa2})`;
        ctx.lineWidth = 2;
        ctx.shadowColor = '#ff5050';
        ctx.shadowBlur = 6;
        ctx.stroke();
        ctx.shadowBlur = 0;
        ctx.restore();
      }
    }

    // Flash glow on landing square
    const flashA = Math.sin(lt * Math.PI) * 0.5;
    ctx.save();
    const flashGrad = ctx.createRadialGradient(tcx, tcy, 0, tcx, tcy, SQ * 0.6);
    flashGrad.addColorStop(0, `rgba(255,255,255,${flashA})`);
    flashGrad.addColorStop(0.4, `rgba(${r2},${g2},${b2},${flashA * 0.7})`);
    flashGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = flashGrad;
    ctx.fillRect(tx, ty, SQ, SQ);
    ctx.restore();
  }

  // ── Label ─────────────────────────────────────────────────────────────────
  if (t > 0.08 && t < 0.92) {
    const lt = (t - 0.08) / 0.84;
    const la = lt < 0.15 ? lt / 0.15 : lt > 0.75 ? 1 - (lt - 0.75) / 0.25 : 1;

    // Label floats above the arc apex
    const at2 = clamp((t - 0.08) / 0.74, 0, 1);
    const eti2 = easeInOut(at2);
    const labelX = lerp(fcx, tcx, eti2);
    const labelY2 = lerp(fcy, tcy, eti2) - arcHeight * 4 * eti2 * (1 - eti2) - SQ * 0.65;

    ctx.save();
    ctx.font = 'bold 12px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 14;
    ctx.fillStyle = `rgba(255,255,255,${la})`;
    ctx.fillText(anim.captured ? '🦘 JUMP CAPTURE!' : '🦘 JUMP!', labelX, labelY2);
    ctx.shadowBlur = 0;
    ctx.restore();
  }
}





// ─── Mind Control Animation ───────────────────────────────────────────────────
const MINDCONTROL_DURATION = 2000;

function paintMindControlAnim(
  ctx: CanvasRenderingContext2D,
  anim: MindControlAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / MINDCONTROL_DURATION, 1);
  if (t >= 1) return;

  const tx = anim.targetSq.col * SQ;
  const ty = (7 - anim.targetSq.row) * SQ;
  const cx = tx + SQ / 2;
  const cy = ty + SQ / 2;

  // Psychic palette: deep violet → electric magenta → bright white
  const r1 = 139, g1 = 0,   b1 = 255;  // deep violet
  const r2 = 255, g2 = 0,   b2 = 200;  // hot magenta
  const r3 = 200, g3 = 100, b3 = 255;  // lavender
  const r4 = 255, g4 = 255, b4 = 255;  // white

  // Piece image for the controlled piece
  const img = PIECE_IMAGES[`${anim.playerColor}_${anim.pieceType}`];
  const imgEnemy = PIECE_IMAGES[`${anim.playerColor === 'white' ? 'black' : 'white'}_${anim.pieceType}`];

  // ── Phase 0–0.18: Shockwave ring snaps out — telegraphing the hit ──────────
  if (t < 0.22) {
    const pt = t / 0.22;
    const shockR = easeOut(pt) * SQ * 1.1;
    const shockA = (1 - pt) * 0.85;
    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, shockR, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${r1},${g1},${b1},${shockA})`;
    ctx.lineWidth = 4 - pt * 2.5;
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 22;
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.restore();

    // Inner bright snap
    const snapA = Math.sin(pt * Math.PI) * 0.9;
    ctx.save();
    const snapGrad = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 0.6);
    snapGrad.addColorStop(0, `rgba(${r4},${g4},${b4},${snapA * 0.8})`);
    snapGrad.addColorStop(0.3, `rgba(${r2},${g2},${b2},${snapA * 0.6})`);
    snapGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = snapGrad;
    ctx.beginPath();
    ctx.arc(cx, cy, SQ * 0.6, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  // ── Phase 0.10–0.55: Psychic tendrils spiral inward onto the piece ─────────
  if (t >= 0.08 && t < 0.60) {
    const pt = (t - 0.08) / 0.52;
    const numTendrils = 7;

    for (let i = 0; i < numTendrils; i++) {
      const phaseOffset = (i / numTendrils) * Math.PI * 2;
      const numSeg = 28;

      ctx.save();
      ctx.beginPath();
      for (let s = 0; s <= Math.floor(numSeg * easeIn(pt)); s++) {
        const segT = s / numSeg;
        // Spiral: starts at distance, wraps inward
        const radius = (1 - segT) * SQ * 1.4;
        const angle = phaseOffset + segT * Math.PI * 3.5 - pt * Math.PI * 2;
        const px2 = cx + Math.cos(angle) * radius;
        const py2 = cy + Math.sin(angle) * radius;
        if (s === 0) ctx.moveTo(px2, py2);
        else ctx.lineTo(px2, py2);
      }
      const tendrilFrac = i / numTendrils;
      const tendrilA = (0.5 + Math.sin(pt * Math.PI) * 0.4) * 0.85;
      ctx.strokeStyle = `rgba(${lerp(r1, r2, tendrilFrac) | 0},${lerp(g1, g2, tendrilFrac) | 0},${lerp(b1, b2, tendrilFrac) | 0},${tendrilA})`;
      ctx.lineWidth = 2.5 - pt * 1.2;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 12;
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // Crackling energy at tip of each tendril as they arrive
    if (pt > 0.6) {
      const arrivalT = (pt - 0.6) / 0.4;
      const numSparks = 8;
      for (let i = 0; i < numSparks; i++) {
        const angle = (i / numSparks) * Math.PI * 2 + now / 200;
        const spkR = easeOut(arrivalT) * SQ * 0.48;
        const spkX = cx + Math.cos(angle) * spkR;
        const spkY = cy + Math.sin(angle) * spkR;
        const spkA = (1 - arrivalT) * 0.9;
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(cx, cy);
        ctx.lineTo(spkX, spkY);
        ctx.strokeStyle = i % 2 === 0
          ? `rgba(${r4},${g4},${b4},${spkA})`
          : `rgba(${r2},${g2},${b2},${spkA})`;
        ctx.lineWidth = 1.8;
        ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
        ctx.shadowBlur = 10;
        ctx.stroke();
        ctx.shadowBlur = 0;
        ctx.restore();
      }
    }
  }

  // ── Phase 0.40–0.72: Piece warps — shows enemy then transitions to player ──
  if (t >= 0.38 && t < 0.78) {
    const pt = (t - 0.38) / 0.40;
    const dissolveT = pt < 0.5 ? pt / 0.5 : 1; // 0→1: enemy fades out
    const materT    = pt > 0.5 ? (pt - 0.5) / 0.5 : 0; // 0→1: player fades in

    // Psychic distortion rings closing in
    const numRings = 3;
    for (let ri = 0; ri < numRings; ri++) {
      const ringPhase = (ri / numRings);
      const ringPt = Math.max(0, Math.min(1, (pt - ringPhase * 0.2)));
      if (ringPt <= 0) continue;
      const ringR = (1 - easeOut(ringPt)) * SQ * 0.8 + 3;
      const ringA = (1 - ringPt) * 0.8;
      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, ringR, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(${r3},${g3},${b3},${ringA})`;
      ctx.lineWidth = 2.5;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 14;
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // Enemy piece dissolves upward with chromatic fringe
    if (imgEnemy && imgEnemy.complete && dissolveT < 1) {
      const alpha = Math.max(0, 1 - dissolveT * 1.3);
      const liftY = easeIn(dissolveT) * SQ * 0.35;

      // Color-shifted ghost (red tint = enemy)
      ctx.save();
      ctx.globalAlpha = alpha * 0.5;
      ctx.filter = 'hue-rotate(160deg) saturate(2)';
      ctx.drawImage(imgEnemy, tx - 3, ty - liftY - 2, SQ, SQ);
      ctx.restore();
      // Core piece
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.drawImage(imgEnemy, tx, ty - liftY, SQ, SQ);
      ctx.restore();
    }

    // Player color piece materialises with purple glow
    if (img && img.complete && materT > 0) {
      const alpha = Math.min(1, easeOut(materT));
      const scl = lerp(1.25, 1.0, easeOut(materT));

      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.translate(cx, cy);
      ctx.scale(scl, scl);
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 24 * (1 - materT);
      ctx.drawImage(img, -SQ / 2, -SQ / 2, SQ, SQ);
      ctx.shadowBlur = 0;
      ctx.restore();
    }
  }

  // ── Phase 0.65–1.0: Conversion flash + mind-branded glow ──────────────────
  if (t >= 0.62) {
    const bt = (t - 0.62) / 0.38;

    // Big psychic flash
    const flashA = Math.sin(bt * Math.PI) * 0.9;
    ctx.save();
    const flashGrad = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 1.2);
    flashGrad.addColorStop(0, `rgba(${r4},${g4},${b4},${flashA * 0.95})`);
    flashGrad.addColorStop(0.2, `rgba(${r2},${g2},${b2},${flashA * 0.85})`);
    flashGrad.addColorStop(0.55, `rgba(${r1},${g1},${b1},${flashA * 0.45})`);
    flashGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = flashGrad;
    ctx.beginPath();
    ctx.arc(cx, cy, SQ * 1.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    // Orbiting "mind control" orbs — 3 small orbs circling the piece
    const numOrbs = 3;
    for (let i = 0; i < numOrbs; i++) {
      const angle = (i / numOrbs) * Math.PI * 2 + bt * Math.PI * 4;
      const orbitR = SQ * 0.52 * easeOut(bt);
      const ox = cx + Math.cos(angle) * orbitR;
      const oy = cy + Math.sin(angle) * orbitR;
      const oa = (1 - bt) * 0.9;
      const or = 4.5 - bt * 2;
      ctx.save();
      ctx.beginPath();
      ctx.arc(ox, oy, Math.max(1, or), 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${r3},${g3},${b3},${oa})`;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 12;
      ctx.fill();
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // Persistent glow square tint (piece is now mine)
    const tintA = Math.max(0, (1 - bt) * 0.55);
    ctx.save();
    const tintGrad = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 0.72);
    tintGrad.addColorStop(0, `rgba(${r1},${g1},${b1},${tintA})`);
    tintGrad.addColorStop(1, `rgba(${r1},${g1},${b1},0)`);
    ctx.fillStyle = tintGrad;
    ctx.fillRect(tx, ty, SQ, SQ);
    ctx.restore();
  }

  // ── Floating label ──────────────────────────────────────────────────────────
  if (t > 0.35 && t < 0.95) {
    const lt = (t - 0.35) / 0.60;
    const la = lt < 0.18 ? lt / 0.18 : lt > 0.72 ? 1 - (lt - 0.72) / 0.28 : 1;
    const labelY = cy - SQ * 0.85 - easeOut(lt) * SQ * 0.5;

    ctx.save();
    ctx.font = 'bold 13px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 18;
    ctx.fillStyle = `rgba(${r3},${g3},${b3},${la})`;
    ctx.fillText('🧠 MIND CONTROLLED', cx, labelY);
    ctx.shadowBlur = 0;
    ctx.restore();
  }
}




// ─── Fuse Animation ───────────────────────────────────────────────────────────
const FUSE_DURATION = 1800;

function paintFuseAnim(
  ctx: CanvasRenderingContext2D,
  anim: FuseAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / FUSE_DURATION, 1);
  if (t >= 1) return;

  // Pixel centers
  const x1 = anim.sq1.col * SQ + SQ / 2;
  const y1 = (7 - anim.sq1.row) * SQ + SQ / 2;
  const x2 = anim.sq2.col * SQ + SQ / 2;
  const y2 = (7 - anim.sq2.row) * SQ + SQ / 2;

  const img1 = PIECE_IMAGES[`${anim.color}_${anim.type1}`];
  const img2 = PIECE_IMAGES[`${anim.color}_${anim.type2}`];
  // The piece that appears after the merge (queen for bishop+rook, otherwise type2)
  const mergedType = anim.resultType ?? anim.type2;
  const imgMerged = PIECE_IMAGES[`${anim.color}_${mergedType}`] ?? img2;

  // Gold palette
  const GOLD  = 'rgba(251,191,36,';
  const WHITE = 'rgba(255,255,255,';
  const ORANGE = 'rgba(255,140,0,';

  // ── Phase 0–0.35: Both pieces pulse/shake on their squares ──────────────────
  if (t < 0.38) {
    const pt = t / 0.38;
    const shake = Math.sin(pt * Math.PI * 14) * (1 - pt) * 5;
    const glow = 0.4 + pt * 0.5;

    if (img1 && img1.complete) {
      ctx.save();
      ctx.shadowColor = `${GOLD}${glow})`;
      ctx.shadowBlur = 16 + pt * 18;
      ctx.translate(x1 + shake, y1 + Math.cos(pt * Math.PI * 10) * (1 - pt) * 3);
      const sc = 1 + Math.sin(pt * Math.PI * 7) * 0.06;
      ctx.scale(sc, sc);
      ctx.drawImage(img1, -SQ / 2 + 3, -SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }
    if (img2 && img2.complete) {
      ctx.save();
      ctx.shadowColor = `${GOLD}${glow})`;
      ctx.shadowBlur = 16 + pt * 18;
      ctx.translate(x2 - shake, y2 + Math.cos(pt * Math.PI * 10 + 1) * (1 - pt) * 3);
      const sc = 1 + Math.sin(pt * Math.PI * 7 + 0.5) * 0.06;
      ctx.scale(sc, sc);
      ctx.drawImage(img2, -SQ / 2 + 3, -SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }

    // Energy arc between the two squares
    const arcA = pt * 0.7;
    const midX = (x1 + x2) / 2;
    const midY = (y1 + y2) / 2;
    const perpX = -(y2 - y1) * 0.3;
    const perpY =  (x2 - x1) * 0.3;
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(x1, y1);
    ctx.quadraticCurveTo(midX + perpX, midY + perpY, x2, y2);
    ctx.strokeStyle = `${GOLD}${arcA})`;
    ctx.lineWidth = 2 + pt * 2;
    ctx.shadowColor = `${GOLD}0.8)`;
    ctx.shadowBlur = 12;
    ctx.setLineDash([6, 4]);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.restore();
  }

  // ── Phase 0.30–0.65: Piece 1 flies toward piece 2 ───────────────────────────
  if (t >= 0.28 && t < 0.68) {
    const pt = (t - 0.28) / 0.40;
    const ease = 1 - Math.pow(1 - pt, 3);
    const fx = x1 + (x2 - x1) * ease;
    const fy = y1 + (y2 - y1) * ease;
    const alpha = 1 - pt * 0.3;
    const sc = 1 - pt * 0.25;

    // Trail particles
    const numTrail = 6;
    for (let i = 0; i < numTrail; i++) {
      const trailT = Math.max(0, pt - i * 0.07);
      const trailEase = 1 - Math.pow(1 - trailT, 3);
      const tx2 = x1 + (x2 - x1) * trailEase;
      const ty2 = y1 + (y2 - y1) * trailEase;
      const ta = (1 - i / numTrail) * 0.4 * (1 - pt);
      ctx.save();
      ctx.beginPath();
      ctx.arc(tx2, ty2, (SQ / 2 - 4) * (1 - i * 0.12), 0, Math.PI * 2);
      ctx.fillStyle = `${GOLD}${ta})`;
      ctx.shadowColor = `${GOLD}0.5)`;
      ctx.shadowBlur = 10;
      ctx.fill();
      ctx.restore();
    }

    if (img1 && img1.complete) {
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.shadowColor = `${GOLD}0.9)`;
      ctx.shadowBlur = 20;
      ctx.translate(fx, fy);
      ctx.scale(sc, sc);
      ctx.rotate(pt * Math.PI * 1.5); // spin as it flies
      ctx.drawImage(img1, -SQ / 2 + 3, -SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }

    // Destination piece wobbles in anticipation
    if (img2 && img2.complete) {
      const wobble = Math.sin(pt * Math.PI * 12) * 3 * (1 - pt);
      ctx.save();
      ctx.shadowColor = `${GOLD}0.8)`;
      ctx.shadowBlur = 14 + pt * 20;
      ctx.translate(x2 + wobble, y2);
      ctx.drawImage(img2, -SQ / 2 + 3, -SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }
  }

  // ── Phase 0.60–0.78: IMPACT — blinding flash at sq2 ─────────────────────────
  if (t >= 0.58 && t < 0.80) {
    const pt = (t - 0.58) / 0.22;
    const flashA = Math.sin(pt * Math.PI);

    // White burst
    ctx.save();
    const burst = ctx.createRadialGradient(x2, y2, 0, x2, y2, SQ * 1.6 * pt);
    burst.addColorStop(0, `${WHITE}${flashA * 0.95})`);
    burst.addColorStop(0.2, `${GOLD}${flashA * 0.85})`);
    burst.addColorStop(0.55, `${ORANGE}${flashA * 0.45})`);
    burst.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = burst;
    ctx.beginPath();
    ctx.arc(x2, y2, SQ * 1.6, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    // 8 impact sparks
    for (let i = 0; i < 8; i++) {
      const angle = (i / 8) * Math.PI * 2 + t * 4;
      const spkR = SQ * 0.6 * pt;
      ctx.save();
      ctx.beginPath();
      ctx.moveTo(x2, y2);
      ctx.lineTo(x2 + Math.cos(angle) * spkR, y2 + Math.sin(angle) * spkR);
      ctx.strokeStyle = i % 2 === 0 ? `${WHITE}${flashA * 0.9})` : `${GOLD}${flashA * 0.8})`;
      ctx.lineWidth = 2.5;
      ctx.shadowColor = `${GOLD}0.8)`;
      ctx.shadowBlur = 10;
      ctx.stroke();
      ctx.restore();
    }
  }

  // ── Phase 0.72–1.0: Merged piece materialises with golden aura ──────────────
  if (t >= 0.70) {
    const pt = (t - 0.70) / 0.30;
    const ease = 1 - Math.pow(1 - pt, 3);

    // Expanding ring
    const ringR = SQ * 0.5 + ease * SQ * 0.7;
    const ringA = (1 - pt) * 0.8;
    ctx.save();
    ctx.beginPath();
    ctx.arc(x2, y2, ringR, 0, Math.PI * 2);
    ctx.strokeStyle = `${GOLD}${ringA})`;
    ctx.lineWidth = 3 - pt * 2;
    ctx.shadowColor = `${GOLD}${ringA})`;
    ctx.shadowBlur = 18;
    ctx.stroke();
    ctx.restore();

    // Second ring (staggered)
    if (pt > 0.25) {
      const r2t = (pt - 0.25) / 0.75;
      const r2R = SQ * 0.5 + r2t * SQ * 0.55;
      ctx.save();
      ctx.beginPath();
      ctx.arc(x2, y2, r2R, 0, Math.PI * 2);
      ctx.strokeStyle = `${ORANGE}${(1 - r2t) * 0.6})`;
      ctx.lineWidth = 2;
      ctx.shadowColor = `${ORANGE}0.5)`;
      ctx.shadowBlur = 12;
      ctx.stroke();
      ctx.restore();
    }

    // Merged piece fades in at scale
    if (imgMerged && imgMerged.complete) {
      const sc = 1.5 - ease * 0.5; // starts big, settles to 1
      ctx.save();
      ctx.globalAlpha = ease;
      ctx.shadowColor = `${GOLD}0.9)`;
      ctx.shadowBlur = 28 * (1 - pt);
      ctx.translate(x2, y2);
      ctx.scale(sc, sc);
      ctx.drawImage(imgMerged, -SQ / 2 + 3, -SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }

    // Floating label
    const la = pt < 0.3 ? pt / 0.3 : pt > 0.7 ? 1 - (pt - 0.7) / 0.3 : 1;
    const labelText = mergedType !== anim.type2
      ? `⚗ ${anim.type2}+${anim.type1} → ${mergedType.toUpperCase()}!`
      : `⚗ ${anim.type2}+${anim.type1} FUSED`;
    ctx.save();
    ctx.font = 'bold 13px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = `${GOLD}0.9)`;
    ctx.shadowBlur = 16;
    ctx.fillStyle = `rgba(255,220,80,${la})`;
    ctx.fillText(labelText, x2, y2 - SQ * 0.9 - ease * SQ * 0.4);
    ctx.restore();
  }
}


// ─── Sacrifice Animation ──────────────────────────────────────────────────────
const SACRIFICE_DURATION = 1600;

function paintSacrificeAnim(
  ctx: CanvasRenderingContext2D,
  anim: SacrificeAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / SACRIFICE_DURATION, 1);
  if (t >= 1) return;

  // Crimson palette
  const r1 = 220, g1 = 20,  b1 = 20;   // deep crimson
  const r2 = 255, g2 = 100, b2 = 30;   // orange-red
  const r3 = 255, g3 = 220, b3 = 180;  // light warm

  // Compute centroid of all sacrificed squares
  const count = anim.squares.length;
  let sumCx = 0, sumCy = 0;
  for (const sq of anim.squares) {
    sumCx += sq.col * SQ + SQ / 2;
    sumCy += (7 - sq.row) * SQ + SQ / 2;
  }
  const cX = sumCx / count;
  const cY = sumCy / count;

  for (const sq of anim.squares) {
    const px = sq.col * SQ + SQ / 2;
    const py = (7 - sq.row) * SQ + SQ / 2;

    // ── Phase 0–0.40: Piece dissolves, crimson fire erupts ──────────────────
    if (t < 0.50) {
      const pt = t / 0.50;

      // Square fill — darkening blood pool
      const fillA = Math.min(1, pt * 1.2) * (1 - pt * 0.2);
      ctx.save();
      const sqGrad = ctx.createRadialGradient(px, py, 0, px, py, SQ * 0.72);
      sqGrad.addColorStop(0, `rgba(${r1},${g1},${b1},${fillA * 0.92})`);
      sqGrad.addColorStop(0.5, `rgba(${r1},${g1 / 2 | 0},${b1 / 3 | 0},${fillA * 0.7})`);
      sqGrad.addColorStop(1, `rgba(100,0,0,0)`);
      ctx.fillStyle = sqGrad;
      ctx.fillRect(sq.col * SQ, (7 - sq.row) * SQ, SQ, SQ);
      ctx.restore();

      // Fire ring expanding outward
      const fireR = easeOut(pt) * SQ * 0.65;
      const fireA = (1 - pt) * 0.9;
      ctx.save();
      ctx.beginPath();
      ctx.arc(px, py, fireR, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(${r2},${g2},${b2},${fireA})`;
      ctx.lineWidth = 3.5 - pt * 2;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 20 * (1 - pt);
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.restore();

      // Soul wisps rising upward
      const numWisps = 6;
      for (let i = 0; i < numWisps; i++) {
        const angle = (i / numWisps) * Math.PI * 2 + pt * Math.PI;
        const wDist = easeOut(pt) * SQ * 0.4;
        const wx = px + Math.cos(angle) * wDist;
        const wy = py + Math.sin(angle) * wDist - easeOut(pt) * SQ * 0.3;
        const wa = (1 - pt) * 0.75;
        const wr = lerp(5, 1.5, pt);
        ctx.save();
        ctx.beginPath();
        ctx.arc(wx, wy, wr, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(${r3},${g3},${b3},${wa})`;
        ctx.shadowColor = `rgb(${r2},${g2},${b2})`;
        ctx.shadowBlur = 10;
        ctx.fill();
        ctx.shadowBlur = 0;
        ctx.restore();
      }
    }

    // ── Phase 0.30–0.85: Crimson soul streams toward centroid ───────────────
    if (t >= 0.28 && t < 0.90) {
      const st = (t - 0.28) / 0.62;
      const streakProgress = easeIn(st);

      // Current position of the soul — moves from square toward centroid
      const soulX = lerp(px, cX, streakProgress);
      const soulY = lerp(py, cY, streakProgress) - Math.sin(streakProgress * Math.PI) * SQ * 0.6;

      const streamA = Math.sin(st * Math.PI) * 0.9;
      const orbR = lerp(9, 3, st);

      // Trail
      const trailSteps = 10;
      for (let i = 0; i < trailSteps; i++) {
        const ti = Math.max(0, st - (i / trailSteps) * 0.4);
        const eti = easeIn(ti);
        const trailX = lerp(px, cX, eti);
        const trailY = lerp(py, cY, eti) - Math.sin(eti * Math.PI) * SQ * 0.6;
        const ta = ((1 - i / trailSteps) * streamA * 0.6);
        const tr = lerp(4.5, 0.5, i / trailSteps);
        ctx.save();
        ctx.beginPath();
        ctx.arc(trailX, trailY, tr, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(${lerp(r3, r1, i / trailSteps) | 0},${lerp(g3 / 1.5, g1, i / trailSteps) | 0},${lerp(b3 / 1.5, b1, i / trailSteps) | 0},${ta})`;
        ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
        ctx.shadowBlur = 6;
        ctx.fill();
        ctx.shadowBlur = 0;
        ctx.restore();
      }

      // Soul orb
      ctx.save();
      const orbGrad = ctx.createRadialGradient(soulX, soulY, 0, soulX, soulY, orbR);
      orbGrad.addColorStop(0, `rgba(${r3},${g3},${b3},${streamA})`);
      orbGrad.addColorStop(0.4, `rgba(${r2},${g2},${b2},${streamA * 0.8})`);
      orbGrad.addColorStop(1, `rgba(${r1},0,0,0)`);
      ctx.fillStyle = orbGrad;
      ctx.beginPath();
      ctx.arc(soulX, soulY, orbR, 0, Math.PI * 2);
      ctx.fill();
      ctx.restore();
    }
  }

  // ── Phase 0.70–1.0: Central dark energy implosion ──────────────────────────
  if (t >= 0.68) {
    const bt = (t - 0.68) / 0.32;

    // Swirling vortex at centroid
    const numArms = 8;
    const burstR = easeOut(bt) * SQ * count * 0.35;
    for (let i = 0; i < numArms; i++) {
      const angle = (i / numArms) * Math.PI * 2 + bt * Math.PI * 3;
      const armLen = burstR;
      ctx.save();
      ctx.beginPath();
      ctx.moveTo(cX, cY);
      ctx.lineTo(
        cX + Math.cos(angle) * armLen,
        cY + Math.sin(angle) * armLen,
      );
      const armA = (1 - bt) * 0.75;
      ctx.strokeStyle = `rgba(${r2},${g2},${b2},${armA})`;
      ctx.lineWidth = 3 - bt * 2;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 14;
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // Central flash orb
    const flashA = Math.sin(bt * Math.PI) * 0.95;
    ctx.save();
    const flashGrad = ctx.createRadialGradient(cX, cY, 0, cX, cY, SQ * 1.1);
    flashGrad.addColorStop(0, `rgba(255,255,255,${flashA * 0.9})`);
    flashGrad.addColorStop(0.2, `rgba(${r2},${g2},${b2},${flashA * 0.85})`);
    flashGrad.addColorStop(0.55, `rgba(${r1},${g1 / 2 | 0},0,${flashA * 0.5})`);
    flashGrad.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = flashGrad;
    ctx.beginPath();
    ctx.arc(cX, cY, SQ * 1.1, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    // Burst sparks
    const numSparks = 14;
    for (let i = 0; i < numSparks; i++) {
      const angle = (i / numSparks) * Math.PI * 2;
      const spd = easeOut(bt) * SQ * 0.85;
      const sx = cX + Math.cos(angle) * spd;
      const sy = cY + Math.sin(angle) * spd;
      const sa = (1 - bt) * 0.85;
      ctx.save();
      ctx.beginPath();
      ctx.moveTo(cX + Math.cos(angle) * spd * 0.4, cY + Math.sin(angle) * spd * 0.4);
      ctx.lineTo(sx, sy);
      ctx.strokeStyle = i % 3 === 0
        ? `rgba(255,255,200,${sa})`
        : i % 3 === 1
          ? `rgba(${r2},${g2},${b2},${sa})`
          : `rgba(${r1},${g1},${b1},${sa})`;
      ctx.lineWidth = 2.5 - bt * 1.5;
      ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
      ctx.shadowBlur = 10;
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.restore();
    }

    // Ring pulse
    const ringR = easeOut(bt) * SQ * 1.4;
    const ringA = (1 - bt) * 0.8;
    ctx.save();
    ctx.beginPath();
    ctx.arc(cX, cY, ringR, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${r2},${g2},${b2},${ringA})`;
    ctx.lineWidth = 3.5 - bt * 2.5;
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 18;
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.restore();
  }

  // ── Floating label ──────────────────────────────────────────────────────────
  if (t > 0.1 && t < 0.88) {
    const lt = (t - 0.1) / 0.78;
    const la = lt < 0.15 ? lt / 0.15 : lt > 0.7 ? 1 - (lt - 0.7) / 0.3 : 1;
    const labelY = cY - SQ * 0.6 - easeOut(lt) * SQ * 0.8;
    ctx.save();
    ctx.font = 'bold 13px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = `rgb(${r1},${g1},${b1})`;
    ctx.shadowBlur = 16;
    ctx.fillStyle = `rgba(255,200,150,${la})`;
    ctx.fillText('🩸 SACRIFICED', cX, labelY);
    ctx.shadowBlur = 0;
    ctx.restore();
  }
}

const REVERSE_DURATION = 1200;

function paintReverseAnim(
  ctx: CanvasRenderingContext2D,
  anim: ReverseAnim,
  now: number,
  W: number,
  H: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / REVERSE_DURATION, 1);
  if (t >= 1) return;

  const cx = W / 2, cy = H / 2;

  // ── Phase 0–0.55: Swirling vortex arms converging to center ──────────────
  if (t < 0.55) {
    const p = t / 0.55;
    const numArms = 12;
    ctx.save();
    ctx.translate(cx, cy);
    for (let i = 0; i < numArms; i++) {
      const baseAngle = (i / numArms) * Math.PI * 2;
      const spinAngle = baseAngle + easeIn(p) * Math.PI * 2.5 * (i % 2 === 0 ? 1 : -1);
      const startDist = W * 0.72 * (1 - easeIn(p) * 0.85);
      const alpha = p < 0.15 ? p / 0.15 : 1;
      const [r, g, b] = i % 2 === 0 ? [255, 80, 80] : [80, 120, 255];

      ctx.beginPath();
      const steps = 20;
      for (let s = 0; s <= steps; s++) {
        const st = s / steps;
        const dist = startDist * (1 - st * 0.9);
        const angle = spinAngle + st * Math.PI * 0.7 * (i % 2 === 0 ? 1 : -1);
        const px2 = Math.cos(angle) * dist;
        const py2 = Math.sin(angle) * dist;
        s === 0 ? ctx.moveTo(px2, py2) : ctx.lineTo(px2, py2);
      }
      ctx.strokeStyle = `rgba(${r},${g},${b},${alpha * 0.7})`;
      ctx.lineWidth = 2.5;
      ctx.shadowColor = `rgb(${r},${g},${b})`;
      ctx.shadowBlur = 12;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }
    ctx.restore();
  }

  // ── Phase 0.28–0.72: Board-split flash (red left / blue right) ────────────
  if (t >= 0.28 && t < 0.72) {
    const p = (t - 0.28) / 0.44;
    const flashA = Math.sin(p * Math.PI) * 0.82;

    const leftGrad = ctx.createLinearGradient(0, 0, cx, 0);
    leftGrad.addColorStop(0, `rgba(255,60,60,0)`);
    leftGrad.addColorStop(0.6, `rgba(255,60,60,${flashA * 0.45})`);
    leftGrad.addColorStop(1, `rgba(255,60,60,${flashA * 0.7})`);
    ctx.fillStyle = leftGrad;
    ctx.fillRect(0, 0, cx, H);

    const rightGrad = ctx.createLinearGradient(cx, 0, W, 0);
    rightGrad.addColorStop(0, `rgba(80,120,255,${flashA * 0.7})`);
    rightGrad.addColorStop(0.4, `rgba(80,120,255,${flashA * 0.45})`);
    rightGrad.addColorStop(1, `rgba(80,120,255,0)`);
    ctx.fillStyle = rightGrad;
    ctx.fillRect(cx, 0, cx, H);

    // Central white seam flash
    const seamW = 6 + Math.sin(p * Math.PI) * 30;
    const seamGrad = ctx.createLinearGradient(cx - seamW, 0, cx + seamW, 0);
    seamGrad.addColorStop(0, 'rgba(255,255,255,0)');
    seamGrad.addColorStop(0.5, `rgba(255,255,255,${flashA * 0.95})`);
    seamGrad.addColorStop(1, 'rgba(255,255,255,0)');
    ctx.fillStyle = seamGrad;
    ctx.fillRect(cx - seamW, 0, seamW * 2, H);
  }

  // ── Phase 0.35–0.88: Large spinning ↺ double arrow at center ─────────────
  if (t >= 0.35 && t < 0.88) {
    const p = (t - 0.35) / 0.53;
    const arrowA = Math.sin(p * Math.PI) * 0.95;
    const arrowR = 48 + easeOut(p) * 24;
    const spin = p * Math.PI * 3;

    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(spin);

    for (let side = 0; side < 2; side++) {
      const arcStart = side === 0 ? 0.15 : Math.PI + 0.15;
      const arcEnd   = side === 0 ? Math.PI - 0.15 : Math.PI * 2 - 0.15;
      const [r, g, b] = side === 0 ? [255, 80, 80] : [80, 140, 255];

      ctx.beginPath();
      ctx.arc(0, 0, arrowR, arcStart, arcEnd);
      ctx.strokeStyle = `rgba(${r},${g},${b},${arrowA})`;
      ctx.lineWidth = 6;
      ctx.lineCap = 'round';
      ctx.shadowColor = `rgb(${r},${g},${b})`;
      ctx.shadowBlur = 18;
      ctx.stroke();
      ctx.shadowBlur = 0;

      // Arrowhead at arc end
      const hx = Math.cos(arcEnd) * arrowR;
      const hy = Math.sin(arcEnd) * arrowR;
      const perpAngle = arcEnd + Math.PI / 2;
      ctx.beginPath();
      ctx.moveTo(hx + Math.cos(perpAngle - 0.4) * 14, hy + Math.sin(perpAngle - 0.4) * 14);
      ctx.lineTo(hx, hy);
      ctx.lineTo(hx + Math.cos(perpAngle + 0.4) * 14, hy + Math.sin(perpAngle + 0.4) * 14);
      ctx.strokeStyle = `rgba(${r},${g},${b},${arrowA})`;
      ctx.lineWidth = 5;
      ctx.lineJoin = 'round';
      ctx.shadowColor = `rgb(${r},${g},${b})`;
      ctx.shadowBlur = 14;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }

    // Center glowing dot
    ctx.beginPath();
    ctx.arc(0, 0, 8, 0, Math.PI * 2);
    const dotGrad = ctx.createRadialGradient(0, 0, 0, 0, 0, 8);
    dotGrad.addColorStop(0, `rgba(255,255,255,${arrowA})`);
    dotGrad.addColorStop(1, `rgba(200,200,255,${arrowA * 0.5})`);
    ctx.fillStyle = dotGrad;
    ctx.shadowColor = '#ffffff';
    ctx.shadowBlur = 20;
    ctx.fill();
    ctx.shadowBlur = 0;

    ctx.restore();
  }

  // ── Phase 0.55–0.92: Expanding ring burst ────────────────────────────────
  if (t >= 0.55 && t < 0.92) {
    const p = (t - 0.55) / 0.37;
    for (let ring = 0; ring < 3; ring++) {
      const rp = clamp((p - ring * 0.12) / 0.7, 0, 1);
      if (rp <= 0) continue;
      const ringR2 = easeOut(rp) * (W * 0.65 + ring * 20);
      const ringA2 = (1 - rp) * (0.6 - ring * 0.15);
      const [r, g, b] = ring % 2 === 0 ? [160, 100, 255] : [255, 255, 255];
      ctx.beginPath();
      ctx.arc(cx, cy, ringR2, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(${r},${g},${b},${ringA2})`;
      ctx.lineWidth = 3 - ring * 0.5;
      ctx.shadowColor = `rgb(${r},${g},${b})`;
      ctx.shadowBlur = 10 * (1 - rp);
      ctx.stroke();
      ctx.shadowBlur = 0;
    }
  }

  // ── Label ─────────────────────────────────────────────────────────────────
  if (t > 0.25 && t < 0.88) {
    const lt = (t - 0.25) / 0.63;
    const la = lt < 0.2 ? lt / 0.2 : lt > 0.75 ? 1 - (lt - 0.75) / 0.25 : 1;
    ctx.save();
    ctx.font = 'bold 22px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = '#a855f7';
    ctx.shadowBlur = 28;
    ctx.fillStyle = `rgba(255,255,255,${la})`;
    ctx.fillText('↺  REVERSE!', cx, cy - 80 - easeOut(lt) * 12);
    ctx.font = 'bold 13px "Segoe UI", sans-serif';
    ctx.fillStyle = `rgba(200,160,255,${la * 0.9})`;
    ctx.fillText('turn order swapped', cx, cy - 58 - easeOut(lt) * 12);
    ctx.shadowBlur = 0;
    ctx.restore();
  }
}

// ─── Transform Animation ──────────────────────────────────────────────────────
const TRANSFORM_DURATION = 1400;

const SNIPER_DURATION = 1000;

function paintSniperAnim(
  ctx: CanvasRenderingContext2D,
  anim: SniperAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = Math.min(elapsed / SNIPER_DURATION, 1);
  if (t >= 1) return;

  const x  = anim.sq.col * SQ;
  const y  = (7 - anim.sq.row) * SQ;
  const cx = x + SQ / 2;
  const cy = y + SQ / 2;

  const isRed = anim.variant === 'sniper';
  const primaryCol  = isRed ? '#ff2020' : '#a3e635';
  const [pr, pg, pb] = isRed ? [255, 32, 32] : [163, 230, 53];

  function easeOutL(v: number) { return 1 - (1 - v) * (1 - v); }
  function clampL(v: number, a: number, b: number) { return Math.max(a, Math.min(b, v)); }

  // ── Phase 0–0.18: Laser beam flies in ────────────────────────────────────
  if (t < 0.18) {
    const bt = t / 0.18;
    const startX = isRed ? x + SQ + 80 : x - 80;
    const startY = y - 80;
    const beamEndX = lerp(startX, cx, easeOutL(bt));
    const beamEndY = lerp(startY, cy, easeOutL(bt));

    ctx.save();
    ctx.beginPath();
    ctx.moveTo(isRed ? x + SQ + 80 : x - 80, y - 80);
    ctx.lineTo(beamEndX, beamEndY);
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},${bt * 0.6})`;
    ctx.lineWidth = 8;
    ctx.shadowColor = primaryCol;
    ctx.shadowBlur = 20;
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(isRed ? x + SQ + 80 : x - 80, y - 80);
    ctx.lineTo(beamEndX, beamEndY);
    ctx.strokeStyle = `rgba(255,255,255,${bt * 0.9})`;
    ctx.lineWidth = 2;
    ctx.shadowBlur = 6;
    ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.restore();
  }

  // ── Phase 0.10–0.30: Impact flash ────────────────────────────────────────
  if (t >= 0.10 && t < 0.30) {
    const ft = (t - 0.10) / 0.20;
    const fa = ft < 0.4 ? ft / 0.4 : 1 - (ft - 0.4) / 0.6;
    ctx.save();
    const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 0.7);
    grad.addColorStop(0, `rgba(255,255,255,${fa * 0.95})`);
    grad.addColorStop(0.3, `rgba(${pr},${pg},${pb},${fa * 0.8})`);
    grad.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
    ctx.fillStyle = grad;
    ctx.fillRect(x, y, SQ, SQ);
    ctx.restore();
  }

  // ── Phase 0.15–0.70: Piece shatters into fragments ───────────────────────
  if (t >= 0.15 && t < 0.7) {
    const pt = (t - 0.15) / 0.55;
    const img = PIECE_IMAGES[`${anim.pieceColor}_${anim.pieceType}`];
    if (img && img.complete) {
      const fragments = [
        { sx: 0,     sy: 0,     dx: -1, dy: -1 },
        { sx: SQ/2,  sy: 0,     dx:  1, dy: -1 },
        { sx: 0,     sy: SQ/2,  dx: -1, dy:  1 },
        { sx: SQ/2,  sy: SQ/2,  dx:  1, dy:  1 },
      ];
      ctx.save();
      for (const frag of fragments) {
        const flyDist = easeOut(pt) * SQ * 0.55;
        const destX = x + frag.sx + frag.dx * flyDist;
        const destY = y + frag.sy + frag.dy * flyDist;
        const fragA = clampL(1 - pt * 1.2, 0, 1);
        const rot   = frag.dx * frag.dy * pt * 0.8;

        ctx.save();
        ctx.globalAlpha = fragA;
        ctx.translate(destX + SQ / 4, destY + SQ / 4);
        ctx.rotate(rot);
        ctx.translate(-(destX + SQ / 4), -(destY + SQ / 4));

        ctx.beginPath();
        ctx.rect(destX - 2, destY - 2, SQ / 2 + 4, SQ / 2 + 4);
        ctx.clip();

        ctx.drawImage(img, frag.sx, frag.sy, SQ / 2, SQ / 2,
          destX, destY, SQ / 2, SQ / 2);

        ctx.globalCompositeOperation = 'source-atop';
        ctx.fillStyle = `rgba(${pr},${pg},${pb},${pt * 0.5})`;
        ctx.fillRect(destX, destY, SQ / 2, SQ / 2);
        ctx.globalCompositeOperation = 'source-over';
        ctx.restore();
      }
      ctx.restore();
    }
  }

  // ── Phase 0.18–0.70: Impact rings ────────────────────────────────────────
  if (t >= 0.18 && t < 0.7) {
    const rt = (t - 0.18) / 0.52;
    ctx.save();
    ctx.translate(cx, cy);
    for (let ring = 0; ring < 4; ring++) {
      const rDelay = ring * 0.08;
      const rp = clampL((rt - rDelay) / 0.6, 0, 1);
      if (rp <= 0) continue;
      const radius = easeOut(rp) * (SQ * 0.55 + ring * 8);
      const alpha  = (1 - rp) * (0.7 - ring * 0.12);
      ctx.beginPath();
      ctx.arc(0, 0, radius, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(${pr},${pg},${pb},${alpha})`;
      ctx.lineWidth = ring === 0 ? 3 : 1.5;
      ctx.shadowColor = primaryCol;
      ctx.shadowBlur = 10 * (1 - rp);
      ctx.stroke();
    }
    ctx.shadowBlur = 0;

    const holeT = clampL((rt - 0.05) / 0.3, 0, 1);
    const holeA = holeT < 0.5 ? holeT / 0.5 : 1 - (holeT - 0.5) / 0.5;
    const holeR = easeOut(holeT) * 10;
    if (holeA > 0) {
      ctx.beginPath();
      ctx.arc(0, 0, holeR, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(0,0,0,${holeA * 0.9})`;
      ctx.fill();
      ctx.beginPath();
      ctx.arc(0, 0, holeR + 2, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(255,255,255,${holeA * 0.6})`;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }
    ctx.restore();
  }

  // ── Phase 0.20–0.65: Shrapnel sparks ────────────────────────────────────
  if (t >= 0.20 && t < 0.65) {
    const st = (t - 0.20) / 0.45;
    const numSparks = isRed ? 12 : 8;
    ctx.save();
    for (let si = 0; si < numSparks; si++) {
      const angle = (si / numSparks) * Math.PI * 2 + (isRed ? 0 : Math.PI / numSparks);
      const speed = (0.6 + (si % 3) * 0.2) * SQ * 0.7;
      const sparkT = clampL(st - si * 0.03, 0, 1);
      if (sparkT <= 0) continue;
      const dist  = easeOut(sparkT) * speed;
      const sx2   = cx + Math.cos(angle) * dist;
      const sy2   = cy + Math.sin(angle) * dist - sparkT * sparkT * SQ * 0.3;
      const alpha = 1 - sparkT;
      const len   = 6 * (1 - sparkT * 0.7);

      ctx.beginPath();
      ctx.moveTo(sx2, sy2);
      ctx.lineTo(sx2 - Math.cos(angle) * len, sy2 - Math.sin(angle) * len);
      ctx.strokeStyle = si % 2 === 0
        ? `rgba(${pr},${pg},${pb},${alpha})`
        : `rgba(255,255,200,${alpha * 0.8})`;
      ctx.lineWidth = 1.5;
      ctx.shadowColor = primaryCol;
      ctx.shadowBlur = 4;
      ctx.stroke();
    }
    ctx.shadowBlur = 0;
    ctx.restore();
  }

  // ── Phase 0.45–1.0: Scorched marks & cracks ──────────────────────────────
  if (t >= 0.45) {
    const ct2 = clampL((t - 0.45) / 0.55, 0, 1);
    const crackA = ct2 < 0.3 ? ct2 / 0.3 : ct2 > 0.75 ? 1 - (ct2 - 0.75) / 0.25 : 1;

    ctx.save();
    const scorch = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 0.52);
    scorch.addColorStop(0, `rgba(0,0,0,${crackA * 0.55})`);
    scorch.addColorStop(0.5, `rgba(${Math.floor(pr*0.3)},${Math.floor(pg*0.1)},0,${crackA * 0.3})`);
    scorch.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = scorch;
    ctx.fillRect(x, y, SQ, SQ);

    const numCracks = isRed ? 6 : 4;
    for (let ci2 = 0; ci2 < numCracks; ci2++) {
      const angle = (ci2 / numCracks) * Math.PI * 2 + 0.3;
      const crackLen = (0.4 + (ci2 % 3) * 0.2) * SQ * 0.5 * easeOut(ct2);
      const jag = 4;
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      const segs = 4;
      for (let sg = 1; sg <= segs; sg++) {
        const segT = sg / segs;
        const offA = angle + ((sg % 2 === 0 ? 1 : -1) * 0.15);
        ctx.lineTo(
          cx + Math.cos(offA) * crackLen * segT + (Math.random() - 0.5) * jag * (1 - segT),
          cy + Math.sin(offA) * crackLen * segT + (Math.random() - 0.5) * jag * (1 - segT)
        );
      }
      ctx.strokeStyle = `rgba(${pr},${pg},${pb},${crackA * 0.65})`;
      ctx.lineWidth = 1;
      ctx.stroke();
    }
    ctx.restore();
  }

  // ── Label ────────────────────────────────────────────────────────────────
  if (t > 0.2 && t < 0.85) {
    const lt = (t - 0.2) / 0.65;
    const la = lt < 0.25 ? lt / 0.25 : lt > 0.7 ? 1 - (lt - 0.7) / 0.3 : 1;
    const labelY = y - 18 - easeOut(lt) * 14;
    const label  = isRed ? '🎯 ELIMINATED' : '💀 SELF-REMOVED';

    ctx.save();
    ctx.font = 'bold 11px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = primaryCol;
    ctx.shadowBlur = 14;
    ctx.fillStyle = `rgba(255,255,255,${la})`;
    ctx.fillText(label, cx, labelY);
    ctx.shadowBlur = 0;
    ctx.restore();
  }
}

function paintTransformAnim(
  ctx: CanvasRenderingContext2D,
  anim: TransformAnim,
  now: number,
) {
  const elapsed = now - anim.startTime;
  const t = clamp(elapsed / TRANSFORM_DURATION, 0, 1);
  if (t >= 1) return;

  const isUp = anim.direction === 'up';
  const px = anim.sq.col * SQ;
  const py = (7 - anim.sq.row) * SQ;
  const cx = px + SQ / 2;
  const cy = py + SQ / 2;

  const primaryColor   = isUp ? '#ffd700' : '#9333ea';
  const secondaryColor = isUp ? '#ffffff' : '#c084fc';
  const [pr, pg, pb]   = isUp ? [255, 215, 0]   : [147, 51, 234];
  const [sr, sg, sb]   = isUp ? [255, 255, 255] : [192, 132, 252];

  ctx.save();

  // ── Phase 1 (0–0.35): Charging / gathering energy ────────────────────────
  if (t < 0.35) {
    const p = t / 0.35;
    const ease = easeIn(p);

    const pulseA = ease * 0.6 + Math.sin(now / 80) * 0.15 * p;
    const sqGlow = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 0.9);
    sqGlow.addColorStop(0, `rgba(${pr},${pg},${pb},${pulseA * 0.9})`);
    sqGlow.addColorStop(0.5, `rgba(${pr},${pg},${pb},${pulseA * 0.4})`);
    sqGlow.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
    ctx.fillStyle = sqGlow;
    ctx.fillRect(px, py, SQ, SQ);

    const numLines = 8;
    for (let i = 0; i < numLines; i++) {
      const ang = (i / numLines) * Math.PI * 2;
      const dist = lerp(SQ * 1.2, SQ * 0.15, ease);
      const lx = cx + Math.cos(ang) * dist;
      const ly = cy + Math.sin(ang) * dist;
      const lineAlpha = ease * 0.8;
      ctx.beginPath();
      ctx.moveTo(lx, ly);
      ctx.lineTo(cx, cy);
      ctx.strokeStyle = `rgba(${pr},${pg},${pb},${lineAlpha})`;
      ctx.lineWidth = 1.5 * ease;
      ctx.shadowColor = primaryColor;
      ctx.shadowBlur = 8 * ease;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }

    const ringR = SQ * 0.45;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(now / 300 * (isUp ? 1 : -1));
    for (let i = 0; i < 6; i++) {
      const a = (i / 6) * Math.PI * 2;
      const rx = Math.cos(a) * ringR;
      const ry = Math.sin(a) * ringR;
      ctx.beginPath();
      ctx.arc(rx, ry, 3 * ease, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${sr},${sg},${sb},${ease * 0.9})`;
      ctx.shadowColor = secondaryColor;
      ctx.shadowBlur = 10;
      ctx.fill();
      ctx.shadowBlur = 0;
    }
    ctx.restore();
  }

  // ── Phase 2 (0.28–0.65): Flash + beam ────────────────────────────────────
  if (t >= 0.28 && t < 0.65) {
    const p = (t - 0.28) / 0.37;
    const ease = easeInOut(p);

    const flashA = Math.sin(p * Math.PI) * 0.95;
    const flash = ctx.createRadialGradient(cx, cy, 0, cx, cy, SQ * 1.1);
    flash.addColorStop(0, `rgba(255,255,255,${flashA})`);
    flash.addColorStop(0.2, `rgba(${pr},${pg},${pb},${flashA * 0.8})`);
    flash.addColorStop(0.6, `rgba(${pr},${pg},${pb},${flashA * 0.3})`);
    flash.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
    ctx.fillStyle = flash;
    ctx.beginPath();
    ctx.arc(cx, cy, SQ * 1.1, 0, Math.PI * 2);
    ctx.fill();

    const beamH = SQ * 2.5 * ease;
    const beamW = SQ * 0.35 * (1 - ease * 0.4);
    const beamGrad = ctx.createLinearGradient(cx, isUp ? cy : cy + beamH, cx, isUp ? cy - beamH : cy);
    beamGrad.addColorStop(0, `rgba(255,255,255,${flashA * 0.9})`);
    beamGrad.addColorStop(0.3, `rgba(${pr},${pg},${pb},${flashA * 0.7})`);
    beamGrad.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
    ctx.fillStyle = beamGrad;
    ctx.beginPath();
    ctx.ellipse(cx, isUp ? cy - beamH / 2 : cy + beamH / 2, beamW / 2, beamH / 2, 0, 0, Math.PI * 2);
    ctx.fill();

    for (let r = 0; r < 3; r++) {
      const ringT = clamp((p - r * 0.15) / 0.6, 0, 1);
      if (ringT <= 0) continue;
      const ringR = lerp(0, SQ * (1.2 + r * 0.4), easeOut(ringT));
      const ringA = (1 - ringT) * 0.8;
      ctx.beginPath();
      ctx.arc(cx, cy, ringR, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(${pr},${pg},${pb},${ringA})`;
      ctx.lineWidth = 3 - r;
      ctx.shadowColor = primaryColor;
      ctx.shadowBlur = 12;
      ctx.stroke();
      ctx.shadowBlur = 0;
    }

    const numStars = 12;
    for (let i = 0; i < numStars; i++) {
      const ang = (i / numStars) * Math.PI * 2 + now / 1000;
      const dist = SQ * 0.5 * ease + SQ * 0.3 * Math.sin(now / 200 + i);
      const sx = cx + Math.cos(ang) * dist;
      const sy = cy + Math.sin(ang) * dist;
      const starA = Math.sin(p * Math.PI) * 0.9;
      ctx.save();
      ctx.translate(sx, sy);
      ctx.rotate(ang + now / 500);
      ctx.beginPath();
      const starSize = 4 + 3 * ease;
      for (let j = 0; j < 8; j++) {
        const a2 = (j / 8) * Math.PI * 2 - Math.PI / 2;
        const rad = j % 2 === 0 ? starSize : starSize * 0.4;
        j === 0
          ? ctx.moveTo(Math.cos(a2) * rad, Math.sin(a2) * rad)
          : ctx.lineTo(Math.cos(a2) * rad, Math.sin(a2) * rad);
      }
      ctx.closePath();
      ctx.fillStyle = i % 2 === 0
        ? `rgba(${pr},${pg},${pb},${starA})`
        : `rgba(${sr},${sg},${sb},${starA})`;
      ctx.shadowColor = primaryColor;
      ctx.shadowBlur = 10;
      ctx.fill();
      ctx.shadowBlur = 0;
      ctx.restore();
    }
  }

  // ── Pre-swap (0–0.55): draw old piece stationary ──────────────────────────
  if (t < 0.55) {
    const fromImg = PIECE_IMAGES[`${anim.color}_${anim.fromType}`];
    if (fromImg && fromImg.complete) {
      ctx.save();
      ctx.globalAlpha = 1;
      ctx.shadowColor = primaryColor;
      ctx.shadowBlur = 6 * Math.sin(t / 0.55 * Math.PI);
      ctx.drawImage(fromImg, px + 3, py + 3, SQ - 6, SQ - 6);
      ctx.shadowBlur = 0;
      ctx.restore();
    }
  }

  // ── Phase 3 (0.55–0.82): Piece swap ──────────────────────────────────────
  if (t >= 0.55 && t < 0.82) {
    const p = (t - 0.55) / 0.27;

    const fromImg = PIECE_IMAGES[`${anim.color}_${anim.fromType}`];
    const toImg   = PIECE_IMAGES[`${anim.color}_${anim.toType}`];

    if (fromImg && fromImg.complete && p < 0.6) {
      const flyT = p / 0.6;
      const flyY = isUp
        ? cy - SQ * 0.5 * easeIn(flyT)
        : cy + SQ * 0.5 * easeIn(flyT);
      const alpha = 1 - easeIn(flyT);
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.drawImage(fromImg, cx - SQ / 2 + 3, flyY - SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.restore();
    }

    if (toImg && toImg.complete && p > 0.35) {
      const flyT = (p - 0.35) / 0.65;
      const startY = isUp ? cy + SQ * 0.35 : cy - SQ * 0.35;
      const curY = lerp(startY, cy, easeOut(flyT));
      const alpha = easeOut(flyT);
      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.shadowColor = primaryColor;
      ctx.shadowBlur = 20 * (1 - flyT * 0.5);
      ctx.drawImage(toImg, cx - SQ / 2 + 3, curY - SQ / 2 + 3, SQ - 6, SQ - 6);
      ctx.shadowBlur = 0;
      ctx.restore();
    }
  }

  // ── Phase 4 (0.72–1.0): Settling glow + rune fade ────────────────────────
  if (t >= 0.72) {
    const p = (t - 0.72) / 0.28;
    const fadeA = 1 - easeOut(p);

    const settleGlow = ctx.createRadialGradient(cx, cy, SQ * 0.3, cx, cy, SQ * 0.85);
    settleGlow.addColorStop(0, `rgba(${pr},${pg},${pb},0)`);
    settleGlow.addColorStop(0.5, `rgba(${pr},${pg},${pb},${fadeA * 0.35})`);
    settleGlow.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
    ctx.fillStyle = settleGlow;
    ctx.fillRect(px - SQ * 0.2, py - SQ * 0.2, SQ * 1.4, SQ * 1.4);

    const runePositions = [
      [px + 4,      py + 12],
      [px + SQ - 14, py + 12],
      [px + 4,      py + SQ - 4],
      [px + SQ - 14, py + SQ - 4],
    ];
    ctx.font = '10px serif';
    ctx.fillStyle = `rgba(${pr},${pg},${pb},${fadeA * 0.8})`;
    ctx.shadowColor = primaryColor;
    ctx.shadowBlur = 8 * fadeA;
    const runes = isUp ? ['✦', '★', '✦', '★'] : ['✧', '◆', '✧', '◆'];
    runePositions.forEach(([rx, ry], i) => {
      ctx.fillText(runes[i], rx, ry);
    });
    ctx.shadowBlur = 0;

    const borderA = fadeA * 0.9;
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},${borderA})`;
    ctx.lineWidth = 2.5;
    ctx.shadowColor = primaryColor;
    ctx.shadowBlur = 10 * fadeA;
    ctx.strokeRect(px + 2, py + 2, SQ - 4, SQ - 4);
    ctx.shadowBlur = 0;
  }

  // ── Label ─────────────────────────────────────────────────────────────────
  if (t > 0.1 && t < 0.85) {
    const labelT = t < 0.5 ? t / 0.5 : 1 - (t - 0.5) / 0.35;
    const labelA = clamp(labelT, 0, 1);
    const labelY = isUp
      ? py - 18 - easeOut(labelT) * 10
      : py + SQ + 18 + easeOut(labelT) * 10;

    ctx.font = 'bold 13px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = primaryColor;
    ctx.shadowBlur = 16;
    ctx.fillStyle = `rgba(255,255,255,${labelA})`;
    ctx.fillText(isUp ? '⬆ PROMOTED!' : '⬇ DEMOTED!', cx, labelY);

    ctx.font = 'bold 10px "Segoe UI", sans-serif';
    ctx.fillStyle = `rgba(${pr},${pg},${pb},${labelA * 0.9})`;
    const labelY2 = isUp ? labelY - 16 : labelY + 16;
    ctx.fillText(`${anim.fromType} → ${anim.toType}`, cx, labelY2);
    ctx.shadowBlur = 0;
    ctx.textAlign = 'left';
    ctx.textBaseline = 'alphabetic';
  }

  ctx.restore();
}

interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number; maxLife: number;
  r: number;
  color: string;
  type: 'spark' | 'smoke' | 'ember' | 'lava' | 'swap';
}

export const BoardCanvas = React.memo(function BoardCanvas(props: BoardCanvasProps) {
  const {
    board, turn, sel, hints, lm, check, kingPos,
    cardHighlight, doubleMoveHighlight, bombPieces, bombExploding,
    lavaSquares, lavaExploding, swapAnim, isReviewing, reviewBoard,
    cardPending, onClick, onDragStart, onDrop, doubleMove, transformAnim,
    sniperAnim, teleportAnim, jumpAnim, reverseAnim, sacrificeAnim, sacrificeSelectedSquares, mindControlAnim, mindControlTargetSquare, fuseAnim, fuseSelectedSq, fogZones, viewerColor, invisibleUnder,
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
  const W = 8 * SQ, H = 8 * SQ;
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
          particles.current.push({
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
        particles.current.push({
          x: cx, y: cy, vx: Math.cos(a)*spd, vy: Math.sin(a)*spd - 2,
          life: 1, maxLife: 1, r: 3 + Math.random() * 5,
          color: i < 20 ? '#ff6600' : i < 32 ? '#ffdd00' : '#ffffff',
          type: 'spark',
        });
      }
      for (let i = 0; i < 12; i++) {
        const a = Math.random() * Math.PI * 2;
        const spd = 0.5 + Math.random() * 2;
        particles.current.push({
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
        particles.current.push({
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
        particles.current.push({
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

          let baseColor = light ? '#F0D9B5' : '#B58863';
          if (isLM)  baseColor = light ? '#cdd26a' : '#aaa23a';
          if (isSel) baseColor = '#FFD700';
          if (isChk) baseColor = '#e85555';

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
            } else {
              ctx.strokeStyle = 'rgba(0,0,0,0.22)';
              ctx.lineWidth = 4;
              ctx.beginPath();
              ctx.arc(x + SQ/2, y + SQ/2, SQ/2 - 4, 0, Math.PI*2);
              ctx.stroke();
            }
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

    rafRef.current = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(rafRef.current);
  }, [
    displayBoard, sel, hints, lm, check, kingPos,
    cardHighlight, doubleMoveHighlight, bombPieces, bombExploding,
    lavaSquares, lavaExploding, isReviewing, doubleMove, transformAnim, sniperAnim, reverseAnim, sacrificeAnim, sacrificeSelectedSquares, mindControlAnim, mindControlTargetSquare, fuseAnim, fuseSelectedSq,
    fogZones, viewerColor,
  ]);

  const [localDrag, setLocalDrag] = React.useState<Sq | null>(null);
  const [localDragPos, setLocalDragPos] = React.useState<{ x: number; y: number } | null>(null);
  const localDragRef    = React.useRef<Sq | null>(null);
  const localDragPosRef = React.useRef<{ x: number; y: number } | null>(null);
  React.useEffect(() => { localDragRef.current = localDrag; }, [localDrag]);
  React.useEffect(() => { localDragPosRef.current = localDragPos; }, [localDragPos]);

  const getSquare = (e: React.MouseEvent): Sq => {
    const rect = canvasRef.current!.getBoundingClientRect();
    const x = e.clientX - rect.left, y = e.clientY - rect.top;
    return { row: 7 - Math.floor(y / SQ), col: Math.floor(x / SQ) };
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    if (cardPending || isReviewing) return;
    const sq = getSquare(e);
    if (!sq || sq.row < 0 || sq.row > 7 || sq.col < 0 || sq.col > 7) return;
    const p = displayBoard[sq.row]?.[sq.col];
    if (p?.color === turn) {
      setLocalDrag(sq);
      setLocalDragPos({ x: e.clientX - canvasRef.current!.getBoundingClientRect().left, y: e.clientY - canvasRef.current!.getBoundingClientRect().top });
      onDragStart(e, sq.row, sq.col);
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!localDrag) return;
    const rect = canvasRef.current!.getBoundingClientRect();
    setLocalDragPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
  };

  const handleMouseUp = (e: React.MouseEvent) => {
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
    const sq = getSquare(e);
    if (sq && sq.row >= 0 && sq.row <= 7 && sq.col >= 0 && sq.col <= 7) {
      onClick(sq.row, sq.col);
    }
  };

  return (
    <canvas
      ref={canvasRef}
      width={Math.round(W * dpr)}
      height={Math.round(H * dpr)}
      style={{
        display:'block',
        width: `${W}px`,
        height: `${H}px`,
        cursor: cardPending ? 'crosshair' : (localDrag ? 'grabbing' : 'pointer'),
      }}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onClick={handleClick}
      onMouseLeave={handleMouseUp}
    />
  );
});
