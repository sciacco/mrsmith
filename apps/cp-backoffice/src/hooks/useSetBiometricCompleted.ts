import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import {
  biometricKeys,
  type CompletionRequest,
  type CompletionResponse,
} from '../api/biometric';

interface SetCompletedVariables {
  id: number;
  completed: boolean;
}

// Wraps POST /api/cp-backoffice/v1/biometric-requests/{id}/completion.
// On success: invalidates the list cache so the table refetches with the
// persisted server state. Errors are left for the caller to surface via a
// toast fallback (FINAL.md §Slice S5c).
export function useSetBiometricCompleted() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, completed }: SetCompletedVariables) =>
      api.post<CompletionResponse>(
        `/cp-backoffice/v1/biometric-requests/${id}/completion`,
        { completed } satisfies CompletionRequest,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: biometricKeys.all });
    },
  });
}
