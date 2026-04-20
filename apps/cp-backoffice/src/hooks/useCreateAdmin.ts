import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useApiClient } from '../api/client';
import type { CreateAdminRequest } from '../api/admins';
import { usersKeys } from '../api/users';

// NotificationKey is an internal-to-the-UI identifier used by the checkbox
// group. These keys are NEVER sent on the wire: the hook maps them to the
// wire-format booleans before POSTing. Keeping the mapping centralized here
// prevents accidental leak of UI vocabulary into the DTO.
export type NotificationKey = 'maintenance' | 'marketing';

// Input shape accepted by the hook's mutate() call. The caller hands over
// the raw form fields plus the set of enabled UI checkbox keys; the hook
// assembles the wire-format request body below.
export interface CreateAdminInput {
  customerId: number;
  nome: string;
  cognome: string;
  email: string;
  telefono: string;
  notifications: ReadonlySet<NotificationKey>;
}

// useCreateAdmin posts to /api/cp-backoffice/v1/admins.
//
// DTO assembly happens here, not at the call site:
//   UI key 'maintenance' -> maintenance_on_primary_email
//   UI key 'marketing'   -> marketing_on_primary_email
//
// The hidden upstream skip-switch is intentionally NOT present in the body.
// The backend pins it to false; there is no FE path that can override it.
export function useCreateAdmin() {
  const api = useApiClient();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateAdminInput) => {
      const body: CreateAdminRequest = {
        customer_id: input.customerId,
        nome: input.nome,
        cognome: input.cognome,
        email: input.email,
        telefono: input.telefono,
        maintenance_on_primary_email: input.notifications.has('maintenance'),
        marketing_on_primary_email: input.notifications.has('marketing'),
      };
      return api.post<unknown>('/cp-backoffice/v1/admins', body);
    },
    onSuccess: (_data, vars) => {
      qc.invalidateQueries({ queryKey: usersKeys.byCustomer(vars.customerId) });
    },
  });
}
