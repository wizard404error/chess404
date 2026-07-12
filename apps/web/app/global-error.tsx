'use client';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html>
      <body>
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          padding: '32px',
          background: '#0a0d16',
          color: '#f4efe6',
          textAlign: 'center',
          fontFamily: "'Segoe UI', system-ui, sans-serif",
          gap: '20px',
          margin: 0,
        }}>
          <div style={{ fontSize: '48px', lineHeight: 1 }}>♟</div>
          <div style={{
            fontSize: '13px',
            fontWeight: 700,
            letterSpacing: '0.15em',
            textTransform: 'uppercase',
            color: '#ffbe5a',
          }}>
            CHESS404
          </div>
          <h1 style={{
            fontSize: '24px',
            fontWeight: 800,
            margin: 0,
            color: '#ffbe5a',
          }}>
            Critical error
          </h1>
          <p style={{
            fontSize: '14px',
            color: 'rgba(244,239,230,0.6)',
            maxWidth: '480px',
            lineHeight: 1.6,
            margin: 0,
          }}>
            A critical error occurred. Please reload the application.
          </p>
          <div style={{
            padding: '12px 16px',
            background: 'rgba(255,190,90,0.06)',
            border: '1px solid rgba(255,190,90,0.2)',
            borderRadius: '10px',
            fontSize: '13px',
            color: '#ffbe5a',
            maxWidth: '100%',
            overflow: 'auto',
            textAlign: 'left',
            fontFamily: 'monospace',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}>
            {error?.message ?? 'Unknown error'}
          </div>
          <button
            onClick={() => reset()}
            style={{
              padding: '12px 28px',
              borderRadius: '10px',
              border: '1px solid rgba(255,190,90,0.35)',
              background: 'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)',
              color: '#fff8e0',
              fontWeight: 700,
              fontSize: '14px',
              cursor: 'pointer',
              marginTop: '4px',
              transition: 'opacity 0.15s',
            }}
            onMouseOver={e => (e.currentTarget.style.opacity = '0.85')}
            onMouseOut={e => (e.currentTarget.style.opacity = '1')}
          >
            Reload
          </button>
        </div>
      </body>
    </html>
  );
}
