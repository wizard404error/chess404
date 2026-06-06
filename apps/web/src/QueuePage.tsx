'use client';

import React from 'react';
import type { MatchModeId } from '@chess404/contracts';
import { DEFAULT_MATCH_MODE_ID, OFFICIAL_MATCH_MODES } from '@chess404/contracts';
import type { GuestProfile } from './lib/platform-service';
import type { QueueName, QueueSnapshot, QueueTicket } from './lib/matchmaking-service';
import { cancelTicket, enqueueGuest, fetchQueueSnapshots, fetchQueueTickets, fetchTicket, RateLimitError } from './lib/matchmaking-service';
import { ensureMatch, readStoredRoomMeta, resolveSeatSecret, writeStoredRoomMeta, type StoredRoomMeta } from './lib/match-service';

interface QueuePageProps {
  whiteProfile: GuestProfile | null;
  blackProfile: GuestProfile | null;
  preferredQueue?: QueueName | null;
  preferredModeId?: MatchModeId | null;
  recoveredWhiteTicket?: QueueTicket | null;
  recoveredBlackTicket?: QueueTicket | null;
  recoveryReady?: boolean;
  embedded?: boolean;
}

type QueueSide = 'white' | 'black';

interface StoredTicketRef {
  ticketId: string;
  queue: QueueName;
  modeId: MatchModeId;
}

const DEFAULT_QUEUE: QueueName = 'casual';
const QUEUE_SELECTION_STORAGE_KEY = 'chess404.queue.selection';
const MODE_SELECTION_STORAGE_KEY = 'chess404.mode.selection';

function queueTicketStorageKey(side: QueueSide): string {
  return `chess404.queue.${side}.ticket`;
}

function guestSessionStorageKey(side: QueueSide): string {
  return `chess404.guest.${side}.secret`;
}

function accountIdStorageKey(side: QueueSide): string {
  return `chess404.account.${side}.id`;
}

function accountTokenStorageKey(side: QueueSide): string {
  return `chess404.account.${side}.token`;
}

function readStoredQueueSelection(): QueueName {
  if (typeof window === 'undefined') {
    return DEFAULT_QUEUE;
  }
  const value = window.localStorage.getItem(QUEUE_SELECTION_STORAGE_KEY);
  return value === 'rated' ? 'rated' : DEFAULT_QUEUE;
}

function normalizeModeId(value?: string | null): MatchModeId {
  return value === 'hidden_cards' ? 'hidden_cards' : DEFAULT_MATCH_MODE_ID;
}

