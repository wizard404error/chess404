import React from 'react';
import { fetchSystemStatus, type SystemStatusSnapshot } from './lib/system-service';

export default function StatusPage() {
  const [snapshot, setSnapshot] = React.useState<SystemStatusSnapshot | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setSnapshot(await fetchSystemStatus());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load service status');
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  return (
    <div style={{
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: '24px 28px 30px',
      color: '#f4e8c8',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '18px', gap: '16px', flexWrap: 'wrap' }}>
        <div>
          <div style={{ color: '#ffb830', fontSize: '11px', fontWeight: 800, letterSpacing: '2px', textTransform: 'uppercase', marginBottom: '6px' }}>System Status</div>
          <h2 style={{ margin: 0, fontSize: '30px', color: '#fff4d6' }}>Operations snapshot</h2>
          <div style={{ color: 'rgba(222, 210, 180, 0.7)', fontSize: '13px', marginTop: '6px' }}>
            Live service health for match authority, platform data, and matchmaking.
          </div>
        </div>
        <button
          onClick={() => void load()}
          style={{
            padding: '10px 16px',
            background: 'linear-gradient(180deg, rgba(200,134,10,0.88) 0%, rgba(122,79,8,0.95) 100%)',
            color: '#fff7e3',
            border: '1px solid rgba(255,190,70,0.5)',
            borderRadius: '8px',
            cursor: 'pointer',
            fontWeight: 700,
            fontSize: '13px',
            boxShadow: '0 4px 14px rgba(200,120,20,0.28)',
          }}
        >
          {loading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {error && (
        <div style={{
          marginBottom: '16px',
          padding: '12px 14px',
          borderRadius: '10px',
          background: 'rgba(120, 18, 18, 0.35)',
          border: '1px solid rgba(220, 80, 80, 0.45)',
          color: '#ffd6d6',
          fontSize: '13px',
        }}>
          {error}
        </div>
      )}

      {snapshot && (
        <>
          <div style={{
            marginBottom: '18px',
            padding: '16px 18px',
            borderRadius: '16px',
            background: snapshot.gateway.status === 'ok'
              ? 'linear-gradient(180deg, rgba(18,58,38,0.88) 0%, rgba(12,34,23,0.96) 100%)'
              : 'linear-gradient(180deg, rgba(78,36,12,0.88) 0%, rgba(48,22,10,0.96) 100%)',
            border: snapshot.gateway.status === 'ok'
              ? '1px solid rgba(72, 207, 129, 0.34)'
              : '1px solid rgba(255, 173, 84, 0.34)',
            boxShadow: '0 12px 28px rgba(0,0,0,0.18)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: '14px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
              <div>
                <div style={{ color: '#fff4d6', fontSize: '20px', fontWeight: 800, marginBottom: '6px' }}>
                  Gateway control plane: {snapshot.gateway.status === 'ok' ? 'ready' : 'degraded'}
                </div>
                <div style={{ color: 'rgba(244, 232, 200, 0.74)', fontSize: '13px' }}>
                  The gateway now aggregates backend readiness across realtime, platform, and matchmaking services.
                </div>
              </div>
              <div style={{ color: 'rgba(244, 232, 200, 0.54)', fontSize: '11px' }}>
                Checked {new Date(snapshot.gateway.checkedAt).toLocaleString()}
              </div>
            </div>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', marginTop: '14px' }}>
              {([
                ['Match', snapshot.gateway.services.match],
                ['Platform', snapshot.gateway.services.platform],
                ['Matchmaking', snapshot.gateway.services.matchmaking],
              ] as const).map(([label, service]) => (
                <div key={label} style={{
                  padding: '10px 12px',
                  borderRadius: '12px',
                  minWidth: '180px',
                  background: 'rgba(255,255,255,0.04)',
                  border: `1px solid ${service.healthy ? 'rgba(72, 207, 129, 0.25)' : 'rgba(255, 173, 84, 0.25)'}`,
                }}>
                  <div style={{ color: '#ffcd67', fontSize: '11px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase', marginBottom: '6px' }}>{label}</div>
                  <div style={{ color: service.healthy ? '#9af4be' : '#ffd59a', fontSize: '14px', fontWeight: 700 }}>
                    {service.healthy ? 'Healthy' : 'Unreachable'}
                  </div>
                  <div style={{ color: 'rgba(244, 232, 200, 0.58)', fontSize: '11px', marginTop: '3px' }}>
                    {service.error ? service.error : `HTTP ${service.statusCode ?? 0}`}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(290px, 1fr))', gap: '16px', marginBottom: '18px' }}>
            <StatusCard
              title="Match Service"
              subtitle={`${snapshot.match.service} · ${snapshot.match.rulesVersion}`}
              checkedAt={snapshot.match.checkedAt}
              healthy={snapshot.match.status === 'ok'}
              rows={[
                { label: 'Loaded rooms', value: String(snapshot.match.stats.loadedMatches) },
                { label: 'Active rooms', value: String(snapshot.match.stats.activeMatches) },
                { label: 'Finished rooms', value: String(snapshot.match.stats.finishedMatches) },
                { label: 'Live subscribers', value: String(snapshot.match.stats.subscriberCount) },
                { label: 'Event buffers', value: String(snapshot.match.stats.bufferedEventSets) },
              ]}
            />

            <StatusCard
              title="Platform Service"
              subtitle={`${snapshot.platform.service} · ${snapshot.platform.archiveBackend ?? 'unknown'} archive · ${snapshot.platform.guestStoreBackend ?? 'unknown'} guest store · ${snapshot.platform.claimStoreBackend ?? 'unknown'} claim store`}
              checkedAt={snapshot.platform.checkedAt}
              healthy={snapshot.platform.status === 'ok'}
              rows={[
                { label: 'Archive backend', value: snapshot.platform.archiveBackend ?? 'unknown' },
                { label: 'Archived matches', value: String(snapshot.platform.archive.totalMatches) },
                { label: 'Finished archives', value: String(snapshot.platform.archive.finishedMatches) },
                { label: 'Rated archives', value: String(snapshot.platform.archive.ratedMatches) },
                { label: 'Guest store backend', value: snapshot.platform.guestStoreBackend ?? 'unknown' },
                { label: 'Account store backend', value: snapshot.platform.accountStoreBackend ?? 'unknown' },
                { label: 'Accounts', value: String(snapshot.platform.accounts.accountCount) },
                { label: 'Linked guests', value: String(snapshot.platform.accounts.linkedGuestCount) },
                { label: 'Active account sessions', value: String(snapshot.platform.accounts.activeSessionCount) },
                { label: 'Claim store backend', value: snapshot.platform.claimStoreBackend ?? 'unknown' },
                { label: 'Claim lease (sec)', value: String(snapshot.platform.claimLeaseSeconds ?? 0) },
                { label: 'Cached room claims', value: String(snapshot.platform.claims.cachedClaims) },
                { label: 'Guest profiles', value: String(snapshot.platform.guests.guestCount) },
                { label: 'Ranked guests', value: String(snapshot.platform.guests.rankedPlayers) },
                { label: 'Finalized rated results', value: String(snapshot.platform.guests.finalizedMatchCount) },
              ]}
            />

            <StatusCard
              title="Matchmaking Service"
              subtitle={`${snapshot.matchmaking.service} · ${snapshot.matchmaking.stats.backend ?? 'unknown'} ticket store`}
              checkedAt={snapshot.matchmaking.checkedAt}
              healthy={snapshot.matchmaking.status === 'ok'}
              rows={[
                { label: 'Ticket store backend', value: snapshot.matchmaking.stats.backend ?? 'unknown' },
                { label: 'Total tickets', value: String(snapshot.matchmaking.stats.totalTickets) },
                { label: 'Rated queued', value: String(snapshot.matchmaking.stats.rated.queuedCount) },
                { label: 'Rated matched', value: String(snapshot.matchmaking.stats.rated.matchedCount) },
                { label: 'Casual queued', value: String(snapshot.matchmaking.stats.casual.queuedCount) },
                { label: 'Casual matched', value: String(snapshot.matchmaking.stats.casual.matchedCount) },
                { label: 'Cancelled tickets', value: String(snapshot.matchmaking.stats.rated.cancelledCount + snapshot.matchmaking.stats.casual.cancelledCount) },
              ]}
            />
          </div>

          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
            gap: '14px',
          }}>
            <CompactPanel
              title="Archive Mix"
              items={[
                `Rated: ${snapshot.platform.archive.ratedMatches}`,
                `Casual: ${snapshot.platform.archive.casualMatches}`,
                `Direct: ${snapshot.platform.archive.directMatches}`,
              ]}
            />
            <CompactPanel
              title="Queue Mix"
              items={[
                `Rated matched: ${snapshot.matchmaking.stats.rated.matchedCount}`,
                `Casual matched: ${snapshot.matchmaking.stats.casual.matchedCount}`,
                `Open queued: ${snapshot.matchmaking.stats.rated.queuedCount + snapshot.matchmaking.stats.casual.queuedCount}`,
              ]}
            />
            <CompactPanel
              title="Room State"
              items={[
                `Live rooms: ${snapshot.match.stats.activeMatches}`,
                `Archived finished: ${snapshot.platform.archive.finishedMatches}`,
                `Subscribers: ${snapshot.match.stats.subscriberCount}`,
              ]}
            />
          </div>
        </>
      )}
    </div>
  );
}

function StatusCard(props: {
  title: string;
  subtitle: string;
  checkedAt: string;
  healthy: boolean;
  rows: Array<{ label: string; value: string }>;
}) {
  return (
    <div style={{
      background: 'linear-gradient(180deg, rgba(11,18,30,0.86) 0%, rgba(13,20,34,0.94) 100%)',
      border: '1px solid rgba(255, 175, 55, 0.18)',
      borderRadius: '16px',
      padding: '16px',
      boxShadow: '0 12px 28px rgba(0,0,0,0.18)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'flex-start', marginBottom: '12px' }}>
        <div>
          <div style={{ color: '#fff4d6', fontSize: '18px', fontWeight: 700 }}>{props.title}</div>
          <div style={{ color: 'rgba(222, 210, 180, 0.6)', fontSize: '12px', marginTop: '4px' }}>{props.subtitle}</div>
        </div>
        <div style={{
          padding: '5px 9px',
          borderRadius: '999px',
          background: props.healthy ? 'rgba(46, 204, 113, 0.14)' : 'rgba(255, 173, 84, 0.14)',
          border: props.healthy ? '1px solid rgba(46, 204, 113, 0.35)' : '1px solid rgba(255, 173, 84, 0.35)',
          color: props.healthy ? '#8bf0b2' : '#ffd59a',
          fontSize: '11px',
          fontWeight: 700,
          whiteSpace: 'nowrap',
        }}>
          {props.healthy ? 'Healthy' : 'Degraded'}
        </div>
      </div>

      <div style={{ display: 'grid', gap: '8px' }}>
        {props.rows.map(row => (
          <div key={row.label} style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center' }}>
            <span style={{ color: 'rgba(222, 210, 180, 0.72)', fontSize: '13px' }}>{row.label}</span>
            <span style={{ color: '#ffe6a8', fontSize: '15px', fontWeight: 700, fontFamily: 'monospace' }}>{row.value}</span>
          </div>
        ))}
      </div>

      <div style={{ marginTop: '14px', paddingTop: '10px', borderTop: '1px solid rgba(255,255,255,0.06)', color: 'rgba(190, 178, 150, 0.52)', fontSize: '11px' }}>
        Checked {new Date(props.checkedAt).toLocaleString()}
      </div>
    </div>
  );
}

function CompactPanel(props: { title: string; items: string[] }) {
  return (
    <div style={{
      background: 'rgba(9, 14, 24, 0.7)',
      border: '1px solid rgba(255, 175, 55, 0.14)',
      borderRadius: '14px',
      padding: '14px 16px',
    }}>
      <div style={{ color: '#ffcd67', fontSize: '12px', fontWeight: 800, letterSpacing: '1.5px', textTransform: 'uppercase', marginBottom: '10px' }}>
        {props.title}
      </div>
      <div style={{ display: 'grid', gap: '8px' }}>
        {props.items.map(item => (
          <div key={item} style={{ color: '#f4e8c8', fontSize: '13px' }}>{item}</div>
        ))}
      </div>
    </div>
  );
}
