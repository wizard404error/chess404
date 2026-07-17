import * as Sentry from '@sentry/nextjs';

Sentry.init({
  dsn: process.env.SENTRY_DSN || process.env.NEXT_PUBLIC_SENTRY_DSN,
  environment: process.env.NODE_ENV || 'development',
  tracesSampleRate: 0.2,
  ignoreErrors: [
    'ResizeObserver loop limit exceeded',
    'NetworkError when attempting to fetch resource',
  ],
});
