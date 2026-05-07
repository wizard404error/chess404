import React from 'react';
import type { MatchArchiveEntry } from './lib/platform-service';
import { fetchArchivedMatch, fetchArchivedMatches, fetchGuestArchivedMatches } from './lib/platform-service';

const FILE_LABELS = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function resultLabel(entry: MatchArchiveEntry): string {
  if (entry.status !== 'finished') {
    return entry.status;
  }
  if (!entry.winner) {
    return 'finished';
  }
  return entry.winner === 'draw' ? 'draw' : `${entry.winner} won`;
}

function playerIdentityLabel(
  name?: string,
  guestId?: string,
  accountHandle?: string,
  fallback = 'Guest',
): string {
  const base = name ?? guestId ?? fallback;
  return accountHandle ? `${base} (@${accountHandle})` : base;
}

function playersLabel(entry: MatchArchiveEntry): string {
  const white = playerIdentityLabel(entry.whiteName, entry.whiteGuestId, entry.whiteAccountHandle, 'White guest');
  const black = playerIdentityLabel(entry.blackName, entry.blackGuestId, entry.blackAccountHandle, 'Black guest');
  return `${white} vs ${black}`;
}

function queueLabel(entry: MatchArchiveEntry): string {
  if (entry.queue === 'rated') {
    return 'rated';
  }
  if (entry.queue === 'casual') {
    return 'casual';
  }
  return 'direct';
}

function formatClock(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function listCardNames(cards: MatchArchiveEntry['snapshot']['match']['whiteHand']): string {
  if (cards.length === 0) {
    return 'none';
  }
  return cards.map(card => card.name).join(', ');
}

function collectActiveEffects(match: MatchArchiveEntry['snapshot']['match']): string[] {
  const effects: string[] = [];
  if (match.pendingCard) effects.push(`pending ${match.pendingCard.mechanic}`);
  if (match.doubleMove) effects.push(`double move (${match.doubleMove.type})`);
  if (match.undoAgainst) effects.push(`undo armed vs ${match.undoAgainst}`);
  if (match.radarRevealFor) effects.push(`radar for ${match.radarRevealFor}`);
  if (match.cheaterState) effects.push(`cheater ${match.cheaterState.ownerColor} (${match.cheaterState.turnsLeft})`);
  if (match.invisiblePiece) effects.push(`invisible ${match.invisiblePiece.piece.type}`);
  if (match.lavaSquares?.length) effects.push(`lava ${match.lavaSquares.length}`);
  if (match.bombPieces?.length) effects.push(`bombs ${match.bombPieces.length}`);
  if (match.blackHoles?.length) effects.push(`black holes ${match.blackHoles.length}`);
  if (match.fogZones?.length) effects.push(`fog ${match.fogZones.length}`);
  if (match.fortressZones?.length) effects.push(`fortress ${match.fortressZones.length}`);
  return effects;
}

function replayButtonStyle(disabled: boolean): React.CSSProperties {
  return {
    padding: '6px 10px',
    borderRadius: '8px',
    border: disabled ? '1px solid rgba(255,255,255,0.08)' : '1px solid rgba(255,180,60,0.22)',
    background: disabled
      ? 'rgba(255,255,255,0.03)'
      : 'linear-gradient(180deg, rgba(200,134,10,0.18) 0%, rgba(70,42,8,0.3) 100%)',
    color: disabled ? 'rgba(255,232,180,0.42)' : '#fff2c8',
    fontSize: '11px',
    fontWeight: 800,
    cursor: disabled ? 'not-allowed' : 'pointer',
  };
}

function renderBoardPreview(board: MatchArchiveEntry['snapshot']['match']['board']): React.ReactElement {
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
              {piece && (
                <img
                  src={src ?? ''}
                  alt={`${piece.color} ${piece.type}`}
                  style={{ width: '28px', height: '28px', objectFit: 'contain', pointerEvents: 'none' }}
                />
              )}
              {colIndex === 0 && (
                <span style={{ position: 'absolute', top: '2px', left: '3px', fontSize: '8px', fontWeight: 700, color: dark ? 'rgba(255,245,220,0.85)' : 'rgba(120,72,22,0.82)' }}>
                  {8 - rowIndex}
                </span>
              )}
              {rowIndex === 7 && (
                <span style={{ position: 'absolute', right: '3px', bottom: '2px', fontSize: '8px', fontWeight: 700, color: dark ? 'rgba(255,245,220,0.85)' : 'rgba(120,72,22,0.82)' }}>
                  {FILE_LABELS[colIndex]}
                </span>
              )}
            </div>
          );
        })
      )}
    </div>
  );
}

