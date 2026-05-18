import React from 'react';

interface CardHandProps {
  cards: any[]; // Use actual GameCard type in a real integration
  isActivePlayer: boolean;
  onCardClick: (index: number) => void;
  selectedCardIndex: number | null;
}

export function CardHand({
  cards,
  isActivePlayer,
  onCardClick,
  selectedCardIndex,
}: CardHandProps) {
  return (
    <div className={`card-hand ${!isActivePlayer ? 'card-hand--disabled' : ''}`}>
      {cards.length === 0 ? (
        <div className="card-hand__empty">
          <span className="caption">No cards in hand. Play moves to draw cards.</span>
        </div>
      ) : (
        <div className="card-hand__cards">
          {cards.map((card, index) => {
            const isSelected = index === selectedCardIndex;
            return (
              <button
                key={index}
                className={`card-hand__item ${isSelected ? 'is-selected' : ''}`}
                onClick={() => onCardClick(index)}
                disabled={!isActivePlayer}
              >
                <div className="card-hand__item-inner" style={{ background: card.color || '#333' }}>
                  <span className="card-hand__item-icon">{card.icon}</span>
                  <span className="card-hand__item-name">{card.name}</span>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
