import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { customersKeys, type Customer } from '../api/customers';

export function useCustomers() {
  const api = useApiClient();
  return useQuery({
    queryKey: customersKeys.all,
    queryFn: () => api.get<Customer[]>('/cp-backoffice/v1/customers'),
  });
}
