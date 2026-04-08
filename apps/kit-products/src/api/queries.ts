import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  AssetFlow,
  CustomFieldKey,
  CustomerGroup,
  CustomerGroupBatchUpdateRequest,
  CustomerGroupCreateRequest,
  Product,
  ProductCreateRequest,
  ProductCategory,
  ProductCategoryWriteRequest,
  ProductUpdateRequest,
  Translation,
  TranslationUpdateResponse,
  VocabularyItem,
} from './types';

export const kitProductsKeys = {
  assetFlows: ['kit-products', 'lookup', 'asset-flow'] as const,
  customFieldKeys: ['kit-products', 'lookup', 'custom-field-key'] as const,
  vocabulary: (section: string) => ['kit-products', 'lookup', 'vocabulary', section] as const,
  categories: ['kit-products', 'categories'] as const,
  customerGroups: ['kit-products', 'customer-groups'] as const,
  products: ['kit-products', 'products'] as const,
};

export function useAssetFlows() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.assetFlows,
    queryFn: () => api.get<AssetFlow[]>('/kit-products/v1/lookup/asset-flow'),
  });
}

export function useCustomFieldKeys() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.customFieldKeys,
    queryFn: () => api.get<CustomFieldKey[]>('/kit-products/v1/lookup/custom-field-key'),
  });
}

export function useVocabulary(section: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.vocabulary(section ?? ''),
    queryFn: () =>
      api.get<VocabularyItem[]>(
        `/kit-products/v1/lookup/vocabulary?section=${encodeURIComponent(section!)}`,
      ),
    enabled: section != null && section.length > 0,
  });
}

export function useCategories() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.categories,
    queryFn: () => api.get<ProductCategory[]>('/kit-products/v1/category'),
  });
}

export function useCreateCategory() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ProductCategoryWriteRequest) =>
      api.post<ProductCategory>('/kit-products/v1/category', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.categories });
    },
  });
}

export function useUpdateCategory() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...body }: { id: number } & ProductCategoryWriteRequest) =>
      api.put<ProductCategory>(`/kit-products/v1/category/${id}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.categories });
    },
  });
}

export function useCustomerGroups() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.customerGroups,
    queryFn: () => api.get<CustomerGroup[]>('/kit-products/v1/customer-group'),
  });
}

export function useCreateCustomerGroup() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CustomerGroupCreateRequest) =>
      api.post<CustomerGroup>('/kit-products/v1/customer-group', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.customerGroups });
    },
  });
}

export function useBatchUpdateCustomerGroups() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CustomerGroupBatchUpdateRequest) =>
      api.patch<{ updated: number }>('/kit-products/v1/customer-group', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.customerGroups });
    },
  });
}

export function useProducts() {
  const api = useApiClient();
  return useQuery({
    queryKey: kitProductsKeys.products,
    queryFn: () => api.get<Product[]>('/kit-products/v1/product'),
  });
}

export function useCreateProduct() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ProductCreateRequest) =>
      api.post<Product>('/kit-products/v1/product', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.products });
    },
  });
}

export function useUpdateProduct() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ code, ...body }: { code: string } & ProductUpdateRequest) =>
      api.put<Product>(`/kit-products/v1/product/${encodeURIComponent(code)}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.products });
    },
  });
}

export function useUpdateProductTranslations() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ code, translations }: { code: string; translations: Translation[] }) =>
      api.put<TranslationUpdateResponse>(
        `/kit-products/v1/product/${encodeURIComponent(code)}/translations`,
        { translations },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: kitProductsKeys.products });
    },
  });
}
