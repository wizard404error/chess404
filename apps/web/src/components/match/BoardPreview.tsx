import React from 'react';
import type { Board } from '@chess404/contracts';

const FILE_LABELS = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

interface BoardPreviewProps {
  board: Board;
}

export default function BoardPreview({ board }: BoardPreviewProps): React.ReactElement {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(8, 34px)',
        gridTemplateRows: 'repeat(8, 34px)',
        width: '272px',
        border: '1px solid rgba(255,190,90,0.26)',
        borderRadius: '10px',
        overflow: 'hidden',
        boxShadow: '0 10px 30px rgba(0,0,0,0.26)',
      }}
    >
      {board.flatMap((row, rowIndex) =>
        row.map((piece, colIndex) => {
          const dark = (rowIndex + colIndex) % 2 === 1;
          const label = `${FILE_LABELS[colIndex]}${8 - rowIndex}`;
          const src = piece ? `/pieces/${piece.color}_${piece.type}.svg` : null;
          return (
            <div
              key={label}
              title={label}
              style={{
                width: '34px',
                height: '34px',
                position: 'relative',
                background: dark ? '#b88a62' : '#efd7af',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {piece ? (
                <img
                  src={src ?? ''}
                  alt={`${piece.color} ${piece.type}`}
                  style={{ width: '28px', height: '28px', objectFit: 'contain', pointerEvents: 'none' }}
                />
              ) : null}
              {colIndex === 0 ? (
                <span
                  style={{
                    position: 'absolute',
                    top: '2px',
                    left: '3px',
                    fontSize: '8px',
                    fontWeight: 700,
                    color: dark ? 'rgba(255,245,220,0.85)' : 'rgba(120,72,22,0.82)',
                  }}
                >
                  {8 - rowIndex}
                </span>
              ) : null}
              {rowIndex === 7 ? (
                <span
                  style={{
                    position: 'absolute',
                    right: '3px',
                    bottom: '2px',
                    fontSize: '8px',
                    fontWeight: 700,
                    color: dark ? 'rgba(255,245,220,0.85)' : 'rgba(120,72,22,0.82)',
                  }}
                >
                  {FILE_LABELS[colIndex]}
                </span>
              ) : null}
            </div>
          );
        })
      )}
    </div>
  );
}
