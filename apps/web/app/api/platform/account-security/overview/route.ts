import { proxyPlatform } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(request: Request) {
  return proxyPlatform(request, '/api/platform/account-security/overview');
}
