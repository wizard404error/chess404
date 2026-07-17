'use client';

import React from 'react';
import type { PieceColor } from '../../types';
import { useMatchState } from '../../contexts/MatchStateContext';

interface PlayerCardInfoProps {
  seat: PieceColor;
}

export default function PlayerCardInfo({ seat }: PlayerCardInfoProps) {
  const {
    displayedWhiteName, displayedBlackName,
    displayedWhiteRating, displayedBlackRating,
    timeW, timeB, fmtClock, tickingState,
  } = useMatchState();

  const isWhite = seat === 'white';
  const name = isWhite ? displayedWhiteName : displayedBlackName;
  const rating = isWhite ? displayedWhiteRating : displayedBlackRating;
  const time = isWhite ? timeW : timeB;
  const isTicking = tickingState === seat;

  return (
    <div style={{
      display:'flex', alignItems:'center', gap:'10px',
      padding:'6px 12px', borderRadius:'10px',
      background: isTicking ? 'rgba(255,200,50,0.08)' : 'rgba(20,28,40,0.7)',
      border: isTicking ? '1px solid rgba(255,200,50,0.25)' : '1px solid rgba(255,255,255,0.08)',
      transition:'all 0.3s ease',
    }}>
      <div style={{
        width:'26px', height:'26px', borderRadius:'50%',
        background: isWhite ? 'linear-gradient(135deg, #f0f0f0, #ddd)' : 'linear-gradient(135deg, #333, #111)',
        border:'2px solid rgba(255,255,255,0.2)',
        boxShadow:'0 2px 6px rgba(0,0,0,0.3)',
      }} />
      <div style={{ flex:1 }}>
        <div style={{ fontWeight:700, fontSize:'12px', color:'#f0e6d0' }}>{name}</div>
        {rating != null && (
          <div style={{ fontSize:'10px', color:'rgba(200,190,170,0.7)' }}>Rating: {rating}</div>
        )}
      </div>
      <div style={{
        fontSize:'13px', fontWeight:800, fontFamily:'monospace',
        color: time < 30000 ? '#e74c3c' : time < 60000 ? '#f39c12' : '#f4e8c8',
        animation: time < 10000 ? 'pulse 1s ease-in-out infinite' : 'none',
      }}>
        {fmtClock(time)}
      </div>
    </div>
  );
}
