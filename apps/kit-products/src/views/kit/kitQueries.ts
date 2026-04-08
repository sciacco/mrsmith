import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import type {
  KitCloneRequest,
  KitCreateRequest,
  KitCreateResponse,
  KitCustomValueItem,
  KitCustomValueWriteRequest,
  KitDetail,
  KitProductItem,
  KitProductWriteRequest,
  KitSummary,
  KitWriteRequest,
} from './kitTypes';

export const kitKeys = {
  kits: ['kit-products', 'kits'] as const,
  kit: (id: number) => ['kit-products', 'kit', id] as const,
  kitProducts: (id: number) => ['kit-products', 'kit', id, 'products'] as const,
  kitCustomValues: (id: number) => ['kit-products', 'kit', id, 'custom-values'] as const,
};

export function useKits() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitKeys.kits,
    queryFn: () => api.get<KitSummary[]>('/kit-products/v1/kit'),
  });
}

export function useKit(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: id == null ? ['kit-products', 'kit', 'missing'] : kitKeys.kit(id),
    queryFn: () => api.get<KitDetail>(`/kit-products/v1/kit/${id}`),
    enabled: id != null && Number.isFinite(id),
  });
}

export function useCreateKit() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KitCreateRequest) => api.post<KitCreateResponse>('/kit-products/v1/kit', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitKeys.kits });
    },
  });
}

export function useUpdateKit(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KitWriteRequest) => api.put<KitDetail>(`/kit-products/v1/kit/${id}`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
      queryClient.invalidateQueries({ queryKey: kitKeys.kits });
    },
  });
}

export function useUpdateKitHelp(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { help_url: string | null }) =>
      api.put<KitDetail>(`/kit-products/v1/kit/${id}/help`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useUpdateKitTranslations(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { translations: unknown[] }) =>
      api.put<KitDetail>(`/kit-products/v1/kit/${id}/translations`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useCloneKit() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: KitCloneRequest }) =>
      api.post<KitCreateResponse>(`/kit-products/v1/kit/${id}/clone`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitKeys.kits });
    },
  });
}

export function useDeleteKit() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.delete<void>(`/kit-products/v1/kit/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitKeys.kits });
    },
  });
}

export function useKitProducts(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: id == null ? ['kit-products', 'kit', 'missing', 'products'] : kitKeys.kitProducts(id),
    queryFn: () => api.get<KitProductItem[]>(`/kit-products/v1/kit/${id}/products`),
    enabled: id != null && Number.isFinite(id),
  });
}

export function useCreateKitProduct(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KitProductWriteRequest) =>
      api.post<KitProductItem>(`/kit-products/v1/kit/${id}/products`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitProducts(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useUpdateKitProduct(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, ...body }: { productId: number } & KitProductWriteRequest) =>
      api.put<KitProductItem>(`/kit-products/v1/kit/${id}/products/${productId}`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitProducts(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useBatchUpdateKitProducts(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { items: Array<{ id: number } & KitProductWriteRequest> }) =>
      api.patch<{ updated: number }>(`/kit-products/v1/kit/${id}/products`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitProducts(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useDeleteKitProduct(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (productId: number) =>
      api.delete<void>(`/kit-products/v1/kit/${id}/products/${productId}`),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitProducts(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useKitCustomValues(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: id == null ? ['kit-products', 'kit', 'missing', 'custom-values'] : kitKeys.kitCustomValues(id),
    queryFn: () => api.get<KitCustomValueItem[]>(`/kit-products/v1/kit/${id}/custom-values`),
    enabled: id != null && Number.isFinite(id),
  });
}

export function useCreateKitCustomValue(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KitCustomValueWriteRequest) =>
      api.post<KitCustomValueItem>(`/kit-products/v1/kit/${id}/custom-values`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitCustomValues(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useUpdateKitCustomValue(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ valueId, ...body }: { valueId: number } & KitCustomValueWriteRequest) =>
      api.put<KitCustomValueItem>(`/kit-products/v1/kit/${id}/custom-values/${valueId}`, body),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitCustomValues(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}

export function useDeleteKitCustomValue(id: number | null) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (valueId: number) =>
      api.delete<void>(`/kit-products/v1/kit/${id}/custom-values/${valueId}`),
    onSuccess: () => {
      if (id != null) {
        queryClient.invalidateQueries({ queryKey: kitKeys.kitCustomValues(id) });
        queryClient.invalidateQueries({ queryKey: kitKeys.kit(id) });
      }
    },
  });
}
