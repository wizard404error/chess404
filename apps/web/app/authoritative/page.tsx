'use client';

import * as React from 'react';
import { useAuthoritativeMatch } from '../../src/hooks/useAuthoritativeMatch';

export default function AuthoritativePage() {
  const { snapshot, isLoading, isStreaming, error, create, refresh, sendIntent } = useAuthoritativeMatch();
  const [chatText, setChatText] = React.useState('');

  const match = snapshot?.match;
  const whiteHand = match?.whiteHand ?? [];
  const freezeCard = whiteHand.find(card => card.mechanic === 'freeze');
  const shieldCard = whiteHand.find(card => card.mechanic === 'shield');
  const sniperCard = whiteHand.find(card => card.mechanic === 'sniper');
  const badSniperCard = whiteHand.find(card => card.mechanic === 'badsniper');

  return (
    <main style={{
      minHeight: '100vh',
      background: 'linear-gradient(180deg, #08111b 0%, #101826 100%)',
      color: '#f4efe6',
      padding: '32px 24px 48px',
      fontFamily: '"Segoe UI", sans-serif'
    }}>
      <div style={{ maxWidth: 1100, margin: '0 auto' }}>
        <h1 style={{ margin: '0 0 10px', fontSize: '32px' }}>Authoritative Match Lab</h1>
        <p style={{ margin: '0 0 24px', color: 'rgba(244,239,230,0.72)', maxWidth: 780, lineHeight: 1.6 }}>
          This page talks to the backend match service directly. It is a safe bridge layer for testing server-owned state
          without changing the main Chess404 game UI yet.
        </p>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 20 }}>
          <button onClick={() => void create()} disabled={isLoading} style={primaryButton}>
            {match ? 'Create Another Match' : 'Create Match'}
          </button>
          <button onClick={() => void refresh()} disabled={isLoading || !match} style={secondaryButton}>
            Refresh Snapshot
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'make_move',
              playerId: 'white_player',
              from: { row: 1, col: 4 },
              to: { row: 3, col: 4 }
            })}
            disabled={isLoading || !match}
            style={secondaryButton}
          >
            Play e2-e4
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'make_move',
              playerId: 'black_player',
              from: { row: 6, col: 4 },
              to: { row: 4, col: 4 }
            })}
            disabled={isLoading || !match}
            style={secondaryButton}
          >
            Play e7-e5
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'offer_draw',
              playerId: 'white_player'
            })}
            disabled={isLoading || !match}
            style={secondaryButton}
          >
            Offer Draw
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'respond_draw',
              playerId: 'black_player',
              accept: true
            })}
            disabled={isLoading || !match}
            style={secondaryButton}
          >
            Accept Draw
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'resign',
              playerId: 'black_player'
            })}
            disabled={isLoading || !match}
            style={dangerButton}
          >
            Black Resigns
          </button>
        </div>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 20 }}>
          <button
            onClick={() => freezeCard && void sendIntent({
              type: 'play_card',
              playerId: 'white_player',
              cardId: freezeCard.id
            })}
            disabled={isLoading || !match || !freezeCard}
            style={secondaryButton}
          >
            Queue Freeze
          </button>
          <button
            onClick={() => shieldCard && void sendIntent({
              type: 'play_card',
              playerId: 'white_player',
              cardId: shieldCard.id
            })}
            disabled={isLoading || !match || !shieldCard}
            style={secondaryButton}
          >
            Queue Shield
          </button>
          <button
            onClick={() => sniperCard && void sendIntent({
              type: 'play_card',
              playerId: 'white_player',
              cardId: sniperCard.id
            })}
            disabled={isLoading || !match || !sniperCard}
            style={secondaryButton}
          >
            Queue Sniper
          </button>
          <button
            onClick={() => badSniperCard && void sendIntent({
              type: 'play_card',
              playerId: 'white_player',
              cardId: badSniperCard.id
            })}
            disabled={isLoading || !match || !badSniperCard}
            style={secondaryButton}
          >
            Queue Bad Sniper
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'select_target',
              playerId: 'white_player',
              target: { row: 6, col: 0 }
            })}
            disabled={isLoading || !match || !match.pendingCard}
            style={secondaryButton}
          >
            Target a7
          </button>
          <button
            onClick={() => void sendIntent({
              type: 'select_target',
              playerId: 'white_player',
              target: { row: 1, col: 0 }
            })}
            disabled={isLoading || !match || !match.pendingCard}
            style={secondaryButton}
          >
            Target a2
          </button>
        </div>

        <div style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          marginBottom: 20,
          padding: '10px 14px',
          borderRadius: 999,
          border: `1px solid ${isStreaming ? 'rgba(74,222,128,0.35)' : 'rgba(255,255,255,0.12)'}`,
          background: isStreaming ? 'rgba(74,222,128,0.08)' : 'rgba(255,255,255,0.05)',
          color: isStreaming ? '#9af0ba' : 'rgba(244,239,230,0.74)',
          fontSize: 13,
          fontWeight: 600
        }}>
          <span style={{
            width: 9,
            height: 9,
            borderRadius: '50%',
            background: isStreaming ? '#4ade80' : 'rgba(255,255,255,0.4)',
            boxShadow: isStreaming ? '0 0 12px rgba(74,222,128,0.8)' : 'none'
          }} />
          {isStreaming ? 'Live stream connected' : 'Live stream idle'}
        </div>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 24 }}>
          <input
            value={chatText}
            onChange={(event) => setChatText(event.target.value)}
            placeholder="Send backend-owned chat"
            style={{
              flex: '1 1 320px',
              minWidth: 260,
              padding: '12px 14px',
              borderRadius: 10,
              border: '1px solid rgba(255,255,255,0.15)',
              background: 'rgba(255,255,255,0.06)',
              color: '#f4efe6'
            }}
          />
          <button
            onClick={() => {
              if (!chatText.trim()) return;
              void sendIntent({
                type: 'send_chat',
                playerId: match?.turn === 'black' ? 'black_player' : 'white_player',
                text: chatText
              }).then(() => setChatText(''));
            }}
            disabled={isLoading || !match || !chatText.trim()}
            style={secondaryButton}
          >
            Send Chat
          </button>
        </div>

        {error && (
          <div style={{
            marginBottom: 20,
            padding: '14px 16px',
            borderRadius: 12,
            background: 'rgba(200,50,50,0.12)',
            border: '1px solid rgba(220,90,90,0.45)',
            color: '#ffc6c6'
          }}>
            {error}
          </div>
        )}

        <div style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(280px, 360px) minmax(0, 1fr)',
          gap: 18,
          alignItems: 'start'
        }}>
          <section style={panelStyle}>
            <h2 style={headingStyle}>Match Summary</h2>
            {match ? (
              <>
                <BoardPreview board={match.board} />
                <div style={{ display: 'grid', gap: 10, fontSize: 14, marginTop: 18 }}>
                  <InfoRow label="Match ID" value={match.matchId} mono />
                  <InfoRow label="Rules" value={match.rulesVersion} mono />
                  <InfoRow label="Turn" value={match.turn} />
                  <InfoRow label="Status" value={match.status} />
                  <InfoRow label="Winner" value={String(match.winner ?? 'none')} />
                  <InfoRow label="Replay Head" value={String(snapshot?.replayHead ?? 0)} />
                  <InfoRow label="Draw Offered By" value={String(match.drawOfferedBy ?? 'none')} />
                  <InfoRow label="Pending Card" value={match.pendingCard ? `${match.pendingCard.mechanic} by ${match.pendingCard.ownerColor}` : 'none'} />
                  <InfoRow label="Halfmove" value={String(match.halfMoveClock)} />
                  <InfoRow label="Fullmove" value={String(match.fullMoveNumber)} />
                  <InfoRow label="Clock Running" value={String(match.clock.runningFor ?? 'none')} />
                  <InfoRow label="White Clock" value={String(match.clock.whiteMs)} mono />
                  <InfoRow label="Black Clock" value={String(match.clock.blackMs)} mono />
                </div>
              </>
            ) : (
              <p style={emptyStyle}>No backend match loaded yet.</p>
            )}
          </section>

          <section style={panelStyle}>
            <h2 style={headingStyle}>Snapshot JSON</h2>
            <pre style={preStyle}>
              {snapshot ? JSON.stringify(snapshot, null, 2) : 'Create a match to inspect the authoritative snapshot.'}
            </pre>
          </section>
        </div>
      </div>
    </main>
  );
}

