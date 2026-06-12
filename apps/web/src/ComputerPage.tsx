'use client';

import React from 'react';
import { type MatchModeId, type PieceColor } from '@chess404/contracts';
import { createPrivateMatch, type PrivateMatchIdentity } from './lib/private-match-service';
import { writeStoredRoomMeta } from './lib/match-service';

interface ComputerPageProps {
  identity: PrivateMatchIdentity | null;
  embedded?: boolean;
}

type DifficultyValue = 'beginner' | 'easy' | 'medium' | 'hard' | 'expert';

const DIFFICULTIES: ReadonlyArray<{
  value: DifficultyValue;
  label: string;
  description: string;
}> = [
  { value: 'beginner', label: 'Beginner', description: 'Learns basics, makes occasional mistakes' },
  { value: 'easy', label: 'Easy', description: 'Solid fundamentals, predictable patterns' },
  { value: 'medium', label: 'Medium', description: 'Good tactical awareness, moderate depth' },
  { value: 'hard', label: 'Hard', description: 'Strong player, deep calculation' },
  { value: 'expert', label: 'Expert', description: 'Near-optimal play, full engine depth' },
];

const DEFAULT_DIFFICULTY: DifficultyValue = 'medium';

export default function ComputerPage({ identity, embedded = false }: ComputerPageProps): React.ReactElement {
  const [selectedDifficulty, setSelectedDifficulty] = React.useState<DifficultyValue>(DEFAULT_DIFFICULTY);
  const [inflightDifficulty, setInflightDifficulty] = React.useState<DifficultyValue | null>(null);
  const [error, setError] = React.useState('');
  const [created, setCreated] = React.useState<{ matchId: string } | null>(null);

  // createMatch with the given difficulty. Designed to be called
  // directly from a difficulty-button click. Returns true on
  // success (caller can navigate to the match), false on failure
  // (caller does nothing, error is shown in the UI).
  const createMatch = React.useCallback(async (difficulty: DifficultyValue): Promise<boolean> => {
    if (!identity?.guestId) {
      setError('Your hosted player session is still loading — try again in a moment.')
      return false
    }
    if (inflightDifficulty !== null) {
      // Already creating; ignore the second click.
      return false
    }
    setInflightDifficulty(difficulty)
    setError('')
    try {
      const result = await createPrivateMatch({
        identity,
        queue: 'direct',
        modeId: 'computer' as MatchModeId,
        difficulty,
        clockSeconds: 600,
        preferredSeat: 'white',
      })
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
      })
      setCreated({ matchId: result.matchId })
      return true
    } catch (err) {
      const raw = err instanceof Error ? err.message : 'Failed to create computer match.'
      // Translate the gateway's upstream error messages into
      // user-friendly forms. The gateway's bootstrap path can
      // return "unauthorized guest session" (401) or "unknown
      // guest" (404) when the localStorage session is missing or
      // expired server-side. The user should refresh, not retry.
      const lower = raw.toLowerCase()
      if (lower.includes('unauthorized guest') || lower.includes('unknown guest') || lower.includes('unauthorized')) {
        setError('Your hosted player session expired. Please refresh the page to start a new game.')
      } else if (lower.includes('rate limit') || lower.includes('retry after')) {
        setError('Too many recent requests — wait a few seconds and try again.')
      } else if (lower.includes('context deadline') || lower.includes('match-service unreachable')) {
        setError('The match service is taking too long to respond. Try again in a moment.')
      } else {
        setError(raw)
      }
      return false
    } finally {
      setInflightDifficulty(null)
    }
  }, [identity, inflightDifficulty])

  // One-click: the difficulty button immediately starts the match.
  // The visual selected state (border highlight) follows the
  // selectedDifficulty state, which the in-flight match carries.
  const handleDifficultyClick = React.useCallback((difficulty: DifficultyValue) => {
    setSelectedDifficulty(difficulty)
    void createMatch(difficulty)
  }, [createMatch])

  const openMatch = React.useCallback(() => {
    if (!created?.matchId) return
    window.location.href = `/match/${encodeURIComponent(created.matchId)}`
  }, [created?.matchId])

  if (created) {
    const matchedDifficulty = DIFFICULTIES.find((d) => d.value === selectedDifficulty)
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
            Your {matchedDifficulty?.label ?? 'Medium'} game against the computer is starting. You play as White.
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
        <button
          onClick={() => { setCreated(null); setError('') }}
          style={{
            padding: '10px 14px',
            borderRadius: '10px',
            border: '1px solid rgba(255,255,255,0.08)',
            background: 'transparent',
            color: 'rgba(220,210,255,0.7)',
            fontSize: '12px',
            fontWeight: 700,
            cursor: 'pointer',
          }}
        >
          Pick a different difficulty
        </button>
      </div>
    )
  }

  const creating = inflightDifficulty !== null

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '8px' }}>
        {DIFFICULTIES.map((d) => {
          const isInflight = creating && inflightDifficulty === d.value
          const isSelected = !creating && selectedDifficulty === d.value
          return (
            <button
              key={d.value}
              onClick={() => handleDifficultyClick(d.value)}
              disabled={creating}
              style={{
                padding: '10px 12px',
                borderRadius: '10px',
                border: isInflight
                  ? '1px solid rgba(180,130,255,0.6)'
                  : isSelected
                    ? '1px solid rgba(180,130,255,0.4)'
                    : '1px solid rgba(255,255,255,0.08)',
                background: isInflight
                  ? 'rgba(180,130,255,0.22)'
                  : isSelected
                    ? 'rgba(180,130,255,0.12)'
                    : 'rgba(255,255,255,0.03)',
                color: isInflight || isSelected ? '#e0d0ff' : 'rgba(210,200,230,0.75)',
                cursor: creating && !isInflight ? 'wait' : 'pointer',
                textAlign: 'left',
                opacity: creating && !isInflight ? 0.5 : 1,
              }}
            >
              <div style={{ fontSize: '13px', fontWeight: 800, display: 'flex', alignItems: 'center', gap: '6px' }}>
                {isInflight && (
                  <span style={{
                    display: 'inline-block',
                    width: '8px',
                    height: '8px',
                    borderRadius: '50%',
                    border: '1.5px solid #d4a0ff',
                    borderTopColor: 'transparent',
                    animation: 'chess404-spin 0.7s linear infinite',
                  }} />
                )}
                {d.label}
              </div>
              <div style={{ fontSize: '11px', marginTop: '4px', opacity: 0.7 }}>{d.description}</div>
            </button>
          )
        })}
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
          <div style={{ marginTop: '6px', opacity: 0.7, fontSize: '11px' }}>
            Pick a difficulty above to try again.
          </div>
        </div>
      )}

      {creating && (
        <div style={{ fontSize: '11px', color: 'rgba(220,210,255,0.6)', textAlign: 'center' }}>
          Creating match against the {DIFFICULTIES.find((d) => d.value === inflightDifficulty)?.label.toLowerCase()} engine…
        </div>
      )}
    </div>
  )
}
