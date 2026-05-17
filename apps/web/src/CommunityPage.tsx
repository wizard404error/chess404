import React from 'react';
import type { AccountProfile, GuestProfile, MatchArchiveEntry } from './lib/platform-service';
import { fetchAccounts, fetchGuest, fetchGuestArchivedMatches, fetchGuests } from './lib/platform-service';

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function statLabel(player: GuestProfile): string {
  return `${player.wins}W ${player.losses}L ${player.draws}D`;
}

interface CommunityPageProps {
  whiteProfile?: GuestProfile | null;
  blackProfile?: GuestProfile | null;
  focusGuestId?: string | null;
  onOpenMatch?: (matchId: string) => void;
  onOpenGuestHistory?: (guestId: string) => void;
  onOpenAccount?: (handle: string) => void;
}

export default function CommunityPage({
  whiteProfile = null,
  blackProfile = null,
  focusGuestId = null,
  onOpenMatch,
  onOpenGuestHistory,
  onOpenAccount,
}: CommunityPageProps): React.ReactElement {
  const [guests, setGuests] = React.useState<GuestProfile[]>([]);
  const [accountsByGuestId, setAccountsByGuestId] = React.useState<Record<string, AccountProfile>>({});
  const [selectedGuestId, setSelectedGuestId] = React.useState<string | null>(null);
  const [selectedGuest, setSelectedGuest] = React.useState<GuestProfile | null>(null);
  const [recentMatches, setRecentMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [loadingDetail, setLoadingDetail] = React.useState(false);
  const [loadingMatches, setLoadingMatches] = React.useState(false);
  const [error, setError] = React.useState('');

  const loadGuests = React.useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [nextGuests, accounts] = await Promise.all([
        fetchGuests(24),
        fetchAccounts(100),
      ]);
      const nextAccountsByGuestId: Record<string, AccountProfile> = {};
      for (const account of accounts) {
        for (const guestId of account.linkedGuestIds) {
          nextAccountsByGuestId[guestId] = account;
        }
      }
      setGuests(nextGuests);
      setAccountsByGuestId(nextAccountsByGuestId);
      setSelectedGuestId(currentSelected => currentSelected && nextGuests.some(guest => guest.guestId === currentSelected)
        ? currentSelected
        : nextGuests[0]?.guestId ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load community players.');
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void loadGuests();
  }, [loadGuests]);

  React.useEffect(() => {
    if (focusGuestId) {
      setSelectedGuestId(focusGuestId);
    }
  }, [focusGuestId]);

  React.useEffect(() => {
    if (!selectedGuestId) {
      setSelectedGuest(null);
      setRecentMatches([]);
      return;
    }

    let cancelled = false;
    setLoadingDetail(true);

    void fetchGuest(selectedGuestId)
      .then(guest => {
        if (!cancelled) {
          setSelectedGuest(guest);
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load guest profile.');
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
  }, [selectedGuestId]);

  React.useEffect(() => {
    if (!selectedGuestId) {
      setRecentMatches([]);
      return;
    }

    let cancelled = false;
    setLoadingMatches(true);

    void fetchGuestArchivedMatches(selectedGuestId, 8)
      .then(matches => {
        if (!cancelled) {
          setRecentMatches(matches);
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load guest match history.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingMatches(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedGuestId]);

  const featuredGuest = selectedGuest ?? guests[0] ?? null;
  const featuredAccount = featuredGuest ? accountsByGuestId[featuredGuest.guestId] ?? null : null;
  const describeGuestMatch = React.useCallback((match: MatchArchiveEntry) => {
    const isWhite = match.whiteGuestId === featuredGuest?.guestId;
    const opponentBase = isWhite
      ? (match.blackName ?? match.blackGuestId ?? 'Black guest')
      : (match.whiteName ?? match.whiteGuestId ?? 'White guest');
    const opponentHandle = isWhite ? match.blackAccountHandle : match.whiteAccountHandle;
    const opponent = opponentHandle ? `${opponentBase} (@${opponentHandle})` : opponentBase;
    const result =
      match.winner === 'draw'
        ? 'Draw'
        : match.winner === (isWhite ? 'white' : 'black')
          ? 'Win'
          : match.status === 'finished'
            ? 'Loss'
            : 'Active';
    return { opponent, result };
  }, [featuredGuest?.guestId]);

  return (
    <div style={{ display: 'flex', flex: 1, minHeight: 0, padding: '22px 28px 26px', gap: '18px' }}>
      <div
        style={{
          width: '390px',
          flexShrink: 0,
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
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center' }}>
            <div>
              <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Community Guests</div>
              <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
                Recent local player identities with ratings, linked account handles, and match records from the platform service.
              </div>
            </div>
            <button
              onClick={() => void loadGuests()}
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
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading community profiles...</div>
          ) : guests.length === 0 ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>No guest profiles yet. Create guest sessions or finish a queued match to populate this page.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {guests.map(player => (
                <button
                  key={player.guestId}
                  onClick={() => setSelectedGuestId(player.guestId)}
                  style={{
                    cursor: 'pointer',
                    textAlign: 'left',
                    borderRadius: '14px',
                    border: selectedGuestId === player.guestId
                      ? '1px solid rgba(255,190,90,0.32)'
                      : '1px solid rgba(255,165,40,0.12)',
                    background: selectedGuestId === player.guestId
                      ? 'linear-gradient(180deg, rgba(200,134,10,0.2) 0%, rgba(70,42,8,0.22) 100%)'
                      : 'linear-gradient(180deg, rgba(255,255,255,0.04) 0%, rgba(255,255,255,0.02) 100%)',
                    boxShadow: '0 10px 28px rgba(0,0,0,0.24)',
                    padding: '16px 16px 14px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    color: 'inherit',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'flex-start' }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ color: '#fff2c8', fontSize: '15px', fontWeight: 800, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {player.displayName}
                      </div>
                      {accountsByGuestId[player.guestId] && (
                        <div style={{ color: '#ffd98f', fontSize: '11px', fontWeight: 700, marginTop: '4px' }}>
                          @{accountsByGuestId[player.guestId].handle}
                        </div>
                      )}
                      <div style={{ color: 'rgba(170,190,220,0.62)', fontSize: '11px', marginTop: '4px' }}>
                        {player.guestId}
                      </div>
                    </div>
                    <div style={{ color: '#7ce3aa', fontSize: '15px', fontWeight: 800, flexShrink: 0 }}>
                      {player.rating}
                    </div>
                  </div>

                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                    {whiteProfile?.guestId === player.guestId && (
                      <span style={{ padding: '3px 7px', borderRadius: '999px', background: 'rgba(80,140,255,0.18)', border: '1px solid rgba(110,170,255,0.22)', color: '#dcecff', fontSize: '10px', fontWeight: 800 }}>
                        White Seat
                      </span>
                    )}
                    {blackProfile?.guestId === player.guestId && (
                      <span style={{ padding: '3px 7px', borderRadius: '999px', background: 'rgba(170,100,255,0.14)', border: '1px solid rgba(190,130,255,0.22)', color: '#eadbff', fontSize: '10px', fontWeight: 800 }}>
                        Black Seat
                      </span>
                    )}
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '8px' }}>
                    <div style={{ padding: '10px 12px', borderRadius: '10px', background: 'rgba(255,180,60,0.08)', border: '1px solid rgba(255,180,60,0.1)' }}>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Record</div>
                      <div style={{ color: '#ffe9b5', fontSize: '13px', fontWeight: 800, marginTop: '4px' }}>{statLabel(player)}</div>
                    </div>
                    <div style={{ padding: '10px 12px', borderRadius: '10px', background: 'rgba(100,160,255,0.08)', border: '1px solid rgba(120,180,255,0.1)' }}>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Matches</div>
                      <div style={{ color: '#d8eaff', fontSize: '13px', fontWeight: 800, marginTop: '4px' }}>{player.matchesPlayed}</div>
                    </div>
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', color: 'rgba(255,232,180,0.6)', fontSize: '11px' }}>
                    <span>Joined {formatDateTime(player.createdAt)}</span>
                    <span>Seen {formatDateTime(player.lastSeenAt)}</span>
                  </div>
                </button>
              ))}
            </div>
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
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Profile Focus</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            Selected guest summary from the platform profile store.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {loadingDetail && !featuredGuest ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading guest profile...</div>
          ) : !featuredGuest ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Select a guest from the directory to inspect their profile.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div style={{ padding: '18px', borderRadius: '16px', background: 'linear-gradient(180deg, rgba(200,134,10,0.18) 0%, rgba(70,42,8,0.22) 100%)', border: '1px solid rgba(255,185,70,0.18)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '14px', alignItems: 'flex-start' }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ color: '#fff2c8', fontSize: '22px', fontWeight: 900 }}>{featuredGuest.displayName}</div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px', marginTop: '6px' }}>{featuredGuest.guestId}</div>
                  </div>
                  <div style={{ color: '#7ce3aa', fontSize: '28px', fontWeight: 900 }}>{featuredGuest.rating}</div>
                </div>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '12px' }}>
                  {featuredAccount && (
                    <span style={{ padding: '5px 9px', borderRadius: '999px', background: 'rgba(255,200,100,0.14)', border: '1px solid rgba(255,210,120,0.24)', color: '#ffe8af', fontSize: '11px', fontWeight: 800 }}>
                      @{featuredAccount.handle}
                    </span>
                  )}
                  {whiteProfile?.guestId === featuredGuest.guestId && (
                    <span style={{ padding: '5px 9px', borderRadius: '999px', background: 'rgba(80,140,255,0.18)', border: '1px solid rgba(110,170,255,0.22)', color: '#dcecff', fontSize: '11px', fontWeight: 800 }}>
                      Current White Player
                    </span>
                  )}
                  {blackProfile?.guestId === featuredGuest.guestId && (
                    <span style={{ padding: '5px 9px', borderRadius: '999px', background: 'rgba(170,100,255,0.14)', border: '1px solid rgba(190,130,255,0.22)', color: '#eadbff', fontSize: '11px', fontWeight: 800 }}>
                      Current Black Player
                    </span>
                  )}
                  <button
                    onClick={() => onOpenGuestHistory?.(featuredGuest.guestId)}
                    style={{
                      padding: '5px 9px',
                      borderRadius: '999px',
                      background: 'rgba(255,255,255,0.06)',
                      border: '1px solid rgba(255,255,255,0.12)',
                      color: '#fff1c7',
                      fontSize: '11px',
                      fontWeight: 800,
                      cursor: onOpenGuestHistory ? 'pointer' : 'default',
                    }}
                  >
                    Full History
                  </button>
                  {featuredAccount && (
                    <button
                      onClick={() => onOpenAccount?.(featuredAccount.handle)}
                      style={{
                        padding: '5px 9px',
                        borderRadius: '999px',
                        background: 'rgba(255,180,60,0.10)',
                        border: '1px solid rgba(255,180,60,0.20)',
                        color: '#ffe9b1',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: onOpenAccount ? 'pointer' : 'default',
                      }}
                    >
                      Open Account Profile
                    </button>
                  )}
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: '12px' }}>
                {[
                  { label: 'Matches', value: featuredGuest.matchesPlayed, color: '#d8eaff' },
                  { label: 'Wins', value: featuredGuest.wins, color: '#8ef0b6' },
                  { label: 'Losses', value: featuredGuest.losses, color: '#ffb3a0' },
                  { label: 'Draws', value: featuredGuest.draws, color: '#ffe2a5' },
                ].map(stat => (
                  <div key={stat.label} style={{ padding: '14px 14px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.035)', border: '1px solid rgba(255,165,40,0.08)' }}>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>{stat.label}</div>
                    <div style={{ color: stat.color, fontSize: '20px', fontWeight: 900, marginTop: '6px' }}>{stat.value}</div>
                  </div>
                ))}
              </div>

              <div style={{ padding: '16px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)', display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '14px' }}>
                <div>
                  <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Joined</div>
                  <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px' }}>{formatDateTime(featuredGuest.createdAt)}</div>
                </div>
                <div>
                  <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Last Seen</div>
                  <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px' }}>{formatDateTime(featuredGuest.lastSeenAt)}</div>
                </div>
                {featuredAccount && (
                  <>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Account ID</div>
                      <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px', fontFamily: 'monospace' }}>{featuredAccount.accountId}</div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Account Last Seen</div>
                      <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px' }}>{formatDateTime(featuredAccount.lastSeenAt)}</div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Account Ladder</div>
                      <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px' }}>
                        {featuredAccount.rating ?? 1200} · {featuredAccount.matchesPlayed ?? 0} matches
                      </div>
                    </div>
                    <div>
                      <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Account Record</div>
                      <div style={{ color: '#fff2c8', fontSize: '13px', fontWeight: 700, marginTop: '6px' }}>
                        {featuredAccount.wins ?? 0}W {featuredAccount.losses ?? 0}L {featuredAccount.draws ?? 0}D
                      </div>
                    </div>
                  </>
                )}
              </div>

              <div style={{ padding: '16px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
                  <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Recent Matches</div>
                  {loadingMatches && (
                    <div style={{ color: 'rgba(255,232,180,0.55)', fontSize: '11px' }}>Loading...</div>
                  )}
                </div>

                {recentMatches.length === 0 ? (
                  <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '12px', marginTop: '12px' }}>
                    {loadingMatches ? 'Looking up archived matches...' : 'No archived matches yet for this guest.'}
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '12px' }}>
                    {recentMatches.map(match => {
                      const info = describeGuestMatch(match);
                      return (
                        <button
                          key={match.matchId}
                          onClick={() => onOpenMatch?.(match.matchId)}
                          style={{
                            textAlign: 'left',
                            cursor: onOpenMatch ? 'pointer' : 'default',
                            padding: '12px 12px 11px',
                            borderRadius: '12px',
                            border: '1px solid rgba(255,165,40,0.12)',
                            background: 'linear-gradient(180deg, rgba(18,23,36,0.95) 0%, rgba(11,14,24,0.94) 100%)',
                            color: '#fff2c8',
                          }}
                        >
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
                            <div style={{ fontSize: '12px', fontWeight: 800 }}>{info.result} vs {info.opponent}</div>
                            <div style={{ color: 'rgba(160,184,216,0.64)', fontSize: '10px' }}>{match.status}</div>
                          </div>
                          <div style={{ marginTop: '6px', color: 'rgba(255,232,180,0.7)', fontSize: '11px' }}>
                            {match.moveCount} moves{match.lastMove ? ` · last ${match.lastMove}` : ''}
                          </div>
                          <div style={{ marginTop: '4px', color: 'rgba(170,190,220,0.62)', fontSize: '11px' }}>
                            {formatDateTime(match.updatedAt)}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
