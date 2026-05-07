import { proxyRealtime } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  return proxyRealtime(request, '/api/system/status');
}
