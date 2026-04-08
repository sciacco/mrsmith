import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../../api/client';
import type {
  CustomerBrief,
  DiscountedKit,
  DiscountedKitDetail,
  KitBrief,
  KitDiscountCreateRequest,
  KitDiscountEntry,
  PaginatedResponse,
} from './mistraTypes';

export const mistraKeys = {
  kits: ['kit-products', 'mistra', 'kits'] as const,
  kitDiscounts: (kitId: number | null) => ['kit-products', 'mistra', 'kit-discounts', kitId] as const,
  customers: ['kit-products', 'mistra', 'customers'] as const,
  discountedKits: (customerId: number | null) => ['kit-products', 'mistra', 'discounted-kits', customerId] as const,
  discountedKit: (customerId: number | null, kitId: number | null) =>
    ['kit-products', 'mistra', 'discounted-kit', customerId, kitId] as const,
};

export function useMistraKits() {
  const api = useApiClient();
  return useQuery({
    queryKey: mistraKeys.kits,
    queryFn: () =>
      api.get<PaginatedResponse<KitBrief>>('/kit-products/v1/mistra/kit?page_number=1&disable_pagination=true'),
  });
}

export function useMistraKitDiscounts(kitId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: mistraKeys.kitDiscounts(kitId),
    queryFn: () =>
      api.get<PaginatedResponse<KitDiscountEntry>>(
        `/kit-products/v1/mistra/kit-discount?page_number=1&disable_pagination=true&kit_id=${kitId}`,
      ),
    enabled: kitId != null && Number.isFinite(kitId),
  });
}

export function useUpsertMistraKitDiscount() {
  const api = useApiClient();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KitDiscountCreateRequest) =>
      api.post<{ message: string }>('/kit-products/v1/mistra/kit-discount', body),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: mistraKeys.kitDiscounts(variables.kit_id) });
    },
  });
}

export function useMistraCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: mistraKeys.customers,
    queryFn: () =>
      api.get<PaginatedResponse<CustomerBrief>>(
        '/kit-products/v1/mistra/customer?page_number=1&disable_pagination=true',
      ),
  });
}

export function useMistraDiscountedKits(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: mistraKeys.discountedKits(customerId),
    queryFn: () =>
      api.get<PaginatedResponse<DiscountedKit>>(
        `/kit-products/v1/mistra/discounted-kit?page_number=1&disable_pagination=true&customer_id=${customerId}`,
      ),
    enabled: customerId != null && Number.isFinite(customerId),
  });
}

export function useMistraDiscountedKitDetail(customerId: number | null, kitId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: mistraKeys.discountedKit(customerId, kitId),
    queryFn: () =>
      api.get<DiscountedKitDetail>(
        `/kit-products/v1/mistra/discounted-kit/${kitId}?customer_id=${customerId}`,
      ),
    enabled: customerId != null && kitId != null && Number.isFinite(customerId) && Number.isFinite(kitId),
  });
}
