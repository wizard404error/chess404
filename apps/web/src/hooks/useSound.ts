'use client';

import React from 'react';

type SoundType = 'move' | 'capture' | 'check' | 'timer_warning' | 'game_over' | 'card_play' | 'chat' | 'error';

let audioCtx: AudioContext | null = null;
let initAttempted = false;

function getAudioCtx(): AudioContext {
  if (!audioCtx) {
    audioCtx = new (window.AudioContext || (window as any).webkitAudioContext)();
  }
  if (audioCtx.state === 'suspended') {
    audioCtx.resume();
  }
  return audioCtx;
}

// Initialize audio context on first user gesture (required for iOS)
if (typeof document !== 'undefined') {
  const initAudio = () => {
    if (!initAttempted) {
      initAttempted = true;
      const ctx = getAudioCtx();
      if (ctx.state === 'suspended') ctx.resume();
    }
  };
  document.addEventListener('click', initAudio, { once: true });
  document.addEventListener('touchstart', initAudio, { once: true });
  document.addEventListener('keydown', initAudio, { once: true });
}

function playTone(freq: number, duration: number, type: OscillatorType = 'sine', volume = 0.08) {
  try {
    const ctx = getAudioCtx();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(freq, ctx.currentTime);
    gain.gain.setValueAtTime(volume, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + duration);
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.start();
    osc.stop(ctx.currentTime + duration);
  } catch {}
}

function playNoise(duration: number, volume = 0.04) {
  try {
    const ctx = getAudioCtx();
    const bufferSize = ctx.sampleRate * duration;
    const buffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }
    const source = ctx.createBufferSource();
    source.buffer = buffer;
    const gain = ctx.createGain();
    gain.gain.setValueAtTime(volume, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + duration);
    source.connect(gain);
    gain.connect(ctx.destination);
    source.start();
  } catch {}
}

const SOUND_DEFS: Record<SoundType, () => void> = {
  move: () => {
    playTone(800, 0.06, 'sine', 0.06);
  },
  capture: () => {
    playTone(300, 0.1, 'triangle', 0.08);
    playTone(200, 0.12, 'sine', 0.04);
  },
  check: () => {
    playTone(1000, 0.1, 'square', 0.05);
    setTimeout(() => playTone(1200, 0.1, 'square', 0.05), 100);
  },
  timer_warning: () => {
    playTone(600, 0.05, 'square', 0.03);
  },
  game_over: () => {
    playTone(523, 0.15, 'sine', 0.07);
    setTimeout(() => playTone(659, 0.15, 'sine', 0.07), 150);
    setTimeout(() => playTone(784, 0.3, 'sine', 0.07), 300);
  },
  card_play: () => {
    playTone(500, 0.08, 'sine', 0.05);
    setTimeout(() => playTone(700, 0.08, 'sine', 0.05), 60);
    setTimeout(() => playTone(900, 0.1, 'sine', 0.05), 120);
    playNoise(0.15, 0.02);
  },
  chat: () => {
    playTone(880, 0.06, 'sine', 0.04);
  },
  error: () => {
    playTone(150, 0.15, 'sawtooth', 0.05);
    playNoise(0.12, 0.03);
  },
};

let soundEnabled = true;

export function setSoundEnabled(enabled: boolean) {
  soundEnabled = enabled;
}

export function isSoundEnabled(): boolean {
  return soundEnabled;
}

export function playSound(type: SoundType) {
  if (!soundEnabled || typeof window === 'undefined') return;
  SOUND_DEFS[type]();
}

export function useSound() {
  const [enabled, setEnabled] = React.useState(true);

  const toggle = React.useCallback(() => {
    setEnabled(prev => {
      const next = !prev;
      setSoundEnabled(next);
      if (next) playSound('move');
      return next;
    });
  }, []);

  React.useEffect(() => {
    setSoundEnabled(enabled);
  }, [enabled]);

  return { soundEnabled: enabled, setSoundEnabled: setEnabled, toggleSound: toggle };
}
