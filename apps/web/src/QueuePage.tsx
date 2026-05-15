import React from 'react';
import type { GuestProfile } from './lib/platform-service';
import type { QueueName, QueueTicket } from './lib/matchmaking-service';
import { cancelTicket, enqueueGuest, fetchQueueTickets, fetchTicket } from './lib/matchmaking-service';
import { ensureMatch, readStoredRoomMeta, resolveSeatSecret, writeStoredRoomMeta } from './lib/match-service';

interface QueuePageProps {
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
}

type QueueSide = 'white' | 'black';

interface StoredTicketRef {
  ticketId: string;
  queue: QueueName;
}

const DEFAULT_QUEUE: QueueName = 'casual';
const QUEUE_SELECTION_STORAGE_KEY = 'chess404.queue.selection';

function queueTicketStorageKey(side: QueueSide): string {
  return `chess404.queue.${side}.ticket`;
}

function guestSessionStorageKey(side: QueueSide): string {
  return `chess404.guest.${side}.secret`;
}

function accountIdStorageKey(side: QueueSide): string {
  return `chess404.account.${side}.id`;
}

function readStoredQueueSelection(): QueueName {
  if (typeof window === 'undefined') {
    return DEFAULT_QUEUE;
  }
  const value = window.localStorage.getItem(QUEUE_SELECTION_STORAGE_KEY);
  return value === 'rated' ? 'rated' : DEFAULT_QUEUE;
}

function readStoredTicketRef(side: QueueSide): StoredTicketRef | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = window.localStorage.getItem(queueTicketStorageKey(side));
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<StoredTicketRef>;
    if (!parsed.ticketId) {
      return null;
    }
    return {
      ticketId: parsed.ticketId,
      queue: parsed.queue === 'rated' ? 'rated' : DEFAULT_QUEUE,
    };
  } catch {
    return null;
  }
}

function writeStoredTicketRef(side: QueueSide, ticket: QueueTicket | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  if (!ticket || ticket.status === 'cancelled') {
    window.localStorage.removeItem(queueTicketStorageKey(side));
    return;
  }
  window.localStorage.setItem(queueTicketStorageKey(side), JSON.stringify({
    ticketId: ticket.ticketId,
    queue: ticket.queue,
  }));
}

function clearStoredTicketRef(side: QueueSide): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.removeItem(queueTicketStorageKey(side));
}

function readStoredGuestSessionSecret(side: QueueSide): string | null {
  if (typeof window === 'undefined') {
    return null;
  }
  return window.localStorage.getItem(guestSessionStorageKey(side));
}

