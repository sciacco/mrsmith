import { useQuery } from '@tanstack/react-query';
import type { ApiClient } from '@mrsmith/api-client';
import { useApiClient } from './client';
import type {
  CustomerWithInvoices,
  CustomerWithOrders,
  CustomerWithAccessLines,
  OrderDetailRow,
  InvoiceDocumentsResponse,
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

export interface InvoiceQueryParams {
  cliente: number | null;
  mesi: number | null;
  q: string;
  sort: 'data_documento' | 'documento' | 'totale_netto';
  dir: 'asc' | 'desc';
  page: number;
  pageSize: number;
}

function invoiceSearch(params: InvoiceQueryParams, paginated = true) {
  const search = new URLSearchParams();
  if (params.cliente !== null) search.set('cliente', String(params.cliente));
  if (params.mesi !== null && params.mesi > 0) search.set('mesi', String(params.mesi));
  if (params.q.trim()) search.set('q', params.q.trim());
  search.set('sort', params.sort);
  search.set('dir', params.dir);
  if (paginated) {
    search.set('page', String(params.page));
    search.set('page_size', String(params.pageSize));
  }
  return search.toString();
}

export function useInvoices(params: InvoiceQueryParams) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'invoices', params],
    queryFn: () => api.get<InvoiceDocumentsResponse>(`/panoramica/v1/invoices?${invoiceSearch(params)}`),
    enabled: params.cliente !== null,
    placeholderData: previous => previous,
  });
}

export async function downloadInvoicesExcel(api: ApiClient, params: InvoiceQueryParams) {
  const blob = await api.getBlob(`/panoramica/v1/invoices/export?${invoiceSearch(params, false)}`);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `fatture_${params.cliente ?? 'cliente'}.xlsx`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
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
