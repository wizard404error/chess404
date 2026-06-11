import type { PieceType, PieceColor, Sq } from '../types';
import { SQ as INITIAL_SQ } from '../constants';
import { PIECE_IMAGES } from './images';

export let SQ = INITIAL_SQ;

export function setSQ(v: number) { SQ = v; }

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

export interface BoardArrow {
  from: Sq;
  to: Sq;
  color?: string;
}

export function parseColor(css: string): [number, number, number, number] {
  const m = css.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/);
  if (m) return [+m[1], +m[2], +m[3], m[4] !== undefined ? +m[4] : 1];
  const h = css.replace('#','');
  if (h.length === 6) {
    return [parseInt(h.slice(0,2),16), parseInt(h.slice(2,4),16), parseInt(h.slice(4,6),16), 1];
  }
  return [255,255,255,1];
}

export function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#','');
  return [parseInt(h.slice(0,2),16), parseInt(h.slice(2,4),16), parseInt(h.slice(4,6),16)];
}

export const easeOut  = (t: number) => 1 - Math.pow(1 - t, 3);
export const easeIn   = (t: number) => t * t * t;
export const easeInOut = (t: number) => t < 0.5 ? 4*t*t*t : 1 - Math.pow(-2*t+2,3)/2;
export const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
export const lerp  = (a: number, b: number, t: number) => a + (b - a) * t;
export const ANALYSIS_ARROW_COLOR = 'rgba(244,196,48,0.9)';

export function drawBoardArrow(
  ctx: CanvasRenderingContext2D,
  from: Sq,
  to: Sq,
  color = ANALYSIS_ARROW_COLOR,
  options: { alpha?: number; preview?: boolean; lineWidth?: number } = {},
) {
  const alpha = options.alpha ?? 0.88;
  const fromX = from.col * SQ + SQ / 2;
  const fromY = (7 - from.row) * SQ + SQ / 2;
  const toX = to.col * SQ + SQ / 2;
  const toY = (7 - to.row) * SQ + SQ / 2;

  ctx.save();
  ctx.globalAlpha = alpha;
  ctx.strokeStyle = color;
  ctx.fillStyle = color;
  ctx.lineJoin = 'round';
  ctx.lineCap = 'round';
  ctx.shadowColor = options.preview ? 'rgba(255,235,140,0.45)' : 'rgba(255,220,120,0.65)';
  ctx.shadowBlur = options.preview ? 8 : 10;

  if (from.row === to.row && from.col === to.col) {
    const radius = SQ * 0.28;
    ctx.lineWidth = 6;
    ctx.beginPath();
    ctx.arc(fromX, fromY, radius, 0, Math.PI * 2);
    ctx.stroke();
    ctx.globalAlpha = alpha * 0.2;
    ctx.beginPath();
    ctx.arc(fromX, fromY, radius * 0.72, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    return;
  }

  const dx = toX - fromX;
  const dy = toY - fromY;
  const length = Math.hypot(dx, dy);
  const angle = Math.atan2(dy, dx);
  const shaftWidth = 10;
  const headLength = Math.min(SQ * 0.45, Math.max(SQ * 0.25, length * 0.32));
  const headHalfWidth = shaftWidth * 1.35;
  const shaftEnd = Math.max(0, length - headLength);

  ctx.translate(fromX, fromY);
  ctx.rotate(angle);

  ctx.lineWidth = options.lineWidth ?? shaftWidth;
  if (options.preview) {
    ctx.setLineDash([16, 10]);
  }
  ctx.beginPath();
  ctx.moveTo(0, 0);
  ctx.lineTo(shaftEnd, 0);
  ctx.stroke();
  ctx.setLineDash([]);

  ctx.beginPath();
  ctx.moveTo(length, 0);
  ctx.lineTo(shaftEnd, -headHalfWidth);
  ctx.lineTo(shaftEnd, headHalfWidth);
  ctx.closePath();
  ctx.fill();
  ctx.restore();
}


// ─── Teleport Animation ───────────────────────────────────────────────────────
const TELEPORT_DURATION = 1400;

export function paintTeleportAnim(
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

export function paintJumpAnim(
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

export function paintMindControlAnim(
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
      const or2 = 4.5 - bt * 2;
      ctx.save();
      ctx.beginPath();
      ctx.arc(ox, oy, Math.max(1, or2), 0, Math.PI * 2);
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

export function paintFuseAnim(
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

export function paintSacrificeAnim(
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

export function paintReverseAnim(
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

export function paintSniperAnim(
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

export function paintTransformAnim(
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

export interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number; maxLife: number;
  r: number;
  color: string;
  type: 'spark' | 'smoke' | 'ember' | 'lava' | 'swap';
}

// Hard cap on simultaneously-alive particles. When exceeded, oldest particles
// are evicted (FIFO) so the simulation never grows unbounded and a sustained
// effect burst cannot stall the render loop. 512 is plenty for every card
// animation we ship; the per-frame cost stays in single-digit milliseconds.
export const MAX_PARTICLES = 512;

export function pushParticle(pool: Particle[], p: Particle): void {
  if (pool.length >= MAX_PARTICLES) {
    // Evict oldest entries (FIFO). splice is O(n) but n is small (≤ cap).
    pool.splice(0, pool.length - MAX_PARTICLES + 1);
  }
  pool.push(p);
}
