import App from '../src/App';

function resolveMatchServiceHttpBase(): string {
  return (process.env.NEXT_PUBLIC_MATCH_SERVICE_HTTP_BASE ?? process.env.NEXT_PUBLIC_MATCH_SERVICE_URL ?? '/api/realtime').replace(/\/$/, '');
}

function resolveMatchServiceWsBase(): string {
  const explicit = (process.env.NEXT_PUBLIC_MATCH_SERVICE_WS_URL ?? process.env.NEXT_PUBLIC_MATCH_SERVICE_URL ?? '').replace(/\/$/, '');
  if (explicit) {
    return explicit;
  }

  const httpBase = (process.env.NEXT_PUBLIC_MATCH_SERVICE_HTTP_BASE ?? '').replace(/\/$/, '');
  if (httpBase.endsWith('/api')) {
    return httpBase.slice(0, -4);
  }
  return httpBase;
}

export default function HomePage() {
  return (
    <App
      runtimeConfig={{
        matchServiceHttpBase: resolveMatchServiceHttpBase(),
        matchServiceWsBase: resolveMatchServiceWsBase(),
      }}
    />
  );
}
