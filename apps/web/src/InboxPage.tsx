import React from 'react';
import { type MatchModeId } from '@chess404/contracts';
import {
  fetchAccountNotificationOverview,
  markAccountNotificationRead,
  markAllAccountNotificationsRead,
  type AccountNotificationOverview,
  type AccountNotificationView,
} from './lib/platform-service';
import { modeLabel } from './lib/match-labels';

interface InboxPageProps {
  accountId?: string | null;
  sessionToken?: string | null;
  liveRefreshToken?: number;
  onOpenProfile?: (handle: string) => void;
  onOpenFriends?: () => void;
  onUnreadCountChange?: (count: number) => void;
}

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


function describeNotification(notification: AccountNotificationView): {
  title: string;
  detail: string;
  accent: string;
  border: string;
  background: string;
  actionLabel: string;
} {
  const handle = `@${notification.actor.handle}`;
  switch (notification.kind) {
    case 'friend_request_received':
      return {
        title: `${handle} sent you a friend request`,
        detail: 'Open Friends to accept or decline it.',
        accent: '#b7d8ff',
        border: '1px solid rgba(80,140,220,0.28)',
        background: 'linear-gradient(180deg, rgba(38,56,88,0.32) 0%, rgba(18,24,40,0.18) 100%)',
        actionLabel: 'Open Friends',
      };
    case 'friend_request_accepted':
      return {
        title: `${handle} accepted your friend request`,
        detail: 'Your friends graph is now connected for challenges and future lobbies.',
        accent: '#d7ffd8',
        border: '1px solid rgba(86,204,120,0.26)',
        background: 'linear-gradient(180deg, rgba(28,72,40,0.26) 0%, rgba(18,26,22,0.16) 100%)',
        actionLabel: 'Open Friends',
      };
    case 'direct_challenge_received':
      return {
        title: `${handle} challenged you to ${modeLabel(notification.modeId)}`,
        detail: 'Open Friends to accept or decline the live match invite.',
        accent: '#ffe7a6',
        border: '1px solid rgba(220,170,80,0.32)',
        background: 'linear-gradient(180deg, rgba(88,58,24,0.28) 0%, rgba(26,20,14,0.18) 100%)',
        actionLabel: 'Open Friends',
      };
    case 'direct_challenge_accepted':
      return {
        title: `${handle} accepted your direct challenge`,
        detail: `Your ${modeLabel(notification.modeId)} match is ready.`,
        accent: '#ffd9b0',
        border: '1px solid rgba(220,132,76,0.30)',
        background: 'linear-gradient(180deg, rgba(92,48,26,0.30) 0%, rgba(28,18,14,0.16) 100%)',
        actionLabel: 'Open Friends',
      };
    case 'direct_challenge_declined':
      return {
        title: `${handle} declined your direct challenge`,
        detail: 'You can send a different invite later.',
        accent: '#ffd0d0',
        border: '1px solid rgba(204,92,92,0.28)',
        background: 'linear-gradient(180deg, rgba(88,34,34,0.30) 0%, rgba(22,14,14,0.16) 100%)',
        actionLabel: 'Open Friends',
      };
    case 'direct_challenge_cancelled':
    default:
      return {
        title: `${handle} cancelled a direct challenge`,
        detail: 'The pending invite is no longer active.',
        accent: '#f0ddbe',
        border: '1px solid rgba(255,255,255,0.12)',
        background: 'linear-gradient(180deg, rgba(255,255,255,0.06) 0%, rgba(18,16,14,0.12) 100%)',
        actionLabel: 'Open Friends',
      };
  }
}

