'use client';

/**
 * ClientApp.tsx
 *
 * Loads the App shell exclusively on the client side (ssr: false).
 *
 * Why: App.tsx reads localStorage during state initialisation and calls
 * configureMatchServiceRuntime() as a render-time side effect.  The server
 * generates HTML with empty/null state while the client hydrates with real
 * localStorage data, causing React hydration mismatches that surface as
 * React error #310 (null dispatcher / invalid hook call).
 *
 * By skipping SSR for the App boundary we:
 *  • Eliminate all hydration mismatches (everything initialises fresh on the
 *    client where localStorage, WebSocket, AudioContext, etc. are available).
 *  • Keep the root layout as a proper Server Component so fonts, metadata and
 *    route-level optimisations still work.
 *  • Show the existing loading skeleton (hostedRuntime === null branch in
 *    App.tsx) until the JS bundle evaluates – no blank-page flash.
 */

import dynamic from 'next/dynamic';
import type React from 'react';

// Dynamically import the full App shell, client-side only.
const App = dynamic(() => import('../src/App'), {
  ssr: false,
  // Render the chess-themed loading skeleton while the bundle loads.
  loading: () => (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: '100vh',
      background: '#0a0d16',
      color: '#ffbe5a',
      gap: '16px',
      fontFamily: "'Segoe UI', sans-serif",
    }}>
      <div style={{ fontSize: '28px', fontWeight: 800, letterSpacing: '2px' }}>
        ♟ CHESS404
      </div>
      <div style={{
        width: '200px',
        height: '4px',
        borderRadius: '2px',
        background: 'rgba(255,190,90,0.15)',
        overflow: 'hidden',
      }}>
        <div style={{
          width: '40%',
          height: '100%',
          borderRadius: '2px',
          background: '#ffbe5a',
          animation: 'loadingSlide 1.2s ease-in-out infinite',
        }} />
      </div>
      <style>{`
        @keyframes loadingSlide {
          0%   { transform: translateX(-100%); }
          50%  { transform: translateX(250%); }
          100% { transform: translateX(-100%); }
        }
      `}</style>
    </div>
  ),
});

interface ClientAppProps {
  runtimeConfig?: {
    matchServiceHttpBase?: string;
    matchServiceWsBase?: string;
  };
  children?: React.ReactNode;
}

export default function ClientApp({ runtimeConfig, children }: ClientAppProps) {
  return <App runtimeConfig={runtimeConfig}>{children}</App>;
}
