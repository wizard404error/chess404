import React from 'react';

// ─── Types ────────────────────────────────────────────────────────────────────
export type CardAnimType =
  | 'shield' | 'gambler_win' | 'gambler_lose' | 'reverse'
  | 'freeze' | 'bomb_explode' | 'lava_kill' | 'swap'
  | 'teleport' | 'mindcontrol' | 'clone' | 'sniper'
  | 'bigsacrifice' | 'smallsacrifice' | 'blackhole' | 'fullfusion'
  | 'bomb_explode'
  | 'lava_kill'
  | 'freeze'
  | 'sniper'
  | 'teleport'
  | 'swap'
  | 'clone'
  | 'mindcontrol'
  | 'smallsacrifice'
  | 'bigsacrifice'
  | 'blackhole'
  |'fog_village'
  | null;

interface Props {
  anim: CardAnimType;
  label?: string;
  onDone: () => void;
}

// ─── Math helpers ─────────────────────────────────────────────────────────────
const lerp   = (a: number, b: number, t: number) => a + (b - a) * t;
const easeOut  = (t: number) => 1 - Math.pow(1 - t, 3);
const easeIn   = (t: number) => t * t * t;
const easeInOut = (t: number) => t < 0.5 ? 4*t*t*t : 1 - Math.pow(-2*t+2,3)/2;
const rand   = (min: number, max: number) => min + Math.random() * (max - min);
const clamp  = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));

// ─── Particle ─────────────────────────────────────────────────────────────────
interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  life: number; maxLife: number;
  size: number; color: string;
  shape: 'circle' | 'star' | 'hex' | 'line' | 'card' | 'square';
  angle: number; spin: number; alpha: number;
  gravity?: number;
}

function spawnParticle(overrides: Partial<Particle> & { x: number; y: number; color: string }): Particle {
  return {
    vx: 0, vy: 0, life: 1, maxLife: 1,
    size: 4, shape: 'circle',
    angle: 0, spin: 0, alpha: 1, gravity: 0.15,
    ...overrides,
  };
}

function drawParticle(ctx: CanvasRenderingContext2D, p: Particle) {
  if (p.life <= 0) return;
  const a = clamp((p.life / p.maxLife) * p.alpha, 0, 1);
  ctx.save();
  ctx.globalAlpha = a;
  ctx.translate(p.x, p.y);
  ctx.rotate(p.angle);
  ctx.fillStyle = p.color;
  ctx.shadowColor = p.color;
  ctx.shadowBlur = 8;
  if (p.shape === 'star') {
    ctx.beginPath();
    for (let i = 0; i < 10; i++) {
      const ang = (i / 10) * Math.PI * 2 - Math.PI / 2;
      const r = i % 2 === 0 ? p.size : p.size * 0.4;
      i === 0 ? ctx.moveTo(Math.cos(ang)*r, Math.sin(ang)*r) : ctx.lineTo(Math.cos(ang)*r, Math.sin(ang)*r);
    }
    ctx.closePath(); ctx.fill();
  } else if (p.shape === 'line') {
    ctx.fillRect(-p.size * 3, -1, p.size * 6, 2);
  } else if (p.shape === 'card') {
    const cw = p.size * 1.6, ch = p.size * 2.4;
    ctx.beginPath(); ctx.roundRect(-cw/2, -ch/2, cw, ch, 3); ctx.fill();
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 1; ctx.stroke();
  } else if (p.shape === 'hex') {
    ctx.beginPath();
    for (let i = 0; i < 6; i++) {
      const ang = (i/6)*Math.PI*2 - Math.PI/6;
      i === 0 ? ctx.moveTo(Math.cos(ang)*p.size, Math.sin(ang)*p.size)
              : ctx.lineTo(Math.cos(ang)*p.size, Math.sin(ang)*p.size);
    }
    ctx.closePath(); ctx.fill();
  } else if (p.shape === 'square') {
    ctx.fillRect(-p.size/2, -p.size/2, p.size, p.size);
  } else {
    ctx.beginPath(); ctx.arc(0, 0, p.size, 0, Math.PI*2); ctx.fill();
  }
  ctx.restore();
}

function tickParticles(ps: Particle[], dt: number) {
  for (const p of ps) {
    if (p.life <= 0) continue;
    p.x += p.vx * dt;
    p.y += p.vy * dt;
    p.vy += (p.gravity ?? 0.15) * dt;
    p.angle += p.spin * dt;
    p.life -= dt / 20;
  }
}

// ═════════════════════════════════════════════════════════════════════════════
// PAINTERS
// ═════════════════════════════════════════════════════════════════════════════

