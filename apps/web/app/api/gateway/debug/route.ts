import { proxyGateway } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  return proxyGateway(request, '/api/gateway/debug');
}
