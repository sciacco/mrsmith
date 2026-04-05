import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import type { PaginatedResponse, Budget, ArakIntUser } from '../../api/types';

export const reportKeys = {
  budgetAlerts: (pct: number) =>
    ['budget', 'report-over-percentage', pct] as const,
  unassignedUsers: ['budget', 'report-unassigned-users'] as const,
};

export function useBudgetAlerts(percentage: number | null) {
  const api = useApiClient();
  const normalized =
    percentage !== null ? Math.round(percentage * 10) / 10 : null;

  return useQuery({
    queryKey: reportKeys.budgetAlerts(normalized!),
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<Budget>>(
        `/budget/v1/report/budget-used-over-percentage?percentage=${normalized}&page_number=1&disable_pagination=true`,
      );
      return res.items;
    },
    enabled: normalized !== null,
  });
}

export function useUnassignedUsers() {
  const api = useApiClient();
  return useQuery({
    queryKey: reportKeys.unassignedUsers,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<ArakIntUser>>(
        '/budget/v1/report/unassigned-users?enabled=true&page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}
