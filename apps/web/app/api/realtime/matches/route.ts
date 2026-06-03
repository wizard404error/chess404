import { proxyRealtime } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(request: Request): Promise<Response> {
  if (!isLocalRequest(request)) {
    return Response.json({
      error: 'direct match creation is not public; use the gateway match flow',
    }, { status: 404 });
  }
  return proxyRealtime(request, '/api/matches');
}

function isLocalRequest(request: Request): boolean {
  if (process.env.NODE_ENV === 'production') {
    return false;
  }
  const host = request.headers.get('host')?.toLowerCase() ?? '';
  return host.startsWith('localhost') || host.startsWith('127.0.0.1');
}
