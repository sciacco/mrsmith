import { ApiError } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body?: unknown) => Promise<T>;
  patch: <T>(path: string, body: unknown) => Promise<T>;
  getBlob: (path: string) => Promise<Blob>;
}

export function useRDFApiClient(): ApiClient {
  const { getAccessToken } = useOptionalAuth();

  return useMemo(() => {
    async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
      const token = await getAccessToken();
      const res = await fetch(`/api${path}`, {
        method,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });

      if (!res.ok) {
        let responseBody: unknown;
        try { responseBody = await res.json(); } catch { /* ignore */ }
        throw new ApiError(res.status, res.statusText, path, responseBody);
      }

      return res.json() as Promise<T>;
    }

    async function getBlob(path: string): Promise<Blob> {
      const token = await getAccessToken();
      const res = await fetch(`/api${path}`, {
        method: 'GET',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });

      if (!res.ok) {
        let responseBody: unknown;
        try { responseBody = await res.json(); } catch { /* ignore */ }
        throw new ApiError(res.status, res.statusText, path, responseBody);
      }

      return res.blob();
    }

    return {
      get: <T>(path: string) => request<T>('GET', path),
      post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
      patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
      getBlob,
    };
  }, [getAccessToken]);
}