function readStoredModeSelection(): MatchModeId {
  if (typeof window === 'undefined') {
    return DEFAULT_MATCH_MODE_ID;
  }
  return normalizeModeId(window.localStorage.getItem(MODE_SELECTION_STORAGE_KEY));
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
      modeId: normalizeModeId(parsed.modeId),
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
    modeId: ticket.modeId ?? DEFAULT_MATCH_MODE_ID,
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

function readStoredAccountSession(side: QueueSide): { accountId: string | null; sessionToken: string | null } {
  if (typeof window === 'undefined') {
    return { accountId: null, sessionToken: null };
  }
  return {
    accountId: window.localStorage.getItem(accountIdStorageKey(side)),
    sessionToken: window.localStorage.getItem(accountTokenStorageKey(side)),
  };
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function modeLabel(modeId?: MatchModeId): string {
  return OFFICIAL_MATCH_MODES.find(mode => mode.id === normalizeModeId(modeId))?.label ?? 'Open Cards';
}

export default function QueuePage({
  whiteProfile,
  blackProfile,
  preferredQueue = null,
  preferredModeId = null,
  recoveredWhiteTicket = null,
  recoveredBlackTicket = null,
  recoveryReady = false,
  embedded = false,
}: QueuePageProps): React.ReactElement {
  const [hostedRuntime, setHostedRuntime] = React.useState(false);
  const [queue, setQueue] = React.useState<QueueName>(() => readStoredQueueSelection());
  const [modeId, setModeId] = React.useState<MatchModeId>(() => readStoredModeSelection());
  const [whiteTicket, setWhiteTicket] = React.useState<QueueTicket | null>(null);
  const [blackTicket, setBlackTicket] = React.useState<QueueTicket | null>(null);
  const [queueTickets, setQueueTickets] = React.useState<QueueTicket[]>([]);
  const [queueSnapshot, setQueueSnapshot] = React.useState<QueueSnapshot | null>(null);
  const [error, setError] = React.useState('');
  const [loading, setLoading] = React.useState(false);
  const [restoringTickets, setRestoringTickets] = React.useState(true);
  const hostedAutoOpenMatchRef = React.useRef<string | null>(null);
  const whiteStoredAccount = readStoredAccountSession('white');
  const blackStoredAccount = readStoredAccountSession('black');
  const whiteRatedReady = Boolean((whiteStoredAccount.accountId ?? '').trim() && (whiteStoredAccount.sessionToken ?? '').trim());
  const blackRatedReady = Boolean((blackStoredAccount.accountId ?? '').trim() && (blackStoredAccount.sessionToken ?? '').trim());
  const hostedRatedReady = whiteRatedReady;

  const refreshQueue = React.useCallback(async (queueName: QueueName, nextModeId: MatchModeId) => {
    const [tickets, snapshotPayload] = await Promise.all([
      fetchQueueTickets(queueName, nextModeId),
      fetchQueueSnapshots(queueName, nextModeId),
    ]);
    setQueueTickets(tickets);
    setQueueSnapshot(snapshotPayload.snapshots[0] ?? {
      queue: queueName,
      modeId: nextModeId,
      queuedCount: 0,
      matchedCount: 0,
      cancelledCount: 0,
    });
  }, []);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const hostname = window.location.hostname.toLowerCase();
    setHostedRuntime(hostname !== 'localhost' && hostname !== '127.0.0.1');
  }, []);

  React.useEffect(() => {
    void refreshQueue(queue, modeId).catch(err => {
      setError(err instanceof Error ? err.message : 'Failed to load queue state.');
    });
  }, [queue, modeId, refreshQueue]);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(QUEUE_SELECTION_STORAGE_KEY, queue);
  }, [queue]);

  React.useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(MODE_SELECTION_STORAGE_KEY, modeId);
  }, [modeId]);

  React.useEffect(() => {
    return () => {
      const whiteRef = readStoredTicketRef('white');
      const blackRef = readStoredTicketRef('black');
      const ticketId = whiteRef?.ticketId ?? blackRef?.ticketId;
      if (ticketId) {
        cancelTicket(ticketId).catch(() => {});
        clearStoredTicketRef('white');
        clearStoredTicketRef('black');
      }
    };
  }, []);

  React.useEffect(() => {
    let cancelled = false;

    const restoreTickets = async () => {
      const whiteRef = readStoredTicketRef('white');
      const blackRef = readStoredTicketRef('black');

      if (whiteRef) {
        setQueue(whiteRef.queue);
        setModeId(whiteRef.modeId);
      } else if (blackRef) {
        setQueue(blackRef.queue);
        setModeId(blackRef.modeId);
      }

      if (!whiteRef && !blackRef) {
        setRestoringTickets(false);
        return;
      }

      const hydrate = async (side: QueueSide, stored: StoredTicketRef | null): Promise<QueueTicket | null> => {
        if (!stored) return null;
        try {
          const { ticket } = await fetchTicket(stored.ticketId);
          return ticket;
        } catch (err) {
          const message = err instanceof Error ? err.message : '';
          if (message.includes('404')) {
            clearStoredTicketRef(side);
          }
          return null;
        }
      };

      const [restoredWhite, restoredBlack] = await Promise.all([
        hydrate('white', whiteRef),
        hydrate('black', blackRef),
      ]);
      if (cancelled) return;
      setWhiteTicket(restoredWhite);
      setBlackTicket(restoredBlack);
    };

    void restoreTickets().finally(() => {
      if (!cancelled) setRestoringTickets(false);
    });

    return () => { cancelled = true; };
  }, []);

  React.useEffect(() => {
    writeStoredTicketRef('white', whiteTicket);
  }, [whiteTicket]);

  React.useEffect(() => {
    writeStoredTicketRef('black', blackTicket);
  }, [blackTicket]);

  React.useEffect(() => {
    if (!recoveryReady) {
      return;
    }

    const nextWhite = recoveredWhiteTicket && recoveredWhiteTicket.status === 'queued' ? recoveredWhiteTicket : null;
    const nextBlack = recoveredBlackTicket && recoveredBlackTicket.status === 'queued' ? recoveredBlackTicket : null;

    setWhiteTicket(current => {
      if (nextWhite) {
        if (current?.ticketId === nextWhite.ticketId && current.status === nextWhite.status) {
          return current;
        }
        return nextWhite;
      }
      if (current?.status === 'queued') {
        clearStoredTicketRef('white');
        return null;
      }
      return current;
    });

    setBlackTicket(current => {
      if (nextBlack) {
        if (current?.ticketId === nextBlack.ticketId && current.status === nextBlack.status) {
          return current;
        }
        return nextBlack;
      }
      if (current?.status === 'queued') {
        clearStoredTicketRef('black');
        return null;
      }
      return current;
    });

    const activeTicket = nextWhite ?? nextBlack;
    if (activeTicket) {
      setQueue(activeTicket.queue);
      setModeId(normalizeModeId(activeTicket.modeId));
    }
  }, [recoveryReady, recoveredWhiteTicket, recoveredBlackTicket]);

  React.useEffect(() => {
    if (!hostedRuntime) {
      return;
    }
    clearStoredTicketRef('black');
    setBlackTicket(null);
  }, [hostedRuntime]);

  React.useEffect(() => {
    if (!preferredQueue || whiteTicket || blackTicket) {
      return;
    }
    setQueue(preferredQueue);
  }, [preferredQueue, whiteTicket, blackTicket]);

  React.useEffect(() => {
    if (!preferredModeId || whiteTicket || blackTicket) {
      return;
    }
    setModeId(normalizeModeId(preferredModeId));
  }, [preferredModeId, whiteTicket, blackTicket]);

  const buildHostedAssignedRoomMeta = React.useCallback((ticket: QueueTicket, profile: GuestProfile | null): StoredRoomMeta | null => {
    if (!ticket.assignedRoom || !ticket.seatColor || !profile) {
      return null;
    }
    const existingRoomMeta = readStoredRoomMeta(ticket.assignedRoom);
    const currentAccountId = readStoredAccountId('white') ?? undefined;
    const viewerSeat = ticket.seatColor;
    const opponentGuestId = ticket.matchedWith || undefined;
    const opponentName = ticket.opponentName || existingRoomMeta?.[viewerSeat === 'white' ? 'blackName' : 'whiteName'];

    return {
      ...existingRoomMeta,
      queue: ticket.queue,
      modeId: ticket.modeId ?? existingRoomMeta?.modeId ?? DEFAULT_MATCH_MODE_ID,
      viewerSeat,
      whiteGuestId: viewerSeat === 'white' ? profile.guestId : opponentGuestId ?? existingRoomMeta?.whiteGuestId,
      blackGuestId: viewerSeat === 'black' ? profile.guestId : opponentGuestId ?? existingRoomMeta?.blackGuestId,
      whiteAccountId: viewerSeat === 'white' ? currentAccountId : existingRoomMeta?.whiteAccountId,
      blackAccountId: viewerSeat === 'black' ? currentAccountId : existingRoomMeta?.blackAccountId,
      whiteName: viewerSeat === 'white' ? profile.displayName : opponentName ?? existingRoomMeta?.whiteName,
      blackName: viewerSeat === 'black' ? profile.displayName : opponentName ?? existingRoomMeta?.blackName,
    };
  }, []);

  React.useEffect(() => {
    if (!hostedRuntime || restoringTickets) {
      return;
    }
    const ticket = whiteTicket;
    if (!ticket || ticket.status !== 'matched' || !ticket.assignedRoom || !ticket.seatColor || !whiteProfile) {
      if (!ticket || ticket.status !== 'matched') {
        hostedAutoOpenMatchRef.current = null;
      }
      return;
    }
    if (hostedAutoOpenMatchRef.current === ticket.assignedRoom) {
      return;
    }
    const roomMeta = buildHostedAssignedRoomMeta(ticket, whiteProfile);
    if (!roomMeta) {
      return;
    }
    hostedAutoOpenMatchRef.current = ticket.assignedRoom;
    writeStoredRoomMeta(ticket.assignedRoom, roomMeta);
    clearStoredTicketRef('white');
    window.location.href = `/match/${encodeURIComponent(ticket.assignedRoom)}`;
  }, [hostedRuntime, restoringTickets, whiteTicket, whiteProfile, buildHostedAssignedRoomMeta]);

  const pollingBackoffRef = React.useRef(0);

  React.useEffect(() => {
    if (restoringTickets) {
      return;
    }
    if (!whiteTicket && !blackTicket) {
      pollingBackoffRef.current = 0;
      return;
    }

    const interval = window.setInterval(() => {
      const tasks: Promise<void>[] = [];
      if (whiteTicket?.status === 'queued') {
        tasks.push(
          fetchTicket(whiteTicket.ticketId).then(({ ticket }) => {
            pollingBackoffRef.current = 0;
            setWhiteTicket(ticket);
          })
        );
      }
      if (blackTicket?.status === 'queued') {
        tasks.push(
          fetchTicket(blackTicket.ticketId).then(({ ticket }) => {
            pollingBackoffRef.current = 0;
            setBlackTicket(ticket);
          })
        );
      }
      tasks.push(refreshQueue(queue, modeId));
      void Promise.all(tasks).catch((err) => {
        if (err instanceof RateLimitError) {
          pollingBackoffRef.current = Math.min(pollingBackoffRef.current + 1, 4);
        }
      });
    }, 2500 * (pollingBackoffRef.current > 0 ? pollingBackoffRef.current : 1));

    return () => window.clearInterval(interval);
  }, [whiteTicket, blackTicket, queue, modeId, refreshQueue, restoringTickets]);

  const handleJoin = React.useCallback(async (side: 'white' | 'black') => {
    const profile = side === 'white' ? whiteProfile : blackProfile;
    if (!profile) {
      setError('Guest profile is still loading. Try again in a moment.');
      return;
    }
    const accountIdentity = readStoredAccountSession(side);
    if (queue === 'rated' && (!(accountIdentity.accountId ?? '').trim() || !(accountIdentity.sessionToken ?? '').trim())) {
      setError(hostedRuntime
        ? 'Rated queue requires a signed-in Chess404 account. Sign in first, then join rated.'
        : `${side === 'white' ? 'White' : 'Black'} needs a signed-in Chess404 account before joining rated queue.`);
      return;
    }
    setLoading(true);
    setError('');
    try {
      const result = await enqueueGuest(profile.guestId, queue, modeId, profile.rating, profile.displayName, {
        accountId: accountIdentity.accountId ?? undefined,
        accountSessionToken: accountIdentity.sessionToken ?? undefined,
      });
      if (side === 'white') {
        setWhiteTicket(result.ticket);
      } else {
        setBlackTicket(result.ticket);
      }
      await refreshQueue(queue, modeId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to join queue.');
    } finally {
      setLoading(false);
    }
  }, [whiteProfile, blackProfile, queue, modeId, refreshQueue, hostedRuntime]);

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
      await refreshQueue(queue, modeId);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel queue ticket.');
    } finally {
      setLoading(false);
    }
  }, [whiteTicket, blackTicket, queue, modeId, refreshQueue]);

  const openAccount = React.useCallback(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.location.href = '/account';
  }, []);

  const handleOpenMatch = React.useCallback(async (ticket: QueueTicket) => {
    if (!ticket.assignedRoom) {
      return;
    }
    if (hostedRuntime) {
      return;
    }
    setLoading(true);
    setError('');
    try {
      const existingRoomMeta = readStoredRoomMeta(ticket.assignedRoom);
      const roomMeta = {
        ...existingRoomMeta,
        queue,
        modeId: ticket.modeId ?? existingRoomMeta?.modeId ?? modeId,
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
        modeId: roomMeta.modeId,
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
      window.location.href = `/match/${encodeURIComponent(ticket.assignedRoom)}`;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to open matched room.');
    } finally {
      setLoading(false);
    }
  }, [queue, modeId, whiteProfile, blackProfile, hostedRuntime]);

  const queueCard = (
    side: 'white' | 'black',
    profile: GuestProfile | null,
    ticket: QueueTicket | null,
    title?: string,
  ): React.ReactElement => {
    const ratedReady = side === 'white' ? whiteRatedReady : blackRatedReady;
    const ratedBlocked = queue === 'rated' && !ratedReady;
    const playerTitle = title ?? (hostedRuntime ? 'Your player' : profile?.displayName ?? 'Loading...');
    const cardBackground = hostedRuntime
      ? 'linear-gradient(180deg, rgba(16,28,44,0.78) 0%, rgba(8,16,28,0.9) 100%)'
      : side === 'white'
        ? 'linear-gradient(180deg, rgba(10,40,18,0.78) 0%, rgba(8,20,12,0.88) 100%)'
        : 'linear-gradient(180deg, rgba(34,12,58,0.78) 0%, rgba(16,10,32,0.88) 100%)';
    const cardBorder = hostedRuntime
      ? '1px solid rgba(120,190,255,0.22)'
      : side === 'white'
        ? '1px solid rgba(60,220,110,0.24)'
        : '1px solid rgba(180,110,255,0.24)';

    return (
      <div
        style={{
          padding: '18px',
          borderRadius: '14px',
          background: cardBackground,
          border: cardBorder,
          boxShadow: '0 12px 36px rgba(0,0,0,0.28)',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: '10px', alignItems: 'center' }}>
        <div>
          <div style={{ color: hostedRuntime ? '#e8f2ff' : side === 'white' ? '#dcffe9' : '#eedcff', fontSize: '15px', fontWeight: 800 }}>{playerTitle} ({side})</div>
          <div style={{ color: hostedRuntime ? '#9ed0ff' : side === 'white' ? '#7ce3aa' : '#c9a8ff', fontSize: '12px', marginTop: '4px' }}>Rating {profile?.rating ?? 1200}</div>
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
            <div>Lane: {ticket.queue === 'rated' ? 'Rated Quick Pair' : 'Casual Quick Pair'}</div>
            <div>Mode: {modeLabel(ticket.modeId)}</div>
            <div>Updated: {formatDateTime(ticket.updatedAt)}</div>
            {ticket.status === 'queued' ? <div>Searching for another player in this official mode now.</div> : null}
            {ticket.seatColor && <div>Seat: {ticket.seatColor === 'white' ? 'White pieces' : 'Black pieces'}</div>}
            {ticket.opponentName && <div>Opponent: {ticket.opponentName}</div>}
            {ticket.assignedRoom && <div>Your live match is ready.</div>}
          </>
        ) : ratedBlocked ? (
          <div>Rated quick pair is locked until this player is signed in with a Chess404 account.</div>
        ) : (
          <div>{hostedRuntime ? 'Not in queue yet. Choose a lane and join when you are ready to be paired.' : 'Not in queue yet. Choose a lane and create a local matchmaking ticket when ready.'}</div>
        )}
        </div>

        <div style={{ marginTop: '16px', display: 'flex', gap: '8px' }}>
        {!ticket || ticket.status === 'cancelled' ? (
          <button
            className="btn-primary"
            onClick={() => void handleJoin(side)}
            disabled={loading || restoringTickets || !profile || ratedBlocked}
            style={{
              flex: 1,
              padding: '10px 12px',
              opacity: ratedBlocked ? 0.72 : 1,
            }}
          >
            {!profile
              ? 'Preparing player...'
              : `Join ${queue === 'rated' ? 'Rated' : 'Casual'} - ${modeLabel(modeId)}`}
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
        ) : hostedRuntime ? (
          <div
            style={{
              flex: 1,
              padding: '10px 12px',
              borderRadius: '9px',
              border: '1px solid rgba(255,180,60,0.14)',
              background: 'rgba(255,255,255,0.04)',
              color: 'rgba(255,232,180,0.82)',
              fontWeight: 800,
              textAlign: 'center',
            }}
          >
            Matched - opening game...
          </div>
        ) : (
          <button
            className="btn-primary"
            onClick={() => void handleOpenMatch(ticket)}
            disabled={loading || restoringTickets || !ticket.assignedRoom}
            style={{
              flex: 1,
              padding: '10px 12px',
            }}
          >
            Open match
          </button>
        )}
        </div>
      </div>
    );
  };

  const visiblePlayerCount = hostedRuntime
    ? (queueSnapshot?.queuedCount ?? 0) + (queueSnapshot?.matchedCount ?? 0)
    : queueTickets.length;
  const waitingCount = hostedRuntime
    ? (queueSnapshot?.queuedCount ?? 0)
    : queueTickets.filter((entry) => entry.status === 'queued').length;
  const matchesFormingCount = hostedRuntime
    ? (queueSnapshot?.matchedCount ?? 0)
    : queueTickets.filter((entry) => entry.status === 'matched').length;
  const showEmptyActivity = hostedRuntime ? visiblePlayerCount === 0 : queueTickets.length === 0;

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
      flex: embedded ? undefined : 1,
      minHeight: 0,
      padding: embedded ? 0 : '22px 28px 26px',
      gap: '18px',
      alignItems: 'start',
    }}>
      <div className="stat-card" style={{ padding: 0 }}>
        <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid rgba(255,165,40,0.12)' }}>
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Queue Control</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            {hostedRuntime
              ? 'Quick pair for one hosted player. Casual stays open to guests, and rated unlocks after account sign-in.'
              : 'Local platform queue tickets with simple auto-match when a second player joins.'}
          </div>
          {restoringTickets && (
            <div style={{ color: 'rgba(255,232,180,0.62)', fontSize: '11px', marginTop: '8px' }}>
              Resuming queue state from backend recovery...
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
                {hostedRuntime && name === 'rated' && !hostedRatedReady ? 'rated (sign in)' : name}
              </button>
            ))}
          </div>
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            {OFFICIAL_MATCH_MODES.map(mode => (
              <button
                key={mode.id}
                onClick={() => setModeId(mode.id)}
                disabled={restoringTickets}
                style={{
                  flex: 1,
                  padding: '9px 12px',
                  borderRadius: '9px',
                  border: modeId === mode.id ? '1px solid rgba(120,190,255,0.34)' : '1px solid rgba(255,255,255,0.08)',
                  background: modeId === mode.id
                    ? 'linear-gradient(180deg, rgba(54,102,184,0.24) 0%, rgba(24,40,82,0.34) 100%)'
                    : 'rgba(255,255,255,0.03)',
                  color: modeId === mode.id ? '#e5f0ff' : 'rgba(210,225,255,0.75)',
                  fontSize: '11px',
                  fontWeight: 800,
                  cursor: restoringTickets ? 'not-allowed' : 'pointer',
                  opacity: restoringTickets ? 0.7 : 1,
                }}
                title={mode.rulesSummary}
              >
                {mode.shortLabel}
              </button>
            ))}
          </div>
          <div style={{ color: 'rgba(210,225,255,0.68)', fontSize: '11px', marginTop: '8px', lineHeight: 1.45 }}>
            {OFFICIAL_MATCH_MODES.find(mode => mode.id === modeId)?.rulesSummary}
          </div>
          {queue === 'rated' && (
            <div style={{
              marginTop: '10px',
              padding: '10px 12px',
              borderRadius: '10px',
              border: '1px solid rgba(120,190,255,0.18)',
              background: 'rgba(36,54,96,0.26)',
              color: 'rgba(220,236,255,0.86)',
              fontSize: '11px',
              lineHeight: 1.5,
            }}>
              {hostedRuntime
                ? 'Rated quick pair is account-only for launch. Sign in once, then come back here to queue rated with the same hosted player.'
                : 'Rated queue is account-only for launch-quality matchmaking. Sign in each player seat before joining rated. Casual queue still allows guest play.'}
              {hostedRuntime && !hostedRatedReady ? (
                <button
                  onClick={openAccount}
                  style={{
                    marginTop: '10px',
                    padding: '9px 12px',
                    borderRadius: '9px',
                    border: '1px solid rgba(120,190,255,0.28)',
                    background: 'rgba(255,255,255,0.06)',
                    color: '#eef4ff',
                    fontSize: '11px',
                    fontWeight: 800,
                    cursor: 'pointer',
                  }}
                >
                  Sign in to join rated
                </button>
              ) : null}
            </div>
          )}
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
          <div style={{ color: '#ffcf72', fontSize: '13px', fontWeight: 800, letterSpacing: '1.2px', textTransform: 'uppercase' }}>Queue Activity</div>
          <div style={{ color: 'rgba(255,232,180,0.72)', fontSize: '12px', marginTop: '4px' }}>
            See how busy the selected lane is without exposing raw internal ticket details.
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '20px' }}>
          {showEmptyActivity ? (
            <div style={{ color: 'rgba(255,232,180,0.65)', fontSize: '13px' }}>No tickets in {queue} - {modeLabel(modeId)} yet.</div>
          ) : (
            <div style={{ display: 'grid', gap: '14px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '10px' }}>
                <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)' }}>
                  <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>Players visible</div>
                  <div style={{ color: '#fff2c8', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{visiblePlayerCount}</div>
                </div>
                <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)' }}>
                  <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>Waiting now</div>
                  <div style={{ color: '#9ee6b8', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{waitingCount}</div>
                </div>
                <div style={{ padding: '12px 14px', borderRadius: '12px', background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,165,40,0.08)' }}>
                  <div style={{ color: 'rgba(255,232,180,0.56)', fontSize: '10px', fontWeight: 800, letterSpacing: '1px', textTransform: 'uppercase' }}>Matches forming</div>
                  <div style={{ color: '#ffe4a0', fontSize: '22px', fontWeight: 900, marginTop: '6px' }}>{matchesFormingCount}</div>
                </div>
              </div>
              {hostedRuntime ? (
                <div style={{
                  padding: '14px 16px',
                  borderRadius: '12px',
                  background: 'rgba(255,255,255,0.03)',
                  border: '1px solid rgba(255,165,40,0.08)',
                  color: 'rgba(255,232,180,0.72)',
                  fontSize: '12px',
                  lineHeight: 1.6,
                }}>
                  Hosted play keeps this activity view high-level on purpose. You can see whether the lane is moving without exposing raw player ticket diagnostics.
                </div>
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
                      <div style={{ color: '#fff2c8', fontWeight: 800, fontSize: '12px' }}>
                        {ticket.displayName?.trim() || (ticket.status === 'matched' ? 'Matched player' : 'Queued player')}
                      </div>
                      <div style={{ color: ticket.status === 'matched' ? '#ffe4a0' : ticket.status === 'queued' ? '#9ee6b8' : 'rgba(255,255,255,0.7)', fontSize: '10px', fontWeight: 800, textTransform: 'uppercase' }}>
                        {ticket.status === 'matched' ? 'match ready' : ticket.status}
                      </div>
                    </div>
                    <div style={{ marginTop: '8px', color: 'rgba(255,232,180,0.72)', fontSize: '12px', lineHeight: 1.5 }}>
                      <div>Lane: {ticket.queue === 'rated' ? 'Rated Quick Pair' : 'Casual Quick Pair'}</div>
                      <div>Mode: {modeLabel(ticket.modeId)}</div>
                      <div>Rating: {ticket.rating}</div>
                      <div>{ticket.status === 'matched' ? 'Paired' : 'Queued'}: {formatDateTime(ticket.updatedAt)}</div>
                      {ticket.opponentName && <div>Opponent: {ticket.opponentName}</div>}
                      {ticket.assignedRoom && <div>Board opening now.</div>}
                    </div>
                  </div>
                ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
