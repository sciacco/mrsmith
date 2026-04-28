import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type {
  AlyanteSupplier,
  ArticleCategory,
  Category,
  CategoryPayload,
  CategoryUpdatePayload,
  Country,
  DashboardCategory,
  DashboardDocument,
  DashboardDraft,
  DocumentType,
  PaginatedEnvelope,
  PaymentMethod,
  Provider,
  ProviderCategory,
  ProviderDocument,
  ProviderPayload,
  ProviderReference,
  ProviderSummary,
} from './types';
import { useApiClient } from './client';

const root = '/fornitori/v1';

function unwrap<T>(value: T[] | PaginatedEnvelope<T>): T[] {
  return Array.isArray(value) ? value : value.items ?? [];
}

export function useProviders() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'providers'],
    queryFn: async () => unwrap(await api.get<Provider[] | PaginatedEnvelope<Provider>>(`${root}/provider`)),
  });
}

export function useProviderSummary() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'provider-summary'],
    queryFn: () => api.get<ProviderSummary[]>(`${root}/provider-summary`),
  });
}

export function useProvider(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'provider', id],
    enabled: id != null,
    queryFn: () => api.get<Provider>(`${root}/provider/${id}`),
  });
}

export function useCategories() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'categories'],
    queryFn: async () => unwrap(await api.get<Category[] | PaginatedEnvelope<Category>>(`${root}/category`)),
  });
}

export function useDocumentTypes() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'document-types'],
    queryFn: async () => unwrap(await api.get<DocumentType[] | PaginatedEnvelope<DocumentType>>(`${root}/document-type`)),
  });
}

export function useProviderCategories(providerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'provider-categories', providerId],
    enabled: providerId != null,
    queryFn: async () => unwrap(await api.get<ProviderCategory[] | PaginatedEnvelope<ProviderCategory>>(`${root}/provider/${providerId}/category`)),
  });
}

export function useProviderDocuments(providerId: number | null, categoryId?: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'documents', providerId, categoryId ?? null],
    enabled: providerId != null,
    queryFn: async () => {
      const params = new URLSearchParams({ provider_id: String(providerId) });
      if (categoryId) params.set('category_id', String(categoryId));
      return unwrap(await api.get<ProviderDocument[] | PaginatedEnvelope<ProviderDocument>>(`${root}/document?${params}`));
    },
  });
}

export function useDashboard() {
  const api = useApiClient();
  return {
    drafts: useQuery({ queryKey: ['fornitori', 'dashboard', 'drafts'], queryFn: () => api.get<DashboardDraft[]>(`${root}/dashboard/drafts`) }),
    documents: useQuery({ queryKey: ['fornitori', 'dashboard', 'documents'], queryFn: () => api.get<DashboardDocument[]>(`${root}/dashboard/expiring-documents`) }),
    categories: useQuery({ queryKey: ['fornitori', 'dashboard', 'categories'], queryFn: () => api.get<DashboardCategory[]>(`${root}/dashboard/categories-to-review`) }),
  };
}

export function usePaymentMethods() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'payment-methods'],
    queryFn: () => api.get<PaymentMethod[]>(`${root}/payment-method`),
  });
}

export function useCountries() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'countries'],
    queryFn: () => api.get<Country[]>(`${root}/country`),
  });
}

export function useArticleCategories() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['fornitori', 'article-categories'],
    queryFn: () => api.get<ArticleCategory[]>(`${root}/article-category`),
  });
}

export function useAlyanteSuppliers(search: string) {
  const api = useApiClient();
  const term = search.trim();
  return useQuery({
    queryKey: ['fornitori', 'alyante-suppliers', term],
    enabled: term.length >= 3,
    staleTime: 60_000,
    queryFn: () => api.get<AlyanteSupplier[]>(`${root}/alyante-suppliers?search=${encodeURIComponent(term)}`),
  });
}

export function useFornitoriMutations() {
  const api = useApiClient();
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: ['fornitori'] });

  return {
    createProvider: useMutation({
      mutationFn: (body: ProviderPayload) => api.post<Provider>(`${root}/provider`, body),
      onSuccess: invalidate,
    }),
    updateProvider: useMutation({
      mutationFn: ({ id, body }: { id: number; body: Partial<ProviderPayload> }) => api.put<Provider>(`${root}/provider/${id}`, body),
      onSuccess: invalidate,
    }),
    deleteProvider: useMutation({
      mutationFn: (id: number) => api.delete<void>(`${root}/provider/${id}`),
      onSuccess: invalidate,
    }),
    createReference: useMutation({
      mutationFn: ({ providerId, body }: { providerId: number; body: ProviderReference }) => api.post<ProviderReference>(`${root}/provider/${providerId}/reference`, body),
      onSuccess: invalidate,
    }),
    updateReference: useMutation({
      mutationFn: ({ providerId, refId, body }: { providerId: number; refId: number; body: ProviderReference }) => api.put<ProviderReference>(`${root}/provider/${providerId}/reference/${refId}`, body),
      onSuccess: invalidate,
    }),
    createCategory: useMutation({
      mutationFn: (body: CategoryPayload) => api.post<Category>(`${root}/category`, body),
      onSuccess: invalidate,
    }),
    updateCategory: useMutation({
      mutationFn: ({ id, body }: { id: number; body: CategoryUpdatePayload }) => api.put<Category>(`${root}/category/${id}`, body),
      onSuccess: invalidate,
    }),
    deleteCategory: useMutation({
      mutationFn: (id: number) => api.delete<void>(`${root}/category/${id}`),
      onSuccess: invalidate,
    }),
    getCategory: (id: number) => api.get<Category>(`${root}/category/${id}`),
    createDocumentType: useMutation({
      mutationFn: (body: { name: string }) => api.post<DocumentType>(`${root}/document-type`, body),
      onSuccess: invalidate,
    }),
    updateDocumentType: useMutation({
      mutationFn: ({ id, name }: { id: number; name: string }) => api.put<DocumentType>(`${root}/document-type/${id}`, { name }),
      onSuccess: invalidate,
    }),
    deleteDocumentType: useMutation({
      mutationFn: (id: number) => api.delete<void>(`${root}/document-type/${id}`),
      onSuccess: invalidate,
    }),
    addProviderCategories: useMutation({
      mutationFn: async ({ providerId, categoryIds, critical }: { providerId: number; categoryIds: number[]; critical: boolean }) => {
        await Promise.all(categoryIds.map((id) => api.post<void>(`${root}/provider/${providerId}/category/${id}?critical=${critical}`)));
      },
      onSuccess: invalidate,
    }),
    uploadDocument: useMutation({
      mutationFn: (body: FormData) => api.postFormData<ProviderDocument>(`${root}/document`, body),
      onSuccess: invalidate,
    }),
    updateDocument: useMutation({
      mutationFn: ({ id, body }: { id: number; body: FormData }) => api.patchFormData<ProviderDocument>(`${root}/document/${id}`, body),
      onSuccess: invalidate,
    }),
    setPaymentRda: useMutation({
      mutationFn: ({ code, rda_available }: { code: string; rda_available: boolean }) =>
        api.put<void>(`${root}/payment-method/${encodeURIComponent(code)}/rda-available`, { rda_available }),
      onSuccess: invalidate,
    }),
    setArticleCategory: useMutation({
      mutationFn: ({ articleCode, categoryId }: { articleCode: string; categoryId: number }) =>
        api.put<void>(`${root}/article-category/${encodeURIComponent(articleCode)}`, { category_id: categoryId }),
      onSuccess: invalidate,
    }),
    downloadDocument: (id: number) => api.getBlob(`${root}/document/${id}/download`),
  };
}