function readStoredAccountId(side: QueueSide): string | null {
  if (typeof window === 'undefined') {
    return null;
  }
  return window.localStorage.getItem(accountIdStorageKey(side));
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

export default function QueuePage({ whiteProfile, blackProfile }: QueuePageProps): React.ReactElement {
  const [hostedRuntime, setHostedRuntime] = React.useState(false);
  const [queue, setQueue] = React.useState<QueueName>(() => readStoredQueueSelection());
  const [whiteTicket, setWhiteTicket] = React.useState<QueueTicket | null>(null);
  const [blackTicket, setBlackTicket] = React.useState<QueueTicket | null>(null);
  const [queueTickets, setQueueTickets] = React.useState<QueueTicket[]>([]);
  const [error, setError] = React.useState('');
  const [loading, setLoading] = React.useState(false);
  const [restoringTickets, setRestoringTickets] = React.useState(true);

  const refreshQueue = React.useCallback(async (queueName: QueueName) => {
    const tickets = await fetchQueueTickets(queueName);
    setQueueTickets(tickets);
  }, []);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const hostname = window.location.hostname.toLowerCase();
    setHostedRuntime(hostname !== 'localhost' && hostname !== '127.0.0.1');
  }, []);

  React.useEffect(() => {
    void refreshQueue(queue).catch(err => {
      setError(err instanceof Error ? err.message : 'Failed to load queue state.');
    });
  }, [queue, refreshQueue]);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(QUEUE_SELECTION_STORAGE_KEY, queue);
  }, [queue]);

  React.useEffect(() => {
    let cancelled = false;

    const restoreTickets = async () => {
      const whiteRef = readStoredTicketRef('white');
      const blackRef = readStoredTicketRef('black');

      if (!whiteRef && !blackRef) {
        setRestoringTickets(false);
        return;
      }

      const hydrate = async (side: QueueSide, stored: StoredTicketRef | null): Promise<QueueTicket | null> => {
        if (!stored) {
          return null;
        }
        try {
          const { ticket } = await fetchTicket(stored.ticketId);
          return ticket;
        } catch (err) {
          const message = err instanceof Error ? err.message : '';
          if (message.includes('404')) {
            clearStoredTicketRef(side);
            return null;
          }
          throw err;
        }
      };

      try {
        const [restoredWhite, restoredBlack] = await Promise.all([
          hydrate('white', whiteRef),
          hydrate('black', blackRef),
        ]);
        if (cancelled) {
          return;
        }
        setWhiteTicket(restoredWhite);
        setBlackTicket(restoredBlack);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to resume saved queue tickets.');
        }
      } finally {
        if (!cancelled) {
          setRestoringTickets(false);
        }
      }
    };

    void restoreTickets();

    return () => {
      cancelled = true;
    };
  }, []);

  React.useEffect(() => {
    writeStoredTicketRef('white', whiteTicket);
  }, [whiteTicket]);

  React.useEffect(() => {
    writeStoredTicketRef('black', blackTicket);
  }, [blackTicket]);

  React.useEffect(() => {
    if (!hostedRuntime) {
      return;
    }
    clearStoredTicketRef('black');
    setBlackTicket(null);
  }, [hostedRuntime]);

  React.useEffect(() => {
    if (restoringTickets) {
      return;
    }
    if (!whiteTicket && !blackTicket) {
      return;
    }

    const interval = window.setInterval(() => {
      const tasks: Promise<void>[] = [];
      if (whiteTicket?.status === 'queued') {
        tasks.push(
          fetchTicket(whiteTicket.ticketId).then(({ ticket }) => {
            setWhiteTicket(ticket);
          })
        );
      }
      if (blackTicket?.status === 'queued') {
        tasks.push(
          fetchTicket(blackTicket.ticketId).then(({ ticket }) => {
            setBlackTicket(ticket);
          })
        );
      }
      tasks.push(refreshQueue(queue));
      void Promise.all(tasks).catch(() => {
        // Keep last visible state if polling fails.
      });
    }, 2500);

    return () => window.clearInterval(interval);
  }, [whiteTicket, blackTicket, queue, refreshQueue, restoringTickets]);

  const handleJoin = React.useCallback(async (side: 'white' | 'black') => {
    const profile = side === 'white' ? whiteProfile : blackProfile;
    if (!profile) {
      setError('Guest profile is still loading. Try again in a moment.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await enqueueGuest(profile.guestId, queue, profile.rating);
      if (side === 'white') {
        setWhiteTicket(result.ticket);
      } else {
        setBlackTicket(result.ticket);
      }
      await refreshQueue(queue);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to join queue.');
    } finally {
      setLoading(false);
    }
  }, [whiteProfile, blackProfile, queue, refreshQueue]);

  const handleCancel = React.useCallback(async (side: 'white' | 'black') => {
    const ticket = side === 'white' ? whiteTicket : blackTicket;
    if (!ticket) {
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await cancelTicket(ticket.ticketId);
      if (side === 'white') {
        setWhiteTicket(result.ticket);
      } else {
        setBlackTicket(result.ticket);
      }
      await refreshQueue(queue);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel queue ticket.');
    } finally {
      setLoading(false);
    }
  }, [whiteTicket, blackTicket, queue, refreshQueue]);

  const handleOpenMatch = React.useCallback(async (ticket: QueueTicket) => {
    if (!ticket.assignedRoom) {
      return;
    }
    setLoading(true);
    setError('');
    try {
      const existingRoomMeta = readStoredRoomMeta(ticket.assignedRoom);
      const roomMeta = {
        ...existingRoomMeta,
        queue,
        whiteGuestId: existingRoomMeta?.whiteGuestId ?? whiteProfile?.guestId,
        blackGuestId: existingRoomMeta?.blackGuestId ?? (hostedRuntime ? undefined : blackProfile?.guestId),
        whiteAccountId: existingRoomMeta?.whiteAccountId ?? readStoredAccountId('white') ?? undefined,
        blackAccountId: existingRoomMeta?.blackAccountId ?? (hostedRuntime ? undefined : readStoredAccountId('black') ?? undefined),
        whiteName: existingRoomMeta?.whiteName ?? whiteProfile?.displayName,
        blackName: existingRoomMeta?.blackName ?? (hostedRuntime ? undefined : blackProfile?.displayName),
        whitePlayerSecret: resolveSeatSecret(existingRoomMeta?.whitePlayerSecret, readStoredGuestSessionSecret('white')),
        blackPlayerSecret: resolveSeatSecret(existingRoomMeta?.blackPlayerSecret, hostedRuntime ? null : readStoredGuestSessionSecret('black')),
      };
      writeStoredRoomMeta(ticket.assignedRoom, roomMeta);
      await ensureMatch({
        matchId: ticket.assignedRoom,
        clockSeconds: 600,
        queue: roomMeta.queue,
        whiteGuestId: roomMeta.whiteGuestId,
        blackGuestId: roomMeta.blackGuestId,
        whiteAccountId: roomMeta.whiteAccountId,
        blackAccountId: roomMeta.blackAccountId,
        whiteName: roomMeta.whiteName,
        blackName: roomMeta.blackName,
        whitePlayerSecret: roomMeta.whitePlayerSecret,
        blackPlayerSecret: roomMeta.blackPlayerSecret,
      });
      clearStoredTicketRef('white');
      if (!hostedRuntime) {
        clearStoredTicketRef('black');
      }
      window.location.href = `/?match=${encodeURIComponent(ticket.assignedRoom)}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to open matched room.');
    } finally {
      setLoading(false);
    }
  }, [queue, whiteProfile, blackProfile, hostedRuntime]);

  const queueCard = (
    side: 'white' | 'black',
    profile: GuestProfile | null,
    ticket: QueueTicket | null,
    title?: string,
  ): React.ReactElement => (
    <div
      style={{
        padding: '18px',
        borderRadius: '14px',
        background: side === 'white'
          ? 'linear-gradient(180deg, rgba(10,40,18,0.78) 0%, rgba(8,20,12,0.88) 100%)'
          : 'linear-gradient(180deg, rgba(34,12,58,0.78) 0%, rgba(16,10,32,0.88) 100%)',
        border: side === 'white'
          ? '1px solid rgba(60,220,110,0.24)'
          : '1px solid rgba(180,110,255,0.24)',
        boxShadow: '0 12px 36px rgba(0,0,0,0.28)',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
        <div>
          <div style={{ color: side === 'white' ? '#dcffe9' : '#eedcff', fontSize: '15px', fontWeight: 800 }}>{title ?? profile?.displayName ?? 'Loading...'}</div>
          <div style={{ color: side === 'white' ? '#7ce3aa' : '#c9a8ff', fontSize: '12px', marginTop: '4px' }}>♟ {profile?.rating ?? 1200}</div>
        </div>
        <div style={{
          padding: '4px 9px',
          borderRadius: '999px',
          fontSize: '10px',
          fontWeight: 800,
          textTransform: 'uppercase',
          color: ticket?.status === 'matched' ? '#ffe4a0' : ticket?.status === 'queued' ? '#a9f0c5' : 'rgba(255,255,255,0.7)',
          background: ticket?.status === 'matched' ? 'rgba(180,120,20,0.22)' : ticket?.status === 'queued' ? 'rgba(24,120,62,0.22)' : 'rgba(255,255,255,0.08)',
          border: ticket?.status === 'matched' ? '1px solid rgba(255,180,60,0.2)' : ticket?.status === 'queued' ? '1px solid rgba(78,210,132,0.2)' : '1px solid rgba(255,255,255,0.08)',
        }}>
          {ticket?.status ?? 'idle'}
        </div>
      </div>

      <div style={{ marginTop: '14px', color: 'rgba(255,232,180,0.7)', fontSize: '12px', lineHeight: 1.5 }}>
        {ticket ? (
          <>
            <div>Queue: {ticket.queue}</div>
            <div>Ticket: {ticket.ticketId}</div>
            <div>Updated: {formatDateTime(ticket.updatedAt)}</div>
            {ticket.matchedWith && <div>Matched with: {ticket.matchedWith}</div>}
            {ticket.assignedRoom && <div>Assigned room: {ticket.assignedRoom}</div>}
          </>
        ) : (
          <div>{hostedRuntime ? 'Not in queue yet. Join the selected queue to wait for another online player.' : 'Not in queue yet. Join the selected queue to create a local matchmaking ticket.'}</div>
        )}
      </div>

      <div style={{ marginTop: '16px', display: 'flex', gap: '8px' }}>
        {!ticket || ticket.status === 'cancelled' ? (
          <button
            onClick={() => void handleJoin(side)}
            disabled={loading || restoringTickets || !profile}
            style={{
              flex: 1,
              padding: '10px 12px',
              borderRadius: '9px',
              border: '1px solid rgba(255,180,60,0.25)',
              background: 'linear-gradient(180deg, rgba(200,134,10,0.32) 0%, rgba(122,79,8,0.42) 100%)',
              color: '#fff2c8',
              fontWeight: 800,
              cursor: loading || restoringTickets || !profile ? 'not-allowed' : 'pointer',
            }}
          >
            Join {queue}
          </button>
        ) : ticket.status === 'queued' ? (
          <button
            onClick={() => void handleCancel(side)}
            disabled={loading || restoringTickets}
            style={{
              flex: 1,
              padding: '10px 12px',
              borderRadius: '9px',
              border: '1px solid rgba(231,76,60,0.28)',
              background: 'linear-gradient(180deg, rgba(120,24,24,0.4) 0%, rgba(72,12,12,0.52) 100%)',
              color: '#ffd6cf',
              fontWeight: 800,
              cursor: loading || restoringTickets ? 'not-allowed' : 'pointer',
            }}
          >
            Cancel Ticket
          </button>
        ) : (
          <button
            onClick={() => void handleOpenMatch(ticket)}
            disabled={loading || restoringTickets || !ticket.assignedRoom}
            style={{
              flex: 1,
              padding: '10px 12px',
              borderRadius: '9px',
              border: '1px solid rgba(255,180,60,0.25)',
              background: 'linear-gradient(180deg, rgba(200,134,10,0.32) 0%, rgba(122,79,8,0.42) 100%)',
              color: '#fff2c8',
              fontWeight: 800,
              cursor: loading || restoringTickets || !ticket.assignedRoom ? 'not-allowed' : 'pointer',
            }}
          >
            Open match
          </button>
        )}
      </div>
    </div>
  );

  return (
    <div style={{ display: 'flex', flex: 1, minHeight: 0, padding: '22px 28px 26px', gap: '18px' }}>
      <div style={{
        width: '360px',
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        minHeight: 0,
        background: 'linear-gradient(180deg, rgba(14,18,30,0.98) 0%, rgba(9,12,20,0.96) 100%)',
        border: '1px solid rgba(255,165,40,0.16)',
        borderRadius: '14px',
        boxShadow: '0 12px 40px rgba(0,0,0,0.35)',
        overflow: 'hidden',
      }}>
        <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Queue Control</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            {hostedRuntime
              ? 'Online queue control for this browser session. One browser now behaves like one player by default.'
              : 'Local platform queue tickets with simple auto-match when a second player joins.'}
          </div>
          {restoringTickets && (
            <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', marginTop: '8px' }}>
              Resuming saved queue tickets from this browser...
            </div>
          )}
          <div style={{ display: 'flex', gap: '8px', marginTop: '14px' }}>
            {(['casual', 'rated'] as QueueName[]).map(name => (
              <button
                key={name}
                onClick={() => setQueue(name)}
                disabled={restoringTickets}
                style={{
                  flex: 1,
                  padding: '9px 12px',
                  borderRadius: '9px',
                  border: queue === name ? '1px solid rgba(255,180,60,0.3)' : '1px solid rgba(255,255,255,0.08)',
                  background: queue === name
                    ? 'linear-gradient(180deg, rgba(200,134,10,0.22) 0%, rgba(70,42,8,0.34) 100%)'
                    : 'rgba(255,255,255,0.03)',
                  color: queue === name ? '#fff1c7' : 'rgba(255,232,180,0.75)',
                  textTransform: 'uppercase',
                  fontSize: '11px',
                  fontWeight: 800,
                  cursor: restoringTickets ? 'not-allowed' : 'pointer',
                  opacity: restoringTickets ? 0.7 : 1,
                }}
              >
                {name}
              </button>
            ))}
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '12px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {queueCard('white', whiteProfile, whiteTicket, hostedRuntime ? 'Your player' : undefined)}
          {!hostedRuntime && queueCard('black', blackProfile, blackTicket)}
          {error && (
            <div style={{
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
        </div>
      </div>

      <div style={{
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
      }}>
        <div style={{ padding: '18px 20px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Live Queue Snapshot</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            Current tickets in the selected queue. This is the first real matchmaking surface before region routing and rating bands.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {queueTickets.length === 0 ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>No tickets in {queue} yet.</div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '12px' }}>
              {queueTickets.map(ticket => (
                <div
                  key={ticket.ticketId}
                  style={{
                    padding: '14px',
                    borderRadius: '12px',
                    background: 'rgba(255,255,255,0.03)',
                    border: '1px solid rgba(255,165,40,0.08)',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px', alignItems: 'center' }}>
                    <div style={{ color: '#fff2c8', fontWeight: 800, fontSize: '12px' }}>{ticket.ticketId}</div>
                    <div style={{ color: ticket.status === 'matched' ? '#ffe4a0' : ticket.status === 'queued' ? '#9ee6b8' : 'rgba(255,255,255,0.7)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase' }}>
                      {ticket.status}
                    </div>
                  </div>
                  <div style={{ marginTop: '8px', color: 'rgba(255,232,180,0.72)', fontSize: '12px', lineHeight: 1.5 }}>
                    <div>Guest: {ticket.guestId}</div>
                    <div>Rating: {ticket.rating}</div>
                    <div>Created: {formatDateTime(ticket.createdAt)}</div>
                    {ticket.assignedRoom && <div>Room: {ticket.assignedRoom}</div>}
                    {ticket.matchedWith && <div>Vs: {ticket.matchedWith}</div>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
