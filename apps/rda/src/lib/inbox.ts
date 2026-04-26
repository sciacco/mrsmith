import {
  RDA_APPROVER_AFC_ROLE,
  RDA_APPROVER_EXTRA_BUDGET_ROLE,
  RDA_APPROVER_L1L2_ROLE,
  RDA_APPROVER_NO_LEASING_ROLE,
} from './roles';

export type InboxKind = 'level1-2' | 'leasing' | 'no-leasing' | 'payment-method' | 'budget-increment';

export const inboxConfig: Record<InboxKind, { title: string; role: string }> = {
  'level1-2': { title: 'Approvazioni I° / II° livello', role: RDA_APPROVER_L1L2_ROLE },
  leasing: { title: 'Approvazioni Leasing', role: RDA_APPROVER_AFC_ROLE },
  'no-leasing': { title: 'Approvazioni No-Leasing', role: RDA_APPROVER_NO_LEASING_ROLE },
  'payment-method': { title: 'Approvazioni Metodo Pagamento', role: RDA_APPROVER_AFC_ROLE },
  'budget-increment': { title: 'Approvazioni Incremento Budget', role: RDA_APPROVER_EXTRA_BUDGET_ROLE },
};

export function isInboxKind(value: string | undefined): value is InboxKind {
  return Boolean(value && value in inboxConfig);
}
