import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  DdtCespitoRow,
  EnergiaColoDetailRow,
  EnergiaColoPivotRow,
  MissingArticle,
  OrderHeader,
  OrderRow,
  RDADdtAttachmentRow,
  SalesOrderSummary,
  WhmcsInvoiceLine,
  WhmcsTransaction,
  XConnectOrder,
} from '../types';

export function useInvoiceLines() {
  const api = useApiClient();
  return useQuery<WhmcsInvoiceLine[]>({
    queryKey: ['afc-tools', 'whmcs', 'invoice-lines'],
    queryFn: () => api.get<WhmcsInvoiceLine[]>('/afc-tools/v1/whmcs/invoice-lines'),
  });
}

export function useMissingArticles() {
  const api = useApiClient();
  return useQuery<MissingArticle[]>({
    queryKey: ['afc-tools', 'mistra', 'missing-articles'],
    queryFn: () => api.get<MissingArticle[]>('/afc-tools/v1/mistra/missing-articles'),
  });
}

export function useXConnectOrders() {
  const api = useApiClient();
  return useQuery<XConnectOrder[]>({
    queryKey: ['afc-tools', 'mistra', 'xconnect', 'orders'],
    queryFn: () => api.get<XConnectOrder[]>('/afc-tools/v1/mistra/xconnect/orders'),
  });
}

export function useEnergiaColoPivot(year: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery<EnergiaColoPivotRow[]>({
    queryKey: ['afc-tools', 'energia-colo', 'pivot', year],
    queryFn: () => api.get<EnergiaColoPivotRow[]>(`/afc-tools/v1/energia-colo/pivot?year=${year}`),
    enabled,
  });
}

export function useEnergiaColoDetail(year: string, enabled: boolean) {
  const api = useApiClient();
  return useQuery<EnergiaColoDetailRow[]>({
    queryKey: ['afc-tools', 'energia-colo', 'detail', year],
    queryFn: () => api.get<EnergiaColoDetailRow[]>(`/afc-tools/v1/energia-colo/detail?year=${year}`),
    enabled,
  });
}

export function useSalesOrders() {
  const api = useApiClient();
  return useQuery<SalesOrderSummary[]>({
    queryKey: ['afc-tools', 'orders'],
    queryFn: () => api.get<SalesOrderSummary[]>('/afc-tools/v1/orders'),
  });
}

export function useOrderHeader(id: number) {
  const api = useApiClient();
  return useQuery<OrderHeader>({
    queryKey: ['afc-tools', 'orders', id],
    queryFn: () => api.get<OrderHeader>(`/afc-tools/v1/orders/${id}`),
    enabled: Number.isFinite(id) && id > 0,
  });
}

export function useOrderRows(id: number) {
  const api = useApiClient();
  return useQuery<OrderRow[]>({
    queryKey: ['afc-tools', 'orders', id, 'rows'],
    queryFn: () => api.get<OrderRow[]>(`/afc-tools/v1/orders/${id}/rows`),
    enabled: Number.isFinite(id) && id > 0,
  });
}

export function useDdtCespiti() {
  const api = useApiClient();
  return useQuery<DdtCespitoRow[]>({
    queryKey: ['afc-tools', 'ddt-cespiti'],
    queryFn: () => api.get<DdtCespitoRow[]>('/afc-tools/v1/ddt-cespiti'),
  });
}

// RDA DDT Purchase Order: the query is enabled only after the user presses
// Cerca (runId !== null); a new runId forces a fresh fetch even for the same
// date range.
export function useRdaDdtAttachments(from: string, to: string, runId: number | null) {
  const api = useApiClient();
  return useQuery<RDADdtAttachmentRow[]>({
    queryKey: ['afc-tools', 'rda', 'ddt', runId ?? 'idle', from, to],
    queryFn: () => api.get<RDADdtAttachmentRow[]>(`/afc-tools/v1/rda/ddt?from=${from}&to=${to}`),
    enabled: runId !== null,
    retry: false,
  });
}

// Non-query exports (imperative — used from button onClick handlers).

export function buildTransactionsURL(from: string, to: string): string {
  const qs = new URLSearchParams({ from, to });
  return `/afc-tools/v1/whmcs/transactions?${qs.toString()}`;
}

export async function fetchTransactions(
  api: ReturnType<typeof useApiClient>,
  from: string,
  to: string,
): Promise<WhmcsTransaction[]> {
  return api.get<WhmcsTransaction[]>(buildTransactionsURL(from, to));
}

// Downloads the DDT ZIP for the selected POs over the authenticated blob
// endpoint. The backend streams application/zip.
export async function downloadRdaDdtZip(
  api: ReturnType<typeof useApiClient>,
  from: string,
  to: string,
  poIds: number[],
): Promise<Blob> {
  return api.postBlob('/afc-tools/v1/rda/ddt/download', { from, to, poIds });
}
