import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  CustomerWithInvoices,
  CustomerWithOrders,
  CustomerWithAccessLines,
  OrderDetailRow,
  InvoiceLine,
  AccessLine,
  IaaSAccount,
  ChargeSeriesPoint,
  ChargeCategoryBreakdown,
  WindowsLicense,
  TimooTenant,
  PbxStatsResponse,
} from '../types';

// ── Customer lists ──

export function useCustomersWithInvoices() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'invoices'],
    queryFn: () => api.get<CustomerWithInvoices[]>('/panoramica/v1/customers/with-invoices'),
  });
}

export function useCustomersWithOrders() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'orders'],
    queryFn: () => api.get<CustomerWithOrders[]>('/panoramica/v1/customers/with-orders'),
  });
}

export function useCustomersWithAccessLines() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'access-lines'],
    queryFn: () => api.get<CustomerWithAccessLines[]>('/panoramica/v1/customers/with-access-lines'),
  });
}

// ── Orders ──

export function useOrderStatuses() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'order-statuses'],
    queryFn: () => api.get<string[]>('/panoramica/v1/order-statuses'),
  });
}

export function useOrdersDetail(cliente: number | null, stati: string[]) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'orders', 'detail', cliente, stati],
    queryFn: () => api.get<OrderDetailRow[]>(
      `/panoramica/v1/orders/detail?cliente=${cliente}&stati=${stati.join(',')}`
    ),
    enabled: cliente !== null && stati.length > 0,
  });
}

// ── Invoices ──

export function useInvoices(cliente: number | null, mesi: number | null) {
  const api = useApiClient();
  const params = new URLSearchParams();
  if (cliente !== null) params.set('cliente', String(cliente));
  if (mesi !== null && mesi > 0) params.set('mesi', String(mesi));
  return useQuery({
    queryKey: ['panoramica', 'invoices', cliente, mesi],
    queryFn: () => api.get<InvoiceLine[]>(`/panoramica/v1/invoices?${params}`),
    enabled: cliente !== null,
  });
}

// ── Access Lines ──

export function useConnectionTypes() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'connection-types'],
    queryFn: () => api.get<string[]>('/panoramica/v1/connection-types'),
  });
}

export function useAccessLines(clienti: number[], stati: string[], tipi: string[], enabled: boolean) {
  const api = useApiClient();
  const params = new URLSearchParams();
  if (clienti.length > 0) params.set('clienti', clienti.join(','));
  if (stati.length > 0) params.set('stati', stati.join(','));
  if (tipi.length > 0) params.set('tipi', tipi.join(','));
  return useQuery({
    queryKey: ['panoramica', 'access-lines', clienti, stati, tipi],
    queryFn: () => api.get<AccessLine[]>(`/panoramica/v1/access-lines?${params}`),
    enabled: enabled && clienti.length > 0 && stati.length > 0 && tipi.length > 0,
  });
}

// ── IaaS ──

export function useIaaSAccounts() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'accounts'],
    queryFn: () => api.get<IaaSAccount[]>('/panoramica/v1/iaas/accounts'),
  });
}

export function useChargesSeries(domain: string | null, from: string, to: string, group: string) {
  const api = useApiClient();
  const params = new URLSearchParams({ domain: domain!, from, to, group });
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'charges', domain, from, to, group],
    queryFn: () => api.get<ChargeSeriesPoint[]>(`/panoramica/v1/iaas/charges?${params}`),
    enabled: domain !== null,
    retry: false,
  });
}

export function useChargesByCategory(domain: string | null, from: string, to: string) {
  const api = useApiClient();
  const params = new URLSearchParams({ domain: domain!, from, to });
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'charges-by-category', domain, from, to],
    queryFn: () => api.get<ChargeCategoryBreakdown>(`/panoramica/v1/iaas/charges-by-category?${params}`),
    enabled: domain !== null,
    retry: false,
  });
}

export function useWindowsLicenses() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'windows-licenses'],
    queryFn: () => api.get<WindowsLicense[]>('/panoramica/v1/iaas/windows-licenses'),
  });
}

// ── Timoo ──

export function useTimooTenants() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'timoo', 'tenants'],
    queryFn: () => api.get<TimooTenant[]>('/panoramica/v1/timoo/tenants'),
  });
}

export function usePbxStats(tenantId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'timoo', 'pbx-stats', tenantId],
    queryFn: () => api.get<PbxStatsResponse>(`/panoramica/v1/timoo/pbx-stats?tenant=${tenantId}`),
    enabled: tenantId !== null,
  });
}
