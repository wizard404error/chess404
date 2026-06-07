import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

function safeOrigin(raw: string | undefined | null): string | null {
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

function extraConnectOrigins(): string[] {
  const origins = new Set<string>();
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

function buildCsp(): string {
  return [
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
}

export function middleware(request: NextRequest): NextResponse {
  const response = NextResponse.next();

  response.headers.set('Content-Security-Policy', buildCsp());
  response.headers.set('X-Frame-Options', 'DENY');
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  response.headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');

  return response;
}

export const config = {
  matcher: [
    '/((?!_next/static|_next/image|favicon.ico|pieces/|background.png).*)',
  ],
};
