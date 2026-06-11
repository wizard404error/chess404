'use client';

import React from 'react';
import { OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { MatchModeId } from '@chess404/contracts';
import {
  blockAccount,
  fetchAccountArchivedMatches,
  fetchAccountByHandle,
  fetchAccountLeaderboard,
  fetchModerationOverview,
  submitPlayerReport,
  unblockAccount,
  type AccountProfile,
  type AccountBlockView,
  type AccountLeaderboardSummary,
  type AccountSeasonSummary,
  type MatchArchiveEntry,
  type ModerationOverview,
  type PlayerReportView,
  type SeasonOption,
} from './lib/platform-service';
import { formatDateTime, formatRatingDelta } from './lib/display';

interface ProfilesPageProps {
  focusHandle?: string | null;
  viewerHandle?: string | null;
  accountId?: string | null;
  sessionToken?: string | null;
  onSelectHandle?: (handle: string) => void;
  onOpenReplay?: (matchId: string) => void;
  onOpenAccount?: () => void;
}

function parseModeFilterValue(value: string): MatchModeId | '' {
  return OFFICIAL_MATCH_MODES.some((mode) => mode.id === value as MatchModeId) ? (value as MatchModeId) : '';
}

function describeSeason(summary?: AccountSeasonSummary): string {
  if (!summary) {
    return 'No official season record yet';
  }
  return `${summary.label}: ${summary.matchesPlayed} matches, ${formatRatingDelta(summary.netDelta)}`;
}

function formatWinRate(wins: number, matchesPlayed: number): string {
  if (matchesPlayed <= 0) {
    return '--';
  }
  return `${Math.round((wins / matchesPlayed) * 100)}%`;
}

function describeMatchOutcome(entry: MatchArchiveEntry, accountId: string): string {
  if (entry.winner === 'draw') {
    return 'Draw';
  }
  if (entry.whiteAccountId === accountId) {
    return entry.winner === 'white' ? 'Win' : 'Loss';
  }
  if (entry.blackAccountId === accountId) {
    return entry.winner === 'black' ? 'Win' : 'Loss';
  }
  return entry.status === 'active' ? 'Live' : 'Archived';
}

function describeOpponent(entry: MatchArchiveEntry, accountId: string): string {
  if (entry.whiteAccountId === accountId) {
    return entry.blackAccountHandle ? `@${entry.blackAccountHandle}` : (entry.blackName ?? entry.blackGuestId ?? 'Unknown');
  }
  if (entry.blackAccountId === accountId) {
    return entry.whiteAccountHandle ? `@${entry.whiteAccountHandle}` : (entry.whiteName ?? entry.whiteGuestId ?? 'Unknown');
  }
  return entry.whiteName ?? entry.blackName ?? 'Unknown opponent';
}

export default function ProfilesPage({
  focusHandle = null,
  viewerHandle = null,
  accountId = null,
  sessionToken = null,
  onSelectHandle,
  onOpenReplay,
  onOpenAccount,
}: ProfilesPageProps): React.ReactElement {
  const [searchInput, setSearchInput] = React.useState('');
  const [submittedQuery, setSubmittedQuery] = React.useState('');
  const [selectedModeId, setSelectedModeId] = React.useState<MatchModeId | ''>('');
  const [selectedSeasonId, setSelectedSeasonId] = React.useState('');
  const [directory, setDirectory] = React.useState<AccountProfile[]>([]);
  const [seasons, setSeasons] = React.useState<SeasonOption[]>([]);
  const [directorySummary, setDirectorySummary] = React.useState<AccountLeaderboardSummary | undefined>(undefined);
  const [directoryLoading, setDirectoryLoading] = React.useState(true);
  const [directoryError, setDirectoryError] = React.useState('');
  const [profile, setProfile] = React.useState<AccountProfile | null>(null);
  const [profileLoading, setProfileLoading] = React.useState(false);
  const [profileError, setProfileError] = React.useState('');
  const [recentMatches, setRecentMatches] = React.useState<MatchArchiveEntry[]>([]);
  const [recentMatchesLoading, setRecentMatchesLoading] = React.useState(false);
  const [recentMatchesError, setRecentMatchesError] = React.useState('');
  const [moderationOverview, setModerationOverview] = React.useState<ModerationOverview | null>(null);
  const [moderationLoading, setModerationLoading] = React.useState(false);
  const [moderationError, setModerationError] = React.useState('');
  const [moderationBusy, setModerationBusy] = React.useState(false);
  const [reportCategory, setReportCategory] = React.useState('abuse');
  const [reportDetails, setReportDetails] = React.useState('');
  const [notice, setNotice] = React.useState('');

  const resolvedFocusHandle = (focusHandle ?? '').trim().toLowerCase();
  const authenticatedViewer = Boolean(accountId && sessionToken);

  React.useEffect(() => {
    if (!resolvedFocusHandle) {
      return;
    }
    setSearchInput(resolvedFocusHandle);
  }, [resolvedFocusHandle]);

  const refreshDirectory = React.useCallback(async (query: string, modeId: MatchModeId | '') => {
    setDirectoryLoading(true);
    setDirectoryError('');
    setDirectorySummary(undefined);
    try {
      const payload = await fetchAccountLeaderboard(40, 'rating', undefined, modeId || undefined, query || undefined);
      setDirectory(payload.accounts);
      setSeasons(payload.seasons);
      setDirectorySummary(payload.summary);
    } catch (err) {
      setDirectoryError(err instanceof Error ? err.message : 'Failed to load public profiles.');
      setDirectorySummary(undefined);
    } finally {
      setDirectoryLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refreshDirectory(submittedQuery, selectedModeId);
  }, [refreshDirectory, selectedModeId, submittedQuery]);

  React.useEffect(() => {
    if (!resolvedFocusHandle) {
      setProfile(null);
      setProfileError('');
      setSelectedSeasonId('');
      return;
    }

    let cancelled = false;
    setProfileLoading(true);
    setProfileError('');

    void fetchAccountByHandle(resolvedFocusHandle, selectedSeasonId || undefined, selectedModeId || undefined)
      .then((nextProfile) => {
        if (cancelled) {
          return;
        }
        setProfile(nextProfile);
        if (selectedSeasonId && !nextProfile.selectedSeason) {
          setSelectedSeasonId('');
        }
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return;
        }
        setProfile(null);
        setProfileError(err instanceof Error ? err.message : 'Failed to load public profile.');
      })
      .finally(() => {
        if (!cancelled) {
          setProfileLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [resolvedFocusHandle, selectedModeId, selectedSeasonId]);

  React.useEffect(() => {
    if (!profile?.accountId) {
      setRecentMatches([]);
      setRecentMatchesLoading(false);
      setRecentMatchesError('');
      return;
    }

    let cancelled = false;
    setRecentMatchesLoading(true);
    setRecentMatchesError('');

    void fetchAccountArchivedMatches(profile.accountId, 8, selectedSeasonId || undefined, selectedModeId || undefined)
      .then((matches) => {
        if (!cancelled) {
          setRecentMatches(matches);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setRecentMatches([]);
          setRecentMatchesError(err instanceof Error ? err.message : 'Failed to load public profile matches.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setRecentMatchesLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [profile?.accountId, selectedModeId, selectedSeasonId]);

  React.useEffect(() => {
    if (!authenticatedViewer || !accountId || !sessionToken || !profile?.accountId || profile.accountId === accountId) {
      setModerationOverview(null);
      setModerationLoading(false);
      setModerationError('');
      return;
    }

    let cancelled = false;
    setModerationLoading(true);
    setModerationError('');

    void fetchModerationOverview({ accountId, sessionToken })
      .then((overview) => {
        if (!cancelled) {
          setModerationOverview(overview);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setModerationOverview(null);
          setModerationError(err instanceof Error ? err.message : 'Failed to load trust controls.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setModerationLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountId, authenticatedViewer, profile?.accountId, sessionToken]);

  const openHandle = React.useCallback((handle: string) => {
    const normalized = handle.trim().toLowerCase();
    if (!normalized) {
      return;
    }
    setProfileError('');
    setNotice('');
    onSelectHandle?.(normalized);
  }, [onSelectHandle]);

  const submitDirectorySearch = React.useCallback(() => {
    setSubmittedQuery(searchInput.trim().toLowerCase());
  }, [searchInput]);

  const copyProfileLink = React.useCallback(async () => {
    if (!profile || typeof window === 'undefined') {
      return;
    }
    const profileUrl = `${window.location.origin}/?profile=${encodeURIComponent(profile.handle)}`;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(profileUrl);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = profileUrl;
        textArea.style.position = 'fixed';
        textArea.style.opacity = '0';
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
      }
      setNotice('Profile link copied.');
    } catch {
      setNotice('Copy failed. You can still share the current profile URL from the address bar.');
    }
  }, [profile]);

  const outgoingBlock = React.useMemo<AccountBlockView | null>(() => {
    if (!profile?.accountId) {
      return null;
    }
    return moderationOverview?.outgoingBlocks.find((block) => block.account.accountId === profile.accountId) ?? null;
  }, [moderationOverview, profile?.accountId]);

  const incomingBlock = React.useMemo<AccountBlockView | null>(() => {
    if (!profile?.accountId) {
      return null;
    }
    return moderationOverview?.incomingBlocks.find((block) => block.account.accountId === profile.accountId) ?? null;
  }, [moderationOverview, profile?.accountId]);

  const submittedReportsForFocus = React.useMemo<PlayerReportView[]>(() => {
    if (!profile?.accountId) {
      return [];
    }
    return moderationOverview?.submittedReports.filter((report) => report.target.accountId === profile.accountId) ?? [];
  }, [moderationOverview, profile?.accountId]);

  const handleBlockToggle = React.useCallback(async () => {
    if (!accountId || !sessionToken || !profile?.accountId || moderationBusy) {
      return;
    }
    setModerationBusy(true);
    setModerationError('');
    setNotice('');
    try {
      const nextOverview = outgoingBlock
        ? await unblockAccount({ accountId, sessionToken, targetAccountId: profile.accountId })
        : await blockAccount({
            accountId,
            sessionToken,
            targetAccountId: profile.accountId,
            reason: submittedReportsForFocus.length > 0 ? 'Escalated from prior reports' : '',
          });
      setModerationOverview(nextOverview);
      setNotice(outgoingBlock ? `Unblocked @${profile.handle}` : `Blocked @${profile.handle}`);
    } catch (err) {
      setModerationError(err instanceof Error ? err.message : 'Failed to update account block.');
    } finally {
      setModerationBusy(false);
    }
  }, [accountId, moderationBusy, outgoingBlock, profile?.accountId, profile?.handle, sessionToken, submittedReportsForFocus.length]);

  const handleSubmitReport = React.useCallback(async () => {
    if (!accountId || !sessionToken || !profile?.accountId || moderationBusy) {
      return;
    }
    setModerationBusy(true);
    setModerationError('');
    setNotice('');
    try {
      const nextOverview = await submitPlayerReport({
        accountId,
        sessionToken,
        targetAccountId: profile.accountId,
        category: reportCategory,
        details: reportDetails.trim(),
      });
      setModerationOverview(nextOverview);
      setReportDetails('');
      setNotice(`Report submitted for @${profile.handle}`);
    } catch (err) {
      setModerationError(err instanceof Error ? err.message : 'Failed to submit player report.');
    } finally {
      setModerationBusy(false);
    }
  }, [accountId, moderationBusy, profile?.accountId, profile?.handle, reportCategory, reportDetails, sessionToken]);

  const focusedOwnProfile = Boolean(accountId && profile?.accountId && profile.accountId === accountId);

  const highlightedSeason = profile?.selectedSeason ?? profile?.currentSeason;
  const highlightedMatchesPlayed = highlightedSeason?.matchesPlayed ?? profile?.matchesPlayed ?? 0;
  const highlightedWins = highlightedSeason?.wins ?? profile?.wins ?? 0;
  const highlightedPeak = highlightedSeason?.peakRating ?? profile?.rating ?? 1200;
  const highlightedDelta = highlightedSeason?.netDelta ?? 0;
  const recentForm = React.useMemo(() => {
    const history = profile?.ratingHistory ?? [];
    if (history.length === 0) {
      return 'No recent rated results';
    }
    return history.slice(-5).map((entry) => {
      switch (entry.result) {
        case 'win':
          return 'W';
        case 'loss':
          return 'L';
        default:
          return 'D';
      }
    }).join(' · ');
  }, [profile?.ratingHistory]);
  const spotlightBadges = React.useMemo(() => {
    if (!profile?.accountId || !directorySummary) {
      return [] as string[];
    }
    const badges: string[] = [];
    if (directorySummary.leader?.accountId === profile.accountId) {
      badges.push('Current leader');
    }
    if (directorySummary.biggestClimber?.accountId === profile.accountId) {
      badges.push('Biggest climb');
    }
    if (directorySummary.highestPeak?.accountId === profile.accountId) {
      badges.push('Peak holder');
    }
    if (directorySummary.mostActive?.accountId === profile.accountId) {
      badges.push('Most active');
    }
    return badges;
  }, [directorySummary, profile?.accountId]);

  return (
    <div className={profile ? 'profile-shell' : 'profile-shell profile-shell--stacked'}>
      <div
        className="stat-card"
        style={{
          minWidth: 0,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          padding: 0,
          overflow: 'hidden',
        }}
      >
        <div style={{ padding: '18px 20px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Public Profiles</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px', lineHeight: 1.5 }}>
            Search claimed handles, inspect official-mode ladders, and open shareable public profile views for Chess404 players.
          </div>

          <div style={{ display: 'grid', gap: '10px', marginTop: '14px' }}>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                aria-label="Search by handle"
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    submitDirectorySearch();
                  }
                }}
                placeholder="Search by handle"
                style={{
                  flex: 1,
                  padding: '10px 12px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.24)',
                  background: 'rgba(255,255,255,0.04)',
                  color: '#fff2c8',
                  fontSize: '12px',
                  fontWeight: 700,
                  outline: 'none',
                }}
              />
              <button
                className="btn-primary"
                onClick={submitDirectorySearch}
                style={{ padding: '10px 12px' }}
              >
                Search
              </button>
            </div>

            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
              {viewerHandle && (
                <button
                  onClick={() => openHandle(viewerHandle)}
                  style={{
                    padding: '8px 10px',
                    borderRadius: '999px',
                    border: '1px solid rgba(255,180,60,0.22)',
                    background: 'rgba(255,180,60,0.08)',
                    color: '#ffe7a9',
                    fontSize: '11px',
                    fontWeight: 800,
                    cursor: 'pointer',
                  }}
                >
                  Open my profile
                </button>
              )}
              {resolvedFocusHandle && (
                <button
                  onClick={() => openHandle(resolvedFocusHandle)}
                  style={{
                    padding: '8px 10px',
                    borderRadius: '999px',
                    border: '1px solid rgba(255,180,60,0.16)',
                    background: 'rgba(255,255,255,0.03)',
                    color: '#fff2c8',
                    fontSize: '11px',
                    fontWeight: 800,
                    cursor: 'pointer',
                  }}
                >
                  Refresh focus
                </button>
              )}
            </div>

            <select
              aria-label="Filter by mode"
              value={selectedModeId}
              onChange={(event) => setSelectedModeId(parseModeFilterValue(event.target.value))}
              style={{
                padding: '9px 10px',
                borderRadius: '10px',
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
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {directoryError && (
            <div style={{
              marginBottom: '14px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(120,20,20,0.22)',
              border: '1px solid rgba(231,76,60,0.32)',
              color: '#ffb1a7',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {directoryError}
            </div>
          )}

          {directoryLoading ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading public profiles...</div>
          ) : directory.length === 0 ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px', lineHeight: 1.6 }}>
              {submittedQuery
                ? 'No claimed handles match that search yet.'
                : 'No claimed public profiles exist yet. Open the Account tab to claim a handle.'}
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {directory.map((account) => {
                const season = account.selectedSeason ?? account.currentSeason;
                const focused = resolvedFocusHandle === account.handle;
                return (
                  <button
                    key={account.accountId}
                    className={`table-row ${focused ? 'stat-card' : ''}`}
                    onClick={() => openHandle(account.handle)}
                    style={{
                      textAlign: 'left',
                      cursor: 'pointer',
                      borderRadius: '14px',
                      border: focused ? '1px solid rgba(255,190,90,0.34)' : '1px solid rgba(255,165,40,0.12)',
                      background: focused
                        ? 'linear-gradient(180deg, rgba(200,134,10,0.2) 0%, rgba(70,42,8,0.22) 100%)'
                        : 'rgba(255,255,255,0.03)',
                      padding: '14px 15px 13px',
                      display: 'grid',
                      gap: '8px',
                      color: 'inherit',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'flex-start' }}>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ color: '#fff2c8', fontSize: '14px', fontWeight: 800, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {account.displayName ?? account.handle}
                        </div>
                        <div style={{ color: '#ffd98f', fontSize: '11px', fontWeight: 700, marginTop: '4px' }}>
                          @{account.handle}
                        </div>
                      </div>
                      <div style={{ color: '#7ce3aa', fontSize: '16px', fontWeight: 900, flexShrink: 0 }}>
                        {account.selectedSeason?.ratingEnd ?? account.rating ?? 1200}
                      </div>
                    </div>
                    <div style={{ color: 'rgba(170,190,220,0.62)', fontSize: '11px' }}>
                      {account.matchesPlayed ?? 0} matches - {account.wins ?? 0}W {account.losses ?? 0}L {account.draws ?? 0}D
                    </div>
                    <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '11px' }}>
                      {describeSeason(season)}
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <div
        className="stat-card"
        style={{
          flex: 1,
          minWidth: 0,
          minHeight: 0,
          display: 'flex',
          flexDirection: 'column',
          padding: 0,
          overflow: 'hidden',
        }}
      >
        <div style={{ padding: '18px 20px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Profile Focus</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px', lineHeight: 1.5 }}>
            Public handles tie rankings, replay history, and future social identity into one player-facing destination.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {profileError && (
            <div style={{
              marginBottom: '14px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(120,20,20,0.22)',
              border: '1px solid rgba(231,76,60,0.32)',
              color: '#ffb1a7',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {profileError}
            </div>
          )}

          {notice && (
            <div style={{
              marginBottom: '14px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(28,64,42,0.34)',
              border: '1px solid rgba(88,180,126,0.35)',
              color: '#d8ffe7',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {notice}
            </div>
          )}

          {moderationError && (
            <div style={{
              marginBottom: '14px',
              padding: '12px 14px',
              borderRadius: '10px',
              background: 'rgba(120,20,20,0.22)',
              border: '1px solid rgba(231,76,60,0.32)',
              color: '#ffb1a7',
              fontSize: '12px',
              fontWeight: 700,
            }}>
              {moderationError}
            </div>
          )}

          {profileLoading ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>Loading public profile...</div>
          ) : !profile ? (
            <div className="empty-state">
              <div>
                <div className="empty-state__icon">♟</div>
                <div className="empty-state__title">
                  {resolvedFocusHandle ? 'That handle is not live yet' : 'Choose a player profile'}
                </div>
                <div className="empty-state__body">
                  {resolvedFocusHandle
                    ? 'The requested handle could not be resolved to a claimed Chess404 account. Try another search or open a visible ladder profile from the directory.'
                    : 'Open a claimed handle from the directory to see ratings, season momentum, spotlight badges, and recent replay history in one shareable player page.'}
                </div>
                {onOpenAccount ? (
                  <div style={{ display: 'flex', justifyContent: 'center', marginTop: '18px' }}>
                    <button
                      className="btn-primary"
                      onClick={onOpenAccount}
                      style={{ padding: '11px 16px' }}
                    >
                      Open Account
                    </button>
                  </div>
                ) : null}
              </div>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div style={{ padding: '18px', borderRadius: '16px', background: 'linear-gradient(180deg, rgba(200,134,10,0.18) 0%, rgba(70,42,8,0.22) 100%)', border: '1px solid rgba(255,185,70,0.18)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '14px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ color: '#fff2c8', fontSize: '24px', fontWeight: 900 }}>{profile.displayName ?? profile.handle}</div>
                    <div style={{ color: '#ffd98f', fontSize: '13px', fontWeight: 800, marginTop: '6px' }}>@{profile.handle}</div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px', marginTop: '8px' }}>
                      Joined {formatDateTime(profile.createdAt)} · last seen {formatDateTime(profile.lastSeenAt)}
                    </div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ color: '#7ce3aa', fontSize: '30px', fontWeight: 900 }}>{highlightedSeason?.ratingEnd ?? profile.rating ?? 1200}</div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px', marginTop: '6px' }}>
                      {describeSeason(highlightedSeason)}
                    </div>
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '14px' }}>
                  <button
                    onClick={() => void copyProfileLink()}
                    style={{
                      padding: '8px 10px',
                      borderRadius: '999px',
                      border: '1px solid rgba(255,210,120,0.28)',
                      background: 'rgba(255,200,100,0.14)',
                      color: '#ffe8af',
                      fontSize: '11px',
                      fontWeight: 800,
                      cursor: 'pointer',
                    }}
                  >
                    Copy profile link
                  </button>
                  {viewerHandle && viewerHandle === profile.handle && (
                    <span style={{
                      padding: '8px 10px',
                      borderRadius: '999px',
                      border: '1px solid rgba(110,170,255,0.22)',
                      background: 'rgba(80,140,255,0.18)',
                      color: '#dcecff',
                      fontSize: '11px',
                      fontWeight: 800,
                    }}>
                      Your claimed handle
                    </span>
                  )}
                  {spotlightBadges.map((badge) => (
                    <span
                      key={badge}
                      style={{
                        padding: '8px 10px',
                        borderRadius: '999px',
                        border: '1px solid rgba(136,214,255,0.24)',
                        background: 'rgba(88,176,255,0.14)',
                        color: '#ddf2ff',
                        fontSize: '11px',
                        fontWeight: 800,
                      }}
                    >
                      {badge}
                    </span>
                  ))}
                </div>
              </div>

              <div style={{ padding: '16px 18px', borderRadius: '14px', background: 'rgba(255,255,255,0.035)', border: '1px solid rgba(255,165,40,0.1)', display: 'grid', gap: '12px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
                  <div>
                    <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, letterSpacing: '0.9px', textTransform: 'uppercase' }}>Competitive snapshot</div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px', marginTop: '6px', lineHeight: 1.5 }}>
                      {directorySummary?.seasonLabel ?? 'Current ladder'} · {selectedModeId ? OFFICIAL_MATCH_MODES.find((mode) => mode.id === selectedModeId)?.label ?? 'Official mode' : 'All official modes'}
                    </div>
                  </div>
                  {directorySummary?.leader?.accountId === profile.accountId && (
                    <div style={{ color: '#7ce3aa', fontSize: '12px', fontWeight: 800 }}>Leading this lane now</div>
                  )}
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '10px' }}>
                  <div style={{ padding: '12px 13px', borderRadius: '12px', background: 'rgba(255,180,60,0.08)', border: '1px solid rgba(255,180,60,0.16)' }}>
                    <div style={{ color: '#ffcf72', fontSize: '10px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Win rate</div>
                    <div style={{ color: '#fff4d2', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{formatWinRate(highlightedWins, highlightedMatchesPlayed)}</div>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', marginTop: '4px' }}>{highlightedWins} wins across {highlightedMatchesPlayed} matches</div>
                  </div>
                  <div style={{ padding: '12px 13px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.1)' }}>
                    <div style={{ color: '#ffcf72', fontSize: '10px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Peak rating</div>
                    <div style={{ color: '#fff4d2', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{highlightedPeak}</div>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', marginTop: '4px' }}>{highlightedSeason?.label ?? 'Overall ladder peak'}</div>
                  </div>
                  <div style={{ padding: '12px 13px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.1)' }}>
                    <div style={{ color: '#ffcf72', fontSize: '10px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Season delta</div>
                    <div style={{ color: highlightedDelta >= 0 ? '#7ce3aa' : '#ffb1a7', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{formatRatingDelta(highlightedDelta)}</div>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', marginTop: '4px' }}>{highlightedSeason ? highlightedSeason.label : 'No season delta yet'}</div>
                  </div>
                  <div style={{ padding: '12px 13px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.1)' }}>
                    <div style={{ color: '#ffcf72', fontSize: '10px', fontWeight: 800, letterSpacing: '0.8px', textTransform: 'uppercase' }}>Recent form</div>
                    <div style={{ color: '#fff4d2', fontSize: '18px', fontWeight: 900, marginTop: '8px' }}>{recentForm}</div>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', marginTop: '6px' }}>Last five rated decisions in this lane</div>
                  </div>
                </div>
              </div>

              <div style={{ padding: '16px 18px', borderRadius: '14px', background: 'rgba(255,255,255,0.035)', border: '1px solid rgba(255,165,40,0.1)', display: 'grid', gap: '12px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
                  <div>
                    <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, letterSpacing: '0.9px', textTransform: 'uppercase' }}>Trust & Safety</div>
                    <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px', marginTop: '6px', lineHeight: 1.5 }}>
                      Blocks shut down new friend requests and direct challenges between two accounts. Reports create a moderation record against the player profile you are viewing.
                    </div>
                  </div>
                  {moderationLoading && (
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '11px', fontWeight: 700 }}>Loading trust state...</div>
                  )}
                </div>

                {!authenticatedViewer ? (
                  <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
                    <div style={{ color: 'rgba(255,232,180,0.68)', fontSize: '12px', lineHeight: 1.5 }}>
                      Sign in with a claimed Chess404 account to block players or file reports.
                    </div>
                    {onOpenAccount && (
                      <button
                        onClick={onOpenAccount}
                        style={{
                          padding: '8px 12px',
                          borderRadius: '999px',
                          border: '1px solid rgba(255,180,60,0.26)',
                          background: 'rgba(255,180,60,0.12)',
                          color: '#ffe8af',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: 'pointer',
                        }}
                      >
                        Open account
                      </button>
                    )}
                  </div>
                ) : focusedOwnProfile ? (
                  <div style={{ color: 'rgba(255,232,180,0.68)', fontSize: '12px', lineHeight: 1.5 }}>
                    This is your own public profile. Trust controls appear when you inspect another player.
                  </div>
                ) : (
                  <>
                    {(outgoingBlock || incomingBlock) && (
                      <div style={{ display: 'grid', gap: '8px' }}>
                        {outgoingBlock && (
                          <div style={{ padding: '11px 12px', borderRadius: '10px', background: 'rgba(120,20,20,0.22)', border: '1px solid rgba(231,76,60,0.28)', color: '#ffcebf', fontSize: '12px', fontWeight: 700 }}>
                            You blocked @{profile.handle}{outgoingBlock.reason ? ` - ${outgoingBlock.reason}` : ''}.
                          </div>
                        )}
                        {incomingBlock && (
                          <div style={{ padding: '11px 12px', borderRadius: '10px', background: 'rgba(72,88,140,0.18)', border: '1px solid rgba(110,140,220,0.26)', color: '#dce8ff', fontSize: '12px', fontWeight: 700 }}>
                            @{profile.handle} has blocked you. Social actions should remain locked from either side.
                          </div>
                        )}
                      </div>
                    )}

                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                      <button
                        onClick={() => void handleBlockToggle()}
                        disabled={moderationBusy}
                        style={{
                          padding: '9px 12px',
                          borderRadius: '999px',
                          border: outgoingBlock ? '1px solid rgba(90,170,130,0.28)' : '1px solid rgba(231,76,60,0.28)',
                          background: outgoingBlock ? 'rgba(52,120,82,0.18)' : 'rgba(140,24,24,0.22)',
                          color: outgoingBlock ? '#d8ffe7' : '#ffd3c7',
                          fontSize: '11px',
                          fontWeight: 800,
                          cursor: moderationBusy ? 'wait' : 'pointer',
                          opacity: moderationBusy ? 0.7 : 1,
                        }}
                      >
                        {outgoingBlock ? 'Unblock player' : 'Block player'}
                      </button>
                      <span style={{ alignSelf: 'center', color: 'rgba(255,232,180,0.56)', fontSize: '11px', fontWeight: 700 }}>
                        {submittedReportsForFocus.length > 0 ? `${submittedReportsForFocus.length} report(s) already submitted` : 'No prior reports submitted from this account'}
                      </span>
                    </div>

                    <div style={{ display: 'grid', gap: '10px' }}>
                      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                        <select
                          aria-label="Report category"
                          value={reportCategory}
                          onChange={(event) => setReportCategory(event.target.value)}
                          style={{
                            padding: '9px 10px',
                            borderRadius: '10px',
                            border: '1px solid rgba(255,180,60,0.24)',
                            background: 'rgba(255,255,255,0.04)',
                            color: '#fff2c8',
                            fontSize: '12px',
                            fontWeight: 700,
                          }}
                        >
                          {['abuse', 'harassment', 'spam', 'impersonation', 'cheating', 'other'].map((category) => (
                            <option key={category} value={category}>
                              {category}
                            </option>
                          ))}
                        </select>
                        <button
                          onClick={() => void handleSubmitReport()}
                          disabled={moderationBusy}
                          style={{
                            padding: '9px 12px',
                            borderRadius: '999px',
                            border: '1px solid rgba(255,180,60,0.26)',
                            background: 'rgba(255,180,60,0.12)',
                            color: '#ffe8af',
                            fontSize: '11px',
                            fontWeight: 800,
                            cursor: moderationBusy ? 'wait' : 'pointer',
                            opacity: moderationBusy ? 0.7 : 1,
                          }}
                        >
                          Submit report
                        </button>
                      </div>
                      <textarea
                        value={reportDetails}
                        onChange={(event) => setReportDetails(event.target.value)}
                        placeholder="Add optional evidence or context for the moderation record."
                        rows={3}
                        style={{
                          width: '100%',
                          resize: 'vertical',
                          padding: '10px 12px',
                          borderRadius: '10px',
                          border: '1px solid rgba(255,180,60,0.18)',
                          background: 'rgba(255,255,255,0.035)',
                          color: '#fff2c8',
                          fontSize: '12px',
                          lineHeight: 1.5,
                        }}
                      />
                    </div>
                  </>
                )}
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: '12px' }}>
                {[
                  { label: 'Matches', value: profile.matchesPlayed ?? 0, color: '#d8eaff' },
                  { label: 'Wins', value: profile.wins ?? 0, color: '#8ef0b6' },
                  { label: 'Losses', value: profile.losses ?? 0, color: '#ffb3a0' },
                  { label: 'Draws', value: profile.draws ?? 0, color: '#ffe2a5' },
                ].map((stat) => (
                  <div key={stat.label} style={{ padding: '14px 14px 12px', borderRadius: '12px', background: 'rgba(255,255,255,0.035)', border: '1px solid rgba(255,165,40,0.08)' }}>
                    <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '10px', fontWeight: 700, letterSpacing: '0.8px', textTransform: 'uppercase' }}>{stat.label}</div>
                    <div style={{ color: stat.color, fontSize: '20px', fontWeight: 900, marginTop: '6px' }}>{stat.value}</div>
                  </div>
                ))}
              </div>

              {seasons.length > 0 && (
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  <button
                    onClick={() => setSelectedSeasonId('')}
                    style={{
                      padding: '8px 10px',
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
                  {seasons.map((season) => (
                    <button
                      key={season.seasonId}
                      onClick={() => setSelectedSeasonId(season.seasonId)}
                      style={{
                        padding: '8px 10px',
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
              )}

              <div style={{ padding: '16px', borderRadius: '14px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)' }}>
                <div style={{ color: '#ffcf72', fontSize: '12px', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '1px' }}>Recent account results</div>
                {recentMatchesLoading ? (
                  <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '12px', marginTop: '12px' }}>Loading archived matches...</div>
                ) : recentMatchesError ? (
                  <div style={{ color: '#ffb1a7', fontSize: '12px', fontWeight: 700, marginTop: '12px' }}>{recentMatchesError}</div>
                ) : recentMatches.length === 0 ? (
                  <div style={{ color: 'rgba(255,232,180,0.58)', fontSize: '12px', marginTop: '12px' }}>No archived account matches match the current filters yet.</div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginTop: '12px' }}>
                    {recentMatches.map((match) => (
                      <button
                        key={match.matchId}
                        onClick={() => onOpenReplay?.(match.matchId)}
                        style={{
                          textAlign: 'left',
                          cursor: onOpenReplay ? 'pointer' : 'default',
                          padding: '12px 12px 11px',
                          borderRadius: '12px',
                          border: '1px solid rgba(255,165,40,0.12)',
                          background: 'linear-gradient(180deg, rgba(18,23,36,0.95) 0%, rgba(11,14,24,0.94) 100%)',
                          color: '#fff2c8',
                        }}
                      >
                        <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
                          <div style={{ fontSize: '12px', fontWeight: 800 }}>
                            {describeMatchOutcome(match, profile.accountId)} vs {describeOpponent(match, profile.accountId)}
                          </div>
                          <div style={{ color: 'rgba(160,184,216,0.64)', fontSize: '10px' }}>
                            {match.queue ?? 'direct'} · {match.status}
                          </div>
                        </div>
                        <div style={{ marginTop: '6px', color: 'rgba(255,232,180,0.7)', fontSize: '11px' }}>
                          {match.moveCount} moves · updated {formatDateTime(match.updatedAt)}
                        </div>
                      </button>
                    ))}
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
