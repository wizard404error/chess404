import { proxyPlatform } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ guestId: string }> }
): Promise<Response> {
  const { guestId } = await context.params;
  const { search } = new URL(request.url);
  return proxyPlatform(request, `/api/platform/guests/${guestId}${search}`);
}