function BoardPreview({ board }: { board: Array<Array<{ type: string; color: string } | null>> }) {
  return (
    <div>
      <h3 style={{ margin: '0 0 12px', fontSize: 15, color: 'rgba(244,239,230,0.82)' }}>Backend Board Preview</h3>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(8, 1fr)',
        gap: 0,
        width: '100%',
        aspectRatio: '1 / 1',
        borderRadius: 14,
        overflow: 'hidden',
        border: '1px solid rgba(255,255,255,0.12)',
        boxShadow: 'inset 0 0 0 1px rgba(0,0,0,0.25)'
      }}>
        {board.map((row, rowIndex) =>
          row.map((piece, colIndex) => {
            const lightSquare = (rowIndex + colIndex) % 2 === 0;
            return (
              <div
                key={`${rowIndex}-${colIndex}`}
                style={{
                  background: lightSquare ? '#f0d9b5' : '#b58863',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  position: 'relative'
                }}
              >
                <span style={{
                  position: 'absolute',
                  top: 4,
                  left: 5,
                  fontSize: 9,
                  color: lightSquare ? 'rgba(60,40,20,0.6)' : 'rgba(255,245,225,0.55)',
                  pointerEvents: 'none'
                }}>
                  {rowIndex === 7 ? 'abcdefgh'[colIndex] : ''}
                </span>
                <span style={{
                  position: 'absolute',
                  bottom: 4,
                  right: 5,
                  fontSize: 9,
                  color: lightSquare ? 'rgba(60,40,20,0.6)' : 'rgba(255,245,225,0.55)',
                  pointerEvents: 'none'
                }}>
                  {colIndex === 0 ? rowIndex + 1 : ''}
                </span>
                {piece ? (
                  <img
                    src={`/pieces/${piece.color}_${piece.type}.svg`}
                    alt={`${piece.color} ${piece.type}`}
                    style={{
                      width: '74%',
                      height: '74%',
                      objectFit: 'contain',
                      filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.25))'
                    }}
                  />
                ) : null}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr', gap: 12 }}>
      <span style={{ color: 'rgba(244,239,230,0.6)' }}>{label}</span>
      <span style={{ fontFamily: mono ? 'Consolas, monospace' : 'inherit', wordBreak: 'break-word' }}>{value}</span>
    </div>
  );
}

const panelStyle: React.CSSProperties = {
  borderRadius: 18,
  border: '1px solid rgba(255,255,255,0.1)',
  background: 'rgba(255,255,255,0.04)',
  boxShadow: '0 18px 60px rgba(0,0,0,0.2)',
  padding: 20
};

const headingStyle: React.CSSProperties = {
  margin: '0 0 16px',
  fontSize: 18
};

const preStyle: React.CSSProperties = {
  margin: 0,
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-word',
  fontSize: 13,
  lineHeight: 1.45,
  fontFamily: 'Consolas, monospace',
  color: '#d9e7ff'
};

const emptyStyle: React.CSSProperties = {
  margin: 0,
  color: 'rgba(244,239,230,0.62)'
};

const primaryButton: React.CSSProperties = {
  border: 'none',
  borderRadius: 10,
  padding: '12px 16px',
  background: 'linear-gradient(135deg, #1d73d4 0%, #1454a0 100%)',
  color: '#fff',
  fontWeight: 700,
  cursor: 'pointer'
};

const secondaryButton: React.CSSProperties = {
  border: '1px solid rgba(255,255,255,0.12)',
  borderRadius: 10,
  padding: '12px 16px',
  background: 'rgba(255,255,255,0.06)',
  color: '#f4efe6',
  fontWeight: 600,
  cursor: 'pointer'
};

const dangerButton: React.CSSProperties = {
  ...secondaryButton,
  border: '1px solid rgba(220,90,90,0.35)',
  background: 'rgba(220,90,90,0.12)',
  color: '#ffd3d3'
};
