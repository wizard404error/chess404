'use client';

import * as React from 'react';
import type { MatchSnapshotMessage, PlayerIntent } from '@chess404/contracts';
import { applyIntent, connectToMatchStream, createMatch, fetchMatch } from '../lib/match-service';

type IntentWithoutMatch = PlayerIntent extends infer T
  ? T extends { matchId: string }
    ? Omit<T, 'matchId'>
    : never
  : never;

let _clientMoveCounter = 0;

function nextClientMoveId(): string {
  _clientMoveCounter += 1;
  return "mv_" + Date.now() + "_" + _clientMoveCounter + "_" + Math.random().toString(36).slice(2, 9);
}

export function useAuthoritativeMatch() {
  const [snapshot, setSnapshot] = React.useState<MatchSnapshotMessage | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [isStreaming, setIsStreaming] = React.useState(false);

  // Ref to hold latest snapshot, avoiding stale closures in callbacks
  const snapshotRef = React.useRef(snapshot);
  snapshotRef.current = snapshot;

  const create = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const next = await createMatch();
      setSnapshot(next);
      return next;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create match';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const refresh = React.useCallback(async (matchId?: string) => {
    const id = matchId ?? snapshotRef.current?.match.matchId;
    if (!id) {
      throw new Error('No match id available');
    }

    setIsLoading(true);
    setError(null);
    try {
      const next = await fetchMatch(id);
      setSnapshot(next);
      return next;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch match';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const sendIntent = React.useCallback(async (intent: IntentWithoutMatch) => {
    const matchId = snapshotRef.current?.match.matchId;
    if (!matchId) {
      throw new Error('Create a match before sending intents');
    }

    const withId = {
      ...intent,
      clientMoveId: intent.clientMoveId ?? nextClientMoveId(),
    };

    setIsLoading(true);
    setError(null);
    try {
      const next = await applyIntent(matchId, withId);
      setSnapshot(next);
      return next;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to apply intent';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  React.useEffect(() => {
    const matchId = snapshotRef.current?.match.matchId;
    if (!matchId) {
      setIsStreaming(false);
      return;
    }

    setIsStreaming(true);
    const { disconnect } = connectToMatchStream(matchId, {
      onSnapshot: (next) => {
        setSnapshot(next);
        setError(null);
      },
      onError: () => {
        setIsStreaming(false);
      }
    });

    return () => {
      setIsStreaming(false);
      disconnect();
    };
  }, [snapshot?.match.matchId]);

  // Polling fallback: only poll if NOT streaming (or as periodic sync)
  // Reduced interval and skip when WS is active
  React.useEffect(() => {
    const currentSnapshot = snapshotRef.current;
    const matchId = currentSnapshot?.match.matchId;
    if (!matchId || currentSnapshot?.match.status !== 'active') {
      return;
    }

    const interval = window.setInterval(() => {
      // Skip polling if the stream is currently connected
      if (isStreaming) {
        return;
      }
      void fetchMatch(matchId).then(next => {
        setSnapshot(next);
        setError(null);
      }).catch(() => {
        // Ignore periodic refresh errors; websocket and explicit actions already surface failures.
      });
    }, 5000);

    return () => window.clearInterval(interval);
  }, [snapshot?.match.matchId, snapshot?.match.status, isStreaming]);

  return {
    snapshot,
    isLoading,
    isStreaming,
    error,
    create,
    refresh,
    sendIntent
  };
}
