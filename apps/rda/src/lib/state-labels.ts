export const PO_STATES = {
  DRAFT: 'DRAFT',
  PENDING_APPROVAL: 'PENDING_APPROVAL',
  PENDING_APPROVAL_PAYMENT_METHOD: 'PENDING_APPROVAL_PAYMENT_METHOD',
  PENDING_LEASING: 'PENDING_LEASING',
  PENDING_LEASING_ORDER_CREATION: 'PENDING_LEASING_ORDER_CREATION',
  PENDING_APPROVAL_NO_LEASING: 'PENDING_APPROVAL_NO_LEASING',
  PENDING_BUDGET_INCREMENT: 'PENDING_BUDGET_INCREMENT',
  PENDING_SEND: 'PENDING_SEND',
  PENDING_VERIFICATION: 'PENDING_VERIFICATION',
} as const;

const labels: Record<string, string> = {
  DRAFT: 'BOZZA',
  PENDING_APPROVAL: 'IN ATTESA APPROVAZIONE',
  APPROVED: 'APPROVATO',
  REJECTED: 'RIFIUTATO',
  PENDING_APPROVAL_PAYMENT_METHOD: 'IN ATTESA METODO PAGAMENTO',
  PENDING_LEASING: 'IN ATTESA LEASING',
  PENDING_LEASING_ORDER_CREATION: 'IN ATTESA CREAZIONE LEASING',
  PENDING_APPROVAL_NO_LEASING: 'IN ATTESA NO LEASING',
  PENDING_BUDGET_INCREMENT: 'IN ATTESA INCREMENTO BUDGET',
  PENDING_SEND: 'IN ATTESA INVIO',
  PENDING_VERIFICATION: 'IN ATTESA VERIFICA CONFORMITÀ',
  SENT: 'INVIATO',
  CLOSED: 'CHIUSO',
};

export function stateLabel(state?: string | null): string {
  if (!state) return '-';
  return labels[state] ?? state.replaceAll('_', ' ');
}

export function stateTone(state?: string | null): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  if (!state) return 'neutral';
  if (state === 'DRAFT') return 'neutral';
  if (state.includes('REJECT')) return 'danger';
  if (state.includes('APPROVED') || state === 'CLOSED' || state === 'SENT') return 'success';
  if (state.includes('PENDING')) return 'warning';
  return 'info';
}
