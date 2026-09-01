import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { AlyanteInvoiceRow } from '../types';

export function useAlyanteInvoices() {
  const api = useApiClient();
  return useQuery<AlyanteInvoiceRow[]>({
    queryKey: ['smart-passive', 'alyante-invoices'],
    queryFn: () => api.get<AlyanteInvoiceRow[]>('/smart-passive/v1/alyante-invoices'),
  });
}
