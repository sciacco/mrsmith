import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { customerStatesKeys, type CustomerState } from '../api/customerStates';

// useCustomerStates fetches the full customer-state list from
// GET /api/cp-backoffice/v1/customer-states. Called on page mount so the
// modal select is ready the moment the operator clicks the CTA.
export function useCustomerStates() {
  const api = useApiClient();
  return useQuery({
    queryKey: customerStatesKeys.all,
    queryFn: () => api.get<CustomerState[]>('/cp-backoffice/v1/customer-states'),
  });
}
