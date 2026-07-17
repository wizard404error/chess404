'use client';

import React from 'react';
import type { TutorialState } from '../hooks/useTutorial';

interface Props {
  tutorial: TutorialState;
  activePage: string;
}

const STEPS: Record<string, { title: string; body: string; tip?: string }> = {
  welcome: {
    title: 'Welcome to CardChess! ♟️✨',
    body: 'CardChess blends classic chess with collectible card mechanics. Each player draws a hand of cards as the game progresses — spells and traps that can turn the tide of battle.',
    tip: 'Start by playing vs the computer to learn the ropes.',
  },
  board: {
    title: 'The Board & Pieces',
    body: 'The 8×8 board works just like standard chess. Drag or click a piece to see legal moves, then click a highlighted square to move. Cards become available starting round 3.',
    tip: 'Your pieces are color-coded — white at the bottom, black at the top.',
  },
  cards: {
    title: 'Your Hand of Cards',
    body: 'Cards appear below (or above) the board. Click one to preview its effect in the left panel, then use it by clicking "use card". Some cards require targeting a square or piece on the board.',
    tip: 'Hover any card for a quick tooltip. Cards can only be played on your turn, one per turn.',
  },
};

export function OnboardingTutorial({ tutorial, activePage }: Props) {
  const { active, step, next, dismiss } = tutorial;

  if (!active || step === 'complete') return null;

  const s = STEPS[step];
  if (!s) return null;

  const isMatchPage = activePage === 'Match';

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 10000,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'rgba(0,0,0,0.7)',
      backdropFilter: 'blur(4px)',
      animation: 'fadeIn 0.3s ease',
    }}>
      <style>{`
        @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
        @keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
      `}</style>
      <div style={{
        background: 'linear-gradient(160deg, rgba(15,8,30,0.98) 0%, rgba(8,6,18,0.99) 100%)',
        border: '1px solid rgba(200,134,10,0.4)',
        borderRadius: '20px',
        padding: '32px',
        maxWidth: '480px',
        width: '90vw',
        boxShadow: '0 24px 64px rgba(0,0,0,0.6), 0 0 40px rgba(200,134,10,0.15)',
        animation: 'slideUp 0.35s cubic-bezier(0.34,1.56,0.64,1)',
      }}>
        <div style={{ fontSize: '28px', marginBottom: '8px', textAlign: 'center' }}>{step === 'welcome' ? '♟️' : step === 'board' ? '🏰' : '🃏'}</div>
        <h2 style={{ color: '#ffd700', fontSize: '20px', fontWeight: 800, textAlign: 'center', margin: '0 0 12px' }}>{s.title}</h2>
        <p style={{ color: 'rgba(200,195,220,0.95)', fontSize: '13px', lineHeight: 1.7, textAlign: 'center', margin: '0 0 16px' }}>{s.body}</p>
        {s.tip && (
          <div style={{
            background: 'rgba(200,134,10,0.1)',
            border: '1px solid rgba(200,134,10,0.25)',
            borderRadius: '8px',
            padding: '10px 14px',
            marginBottom: '20px',
            fontSize: '11px',
            color: '#ffcf72',
            textAlign: 'center',
            fontWeight: 600,
          }}>
            💡 {s.tip}
          </div>
        )}
        <div style={{ display: 'flex', gap: '10px', justifyContent: 'center' }}>
          <button onClick={dismiss} style={{
            padding: '10px 20px',
            background: 'rgba(255,255,255,0.06)',
            color: 'rgba(200,195,220,0.7)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: '8px',
            cursor: 'pointer',
            fontSize: '12px',
            fontWeight: 600,
          }}>
            Skip
          </button>
          <button onClick={next} style={{
            padding: '10px 28px',
            background: 'linear-gradient(135deg, #c8860a, #8b5e0a)',
            color: '#fff',
            border: 'none',
            borderRadius: '10px',
            cursor: 'pointer',
            fontSize: '13px',
            fontWeight: 700,
          }}>
            {step === 'cards' ? 'Got it! ✨' : 'Next →'}
          </button>
        </div>
        <div style={{ display: 'flex', gap: '6px', justifyContent: 'center', marginTop: '16px' }}>
          {['welcome', 'board', 'cards'].map(s => (
            <div key={s} style={{
              width: '8px', height: '8px', borderRadius: '50%',
              background: step === s ? '#ffd700' : 'rgba(255,255,255,0.15)',
              transition: 'background 0.2s',
            }} />
          ))}
        </div>
      </div>
    </div>
  );
}
