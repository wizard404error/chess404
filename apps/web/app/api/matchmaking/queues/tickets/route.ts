import { proxyMatchmaking } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  const { search } = new URL(request.url);
  return proxyMatchmaking(request, `/api/queues/tickets${search}`);
}

export async function POST(request: Request): Promise<Response> {
  return proxyMatchmaking(request, '/api/queues/tickets');
}
