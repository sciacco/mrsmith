import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { timooDailyStatsKeys, type TimooDailyStat } from '../api/timooDailyStats';

export function useTimooDailyStats() {
  const api = useApiClient();
  return useQuery({
    queryKey: timooDailyStatsKeys.all,
    queryFn: () => api.get<TimooDailyStat[]>('/reports/v1/timoo/daily-stats'),
  });
}
