import { ApiError } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body: unknown) => Promise<T>;
  patch: <T>(path: string, body: unknown) => Promise<T>;
  delete: <T>(path: string) => Promise<T>;
}

export function useApiClient(): ApiClient {
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
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!res.ok) {
        let payload: unknown;
        try {
          payload = await res.json();
        } catch {
          payload = undefined;
        }
        throw new ApiError(res.status, res.statusText, path, payload);
      }

      if (res.status === 204) {
        return undefined as T;
      }

      return res.json() as Promise<T>;
    }

    return {
      get: <T>(path: string) => request<T>('GET', path),
      post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
      patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
      delete: <T>(path: string) => request<T>('DELETE', path),
    };
  }, [getAccessToken]);
}
