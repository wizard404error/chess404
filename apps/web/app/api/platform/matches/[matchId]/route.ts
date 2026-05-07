import { proxyPlatform } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  return proxyPlatform(request, `/api/platform/matches/${matchId}`);
}
