import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  Template, ProductCategory, Kit, Customer, Deal, Owner,
  PaymentMethod, CustomerPayment, CustomerOrder, QuoteListResponse,
  Quote, HSStatus, QuoteRow, ProductGroup, PublishPrecheck, ProductVariant,
} from './types';

// ── Reference data hooks ──

export function useTemplates(params?: { type?: string; lang?: string; is_colo?: string }) {
  const api = useApiClient();
  const search = new URLSearchParams();
  if (params?.type) search.set('type', params.type);
  if (params?.lang) search.set('lang', params.lang);
  if (params?.is_colo) search.set('is_colo', params.is_colo);
  const qs = search.toString();

  return useQuery({
    queryKey: ['templates', params],
    queryFn: () => api.get<Template[]>(`/quotes/v1/templates${qs ? '?' + qs : ''}`),
  });
}

export function useCategories(params?: { excludeIds?: number[]; enabled?: boolean }) {
  const api = useApiClient();
  const search = new URLSearchParams();
  if (params?.excludeIds && params.excludeIds.length > 0) {
    search.set('exclude_ids', params.excludeIds.join(','));
  }
  const qs = search.toString();
  return useQuery({
    queryKey: ['categories', params?.excludeIds ?? [], params?.enabled ?? true],
    queryFn: () => api.get<ProductCategory[]>(
      `/quotes/v1/categories${qs ? '?' + qs : ''}`
    ),
    enabled: params?.enabled ?? true,
  });
}

export function useKits(options?: { enabled?: boolean; includeIds?: number[] }) {
  const api = useApiClient();
  const includeIds = (options?.includeIds ?? [])
    .filter((id): id is number => Number.isInteger(id) && id > 0)
    .sort((a, b) => a - b);

  const search = new URLSearchParams();
  if (includeIds.length > 0) {
    search.set('include_ids', includeIds.join(','));
  }
  const qs = search.toString();

  return useQuery({
    queryKey: ['kits', includeIds],
    queryFn: () => api.get<Kit[]>(`/quotes/v1/kits${qs ? '?' + qs : ''}`),
    enabled: options?.enabled ?? true,
  });
}

export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['customers'],
    queryFn: () => api.get<Customer[]>('/quotes/v1/customers'),
  });
}

export function useDeals() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['deals'],
    queryFn: () => api.get<Deal[]>('/quotes/v1/deals'),
  });
}

export function useDeal(id: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['deal', id],
    queryFn: () => api.get<Deal>(`/quotes/v1/deals/${id}`),
    enabled: id !== null,
  });
}

export function useOwners() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['owners'],
    queryFn: () => api.get<Owner[]>('/quotes/v1/owners'),
  });
}

export function usePaymentMethods() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['payment-methods'],
    queryFn: () => api.get<PaymentMethod[]>('/quotes/v1/payment-methods'),
  });
}

export function useCustomerPayment(customerId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['customer-payment', customerId],
    queryFn: () => api.get<CustomerPayment>(`/quotes/v1/customer-payment/${customerId}`),
    enabled: customerId !== null && customerId !== '',
  });
}

export function useQuotes(params: {
  page?: number;
  status?: string;
  owner?: string;
  q?: string;
  date_from?: string;
  date_to?: string;
  sort?: string;
  dir?: string;
}) {
  const api = useApiClient();
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === '') return;
    search.set(key, String(value));
  });
  const qs = search.toString();

  return useQuery({
    queryKey: ['quotes', params],
    queryFn: () => api.get<QuoteListResponse>(`/quotes/v1/quotes${qs ? '?' + qs : ''}`),
    placeholderData: (prev) => prev,
  });
}

// ── Quote create ──

export function useCreateQuote() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Record<string, unknown>) =>
      api.post<{ id: number; quote_number: string; status: string }>('/quotes/v1/quotes', data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['quotes'] });
    },
  });
}

// ── Quote detail hooks ──

export function useQuote(id: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['quote', id],
    queryFn: () => api.get<Quote>(`/quotes/v1/quotes/${id}`),
  });
}

export function useUpdateQuote() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<Quote> }) =>
      api.put<Quote>(`/quotes/v1/quotes/${id}`, data),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: ['quote', variables.id] });
      void qc.invalidateQueries({ queryKey: ['quotes'] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', variables.id] });
    },
  });
}

export function useHSStatus(id: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['hs-status', id],
    queryFn: () => api.get<HSStatus>(`/quotes/v1/quotes/${id}/hs-status`),
  });
}

export function usePublishPrecheck(id: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['publish-precheck', id],
    queryFn: () => api.get<PublishPrecheck>(`/quotes/v1/quotes/${id}/publish-precheck`),
    enabled: id > 0,
  });
}

// ── Publish ──

export function usePublishQuote() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      api.post<{ success: boolean; steps: { step: number; name: string; status: string; error?: string }[] }>(
        `/quotes/v1/quotes/${id}/publish`, {}
      ),
    onSuccess: (_data, id) => {
      void qc.invalidateQueries({ queryKey: ['quote', id] });
      void qc.invalidateQueries({ queryKey: ['hs-status', id] });
      void qc.invalidateQueries({ queryKey: ['quotes'] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', id] });
    },
  });
}

// ── Delete ──

export function useDeleteQuote() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.delete(`/quotes/v1/quotes/${id}`),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['quotes'] });
    },
  });
}

// ── Kit rows and products hooks ──

export function useQuoteRows(quoteId: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['quote-rows', quoteId],
    queryFn: () => api.get<QuoteRow[]>(`/quotes/v1/quotes/${quoteId}/rows`),
  });
}

export function useAddRow() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ quoteId, kitId }: { quoteId: number; kitId: number }) =>
      api.post<QuoteRow>(`/quotes/v1/quotes/${quoteId}/rows`, { kit_id: kitId }),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: ['quote-rows', variables.quoteId] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', variables.quoteId] });
    },
  });
}

export function useDeleteRow() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ quoteId, rowId }: { quoteId: number; rowId: number }) =>
      api.delete(`/quotes/v1/quotes/${quoteId}/rows/${rowId}`),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: ['quote-rows', variables.quoteId] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', variables.quoteId] });
    },
  });
}

export function useUpdateRowPosition() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ quoteId, rowId, position }: { quoteId: number; rowId: number; position: number }) =>
      api.put(`/quotes/v1/quotes/${quoteId}/rows/${rowId}/position`, { position }),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: ['quote-rows', variables.quoteId] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', variables.quoteId] });
    },
  });
}

export function useRowProducts(quoteId: number, rowId: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['row-products', quoteId, rowId],
    queryFn: () => api.get<ProductGroup[]>(`/quotes/v1/quotes/${quoteId}/rows/${rowId}/products`),
    enabled: rowId > 0,
  });
}

export function useUpdateProduct() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ quoteId, rowId, productId, data }: {
      quoteId: number; rowId: number; productId: number; data: Partial<ProductVariant>;
    }) => api.put(`/quotes/v1/quotes/${quoteId}/rows/${rowId}/products/${productId}`, data),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: ['row-products', variables.quoteId, variables.rowId] });
      void qc.invalidateQueries({ queryKey: ['quote-rows', variables.quoteId] });
      void qc.invalidateQueries({ queryKey: ['publish-precheck', variables.quoteId] });
    },
  });
}

export function useCustomerOrders(customerId: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['customer-orders', customerId],
    queryFn: () => api.get<CustomerOrder[]>(`/quotes/v1/customer-orders/${customerId}`),
    enabled: customerId !== null && customerId !== '',
  });
}
