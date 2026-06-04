'use client';

import { useEffect, useRef, useState, useCallback } from 'react';

export interface EngineEval {
  score: number;
  mate: number | null;
  best: string;
  depth: number;
}

export const useStockfish = (shouldInit: boolean) => {
  const engineRef      = useRef<Worker | null>(null);
  const readyRef       = useRef(false);
  const activeRef      = useRef(false);
  const urlRef         = useRef<string>('');
  const currentTurnRef = useRef<'white' | 'black'>('white');
  const pendingRef     = useRef<{ fen: string; turn: 'white' | 'black' } | null>(null);
  const genRef         = useRef(0);

  const [isReady,    setIsReady]    = useState(false);
  const [isThinking, setIsThinking] = useState(false);
  const [ev,         setEv]         = useState<EngineEval | null>(null);
  const [sfErr,      setSfErr]      = useState('');

  // ── Send a new position and start infinite search ─────────────────────────
  const sendSearch = useCallback((fen: string, turn: 'white' | 'black') => {
    const w = engineRef.current;
    if (!w) return;
    currentTurnRef.current = turn;
    activeRef.current      = true;
    pendingRef.current     = null;
    w.postMessage(`position fen ${fen}`);
    w.postMessage('go infinite');
    setIsThinking(true);
  }, []);

  const spawnWorker = useCallback((url: string) => {
    const myGen = ++genRef.current;

    if (engineRef.current) {
      try { engineRef.current.terminate(); } catch { /* ignore */ }
      engineRef.current = null;
    }
    readyRef.current  = false;
    activeRef.current = false;
    setIsReady(false);
    setIsThinking(false);

    let worker: Worker;
    try { worker = new Worker(url); }
    catch (e) { setSfErr(`Worker creation failed: ${e}`); return; }

    engineRef.current = worker;

    worker.onmessage = (e: MessageEvent) => {
      if (myGen !== genRef.current) return;

      const line: string = typeof e.data === 'string' ? e.data : '';
      if (!line) return;

      if (line === 'uciok') {
        worker.postMessage('setoption name Hash value 64');
        worker.postMessage('setoption name Threads value 1');
        worker.postMessage('isready');
        return;
      }

      if (line === 'readyok') {
        readyRef.current = true;
        setIsReady(true);
        const p = pendingRef.current;
        if (p) sendSearch(p.fen, p.turn);
        return;
      }

      // Stream live evals as they come in — updates every depth increment
      if (line.startsWith('info') && line.includes('score') && activeRef.current) {
        const depthM = line.match(/depth (\d+)/);
        const cpM    = line.match(/score cp (-?\d+)/);
        const mateM  = line.match(/score mate (-?\d+)/);
        // Skip incomplete info lines (no depth yet)
        if (!depthM) return;
        const depth = parseInt(depthM[1]);
        const flip  = currentTurnRef.current === 'black' ? -1 : 1;

        // Also extract best move from info line if present (pv = principal variation)
        const pvM = line.match(/ pv ([a-h][1-8][a-h][1-8][qrbn]?)/);
        const liveBest = pvM ? pvM[1] : undefined;

        if (cpM) {
          const score = (parseInt(cpM[1]) * flip) / 100;
          setEv(prev => ({ score, mate: null, best: liveBest ?? prev?.best ?? '', depth }));
        }
        if (mateM) {
          const raw    = parseInt(mateM[1]);
          const mateIn = raw * flip;
          setEv(prev => ({
            score: mateIn > 0 ? 999 : -999,
            mate:  mateIn,
            best:  liveBest ?? prev?.best ?? '',
            depth,
          }));
        }
      }

      // bestmove only fires after we send "stop"
      if (line.startsWith('bestmove')) {
        activeRef.current = false;
        setIsThinking(false);
        const uci = line.split(' ')[1];
        if (uci && uci !== '(none)') {
          setEv(prev => prev ? { ...prev, best: uci } : null);
        }
        // Fire any queued search (e.g. user navigated while engine was running)
        const p = pendingRef.current;
        if (p) sendSearch(p.fen, p.turn);
      }
    };

    worker.onerror = (e) => {
      if (myGen !== genRef.current) return;
      console.warn('Stockfish worker error — restarting…', e.message);
      activeRef.current = false;
      setIsThinking(false);
      if (urlRef.current) {
        setTimeout(() => {
          if (myGen !== genRef.current) return;
          spawnWorker(urlRef.current);
        }, 300);
      } else {
        setSfErr(`Stockfish error: ${e.message}`);
      }
    };

    worker.postMessage('uci');
  }, [sendSearch]);

  useEffect(() => {
    if (!shouldInit) {
      genRef.current++;
      activeRef.current = false;
      if (engineRef.current) {
        engineRef.current.terminate();
        engineRef.current = null;
      }
      return;
    }

    const base = (process.env.PUBLIC_URL || '').replace(/\/$/, '');
    const candidates = [
      `${base}/stockfish-18-lite-single.js`,
      `/stockfish-18-lite-single.js`,
    ];

    const init = async () => {
      let url = '';
      for (const p of candidates) {
        try {
          const res  = await fetch(p);
          const text = await res.text();
          if (res.ok && !text.trimStart().startsWith('<')) { url = p; break; }
        } catch { /* skip */ }
      }
      if (!url) {
        setSfErr(
          'stockfish-18-lite-single.js not found. ' +
          'Run: npm install stockfish, then copy stockfish-18-lite-single.js ' +
          'and stockfish-18-lite-single.wasm from node_modules/stockfish/src/ ' +
          'into your public/ folder and restart.'
        );
        return;
      }
      urlRef.current = url;
      spawnWorker(url);
    };

    init();
    return () => {
      genRef.current++;
      activeRef.current = false;
      engineRef.current?.terminate();
      engineRef.current = null;
    };
  }, [shouldInit, spawnWorker]);

  // ── analyse ───────────────────────────────────────────────────────────────
  const analyse = useCallback((fen: string, turn: 'white' | 'black' = 'white') => {
    if (!engineRef.current || !readyRef.current) {
      pendingRef.current = { fen, turn };
      return;
    }

    if (activeRef.current) {
      // Queue the new position and stop current search.
      // bestmove handler will fire the queued search once engine is idle.
      pendingRef.current = { fen, turn };
      engineRef.current.postMessage('stop');
      return;
    }

    sendSearch(fen, turn);
  }, [sendSearch]);

  // ── stop ─────────────────────────────────────────────────────────────────
  const stop = useCallback(() => {
    pendingRef.current = null;
    activeRef.current  = false;
    if (engineRef.current) {
      try { engineRef.current.postMessage('stop'); } catch { /* ignore */ }
    }
    setIsThinking(false);
  }, []);

  const resetEval = useCallback(() => setEv(null), []);

  return { isReady, isThinking, ev, sfErr, analyse, stop, resetEval };
};
