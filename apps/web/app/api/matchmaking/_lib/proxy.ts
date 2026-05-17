import { proxyInternalService } from '../../_lib/internal-service';

export async function proxyMatchmaking(request: Request, path: string): Promise<Response> {
  return proxyInternalService(request, path, {
    explicitUrl: process.env.MATCHMAKING_SERVICE_INTERNAL_URL,
    fallbackUrl: 'http://matchmaking-service.railway.internal:8080',
    envName: 'MATCHMAKING_SERVICE_INTERNAL_URL',
    serviceName: 'matchmaking-service',
  });
}
