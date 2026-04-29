import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type {
  AttachmentType,
  Article,
  BudgetForUser,
  Country,
  CreatePOPayload,
  DefaultPaymentMethod,
  PagedEnvelope,
  PatchPOPayload,
  PaymentMethod,
  PoComment,
  PoDetail,
  PoPreview,
  ProviderPayload,
  ProviderReference,
  ProviderSummary,
  RdaPermissions,
  RdaUser,
  RowPayload,
} from './types';
import { useApiClient } from './client';

const rdaRoot = '/rda/v1';
const fornitoriRoot = '/fornitori/v1';

type DraftProviderResponse = Partial<ProviderSummary> & Pick<ProviderSummary, 'id'>;

function unwrap<T>(value: T[] | PagedEnvelope<T>): T[] {
  return Array.isArray(value) ? value : value.items ?? [];
}

function providerSummaryFromDraft(response: DraftProviderResponse, body: ProviderPayload): ProviderSummary {
  return {
    ...response,
    id: response.id,
    company_name: response.company_name ?? body.company_name,
    state: response.state ?? body.state,
    default_payment_method: response.default_payment_method ?? body.default_payment_method ?? null,
    language: response.language ?? body.language,
    vat_number: response.vat_number ?? body.vat_number,
    ref: response.ref ?? body.ref,
    refs: response.refs ?? (response.ref ? [response.ref] : body.ref ? [body.ref] : undefined),
  };
}

function invalidatePO(queryClient: ReturnType<typeof useQueryClient>, id?: number) {
  queryClient.invalidateQueries({ queryKey: ['rda', 'pos'] });
  queryClient.invalidateQueries({ queryKey: ['rda', 'inbox'] });
  if (id) {
    queryClient.invalidateQueries({ queryKey: ['rda', 'po', id] });
    queryClient.invalidateQueries({ queryKey: ['rda', 'comments', id] });
  }
}

export function usePermissions() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'permissions'],
    queryFn: () => api.get<RdaPermissions>(`${rdaRoot}/me/permissions`),
  });
}

export function useBudgets() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'budgets'],
    queryFn: async () => unwrap(await api.get<BudgetForUser[] | PagedEnvelope<BudgetForUser>>(`${rdaRoot}/budgets`)),
  });
}

export function usePaymentMethods() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'payment-methods'],
    queryFn: () => api.get<PaymentMethod[]>(`${rdaRoot}/payment-methods`),
  });
}

export function usePaymentMethodDefault() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'payment-method-default'],
    queryFn: () => api.get<DefaultPaymentMethod>(`${rdaRoot}/payment-methods/default`),
  });
}

export function useArticleCatalog() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'articles', 'catalog'],
    queryFn: async () => unwrap(await api.get<Article[] | PagedEnvelope<Article>>(`${rdaRoot}/articles`)),
  });
}

export function useUserSearch(search: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'users', search],
    enabled,
    queryFn: async () => {
      const params = new URLSearchParams({ search });
      return unwrap(await api.get<RdaUser[] | PagedEnvelope<RdaUser>>(`${rdaRoot}/users?${params}`));
    },
  });
}

export function useMyPOs() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'pos'],
    queryFn: async () => unwrap(await api.get<PoPreview[] | PagedEnvelope<PoPreview>>(`${rdaRoot}/pos`)),
  });
}

export function useInbox(kind: string | undefined) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'inbox', kind],
    enabled: Boolean(kind),
    queryFn: async () => unwrap(await api.get<PoPreview[] | PagedEnvelope<PoPreview>>(`${rdaRoot}/pos/inbox/${kind}`)),
  });
}

export function usePODetail(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'po', id],
    enabled: id != null,
    queryFn: () => api.get<PoDetail>(`${rdaRoot}/pos/${id}`),
  });
}

export function usePOComments(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['rda', 'comments', id],
    enabled: id != null,
    queryFn: async () => unwrap(await api.get<PoComment[] | PagedEnvelope<PoComment>>(`${rdaRoot}/pos/${id}/comments`)),
  });
}

export function useProviders() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'providers', 'rda'],
    queryFn: async () => {
      const params = new URLSearchParams({ disable_pagination: 'true', page_number: '1', usable: 'true' });
      return unwrap(await api.get<ProviderSummary[] | PagedEnvelope<ProviderSummary>>(`${fornitoriRoot}/provider?${params}`));
    },
  });
}

export function useCountries() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'countries'],
    queryFn: () => api.get<Country[]>(`${fornitoriRoot}/country`),
  });
}

export function useProvider(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'provider', id],
    enabled: id != null,
    queryFn: () => api.get<ProviderSummary>(`${fornitoriRoot}/provider/${id}`),
  });
}

