import { createApiClient, type ApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

export function useApiClient(): ApiClient {
  const { getAccessToken, forceRefreshToken, login } = useOptionalAuth();

  return useMemo(
    () =>
      createApiClient({
        baseUrl: '/api',
        getToken: getAccessToken,
        forceRefreshToken,
        onUnauthorized: () => {
          login();
        },
      }),
    [forceRefreshToken, getAccessToken, login],
  );
}
