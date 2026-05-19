import './globals.css';
import type { Metadata } from 'next';
import { Inter, JetBrains_Mono } from 'next/font/google';

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

import App from '../src/App';

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

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${inter.variable} ${jetBrainsMono.variable}`}>
      <body>
        <App runtimeConfig={{
          matchServiceHttpBase: resolveMatchServiceHttpBase(),
          matchServiceWsBase: resolveMatchServiceWsBase(),
        }}>
          {children}
        </App>
      </body>
    </html>
  );
}
