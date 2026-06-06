import path from 'node:path';

function safeOrigin(raw) {
  if (!raw) return null;
  const trimmed = String(raw).trim().replace(/\/$/, '');
  if (!trimmed) return null;
  if (trimmed.includes('${{') || /:\s*$/.test(trimmed)) return null;
  try {
    const u = new URL(trimmed);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return null;
    if (!u.host) return null;
    return `${u.protocol}//${u.host}`;
  } catch {
    return null;
  }
}

function extraConnectOrigins() {
  const origins = new Set();
  for (const key of [
    'NEXT_PUBLIC_MATCH_SERVICE_HTTP_BASE',
    'NEXT_PUBLIC_MATCH_SERVICE_WS_URL',
    'NEXT_PUBLIC_MATCH_SERVICE_URL',
  ]) {
    const origin = safeOrigin(process.env[key]);
    if (origin) origins.add(origin);
    if (origin && origin.startsWith('https://')) {
      origins.add('wss://' + origin.slice('https://'.length));
      origins.add('http://' + origin.slice('https://'.length));
    } else if (origin && origin.startsWith('http://')) {
      origins.add('ws://' + origin.slice('http://'.length));
    }
  }
  return Array.from(origins);
}

const csp = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  "connect-src 'self' ws: wss: " + extraConnectOrigins().join(' '),
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join('; ');

const nextConfig = {
  transpilePackages: ['@chess404/contracts', '@chess404/game-core'],
  outputFileTracingRoot: path.join(process.cwd(), '../..'),
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          { key: 'Content-Security-Policy', value: csp },
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
        ],
      },
    ];
  },
};

export default nextConfig;
