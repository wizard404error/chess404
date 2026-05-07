import { proxyPlatform } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ accountId: string }> },
): Promise<Response> {
  const { accountId } = await context.params;
  return proxyPlatform(request, `/api/platform/accounts/${encodeURIComponent(accountId)}`);
}
