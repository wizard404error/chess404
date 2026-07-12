import Link from 'next/link';

export default function NotFound() {
  return (
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
        fontSize: '72px',
        fontWeight: 900,
        margin: 0,
        color: '#ffbe5a',
        lineHeight: 1,
      }}>
        404
      </h1>
      <p style={{
        fontSize: '18px',
        fontWeight: 600,
        margin: 0,
        color: '#f4efe6',
      }}>
        Page not found
      </p>
      <p style={{
        fontSize: '14px',
        color: 'rgba(244,239,230,0.6)',
        maxWidth: '400px',
        lineHeight: 1.6,
        margin: 0,
      }}>
        The page you&apos;re looking for doesn&apos;t exist or has been moved.
      </p>
      <Link
        href="/play"
        style={{
          padding: '12px 28px',
          borderRadius: '10px',
          border: '1px solid rgba(255,190,90,0.35)',
          background: 'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)',
          color: '#fff8e0',
          fontWeight: 700,
          fontSize: '14px',
          cursor: 'pointer',
          textDecoration: 'none',
          marginTop: '4px',
        }}
      >
        Go to Play
      </Link>
    </div>
  );
}
