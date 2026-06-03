export const dynamic = 'force-dynamic';

export async function POST(request: Request): Promise<Response> {
  if (isLocalRequest(request)) {
    return Response.json({
      error: 'client-side account result finalization is disabled; use the trusted backend finalizer',
    }, { status: 403 });
  }
  return Response.json({
    error: 'client-side account result finalization is disabled',
  }, { status: 404 });
}

function isLocalRequest(request: Request): boolean {
  if (process.env.NODE_ENV === 'production') {
    return false;
  }
  const host = request.headers.get('host')?.toLowerCase() ?? '';
  return host.startsWith('localhost') || host.startsWith('127.0.0.1');
}
