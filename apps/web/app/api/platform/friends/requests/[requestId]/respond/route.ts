import { proxyPlatform } from '../../../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(
  request: Request,
  context: { params: Promise<{ requestId: string }> },
): Promise<Response> {
  const params = await context.params;
  return proxyPlatform(request, `/api/platform/friends/requests/${encodeURIComponent(params.requestId)}/respond`);
}
