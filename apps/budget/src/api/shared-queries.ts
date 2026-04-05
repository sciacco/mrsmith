import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { PaginatedResponse, ArakIntUser, Group, CostCenter } from './types';

export const sharedKeys = {
  users: ['budget', 'users'] as const,
  groups: ['budget', 'groups'] as const,
  costCenters: ['budget', 'cost-centers'] as const,
};

export function useUsers() {
  const api = useApiClient();
  return useQuery({
    queryKey: sharedKeys.users,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<ArakIntUser>>(
        '/users-int/v1/user?page_number=1&disable_pagination=true&enabled=true',
      );
      return res.items;
    },
  });
}

export function useGroups() {
  const api = useApiClient();
  return useQuery({
    queryKey: sharedKeys.groups,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<Group>>(
        '/budget/v1/group?page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}

export function useCostCenters() {
  const api = useApiClient();
  return useQuery({
    queryKey: sharedKeys.costCenters,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<CostCenter>>(
        '/budget/v1/cost-center?page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}
