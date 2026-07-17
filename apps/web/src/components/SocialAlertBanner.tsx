'use client';

import React from 'react';
import type { SocialAlert } from '../lib/match-labels';

interface Props {
  visible: SocialAlert | null;
  onAction: () => void;
  onDismiss: () => void;
}

export function SocialAlertBanner({ visible, onAction, onDismiss }: Props) {
  if (!visible) return null;

  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px',
      padding: '12px 22px',
      background: visible.action === 'match'
        ? 'linear-gradient(90deg, rgba(22,64,40,0.92) 0%, rgba(16,42,32,0.95) 100%)'
        : 'linear-gradient(90deg, rgba(70,44,12,0.92) 0%, rgba(32,22,10,0.95) 100%)',
      borderBottom: '1px solid rgba(255,180,60,0.18)',
      boxShadow: '0 8px 24px rgba(0,0,0,0.18)',
      position: 'relative', zIndex: 90,
    }}>
      <div style={{ display: 'grid', gap: '4px', minWidth: 0 }}>
        <div style={{ color: '#fff5d6', fontSize: '15px', fontWeight: 800, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {visible.title}
        </div>
        <div style={{ color: 'rgba(255,236,194,0.78)', fontSize: '12px', lineHeight: 1.5 }}>
          {visible.detail}
        </div>
      </div>
      <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', flexShrink: 0 }}>
        <button onClick={onAction} style={{
          padding: '9px 14px', borderRadius: '10px',
          border: '1px solid rgba(255,215,120,0.26)',
          background: 'rgba(255,255,255,0.08)',
          color: '#fff8de', fontSize: '12px', fontWeight: 800, cursor: 'pointer',
        }}>
          {visible.actionLabel}
        </button>
        <button onClick={onDismiss} style={{
          padding: '9px 14px', borderRadius: '10px',
          border: '1px solid rgba(255,255,255,0.10)',
          background: 'rgba(255,255,255,0.03)',
          color: 'rgba(255,236,194,0.82)', fontSize: '12px', fontWeight: 700, cursor: 'pointer',
        }}>
          Dismiss
        </button>
      </div>
    </div>
  );
}
