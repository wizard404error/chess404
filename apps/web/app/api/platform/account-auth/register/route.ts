import { proxyPlatform } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(request: Request): Promise<Response> {
  return proxyPlatform(request, '/api/platform/account-auth/register');
}
