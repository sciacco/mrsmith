import { ApiError, createApiClient, type ApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

const rdaRoot = '/rda/v1';

export function useApiClient(): ApiClient {
  const { getAccessToken, forceRefreshToken } = useOptionalAuth();

  return useMemo(
    () => {
      const client = createApiClient({
        baseUrl: '/api',
        getToken: getAccessToken,
        forceRefreshToken,
      });
      return import.meta.env.DEV ? createRdaDevApiClient(client) : client;
    },
    [forceRefreshToken, getAccessToken],
  );
}

function createRdaDevApiClient(client: ApiClient): ApiClient {
  async function withDiagnostics<T>(method: string, path: string, request: () => Promise<T>): Promise<T> {
    try {
      return await request();
    } catch (error) {
      logRdaApiError(method, path, error);
      throw error;
    }
  }

  return {
    get: <T>(path: string) => withDiagnostics('GET', path, () => client.get<T>(path)),
    post: <T>(path: string, body?: unknown) => withDiagnostics('POST', path, () => client.post<T>(path, body)),
    put: <T>(path: string, body?: unknown) => withDiagnostics('PUT', path, () => client.put<T>(path, body)),
    patch: <T>(path: string, body?: unknown) => withDiagnostics('PATCH', path, () => client.patch<T>(path, body)),
    delete: <T>(path: string) => withDiagnostics('DELETE', path, () => client.delete<T>(path)),
    getBlob: (path: string) => withDiagnostics('GET', path, () => client.getBlob(path)),
    postBlob: (path: string, body?: unknown) => withDiagnostics('POST', path, () => client.postBlob(path, body)),
    postFormData: <T>(path: string, body: FormData) => withDiagnostics('POST', path, () => client.postFormData<T>(path, body)),
    patchFormData: <T>(path: string, body: FormData) => withDiagnostics('PATCH', path, () => client.patchFormData<T>(path, body)),
  };
}

function logRdaApiError(method: string, path: string, error: unknown) {
  if (!(error instanceof ApiError) || !path.startsWith(rdaRoot)) return;

  const body = error.body;
  const upstream = body && typeof body === 'object' && 'upstream' in body ? body.upstream : undefined;

  console.error('[RDA API] request failed', {
    method,
    path,
    status: error.status,
    statusText: error.statusText,
    body,
    upstream,
    error,
  });
}
