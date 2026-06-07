import path from 'node:path';

const nextConfig = {
  transpilePackages: ['@chess404/contracts', '@chess404/game-core'],
  outputFileTracingRoot: path.join(process.cwd(), '../..'),
};

export default nextConfig;
