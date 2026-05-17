import { proxyInternalService, proxyInternalServiceStream } from '../../_lib/internal-service';

export async function proxyPlatform(request: Request, path: string): Promise<Response> {
  return proxyInternalService(request, path, {
    explicitUrl: process.env.PLATFORM_SERVICE_INTERNAL_URL,
    fallbackUrl: 'http://platform-service.railway.internal:8080',
    envName: 'PLATFORM_SERVICE_INTERNAL_URL',
    serviceName: 'platform-service',
  });
}

export async function proxyPlatformStream(request: Request, path: string): Promise<Response> {
  return proxyInternalServiceStream(request, path, {
    explicitUrl: process.env.PLATFORM_SERVICE_INTERNAL_URL,
    fallbackUrl: 'http://platform-service.railway.internal:8080',
    envName: 'PLATFORM_SERVICE_INTERNAL_URL',
    serviceName: 'platform-service',
  });
}
