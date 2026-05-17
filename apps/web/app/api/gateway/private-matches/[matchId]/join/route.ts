import { proxyGateway } from '../../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  return proxyGateway(request, `/api/private-matches/${encodeURIComponent(matchId)}/join`);
}
