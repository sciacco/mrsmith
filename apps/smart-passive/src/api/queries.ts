import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { AlyanteInvoiceRow, ArakRDARow, MatchingFunnelResponse, MatchingFunnelScope } from '../types';

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

export function useArakRDAs() {
  const api = useApiClient();
  return useQuery<ArakRDARow[]>({
    queryKey: ['smart-passive', 'arak-rdas'],
    queryFn: () => api.get<ArakRDARow[]>('/smart-passive/v1/arak-rdas'),
  });
}
