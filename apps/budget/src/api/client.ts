import { createApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

export function useApiClient() {
  const { token } = useOptionalAuth();
  return useMemo(
    () => createApiClient({
      baseUrl: '/api',
      getToken: () => token,
    }),
    [token],
  );
}
