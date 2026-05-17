import { proxyPlatformStream } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(request: Request): Promise<Response> {
  return proxyPlatformStream(request, '/api/platform/inbox/stream');
}
