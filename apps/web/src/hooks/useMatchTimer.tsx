'use client';

import React from 'react';
import type { PieceColor } from '@chess404/contracts';

export interface UseMatchTimerProps {
  initialClockStart: number;
  initialAbortSecs: number;
  over: boolean;
  authoritativeLive: boolean;
  onTimeout: (loser: PieceColor) => void;
  onAbort: () => void;
}

export function useMatchTimer({
  initialClockStart,
  initialAbortSecs,
  over,
  authoritativeLive,
  onTimeout,
  onAbort,
}: UseMatchTimerProps) {
  const [timeW, setTimeW] = React.useState(initialClockStart);
  const [timeB, setTimeB] = React.useState(initialClockStart);
  const [clockActive, setClockActive] = React.useState(false);

  const tickingRef = React.useRef<PieceColor | null>(null);
  const [tickingState, setTickingState] = React.useState<PieceColor | null>(null);

  const [abortCountdown, setAbortCountdown] = React.useState(initialAbortSecs);
  const [abortActive, setAbortActive] = React.useState(true);
  const abortRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

  // Use refs for callbacks to avoid stale closures in intervals
  const onTimeoutRef = React.useRef(onTimeout);
  onTimeoutRef.current = onTimeout;
  const onAbortRef = React.useRef(onAbort);
  onAbortRef.current = onAbort;

  const setTicking = React.useCallback((v: PieceColor | null) => {
    tickingRef.current = v;
    setTickingState(v);
  }, []);

  const stopAbortCountdown = React.useCallback((resetToDefault = false) => {
    if (abortRef.current) {
      clearInterval(abortRef.current);
      abortRef.current = null;
    }
    setAbortActive(false);
    if (resetToDefault) {
      setAbortCountdown(initialAbortSecs);
    }
  }, [initialAbortSecs]);

  const startAbortCountdown = React.useCallback((onStart?: () => void) => {
    if (abortRef.current) clearInterval(abortRef.current);
    setAbortCountdown(initialAbortSecs);
    setAbortActive(true);
    if (onStart) onStart();
    
    let remaining = initialAbortSecs;
    abortRef.current = setInterval(() => {
      remaining -= 1;
      setAbortCountdown(remaining);
      if (remaining <= 0) {
        clearInterval(abortRef.current!);
        abortRef.current = null;
        setAbortActive(false);
        onAbortRef.current();
      }
    }, 1000);
  }, [initialAbortSecs]);

  const resetTimer = React.useCallback(() => {
    setTimeW(initialClockStart);
    setTimeB(initialClockStart);
    if (abortRef.current) clearInterval(abortRef.current);
    abortRef.current = null;
    setTicking(null);
    setClockActive(false);
    setAbortCountdown(initialAbortSecs);
    setAbortActive(true);
  }, [initialClockStart, initialAbortSecs, setTicking]);

  // Clock display is server-driven only — no local countdown.
  // timeW/timeB are updated exclusively from authoritative snapshots or local game ticks.

  return {
    timeW, setTimeW,
    timeB, setTimeB,
    clockActive, setClockActive,
    tickingState, tickingRef, setTicking,
    abortCountdown, setAbortCountdown,
    abortActive, setAbortActive,
    startAbortCountdown, stopAbortCountdown, resetTimer,
    abortRef, setTickingState,
  };
}
