import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { biometricKeys, type BiometricRequestRow } from '../api/biometric';

// Fetches the flat biometric-requests list from
// GET /api/cp-backoffice/v1/biometric-requests. Backend applies
// ORDER BY data_richiesta DESC and returns the full list (no pagination).
export function useBiometricRequests() {
  const api = useApiClient();
  return useQuery({
    queryKey: biometricKeys.all,
    queryFn: () => api.get<BiometricRequestRow[]>('/cp-backoffice/v1/biometric-requests'),
  });
}
