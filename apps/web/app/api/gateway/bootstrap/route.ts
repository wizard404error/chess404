import { proxyGateway } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  return proxyGateway(request, '/api/session/bootstrap');
}

export async function POST(request: Request): Promise<Response> {
  return proxyGateway(request, '/api/session/bootstrap');
}
