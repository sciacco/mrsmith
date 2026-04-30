import type { RdaPermissions } from '../api/types';

export type InboxKind = 'level1-2' | 'leasing' | 'no-leasing' | 'payment-method' | 'budget-increment';

export const inboxOrder: InboxKind[] = ['level1-2', 'leasing', 'no-leasing', 'payment-method', 'budget-increment'];

export const inboxConfig: Record<InboxKind, { title: string; shortTitle: string; permission: keyof RdaPermissions }> = {
  'level1-2': { title: 'Approvazioni I° / II° livello', shortTitle: 'I/II livello', permission: 'is_approver' },
  leasing: { title: 'Approvazioni Leasing', shortTitle: 'Leasing', permission: 'is_afc' },
  'no-leasing': { title: 'Approvazioni No-Leasing', shortTitle: 'No leasing', permission: 'is_approver_no_leasing' },
  'payment-method': { title: 'Approvazioni Metodo Pagamento', shortTitle: 'Metodo pagamento', permission: 'is_afc' },
  'budget-increment': { title: 'Approvazioni Incremento Budget', shortTitle: 'Incremento budget', permission: 'is_approver_extra_budget' },
};

export function isInboxKind(value: string | undefined): value is InboxKind {
  return Boolean(value && value in inboxConfig);
}

export function authorizedInboxKinds(permissions?: RdaPermissions): InboxKind[] {
  if (!permissions) return [];
  return inboxOrder.filter((kind) => permissions[inboxConfig[kind].permission]);
}
