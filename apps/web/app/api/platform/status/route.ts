import { proxyPlatform } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  return proxyPlatform(request, '/api/platform/status');
}
