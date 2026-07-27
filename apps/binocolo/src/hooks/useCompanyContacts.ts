import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type { MACompanyContact, MACompanyContactWrite } from '../api/types';

export const companyContactsKey = (companyKey: string) => ['ma-company-contacts', companyKey] as const;

export function useCompanyContacts(companyKey: string) {
  const api = useApiClient();
  return useQuery({
    queryKey: companyContactsKey(companyKey),
    queryFn: () => api.get<MACompanyContact[]>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/contacts`),
    enabled: Boolean(companyKey),
  });
}

export function useCompanyContactMutations(companyKey: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: companyContactsKey(companyKey) });
  const root = `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/contacts`;
  const create = useMutation({ mutationFn: (body: MACompanyContactWrite) => api.post<MACompanyContact>(root, body), onSuccess: invalidate });
  const update = useMutation({ mutationFn: ({ id, body }: { id: string; body: MACompanyContactWrite }) => api.put<MACompanyContact>(`${root}/${encodeURIComponent(id)}`, body), onSuccess: invalidate });
  const remove = useMutation({ mutationFn: (id: string) => api.delete<void>(`${root}/${encodeURIComponent(id)}`), onSuccess: invalidate });
  return { create, update, remove };
}