// ─── SHIELD ──────────────────────────────────────────────────────────────────
function paintShield(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;
  const ease = easeOut(Math.min(t*2, 1));

  // Vignette
  const vign = ctx.createRadialGradient(cx, cy, 0, cx, cy, W*0.65);
  vign.addColorStop(0, `rgba(20,50,20,${0.55*ease})`);
  vign.addColorStop(1, 'rgba(0,0,0,0)');
  ctx.fillStyle = vign; ctx.fillRect(0, 0, W, H);

  // Hex rings
  for (let r = 0; r < 3; r++) {
    const rt = clamp((t - r*0.15) / 0.6, 0, 1);
    if (rt <= 0) continue;
    const radius = lerp(40, 280+r*60, easeOut(rt));
    const alpha = (1-rt)*0.7;
    ctx.save(); ctx.translate(cx, cy);
    ctx.beginPath();
    for (let i = 0; i < 6; i++) {
      const ang = (i/6)*Math.PI*2 - Math.PI/6;
      i === 0 ? ctx.moveTo(Math.cos(ang)*radius, Math.sin(ang)*radius)
              : ctx.lineTo(Math.cos(ang)*radius, Math.sin(ang)*radius);
    }
    ctx.closePath();
    ctx.strokeStyle = `rgba(74,222,128,${alpha})`;
    ctx.lineWidth = 3-r; ctx.shadowColor = `rgba(74,222,128,${alpha*0.8})`; ctx.shadowBlur = 14;
    ctx.stroke(); ctx.restore();
  }

  // Bubble
  const bubbleT = easeOut(Math.min(t*3, 1));
  const bScale = bubbleT < 0.6 ? lerp(0,1.15,bubbleT/0.6) : lerp(1.15,1,(bubbleT-0.6)/0.4);
  const bR = 90 * bScale;
  if (bR > 0) {
    ctx.save(); ctx.translate(cx, cy);
    const glow = ctx.createRadialGradient(0,0,bR*0.5, 0,0,bR*1.5);
    glow.addColorStop(0, `rgba(74,222,128,${0.25*bubbleT})`); glow.addColorStop(1,'rgba(0,0,0,0)');
    ctx.fillStyle = glow; ctx.beginPath(); ctx.arc(0,0,bR*1.5,0,Math.PI*2); ctx.fill();
    const fill = ctx.createRadialGradient(-bR*0.3,-bR*0.3,0, 0,0,bR);
    fill.addColorStop(0,`rgba(160,255,180,${0.35*bubbleT})`);
    fill.addColorStop(0.5,`rgba(74,222,128,${0.18*bubbleT})`);
    fill.addColorStop(1,`rgba(20,100,50,${0.25*bubbleT})`);
    ctx.fillStyle = fill; ctx.beginPath(); ctx.arc(0,0,bR,0,Math.PI*2); ctx.fill();
    ctx.strokeStyle = `rgba(74,222,128,${0.9*bubbleT})`; ctx.lineWidth = 2.5;
    ctx.shadowColor = 'rgba(74,222,128,0.8)'; ctx.shadowBlur = 18; ctx.stroke();
    ctx.shadowBlur = 0;
    ctx.font = `bold ${Math.floor(bR*0.72)}px serif`;
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    ctx.fillStyle = `rgba(255,255,255,${0.9*bubbleT})`; ctx.fillText('🛡️', 0, 0);
    ctx.restore();
  }

  // Rotating hex grid
  if (t > 0.1 && t < 0.85) {
    const hexA = Math.min(t*4,1) * Math.max(0,1-(t-0.7)/0.15) * 0.3;
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(t*Math.PI*0.5);
    for (let ring = 1; ring <= 3; ring++) {
      const r = ring*50;
      ctx.beginPath();
      for (let i = 0; i < 6; i++) {
        const a = (i/6)*Math.PI*2;
        i===0 ? ctx.moveTo(Math.cos(a)*r, Math.sin(a)*r) : ctx.lineTo(Math.cos(a)*r, Math.sin(a)*r);
      }
      ctx.closePath(); ctx.strokeStyle=`rgba(74,222,128,${hexA})`; ctx.lineWidth=1; ctx.stroke();
    }
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  // Text
  if (t > 0.3 && t < 0.85) {
    const tt = (t-0.3)/0.55;
    const ta = tt < 0.4 ? tt/0.4 : 1-(tt-0.4)/0.6;
    ctx.save();
    ctx.font = 'bold 36px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(74,222,128,0.9)'; ctx.shadowBlur=24;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('SHIELDED!', cx, cy-140-tt*20);
    ctx.font='bold 18px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(74,222,128,${ta})`;
    ctx.fillText('Protected for 1 turn', cx, cy-102-tt*20);
    ctx.restore();
  }
}

// ─── FREEZE ───────────────────────────────────────────────────────────────────
function paintFreeze(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;
  const ease = easeOut(Math.min(t*2,1));

  // Blue vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0, `rgba(0,30,80,${0.7*ease})`);
  vign.addColorStop(1,'rgba(0,5,20,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Ice crack lines radiating from center
  const numCracks = 8;
  for (let i = 0; i < numCracks; i++) {
    const ang = (i/numCracks)*Math.PI*2;
    const crackT = clamp((t - i*0.04)/0.6, 0, 1);
    if (crackT <= 0) continue;
    const len = lerp(0, 220+rand(0,80), easeOut(crackT));
    const alpha = (1-t*0.8)*0.6;
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(ang);
    ctx.beginPath(); ctx.moveTo(0,0);
    // Jagged crack
    let cx2=0, cy2=0;
    const segs = 6;
    for (let s = 1; s <= segs; s++) {
      const sl = (s/segs)*len;
      const jitter = (Math.random()-0.5)*20*(1-s/segs);
      ctx.lineTo(jitter, sl);
    }
    ctx.strokeStyle=`rgba(147,210,255,${alpha})`; ctx.lineWidth=1.5;
    ctx.shadowColor='rgba(147,210,255,0.8)'; ctx.shadowBlur=8; ctx.stroke();
    ctx.restore();
  }

  // Central ice shard burst
  const burstT = easeOut(Math.min(t*2.5,1));
  if (burstT > 0) {
    const fade = t > 0.6 ? 1-(t-0.6)/0.4 : 1;
    ctx.save(); ctx.translate(cx,cy);
    ctx.font=`${Math.floor(80*burstT)}px serif`;
    ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(147,210,255,1)'; ctx.shadowBlur=50;
    ctx.globalAlpha=fade;
    ctx.fillText('❄️', 0, 0);
    ctx.restore();

    // Ice rings
    for (let r = 0; r < 3; r++) {
      const rt = clamp((t-r*0.1)/0.5, 0, 1);
      if (rt <= 0) continue;
      const radius = lerp(20, 200+r*50, easeOut(rt));
      const ra = (1-rt)*0.5*fade;
      ctx.save(); ctx.translate(cx,cy);
      ctx.beginPath(); ctx.arc(0,0,radius,0,Math.PI*2);
      ctx.strokeStyle=`rgba(147,210,255,${ra})`; ctx.lineWidth=2;
      ctx.shadowColor=`rgba(147,210,255,${ra})`; ctx.shadowBlur=10; ctx.stroke();
      ctx.restore();
    }
  }

  ps.forEach(p => drawParticle(ctx, p));

  // Text
  if (t > 0.2 && t < 0.85) {
    const tt = (t-0.2)/0.65;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(147,210,255,0.9)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('FROZEN!', cx, cy+100);
    ctx.font='bold 16px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(147,210,255,${ta})`;
    ctx.fillText('Cannot move for 1 turn', cx, cy+135);
    ctx.restore();
  }
}

// ─── BOMB EXPLODE ─────────────────────────────────────────────────────────────
function paintBombExplode(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Screen flash at start
  if (t < 0.12) {
    const flashA = (1-t/0.12)*0.85;
    ctx.fillStyle=`rgba(255,220,100,${flashA})`; ctx.fillRect(0,0,W,H);
  }

  // Dark smoke overlay
  const bgA = clamp(t*2,0,0.6);
  ctx.fillStyle=`rgba(10,5,0,${bgA})`; ctx.fillRect(0,0,W,H);

  // Fireball
  const fbT = easeOut(Math.min(t*3,1));
  const fbR = lerp(0, W*0.35, fbT) * (1 - t*0.4);
  if (fbR > 0) {
    const fb = ctx.createRadialGradient(cx,cy,0, cx,cy,fbR);
    fb.addColorStop(0,'rgba(255,255,255,0.95)');
    fb.addColorStop(0.15,'rgba(255,220,0,0.9)');
    fb.addColorStop(0.4,'rgba(255,80,0,0.7)');
    fb.addColorStop(0.7,'rgba(150,20,0,0.4)');
    fb.addColorStop(1,'transparent');
    ctx.fillStyle=fb; ctx.beginPath(); ctx.arc(cx,cy,fbR*1.1,0,Math.PI*2); ctx.fill();
  }

  // Shockwave rings
  for (let r = 0; r < 3; r++) {
    const rt = clamp((t - r*0.08)/0.7, 0, 1);
    if (rt <= 0) continue;
    const ringR = lerp(0, W*0.6, easeOut(rt));
    const ra = (1-rt)*0.7;
    ctx.strokeStyle=`rgba(255,${200-r*50},50,${ra})`; ctx.lineWidth=4-r;
    ctx.shadowColor=`rgba(255,150,0,${ra})`; ctx.shadowBlur=15;
    ctx.beginPath(); ctx.arc(cx,cy,ringR,0,Math.PI*2); ctx.stroke();
  }

  ps.forEach(p => drawParticle(ctx, p));

  // Screen shake simulation via transform
  if (t < 0.3) {
    const shakeX = Math.sin(t*80)*(1-t/0.3)*8;
    const shakeY = Math.cos(t*60)*(1-t/0.3)*6;
    ctx.save(); ctx.translate(shakeX,shakeY); ctx.restore();
  }

  // Text
  if (t > 0.2 && t < 0.85) {
    const tt = (t-0.2)/0.65;
    const ta = tt < 0.2 ? tt/0.2 : 1-(tt-0.2)/0.8;
    ctx.save();
    ctx.font='bold 44px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(255,100,0,1)'; ctx.shadowBlur=35;
    ctx.fillStyle=`rgba(255,220,100,${ta})`; ctx.fillText('💥 BOOM!', cx, cy-120);
    ctx.restore();
  }
}

// ─── LAVA KILL ────────────────────────────────────────────────────────────────
function paintLavaKill(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Red-orange vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0, `rgba(80,20,0,${0.8*easeOut(Math.min(t*2,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Lava eruption rings
  for (let r = 0; r < 4; r++) {
    const rt = clamp((t-r*0.07)/0.6, 0, 1);
    if (rt <= 0) continue;
    const radius = lerp(20, 260+r*40, easeOut(rt));
    const ra = (1-rt)*0.65;
    const red = Math.floor(200+r*15);
    ctx.save(); ctx.translate(cx,cy);
    ctx.beginPath(); ctx.arc(0,0,radius,0,Math.PI*2);
    ctx.strokeStyle=`rgba(${red},${80-r*15},0,${ra})`;
    ctx.lineWidth=3; ctx.shadowColor=`rgba(255,80,0,${ra})`; ctx.shadowBlur=16; ctx.stroke();
    ctx.restore();
  }

  // Central lava ball
  const lbT = easeOut(Math.min(t*3,1));
  const lbR = 80*lbT*(1-t*0.5);
  if (lbR > 0) {
    ctx.save(); ctx.translate(cx,cy);
    const lg = ctx.createRadialGradient(0,0,0, 0,0,lbR);
    lg.addColorStop(0,'rgba(255,255,200,0.95)');
    lg.addColorStop(0.3,'rgba(255,180,0,0.9)');
    lg.addColorStop(0.7,'rgba(255,60,0,0.7)');
    lg.addColorStop(1,'rgba(150,20,0,0)');
    ctx.fillStyle=lg; ctx.beginPath(); ctx.arc(0,0,lbR,0,Math.PI*2); ctx.fill();
    ctx.font=`bold ${Math.floor(lbR*0.8)}px serif`;
    ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(255,100,0,0.9)'; ctx.shadowBlur=30;
    ctx.fillText('🌋', 0, 0);
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.25 && t < 0.9) {
    const tt = (t-0.25)/0.65;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(255,100,0,0.9)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,200,100,${ta})`; ctx.fillText('INCINERATED!', cx, cy+110);
    ctx.restore();
  }
}

// ─── SWAP ─────────────────────────────────────────────────────────────────────
function paintSwap(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[], label: string) {
  const cx = W/2, cy = H/2;
  const ease = easeOut(Math.min(t*2,1));

  // Purple vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.65);
  vign.addColorStop(0, `rgba(30,0,60,${0.6*ease})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Spiral rings
  for (let r = 0; r < 4; r++) {
    const rt = clamp((t-r*0.1)/0.6, 0, 1);
    if (rt <= 0) continue;
    const radius = lerp(20, 220+r*40, easeOut(rt));
    const ra = (1-rt)*0.55;
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(t*Math.PI*(r%2===0?2:-2));
    ctx.beginPath();
    for (let a = 0; a < Math.PI*2; a += 0.3) {
      const px = Math.cos(a)*radius, py = Math.sin(a)*radius;
      a===0 ? ctx.moveTo(px,py) : ctx.lineTo(px,py);
    }
    ctx.closePath();
    ctx.strokeStyle=`rgba(192,132,252,${ra})`; ctx.lineWidth=2;
    ctx.shadowColor=`rgba(192,132,252,${ra})`; ctx.shadowBlur=12; ctx.stroke();
    ctx.restore();
  }

  // Two orbs swopping
  const orb1X = lerp(cx-120, cx+120, easeInOut(t));
  const orb2X = lerp(cx+120, cx-120, easeInOut(t));
  const orbArcY = Math.sin(t*Math.PI)*-60;
  for (const [ox, oy, col] of [[orb1X, cy+orbArcY, '#a78bfa'],[orb2X, cy-orbArcY, '#34d399']] as [number,number,string][]) {
    ctx.save(); ctx.translate(ox, oy);
    const og = ctx.createRadialGradient(0,0,0, 0,0,28);
    og.addColorStop(0,'rgba(255,255,255,0.9)'); og.addColorStop(0.4,col+'cc'); og.addColorStop(1,col+'00');
    ctx.fillStyle=og; ctx.beginPath(); ctx.arc(0,0,28,0,Math.PI*2); ctx.fill();
    ctx.font='28px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.fillText('♟', 0, 2);
    ctx.restore();
  }

  // Center flash
  if (t > 0.4 && t < 0.65) {
    const ft = (t-0.4)/0.25;
    const fa = Math.sin(ft*Math.PI)*0.7;
    const fg = ctx.createRadialGradient(cx,cy,0, cx,cy,80);
    fg.addColorStop(0,`rgba(255,255,255,${fa})`); fg.addColorStop(1,'transparent');
    ctx.fillStyle=fg; ctx.beginPath(); ctx.arc(cx,cy,80,0,Math.PI*2); ctx.fill();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.3 && t < 0.9) {
    const tt = (t-0.3)/0.6;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 36px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(192,132,252,0.9)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('SWAPPED!', cx, cy-130);
    if (label) {
      ctx.font='bold 15px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(192,132,252,${ta})`;
      ctx.fillText(label, cx, cy-96);
    }
    ctx.restore();
  }
}

