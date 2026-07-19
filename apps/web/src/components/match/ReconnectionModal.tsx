'use client';
import React from 'react';

interface ReconnectionModalProps {
  show: boolean;
  title?: string;
  subtitle?: string;
  onRetry?: () => void;
  retryLabel?: string;
}

export function ReconnectionModal({ show, title, subtitle, onRetry, retryLabel }: ReconnectionModalProps) {
  if (!show) return null;

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 9999,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'rgba(0,0,0,0.70)', backdropFilter: 'blur(4px)',
    }}>
      <div style={{
        background: '#1a1a2e', border: '1px solid rgba(99,102,241,0.3)',
        borderRadius: '12px', padding: '32px 40px', maxWidth: '400px',
        textAlign: 'center', boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
      }}>
        <div style={{ fontSize: '36px', marginBottom: '12px' }}>
          <span style={{ display: 'inline-block', animation: 'spin 1.5s linear infinite' }}>⟳</span>
        </div>
        <h2 style={{ color: '#e2e8f0', margin: '0 0 8px', fontSize: '18px', fontWeight: 600 }}>
          {title || 'Connection Lost'}
        </h2>
        <p style={{ color: '#94a3b8', margin: '0 0 20px', fontSize: '13px', lineHeight: 1.5 }}>
          {subtitle || 'Your live match stream was interrupted. Reconnecting in the background\u2026'}
        </p>
        {onRetry && (
          <button onClick={onRetry} style={{
            padding: '10px 24px', borderRadius: '8px', border: 'none',
            background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
            color: '#fff', fontSize: '14px', fontWeight: 600, cursor: 'pointer',
          }}>
            {retryLabel || 'Retry Connection'}
          </button>
        )}
      </div>
      <style>{`
        @keyframes spin { to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}
