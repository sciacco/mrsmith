import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { AlyanteInvoiceRow, ArakRDARow, MatchingFunnelFilter, MatchingFunnelResponse, MatchingFunnelScope, SDIImportStatus } from '../types';

export function useAlyanteInvoices() {
  const api = useApiClient();
  return useQuery<AlyanteInvoiceRow[]>({
    queryKey: ['smart-passive', 'alyante-invoices'],
    queryFn: () => api.get<AlyanteInvoiceRow[]>('/smart-passive/v1/alyante-invoices'),
  });
}

export function useMatchingFunnel(scope: MatchingFunnelScope) {
  const api = useApiClient();
  return useQuery<MatchingFunnelResponse>({
    queryKey: ['smart-passive', 'matching-funnel', scope],
    queryFn: () => api.get<MatchingFunnelResponse>(`/smart-passive/v1/matching-funnel?scope=${scope}`),
  });
}

export function useMatchingSuggestions(filter: MatchingFunnelFilter | null) {
  const api = useApiClient();
  const params = new URLSearchParams();
  if (filter) {
    params.set('scope', filter.scope);
    if (filter.from) params.set('from', filter.from);
    if (filter.to) params.set('to', filter.to);
  }
  return useQuery<MatchingFunnelResponse>({
    queryKey: ['smart-passive', 'matching-suggestions', filter?.scope ?? '', filter?.from ?? '', filter?.to ?? ''],
    queryFn: () => api.get<MatchingFunnelResponse>(`/smart-passive/v1/matching-funnel?${params.toString()}`),
    enabled: filter !== null,
  });
}

export function useSDIImportStatus() {
  const api = useApiClient();
  return useQuery<SDIImportStatus>({
    queryKey: ['smart-passive', 'sdi-import-status'],
    queryFn: () => api.get<SDIImportStatus>('/smart-passive/v1/sdi-import/status'),
  });
}

export function useArakRDAs() {
  const api = useApiClient();
  return useQuery<ArakRDARow[]>({
    queryKey: ['smart-passive', 'arak-rdas'],
    queryFn: () => api.get<ArakRDARow[]>('/smart-passive/v1/arak-rdas'),
  });
}
