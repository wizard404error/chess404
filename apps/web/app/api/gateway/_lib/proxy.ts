import { proxyInternalService } from '../../_lib/internal-service';

export async function proxyGateway(request: Request, path: string): Promise<Response> {
  return proxyInternalService(request, path, {
    explicitUrl: process.env.GATEWAY_INTERNAL_URL,
    fallbackUrl: 'http://gateway.railway.internal:8080',
    envName: 'GATEWAY_INTERNAL_URL',
    serviceName: 'gateway',
  });
}
