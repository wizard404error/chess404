'use client';

import React from 'react';
import { type MatchModeId, type PieceColor } from '@chess404/contracts';
import { createPrivateMatch, type PrivateMatchIdentity } from './lib/private-match-service';
import { writeStoredRoomMeta } from './lib/match-service';

interface ComputerPageProps {
  identity: PrivateMatchIdentity | null;
  embedded?: boolean;
}

const DIFFICULTIES = [
  { value: 'beginner', label: 'Beginner', description: 'Learns basics, makes occasional mistakes' },
  { value: 'easy', label: 'Easy', description: 'Solid fundamentals, predictable patterns' },
  { value: 'medium', label: 'Medium', description: 'Good tactical awareness, moderate depth' },
  { value: 'hard', label: 'Hard', description: 'Strong player, deep calculation' },
  { value: 'expert', label: 'Expert', description: 'Near-optimal play, full engine depth' },
] as const;

export default function ComputerPage({ identity, embedded = false }: ComputerPageProps): React.ReactElement {
  const [difficulty, setDifficulty] = React.useState('medium');
  const [creating, setCreating] = React.useState(false);
  const [error, setError] = React.useState('');
  const [created, setCreated] = React.useState<{ matchId: string } | null>(null);

  const handleCreate = React.useCallback(async () => {
    if (!identity?.guestId) {
      setError('Your hosted player session is still loading.');
      return;
    }
    setCreating(true);
    setError('');
    try {
      const result = await createPrivateMatch({
        identity,
        queue: 'direct',
        modeId: 'computer' as MatchModeId,
        difficulty,
        clockSeconds: 600,
        preferredSeat: 'white',
      });
      writeStoredRoomMeta(result.matchId, {
        queue: 'direct',
        modeId: 'computer' as MatchModeId,
        clockSeconds: 600,
        viewerSeat: result.seatColor,
        whiteGuestId: result.snapshot.match.whiteGuestId,
        blackGuestId: result.snapshot.match.blackGuestId,
        whiteAccountId: result.snapshot.match.whiteAccountId,
        blackAccountId: result.snapshot.match.blackAccountId,
        whiteName: result.snapshot.match.whiteName,
        blackName: result.snapshot.match.blackName,
        whitePlayerSecret: result.seatColor === 'white' ? result.claim?.playerSecret : undefined,
        blackPlayerSecret: result.seatColor === 'black' ? result.claim?.playerSecret : undefined,
      });
      setCreated({ matchId: result.matchId });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create computer match.');
    } finally {
      setCreating(false);
    }
  }, [identity, difficulty]);

  const openMatch = React.useCallback(() => {
    if (!created?.matchId) return;
    window.location.href = `/match/${encodeURIComponent(created.matchId)}`;
  }, [created?.matchId]);

  if (created) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
        <div style={{
          padding: '14px',
          borderRadius: '12px',
          background: 'rgba(180,130,255,0.08)',
          border: '1px solid rgba(180,130,255,0.2)',
        }}>
          <div style={{ color: '#d4a0ff', fontSize: '14px', fontWeight: 800 }}>
            Match Ready
          </div>
          <div style={{ color: 'rgba(220,210,255,0.8)', fontSize: '13px', marginTop: '6px' }}>
            Your game against the computer is starting. You play as White.
          </div>
        </div>
        <button
          onClick={openMatch}
          style={{
            padding: '12px 16px',
            borderRadius: '10px',
            border: '1px solid rgba(180,130,255,0.36)',
            background: 'linear-gradient(180deg, rgba(130,80,210,0.95) 0%, rgba(70,40,130,0.98) 100%)',
            color: '#f7fbff',
            fontWeight: 800,
            fontSize: '13px',
            cursor: 'pointer',
          }}
        >
          Start Playing
        </button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '8px' }}>
        {DIFFICULTIES.map((d) => (
          <button
            key={d.value}
            onClick={() => setDifficulty(d.value)}
            style={{
              padding: '10px 12px',
              borderRadius: '10px',
              border: difficulty === d.value
                ? '1px solid rgba(180,130,255,0.4)'
                : '1px solid rgba(255,255,255,0.08)',
              background: difficulty === d.value
                ? 'rgba(180,130,255,0.12)'
                : 'rgba(255,255,255,0.03)',
              color: difficulty === d.value ? '#e0d0ff' : 'rgba(210,200,230,0.75)',
              cursor: 'pointer',
              textAlign: 'left',
            }}
          >
            <div style={{ fontSize: '13px', fontWeight: 800 }}>{d.label}</div>
            <div style={{ fontSize: '11px', marginTop: '4px', opacity: 0.7 }}>{d.description}</div>
          </button>
        ))}
      </div>

      {error && (
        <div style={{
          padding: '10px 12px',
          borderRadius: '8px',
          background: 'rgba(255,80,80,0.1)',
          border: '1px solid rgba(255,80,80,0.2)',
          color: '#ff8888',
          fontSize: '12px',
        }}>
          {error}
        </div>
      )}

      <button
        onClick={handleCreate}
        disabled={creating}
        style={{
          padding: '12px 16px',
          borderRadius: '10px',
          border: '1px solid rgba(180,130,255,0.36)',
          background: creating
            ? 'rgba(180,130,255,0.3)'
            : 'linear-gradient(180deg, rgba(130,80,210,0.95) 0%, rgba(70,40,130,0.98) 100%)',
          color: '#f7fbff',
          fontWeight: 800,
          fontSize: '13px',
          cursor: creating ? 'wait' : 'pointer',
          opacity: creating ? 0.7 : 1,
        }}
      >
        {creating ? 'Creating match...' : `Play vs ${DIFFICULTIES.find(d => d.value === difficulty)?.label ?? 'Computer'}`}
      </button>
    </div>
  );
}
