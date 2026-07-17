'use client';

import React from 'react';

const TUTORIAL_DONE_KEY = 'chess404_tutorial_done';

type TutorialStep = 'welcome' | 'board' | 'cards' | 'complete';

export interface TutorialState {
  active: boolean;
  step: TutorialStep;
  dismiss: () => void;
  next: () => void;
  prev: () => void;
}

export function useTutorial(): TutorialState {
  const [active, setActive] = React.useState(() => {
    if (typeof window === 'undefined') return false;
    return localStorage.getItem(TUTORIAL_DONE_KEY) !== 'true';
  });
  const [step, setStep] = React.useState<TutorialStep>('welcome');

  const finish = React.useCallback(() => {
    setActive(false);
    setStep('complete');
    if (typeof window !== 'undefined') {
      localStorage.setItem(TUTORIAL_DONE_KEY, 'true');
    }
  }, []);

  return {
    active,
    step,
    dismiss: finish,
    next: () => {
      if (step === 'welcome') setStep('board');
      else if (step === 'board') setStep('cards');
      else if (step === 'cards') finish();
    },
    prev: () => {
      if (step === 'board') setStep('welcome');
      else if (step === 'cards') setStep('board');
    },
  };
}
