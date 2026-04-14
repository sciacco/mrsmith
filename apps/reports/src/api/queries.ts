import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  MorAnomaly,
  TimooDailyStat,
  PendingActivation,
  ActivationRow,
  RenewalSummary,
  RenewalRow,
} from '../types';

export function useMorAnomalies() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['mor-anomalies'],
    queryFn: () => api.get<MorAnomaly[]>('/reports/v1/mor-anomalies'),
  });
}

export function useTimooDailyStats() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['timoo-daily-stats'],
    queryFn: () => api.get<TimooDailyStat[]>('/reports/v1/timoo/daily-stats'),
  });
}

export function useOrderStatuses() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['reports', 'order-statuses'],
    queryFn: () => api.get<string[]>('/reports/v1/order-statuses'),
  });
}

export function usePendingActivations() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['pending-activations'],
    queryFn: () => api.get<PendingActivation[]>('/reports/v1/pending-activations'),
  });
}

export function usePendingActivationRows(orderNumber: string | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['pending-activation-rows', orderNumber],
    queryFn: () => api.get<ActivationRow[]>(`/reports/v1/pending-activations/${orderNumber}/rows`),
    enabled: !!orderNumber,
  });
}

export function useUpcomingRenewals(months: number, minMrc: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['upcoming-renewals', months, minMrc],
    queryFn: () => api.get<RenewalSummary[]>(`/reports/v1/upcoming-renewals?months=${months}&minMrc=${minMrc}`),
    placeholderData: (prev) => prev,
  });
}

export function useRenewalRows(customerId: string | null, months: number, minMrc: number) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['renewal-rows', customerId, months, minMrc],
    queryFn: () => api.get<RenewalRow[]>(`/reports/v1/upcoming-renewals/${customerId}/rows?months=${months}&minMrc=${minMrc}`),
    enabled: !!customerId,
  });
}

export function useConnectionTypes() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['connection-types'],
    queryFn: () => api.get<string[]>('/reports/v1/connection-types'),
  });
}
