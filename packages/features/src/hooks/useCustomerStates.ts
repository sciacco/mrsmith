import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { customerStatesKeys, type CustomerState } from '../api/customerStates';

export function useCustomerStates() {
  const api = useApiClient();
  return useQuery({
    queryKey: customerStatesKeys.all,
    queryFn: () => api.get<CustomerState[]>('/cp-backoffice/v1/customer-states'),
  });
}
