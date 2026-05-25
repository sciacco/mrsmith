import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import { sharedKeys } from '../../api/shared-queries';
import type {
  PaginatedResponse,
  ArakIntUser,
  ArakIntRole,
  ArakIntUserNew,
  ArakIntUserEdit,
  IdResponse,
  MessageResponse,
} from '../../api/types';

export const userKeys = {
  all: sharedKeys.allUsers,
  roles: sharedKeys.roles,
};

export function useAllUsers() {
  const api = useApiClient();
  return useQuery({
    queryKey: userKeys.all,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<ArakIntUser>>(
        '/users-int/v1/user?page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}

export function useRoles() {
  const api = useApiClient();
  return useQuery({
    queryKey: userKeys.roles,
    queryFn: async () => {
      const res = await api.get<PaginatedResponse<ArakIntRole>>(
        '/users-int/v1/role?page_number=1&disable_pagination=true',
      );
      return res.items;
    },
  });
}

function invalidateUserLists(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: userKeys.all });
  queryClient.invalidateQueries({ queryKey: sharedKeys.users });
}

export function useCreateUser() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ArakIntUserNew) =>
      api.post<IdResponse>('/users-int/v1/user', body),
    onSuccess: () => invalidateUserLists(queryClient),
  });
}

export function useEditUser() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: ArakIntUserEdit }) =>
      api.put<MessageResponse>(`/users-int/v1/user/${id}`, body),
    onSuccess: () => invalidateUserLists(queryClient),
  });
}

export function useDeleteUser() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      api.delete<MessageResponse>(`/users-int/v1/user/${id}`),
    onSuccess: () => invalidateUserLists(queryClient),
  });
}
