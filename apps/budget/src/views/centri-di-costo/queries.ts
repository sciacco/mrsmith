import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import { sharedKeys } from '../../api/shared-queries';
import type {
  MessageResponse,
  CostCenterDetails,
  CostCenterNew,
  CostCenterEdit,
} from '../../api/types';

export { useCostCenters, useUsers, useGroups } from '../../api/shared-queries';

export const costCenterKeys = {
  details: (name: string) => ['budget', 'cost-center-details', name] as const,
};

export function useCostCenterDetails(name: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: costCenterKeys.details(name!),
    queryFn: async () => {
      const encoded = encodeURIComponent(name!);
      return api.get<CostCenterDetails>(`/budget/v1/cost-center/${encoded}`);
    },
    enabled: !!name,
  });
}

export function useCreateCostCenter() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CostCenterNew) =>
      api.post<MessageResponse>('/budget/v1/cost-center', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: sharedKeys.costCenters });
    },
  });
}

export function useEditCostCenter() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: { name: string; body: CostCenterEdit }) => {
      const encoded = encodeURIComponent(name);
      return api.put<MessageResponse>(`/budget/v1/cost-center/${encoded}`, body);
    },
    onSuccess: (_data, variables) => {
      const { name, body } = variables;
      queryClient.invalidateQueries({ queryKey: sharedKeys.costCenters });
      if (body.new_name && body.new_name !== name) {
        queryClient.removeQueries({ queryKey: costCenterKeys.details(name) });
      } else {
        queryClient.invalidateQueries({ queryKey: costCenterKeys.details(name) });
      }
    },
  });
}

export function useDisableCostCenter() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => {
      const encoded = encodeURIComponent(name);
      return api.put<MessageResponse>(`/budget/v1/cost-center/${encoded}`, { enabled: false });
    },
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: sharedKeys.costCenters });
      queryClient.invalidateQueries({ queryKey: costCenterKeys.details(name) });
    },
  });
}

export function useEnableCostCenter() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => {
      const encoded = encodeURIComponent(name);
      return api.put<MessageResponse>(`/budget/v1/cost-center/${encoded}`, { enabled: true });
    },
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: sharedKeys.costCenters });
      queryClient.invalidateQueries({ queryKey: costCenterKeys.details(name) });
    },
  });
}
