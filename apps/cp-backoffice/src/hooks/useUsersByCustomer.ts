import { useQuery } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import { usersKeys, type User } from '../api/users';

// useUsersByCustomer fetches the user list for a single customer from
// GET /api/cp-backoffice/v1/users?customer_id=<id>.
//
// The hook is gated on `customerId`: when null, React Query leaves the query
// disabled and never hits the network. This is the FE-side enforcement of the
// "no user fetch until a customer is selected" lock (the backend also rejects
// missing/empty customer_id with 400 as the authoritative guard).
export function useUsersByCustomer(customerId: number | null) {
  const api = useApiClient();
  return useQuery({
    queryKey:
      customerId != null
        ? usersKeys.byCustomer(customerId)
        : usersKeys.all,
    queryFn: () =>
      api.get<User[]>(
        `/cp-backoffice/v1/users?customer_id=${encodeURIComponent(String(customerId))}`,
      ),
    enabled: customerId != null,
  });
}
