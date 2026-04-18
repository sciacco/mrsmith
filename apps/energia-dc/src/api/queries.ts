import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type {
  BillingCharge,
  CustomerKWParams,
  KWPoint,
  LookupItem,
  LowConsumptionParams,
  LowConsumptionRow,
  NoVariableRack,
  PowerReadingsPage,
  PowerReadingsParams,
  RackDetail,
  RackSocketStatus,
  RackStatPoint,
} from './types';

function withSearch(path: string, params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined) {
      search.set(key, String(value));
    }
  });
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}

export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'customers'],
    queryFn: () => api.get<LookupItem[]>('/energia-dc/v1/customers'),
  });
}

export function useSites(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'sites', customerId],
    queryFn: () => api.get<LookupItem[]>(`/energia-dc/v1/customers/${customerId}/sites`),
    enabled: customerId !== null,
  });
}

export function useRooms(siteId: number | null, customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'rooms', siteId, customerId],
    queryFn: () =>
      api.get<LookupItem[]>(
        withSearch(`/energia-dc/v1/sites/${siteId}/rooms`, { customerId: customerId ?? undefined }),
      ),
    enabled: siteId !== null && customerId !== null,
  });
}

export function useRacks(roomId: number | null, customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'racks', roomId, customerId],
    queryFn: () =>
      api.get<LookupItem[]>(
        withSearch(`/energia-dc/v1/rooms/${roomId}/racks`, { customerId: customerId ?? undefined }),
      ),
    enabled: roomId !== null && customerId !== null,
  });
}

export function useRackDetail(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'rack-detail', rackId],
    queryFn: () => api.get<RackDetail>(`/energia-dc/v1/racks/${rackId}`),
    enabled: rackId !== null,
  });
}

export function useRackSocketStatus(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'socket-status', rackId],
    queryFn: () => api.get<RackSocketStatus[]>(`/energia-dc/v1/racks/${rackId}/socket-status`),
    enabled: rackId !== null,
  });
}

export function useRackStatsLastDays(rackId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'rack-stats-last-days', rackId],
    queryFn: () => api.get<RackStatPoint[]>(`/energia-dc/v1/racks/${rackId}/stats-last-days`),
    enabled: rackId !== null,
  });
}

export function usePowerReadings(params: PowerReadingsParams | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'power-readings', params],
    queryFn: () =>
      api.get<PowerReadingsPage>(
        withSearch(`/energia-dc/v1/racks/${params?.rackId}/power-readings`, {
          from: params?.from,
          to: params?.to,
          page: params?.page,
          size: params?.size,
        }),
      ),
    enabled: params !== null,
  });
}

export function useCustomerKWSeries(params: CustomerKWParams | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'customer-kw', params],
    queryFn: () =>
      api.get<KWPoint[]>(
        withSearch(`/energia-dc/v1/customers/${params?.customerId}/kw`, {
          period: params?.period,
          cosfi: params?.cosfi,
        }),
      ),
    enabled: params !== null,
  });
}

export function useBillingCharges(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'billing', customerId],
    queryFn: () => api.get<BillingCharge[]>(`/energia-dc/v1/customers/${customerId}/addebiti`),
    enabled: customerId !== null,
  });
}

export function useNoVariableCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'no-variable-customers'],
    queryFn: () => api.get<LookupItem[]>('/energia-dc/v1/no-variable-billing/customers'),
  });
}

export function useNoVariableRacks(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'no-variable-racks', customerId],
    queryFn: () =>
      api.get<NoVariableRack[]>(`/energia-dc/v1/no-variable-billing/customers/${customerId}/racks`),
    enabled: customerId !== null,
  });
}

export function useLowConsumption(params: LowConsumptionParams | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['energia-dc', 'low-consumption', params],
    queryFn: () =>
      api.get<LowConsumptionRow[]>(
        withSearch('/energia-dc/v1/low-consumption', {
          min: params?.min,
          customerId: params?.customerId ?? undefined,
        }),
      ),
    enabled: params !== null,
  });
}
