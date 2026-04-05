import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import { sharedKeys } from '../../api/shared-queries';
import type { MessageResponse, GroupDetails, GroupNew, GroupEdit } from '../../api/types';

export { useGroups, useUsers } from '../../api/shared-queries';

export const groupKeys = {
  all: sharedKeys.groups,
  details: (name: string) => ['budget', 'group-details', name] as const,
};

export function useGroupDetails(name: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: groupKeys.details(name!),
    queryFn: async () => {
      const encoded = encodeURIComponent(name!);
      return api.get<GroupDetails>(`/budget/v1/group/${encoded}`);
    },
    enabled: !!name,
  });
}

export function useCreateGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: GroupNew) =>
      api.post<MessageResponse>('/budget/v1/group', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.all });
    },
  });
}

export function useEditGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: { name: string; body: GroupEdit }) => {
      const encoded = encodeURIComponent(name);
      return api.put<MessageResponse>(`/budget/v1/group/${encoded}`, body);
    },
    onSuccess: (_data, variables) => {
      const { name, body } = variables;
      queryClient.invalidateQueries({ queryKey: groupKeys.all });
      if (body.new_name && body.new_name !== name) {
        queryClient.removeQueries({ queryKey: groupKeys.details(name) });
      } else {
        queryClient.invalidateQueries({ queryKey: groupKeys.details(name) });
      }
    },
  });
}

export function useDeleteGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => {
      const encoded = encodeURIComponent(name);
      return api.delete<MessageResponse>(`/budget/v1/group/${encoded}`);
    },
    onSuccess: (_data, name) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.all });
      queryClient.removeQueries({ queryKey: groupKeys.details(name) });
    },
  });
}
