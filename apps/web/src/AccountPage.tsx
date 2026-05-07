import React from 'react';
import {
  claimAccount,
  fetchAccountArchivedMatches,
  fetchAccount,
  resumeAccountSession,
  type AccountProfile,
  type AccountRatingHistoryEntry,
  type AccountSession,
  type GuestProfile,
  type MatchArchiveEntry,
} from './lib/platform-service';

const WHITE_GUEST_ID_STORAGE_KEY = 'chess404.guest.white';
const BLACK_GUEST_ID_STORAGE_KEY = 'chess404.guest.black';
const WHITE_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.white.secret';
const BLACK_GUEST_SECRET_STORAGE_KEY = 'chess404.guest.black.secret';
const WHITE_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.white.token';
const BLACK_GUEST_TOKEN_STORAGE_KEY = 'chess404.guest.black.token';

const WHITE_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.white.id';
const BLACK_ACCOUNT_ID_STORAGE_KEY = 'chess404.account.black.id';
const WHITE_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.white.token';
const BLACK_ACCOUNT_TOKEN_STORAGE_KEY = 'chess404.account.black.token';
const WHITE_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.white.expiresAt';
const BLACK_ACCOUNT_EXPIRY_STORAGE_KEY = 'chess404.account.black.expiresAt';

function formatDateTime(value?: string): string {
  if (!value) {
    return 'Unknown';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatRatingDelta(delta: number): string {
  return delta > 0 ? `+${delta}` : `${delta}`;
}

function readStoredGuestIdentity(side: 'white' | 'black'): {
  guestId?: string;
  sessionSecret?: string;
  sessionToken?: string;
} {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    guestId: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_ID_STORAGE_KEY : BLACK_GUEST_ID_STORAGE_KEY) ?? undefined,
    sessionSecret: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_SECRET_STORAGE_KEY : BLACK_GUEST_SECRET_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(side === 'white' ? WHITE_GUEST_TOKEN_STORAGE_KEY : BLACK_GUEST_TOKEN_STORAGE_KEY) ?? undefined,
  };
}

function readStoredAccountSession(side: 'white' | 'black'): {
  accountId?: string;
  sessionToken?: string;
  expiresAt?: string;
} {
  if (typeof window === 'undefined') {
    return {};
  }
  return {
    accountId: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY) ?? undefined,
    sessionToken: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY) ?? undefined,
    expiresAt: window.localStorage.getItem(side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY) ?? undefined,
  };
}

function writeStoredAccountSession(side: 'white' | 'black', session: AccountSession | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  const idKey = side === 'white' ? WHITE_ACCOUNT_ID_STORAGE_KEY : BLACK_ACCOUNT_ID_STORAGE_KEY;
  const tokenKey = side === 'white' ? WHITE_ACCOUNT_TOKEN_STORAGE_KEY : BLACK_ACCOUNT_TOKEN_STORAGE_KEY;
  const expiryKey = side === 'white' ? WHITE_ACCOUNT_EXPIRY_STORAGE_KEY : BLACK_ACCOUNT_EXPIRY_STORAGE_KEY;
  if (!session) {
    window.localStorage.removeItem(idKey);
    window.localStorage.removeItem(tokenKey);
    window.localStorage.removeItem(expiryKey);
    return;
  }
  window.localStorage.setItem(idKey, session.account.accountId);
  window.localStorage.setItem(tokenKey, session.sessionToken);
  window.localStorage.setItem(expiryKey, session.expiresAt);
}

function suggestHandle(seed: string): string {
  const normalized = seed
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 24);
  if (normalized.length >= 3) {
    return normalized;
  }
  return 'cardchess_player';
}

function describePerspective(entry: MatchArchiveEntry, accountId: string): 'white' | 'black' | 'unknown' {
  if (entry.whiteAccountId === accountId) {
    return 'white';
  }
  if (entry.blackAccountId === accountId) {
    return 'black';
  }
  return 'unknown';
}

function describeOutcome(entry: MatchArchiveEntry, accountId: string): string {
  const perspective = describePerspective(entry, accountId);
  if (entry.winner === 'draw') {
    return 'Draw';
  }
  if (perspective === 'white') {
    return entry.winner === 'white' ? 'Win' : 'Loss';
  }
  if (perspective === 'black') {
    return entry.winner === 'black' ? 'Win' : 'Loss';
  }
  return entry.winner ? `Winner: ${entry.winner}` : 'Unknown';
}