// ─── TELEPORT ─────────────────────────────────────────────────────────────────
function paintTeleport(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Cyan vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.65);
  vign.addColorStop(0,`rgba(0,30,50,${0.65*easeOut(Math.min(t*2,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Portal vortex rings (spinning)
  for (let r = 0; r < 5; r++) {
    const rt = clamp((t-r*0.07)/0.5, 0, 1);
    if (rt <= 0) continue;
    const radius = lerp(10, 160+r*35, easeOut(rt))*(t > 0.6 ? 1-(t-0.6)/0.4 : 1);
    const ra = (1-rt)*0.7;
    const dir = r%2===0?1:-1;
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(t*Math.PI*4*dir);
    ctx.beginPath(); ctx.arc(0,0,Math.max(1,radius),0,Math.PI*2);
    ctx.strokeStyle=`rgba(${r%2===0?'34,211,238':'99,102,241'},${ra})`;
    ctx.lineWidth=2.5; ctx.shadowColor=`rgba(34,211,238,${ra})`; ctx.shadowBlur=14; ctx.stroke();
    ctx.restore();
  }

  // Center ✨
  const iconT = easeOut(Math.min(t*2.5,1))*(t>0.65 ? 1-(t-0.65)/0.35 : 1);
  if (iconT > 0) {
    ctx.save(); ctx.translate(cx,cy); ctx.scale(iconT,iconT);
    ctx.font='72px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(34,211,238,0.9)'; ctx.shadowBlur=50;
    ctx.fillText('✨',0,0); ctx.restore();
  }

  // Particle column sucking in then bursting out
  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.2 && t < 0.85) {
    const tt = (t-0.2)/0.65;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 36px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(34,211,238,0.95)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('TELEPORTED!', cx, cy+110);
    ctx.restore();
  }
}

// ─── MIND CONTROL ─────────────────────────────────────────────────────────────
function paintMindControl(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Purple-gold vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0,`rgba(30,0,50,${0.75*easeOut(Math.min(t*1.5,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Hypno spiral
  const numArms = 6;
  for (let arm = 0; arm < numArms; arm++) {
    const armAngle = (arm/numArms)*Math.PI*2 + t*Math.PI*3;
    ctx.save(); ctx.translate(cx,cy);
    ctx.beginPath();
    for (let i = 0; i < 80; i++) {
      const a = armAngle + (i/80)*Math.PI*4;
      const r = (i/80)*200;
      const px = Math.cos(a)*r, py = Math.sin(a)*r;
      i===0 ? ctx.moveTo(px,py) : ctx.lineTo(px,py);
    }
    const ra = 0.4*(1-t*0.5);
    ctx.strokeStyle=arm%2===0?`rgba(167,139,250,${ra})`:`rgba(245,158,11,${ra})`;
    ctx.lineWidth=1.5; ctx.shadowColor=`rgba(167,139,250,${ra})`; ctx.shadowBlur=8; ctx.stroke();
    ctx.restore();
  }

  // Central 🧠
  const iconT = easeOut(Math.min(t*2,1))*(t>0.7 ? 1-(t-0.7)/0.3 : 1);
  if (iconT > 0) {
    ctx.save(); ctx.translate(cx,cy);
    const pulse = 1+Math.sin(t*Math.PI*8)*0.08;
    ctx.scale(iconT*pulse,iconT*pulse);
    ctx.font='80px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(167,139,250,0.9)'; ctx.shadowBlur=50;
    ctx.fillText('🧠',0,0); ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.25 && t < 0.88) {
    const tt = (t-0.25)/0.63;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(167,139,250,0.9)'; ctx.shadowBlur=30;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('MIND CONTROLLED!', cx, cy+120);
    ctx.font='bold 16px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(245,158,11,${ta})`;
    ctx.fillText('Piece permanently stolen', cx, cy+154);
    ctx.restore();
  }
}

// ─── CLONE ────────────────────────────────────────────────────────────────────
function paintClone(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Green sci-fi vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0,`rgba(0,30,20,${0.7*easeOut(Math.min(t*2,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // DNA helix side lines
  for (let strand = 0; strand < 2; strand++) {
    ctx.save(); ctx.translate(cx,cy);
    ctx.beginPath();
    for (let i = 0; i < 60; i++) {
      const yy = (i/60)*400-200;
      const xx = Math.cos((i/60)*Math.PI*4 + t*Math.PI*3 + strand*Math.PI)*70;
      i===0 ? ctx.moveTo(xx,yy) : ctx.lineTo(xx,yy);
    }
    const ra = 0.5*(1-t*0.4);
    ctx.strokeStyle=strand===0?`rgba(52,211,153,${ra})`:`rgba(96,165,250,${ra})`;
    ctx.lineWidth=2; ctx.shadowColor=`rgba(52,211,153,${ra})`; ctx.shadowBlur=10; ctx.stroke();
    ctx.restore();
  }

  // Cross rungs
  for (let i = 0; i < 8; i++) {
    const yy = cy + ((i/8)*400-200);
    const xx1 = cx + Math.cos((i/8)*Math.PI*4 + t*Math.PI*3)*70;
    const xx2 = cx + Math.cos((i/8)*Math.PI*4 + t*Math.PI*3+Math.PI)*70;
    const ra = 0.3*(1-t*0.5);
    ctx.strokeStyle=`rgba(134,239,172,${ra})`; ctx.lineWidth=1.5;
    ctx.beginPath(); ctx.moveTo(xx1,yy); ctx.lineTo(xx2,yy); ctx.stroke();
  }

  // Two piece icons splitting apart
  const splitX = lerp(0,90,easeOut(t));
  for (const [ox,col] of [[-splitX,'rgba(52,211,153,0.9)'],[splitX,'rgba(96,165,250,0.9)']] as [number,string][]) {
    const fade = t > 0.7 ? 1-(t-0.7)/0.3 : 1;
    ctx.save(); ctx.translate(cx+ox, cy);
    ctx.globalAlpha = easeOut(Math.min(t*3,1))*fade;
    ctx.font='52px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor=col; ctx.shadowBlur=30; ctx.fillText('♟',0,0);
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.2 && t < 0.85) {
    const tt = (t-0.2)/0.65;
    const ta = tt < 0.3 ? tt/0.3 : 1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(52,211,153,0.9)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('CLONED!', cx, cy+130);
    ctx.restore();
  }
}

// ─── SNIPER ───────────────────────────────────────────────────────────────────
function paintSniper(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Dark red vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0,`rgba(40,0,0,${0.8*easeOut(Math.min(t*3,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Scope crosshair
  const scopeT = easeOut(Math.min(t*3,1));
  const scopeR = lerp(200, 60, scopeT);
  const scopeA = Math.min(scopeT, t > 0.5 ? 1-(t-0.5)*2 : 1);
  if (scopeA > 0) {
    ctx.save(); ctx.translate(cx,cy);
    ctx.strokeStyle=`rgba(255,30,30,${0.8*scopeA})`; ctx.lineWidth=2;
    ctx.shadowColor='rgba(255,0,0,0.7)'; ctx.shadowBlur=12;
    ctx.beginPath(); ctx.arc(0,0,scopeR,0,Math.PI*2); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(-scopeR-20,0); ctx.lineTo(scopeR+20,0); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(0,-scopeR-20); ctx.lineTo(0,scopeR+20); ctx.stroke();
    // Inner dot
    ctx.beginPath(); ctx.arc(0,0,4,0,Math.PI*2);
    ctx.fillStyle=`rgba(255,30,30,${scopeA})`; ctx.fill();
    ctx.restore();
  }

  // Bullet flash
  if (t < 0.15) {
    const fa = (1-t/0.15)*0.9;
    ctx.fillStyle=`rgba(255,255,200,${fa})`; ctx.fillRect(0,0,W,H);
  }

  // Impact rings
  if (t > 0.3) {
    for (let r = 0; r < 3; r++) {
      const rt = clamp((t-0.3-r*0.08)/0.5, 0, 1);
      if (rt <= 0) continue;
      const radius = lerp(0, 150+r*40, easeOut(rt));
      const ra = (1-rt)*0.6;
      ctx.save(); ctx.translate(cx,cy);
      ctx.beginPath(); ctx.arc(0,0,radius,0,Math.PI*2);
      ctx.strokeStyle=`rgba(255,50,50,${ra})`; ctx.lineWidth=2.5;
      ctx.shadowColor=`rgba(255,0,0,${ra})`; ctx.shadowBlur=14; ctx.stroke();
      ctx.restore();
    }
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.25 && t < 0.85) {
    const tt=(t-0.25)/0.6; const ta=tt<0.3?tt/0.3:1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 40px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(255,50,50,0.9)'; ctx.shadowBlur=30;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('🎯 ELIMINATED!', cx, cy+110);
    ctx.restore();
  }
}

// ─── SACRIFICE ────────────────────────────────────────────────────────────────
function paintSacrifice(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[], label: string) {
  const cx = W/2, cy = H/2;

  // Dark red-gold vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  vign.addColorStop(0,`rgba(40,10,0,${0.8*easeOut(Math.min(t*2,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Rising energy pillars
  const numPillars=8;
  for (let i=0; i<numPillars; i++) {
    const ang=(i/numPillars)*Math.PI*2;
    const rt=clamp((t-i*0.05)/0.7,0,1);
    if (rt<=0) continue;
    const r=lerp(0,160,easeOut(rt));
    const px=cx+Math.cos(ang)*120, py=cy+Math.sin(ang)*120;
    const ra=(1-rt*0.5)*0.6;
    ctx.save(); ctx.translate(px,py);
    const pg=ctx.createRadialGradient(0,0,0,0,0,20);
    pg.addColorStop(0,`rgba(245,158,11,${ra})`); pg.addColorStop(1,'rgba(220,38,38,0)');
    ctx.fillStyle=pg; ctx.beginPath(); ctx.arc(0,0,20,0,Math.PI*2); ctx.fill();
    ctx.restore();
    ctx.strokeStyle=`rgba(220,38,38,${ra*0.6})`; ctx.lineWidth=1.5;
    ctx.setLineDash([4,4]);
    ctx.beginPath(); ctx.moveTo(px,py); ctx.lineTo(cx,cy); ctx.stroke();
    ctx.setLineDash([]);
  }

  // Central implosion
  const coreT=easeOut(Math.min(t*2,1))*(t>0.65?1-(t-0.65)/0.35:1);
  if (coreT>0) {
    ctx.save(); ctx.translate(cx,cy);
    const cg=ctx.createRadialGradient(0,0,0,0,0,70*coreT);
    cg.addColorStop(0,'rgba(255,220,100,0.9)'); cg.addColorStop(0.4,'rgba(220,38,38,0.7)'); cg.addColorStop(1,'transparent');
    ctx.fillStyle=cg; ctx.beginPath(); ctx.arc(0,0,70*coreT,0,Math.PI*2); ctx.fill();
    ctx.font=`bold ${Math.floor(60*coreT)}px serif`; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(245,158,11,0.9)'; ctx.shadowBlur=40;
    ctx.fillText('💎',0,0); ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.2 && t < 0.85) {
    const tt=(t-0.2)/0.65; const ta=tt<0.3?tt/0.3:1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 36px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(245,158,11,0.95)'; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('SACRIFICE!', cx, cy-130);
    if (label) {
      ctx.font='bold 16px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(245,158,11,${ta})`;
      ctx.fillText(label, cx, cy-95);
    }
    ctx.restore();
  }
}

// ─── BLACK HOLE ───────────────────────────────────────────────────────────────
function paintBlackHole(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Dark vignette growing
  const bgA=clamp(t*2,0,0.85);
  ctx.fillStyle=`rgba(0,0,5,${bgA})`; ctx.fillRect(0,0,W,H);

  // Accretion disk
  for (let ring=0; ring<5; ring++) {
    const rt=clamp((t-ring*0.06)/0.5,0,1);
    if (rt<=0) continue;
    const radius=lerp(200-ring*30,40,easeIn(t));
    const ra=(1-rt*0.3)*0.5;
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(t*Math.PI*(ring%2===0?6:-4));
    ctx.beginPath(); ctx.ellipse(0,0,radius,radius*0.25,0,0,Math.PI*2);
    const col=ring<2?`rgba(96,165,250,${ra})`:`rgba(167,139,250,${ra})`;
    ctx.strokeStyle=col; ctx.lineWidth=2;
    ctx.shadowColor=col; ctx.shadowBlur=15; ctx.stroke();
    ctx.restore();
  }

  // Central black void
  const voidR=lerp(0,80,easeOut(Math.min(t*3,1)));
  if (voidR>0) {
    ctx.save(); ctx.translate(cx,cy);
    const vg=ctx.createRadialGradient(0,0,0,0,0,voidR*1.5);
    vg.addColorStop(0,'rgba(0,0,0,1)');
    vg.addColorStop(0.6,'rgba(10,0,20,0.8)');
    vg.addColorStop(1,'transparent');
    ctx.fillStyle=vg; ctx.beginPath(); ctx.arc(0,0,voidR*1.5,0,Math.PI*2); ctx.fill();
    ctx.font=`${Math.floor(voidR)}px serif`; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(96,165,250,0.7)'; ctx.shadowBlur=30;
    ctx.fillText('🕳️',0,0); ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t > 0.2 && t < 0.85) {
    const tt=(t-0.2)/0.65; const ta=tt<0.3?tt/0.3:1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 36px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(96,165,250,0.9)'; ctx.shadowBlur=30;
    ctx.fillStyle=`rgba(200,200,255,${ta})`; ctx.fillText('BLACK HOLE!', cx, cy+120);
    ctx.font='bold 15px "Segoe UI",sans-serif'; ctx.fillStyle=`rgba(96,165,250,${ta})`;
    ctx.fillText('Gravity trap set — detonates in 2 turns', cx, cy+153);
    ctx.restore();
  }
}

// ─── FULL FUSION ──────────────────────────────────────────────────────────────
function paintFullFusion(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;

  // Electric white-blue vignette
  const vign = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.65);
  vign.addColorStop(0,`rgba(0,20,60,${0.7*easeOut(Math.min(t*2,1))})`);
  vign.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=vign; ctx.fillRect(0,0,W,H);

  // Lightning arcs
  const numArcs=6;
  for (let i=0; i<numArcs; i++) {
    if (Math.random()>0.5) continue;
    const ang=(i/numArcs)*Math.PI*2+t*0.5;
    const len=rand(60,180);
    ctx.save(); ctx.translate(cx,cy); ctx.rotate(ang);
    ctx.beginPath(); ctx.moveTo(0,0);
    let xx=0,yy=0;
    for (let s=0; s<8; s++) {
      xx+=(Math.random()-0.5)*30; yy+=len/8;
      ctx.lineTo(xx,yy);
    }
    const ra=rand(0.3,0.8)*(1-t*0.7);
    ctx.strokeStyle=`rgba(${Math.random()>0.5?'147,197,253':'255,255,255'},${ra})`;
    ctx.lineWidth=1.5; ctx.shadowColor='rgba(147,197,253,0.8)'; ctx.shadowBlur=12; ctx.stroke();
    ctx.restore();
  }

  // Flash
  if (t < 0.1) {
    ctx.fillStyle=`rgba(200,230,255,${(1-t/0.1)*0.6})`; ctx.fillRect(0,0,W,H);
  }

  // Two pieces merging
  const mergeT=easeIn(Math.min(t*2,1));
  const offset=lerp(100,0,mergeT);
  for (const [ox,col] of [[-offset,'rgba(96,165,250,0.9)'],[offset,'rgba(245,158,11,0.9)']] as [number,string][]) {
    const fade=t>0.5?1-(t-0.5)*1.5:1;
    ctx.save(); ctx.translate(cx+ox, cy);
    ctx.globalAlpha=easeOut(Math.min(t*3,1))*Math.max(0,fade);
    ctx.font='52px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor=col; ctx.shadowBlur=25; ctx.fillText('♟',0,0);
    ctx.restore();
  }
  // Fused piece
  if (t>0.45) {
    const ft=easeOut((t-0.45)/0.3);
    ctx.save(); ctx.translate(cx,cy);
    ctx.globalAlpha=Math.min(ft,1)*(t>0.8?1-(t-0.8)/0.2:1);
    ctx.scale(1+ft*0.4,1+ft*0.4);
    ctx.font='60px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(255,200,50,0.9)'; ctx.shadowBlur=40; ctx.fillText('⚡',0,0);
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));

  if (t>0.35 && t<0.88) {
    const tt=(t-0.35)/0.53; const ta=tt<0.3?tt/0.3:1-(tt-0.3)/0.7;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(147,197,253,0.95)'; ctx.shadowBlur=30;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('⚡ FULL FUSION!', cx, cy+120);
    ctx.restore();
  }
}

// ─── GAMBLER ─────────────────────────────────────────────────────────────────
function paintGambler(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, isWin: boolean, ps: Particle[], label: string) {
  const cx = W/2, cy = H/2;
  const primaryColor = isWin ? '#2ecc71' : '#e74c3c';
  const primaryRgb   = isWin ? '46,204,113' : '231,76,60';

  const bg = ctx.createRadialGradient(cx,cy,0, cx,cy,W*0.7);
  bg.addColorStop(0,`rgba(${isWin?'10,40,20':'40,10,10'},${0.7*Math.min(t*3,1)})`);
  bg.addColorStop(1,'rgba(0,0,0,0)');
  ctx.fillStyle=bg; ctx.fillRect(0,0,W,H);

  const SPIN_DUR=0.55;
  const spinT=Math.min(t/SPIN_DUR,1);
  const spinRaw=spinT*Math.PI*6;
  const coinScaleX=Math.abs(Math.cos(spinRaw));
  const coinR=70, coinY=cy-20;

  ctx.save(); ctx.translate(cx,coinY+coinR+12); ctx.scale(coinScaleX*1.1,0.18);
  ctx.beginPath(); ctx.arc(0,0,coinR,0,Math.PI*2);
  ctx.fillStyle=`rgba(0,0,0,${0.35*spinT})`; ctx.fill(); ctx.restore();

  ctx.save(); ctx.translate(cx,coinY); ctx.scale(coinScaleX,1);
  const phase=spinRaw%(Math.PI*2), isFront=Math.cos(phase)>=0;
  const cg=ctx.createRadialGradient(-coinR*0.25,-coinR*0.25,0, 0,0,coinR);
  if (isFront) { cg.addColorStop(0,'#ffe87a'); cg.addColorStop(0.4,'#f5c842'); cg.addColorStop(1,'#c8900a'); }
  else { cg.addColorStop(0,'#d4a020'); cg.addColorStop(0.5,'#a87010'); cg.addColorStop(1,'#6b4200'); }
  ctx.beginPath(); ctx.arc(0,0,coinR,0,Math.PI*2);
  ctx.fillStyle=cg; ctx.shadowColor='rgba(245,200,60,0.6)'; ctx.shadowBlur=20; ctx.fill();
  ctx.strokeStyle='#c8900a'; ctx.lineWidth=3; ctx.stroke();
  ctx.shadowBlur=0; ctx.globalAlpha=Math.abs(Math.cos(spinRaw));
  ctx.font=`bold ${Math.floor(coinR*0.8)}px serif`; ctx.textAlign='center'; ctx.textBaseline='middle';
  ctx.fillStyle=isFront?'rgba(100,50,0,0.85)':'rgba(80,40,0,0.7)';
  ctx.fillText(isFront?'🎲':'?',0,2); ctx.restore();

  if (t>SPIN_DUR) {
    const resultT=(t-SPIN_DUR)/(1-SPIN_DUR);
    const ringT=Math.min(resultT*1.8,1);
    ctx.save(); ctx.translate(cx,coinY);
    ctx.beginPath(); ctx.arc(0,0,lerp(coinR,coinR*3.5,easeOut(ringT)),0,Math.PI*2);
    ctx.strokeStyle=`rgba(${primaryRgb},${(1-ringT)*0.8})`; ctx.lineWidth=4;
    ctx.shadowColor=primaryColor; ctx.shadowBlur=20; ctx.stroke(); ctx.restore();

    if (resultT>0.1) {
      const ring2T=Math.min((resultT-0.1)*2,1);
      ctx.save(); ctx.translate(cx,coinY);
      ctx.beginPath(); ctx.arc(0,0,lerp(coinR,coinR*3,easeOut(ring2T)),0,Math.PI*2);
      ctx.strokeStyle=`rgba(${primaryRgb},${(1-ring2T)*0.5})`; ctx.lineWidth=2; ctx.stroke(); ctx.restore();
    }

    ctx.save(); ctx.translate(cx,coinY);
    const iconScale=lerp(0,1,easeOut(Math.min(resultT*3,1)));
    ctx.scale(iconScale,iconScale);
    ctx.font=`bold ${Math.floor(coinR*1.1)}px serif`; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor=primaryColor; ctx.shadowBlur=30;
    ctx.fillText(isWin?'🏆':'💸',0,2); ctx.restore();

    const textAlpha=Math.min(resultT*3,1);
    ctx.save();
    ctx.font='bold 40px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor=`rgba(${primaryRgb},0.9)`; ctx.shadowBlur=28;
    ctx.fillStyle=`rgba(255,255,255,${textAlpha})`;
    ctx.fillText(isWin?'YOU WIN!':'YOU LOSE!', cx, coinY-coinR-50);
    if (label && resultT>0.2) {
      const subA=Math.min((resultT-0.2)*4,1);
      ctx.font='600 16px "Segoe UI",sans-serif'; ctx.shadowBlur=12;
      ctx.fillStyle=`rgba(${primaryRgb},${subA})`; ctx.fillText(label, cx, coinY-coinR-4);
    }
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));
}

// ─── REVERSE ─────────────────────────────────────────────────────────────────
function paintReverse(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, ps: Particle[]) {
  const cx = W/2, cy = H/2;
  const bgA=Math.min(t*4,1)*0.72;
  ctx.fillStyle=`rgba(0,5,20,${bgA})`; ctx.fillRect(0,0,W,H);

  const numScans=6;
  for (let s=0; s<numScans; s++) {
    const offset=((t*-1.8+s/numScans)%1+1)%1;
    const scanY=offset*H, scanA=0.35*Math.max(0,1-t*1.5);
    ctx.fillStyle=`rgba(100,180,255,${scanA})`; ctx.fillRect(0,scanY-1.5,W,3);
    const g=ctx.createLinearGradient(0,scanY-8,0,scanY+8);
    g.addColorStop(0,'transparent'); g.addColorStop(0.5,`rgba(100,180,255,${scanA*0.5})`); g.addColorStop(1,'transparent');
    ctx.fillStyle=g; ctx.fillRect(0,scanY-8,W,16);
  }

  for (let r=0; r<3; r++) {
    const rt=clamp((t-r*0.12)/0.55,0,1);
    if (rt<=0) continue;
    const radius=lerp(30,200+r*50,easeOut(rt));
    const ra=(1-rt)*0.6;
    ctx.save(); ctx.translate(cx,cy-20);
    const arcA=-t*Math.PI*4;
    ctx.beginPath(); ctx.arc(0,0,radius,arcA,arcA+Math.PI*1.8);
    ctx.strokeStyle=`rgba(96,165,250,${ra})`; ctx.lineWidth=2-r*0.4;
    ctx.shadowColor=`rgba(96,165,250,${ra})`; ctx.shadowBlur=12; ctx.stroke();
    const tipX=Math.cos(arcA+Math.PI*1.8)*radius, tipY=Math.sin(arcA+Math.PI*1.8)*radius;
    ctx.fillStyle=`rgba(96,165,250,${ra*1.2})`; ctx.beginPath(); ctx.arc(tipX,tipY,4-r,0,Math.PI*2); ctx.fill();
    ctx.restore();
  }

  const iconT=easeOut(Math.min(t*2.5,1))*(t>0.7?1-(t-0.7)/0.3:1);
  if (iconT>0) {
    ctx.save(); ctx.translate(cx,cy-20);
    const pulse=1+Math.sin(t*Math.PI*6)*0.06;
    ctx.scale(iconT*pulse,iconT*pulse);
    ctx.font='72px serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(96,165,250,0.9)'; ctx.shadowBlur=40; ctx.globalAlpha=iconT;
    ctx.fillText('⏪',0,0); ctx.restore();
  }

  if (t<0.6) {
    for (let b=0; b<Math.floor(rand(2,6)); b++) {
      if (Math.random()>0.35) continue;
      ctx.save();
      ctx.fillStyle=`rgba(${Math.random()>0.5?'96,165,250':'255,100,100'},${rand(0.15,0.45)})`;
      ctx.fillRect((Math.random()-0.5)*30, Math.random()*H, W, rand(2,18));
      ctx.restore();
    }
  }

  if (t>0.25 && t<0.9) {
    const tt=(t-0.25)/0.65; const ta=tt<0.3?tt/0.3:1-(tt-0.3)/0.7;
    const wobbleX=Math.sin(t*Math.PI*12)*(1-t)*5;
    ctx.save();
    ctx.font='bold 38px "Segoe UI",sans-serif'; ctx.textAlign='center'; ctx.textBaseline='middle';
    ctx.shadowColor='rgba(96,165,250,0.95)'; ctx.shadowBlur=30;
    ctx.fillStyle=`rgba(255,255,255,${ta})`; ctx.fillText('TIME REVERSED',cx+wobbleX,cy+80);
    ctx.font='600 16px "Segoe UI",sans-serif'; ctx.shadowBlur=14;
    ctx.fillStyle=`rgba(96,165,250,${ta})`; ctx.fillText("Opponent's last move undone",cx+wobbleX*0.5,cy+118);
    ctx.restore();
  }

  ps.forEach(p => drawParticle(ctx, p));
}

// ═════════════════════════════════════════════════════════════════════════════
// PARTICLE SEEDS
// ═════════════════════════════════════════════════════════════════════════════
function seedParticles(anim: CardAnimType, W: number, H: number): Particle[] {
  const cx=W/2, cy=H/2;
  const ps: Particle[]=[];
  const burst=(count:number,cx2:number,cy2:number,colors:string[],shapes:Particle['shape'][],spdMin=2,spdMax=8,gravMul=1)=>{
    for (let i=0;i<count;i++) {
      const a=Math.random()*Math.PI*2, spd=rand(spdMin,spdMax);
      ps.push(spawnParticle({
        x:cx2, y:cy2,
        vx:Math.cos(a)*spd, vy:Math.sin(a)*spd-rand(0,2),
        size:rand(3,9),
        color:colors[Math.floor(Math.random()*colors.length)],
        shape:shapes[Math.floor(Math.random()*shapes.length)],
        spin:(Math.random()-0.5)*0.25,
        alpha:rand(0.8,1),
        gravity:0.15*gravMul,
      }));
    }
  };
  const inward=(count:number,colors:string[])=>{
    for (let i=0;i<count;i++) {
      const a=Math.random()*Math.PI*2, dist=rand(100,350);
      ps.push(spawnParticle({
        x:cx+Math.cos(a)*dist, y:cy+Math.sin(a)*dist,
        vx:-Math.cos(a)*rand(1,4), vy:-Math.sin(a)*rand(1,4),
        size:rand(2,7),
        color:colors[Math.floor(Math.random()*colors.length)],
        shape:Math.random()>0.6?'line':'circle',
        angle:a+Math.PI, spin:(Math.random()-0.5)*0.15,
        alpha:rand(0.5,0.9), gravity:0,
      }));
    }
  };

  if (anim==='shield')
    burst(60,cx,cy-20,['#4ade80','#f0fdf4'],['star','circle'],1.5,5.5);
  else if (anim==='freeze')
    burst(50,cx,cy,['#93c5fd','#e0f2fe','#bfdbfe'],['hex','circle','star'],2,7,0.08);
  else if (anim==='gambler_win'||anim==='gambler_lose') {
    const isW=anim==='gambler_win';
    burst(80,cx,cy-20,isW?['#2ecc71','#f1c40f']:['#e74c3c','#e67e22'],['star','card','circle'],2,8);
  } else if (anim==='reverse')
    inward(50,['#60a5fa','#e0f2fe']);
  else if (anim==='bomb_explode') {
    burst(60,cx,cy,['#ff6600','#ffdd00','#ffffff'],['circle','star'],3,10,0.2);
    burst(20,cx,cy,['rgba(60,60,60,0.8)'],['circle'],0.5,2,0.05);
  } else if (anim==='lava_kill')
    burst(50,cx,cy,['#ff4500','#ffcc00','#ff8800'],['circle','star'],2,9,0.2);
  else if (anim==='swap')
    burst(60,cx,cy,['#a78bfa','#34d399','#ffffff'],['circle','star','hex'],2,7);
  else if (anim==='teleport') {
    for (let i=0;i<60;i++) {
      const a=Math.random()*Math.PI*2, dist=rand(20,80);
      ps.push(spawnParticle({
        x:cx+Math.cos(a)*dist, y:cy+Math.sin(a)*dist,
        vx:Math.cos(a)*rand(2,6), vy:Math.sin(a)*rand(2,6),
        size:rand(2,6), color:Math.random()>0.5?'#22d3ee':'#818cf8',
        shape:Math.random()>0.5?'circle':'star', spin:(Math.random()-0.5)*0.3,
        alpha:rand(0.7,1), gravity:0.05,
      }));
    }
  } else if (anim==='mindcontrol')
    burst(50,cx,cy,['#a78bfa','#f59e0b','#c084fc'],['circle','star','hex'],1,6);
  else if (anim==='clone')
    burst(60,cx,cy,['#34d399','#60a5fa','#a7f3d0'],['circle','hex','star'],1.5,6);
  else if (anim==='sniper')
    burst(40,cx,cy,['#ff3333','#ff8800','#ffee00'],['circle','star'],3,10);
  else if (anim==='bigsacrifice'||anim==='smallsacrifice')
    burst(60,cx,cy,['#f59e0b','#ef4444','#fbbf24'],['star','circle','hex'],2,8);
  else if (anim==='blackhole')
    inward(70,['#60a5fa','#818cf8','#c084fc']);
  else if (anim==='fullfusion')
    burst(70,cx,cy,['#93c5fd','#ffffff','#fef08a'],['circle','star','hex'],2,9);
  return ps;
}

// ═════════════════════════════════════════════════════════════════════════════
// DURATION
// ═════════════════════════════════════════════════════════════════════════════
function getDuration(anim: CardAnimType): number {
  switch(anim) {
    case 'gambler_win':    return 2400;
    case 'gambler_lose':   return 2200;
    case 'reverse':        return 2000;
    case 'mindcontrol':    return 2200;
    case 'blackhole':      return 2200;
    case 'fullfusion':     return 2000;
    case 'bomb_explode':   return 1800;
    case 'lava_kill':      return 1800;
    default:               return 1800;
  }
}

// ═════════════════════════════════════════════════════════════════════════════
// COMPONENT
// ═════════════════════════════════════════════════════════════════════════════
export const CardAnimOverlay: React.FC<Props> = ({ anim, label='', onDone }) => {
  const canvasRef = React.useRef<HTMLCanvasElement>(null);
  const rafRef    = React.useRef<number>(0);
  const startRef  = React.useRef<number>(0);
  const psRef     = React.useRef<Particle[]>([]);
  const doneRef   = React.useRef(false);
  const DURATION  = React.useMemo(() => getDuration(anim), [anim]);

  // Seed particles when anim changes
  React.useEffect(() => {
    if (!anim) return;
    doneRef.current = false;
    startRef.current = 0;
    const canvas = canvasRef.current;
    if (!canvas) return;
    psRef.current = seedParticles(anim, canvas.width, canvas.height);
  }, [anim]);

  // Animation loop
  React.useEffect(() => {
    if (!anim) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d')!;
    const W = canvas.width, H = canvas.height;
    const isWin = anim === 'gambler_win';

    // ── TEMPORARILY DISABLED ──────────────────────────────────────────────────
    if (anim !== 'gambler_win' && anim !== 'gambler_lose') { onDone(); return; }
    // ─────────────────────────────────────────────────────────────────────────

    let last = performance.now();

    const tick = (now: number) => {
      if (startRef.current === 0) startRef.current = now;
      const t = clamp((now - startRef.current) / DURATION, 0, 1);
      const dt = clamp((now - last) / 16.67, 0.5, 3);
      last = now;

      ctx.clearRect(0, 0, W, H);

      // Tick particles
      tickParticles(psRef.current, dt);

      // Paint
      // ── TEMPORARILY DISABLED ─────────────────────────────────────────────
      switch (anim) {
        // case 'shield':         paintShield(ctx,W,H,t,psRef.current); break;
        // case 'freeze':         paintFreeze(ctx,W,H,t,psRef.current); break;
        // case 'bomb_explode':   paintBombExplode(ctx,W,H,t,psRef.current); break;
        // case 'lava_kill':      paintLavaKill(ctx,W,H,t,psRef.current); break;
        // case 'swap':           paintSwap(ctx,W,H,t,psRef.current,label); break;
        // case 'teleport':       paintTeleport(ctx,W,H,t,psRef.current); break;
        // case 'mindcontrol':    paintMindControl(ctx,W,H,t,psRef.current); break;
        // case 'clone':          paintClone(ctx,W,H,t,psRef.current); break;
        // case 'sniper':         paintSniper(ctx,W,H,t,psRef.current); break;
        // case 'bigsacrifice':
        // case 'smallsacrifice': paintSacrifice(ctx,W,H,t,psRef.current,label); break;
        // case 'blackhole':      paintBlackHole(ctx,W,H,t,psRef.current); break;
        // case 'fullfusion':     paintFullFusion(ctx,W,H,t,psRef.current); break;
        // case 'reverse':        paintReverse(ctx,W,H,t,psRef.current); break;
        case 'gambler_win':
        case 'gambler_lose':     paintGambler(ctx,W,H,t,isWin,psRef.current,label); break;
      }
      // ── END TEMPORARILY DISABLED ──────────────────────────────────────────

      if (t < 1) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        ctx.clearRect(0,0,W,H);
        if (!doneRef.current) { doneRef.current=true; onDone(); }
      }
    };

    rafRef.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafRef.current);
  }, [anim, DURATION, label, onDone]);

  if (!anim) return null;

  return (
    <div style={{ position:'fixed', inset:0, zIndex:9000, pointerEvents:'none' }}>
      <canvas
        ref={canvasRef}
        width={window.innerWidth}
        height={window.innerHeight}
        style={{ position:'absolute', inset:0, width:'100%', height:'100%' }}
      />
    </div>
  );
};

export default CardAnimOverlay;