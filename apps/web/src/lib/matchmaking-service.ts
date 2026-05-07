export type QueueName = 'casual' | 'rated';
export type TicketStatus = 'queued' | 'matched' | 'cancelled';

export interface QueueTicket {
  ticketId: string;
  guestId: string;
  queue: QueueName;
  status: TicketStatus;
  rating: number;
  createdAt: string;
  updatedAt: string;
  matchedAt?: string;
  matchedWith?: string;
  assignedRoom?: string;
}

export interface QueueSnapshot {
  queue: QueueName;
  queuedCount: number;
  matchedCount: number;
  cancelledCount: number;
}

export async function enqueueGuest(guestId: string, queue: QueueName, rating: number): Promise<{ ticket: QueueTicket; snapshot: QueueSnapshot }> {
  const response = await fetch('/api/matchmaking/queues/tickets', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ guestId, queue, rating }),
  });
  return unwrapResponse(response);
}

export async function fetchQueueTickets(queue: QueueName): Promise<QueueTicket[]> {
  const response = await fetch(`/api/matchmaking/queues/tickets?queue=${queue}`, {
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
