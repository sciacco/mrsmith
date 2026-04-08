import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type {
  Kit, KitProduct, Customer, GrappaCustomer, CustomerGroup,
  KitGroupDiscount, CreditBalance, CreditTransaction, TransactionRequest,
  TimooPricing, TimooPricingRequest, IaaSPricing, IaaSPricingRequest,
  IaaSAccount, IaaSCreditUpdateItem, Rack, RackDiscountUpdateItem,
} from '../types';

// ── Kits ──

export function useKits() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'kits'],
    queryFn: () => api.get<Kit[]>('/listini/v1/kits'),
  });
}

export function useKitProducts(kitId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'kit-products', kitId],
    queryFn: () => api.get<KitProduct[]>(`/listini/v1/kits/${kitId}/products`),
    enabled: kitId != null,
  });
}

export function useKitHelpUrl(kitId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'kit-help-url', kitId],
    queryFn: () => api.get<{ help_url: string | null }>(`/listini/v1/kits/${kitId}/help-url`),
    enabled: kitId != null,
  });
}

// ── Customers ──

export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'customers'],
    queryFn: () => api.get<Customer[]>('/listini/v1/customers'),
  });
}

export function useERPLinkedCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'customers-erp-linked'],
    queryFn: () => api.get<Customer[]>('/listini/v1/customers/erp-linked'),
  });
}

export function useGrappaCustomers(exclude?: string) {
  const api = useApiClient();
  const params = exclude ? `?exclude=${exclude}` : '';
  return useQuery({
    queryKey: ['listini', 'grappa-customers', exclude],
    queryFn: () => api.get<GrappaCustomer[]>(`/listini/v1/grappa/customers${params}`),
  });
}

export function useRackCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'rack-customers'],
    queryFn: () => api.get<GrappaCustomer[]>('/listini/v1/grappa/rack-customers'),
  });
}

// ── IaaS Pricing ──

export function useIaaSPricing(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'iaas-pricing', customerId],
    queryFn: () => api.get<IaaSPricing>(`/listini/v1/grappa/customers/${customerId}/iaas-pricing`),
    enabled: customerId != null,
  });
}

export function useUpsertIaaSPricing() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, data }: { customerId: number; data: IaaSPricingRequest }) =>
      api.post(`/listini/v1/grappa/customers/${customerId}/iaas-pricing`, data),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['listini', 'iaas-pricing', vars.customerId] });
    },
  });
}

// ── Timoo Pricing ──

export function useTimooPricing(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'timoo-pricing', customerId],
    queryFn: () => api.get<TimooPricing>(`/listini/v1/customers/${customerId}/pricing/timoo`),
    enabled: customerId != null,
  });
}

export function useUpsertTimooPricing() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, data }: { customerId: number; data: TimooPricingRequest }) =>
      api.put(`/listini/v1/customers/${customerId}/pricing/timoo`, data),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['listini', 'timoo-pricing', vars.customerId] });
    },
  });
}

// ── Customer Groups ──

export function useCustomerGroups() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'customer-groups'],
    queryFn: () => api.get<CustomerGroup[]>('/listini/v1/customer-groups'),
  });
}

export function useCustomerGroupIds(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'customer-group-ids', customerId],
    queryFn: () => api.get<{ groupIds: number[] }>(`/listini/v1/customers/${customerId}/groups`),
    enabled: customerId != null,
  });
}

export function useSyncCustomerGroups() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, groupIds }: { customerId: number; groupIds: number[] }) =>
      api.patch(`/listini/v1/customers/${customerId}/groups`, { groupIds }),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['listini', 'customer-group-ids', vars.customerId] });
    },
  });
}

export function useKitDiscountsByGroup(groupId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'kit-discounts-by-group', groupId],
    queryFn: () => api.get<KitGroupDiscount[]>(`/listini/v1/customer-groups/${groupId}/kit-discounts`),
    enabled: groupId != null,
  });
}

// ── Credits ──

export function useCreditBalance(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'credit-balance', customerId],
    queryFn: () => api.get<CreditBalance>(`/listini/v1/customers/${customerId}/credit`),
    enabled: customerId != null,
  });
}

export function useTransactions(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'transactions', customerId],
    queryFn: () => api.get<CreditTransaction[]>(`/listini/v1/customers/${customerId}/transactions`),
    enabled: customerId != null,
  });
}

export function useCreateTransaction() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ customerId, data }: { customerId: number; data: TransactionRequest }) =>
      api.post(`/listini/v1/customers/${customerId}/transactions`, data),
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: ['listini', 'credit-balance', vars.customerId] });
      qc.invalidateQueries({ queryKey: ['listini', 'transactions', vars.customerId] });
    },
  });
}

// ── IaaS Accounts ──

export function useIaaSAccounts() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'iaas-accounts'],
    queryFn: () => api.get<IaaSAccount[]>('/listini/v1/grappa/iaas-accounts'),
  });
}

export function useBatchUpdateIaaSCredits() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (items: IaaSCreditUpdateItem[]) =>
      api.patch('/listini/v1/grappa/iaas-accounts/credits', { items }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['listini', 'iaas-accounts'] });
    },
  });
}

// ── Racks ──

export function useCustomerRacks(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['listini', 'customer-racks', customerId],
    queryFn: () => api.get<Rack[]>(`/listini/v1/grappa/customers/${customerId}/racks`),
    enabled: customerId != null,
  });
}

export function useBatchUpdateRackDiscounts() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (items: RackDiscountUpdateItem[]) =>
      api.patch('/listini/v1/grappa/racks/discounts', { items }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['listini', 'customer-racks'] });
    },
  });
}
