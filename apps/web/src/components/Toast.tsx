'use client';

import React from 'react';

export interface ToastMessage {
  id: string;
  text: string;
  type: 'info' | 'success' | 'warning' | 'error';
}

interface ToastContainerProps {
  messages: ToastMessage[];
  onDismiss: (id: string) => void;
}

const TOAST_COLORS: Record<string, { bg: string; border: string; text: string }> = {
  info:    { bg: 'rgba(59,130,246,0.12)', border: 'rgba(59,130,246,0.3)',  text: '#93c5fd' },
  success: { bg: 'rgba(34,197,94,0.12)',  border: 'rgba(34,197,94,0.3)',   text: '#86efac' },
  warning: { bg: 'rgba(245,158,11,0.12)', border: 'rgba(245,158,11,0.3)',  text: '#fcd34d' },
  error:   { bg: 'rgba(239,68,68,0.12)',  border: 'rgba(239,68,68,0.3)',   text: '#fca5a5' },
};

export function ToastContainer({ messages, onDismiss }: ToastContainerProps) {
  if (messages.length === 0) return null;
  return (
    <div style={{
      position: 'fixed', bottom: '20px', right: '20px', zIndex: 9999,
      display: 'flex', flexDirection: 'column', gap: '8px', maxWidth: '360px',
    }}>
      {messages.map(msg => {
        const colors = TOAST_COLORS[msg.type] || TOAST_COLORS.info;
        return (
          <div key={msg.id} style={{
            padding: '10px 14px',
            borderRadius: '10px',
            background: colors.bg,
            border: `1px solid ${colors.border}`,
            color: colors.text,
            fontSize: '12px',
            lineHeight: 1.5,
            display: 'flex',
            gap: '10px',
            alignItems: 'flex-start',
            boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
            backdropFilter: 'blur(8px)',
            animation: 'toastSlideIn 0.25s ease-out',
          }}>
            <div style={{ flex: 1 }}>{msg.text}</div>
            <button onClick={() => onDismiss(msg.id)} style={{
              background: 'none', border: 'none', color: 'rgba(255,255,255,0.4)',
              cursor: 'pointer', fontSize: '14px', padding: '0', lineHeight: 1,
            }}>✕</button>
          </div>
        );
      })}
    </div>
  );
}
