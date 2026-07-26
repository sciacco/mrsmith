import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type { MACompanyActivityResponse, MATargetOutcome } from '../api/types';

export function companyActivityKey(companyKey: string, includeDeleted = false) {
  return ['ma-company-activity', companyKey, includeDeleted] as const;
}

export function useCompanyActivity(companyKey: string, options: { includeDeleted?: boolean } = {}) {
  const api = useApiClient();
  const includeDeleted = options.includeDeleted ?? false;
  return useQuery({
    queryKey: companyActivityKey(companyKey, includeDeleted),
    queryFn: () =>
      api.get<MACompanyActivityResponse>(
        `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/activity${includeDeleted ? '?includeDeleted=true' : ''}`,
      ),
    enabled: Boolean(companyKey),
  });
}

export function useAnnotationMutations(companyKey: string, initiativeId?: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['ma-company-activity', companyKey] }),
      queryClient.invalidateQueries({ queryKey: ['ma-board'] }),
      queryClient.invalidateQueries({ queryKey: ['ma-pipeline'] }),
      queryClient.invalidateQueries({ queryKey: ['ma-initiatives', 'active'] }),
      queryClient.invalidateQueries({ queryKey: ['ma-company-overview', companyKey] }),
    ]);
  };

  const create = useMutation({
    mutationFn: (body: string) =>
      api.post<MATargetOutcome>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/annotations`, {
        body,
        ...(initiativeId ? { initiativeId } : {}),
      }),
    onSuccess: invalidate,
  });
  const update = useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      api.patch<void>(`/binocolo/v1/ma/annotations/${encodeURIComponent(id)}`, { body }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.delete<void>(`/binocolo/v1/ma/annotations/${encodeURIComponent(id)}`),
    onSuccess: invalidate,
  });

  return { create, update, remove };
}
