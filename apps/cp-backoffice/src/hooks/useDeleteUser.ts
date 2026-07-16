import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { usersKeys } from '../api/users';

export interface DeleteUserInput {
  userId: number;
  customerId: number;
}

// useDeleteUser calls DELETE /api/cp-backoffice/v1/users/{id}.
// Irreversible upstream (Mistra NG /users/v2/user/{userId}); the caller is
// responsible for gating the mutation behind the type-to-confirm dialog.
export function useDeleteUser() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: DeleteUserInput) =>
      api.delete<unknown>(`/cp-backoffice/v1/users/${input.userId}`),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: usersKeys.byCustomer(vars.customerId) });
    },
  });
}
