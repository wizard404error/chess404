import React from 'react';

export interface PlayerBarProps {
  seat: 'white' | 'black';
  playerName: string;
  rating: string | number;
  timeMs: number;
  isClockActive: boolean;
  seatBadge?: string;
}

export function formatClock(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

export function PlayerBar({
  seat,
  playerName,
  rating,
  timeMs,
  isClockActive,
  seatBadge
}: PlayerBarProps) {
  const isWhite = seat === 'white';
  const timeUrgent = timeMs <= 30000;
  
  return (
    <div className={`card-surface player-bar player-bar--${seat}`}>
      <div className="player-bar__avatar">
        {/* Placeholder avatar */}
        <span>{isWhite ? '🕵️' : '👤'}</span>
      </div>
      
      <div className="player-bar__info">
        <div className="player-bar__name-row">
          <span className="player-bar__name">{playerName}</span>
          {seatBadge && <span className={`badge ${seatBadge === 'You' ? 'badge--success' : ''}`}>{seatBadge}</span>}
        </div>
        <div className="player-bar__stats">
          <span className="player-bar__rating">♟ {rating}</span>
          <span className={`player-bar__clock mono ${timeUrgent ? 'player-bar__clock--urgent' : ''} ${isClockActive ? 'player-bar__clock--active' : ''}`}>
            ⏱ {formatClock(timeMs)}
          </span>
        </div>
      </div>
    </div>
  );
}
