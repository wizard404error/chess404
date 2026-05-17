import * as React from 'react';
import {
  fetchModerationAdminOverview,
  resolveModerationReport,
  type ModerationAdminOverview,
} from './lib/platform-service';

interface AdminModerationPageProps {
  accountId?: string | null;
  sessionToken?: string | null;
  onOpenProfile?: (handle: string) => void;
}

const STATUS_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: 'All reports' },
  { value: 'open', label: 'Open' },
  { value: 'under_review', label: 'Under review' },
  { value: 'resolved_actioned', label: 'Resolved: actioned' },
  { value: 'resolved_dismissed', label: 'Resolved: dismissed' },
];

function formatDateTime(value?: string | null): string {
  if (!value) {
    return 'Unknown time';
  }
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) {
    return value;
  }
  return new Date(timestamp).toLocaleString();
}

export default function AdminModerationPage({
  accountId,
  sessionToken,
  onOpenProfile,
}: AdminModerationPageProps): React.ReactElement {
  const [overview, setOverview] = React.useState<ModerationAdminOverview | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');
  const [busyReportId, setBusyReportId] = React.useState('');
  const [filterStatus, setFilterStatus] = React.useState('');
  const [notes, setNotes] = React.useState<Record<string, string>>({});

  const loadOverview = React.useCallback(async (nextStatus = filterStatus) => {
    if (!accountId || !sessionToken) {
      setOverview(null);
      setError('');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const next = await fetchModerationAdminOverview({
        accountId,
        sessionToken,
        limit: 24,
        status: nextStatus,
      });
      setOverview(next);
      setFilterStatus(next.selectedStatus ?? nextStatus);
    } catch (err) {
      setOverview(null);
      setError(err instanceof Error ? err.message : 'Failed to load moderation admin queue.');
    } finally {
      setLoading(false);
    }
  }, [accountId, filterStatus, sessionToken]);

  React.useEffect(() => {
    void loadOverview(filterStatus);
  }, [loadOverview, filterStatus]);

  const handleResolve = React.useCallback(async (
    reportId: string,
    action: 'under_review' | 'resolved_actioned' | 'resolved_dismissed',
    restriction?: 'suspended' | 'banned' | 'clear'
  ) => {
    if (!accountId || !sessionToken) {
      return;
    }
    setBusyReportId(reportId);
    setError('');
    try {
      const next = await resolveModerationReport({
        accountId,
        sessionToken,
        reportId,
        action,
        restriction,
        note: notes[reportId] ?? '',
        limit: 24,
        status: filterStatus,
      });
      setOverview(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update moderation review.');
    } finally {
      setBusyReportId('');
    }
  }, [accountId, filterStatus, notes, sessionToken]);

  const canManage = Boolean(accountId && sessionToken);

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '28px 30px 40px' }}>
      <div style={{ display: 'grid', gap: '10px', marginBottom: '24px' }}>
        <h2 style={{ margin: 0, fontSize: '30px', color: '#fff4d6' }}>Moderation Admin</h2>
        <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '14px', maxWidth: '920px', lineHeight: 1.6 }}>
          This review queue turns player reports into a real launch-grade moderation workflow. Admins can triage reports, move them into review, resolve them with action or dismissal, and apply real suspensions or bans when needed.
        </div>
      </div>

      {!canManage ? (
        <div style={{
          padding: '16px 18px',
          borderRadius: '16px',
          border: '1px solid rgba(255,180,60,0.18)',
          background: 'rgba(255,255,255,0.03)',
          color: 'rgba(244,232,200,0.74)',
        }}>
          Sign in with an account session on this device to load the moderation admin queue.
        </div>
      ) : (
        <>
          <div style={{
            display: 'flex',
            gap: '12px',
            flexWrap: 'wrap',
            alignItems: 'center',
            marginBottom: '18px',
            padding: '16px 18px',
            borderRadius: '16px',
            border: '1px solid rgba(255,180,60,0.18)',
            background: 'rgba(255,255,255,0.03)',
          }}>
            <label style={{ display: 'grid', gap: '6px', minWidth: '220px' }}>
              <span style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
                Queue filter
              </span>
              <select
                value={filterStatus}
                onChange={(event) => setFilterStatus(event.target.value)}
                style={{
                  padding: '10px 12px',
                  borderRadius: '10px',
                  border: '1px solid rgba(255,180,60,0.22)',
                  background: 'rgba(12,8,24,0.8)',
                  color: '#fff3d3',
                }}
              >
                {STATUS_OPTIONS.map((option) => (
                  <option key={option.value || 'all'} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <button
              onClick={() => void loadOverview(filterStatus)}
              disabled={loading}
              style={{
                padding: '11px 16px',
                borderRadius: '10px',
                border: '1px solid rgba(255,190,70,0.38)',
                background: loading ? 'rgba(120,90,35,0.28)' : 'linear-gradient(180deg, rgba(200,134,10,0.9) 0%, rgba(122,80,8,0.96) 100%)',
                color: '#fff7e3',
                fontWeight: 800,
                cursor: loading ? 'not-allowed' : 'pointer',
                marginTop: '18px',
              }}
            >
              {loading ? 'Refreshing…' : 'Refresh queue'}
            </button>
            {overview?.viewer?.handle ? (
              <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '18px' }}>
                Signed in as <strong style={{ color: '#fff3d3' }}>@{overview.viewer.handle}</strong>
              </div>
            ) : null}
          </div>

          {error ? (
            <div style={{
              marginBottom: '18px',
              padding: '14px 16px',
              borderRadius: '14px',
              border: '1px solid rgba(255,120,120,0.24)',
              background: 'rgba(90,24,24,0.28)',
              color: '#ffd7d7',
              fontWeight: 700,
            }}>
              {error}
            </div>
          ) : null}

          <div style={{ display: 'grid', gap: '24px' }}>
            <section style={{ display: 'grid', gap: '14px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
                <div>
                  <div style={{ color: '#fff2c8', fontSize: '18px', fontWeight: 800 }}>Report queue</div>
                  <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                    Reports here are authoritative backend records, not browser-only flags.
                  </div>
                </div>
                <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px' }}>
                  {overview ? `${overview.reports.length} visible report${overview.reports.length === 1 ? '' : 's'}` : 'No queue loaded yet'}
                </div>
              </div>

              {(overview?.reports.length ?? 0) === 0 ? (
                <div style={{
                  padding: '18px',
                  borderRadius: '16px',
                  border: '1px solid rgba(255,180,60,0.16)',
                  background: 'rgba(255,255,255,0.03)',
                  color: 'rgba(244,232,200,0.7)',
                }}>
                  {loading ? 'Loading moderation reports…' : 'No reports match the current filter.'}
                </div>
              ) : (
                overview?.reports.map((report) => (
                  <article
                    key={report.reportId}
                    style={{
                      padding: '18px',
                      borderRadius: '18px',
                      border: '1px solid rgba(255,180,60,0.18)',
                      background: 'rgba(255,255,255,0.03)',
                      display: 'grid',
                      gap: '14px',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', alignItems: 'flex-start' }}>
                      <div style={{ display: 'grid', gap: '8px' }}>
                        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
                          <span style={{
                            padding: '5px 10px',
                            borderRadius: '999px',
                            background: 'rgba(255,180,60,0.16)',
                            border: '1px solid rgba(255,180,60,0.22)',
                            color: '#ffcf72',
                            fontSize: '11px',
                            fontWeight: 800,
                            letterSpacing: '0.9px',
                            textTransform: 'uppercase',
                          }}>
                            {report.status.replace(/_/g, ' ')}
                          </span>
                          <span style={{ color: '#fff4d6', fontWeight: 800 }}>{report.category}</span>
                        </div>
                        <div style={{ color: '#fff7e6', fontSize: '15px', lineHeight: 1.6 }}>
                          {report.details?.trim() || 'No additional detail was provided by the reporting player.'}
                        </div>
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.64)', fontSize: '12px', textAlign: 'right' }}>
                        <div>Reported {formatDateTime(report.createdAt)}</div>
                        <div>Updated {formatDateTime(report.updatedAt)}</div>
                      </div>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px' }}>
                      <ProfileMiniCard title="Reporter" handle={report.reporter.handle} presence={report.reporter.presenceStatus} onOpenProfile={onOpenProfile} />
                      <ProfileMiniCard title="Target" handle={report.target.handle} presence={report.target.presenceStatus} onOpenProfile={onOpenProfile} />
                      <ProfileMiniCard
                        title="Last reviewer"
                        handle={report.reviewedBy?.handle ?? 'unassigned'}
                        presence={report.reviewedBy?.presenceStatus}
                        onOpenProfile={report.reviewedBy ? onOpenProfile : undefined}
                        muted={!report.reviewedBy}
                      />
                    </div>

                    {report.resolutionNote ? (
                      <div style={{
                        padding: '12px 14px',
                        borderRadius: '12px',
                        background: 'rgba(255,255,255,0.035)',
                        border: '1px solid rgba(255,255,255,0.08)',
                        color: 'rgba(244,232,200,0.78)',
                        fontSize: '13px',
                        lineHeight: 1.6,
                      }}>
                        <strong style={{ color: '#fff3d3' }}>Resolution note:</strong> {report.resolutionNote}
                      </div>
                    ) : null}

                    {report.targetRestriction ? (
                      <div style={{
                        padding: '12px 14px',
                        borderRadius: '12px',
                        background: report.targetRestriction.kind === 'banned' ? 'rgba(90,24,24,0.28)' : 'rgba(110,76,18,0.22)',
                        border: report.targetRestriction.kind === 'banned' ? '1px solid rgba(255,120,120,0.24)' : '1px solid rgba(255,190,70,0.24)',
                        color: '#fff0d8',
                        fontSize: '13px',
                        lineHeight: 1.6,
                      }}>
                        <strong style={{ color: '#fff7e6', textTransform: 'capitalize' }}>{report.targetRestriction.kind}</strong>
                        {' '}is active for @{report.target.handle}
                        {report.targetRestriction.reason ? `: ${report.targetRestriction.reason}` : '.'}
                      </div>
                    ) : null}

                    <label style={{ display: 'grid', gap: '8px' }}>
                      <span style={{ color: '#ffcf72', fontSize: '11px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>
                        Moderator note
                      </span>
                      <textarea
                        value={notes[report.reportId] ?? report.resolutionNote ?? ''}
                        onChange={(event) => setNotes((current) => ({ ...current, [report.reportId]: event.target.value }))}
                        rows={3}
                        style={{
                          width: '100%',
                          resize: 'vertical',
                          minHeight: '86px',
                          padding: '12px 14px',
                          borderRadius: '12px',
                          border: '1px solid rgba(255,180,60,0.22)',
                          background: 'rgba(12,8,24,0.8)',
                          color: '#fff4dd',
                          lineHeight: 1.5,
                        }}
                      />
                    </label>

                    <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                      <ActionButton
                        label={busyReportId === report.reportId ? 'Saving…' : 'Mark under review'}
                        disabled={busyReportId === report.reportId}
                        onClick={() => void handleResolve(report.reportId, 'under_review')}
                        tone="neutral"
                      />
                      <ActionButton
                        label={busyReportId === report.reportId ? 'Saving…' : 'Resolve only'}
                        disabled={busyReportId === report.reportId}
                        onClick={() => void handleResolve(report.reportId, 'resolved_actioned')}
                        tone="success"
                      />
                      <ActionButton
                        label={busyReportId === report.reportId ? 'Saving…' : 'Resolve + suspend'}
                        disabled={busyReportId === report.reportId}
                        onClick={() => void handleResolve(report.reportId, 'resolved_actioned', 'suspended')}
                        tone="warn"
                      />
                      <ActionButton
                        label={busyReportId === report.reportId ? 'Saving…' : 'Resolve + ban'}
                        disabled={busyReportId === report.reportId}
                        onClick={() => void handleResolve(report.reportId, 'resolved_actioned', 'banned')}
                        tone="danger"
                      />
                      <ActionButton
                        label={busyReportId === report.reportId ? 'Saving…' : 'Resolve: dismissed'}
                        disabled={busyReportId === report.reportId}
                        onClick={() => void handleResolve(report.reportId, 'resolved_dismissed')}
                        tone="danger"
                      />
                      {report.targetRestriction ? (
                        <ActionButton
                          label={busyReportId === report.reportId ? 'Saving…' : 'Clear restriction'}
                          disabled={busyReportId === report.reportId}
                          onClick={() => void handleResolve(report.reportId, 'resolved_actioned', 'clear')}
                          tone="neutral"
                        />
                      ) : null}
                    </div>
                  </article>
                ))
              )}
            </section>

            <section style={{ display: 'grid', gap: '14px' }}>
              <div>
                <div style={{ color: '#fff2c8', fontSize: '18px', fontWeight: 800 }}>Active restrictions</div>
                <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                  Suspended and banned accounts are blocked from normal platform access until moderators clear the restriction.
                </div>
              </div>
              {(overview?.activeRestrictions.length ?? 0) === 0 ? (
                <div style={{
                  padding: '18px',
                  borderRadius: '16px',
                  border: '1px solid rgba(255,180,60,0.16)',
                  background: 'rgba(255,255,255,0.03)',
                  color: 'rgba(244,232,200,0.7)',
                }}>
                  No active suspensions or bans.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '10px' }}>
                  {overview?.activeRestrictions.map((restriction) => (
                    <div
                      key={restriction.restrictionId}
                      style={{
                        padding: '14px 16px',
                        borderRadius: '14px',
                        border: restriction.kind === 'banned' ? '1px solid rgba(255,120,120,0.24)' : '1px solid rgba(255,190,70,0.24)',
                        background: restriction.kind === 'banned' ? 'rgba(90,24,24,0.24)' : 'rgba(90,62,24,0.2)',
                        display: 'grid',
                        gap: '6px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
                        <div style={{ color: '#fff3dc', fontWeight: 800, textTransform: 'capitalize' }}>
                          {restriction.kind} on @{restriction.account.handle}
                        </div>
                        <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
                          Updated {formatDateTime(restriction.updatedAt)}
                        </div>
                      </div>
                      {restriction.reason ? (
                        <div style={{ color: 'rgba(255,244,222,0.82)', fontSize: '13px', lineHeight: 1.6 }}>
                          {restriction.reason}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section style={{ display: 'grid', gap: '14px' }}>
              <div>
                <div style={{ color: '#fff2c8', fontSize: '18px', fontWeight: 800 }}>Recent moderator actions</div>
                <div style={{ color: 'rgba(244,232,200,0.66)', fontSize: '12px', marginTop: '4px' }}>
                  This is the durable audit trail for who touched a report and how its state changed.
                </div>
              </div>
              {(overview?.recentActions.length ?? 0) === 0 ? (
                <div style={{
                  padding: '18px',
                  borderRadius: '16px',
                  border: '1px solid rgba(255,180,60,0.16)',
                  background: 'rgba(255,255,255,0.03)',
                  color: 'rgba(244,232,200,0.7)',
                }}>
                  No moderator actions have been recorded yet.
                </div>
              ) : (
                <div style={{ display: 'grid', gap: '10px' }}>
                  {overview?.recentActions.map((action) => (
                    <div
                      key={action.actionId}
                      style={{
                        padding: '14px 16px',
                        borderRadius: '14px',
                        border: '1px solid rgba(255,180,60,0.14)',
                        background: 'rgba(255,255,255,0.025)',
                        display: 'grid',
                        gap: '6px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
                        <div style={{ color: '#fff4d6', fontWeight: 700 }}>
                          @{action.moderator.handle} moved report <span style={{ color: '#ffcf72' }}>{action.reportId}</span> from {action.previousStatus.replace(/_/g, ' ')} to {action.nextStatus.replace(/_/g, ' ')}
                        </div>
                        <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '12px' }}>
                          {formatDateTime(action.createdAt)}
                        </div>
                      </div>
                      <div style={{ color: 'rgba(244,232,200,0.76)', fontSize: '13px' }}>
                        Reporter @{action.reporter.handle} · Target @{action.target.handle}
                      </div>
                      {action.note ? (
                        <div style={{ color: 'rgba(244,232,200,0.72)', fontSize: '13px', lineHeight: 1.6 }}>
                          {action.note}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        </>
      )}
    </div>
  );
}

function ProfileMiniCard({
  title,
  handle,
  presence,
  onOpenProfile,
  muted = false,
}: {
  title: string;
  handle: string;
  presence?: string;
  onOpenProfile?: ((handle: string) => void) | undefined;
  muted?: boolean;
}): React.ReactElement {
  return (
    <div style={{
      padding: '12px 14px',
      borderRadius: '12px',
      border: '1px solid rgba(255,255,255,0.08)',
      background: 'rgba(255,255,255,0.025)',
      display: 'grid',
      gap: '6px',
    }}>
      <div style={{ color: 'rgba(244,232,200,0.62)', fontSize: '11px', letterSpacing: '0.9px', textTransform: 'uppercase', fontWeight: 800 }}>
        {title}
      </div>
      <button
        onClick={() => onOpenProfile?.(handle)}
        disabled={!onOpenProfile || muted}
        style={{
          padding: 0,
          border: 'none',
          background: 'transparent',
          color: muted ? 'rgba(244,232,200,0.54)' : '#fff3d3',
          fontWeight: 800,
          fontSize: '14px',
          textAlign: 'left',
          cursor: !onOpenProfile || muted ? 'default' : 'pointer',
        }}
      >
        @{handle}
      </button>
      <div style={{ color: 'rgba(244,232,200,0.6)', fontSize: '12px' }}>
        {presence ? presence.replace(/_/g, ' ') : muted ? 'No reviewer yet' : 'Presence unavailable'}
      </div>
    </div>
  );
}

function ActionButton({
  label,
  disabled,
  onClick,
  tone,
}: {
  label: string;
  disabled: boolean;
  onClick: () => void;
  tone: 'neutral' | 'success' | 'warn' | 'danger';
}): React.ReactElement {
  const background = tone === 'success'
    ? 'linear-gradient(180deg, rgba(46,140,90,0.92) 0%, rgba(23,88,53,0.96) 100%)'
    : tone === 'warn'
      ? 'linear-gradient(180deg, rgba(180,126,26,0.92) 0%, rgba(114,74,12,0.96) 100%)'
    : tone === 'danger'
      ? 'linear-gradient(180deg, rgba(148,66,66,0.92) 0%, rgba(96,33,33,0.96) 100%)'
      : 'rgba(255,255,255,0.05)';
  const border = tone === 'neutral'
    ? '1px solid rgba(255,180,60,0.2)'
    : tone === 'success'
      ? '1px solid rgba(125,255,180,0.25)'
      : tone === 'warn'
        ? '1px solid rgba(255,210,120,0.28)'
      : '1px solid rgba(255,140,140,0.22)';
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        padding: '10px 14px',
        borderRadius: '10px',
        border,
        background: disabled ? 'rgba(120,90,35,0.28)' : background,
        color: '#fff7e3',
        fontWeight: 800,
        cursor: disabled ? 'not-allowed' : 'pointer',
      }}
    >
      {label}
    </button>
  );
}
