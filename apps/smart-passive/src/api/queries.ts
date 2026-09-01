import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { AlyanteInvoiceRow, ArakRDARow } from '../types';

export function useAlyanteInvoices() {
  const api = useApiClient();
  return useQuery<AlyanteInvoiceRow[]>({
    queryKey: ['smart-passive', 'alyante-invoices'],
    queryFn: () => api.get<AlyanteInvoiceRow[]>('/smart-passive/v1/alyante-invoices'),
  });
}

export function useArakRDAs() {
  const api = useApiClient();
  return useQuery<ArakRDARow[]>({
    queryKey: ['smart-passive', 'arak-rdas'],
    queryFn: () => api.get<ArakRDARow[]>('/smart-passive/v1/arak-rdas'),
  });
}