export default function InboxPage({
  accountId = null,
  sessionToken = null,
  liveRefreshToken = 0,
  onOpenProfile,
  onOpenFriends,
  onUnreadCountChange,
}: InboxPageProps): React.ReactElement {
  const [overview, setOverview] = React.useState<AccountNotificationOverview | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');
  const [notice, setNotice] = React.useState('');
  const [busyNotificationId, setBusyNotificationId] = React.useState<string | null>(null);

  const loadOverview = React.useCallback(async () => {
    if (!accountId || !sessionToken) {
      setOverview(null);
      onUnreadCountChange?.(0);
      return;
    }
    setLoading(true);
    setError('');
    try {
      const nextOverview = await fetchAccountNotificationOverview({
        accountId,
        sessionToken,
        limit: 48,
      });
      setOverview(nextOverview);
      onUnreadCountChange?.(nextOverview.unreadCount);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load inbox.');
    } finally {
      setLoading(false);
    }
  }, [accountId, onUnreadCountChange, sessionToken]);

  React.useEffect(() => {
    if (busyNotificationId) {
      return;
    }
    void loadOverview();
  }, [busyNotificationId, liveRefreshToken, loadOverview]);

  const markRead = React.useCallback(async (notification: AccountNotificationView) => {
    if (!accountId || !sessionToken || notification.readAt) {
      return;
    }
    setBusyNotificationId(notification.notificationId);
    setError('');
    setNotice('');
    try {
      const nextOverview = await markAccountNotificationRead({
        accountId,
        sessionToken,
        notificationId: notification.notificationId,
      });
      setOverview(nextOverview);
      onUnreadCountChange?.(nextOverview.unreadCount);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to mark notification as read.');
    } finally {
      setBusyNotificationId(null);
    }
  }, [accountId, onUnreadCountChange, sessionToken]);

  const markAllRead = React.useCallback(async () => {
    if (!accountId || !sessionToken) {
      return;
    }
    setBusyNotificationId('__all__');
    setError('');
    setNotice('');
    try {
      const nextOverview = await markAllAccountNotificationsRead({
        accountId,
        sessionToken,
      });
      setOverview(nextOverview);
      onUnreadCountChange?.(nextOverview.unreadCount);
      setNotice(nextOverview.unreadCount === 0 ? 'Inbox marked as read.' : 'Unread state refreshed.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to mark inbox as read.');
    } finally {
      setBusyNotificationId(null);
    }
  }, [accountId, onUnreadCountChange, sessionToken]);

  const renderNotification = React.useCallback((notification: AccountNotificationView) => {
    const descriptor = describeNotification(notification);
    const unread = !notification.readAt;
    return (
      <article
        key={notification.notificationId}
        style={{
          padding: '18px 18px 16px',
          borderRadius: '16px',
          border: descriptor.border,
          background: descriptor.background,
          boxShadow: unread ? '0 12px 28px rgba(0,0,0,0.18)' : '0 8px 18px rgba(0,0,0,0.12)',
          opacity: unread ? 1 : 0.82,
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: '16px', alignItems: 'flex-start' }}>
          <div style={{ display: 'grid', gap: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
              <span
                style={{
                  padding: '4px 10px',
                  borderRadius: '999px',
                  background: unread ? 'rgba(255,215,0,0.16)' : 'rgba(255,255,255,0.08)',
                  color: unread ? '#ffe7a6' : 'rgba(255,232,180,0.74)',
                  border: unread ? '1px solid rgba(255,215,0,0.24)' : '1px solid rgba(255,255,255,0.08)',
                  fontSize: '11px',
                  fontWeight: 700,
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                }}
              >
                {unread ? 'Unread' : 'Read'}
              </span>
              <button
                onClick={() => onOpenProfile?.(notification.actor.handle)}
                style={{
                  padding: 0,
                  background: 'transparent',
                  border: 'none',
                  color: descriptor.accent,
                  fontWeight: 700,
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
              >
                @{notification.actor.handle}
              </button>
            </div>
            <div style={{ color: '#fff6dc', fontSize: '17px', fontWeight: 700, lineHeight: 1.35 }}>
              {descriptor.title}
            </div>
            <div style={{ color: 'rgba(255,236,196,0.82)', fontSize: '13px', lineHeight: 1.55 }}>
              {descriptor.detail}
            </div>
            <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '12px' }}>
              {formatDateTime(notification.updatedAt)}
            </div>
          </div>
          <div style={{ display: 'grid', gap: '8px', minWidth: '150px' }}>
            <button
              onClick={() => onOpenFriends?.()}
              style={{
                padding: '10px 12px',
                borderRadius: '10px',
                border: '1px solid rgba(255,255,255,0.12)',
                background: 'rgba(255,255,255,0.06)',
                color: '#fff6dc',
                fontWeight: 700,
                cursor: 'pointer',
              }}
            >
              {descriptor.actionLabel}
            </button>
            <button
              onClick={() => void markRead(notification)}
              disabled={!unread || busyNotificationId === notification.notificationId}
              style={{
                padding: '10px 12px',
                borderRadius: '10px',
                border: unread ? '1px solid rgba(255,215,0,0.22)' : '1px solid rgba(255,255,255,0.08)',
                background: unread ? 'rgba(255,215,0,0.08)' : 'rgba(255,255,255,0.025)',
                color: unread ? '#ffe7a6' : 'rgba(255,232,180,0.48)',
                fontWeight: 600,
                cursor: unread ? 'pointer' : 'default',
              }}
            >
              {busyNotificationId === notification.notificationId ? 'Updating…' : (unread ? 'Mark Read' : 'Already Read')}
            </button>
          </div>
        </div>
      </article>
    );
  }, [busyNotificationId, markRead, onOpenFriends, onOpenProfile]);

  if (!accountId || !sessionToken) {
    return (
      <section style={{ padding: '28px', display: 'grid', gap: '16px' }}>
        <h2 style={{ color: '#fff3cf', fontSize: '24px', margin: 0 }}>Inbox</h2>
        <div style={{ color: 'rgba(255,232,180,0.78)', maxWidth: '720px', lineHeight: 1.7 }}>
          Sign in with a claimed account to unlock your persistent social inbox for friend requests, direct challenges, and platform notifications.
        </div>
      </section>
    );
  }

  const notifications = overview?.notifications ?? [];

  return (
    <section style={{ padding: '26px', display: 'grid', gap: '18px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '16px', alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <div style={{ display: 'grid', gap: '8px' }}>
          <h2 style={{ color: '#fff3cf', fontSize: '28px', margin: 0 }}>Inbox</h2>
          <div style={{ color: 'rgba(255,232,180,0.74)', maxWidth: '760px', lineHeight: 1.6 }}>
            Persistent social updates for your account: friend graph activity, direct challenge flow, and live platform signals that should survive page switches and offline time.
          </div>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
          <div
            style={{
              padding: '10px 14px',
              borderRadius: '12px',
              border: '1px solid rgba(255,215,0,0.18)',
              background: 'rgba(255,215,0,0.06)',
              color: '#ffe7a6',
              fontWeight: 700,
            }}
          >
            {overview?.unreadCount ?? 0} unread
          </div>
          <button
            onClick={() => void markAllRead()}
            disabled={!overview || overview.unreadCount === 0 || busyNotificationId === '__all__'}
            style={{
              padding: '10px 14px',
              borderRadius: '12px',
              border: '1px solid rgba(255,255,255,0.12)',
              background: 'rgba(255,255,255,0.06)',
              color: '#fff6dc',
              fontWeight: 700,
              cursor: overview?.unreadCount ? 'pointer' : 'default',
            }}
          >
            {busyNotificationId === '__all__' ? 'Updating…' : 'Mark All Read'}
          </button>
          <button
            onClick={() => void loadOverview()}
            style={{
              padding: '10px 14px',
              borderRadius: '12px',
              border: '1px solid rgba(255,255,255,0.12)',
              background: 'rgba(255,255,255,0.04)',
              color: 'rgba(255,232,180,0.88)',
              fontWeight: 600,
              cursor: 'pointer',
            }}
          >
            Refresh
          </button>
        </div>
      </div>

      {notice ? (
        <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(86,204,120,0.08)', border: '1px solid rgba(86,204,120,0.18)', color: '#d7ffd8' }}>
          {notice}
        </div>
      ) : null}
      {error ? (
        <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(214,90,90,0.08)', border: '1px solid rgba(214,90,90,0.18)', color: '#ffd7d7' }}>
          {error}
        </div>
      ) : null}

      {loading && !overview ? (
        <div style={{ color: 'rgba(255,232,180,0.72)' }}>Loading inbox…</div>
      ) : null}

      {!loading && notifications.length === 0 ? (
        <div style={{ padding: '24px', borderRadius: '18px', border: '1px solid rgba(255,255,255,0.10)', background: 'rgba(255,255,255,0.03)', color: 'rgba(255,232,180,0.72)', lineHeight: 1.7 }}>
          Your inbox is clear. As friends send requests or direct challenges, this account-level feed will keep the history in one place.
        </div>
      ) : null}

      <div style={{ display: 'grid', gap: '14px' }}>
        {notifications.map(renderNotification)}
      </div>
    </section>
  );
}
