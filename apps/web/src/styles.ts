// ─── Global CSS Animations ────────────────────────────────────────────────────

export const GLOBAL_STYLES = `
  @keyframes cardReveal { from { opacity:0; transform:scale(0.95); } to { opacity:1; transform:scale(1); } }
  @keyframes pulse { 0%,100% { opacity:1; } 50% { opacity:0.4; } }
  @keyframes frozenPulse {
    0%   { box-shadow: inset 0 0 0 2px #60a5fa88, 0 0 0 0px #93c5fdaa; opacity: 0.85; }
    50%  { box-shadow: inset 0 0 0 3px #bfdbfecc, 0 0 10px 3px #3b82f680; opacity: 1; }
    100% { box-shadow: inset 0 0 0 2px #60a5fa88, 0 0 0 0px #93c5fdaa; opacity: 0.85; }
  }
  @keyframes iceShimmer {
    0%   { background-position: 200% 0; opacity: 0.5; }
    50%  { opacity: 0.85; }
    100% { background-position: -200% 0; opacity: 0.5; }
  }
  @keyframes iceCrystal {
    0%,100% { transform: scale(1) rotate(0deg); opacity: 0.9; }
    25%  { transform: scale(1.15) rotate(5deg); opacity: 1; }
    75%  { transform: scale(0.9) rotate(-5deg); opacity: 0.75; }
  }
  @keyframes frozenBreath {
    0%,100% { transform: scaleX(1); opacity: 0.6; }
    50% { transform: scaleX(1.08); opacity: 1; }
  }
  @keyframes lavaGlow { 0%,100% { opacity:0.7; transform:scale(1); } 50% { opacity:1; transform:scale(1.04); } }
  @keyframes lavaBubble { 0% { transform:translateY(0) scale(1); opacity:1; } 100% { transform:translateY(-8px) scale(1.3); opacity:0; } }
  @keyframes lavaExplode { 0% { transform:scale(1); opacity:1; filter:brightness(1); } 40% { transform:scale(1.6); opacity:1; filter:brightness(3) saturate(2); } 100% { transform:scale(2.5); opacity:0; filter:brightness(1); } }
  @keyframes radarSweep { 0% { transform:rotate(0deg); } 100% { transform:rotate(360deg); } }
  @keyframes radarPing { 0% { transform:scale(0.5); opacity:1; } 100% { transform:scale(2); opacity:0; } }
  @keyframes radarReveal { 0% { opacity:0; transform:translateY(-10px) scale(0.9); } 100% { opacity:1; transform:translateY(0) scale(1); } }
  @keyframes dmPulse { 0%,100% { box-shadow: inset 0 0 0 3px rgba(74,222,128,0.9); } 50% { box-shadow: inset 0 0 0 4px rgba(74,222,128,1), 0 0 12px rgba(74,222,128,0.5); } }
  @keyframes dmBlock { 0%,100% { box-shadow: inset 0 0 0 3px rgba(231,76,60,0.9); } 50% { box-shadow: inset 0 0 0 4px rgba(231,76,60,1), 0 0 12px rgba(231,76,60,0.5); } }

  /* ── SWAP animations ── */
  @keyframes swapSquarePulse {
    0%   { opacity: 0; transform: scale(0.85); }
    30%  { opacity: 1; transform: scale(1.05); }
    70%  { opacity: 1; transform: scale(1); }
    100% { opacity: 0; transform: scale(1.1); }
  }
  @keyframes swapArcDraw {
    0%   { stroke-dashoffset: 600; opacity: 1; }
    55%  { stroke-dashoffset: 0;   opacity: 1; }
    80%  { stroke-dashoffset: 0;   opacity: 0.6; }
    100% { stroke-dashoffset: 0;   opacity: 0; }
  }
  @keyframes swapDotTravel {
    0%   { opacity: 0; }
    10%  { opacity: 1; }
    85%  { opacity: 1; }
    100% { opacity: 0; }
  }
  @keyframes swapBurst {
    0%   { r: 0;  opacity: 1; }
    40%  { r: 22; opacity: 0.8; }
    100% { r: 38; opacity: 0; }
  }
  @keyframes swapSpark {
    0%   { opacity: 1; transform: translate(0,0) scale(1); }
    100% { opacity: 0; transform: translate(var(--tx), var(--ty)) scale(0.2); }
  }
  @keyframes swapRingPop {
    0%   { r: 8;  stroke-width: 3; opacity: 0.9; }
    60%  { r: 28; stroke-width: 1.5; opacity: 0.5; }
    100% { r: 42; stroke-width: 0.5; opacity: 0; }
  }
  @keyframes swapCenterFlash {
    0%   { opacity: 0; r: 4;  }
    20%  { opacity: 1; r: 16; }
    60%  { opacity: 0.5; r: 12; }
    100% { opacity: 0; r: 6;  }
  }

  /* ── BOMB animations ── */
  @keyframes bombTick {
    0%   { transform: scale(1) rotate(0deg);   filter: brightness(1); }
    45%  { transform: scale(1.18) rotate(-8deg); filter: brightness(1.4) drop-shadow(0 0 6px #ff6600); }
    50%  { transform: scale(1.22) rotate(8deg);  filter: brightness(1.6) drop-shadow(0 0 8px #ff4400); }
    55%  { transform: scale(1.18) rotate(-4deg); filter: brightness(1.4) drop-shadow(0 0 6px #ff6600); }
    100% { transform: scale(1) rotate(0deg);   filter: brightness(1); }
  }
  @keyframes bombFuse {
    0%   { opacity: 1; transform: scaleY(1); }
    100% { opacity: 0.3; transform: scaleY(0.2); }
  }
  @keyframes bombGlow {
    0%,100% { box-shadow: inset 0 0 0 2px rgba(255,100,0,0.6), 0 0 8px rgba(255,80,0,0.4); }
    50%      { box-shadow: inset 0 0 0 3px rgba(255,60,0,0.9), 0 0 18px rgba(255,60,0,0.7); }
  }
  @keyframes bombExplode {
    0%   { transform: scale(1);   opacity: 1;   filter: brightness(1); }
    15%  { transform: scale(0.8); opacity: 1;   filter: brightness(3) saturate(3); }
    30%  { transform: scale(2.8); opacity: 1;   filter: brightness(4) saturate(3) hue-rotate(20deg); }
    55%  { transform: scale(3.5); opacity: 0.8; filter: brightness(3) saturate(2); }
    80%  { transform: scale(4.2); opacity: 0.3; filter: brightness(1.5); }
    100% { transform: scale(5);   opacity: 0;   filter: brightness(1); }
  }
  @keyframes bombShockwave {
    0%   { transform: scale(0.5); opacity: 0.9; border-width: 4px; }
    60%  { opacity: 0.5; }
    100% { transform: scale(4.5); opacity: 0; border-width: 1px; }
  }
  @keyframes bombSmoke {
    0%   { transform: translateY(0) scale(1) rotate(0deg);   opacity: 0.9; }
    40%  { transform: translateY(-20px) scale(1.4) rotate(15deg); opacity: 0.7; }
    100% { transform: translateY(-50px) scale(2.5) rotate(40deg); opacity: 0; }
  }
  @keyframes bombFlash {
    0%,100% { opacity: 0; }
    10%,30% { opacity: 0.95; background: radial-gradient(circle at 50% 50%, #fff 0%, #ffdd00 30%, #ff6600 60%, transparent 80%); }
    20%     { opacity: 0.6; }
  }
  @keyframes bombRadiusAura {
    0%,100% { opacity: 0.25; }
    50%      { opacity: 0.55; }
  }

  /* ── JOKER animations ── */
  @keyframes jokerSpin {
    0%   { transform: rotateY(0deg) scale(1); }
    50%  { transform: rotateY(180deg) scale(1.1); }
    100% { transform: rotateY(360deg) scale(1); }
  }
  @keyframes jokerFloat {
    0%,100% { transform: translateY(0px); }
    50%      { transform: translateY(-8px); }
  }
  @keyframes jokerGlitter {
    0%   { opacity: 0; transform: scale(0) rotate(0deg); }
    30%  { opacity: 1; transform: scale(1.2) rotate(180deg); }
    70%  { opacity: 0.8; transform: scale(0.9) rotate(300deg); }
    100% { opacity: 0; transform: scale(0) rotate(360deg); }
  }
  @keyframes jokerPickerReveal {
    0%   { opacity: 0; transform: scale(0.85) translateY(20px); }
    100% { opacity: 1; transform: scale(1) translateY(0); }
  }
  @keyframes jokerCardHover {
    0%,100% { box-shadow: 0 4px 20px rgba(245,158,11,0.3); }
    50%      { box-shadow: 0 8px 32px rgba(245,158,11,0.7), 0 0 20px rgba(245,158,11,0.4); }
  }
  @keyframes jokerTransform {
    0%   { transform: scale(1) rotateZ(0deg);   filter: brightness(1); }
    25%  { transform: scale(1.3) rotateZ(15deg);  filter: brightness(2) hue-rotate(60deg); }
    50%  { transform: scale(0.5) rotateZ(-30deg); filter: brightness(3) hue-rotate(180deg); }
    75%  { transform: scale(1.2) rotateZ(10deg);  filter: brightness(2) hue-rotate(300deg); }
    100% { transform: scale(1) rotateZ(0deg);   filter: brightness(1); }
  }
`;

