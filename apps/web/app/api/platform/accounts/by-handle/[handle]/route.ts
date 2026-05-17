import { proxyPlatform } from '../../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ handle: string }> },
): Promise<Response> {
  const { handle } = await context.params;
  const { search } = new URL(request.url);
  return proxyPlatform(request, `/api/platform/accounts/by-handle/${encodeURIComponent(handle)}${search}`);
}
