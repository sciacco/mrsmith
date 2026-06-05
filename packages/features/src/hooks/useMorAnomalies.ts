import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { morAnomaliesKeys, type MorAnomaly } from '../api/morAnomalies';

export function useMorAnomalies() {
  const api = useApiClient();
  return useQuery({
    queryKey: morAnomaliesKeys.all,
    queryFn: () => api.get<MorAnomaly[]>('/reports/v1/mor-anomalies'),
  });
}
