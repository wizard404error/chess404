import path from 'node:path';
import { withSentryConfig } from '@sentry/nextjs';

const nextConfig = {
  transpilePackages: ['@chess404/contracts', '@chess404/game-core'],
  outputFileTracingRoot: path.join(process.cwd(), '../..'),
};

export default withSentryConfig(nextConfig, {
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  silent: process.env.NODE_ENV === 'production',
  widenClientFileUpload: true,
  hideSourceMaps: true,
  disableLogger: true,
  tunnelRoute: '/monitoring',
});
