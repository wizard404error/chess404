import type { MatchModeId } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID } from '@chess404/contracts';

export type QueueName = 'casual' | 'rated';
export type TicketStatus = 'queued' | 'matched' | 'cancelled';

export interface QueueTicket {
  ticketId: string;
  guestId: string;
  displayName?: string;
  queue: QueueName;
  modeId?: MatchModeId;
  status: TicketStatus;
  rating: number;
  createdAt: string;
  updatedAt: string;
  matchedAt?: string;
  matchedWith?: string;
  seatColor?: 'white' | 'black';
  opponentName?: string;
  assignedRoom?: string;
}

export interface QueueSnapshot {
  queue: QueueName;
  modeId?: MatchModeId;
  queuedCount: number;
  matchedCount: number;
  cancelledCount: number;
}

export interface QueueSnapshotResponse {
  snapshots: QueueSnapshot[];
  checkedAt?: string;
}

export interface EnqueueGuestAccountIdentity {
  accountId?: string;
  accountSessionToken?: string;
}

export async function enqueueGuest(
  guestId: string,
  queue: QueueName,
  modeId: MatchModeId,
  rating: number,
  displayName?: string,
  accountIdentity: EnqueueGuestAccountIdentity = {},
): Promise<{ ticket: QueueTicket; snapshot: QueueSnapshot }> {
  const response = await fetch('/api/matchmaking/queues/tickets', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      guestId,
      queue,
      modeId,
      rating,
      displayName,
      accountId: accountIdentity.accountId,
      accountSessionToken: accountIdentity.accountSessionToken,
    }),
  });
  return unwrapResponse(response);
}

export async function fetchQueueTickets(queue: QueueName, modeId: MatchModeId = DEFAULT_MATCH_MODE_ID): Promise<QueueTicket[]> {
  const params = new URLSearchParams({ queue, modeId });
  const response = await fetch(`/api/matchmaking/queues/tickets?${params.toString()}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });
  const payload = await unwrapResponse<{ tickets?: QueueTicket[] }>(response);
  return payload.tickets ?? [];
}

export async function fetchTicket(ticketId: string): Promise<{ ticket: QueueTicket; snapshot: QueueSnapshot }> {
  const response = await fetch(`/api/matchmaking/queues/tickets/${ticketId}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });
  return unwrapResponse(response);
}

export async function cancelTicket(ticketId: string): Promise<{ ticket: QueueTicket; snapshot: QueueSnapshot }> {
  const response = await fetch(`/api/matchmaking/queues/tickets/${ticketId}`, {
    method: 'DELETE',
    headers: {
      'Content-Type': 'application/json',
    },
  });
  return unwrapResponse(response);
}

async function unwrapResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Request failed with ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload?.error) {
        message = payload.error;
      }
    } catch {
      // Ignore parse failures and keep fallback message.
    }
    throw new Error(message);
  }
  return response.json() as Promise<T>;
}

export async function fetchQueueSnapshots(queue?: QueueName, modeId?: MatchModeId): Promise<QueueSnapshotResponse> {
  const params = new URLSearchParams();
  if (queue) {
    params.set('queue', queue);
  }
  if (modeId) {
    params.set('modeId', modeId);
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
  const response = await fetch(`/api/matchmaking/queues/snapshots${suffix}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    cache: 'no-store',
  });
  const payload = await unwrapResponse<QueueSnapshotResponse>(response);
  return {
    snapshots: payload.snapshots ?? [],
    checkedAt: payload.checkedAt,
  };
}
