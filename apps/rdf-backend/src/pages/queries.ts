import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type {
  Supplier,
  SupplierCreateInput,
  SupplierListResponse,
  SupplierUpdateInput,
} from '../api/types';

export type SortKey = 'id' | 'nome';
export type SortOrder = 'asc' | 'desc';

export interface SupplierListParams {
  search: string;
  sort: SortKey;
  order: SortOrder;
  page: number;
  pageSize: number;
}

export const supplierKeys = {
  all: ['rdf-backend', 'suppliers'] as const,
  list: (params: SupplierListParams) => ['rdf-backend', 'suppliers', params] as const,
};

export function useSuppliers(params: SupplierListParams) {
  const api = useApiClient();

  return useQuery({
    queryKey: supplierKeys.list(params),
    queryFn: async () => {
      const qs = new URLSearchParams({
        search: params.search,
        sort: params.sort,
        order: params.order,
        page: String(params.page),
        pageSize: String(params.pageSize),
      });
      return api.get<SupplierListResponse>(`/rdf-backend/v1/fornitori?${qs.toString()}`);
    },
    placeholderData: (previousData) => previousData,
  });
}

export function useCreateSupplier() {
  const api = useApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: SupplierCreateInput) =>
      api.post<Supplier>('/rdf-backend/v1/fornitori', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: supplierKeys.all });
    },
  });
}

export function useUpdateSupplier() {
  const api = useApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: SupplierUpdateInput }) =>
      api.patch<Supplier>(`/rdf-backend/v1/fornitori/${id}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: supplierKeys.all });
    },
  });
}

export function useDeleteSupplier() {
  const api = useApiClient();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) =>
      api.delete<void>(`/rdf-backend/v1/fornitori/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: supplierKeys.all });
    },
  });
}
