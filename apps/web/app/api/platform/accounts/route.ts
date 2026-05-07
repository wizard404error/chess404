import { proxyPlatform } from '../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(request: Request): Promise<Response> {
  const { search } = new URL(request.url);
  return proxyPlatform(request, `/api/platform/accounts${search}`);
}
