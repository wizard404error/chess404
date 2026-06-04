'use client';

import React from 'react';
import type { Board, Snapshot } from '../types';

interface UseMatchReplayProps {
  snapshots: Snapshot[];
  over: boolean;
  resetEval: () => void;
}

export function useMatchReplay({
  snapshots,
  over,
  resetEval,
}: UseMatchReplayProps) {
  const [reviewIdx, setReviewIdx] = React.useState<number>(-1);
  const [reviewBoard, setReviewBoard] = React.useState<Board | null>(null);

  const goToSnap = React.useCallback((idx: number) => {
    if (idx < 0 || idx >= snapshots.length) return;
    const s = snapshots[idx];
    setReviewIdx(idx);
    setReviewBoard(s.board);
    resetEval();
  }, [snapshots, resetEval]);

  const reviewFirst = React.useCallback(() => goToSnap(0), [goToSnap]);
  const reviewPrev  = React.useCallback(() => goToSnap(reviewIdx <= 0 ? 0 : reviewIdx - 1), [goToSnap, reviewIdx]);
  const reviewNext  = React.useCallback(() => {
    if (reviewIdx < snapshots.length - 1) goToSnap(reviewIdx + 1);
    else { setReviewIdx(-1); setReviewBoard(null); resetEval(); }
  }, [goToSnap, reviewIdx, snapshots.length, resetEval]);
  const reviewLast  = React.useCallback(() => { setReviewIdx(-1); setReviewBoard(null); resetEval(); }, [resetEval]);

  const isReviewing = reviewIdx >= 0 && over;

  return {
    reviewIdx,
    setReviewIdx,
    reviewBoard,
    setReviewBoard,
    isReviewing,
    goToSnap,
    reviewFirst,
    reviewPrev,
    reviewNext,
    reviewLast,
  };
}
