import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '@mrsmith/api-client';
import { useApiClient } from '../api/client';
import { customersKeys, type UpdateStateRequest } from '../api/customers';

// formatErrorToast preserves the business-facing toast format locked by
// FINAL.md §Slice S5a: `{HTTP status} — {upstream message}` on a business
// error, with `Qualcosa e' andato storto` as the fallback when we have no
// structured error to show.
//
// The backend wraps upstream business errors as
//   { error: 'upstream_error', message: '<msg>' }
// and forwards the upstream status code. ApiError exposes both.
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

// useUpdateCustomerState drives the modal Confirm. On success it invalidates
// the customer list so the table refetches; on error the caller surfaces
// the formatted toast via formatErrorToast.
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