interface HistoryPageProps {
  focusMatchId?: string | null;
  focusGuestId?: string | null;
  onOpenGuest?: (guestId: string) => void;
  onClearGuestFocus?: () => void;
}

export default function HistoryPage({
  focusMatchId = null,
  focusGuestId = null,
  onOpenGuest,
  onClearGuestFocus,
}: HistoryPageProps): React.ReactElement {
  const [matches, setMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [selectedMatchId, setSelectedMatchId] = React.useState<string | null>(null);
  const [selectedMatch, setSelectedMatch] = React.useState<MatchArchiveEntry | null>(null);
  const [selectedReplayIndex, setSelectedReplayIndex] = React.useState(0);
  const [loadingList, setLoadingList] = React.useState(true);
  const [loadingDetail, setLoadingDetail] = React.useState(false);
  const [error, setError] = React.useState('');

  const loadMatches = React.useCallback(async (preserveSelection = true) => {
    setLoadingList(true);
    setError('');
    try {
      const nextMatches = focusGuestId
        ? await fetchGuestArchivedMatches(focusGuestId, 40)
        : await fetchArchivedMatches(40);
      setMatches(nextMatches);
      setSelectedMatchId(currentSelected => {
        if (preserveSelection && currentSelected && nextMatches.some(match => match.matchId === currentSelected)) {
          return currentSelected;
        }
        return nextMatches[0]?.matchId ?? null;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load match history.');
    } finally {
      setLoadingList(false);
    }
  }, [focusGuestId]);

  React.useEffect(() => {
    void loadMatches(false);
  }, [loadMatches]);

  React.useEffect(() => {
    if (focusMatchId) {
      setSelectedMatchId(focusMatchId);
    }
  }, [focusMatchId]);

  React.useEffect(() => {
    if (!selectedMatchId) {
      setSelectedMatch(null);
      return;
    }

    let cancelled = false;
    setLoadingDetail(true);
    setError('');

    void fetchArchivedMatch(selectedMatchId)
      .then(match => {
        if (!cancelled) {
          setSelectedMatch(match);
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load match details.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDetail(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedMatchId]);

  React.useEffect(() => {
    const replayFrames = selectedMatch?.snapshot.replayFrames ?? [];
    setSelectedReplayIndex(replayFrames.length > 0 ? replayFrames.length - 1 : 0);
  }, [selectedMatch]);

  const snapshot = selectedMatch?.snapshot.match;
  const replayFrames = selectedMatch?.snapshot.replayFrames ?? [];
  const activeReplayFrame = replayFrames[selectedReplayIndex] ?? null;
  const previewBoard = activeReplayFrame?.board ?? snapshot?.board ?? [];
  const replayLastMove = activeReplayFrame && activeReplayFrame.moveHistory.length > 0
    ? activeReplayFrame.moveHistory[activeReplayFrame.moveHistory.length - 1]
    : null;
  const recentEvents = selectedMatch?.snapshot.events ?? [];
  const activeEffects = snapshot ? collectActiveEffects(snapshot) : [];

  return (
    <div style={{ display: 'flex', flex: 1, minHeight: 0, padding: '22px 28px 26px', gap: '18px' }}>
      <div
        style={{
          width: '360px',
          flexShrink: 0,
          display: 'flex',
          flexDirection: 'column',
          minHeight: 0,
          background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
          border: '1px solid rgba(255,165,40,0.16)',
          borderRadius: '14px',
          boxShadow: '0 12px 40px rgba(0,0,0,0.35)',
          overflow: 'hidden',
        }}
      >
        <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px' }}>
            <div>
              <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Match History</div>
              <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
                {focusGuestId ? `Archived matches for ${focusGuestId}.` : 'Archived matches from the local platform store.'}
              </div>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              {focusGuestId && onClearGuestFocus && (
                <button
                  onClick={onClearGuestFocus}
                  style={{
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid rgba(255,255,255,0.12)',
                    background: 'rgba(255,255,255,0.04)',
                    color: 'rgba(255,232,180,0.85)',
                    fontSize: '12px',
                    fontWeight: 700,
                    cursor: 'pointer',
                  }}
                >
                  All Matches
                </button>
              )}
              <button
                onClick={() => void loadMatches(true)}
                style={{
                  padding: '8px 12px',
                  borderRadius: '8px',
                  border: '1px solid rgba(255,180,60,0.35)',
                  background: 'linear-gradient(180deg, rgba(200,134,10,0.32) 0%, rgba(122,79,8,0.4) 100%)',
                  color: '#fff2c8',
                  fontSize: '12px',
                  fontWeight: 700,
                  cursor: 'pointer',
                }}
              >
                Refresh
              </button>
            </div>
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '10px' }}>
          {loadingList ? (
            <div style={{ padding: '16px', color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading archived matches...</div>
          ) : matches.length === 0 ? (
            <div style={{ padding: '16px', color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>No archived matches yet. Start a game and make a move to populate history.</div>
          ) : (
            matches.map(match => {
              const selected = match.matchId === selectedMatchId;
              return (
                <button
                  key={match.matchId}
                  onClick={() => setSelectedMatchId(match.matchId)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    marginBottom: '10px',
                    padding: '14px 14px 12px',
                    borderRadius: '12px',
                    border: selected ? '1px solid rgba(255,180,60,0.58)' : '1px solid rgba(255,165,40,0.12)',
                    background: selected
                      ? 'linear-gradient(180deg, rgba(200,134,10,0.22) 0%, rgba(70,42,8,0.34) 100%)'
                      : 'linear-gradient(180deg, rgba(18,23,36,0.95) 0%, rgba(11,14,24,0.94) 100%)',
                    color: '#fff4d0',
                    cursor: 'pointer',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
                    <div style={{ fontWeight: 800, fontSize: '12px', color: '#ffcf72' }}>{match.matchId}</div>
                    <div
                      style={{
                        padding: '3px 8px',
                        borderRadius: '999px',
                        fontSize: '10px',
                        fontWeight: 800,
                        textTransform: 'uppercase',
                        color: match.status === 'finished' ? '#ffd28a' : '#9ee6b8',
                        background: match.status === 'finished' ? 'rgba(168,110,22,0.22)' : 'rgba(24,120,62,0.22)',
                        border: match.status === 'finished' ? '1px solid rgba(255,180,60,0.28)' : '1px solid rgba(78,210,132,0.28)',
                      }}
                    >
                      {resultLabel(match)}
                    </div>
                  </div>
                  <div style={{ marginTop: '8px', fontSize: '11px', color: 'rgba(255,232,180,0.7)' }}>
                    {match.moveCount} moves
                    {match.lastMove ? ` · last ${match.lastMove}` : ''}
                    {` · ${queueLabel(match)}`}
                  </div>
                  <div style={{ marginTop: '5px', fontSize: '11px', color: 'rgba(210,220,255,0.62)' }}>
                    {playersLabel(match)}
                  </div>
                  <div style={{ marginTop: '5px', fontSize: '11px', color: 'rgba(170,190,220,0.62)' }}>
                    Updated {formatDateTime(match.updatedAt)}
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>

      <div
        style={{
          flex: 1,
          minWidth: 0,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
          border: '1px solid rgba(255,165,40,0.16)',
          borderRadius: '14px',
          boxShadow: '0 12px 40px rgba(0,0,0,0.35)',
          overflow: 'hidden',
        }}
      >
        <div style={{ padding: '18px 20px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Match Detail</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            Snapshot-backed detail from the platform archive. This is the first persistence layer before Postgres and Redis.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {error && (
            <div
              style={{
                marginBottom: '16px',
                padding: '12px 14px',
                borderRadius: '10px',
                background: 'rgba(120,20,20,0.22)',
                border: '1px solid rgba(231,76,60,0.32)',
                color: '#ffb1a7',
                fontSize: '12px',
                fontWeight: 700,
              }}
            >
              {error}
            </div>
          )}

          {loadingDetail ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading match detail...</div>
          ) : !selectedMatch || !snapshot ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Pick a match from the left to inspect its archived state.</div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1.35fr) minmax(280px, 0.85fr)', gap: '18px' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', minWidth: 0 }}>
                <div
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    background: 'rgba(0,0,0,0.18)',
                    border: '1px solid rgba(255,165,40,0.1)',
                  }}
                >
                  <div style={{ fontSize: '15px', fontWeight: 800, color: '#fff2c8' }}>{selectedMatch.matchId}</div>
                  <div style={{ marginTop: '6px', fontSize: '12px', color: 'rgba(210,220,255,0.72)' }}>{playersLabel(selectedMatch)}</div>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '10px' }}>
                    {selectedMatch.whiteGuestId && (
                      <button
                        onClick={() => onOpenGuest?.(selectedMatch.whiteGuestId!)}
                        style={{
                          padding: '5px 9px',
                          borderRadius: '999px',
                          background: 'rgba(80,140,255,0.16)',
                          border: '1px solid rgba(110,170,255,0.22)',
                          color: '#dcecff',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: onOpenGuest ? 'pointer' : 'default',
                        }}
                      >
                        {playerIdentityLabel(selectedMatch.whiteName, selectedMatch.whiteGuestId, selectedMatch.whiteAccountHandle, 'White guest')}
                      </button>
                    )}
                    {selectedMatch.blackGuestId && (
                      <button
                        onClick={() => onOpenGuest?.(selectedMatch.blackGuestId!)}
                        style={{
                          padding: '5px 9px',
                          borderRadius: '999px',
                          background: 'rgba(170,100,255,0.14)',
                          border: '1px solid rgba(190,130,255,0.22)',
                          color: '#eadbff',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: onOpenGuest ? 'pointer' : 'default',
                        }}
                      >
                        {playerIdentityLabel(selectedMatch.blackName, selectedMatch.blackGuestId, selectedMatch.blackAccountHandle, 'Black guest')}
                      </button>
                    )}
                  </div>
                  <div style={{ marginTop: '8px', display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '10px 14px', fontSize: '12px' }}>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Status:</span> <span style={{ color: '#fff4d0' }}>{snapshot.status}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Winner:</span> <span style={{ color: '#fff4d0' }}>{snapshot.winner ?? 'none'}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Queue:</span> <span style={{ color: '#fff4d0' }}>{queueLabel(selectedMatch)}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Turn:</span> <span style={{ color: '#fff4d0' }}>{activeReplayFrame?.turn ?? snapshot.turn}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>White account:</span> <span style={{ color: '#fff4d0' }}>{selectedMatch.whiteAccountHandle ? `@${selectedMatch.whiteAccountHandle}` : selectedMatch.whiteAccountId ?? 'guest-only'}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Black account:</span> <span style={{ color: '#fff4d0' }}>{selectedMatch.blackAccountHandle ? `@${selectedMatch.blackAccountHandle}` : selectedMatch.blackAccountId ?? 'guest-only'}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Moves:</span> <span style={{ color: '#fff4d0' }}>{selectedMatch.moveCount}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Created:</span> <span style={{ color: '#fff4d0' }}>{formatDateTime(selectedMatch.createdAt)}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Updated:</span> <span style={{ color: '#fff4d0' }}>{formatDateTime(selectedMatch.updatedAt)}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Last move:</span> <span style={{ color: '#fff4d0' }}>{replayLastMove ?? selectedMatch.lastMove ?? 'none'}</span></div>
                    <div><span style={{ color: 'rgba(255,232,180,0.58)' }}>Rules:</span> <span style={{ color: '#fff4d0' }}>{snapshot.rulesVersion}</span></div>
                  </div>
                  <div style={{ marginTop: '16px', display: 'flex', justifyContent: 'center' }}>
                    {renderBoardPreview(previewBoard)}
                  </div>
                  <div style={{ marginTop: '14px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
                      <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px' }}>
                        Replay frame {replayFrames.length === 0 ? 0 : selectedReplayIndex} of {Math.max(replayFrames.length - 1, 0)}
                        {activeReplayFrame ? ` · ${activeReplayFrame.moveHistory.length} moves recorded` : ''}
                      </div>
                      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                        <button
                          onClick={() => setSelectedReplayIndex(0)}
                          disabled={replayFrames.length === 0 || selectedReplayIndex === 0}
                          style={replayButtonStyle(replayFrames.length === 0 || selectedReplayIndex === 0)}
                        >
                          Start
                        </button>
                        <button
                          onClick={() => setSelectedReplayIndex(current => Math.max(0, current - 1))}
                          disabled={replayFrames.length === 0 || selectedReplayIndex === 0}
                          style={replayButtonStyle(replayFrames.length === 0 || selectedReplayIndex === 0)}
                        >
                          Prev
                        </button>
                        <button
                          onClick={() => setSelectedReplayIndex(current => Math.min(replayFrames.length - 1, current + 1))}
                          disabled={replayFrames.length === 0 || selectedReplayIndex >= replayFrames.length - 1}
                          style={replayButtonStyle(replayFrames.length === 0 || selectedReplayIndex >= replayFrames.length - 1)}
                        >
                          Next
                        </button>
                        <button
                          onClick={() => setSelectedReplayIndex(Math.max(replayFrames.length - 1, 0))}
                          disabled={replayFrames.length === 0 || selectedReplayIndex >= replayFrames.length - 1}
                          style={replayButtonStyle(replayFrames.length === 0 || selectedReplayIndex >= replayFrames.length - 1)}
                        >
                          Live
                        </button>
                      </div>
                    </div>
                    {replayFrames.length > 0 && (
                      <input
                        type="range"
                        min={0}
                        max={Math.max(replayFrames.length - 1, 0)}
                        step={1}
                        value={Math.min(selectedReplayIndex, Math.max(replayFrames.length - 1, 0))}
                        onChange={event => setSelectedReplayIndex(Number(event.target.value))}
                        style={{ width: '100%' }}
                      />
                    )}
                  </div>
                </div>

                <div
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    background: 'rgba(0,0,0,0.18)',
                    border: '1px solid rgba(255,165,40,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', fontWeight: 800, color: '#ffcf72', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>State Summary</div>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '10px 16px', fontSize: '12px' }}>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '3px' }}>White clock</div>
                      <div style={{ color: '#fff4d0', fontWeight: 700 }}>{formatClock(snapshot.clock.whiteMs)}</div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '3px' }}>Black clock</div>
                      <div style={{ color: '#fff4d0', fontWeight: 700 }}>{formatClock(snapshot.clock.blackMs)}</div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '3px' }}>White hand</div>
                      <div style={{ color: '#fff4d0', lineHeight: 1.45 }}>{listCardNames(snapshot.whiteHand)}</div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '3px' }}>Black hand</div>
                      <div style={{ color: '#fff4d0', lineHeight: 1.45 }}>{listCardNames(snapshot.blackHand)}</div>
                    </div>
                  </div>
                  <div style={{ marginTop: '14px' }}>
                    <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '6px', fontSize: '12px' }}>Active effects</div>
                    {activeEffects.length === 0 ? (
                      <div style={{ color: 'rgba(255,232,180,0.5)', fontSize: '12px' }}>No active timed or pending effects in this snapshot.</div>
                    ) : (
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                        {activeEffects.map(effect => (
                          <span
                            key={effect}
                            style={{
                              padding: '4px 8px',
                              borderRadius: '999px',
                              background: 'rgba(200,134,10,0.16)',
                              border: '1px solid rgba(255,180,60,0.2)',
                              color: '#ffe6b2',
                              fontSize: '11px',
                              fontWeight: 700,
                            }}
                          >
                            {effect}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                <div
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    background: 'rgba(0,0,0,0.18)',
                    border: '1px solid rgba(255,165,40,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', fontWeight: 800, color: '#ffcf72', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>Replay Head and Events</div>
                  <div style={{ fontSize: '12px', color: 'rgba(255,232,180,0.75)', marginBottom: '10px' }}>Replay head: {selectedMatch.snapshot.replayHead}</div>
                  {recentEvents.length === 0 ? (
                    <div style={{ fontSize: '12px', color: 'rgba(255,232,180,0.58)' }}>No stored events on this snapshot yet.</div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {recentEvents.slice(-6).reverse().map(event => (
                        <div
                          key={event.id}
                          style={{
                            padding: '10px 12px',
                            borderRadius: '10px',
                            background: 'rgba(255,255,255,0.03)',
                            border: '1px solid rgba(255,165,40,0.08)',
                          }}
                        >
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
                            <div style={{ color: '#fff2c8', fontWeight: 700, fontSize: '12px' }}>{event.type}</div>
                            <div style={{ color: 'rgba(170,190,220,0.62)', fontSize: '11px' }}>{formatDateTime(event.at)}</div>
                          </div>
                          {event.actorId && <div style={{ marginTop: '4px', color: 'rgba(255,232,180,0.65)', fontSize: '11px' }}>Actor: {event.actorId}</div>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div
                  style={{
                    padding: '16px',
                    borderRadius: '12px',
                    background: 'rgba(0,0,0,0.18)',
                    border: '1px solid rgba(255,165,40,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', fontWeight: 800, color: '#ffcf72', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>Move History</div>
                  {snapshot.moveHistory.length === 0 ? (
                    <div style={{ fontSize: '12px', color: 'rgba(255,232,180,0.58)' }}>No moves recorded yet.</div>
                  ) : (
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '8px' }}>
                      {snapshot.moveHistory.map((move, index) => (
                        <button
                          key={`${index}-${move}`}
                          onClick={() => setSelectedReplayIndex(Math.min(index + 1, Math.max(replayFrames.length - 1, 0)))}
                          style={{
                            padding: '8px 10px',
                            borderRadius: '9px',
                            background: selectedReplayIndex === index + 1
                              ? 'linear-gradient(180deg, rgba(200,134,10,0.22) 0%, rgba(70,42,8,0.34) 100%)'
                              : 'rgba(255,255,255,0.03)',
                            border: selectedReplayIndex === index + 1
                              ? '1px solid rgba(255,180,60,0.35)'
                              : '1px solid rgba(255,165,40,0.08)',
                            color: '#fff2c8',
                            fontSize: '12px',
                            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                            cursor: replayFrames.length > 0 ? 'pointer' : 'default',
                            textAlign: 'left',
                          }}
                        >
                          {index + 1}. {move}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              <div
                style={{
                  minWidth: 0,
                  padding: '16px',
                  borderRadius: '12px',
                  background: 'rgba(0,0,0,0.18)',
                  border: '1px solid rgba(255,165,40,0.1)',
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                <div style={{ fontSize: '12px', fontWeight: 800, color: '#ffcf72', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>Chat & Snapshot JSON</div>
                <div style={{ marginBottom: '14px', paddingBottom: '14px', borderBottom: '1px solid rgba(255,165,40,0.08)' }}>
                  <div style={{ color: 'rgba(255,232,180,0.58)', marginBottom: '8px', fontSize: '12px' }}>Chat log</div>
                  {snapshot.chatMessages.length === 0 ? (
                    <div style={{ fontSize: '12px', color: 'rgba(255,232,180,0.5)' }}>No chat messages archived for this match.</div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '180px', overflowY: 'auto', paddingRight: '4px' }}>
                      {snapshot.chatMessages.map((message, index) => (
                        <div
                          key={`${message.sentAt}-${index}`}
                          style={{
                            padding: '8px 10px',
                            borderRadius: '9px',
                            background: 'rgba(255,255,255,0.03)',
                            border: '1px solid rgba(255,165,40,0.08)',
                          }}
                        >
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px', marginBottom: '4px' }}>
                            <span style={{ color: message.sender === 'white' ? '#ffe6a3' : '#cdb7ff', fontSize: '11px', fontWeight: 800 }}>{message.sender}</span>
                            <span style={{ color: 'rgba(170,190,220,0.62)', fontSize: '10px' }}>{formatDateTime(message.sentAt)}</span>
                          </div>
                          <div style={{ color: '#fff4d0', fontSize: '12px', lineHeight: 1.4 }}>{message.text}</div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                <pre
                  style={{
                    margin: 0,
                    flex: 1,
                    minHeight: '260px',
                    overflow: 'auto',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                    color: '#d9e9ff',
                    fontSize: '11px',
                    lineHeight: 1.45,
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                  }}
                >
                  {JSON.stringify(selectedMatch, null, 2)}
                </pre>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
