import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { CoverageResult, LocationOption } from '../types';

export function useStates() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['coperture', 'states'],
    queryFn: () => api.get<LocationOption[]>('/coperture/v1/states'),
  });
}

export function useCities(stateId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['coperture', 'cities', stateId],
    queryFn: () => api.get<LocationOption[]>(`/coperture/v1/states/${stateId}/cities`),
    enabled: stateId !== null,
  });
}

export function useAddresses(cityId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['coperture', 'addresses', cityId],
    queryFn: () => api.get<LocationOption[]>(`/coperture/v1/cities/${cityId}/addresses`),
    enabled: cityId !== null,
  });
}

export function useHouseNumbers(addressId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['coperture', 'house-numbers', addressId],
    queryFn: () => api.get<LocationOption[]>(`/coperture/v1/addresses/${addressId}/house-numbers`),
    enabled: addressId !== null,
  });
}

export function useCoverage(houseNumberId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['coperture', 'coverage', houseNumberId],
    queryFn: () => api.get<CoverageResult[]>(`/coperture/v1/house-numbers/${houseNumberId}/coverage`),
    enabled: houseNumberId !== null,
  });
}
