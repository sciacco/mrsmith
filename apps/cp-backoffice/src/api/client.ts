import { createApiClient } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

// All /api/cp-backoffice/v1/* calls go through this client so Bearer tokens,
// refresh-once-on-401, and the local-auth preflight all behave like every
// other mini-app (see apps/compliance/src/api/client.ts).
export function useApiClient() {
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