function describeOpponent(entry: MatchArchiveEntry, accountId: string): string {
  const perspective = describePerspective(entry, accountId);
  if (perspective === 'white') {
    return entry.blackAccountHandle ? `@${entry.blackAccountHandle}` : (entry.blackName ?? entry.blackGuestId ?? 'Unknown');
  }
  if (perspective === 'black') {
    return entry.whiteAccountHandle ? `@${entry.whiteAccountHandle}` : (entry.whiteName ?? entry.whiteGuestId ?? 'Unknown');
  }
  return entry.whiteName ?? entry.blackName ?? entry.matchId;
}

interface AccountPageProps {
  whiteProfile?: GuestProfile | null;
  blackProfile?: GuestProfile | null;
}

interface AccountSeatPanelProps {
  side: 'white' | 'black';
  label: string;
  accent: string;
  guestProfile?: GuestProfile | null;
}

function AccountSeatPanel({ side, label, accent, guestProfile = null }: AccountSeatPanelProps): React.ReactElement {
  const [guestIdentity, setGuestIdentity] = React.useState(() => readStoredGuestIdentity(side));
  const [accountSession, setAccountSession] = React.useState<AccountSession | null>(null);
  const [accountProfile, setAccountProfile] = React.useState<AccountProfile | null>(null);
  const [handle, setHandle] = React.useState('');
  const [loading, setLoading] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');
  const [selectedSeasonId, setSelectedSeasonId] = React.useState('');
  const [recentMatches, setRecentMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [recentMatchesLoading, setRecentMatchesLoading] = React.useState(false);
  const [recentMatchesError, setRecentMatchesError] = React.useState('');

  React.useEffect(() => {
    setGuestIdentity(readStoredGuestIdentity(side));
  }, [side, guestProfile?.guestId]);

  React.useEffect(() => {
    setHandle(current => {
      if (current.trim()) {
        return current;
      }
      if (accountProfile?.handle) {
        return accountProfile.handle;
      }
      if (guestProfile?.displayName) {
        return suggestHandle(guestProfile.displayName);
      }
      if (guestIdentity.guestId) {
        return suggestHandle(guestIdentity.guestId);
      }
      return current;
    });
  }, [accountProfile?.handle, guestIdentity.guestId, guestProfile?.displayName]);

  const refreshStoredAccount = React.useCallback(async () => {
    setLoading(true);
    setError('');
    const nextGuestIdentity = readStoredGuestIdentity(side);
    setGuestIdentity(nextGuestIdentity);
    const storedAccount = readStoredAccountSession(side);
    if (!storedAccount.accountId || !storedAccount.sessionToken) {
      setAccountSession(null);
      setAccountProfile(null);
      setLoading(false);
      return;
    }

    try {
      const session = await resumeAccountSession({
        accountId: storedAccount.accountId,
        sessionToken: storedAccount.sessionToken,
      });
      writeStoredAccountSession(side, session);
      setAccountSession(session);
      setAccountProfile(current => current?.accountId === session.account.accountId ? current : session.account);
      setNotice('');
    } catch (err) {
      writeStoredAccountSession(side, null);
      setAccountSession(null);
      try {
        const publicProfile = await fetchAccount(storedAccount.accountId);
        setAccountProfile(publicProfile);
        setHandle(publicProfile.handle);
        setNotice('Stored account session expired. Claim again to renew it.');
      } catch {
        setAccountProfile(null);
        setNotice('');
      }
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to resume account session.');
      }
    } finally {
      setLoading(false);
    }
  }, [side]);

  React.useEffect(() => {
    void refreshStoredAccount();
  }, [refreshStoredAccount]);

  React.useEffect(() => {
    const accountId = accountSession?.account.accountId ?? accountProfile?.accountId;
    if (!accountId) {
      return;
    }

    let cancelled = false;
    void fetchAccount(accountId, selectedSeasonId || undefined)
      .then(profile => {
        if (cancelled) {
          return;
        }
        setAccountProfile(profile);
        if (selectedSeasonId && !profile.selectedSeason) {
          setSelectedSeasonId('');
        }
      })
      .catch(() => {
        // Keep the session-backed profile if the detailed fetch fails.
      });

    return () => {
      cancelled = true;
    };
  }, [accountProfile?.accountId, accountSession?.account.accountId, selectedSeasonId]);

  React.useEffect(() => {
    const accountId = accountSession?.account.accountId ?? accountProfile?.accountId;
    if (!accountId) {
      setRecentMatches([]);
      setRecentMatchesLoading(false);
      setRecentMatchesError('');
      return;
    }

    let cancelled = false;
    setRecentMatchesLoading(true);
    setRecentMatchesError('');

    void fetchAccountArchivedMatches(accountId, 6, selectedSeasonId || undefined)
      .then(matches => {
        if (cancelled) {
          return;
        }
        setRecentMatches(matches);
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setRecentMatches([]);
        setRecentMatchesError(err instanceof Error ? err.message : 'Failed to load recent account matches.');
      })
      .finally(() => {
        if (!cancelled) {
          setRecentMatchesLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountProfile?.accountId, accountSession?.account.accountId, selectedSeasonId]);

  const submitClaim = React.useCallback(async () => {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const liveGuestIdentity = readStoredGuestIdentity(side);
      setGuestIdentity(liveGuestIdentity);
      if (!liveGuestIdentity.guestId) {
        throw new Error('No guest session is available for this seat yet.');
      }
      const desiredHandle = (accountProfile?.handle ?? handle).trim();
      const session = await claimAccount({
        guestId: liveGuestIdentity.guestId,
        sessionSecret: liveGuestIdentity.sessionSecret,
        sessionToken: liveGuestIdentity.sessionToken,
        handle: desiredHandle,
      });
      writeStoredAccountSession(side, session);
      setAccountSession(session);
      setAccountProfile(session.account);
      setHandle(session.account.handle);
      setSelectedSeasonId('');
      setNotice('Account session is active on this device.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to claim account.');
    } finally {
      setBusy(false);
    }
  }, [accountProfile?.handle, handle, side]);

  const renewSession = React.useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      await refreshStoredAccount();
      setNotice('Account session refreshed.');
    } finally {
      setBusy(false);
    }
  }, [refreshStoredAccount]);

  const profile = guestProfile;
  const activeAccount = accountProfile ?? accountSession?.account ?? null;
  const displayedRatingHistory = [...(activeAccount?.ratingHistory ?? [])].slice(-6).reverse();
  const displayedSeasonHistory = activeAccount?.seasonHistory?.slice(0, 4) ?? [];
  const highlightedSeason = activeAccount?.selectedSeason ?? activeAccount?.currentSeason;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '14px',
        minWidth: 0,
        background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
        border: `1px solid ${accent}`,
        borderRadius: '18px',
        padding: '20px',
        boxShadow: '0 16px 42px rgba(0,0,0,0.32)',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '16px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div>
          <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.8px', textTransform: 'uppercase', marginBottom: '6px' }}>{label}</div>
          <div style={{ color: '#fff4d6', fontSize: '24px', fontWeight: 800 }}>
            {profile?.displayName ?? guestIdentity.guestId ?? 'Guest seat'}
          </div>
          <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px', marginTop: '6px' }}>
            {profile ? `Guest rating ${profile.rating} · ${profile.wins}W ${profile.losses}L ${profile.draws}D` : 'Waiting for guest profile bootstrap.'}
          </div>
        </div>
        <div style={{
          padding: '8px 12px',
          borderRadius: '999px',
          border: `1px solid ${accent}`,
          color: '#ffe4a3',
          fontSize: '12px',
          fontWeight: 700,
          background: 'rgba(255,255,255,0.04)',
        }}>
          {accountSession ? 'Signed in locally' : activeAccount ? 'Claimed account' : 'Guest-only'}
        </div>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: '10px',
      }}>
        <MetaTile label="Guest ID" value={guestIdentity.guestId ?? 'Not available'} />
        <MetaTile label="Guest token" value={guestIdentity.sessionToken ? 'Active' : guestIdentity.sessionSecret ? 'Secret only' : 'Missing'} />
        <MetaTile label="Account handle" value={activeAccount?.handle ?? 'Unclaimed'} />
        <MetaTile label="Account session" value={accountSession?.expiresAt ? `Expires ${formatDateTime(accountSession.expiresAt)}` : 'No active session'} />
      </div>

      {error && (
        <div style={{
          padding: '12px 14px',
          borderRadius: '12px',
          background: 'rgba(120,18,18,0.34)',
          border: '1px solid rgba(220,80,80,0.45)',
          color: '#ffd6d6',
          fontSize: '13px',
          fontWeight: 700,
        }}>
          {error}
        </div>
      )}

      {notice && (
        <div style={{
          padding: '12px 14px',
          borderRadius: '12px',
          background: 'rgba(28,64,42,0.34)',
          border: '1px solid rgba(88,180,126,0.35)',
          color: '#d8ffe7',
          fontSize: '13px',
          fontWeight: 700,
        }}>
          {notice}
        </div>
      )}

      {loading ? (
        <div style={{ color: 'rgba(244,232,200,0.7)', fontSize: '13px' }}>Checking stored account session...</div>
      ) : (
        <>
          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '8px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '18px', fontWeight: 800 }}>@{activeAccount.handle}</div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Account ID: <span style={{ color: '#ffe9b1', fontFamily: 'monospace' }}>{activeAccount.accountId}</span>
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Primary guest: <span style={{ color: '#ffe9b1', fontFamily: 'monospace' }}>{activeAccount.primaryGuestId}</span>
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Linked guests: {activeAccount.linkedGuestIds.length}
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Ladder: {activeAccount.rating ?? 1200} · {activeAccount.matchesPlayed ?? 0} matches · {activeAccount.wins ?? 0}W {activeAccount.losses ?? 0}L {activeAccount.draws ?? 0}D
              </div>
              <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                Last seen: {formatDateTime(activeAccount.lastSeenAt)}
              </div>
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Season view</div>
              {highlightedSeason ? (
                <>
                  <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800 }}>
                    {highlightedSeason.label}
                  </div>
                  <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px' }}>
                    {highlightedSeason.matchesPlayed} matches - {highlightedSeason.wins}W {highlightedSeason.losses}L {highlightedSeason.draws}D - peak {highlightedSeason.peakRating}
                  </div>
                  <div style={{ color: '#7dffb4', fontSize: '13px', fontWeight: 700 }}>
                    Season delta {formatRatingDelta(highlightedSeason.netDelta)} to {highlightedSeason.ratingEnd}
                  </div>
                </>
              ) : (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No season summary exists yet because this account has not completed an account-owned rated result.
                </div>
              )}
              {displayedSeasonHistory.length > 0 && (
                <div style={{ display: 'grid', gap: '8px', marginTop: '4px' }}>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <button
                      onClick={() => setSelectedSeasonId('')}
                      style={{
                        padding: '7px 10px',
                        borderRadius: '999px',
                        border: selectedSeasonId === '' ? '1px solid rgba(255,180,60,0.38)' : '1px solid rgba(255,180,60,0.18)',
                        background: selectedSeasonId === '' ? 'rgba(255,180,60,0.14)' : 'rgba(255,255,255,0.03)',
                        color: '#ffe9b1',
                        fontSize: '11px',
                        fontWeight: 800,
                        cursor: 'pointer',
                      }}
                    >
                      All seasons
                    </button>
                    {displayedSeasonHistory.map((season) => (
                      <button
                        key={`season-chip-${season.seasonId}`}
                        onClick={() => setSelectedSeasonId(season.seasonId)}
                        style={{
                          padding: '7px 10px',
                          borderRadius: '999px',
                          border: selectedSeasonId === season.seasonId ? '1px solid rgba(255,180,60,0.38)' : '1px solid rgba(255,180,60,0.18)',
                          background: selectedSeasonId === season.seasonId ? 'rgba(255,180,60,0.14)' : 'rgba(255,255,255,0.03)',
                          color: '#ffe9b1',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        {season.label}
                      </button>
                    ))}
                  </div>
                  {displayedSeasonHistory.map((season) => (
                    <SeasonSummaryRow key={season.seasonId} summary={season} />
                  ))}
                </div>
              )}
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>Rating history</div>
              {displayedRatingHistory.length === 0 ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No account-owned rating changes have been recorded yet.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '8px' }}>
                  {displayedRatingHistory.map((entry) => (
                    <RatingHistoryRow key={`${entry.matchId}-${entry.at}`} entry={entry} />
                  ))}
                </div>
              )}
            </div>
          )}

          {activeAccount && (
            <div style={{
              borderRadius: '14px',
              border: '1px solid rgba(255,180,60,0.18)',
              background: 'rgba(255,255,255,0.03)',
              padding: '16px',
              display: 'grid',
              gap: '10px',
            }}>
              <div style={{ color: '#fff2c8', fontSize: '16px', fontWeight: 800 }}>
                {selectedSeasonId ? 'Season matches' : 'Recent account matches'}
              </div>
              {recentMatchesLoading ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>Loading archived matches...</div>
              ) : recentMatchesError ? (
                <div style={{ color: '#ffd6d6', fontSize: '13px', fontWeight: 700 }}>{recentMatchesError}</div>
              ) : recentMatches.length === 0 ? (
                <div style={{ color: 'rgba(244,232,200,0.68)', fontSize: '13px' }}>
                  No archived matches are linked to this account yet.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '8px' }}>
                  {recentMatches.map((entry) => (
                    <div
                      key={entry.matchId}
                      style={{
                        borderRadius: '12px',
                        border: '1px solid rgba(255,180,60,0.12)',
                        background: 'rgba(255,255,255,0.022)',
                        padding: '12px 14px',
                        display: 'grid',
                        gap: '5px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
                        <div style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>
                          {describeOutcome(entry, activeAccount.accountId)} vs {describeOpponent(entry, activeAccount.accountId)}
                        </div>
                        <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
                          {entry.queue ?? 'direct'} · {entry.status}
                        </div>
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '12px' }}>
                        {entry.moveCount} moves · updated {formatDateTime(entry.updatedAt)}
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px', fontFamily: 'monospace' }}>
                        {entry.matchId}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <div style={{ display: 'grid', gap: '10px' }}>
            <label style={{ display: 'grid', gap: '6px' }}>
              <span style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 700, letterSpacing: '0.6px', textTransform: 'uppercase' }}>
                {activeAccount ? 'Renew using handle' : 'Claim handle'}
              </span>
              <input
                value={handle}
                onChange={(event) => setHandle(event.target.value)}
                placeholder="aurora_fox"
                style={{
                  width: '100%',
                  padding: '12px 14px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.28)',
                  background: 'rgba(8,10,18,0.92)',
                  color: '#fff6df',
                  fontSize: '14px',
                  outline: 'none',
                }}
              />
            </label>
            <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
              Handles are lowercase, 3-24 characters, and can use letters, numbers, `_`, or `-`.
            </div>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              <button
                onClick={() => void submitClaim()}
                disabled={busy || !handle.trim()}
                style={{
                  padding: '11px 16px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,190,70,0.48)',
                  background: busy || !handle.trim()
                    ? 'rgba(120,90,35,0.38)'
                    : 'linear-gradient(180deg, rgba(200,134,10,0.92) 0%, rgba(122,79,8,0.95) 100%)',
                  color: '#fff7e3',
                  fontSize: '13px',
                  fontWeight: 800,
                  cursor: busy || !handle.trim() ? 'not-allowed' : 'pointer',
                }}
              >
                {busy ? 'Working...' : activeAccount ? 'Renew Account Session' : 'Claim Account'}
              </button>
              {accountSession && (
                <button
                  onClick={() => void renewSession()}
                  disabled={busy}
                  style={{
                    padding: '11px 16px',
                    borderRadius: '10px',
                    border: '1px solid rgba(255,180,60,0.3)',
                    background: 'transparent',
                    color: '#ffe9b1',
                    fontSize: '13px',
                    fontWeight: 700,
                    cursor: busy ? 'not-allowed' : 'pointer',
                  }}
                >
                  Refresh Session
                </button>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function MetaTile({ label, value }: { label: string; value: string }): React.ReactElement {
  return (
    <div style={{
      padding: '12px 14px',
      borderRadius: '12px',
      border: '1px solid rgba(255,180,60,0.12)',
      background: 'rgba(255,255,255,0.025)',
      minWidth: 0,
    }}>
      <div style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase', marginBottom: '6px' }}>{label}</div>
      <div style={{ color: '#fff3d1', fontSize: '13px', fontWeight: 700, wordBreak: 'break-word' }}>{value}</div>
    </div>
  );
}

function RatingHistoryRow({ entry }: { entry: AccountRatingHistoryEntry }): React.ReactElement {
  const badgeColor =
    entry.result === 'win'
      ? 'rgba(88,180,126,0.22)'
      : entry.result === 'loss'
        ? 'rgba(220,80,80,0.22)'
        : 'rgba(140,160,190,0.18)';
  const badgeBorder =
    entry.result === 'win'
      ? 'rgba(88,180,126,0.35)'
      : entry.result === 'loss'
        ? 'rgba(220,80,80,0.35)'
        : 'rgba(160,176,204,0.26)';
  const deltaColor = entry.delta > 0 ? '#7dffb4' : entry.delta < 0 ? '#ff9d9d' : '#e5d9b7';

  return (
    <div
      style={{
        borderRadius: '12px',
        border: '1px solid rgba(255,180,60,0.12)',
        background: 'rgba(255,255,255,0.022)',
        padding: '12px 14px',
        display: 'grid',
        gap: '6px',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
          <span
            style={{
              padding: '4px 8px',
              borderRadius: '999px',
              border: `1px solid ${badgeBorder}`,
              background: badgeColor,
              color: '#fff4d6',
              fontSize: '11px',
              fontWeight: 800,
              letterSpacing: '0.8px',
            }}
          >
            {entry.result.toUpperCase()}
          </span>
          <span style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>
            {formatRatingDelta(entry.delta)} to {entry.ratingAfter}
          </span>
        </div>
        <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px' }}>
          {formatDateTime(entry.at)}
        </div>
      </div>
      <div style={{ color: 'rgba(244,232,200,0.74)', fontSize: '12px' }}>
        Started at {entry.ratingBefore}, now {entry.ratingAfter}, match #{entry.matchesPlayed}
      </div>
      <div style={{ color: deltaColor, fontSize: '12px', fontWeight: 700 }}>
        Match {entry.matchId}
        {entry.opponentAccountId ? ` - opponent ${entry.opponentAccountId}` : ''}
      </div>
    </div>
  );
}

function SeasonSummaryRow({ summary }: { summary: NonNullable<AccountProfile['currentSeason']> }): React.ReactElement {
  return (
    <div
      style={{
        borderRadius: '12px',
        border: '1px solid rgba(255,180,60,0.12)',
        background: 'rgba(255,255,255,0.022)',
        padding: '12px 14px',
        display: 'grid',
        gap: '6px',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <div style={{ color: '#fff4d6', fontSize: '14px', fontWeight: 800 }}>{summary.label}</div>
        <div style={{ color: '#7dffb4', fontSize: '12px', fontWeight: 700 }}>
          {formatRatingDelta(summary.netDelta)} to {summary.ratingEnd}
        </div>
      </div>
      <div style={{ color: 'rgba(244,232,200,0.74)', fontSize: '12px' }}>
        {summary.matchesPlayed} matches - {summary.wins}W {summary.losses}L {summary.draws}D - peak {summary.peakRating}
      </div>
      <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
        {formatDateTime(summary.startedAt)} to {formatDateTime(summary.lastPlayedAt)}
      </div>
    </div>
  );
}

export default function AccountPage({
  whiteProfile = null,
  blackProfile = null,
}: AccountPageProps): React.ReactElement {
  return (
    <div style={{
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: '24px 28px 30px',
      color: '#f4e8c8',
    }}>
      <div style={{ marginBottom: '22px' }}>
        <div style={{ color: '#ffb830', fontSize: '11px', fontWeight: 800, letterSpacing: '2px', textTransform: 'uppercase', marginBottom: '6px' }}>Account Layer</div>
        <h2 style={{ margin: 0, fontSize: '30px', color: '#fff4d6' }}>Guest-to-account upgrade</h2>
        <div style={{ color: 'rgba(222, 210, 180, 0.72)', fontSize: '13px', marginTop: '8px', maxWidth: '760px' }}>
          This is the first real account slice on top of guest identity. Each seat can claim a reusable handle and keep a renewable local account session without changing the live match flow yet.
        </div>
      </div>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))',
        gap: '18px',
      }}>
        <AccountSeatPanel side="white" label="White Seat Account" accent="rgba(255,210,120,0.34)" guestProfile={whiteProfile} />
        <AccountSeatPanel side="black" label="Black Seat Account" accent="rgba(158,120,255,0.34)" guestProfile={blackProfile} />
      </div>
    </div>
  );
}
