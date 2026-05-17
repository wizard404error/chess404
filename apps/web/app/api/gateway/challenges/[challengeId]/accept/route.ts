import { proxyGateway } from '../../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function POST(
  request: Request,
  context: { params: Promise<{ challengeId: string }> }
): Promise<Response> {
  const { challengeId } = await context.params;
  return proxyGateway(request, `/api/challenges/${encodeURIComponent(challengeId)}/accept`);
}
