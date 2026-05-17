import React from 'react';
import { OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import type { QueueName, QueueSnapshot } from './lib/matchmaking-service';
import { fetchQueueSnapshots } from './lib/matchmaking-service';

interface ModesPageProps {
  onPlayMode?: (modeId: MatchModeId, queue: QueueName) => void;
}

function emptySnapshot(queue: QueueName, modeId: MatchModeId): QueueSnapshot {
  return {
    queue,
    modeId,
    queuedCount: 0,
    matchedCount: 0,
    cancelledCount: 0,
  };
}

function resolveSnapshot(snapshots: QueueSnapshot[], queue: QueueName, modeId: MatchModeId): QueueSnapshot {
  return snapshots.find((snapshot) => snapshot.queue === queue && snapshot.modeId === modeId) ?? emptySnapshot(queue, modeId);
}

export default function ModesPage({ onPlayMode }: ModesPageProps): React.ReactElement {
  const [snapshots, setSnapshots] = React.useState<QueueSnapshot[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState('');

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const payload = await fetchQueueSnapshots();
      setSnapshots(payload.snapshots);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load live queue health.');
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

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
              <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Official Modes</div>
              <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
                Curated competitive formats with live queue health, clear rules, and direct play entry into the selected lane.
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
              Refresh Live Health
            </button>
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

          {loading ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading mode health...</div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '16px' }}>
              {OFFICIAL_MATCH_MODES.map((mode) => {
                const casual = resolveSnapshot(snapshots, 'casual', mode.id);
                const rated = resolveSnapshot(snapshots, 'rated', mode.id);
                const queuedTotal = casual.queuedCount + rated.queuedCount;
                const matchedTotal = casual.matchedCount + rated.matchedCount;
                return (
                  <div
                    key={mode.id}
                    style={{
                      borderRadius: '16px',
                      border: '1px solid rgba(255,165,40,0.14)',
                      background: 'linear-gradient(180deg, rgba(24,30,44,0.96) 0%, rgba(12,16,26,0.98) 100%)',
                      boxShadow: '0 18px 50px rgba(0,0,0,0.28)',
                      padding: '18px',
                      display: 'grid',
                      gap: '14px',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
                      <div>
                        <div style={{ color: '#fff2c8', fontSize: '19px', fontWeight: 800 }}>{mode.label}</div>
                        <div style={{ color: 'rgba(255,232,180,0.68)', fontSize: '12px', marginTop: '5px', lineHeight: 1.5 }}>
                          {mode.rulesSummary}
                        </div>
                      </div>
                      <div style={{
                        minWidth: '74px',
                        padding: '8px 10px',
                        borderRadius: '12px',
                        textAlign: 'center',
                        border: '1px solid rgba(110,170,255,0.22)',
                        background: 'rgba(80,140,255,0.10)',
                      }}>
                        <div style={{ color: '#8ec5ff', fontSize: '11px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Waiting</div>
                        <div style={{ color: '#e5f0ff', fontSize: '22px', fontWeight: 900 }}>{queuedTotal}</div>
                      </div>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: '10px' }}>
                      <div style={{ padding: '10px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}>
                        <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Casual Waiting</div>
                        <div style={{ color: '#ffe7a9', fontSize: '18px', fontWeight: 800, marginTop: '4px' }}>{casual.queuedCount}</div>
                      </div>
                      <div style={{ padding: '10px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}>
                        <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Rated Waiting</div>
                        <div style={{ color: '#ffe7a9', fontSize: '18px', fontWeight: 800, marginTop: '4px' }}>{rated.queuedCount}</div>
                      </div>
                      <div style={{ padding: '10px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)' }}>
                        <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Recently Matched</div>
                        <div style={{ color: '#7ce3aa', fontSize: '18px', fontWeight: 800, marginTop: '4px' }}>{matchedTotal}</div>
                      </div>
                    </div>

                    <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                      <button
                        onClick={() => onPlayMode?.(mode.id, 'casual')}
                        style={{
                          flex: 1,
                          minWidth: '140px',
                          padding: '11px 14px',
                          borderRadius: '10px',
                          border: '1px solid rgba(255,180,60,0.28)',
                          background: 'rgba(255,180,60,0.08)',
                          color: '#fff0c6',
                          fontSize: '12px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        Play Casual
                      </button>
                      <button
                        onClick={() => onPlayMode?.(mode.id, 'rated')}
                        style={{
                          flex: 1,
                          minWidth: '140px',
                          padding: '11px 14px',
                          borderRadius: '10px',
                          border: '1px solid rgba(110,170,255,0.32)',
                          background: 'linear-gradient(180deg, rgba(54,102,184,0.24) 0%, rgba(24,40,82,0.34) 100%)',
                          color: '#e5f0ff',
                          fontSize: '12px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        Play Rated
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
