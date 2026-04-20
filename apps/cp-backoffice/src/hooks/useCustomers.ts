import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { customersKeys, type Customer } from '../api/customers';

// useCustomers fetches the full customer list from
// GET /api/cp-backoffice/v1/customers. The backend pins
// disable_pagination=true upstream, so the response is the complete list.
export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: customersKeys.all,
    queryFn: () => api.get<Customer[]>('/cp-backoffice/v1/customers'),
  });
}
