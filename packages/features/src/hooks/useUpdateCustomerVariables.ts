import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { customersKeys } from '../api/customers';

interface UpdateVariablesVars {
  customerId: number;
  variables: Array<{ resource: string; action: string }> | null;
}

export function useUpdateCustomerVariables() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, variables }: UpdateVariablesVars) => {
      return api.put<unknown>(`/cp-backoffice/v1/customers/${customerId}/variables`, { variables });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: customersKeys.all });
    },
  });
}
