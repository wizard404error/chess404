const backendBaseUrl = resolveBackendBaseUrl(
  process.env.GATEWAY_INTERNAL_URL,
  'http://gateway.railway.internal:8080',
);

export async function proxyGateway(request: Request, path: string): Promise<Response> {
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

function resolveBackendBaseUrl(explicit: string | undefined, fallback: string): string {
  const value = explicit?.trim().replace(/\/$/, '');
  if (!value || value.includes('${{') || /:\s*$/.test(value)) {
    return fallback;
  }
  return value;
}
