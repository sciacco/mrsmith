import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  CustomerWithInvoices,
  CustomerWithOrders,
  CustomerWithAccessLines,
  OrderSummaryRow,
  OrderDetailRow,
  InvoiceLine,
  AccessLine,
  IaaSAccount,
  DailyCharge,
  MonthlyCharge,
  ChargeBreakdown,
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

export function useCustomersWithOrders(variant: 'a' | 'b') {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'orders', variant],
    queryFn: () => api.get<CustomerWithOrders[]>(`/panoramica/v1/customers/with-orders?variant=${variant}`),
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

export function useOrdersSummary(cliente: number | null, stati: string[]) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'orders', 'summary', cliente, stati],
    queryFn: () => api.get<OrderSummaryRow[]>(
      `/panoramica/v1/orders/summary?cliente=${cliente}&stati=${stati.join(',')}`
    ),
    enabled: cliente !== null && stati.length > 0,
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

export function useDailyCharges(domain: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'daily-charges', domain],
    queryFn: () => api.get<DailyCharge[]>(`/panoramica/v1/iaas/daily-charges?domain=${domain}`),
    enabled: domain !== null,
  });
}

export function useMonthlyCharges(domain: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'monthly-charges', domain],
    queryFn: () => api.get<MonthlyCharge[]>(`/panoramica/v1/iaas/monthly-charges?domain=${domain}`),
    enabled: domain !== null,
  });
}

export function useChargeBreakdown(domain: string | null, day: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'iaas', 'charge-breakdown', domain, day],
    queryFn: () => api.get<ChargeBreakdown>(`/panoramica/v1/iaas/charge-breakdown?domain=${domain}&day=${day}`),
    enabled: domain !== null && day !== null,
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
