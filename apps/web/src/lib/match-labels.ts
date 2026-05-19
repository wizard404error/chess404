/**
 * match-labels.ts
 *
 * Pure display-formatting helpers for match mode, queue, finish reason,
 * and social alert structures — shared across App.tsx, useMatchEngine.tsx,
 * and all page-level components that previously defined their own copies.
 */

import {
  DEFAULT_MATCH_MODE_ID,
  OFFICIAL_MATCH_MODES,
  type MatchFinishReason,
  type MatchModeId,
} from '@chess404/contracts';
import type { AccountNotificationView } from './platform-service';
import type { QueueName } from './matchmaking-service';

// ── Mode label ────────────────────────────────────────────────────────────────

export function modeLabel(modeId?: MatchModeId | string | null): string {
  return (
    OFFICIAL_MATCH_MODES.find((mode) => mode.id === (modeId ?? DEFAULT_MATCH_MODE_ID))?.label ??
    'Open Cards'
  );
}

// ── Queue label ───────────────────────────────────────────────────────────────

export function queueLabel(queue?: QueueName | 'direct' | string | null): string {
  if (queue === 'rated') return 'Rated';
  if (queue === 'casual') return 'Casual';
  return 'Direct';
}

// ── Finish reason label ───────────────────────────────────────────────────────

export function finishReasonLabel(reason?: MatchFinishReason | string | null): string | null {
  switch (reason) {
    case 'checkmate':
      return 'Checkmate';
    case 'stalemate':
      return 'Stalemate';
    case 'insufficient_material':
      return 'Insufficient Material';
    case 'threefold_repetition':
      return 'Threefold Repetition';
    case 'fifty_move_rule':
      return '50-Move Rule';
    case 'timeout':
      return 'Timeout';
    case 'abandon':
      return 'Abandonment';
    case 'resign':
      return 'Resignation';
    case 'abort':
      return 'Early Abort';
    case 'draw_agreement':
      return 'Mutual Agreement';
    default:
      return null;
  }
}

// ── Social alert ──────────────────────────────────────────────────────────────

export type SocialAlert = {
  id: string;
  title: string;
  detail: string;
  actionLabel: string;
  action: 'friends' | 'match';
  matchId?: string;
};

export function buildSocialAlert(notification: AccountNotificationView): SocialAlert | null {
  const handle = `@${notification.actor.handle}`;
  switch (notification.kind) {
    case 'direct_challenge_accepted':
      if (!notification.matchId) return null;
      return {
        id: notification.notificationId,
        title: `${handle} accepted your challenge`,
        detail: `${modeLabel(notification.modeId)} is ready to play now.`,
        actionLabel: 'Open Match',
        action: 'match',
        matchId: notification.matchId,
      };
    case 'direct_challenge_received':
      return {
        id: notification.notificationId,
        title: `${handle} challenged you to ${modeLabel(notification.modeId)}`,
        detail: 'Open Friends to accept or decline the invite.',
        actionLabel: 'Open Friends',
        action: 'friends',
      };
    case 'friend_request_received':
      return {
        id: notification.notificationId,
        title: `${handle} sent you a friend request`,
        detail: 'Open Friends to accept or decline it.',
        actionLabel: 'Open Friends',
        action: 'friends',
      };
    case 'friend_request_accepted':
      return {
        id: notification.notificationId,
        title: `${handle} accepted your friend request`,
        detail: 'You can challenge them directly from your friends list now.',
        actionLabel: 'Open Friends',
        action: 'friends',
      };
    default:
      return null;
  }
}
