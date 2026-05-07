import { proxyMatchmaking } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  return proxyMatchmaking(request, '/api/status');
}
