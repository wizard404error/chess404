import { proxyRealtime } from '../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  return proxyRealtime(request, `/api/matches/${matchId}`);
}

export async function POST(
  request: Request,
  context: { params: Promise<{ matchId: string }> }
): Promise<Response> {
  const { matchId } = await context.params;
  return proxyRealtime(request, `/api/matches/${matchId}/intents`);
}
