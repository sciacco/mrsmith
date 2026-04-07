import { createApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

export function useApiClient() {
  const { getAccessToken } = useOptionalAuth();
  return useMemo(
    () => createApiClient({
      baseUrl: '/api',
      getToken: getAccessToken,
    }),
    [getAccessToken],
  );
}
