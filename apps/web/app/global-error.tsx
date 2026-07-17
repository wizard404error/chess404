'use client';

import * as Sentry from '@sentry/nextjs';
import Error from 'next/error';
import { useEffect } from 'react';

export default function GlobalError({ error }: { error: Error & { digest?: string } }) {
  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  return (
    <html>
      <body style={{ margin: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', background: '#0a0616', color: '#e8dcc8', fontFamily: 'sans-serif' }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: '48px', marginBottom: '16px' }}>⚠️</div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, margin: '0 0 8px' }}>Something went wrong</h1>
          <p style={{ fontSize: '13px', color: 'rgba(200,185,140,0.7)', margin: '0 0 20px' }}>This error has been reported.</p>
          <button onClick={() => window.location.reload()} style={{
            padding: '10px 24px', background: '#c8860a', color: '#fff', border: 'none',
            borderRadius: '8px', cursor: 'pointer', fontWeight: 700, fontSize: '13px',
          }}>
            Reload Page
          </button>
        </div>
      </body>
    </html>
  );
}
