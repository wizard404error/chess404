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
  const clockRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

  const tickingRef = React.useRef<PieceColor | null>(null);
  const [tickingState, setTickingState] = React.useState<PieceColor | null>(null);

  const [abortCountdown, setAbortCountdown] = React.useState(initialAbortSecs);
  const [abortActive, setAbortActive] = React.useState(true);
  const abortRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

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
        onAbort();
      }
    }, 1000);
  }, [initialAbortSecs, onAbort]);

  const resetTimer = React.useCallback(() => {
    setTimeW(initialClockStart);
    setTimeB(initialClockStart);
    if (clockRef.current) clearInterval(clockRef.current);
    clockRef.current = null;
    if (abortRef.current) clearInterval(abortRef.current);
    abortRef.current = null;
    setTicking(null);
    setClockActive(false);
    setAbortCountdown(initialAbortSecs);
    setAbortActive(true);
  }, [initialClockStart, initialAbortSecs, setTicking]);

  React.useEffect(() => {
    if (clockRef.current) clearInterval(clockRef.current);
    if (!clockActive || over || authoritativeLive) return;
    clockRef.current = setInterval(() => {
      const ticking = tickingRef.current;
      if (ticking === null) return;
      if (ticking === 'white') {
        setTimeW(t => {
          if (t <= 1) { clearInterval(clockRef.current!); onTimeout('white'); return 0; }
          return t - 1;
        });
      } else {
        setTimeB(t => {
          if (t <= 1) { clearInterval(clockRef.current!); onTimeout('black'); return 0; }
          return t - 1;
        });
      }
    }, 1000);
    return () => { if (clockRef.current) clearInterval(clockRef.current); };
  }, [clockActive, over, authoritativeLive, onTimeout]);

  return {
    timeW, setTimeW,
    timeB, setTimeB,
    clockActive, setClockActive,
    tickingState, tickingRef, setTicking,
    abortCountdown, setAbortCountdown,
    abortActive, setAbortActive,
    startAbortCountdown, stopAbortCountdown, resetTimer,
    clockRef, abortRef, setTickingState,
  };
}
