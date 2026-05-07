import './globals.css';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Chess404',
  description: 'Card-powered competitive chess moving toward a server-authoritative architecture.'
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
