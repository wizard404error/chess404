import React from 'react';
import { OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import { fetchArchivedMatches, type MatchArchiveEntry, type PublicMatchStatusFilter } from './lib/platform-service';
import { formatDateTime, formatMatchPlayers, formatMatchResult, formatModeLabel } from './lib/display';
import { buildLiveMatchUrl, buildReplayPageUrl, copyTextToClipboard } from './lib/session-storage';
import { modeLabel, queueLabel } from './lib/match-labels';

const FILE_LABELS = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

interface WatchPageProps {
  onWatchMatch?: (matchId: string) => void;
  onOpenReplay?: (matchId: string) => void;
}


function resultLabel(entry: MatchArchiveEntry): string {
  return formatMatchResult({
    status: entry.status,
    winner: entry.winner,
    finishReason: entry.finishReason,
  });
}

function renderBoardPreview(board: MatchArchiveEntry['snapshot']['match']['board']): React.ReactElement {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(8, 1fr)',
        width: '100%',
        maxWidth: '240px',
        aspectRatio: '1',
        borderRadius: '14px',
        overflow: 'hidden',
        border: '1px solid rgba(255,190,90,0.2)',
        boxShadow: '0 10px 28px rgba(0,0,0,0.22)',
      }}
    >
      {board.flatMap((row, rowIndex) =>
        row.map((piece, colIndex) => {
          const dark = (rowIndex + colIndex) % 2 === 1;
          const src = piece ? `/pieces/${piece.color}_${piece.type}.svg` : null;
          const square = `${FILE_LABELS[colIndex]}${8 - rowIndex}`;
          return (
            <div
              key={square}
              style={{
                background: dark ? '#a97853' : '#ead4ad',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                position: 'relative',
              }}
            >
              {piece ? (
                <img
                  src={src ?? ''}
                  alt={`${piece.color} ${piece.type}`}
                  style={{ width: '78%', height: '78%', objectFit: 'contain', pointerEvents: 'none' }}
                />
              ) : null}
            </div>
          );
        })
      )}
    </div>
  );
}

