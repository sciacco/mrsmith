import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import { customersKeys, type UpdateStateRequest } from '../api/customers';

export function formatErrorToast(error: unknown): string {
  if (error instanceof ApiError) {
    const msg = pickMessage(error.body);
    if (msg) {
      return `${error.status} \u2014 ${msg}`;
    }
  }
  return "Qualcosa e' andato storto";
}

function pickMessage(body: unknown): string | undefined {
  if (typeof body === 'object' && body !== null && 'message' in body) {
    const raw = (body as { message: unknown }).message;
    if (typeof raw === 'string' && raw.trim().length > 0) return raw;
  }
  return undefined;
}

interface UpdateCustomerStateVars {
  customerId: number;
  stateId: number;
}

export function useUpdateCustomerState() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, stateId }: UpdateCustomerStateVars) => {
      const body: UpdateStateRequest = { state_id: stateId };
      return api.put<unknown>(`/cp-backoffice/v1/customers/${customerId}/state`, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: customersKeys.all });
    },
  });
}
