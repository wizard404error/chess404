import { proxyRealtime } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(request: Request): Promise<Response> {
  return proxyRealtime(request, '/api/matches');
}
