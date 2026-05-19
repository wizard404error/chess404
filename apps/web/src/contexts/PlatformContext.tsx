'use client';

import React, { createContext, useContext } from 'react';

// Using 'any' for rapid iteration during this massive refactor. 
// Can be strictly typed later.
export const PlatformContext = createContext<any>(null);

export function usePlatform() {
  const context = useContext(PlatformContext);
  if (!context) {
    throw new Error('usePlatform must be used within a PlatformContext.Provider');
  }
  return context;
}
