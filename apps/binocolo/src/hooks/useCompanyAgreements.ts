import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type { MACompanyAgreement, MACompanyAgreementWrite } from '../api/types';

export const companyAgreementsKey = (companyKey: string) => ['ma-company-agreements', companyKey] as const;

export function useCompanyAgreements(companyKey: string) {
  const api = useApiClient();
  return useQuery({
    queryKey: companyAgreementsKey(companyKey),
    queryFn: () => api.get<MACompanyAgreement[]>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/agreements`),
    enabled: Boolean(companyKey),
  });
}

export function useCompanyAgreementMutations(companyKey: string) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const root = `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/agreements`;
  // Every mutation writes a diary event ('accordo') into the shared activity feed.
  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: companyAgreementsKey(companyKey) }),
      queryClient.invalidateQueries({ queryKey: ['ma-company-activity', companyKey] }),
      queryClient.invalidateQueries({ queryKey: ['ma-companies-search'] }),
    ]);
  const create = useMutation({ mutationFn: (body: MACompanyAgreementWrite) => api.post<MACompanyAgreement>(root, body), onSuccess: invalidate });
  const update = useMutation({ mutationFn: ({ id, body }: { id: string; body: MACompanyAgreementWrite }) => api.put<MACompanyAgreement>(`${root}/${encodeURIComponent(id)}`, body), onSuccess: invalidate });
  const remove = useMutation({ mutationFn: (id: string) => api.delete<void>(`${root}/${encodeURIComponent(id)}`), onSuccess: invalidate });
  return { create, update, remove };
}