export default function WatchPage({ onWatchMatch, onOpenReplay }: WatchPageProps): React.ReactElement {
  const [status, setStatus] = React.useState<PublicMatchStatusFilter>('active');
  const [modeId, setModeId] = React.useState<MatchModeId | ''>('');
  const [matches, setMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setMatches(await fetchArchivedMatches(24, modeId || undefined, status));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load public match feed.');
    } finally {
      setLoading(false);
    }
  }, [modeId, status]);

  const copyLiveLink = React.useCallback(async (matchId: string) => {
    const matchUrl = buildLiveMatchUrl(matchId);
    if (!matchUrl) {
      return;
    }
    try {
      const copied = await copyTextToClipboard(matchUrl);
      setNotice(copied ? 'Live match link copied.' : matchUrl);
    } catch {
      setNotice(matchUrl);
    }
  }, []);

  const copyReplayLink = React.useCallback(async (matchId: string) => {
    const replayUrl = buildReplayPageUrl(matchId);
    if (!replayUrl) {
      return;
    }
    try {
      const copied = await copyTextToClipboard(replayUrl);
      setNotice(copied ? 'Replay link copied.' : replayUrl);
    } catch {
      setNotice(replayUrl);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  React.useEffect(() => {
    setNotice('');
  }, [modeId, status]);

  return (
    <div style={{ display: 'flex', flex: 1, minHeight: 0, padding: '22px 28px 26px', gap: '18px' }}>
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
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
            <div>
              <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Watch Chess404</div>
              <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px', maxWidth: '720px', lineHeight: 1.5 }}>
                Live spectating and replay discovery start here. Track active public games by official mode, jump into recent finished matches, and copy stable live or replay links that resolve into canonical match destinations.
              </div>
            </div>
            <button
              onClick={() => void refresh()}
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
              Refresh Feed
            </button>
          </div>

          <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', marginTop: '16px' }}>
            {(['active', 'finished'] as PublicMatchStatusFilter[]).map((value) => (
              <button
                key={value}
                onClick={() => setStatus(value)}
                style={{
                  padding: '9px 12px',
                  borderRadius: '9px',
                  border: status === value ? '1px solid rgba(255,180,60,0.3)' : '1px solid rgba(255,255,255,0.08)',
                  background: status === value
                    ? 'linear-gradient(180deg, rgba(200,134,10,0.22) 0%, rgba(70,42,8,0.34) 100%)'
                    : 'rgba(255,255,255,0.03)',
                  color: status === value ? '#fff1c7' : 'rgba(255,232,180,0.75)',
                  textTransform: 'uppercase',
                  fontSize: '11px',
                  fontWeight: 800,
                  cursor: 'pointer',
                }}
              >
                {value === 'active' ? 'Live Games' : 'Recent Replays'}
              </button>
            ))}
            <select
              value={modeId}
              onChange={(event) => setModeId(OFFICIAL_MATCH_MODES.some((mode) => mode.id === event.target.value) ? (event.target.value as MatchModeId) : '')}
              style={{
                minWidth: '180px',
                padding: '9px 12px',
                borderRadius: '9px',
                border: '1px solid rgba(255,255,255,0.10)',
                background: 'rgba(255,255,255,0.04)',
                color: '#fff2c8',
                fontSize: '12px',
                fontWeight: 700,
              }}
            >
              <option value="">All official modes</option>
              {OFFICIAL_MATCH_MODES.map((mode) => (
                <option key={mode.id} value={mode.id}>
                  {mode.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {error && (
            <div style={{
              marginBottom: '16px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(120,20,20,0.22)',
              border: '1px solid rgba(231,76,60,0.32)',
              color: '#ffb1a7',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {error}
            </div>
          )}
          {notice && (
            <div style={{
              marginBottom: '16px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(22,90,54,0.2)',
              border: '1px solid rgba(78,210,132,0.28)',
              color: '#b8f2cb',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {notice}
            </div>
          )}

          {loading ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading public match feed...</div>
          ) : matches.length === 0 ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px', lineHeight: 1.6 }}>
              {status === 'active'
                ? `No live public games are visible for ${modeId ? formatModeLabel(modeId) : 'the selected filters'} yet.`
                : `No finished public matches are available for ${modeId ? formatModeLabel(modeId) : 'the selected filters'} yet.`}
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '14px' }}>
              {matches.map((entry) => (
                <div
                  key={entry.matchId}
                  style={{
                    borderRadius: '16px',
                    border: '1px solid rgba(255,165,40,0.14)',
                    background: 'linear-gradient(180deg, rgba(24,30,44,0.96) 0%, rgba(12,16,26,0.98) 100%)',
                    boxShadow: '0 18px 50px rgba(0,0,0,0.28)',
                    padding: '16px',
                    display: 'grid',
                    gap: '12px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'flex-start' }}>
                    <div>
                      <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>
                        {formatMatchPlayers({
                          whiteName: entry.whiteName,
                          whiteHandle: entry.whiteAccountHandle,
                          blackName: entry.blackName,
                          blackHandle: entry.blackAccountHandle,
                        })}
                      </div>
                      <div style={{ color: 'rgba(255,232,180,0.7)', fontSize: '12px', marginTop: '5px' }}>
                        {queueLabel(entry.queue)} · {modeLabel(entry.modeId)}
                      </div>
                    </div>
                    <div style={{
                      padding: '5px 9px',
                      borderRadius: '999px',
                      background: entry.status === 'active' ? 'rgba(46, 204, 113, 0.14)' : 'rgba(255,180,60,0.14)',
                      border: entry.status === 'active' ? '1px solid rgba(46, 204, 113, 0.24)' : '1px solid rgba(255,180,60,0.24)',
                      color: entry.status === 'active' ? '#9ee6b8' : '#ffe4a0',
                      fontSize: '10px',
                      fontWeight: 800,
                      textTransform: 'uppercase',
                    }}>
                      {resultLabel(entry)}
                    </div>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '10px' }}>
                    <div style={{ padding: '10px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}>
                      <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Moves</div>
                      <div style={{ color: '#ffe7a9', fontSize: '18px', fontWeight: 800, marginTop: '4px' }}>{entry.moveCount}</div>
                    </div>
                    <div style={{ padding: '10px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}>
                      <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Updated</div>
                      <div style={{ color: '#e5f0ff', fontSize: '12px', fontWeight: 700, marginTop: '6px', lineHeight: 1.5 }}>{formatDateTime(entry.updatedAt)}</div>
                    </div>
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'center', padding: '4px 0 2px' }}>
                    {renderBoardPreview(entry.snapshot.match.board)}
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px' }}>
                      {entry.status === 'active' ? 'Spectate live now or share the public board destination.' : 'Replay is archived and ready to open or share.'}
                    </div>
                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                      {entry.status === 'active' ? (
                        <>
                          <button
                            onClick={() => onWatchMatch?.(entry.matchId)}
                            style={{
                              padding: '10px 14px',
                              borderRadius: '10px',
                              border: '1px solid rgba(110,170,255,0.32)',
                              background: 'linear-gradient(180deg, rgba(54,102,184,0.24) 0%, rgba(24,40,82,0.34) 100%)',
                              color: '#e5f0ff',
                              fontSize: '12px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Watch Live
                          </button>
                          <button
                            onClick={() => void copyLiveLink(entry.matchId)}
                            style={{
                              padding: '10px 14px',
                              borderRadius: '10px',
                              border: '1px solid rgba(255,255,255,0.12)',
                              background: 'rgba(255,255,255,0.04)',
                              color: '#fff0c6',
                              fontSize: '12px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Share Live
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            onClick={() => onOpenReplay?.(entry.matchId)}
                            style={{
                              padding: '10px 14px',
                              borderRadius: '10px',
                              border: '1px solid rgba(255,180,60,0.28)',
                              background: 'rgba(255,180,60,0.08)',
                              color: '#fff0c6',
                              fontSize: '12px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Open Replay
                          </button>
                          <button
                            onClick={() => void copyReplayLink(entry.matchId)}
                            style={{
                              padding: '10px 14px',
                              borderRadius: '10px',
                              border: '1px solid rgba(255,255,255,0.12)',
                              background: 'rgba(255,255,255,0.04)',
                              color: '#fff0c6',
                              fontSize: '12px',
                              fontWeight: 800,
                              cursor: 'pointer',
                            }}
                          >
                            Share Replay
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
