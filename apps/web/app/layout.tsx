import './globals.css';
import type { Metadata } from 'next';
import { Cinzel, Source_Sans_3 } from 'next/font/google';

const cinzel = Cinzel({
  subsets: ['latin'],
  variable: '--font-cinzel',
  weight: ['600', '700'],
  display: 'swap',
});

const sourceSans = Source_Sans_3({
  subsets: ['latin'],
  variable: '--font-source-sans',
  weight: ['400', '500', '600', '700', '800'],
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Chess404',
  description: 'Chess404 is competitive online chess with curated card powers.'
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${cinzel.variable} ${sourceSans.variable}`}>
      <body>{children}</body>
    </html>
  );
}