export function useCreatePO() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreatePOPayload) => api.post<PoDetail | PoPreview>(`${rdaRoot}/pos`, body),
    onSuccess: () => invalidatePO(queryClient),
  });
}

export function usePatchPO(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: PatchPOPayload) => api.patch<PoDetail>(`${rdaRoot}/pos/${id}`, body),
    onSuccess: () => invalidatePO(queryClient, id ?? undefined),
  });
}

export function usePatchPaymentMethod(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payment_method: string) => api.patch<PoDetail>(`${rdaRoot}/pos/${id}/payment-method`, { payment_method }),
    onSuccess: () => invalidatePO(queryClient, id ?? undefined),
  });
}

export function useDeletePO() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.delete<void>(`${rdaRoot}/pos/${id}`),
    onSuccess: () => invalidatePO(queryClient),
  });
}

export type TransitionAction =
  | 'submit'
  | 'approve'
  | 'reject'
  | 'leasing/approve'
  | 'leasing/reject'
  | 'leasing/created'
  | 'no-leasing/approve'
  | 'payment-method/approve'
  | 'budget-increment/approve'
  | 'budget-increment/reject'
  | 'conformity/confirm'
  | 'conformity/reject'
  | 'send-to-provider';

export function useTransitionMutation() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, action, body }: { id: number; action: TransitionAction; body?: unknown }) =>
      api.post<unknown>(`${rdaRoot}/pos/${id}/${action}`, body),
    onSuccess: (_data, variables) => invalidatePO(queryClient, variables.id),
  });
}

export function useCreateRow() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: RowPayload }) => api.post<PoDetail>(`${rdaRoot}/pos/${id}/rows`, body),
    onSuccess: (_data, variables) => invalidatePO(queryClient, variables.id),
  });
}

export function useDeleteRow() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rowId }: { id: number; rowId: number }) => api.delete<void>(`${rdaRoot}/pos/${id}/rows/${rowId}`),
    onSuccess: (_data, variables) => invalidatePO(queryClient, variables.id),
  });
}

export function useReplaceRow() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rowId, body }: { id: number; rowId: number; body: RowPayload }) =>
      api.put<void>(`${rdaRoot}/pos/${id}/rows/${rowId}`, body),
    onSettled: (_data, _error, variables) => {
      if (variables) invalidatePO(queryClient, variables.id);
    },
  });
}

export function useUploadAttachment() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, file, attachmentType }: { id: number; file: File; attachmentType: AttachmentType }) => {
      const body = new FormData();
      body.append('file', file);
      body.append('attachment_type', attachmentType);
      return api.postFormData<unknown>(`${rdaRoot}/pos/${id}/attachments`, body);
    },
    onSuccess: (_data, variables) => invalidatePO(queryClient, variables.id),
  });
}

export function useDeleteAttachment() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, attachmentId }: { id: number; attachmentId: number }) =>
      api.delete<void>(`${rdaRoot}/pos/${id}/attachments/${attachmentId}`),
    onSuccess: (_data, variables) => invalidatePO(queryClient, variables.id),
  });
}

export function usePostComment() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) => api.post<PoComment>(`${rdaRoot}/pos/${id}/comments`, { comment }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['rda', 'comments', variables.id] });
    },
  });
}

export function useRdaDownloads() {
  const api = useApiClient();
  return {
    attachment: (id: number, attachmentId: number) => api.getBlob(`${rdaRoot}/pos/${id}/attachments/${attachmentId}`),
    pdf: (id: number) => api.getBlob(`${rdaRoot}/pos/${id}/pdf`),
  };
}

export function useProviderMutations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const invalidateProviders = () => queryClient.invalidateQueries({ queryKey: ['fornitori'] });
  return {
    createProvider: useMutation({
      mutationFn: async (body: ProviderPayload) => {
        const response = await api.post<DraftProviderResponse>(`${fornitoriRoot}/provider/draft`, body);
        return providerSummaryFromDraft(response, body);
      },
      onSuccess: invalidateProviders,
    }),
    createReference: useMutation({
      mutationFn: ({ providerId, body }: { providerId: number; body: ProviderReference }) =>
        api.post<ProviderReference>(`${fornitoriRoot}/provider/${providerId}/reference`, body),
      onSuccess: invalidateProviders,
    }),
    updateReference: useMutation({
      mutationFn: ({ providerId, refId, body }: { providerId: number; refId: number; body: ProviderReference }) =>
        api.put<ProviderReference>(`${fornitoriRoot}/provider/${providerId}/reference/${refId}`, body),
      onSuccess: invalidateProviders,
    }),
  };
}
