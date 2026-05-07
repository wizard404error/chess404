import { proxyMatchmaking } from '../../../_lib/proxy';

export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  context: { params: Promise<{ ticketId: string }> }
): Promise<Response> {
  const { ticketId } = await context.params;
  return proxyMatchmaking(request, `/api/queues/tickets/${ticketId}`);
}

export async function DELETE(
  request: Request,
  context: { params: Promise<{ ticketId: string }> }
): Promise<Response> {
  const { ticketId } = await context.params;
  return proxyMatchmaking(request, `/api/queues/tickets/${ticketId}`);
}
