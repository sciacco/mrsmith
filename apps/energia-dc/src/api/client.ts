import { createApiClient, type ApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

export function useApiClient(): ApiClient {
  const { getAccessToken, forceRefreshToken } = useOptionalAuth();

  return useMemo(
    () =>
      createApiClient({
        baseUrl: '/api',
        getToken: getAccessToken,
        forceRefreshToken,
      }),
    [forceRefreshToken, getAccessToken],
  );
}
