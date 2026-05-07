const backendBaseUrl = (process.env.PLATFORM_SERVICE_INTERNAL_URL ?? process.env.NEXT_PUBLIC_PLATFORM_SERVICE_URL ?? 'http://127.0.0.1:8083').replace(/\/$/, '');

export async function proxyPlatform(request: Request, path: string): Promise<Response> {
  const url = `${backendBaseUrl}${path}`;
  const init: RequestInit = {
    method: request.method,
    headers: filterHeaders(request.headers),
    cache: 'no-store',
  };

  if (request.method !== 'GET' && request.method !== 'HEAD') {
    init.body = await request.text();
  }

  const upstream = await fetch(url, init);
  const body = await upstream.text();

  return new Response(body, {
    status: upstream.status,
    headers: filterResponseHeaders(upstream.headers),
  });
}

function filterHeaders(headers: Headers): Headers {
  const next = new Headers();
  headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (lower === 'host' || lower === 'connection' || lower === 'content-length') {
      return;
    }
    next.set(key, value);
  });
  return next;
}

function filterResponseHeaders(headers: Headers): Headers {
  const next = new Headers();
  headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (lower === 'content-length' || lower === 'connection' || lower === 'transfer-encoding') {
      return;
    }
    next.set(key, value);
  });
  return next;
}
