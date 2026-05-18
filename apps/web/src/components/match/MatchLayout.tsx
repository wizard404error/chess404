import React from 'react';

interface MatchLayoutProps {
  leftPanel: React.ReactNode;
  board: React.ReactNode;
  topPlayerBar: React.ReactNode;
  bottomPlayerBar: React.ReactNode;
  cardHand: React.ReactNode;
  rightPanel: React.ReactNode;
  gameControls: React.ReactNode;
}

export function MatchLayout({
  leftPanel,
  board,
  topPlayerBar,
  bottomPlayerBar,
  cardHand,
  rightPanel,
  gameControls,
}: MatchLayoutProps) {
  return (
    <div className="match-layout fade-in">
      <div className="match-layout__left">
        {leftPanel}
      </div>
      
      <div className="match-layout__center">
        {topPlayerBar}
        <div className="match-layout__board-wrapper">
          {board}
        </div>
        {bottomPlayerBar}
        {cardHand}
      </div>
      
      <div className="match-layout__right">
        {rightPanel}
        <div className="match-layout__controls">
          {gameControls}
        </div>
      </div>
    </div>
  );
}
