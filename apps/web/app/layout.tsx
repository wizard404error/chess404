import './globals.css';
import type { Metadata } from 'next';
import { headers } from 'next/headers';
import { Inter, JetBrains_Mono } from 'next/font/google';
import ClientApp from './ClientApp';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-sans',
  weight: ['400', '500', '600', '700', '800'],
  display: 'swap',
});

const jetBrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
  weight: ['500', '700'],
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Chess404',
  description: 'Chess404 is competitive online chess with curated card powers.'
};

// Render every request dynamically so runtime config (match-service URLs, etc.)
// is read from env vars at request time, not baked into a 1-year CDN cache at
// build time. See apps/web/middleware.ts for the matching Cache-Control header.
export const dynamic = 'force-dynamic';
export const revalidate = 0;

function resolveMatchServiceHttpBase(): string {
  return (process.env.NEXT_PUBLIC_MATCH_SERVICE_HTTP_BASE ?? process.env.NEXT_PUBLIC_MATCH_SERVICE_URL ?? '/api/realtime').replace(/\/$/, '');
}

function resolveMatchServiceWsBase(): string {
  const explicit = (process.env.NEXT_PUBLIC_MATCH_SERVICE_WS_URL ?? process.env.NEXT_PUBLIC_MATCH_SERVICE_URL ?? '').replace(/\/$/, '');
  if (explicit) return explicit;
  const httpBase = (process.env.NEXT_PUBLIC_MATCH_SERVICE_HTTP_BASE ?? '').replace(/\/$/, '');
  if (httpBase.endsWith('/api')) return httpBase.slice(0, -4);
  return httpBase;
}

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const nonce = (await headers()).get('x-nonce') ?? '';
  return (
    <html lang="en" className={`${inter.variable} ${jetBrainsMono.variable}`} nonce={nonce}>
      <head>
        <link rel="manifest" href="/manifest.json" />
      </head>
      <body>
        <a href="#main-content" className="skip-link">
          Skip to content
        </a>
        <ClientApp runtimeConfig={{
          matchServiceHttpBase: resolveMatchServiceHttpBase(),
          matchServiceWsBase: resolveMatchServiceWsBase(),
        }}>
          {children}
        </ClientApp>
      </body>
    </html>
  );
}

