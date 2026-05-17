import React from 'react';
import { OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import type { AccountLeaderboardSummary, AccountLeaderboardSpotlight, AccountProfile, AccountSeasonSummary, SeasonOption } from './lib/platform-service';
import { fetchAccountLeaderboard } from './lib/platform-service';

interface RankingsPageProps {
  onViewGuest?: (guestId: string) => void;
  onViewAccount?: (handle: string) => void;
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatRatingDelta(delta: number): string {
  return delta > 0 ? `+${delta}` : `${delta}`;
}

function resolveDisplayedSeason(account: AccountProfile): AccountSeasonSummary | undefined {
  return account.selectedSeason ?? account.currentSeason;
}

function describeSeason(summary?: AccountSeasonSummary): string {
  if (!summary) {
    return 'No season matches yet';
  }
  return `${summary.label}: ${summary.matchesPlayed} matches, ${formatRatingDelta(summary.netDelta)}`;
}

function parseModeFilterValue(value: string): MatchModeId | '' {
  return OFFICIAL_MATCH_MODES.some((mode) => mode.id === value as MatchModeId) ? (value as MatchModeId) : '';
}

function formatWinRate(spotlight?: AccountLeaderboardSpotlight): string {
  if (!spotlight || spotlight.matchesPlayed <= 0) {
    return '--';
  }
  const winRate = Math.round((spotlight.wins / spotlight.matchesPlayed) * 100);
  return `${winRate}%`;
}

function renderSpotlightLabel(summary: AccountLeaderboardSummary | undefined, selectedModeId: MatchModeId | ''): string {
  if (summary?.seasonLabel?.trim()) {
    return summary.seasonLabel;
  }
  if (selectedModeId) {
    return `${OFFICIAL_MATCH_MODES.find((mode) => mode.id === selectedModeId)?.label ?? 'Mode'} ladder`;
  }
  return 'Current ladder';
}

export default function RankingsPage({ onViewGuest, onViewAccount }: RankingsPageProps): React.ReactElement {
  const [accounts, setAccounts] = React.useState<AccountProfile[]>([]);
  const [seasons, setSeasons] = React.useState<SeasonOption[]>([]);
  const [summary, setSummary] = React.useState<AccountLeaderboardSummary | undefined>(undefined);
  const [selectedSeasonId, setSelectedSeasonId] = React.useState('');
  const [selectedModeId, setSelectedModeId] = React.useState<MatchModeId | ''>('');
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState('');

  const loadRankings = React.useCallback(async (seasonId?: string, modeId?: MatchModeId) => {
    setLoading(true);
    setError('');
    setSummary(undefined);
    try {
      const payload = await fetchAccountLeaderboard(50, 'rating', seasonId, modeId);
      setAccounts(payload.accounts);
      setSeasons(payload.seasons);
      setSummary(payload.summary);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load rankings.');
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void loadRankings(selectedSeasonId || undefined, selectedModeId || undefined);
  }, [loadRankings, selectedModeId, selectedSeasonId]);

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
              <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Account Rankings</div>
              <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
                Claimed account leaderboard with official-mode lanes and season-aware momentum filters.
              </div>
            </div>
            <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
              <select
                value={selectedModeId}
                onChange={(event) => setSelectedModeId(parseModeFilterValue(event.target.value))}
                style={{
                  padding: '8px 10px',
                  borderRadius: '8px',
                  border: '1px solid rgba(255,180,60,0.24)',
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
              <select
                value={selectedSeasonId}
                onChange={(event) => setSelectedSeasonId(event.target.value)}
                style={{
                  padding: '8px 10px',
                  borderRadius: '8px',
                  border: '1px solid rgba(255,180,60,0.24)',
                  background: 'rgba(255,255,255,0.04)',
                  color: '#fff2c8',
                  fontSize: '12px',
                  fontWeight: 700,
                }}
              >
                <option value="">All seasons</option>
                {seasons.map((season) => (
                  <option key={season.seasonId} value={season.seasonId}>
                    {season.label}
                  </option>
                ))}
              </select>
              <button
                onClick={() => void loadRankings(selectedSeasonId || undefined, selectedModeId || undefined)}
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

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {!loading && summary && accounts.length > 0 && (
            <div style={{ display: 'grid', gap: '12px', marginBottom: '18px' }}>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
                  gap: '10px',
                }}
              >
                <div style={{ padding: '14px 16px', borderRadius: '12px', background: 'rgba(255,180,60,0.08)', border: '1px solid rgba(255,180,60,0.16)' }}>
                  <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '0.9px', textTransform: 'uppercase' }}>{renderSpotlightLabel(summary, selectedModeId)}</div>
                  <div style={{ color: '#fff4d2', fontSize: '20px', fontWeight: 900, marginTop: '8px' }}>{summary.playerCount}</div>
                  <div style={{ color: 'rgba(255,232,180,0.6)', fontSize: '11px', marginTop: '4px' }}>
                    players in this lane · {summary.matchCount} rated results tracked
                  </div>
                </div>
                {[
                  { label: 'Leader', spotlight: summary.leader, value: summary.leader ? `${summary.leader.rating}` : '--', detail: summary.leader ? `${summary.leader.matchesPlayed} matches · ${formatWinRate(summary.leader)} win rate` : 'No leader yet' },
                  { label: 'Biggest climb', spotlight: summary.biggestClimber, value: summary.biggestClimber ? formatRatingDelta(summary.biggestClimber.netDelta) : '--', detail: summary.biggestClimber ? `${summary.biggestClimber.matchesPlayed} matches · rating ${summary.biggestClimber.rating}` : 'No climb data yet' },
                  { label: 'Peak holder', spotlight: summary.highestPeak, value: summary.highestPeak ? `${summary.highestPeak.peakRating}` : '--', detail: summary.highestPeak ? `${summary.highestPeak.matchesPlayed} matches · ${summary.highestPeak.displayName}` : 'No peak yet' },
                  { label: 'Most active', spotlight: summary.mostActive, value: summary.mostActive ? `${summary.mostActive.matchesPlayed}` : '--', detail: summary.mostActive ? `${summary.mostActive.displayName} · ${formatWinRate(summary.mostActive)} win rate` : 'No volume yet' },
                ].map((card) => (
                  <div key={card.label} style={{ padding: '14px 16px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.1)' }}>
                    <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '0.9px', textTransform: 'uppercase' }}>{card.label}</div>
                    <div style={{ color: '#fff4d2', fontSize: '18px', fontWeight: 900, marginTop: '8px' }}>{card.value}</div>
                    <div style={{ color: '#ffd98f', fontSize: '12px', fontWeight: 700, marginTop: '6px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {card.spotlight ? `@${card.spotlight.handle}` : 'Waiting for results'}
                    </div>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', marginTop: '4px', lineHeight: 1.45 }}>
                      {card.detail}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

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
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading rankings...</div>
          ) : accounts.length === 0 ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>
              {selectedSeasonId || selectedModeId
                ? 'No accounts have ladder activity for that mode and season filter yet.'
                : 'No claimed accounts yet. Open the Account tab to claim a handle and start building the account ladder.'}
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {accounts.map((account, index) => {
                const season = resolveDisplayedSeason(account);
                return (
                  <div
                    key={account.accountId}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '72px minmax(0, 1fr) 120px 220px 110px',
                      gap: '12px',
                      alignItems: 'center',
                      padding: '14px 16px',
                      borderRadius: '12px',
                      background: index < 3
                        ? 'linear-gradient(180deg, rgba(200,134,10,0.18) 0%, rgba(70,42,8,0.22) 100%)'
                        : 'rgba(255,255,255,0.03)',
                      border: index < 3
                        ? '1px solid rgba(255,180,60,0.18)'
                        : '1px solid rgba(255,165,40,0.08)',
                    }}
                  >
                    <div style={{ color: index === 0 ? '#ffd76e' : index === 1 ? '#d9e0ef' : index === 2 ? '#d79d72' : 'rgba(255,232,180,0.7)', fontSize: '18px', fontWeight: 900 }}>
                      #{index + 1}
                    </div>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {account.displayName ?? account.handle}
                      </div>
                      <div style={{ color: '#ffd98f', fontSize: '11px', fontWeight: 700, marginTop: '3px' }}>@{account.handle}</div>
                      <div style={{ color: 'rgba(170,190,220,0.62)', fontSize: '11px', marginTop: '4px' }}>
                        {account.matchesPlayed ?? 0} matches - {account.wins ?? 0}W {account.losses ?? 0}L {account.draws ?? 0}D - {account.guestCount ?? account.linkedGuestIds.length} guest{(account.guestCount ?? account.linkedGuestIds.length) === 1 ? '' : 's'}
                      </div>
                      <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '11px', marginTop: '4px' }}>
                        {describeSeason(season)}
                      </div>
                    </div>
                    <div style={{ color: '#7ce3aa', fontSize: '15px', fontWeight: 800, textAlign: 'right' }}>
                      #{selectedSeasonId && season ? season.ratingEnd : (account.rating ?? 1200)}
                    </div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', textAlign: 'right' }}>
                      {season
                        ? `${season.label} peak ${season.peakRating}`
                        : `Last seen ${formatDateTime(account.lastSeenAt)}`}
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                      <button
                        onClick={() => {
                          if (onViewAccount) {
                            onViewAccount(account.handle);
                            return;
                          }
                          onViewGuest?.(account.primaryGuestId);
                        }}
                        style={{
                          padding: '7px 10px',
                          borderRadius: '8px',
                          border: '1px solid rgba(255,180,60,0.22)',
                          background: 'rgba(255,180,60,0.08)',
                          color: '#ffe7a9',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        Profile
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
