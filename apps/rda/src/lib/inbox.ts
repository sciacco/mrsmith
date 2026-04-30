import type { RdaPermissions } from '../api/types';

export type InboxKind = 'level1-2' | 'leasing' | 'no-leasing' | 'payment-method' | 'budget-increment';

export const inboxConfig: Record<InboxKind, { title: string; permission: keyof RdaPermissions }> = {
  'level1-2': { title: 'Approvazioni I° / II° livello', permission: 'is_approver' },
  leasing: { title: 'Approvazioni Leasing', permission: 'is_afc' },
  'no-leasing': { title: 'Approvazioni No-Leasing', permission: 'is_approver_no_leasing' },
  'payment-method': { title: 'Approvazioni Metodo Pagamento', permission: 'is_afc' },
  'budget-increment': { title: 'Approvazioni Incremento Budget', permission: 'is_approver_extra_budget' },
};

export function isInboxKind(value: string | undefined): value is InboxKind {
  return Boolean(value && value in inboxConfig);
}
