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
  DRAFT: 'Bozza',
  PENDING_APPROVAL: 'Attesa approvazione',
  APPROVED: 'Approvata',
  REJECTED: 'Rifiutata',
  PENDING_APPROVAL_PAYMENT_METHOD: 'Attesa metodo',
  PENDING_LEASING: 'Attesa leasing',
  PENDING_LEASING_ORDER_CREATION: 'Ordine leasing',
  PENDING_APPROVAL_NO_LEASING: 'Attesa no leasing',
  PENDING_BUDGET_INCREMENT: 'Attesa budget',
  PENDING_SEND: 'Da inviare',
  PENDING_VERIFICATION: 'Attesa conformità',
  DELIVERED_AND_COMPLIANT: 'Erogata conforme',
  SENT: 'Inviata',
  CLOSED: 'Chiusa',
};

const fullLabels: Record<string, string> = {
  DRAFT: 'Bozza',
  PENDING_APPROVAL: 'In attesa approvazione',
  APPROVED: 'Approvata',
  REJECTED: 'Rifiutata',
  PENDING_APPROVAL_PAYMENT_METHOD: 'In attesa approvazione metodo pagamento',
  PENDING_LEASING: 'In attesa leasing',
  PENDING_LEASING_ORDER_CREATION: 'In attesa creazione ordine leasing',
  PENDING_APPROVAL_NO_LEASING: 'In attesa approvazione no leasing',
  PENDING_BUDGET_INCREMENT: 'In attesa incremento budget',
  PENDING_SEND: 'In attesa invio al fornitore',
  PENDING_VERIFICATION: 'In attesa verifica conformità',
  DELIVERED_AND_COMPLIANT: 'Erogata e conforme',
  SENT: 'Inviata al fornitore',
  CLOSED: 'Chiusa',
};

export function stateLabel(state?: string | null): string {
  if (!state) return '-';
  return labels[state] ?? state.replaceAll('_', ' ');
}

export function stateFullLabel(state?: string | null): string {
  if (!state) return '-';
  return fullLabels[state] ?? state.replaceAll('_', ' ');
}

export type StateTone =
  | 'neutral'
  | 'approved'
  | 'closed'
  | 'compliant'
  | 'danger'
  | 'approval'
  | 'budget'
  | 'draft'
  | 'leasing'
  | 'payment'
  | 'send'
  | 'sent'
  | 'verification';

export function stateTone(state?: string | null): StateTone {
  if (!state) return 'neutral';
  if (state === 'DRAFT') return 'draft';
  if (state.includes('REJECT')) return 'danger';
  if (state === 'APPROVED') return 'approved';
  if (state === 'DELIVERED_AND_COMPLIANT') return 'compliant';
  if (state === 'SENT') return 'sent';
  if (state === 'CLOSED') return 'closed';
  if (state === 'PENDING_APPROVAL') return 'approval';
  if (state === 'PENDING_BUDGET_INCREMENT') return 'budget';
  if (state === 'PENDING_APPROVAL_PAYMENT_METHOD') return 'payment';
  if (state === 'PENDING_LEASING' || state === 'PENDING_LEASING_ORDER_CREATION') return 'leasing';
  if (state === 'PENDING_SEND') return 'send';
  if (state === 'PENDING_VERIFICATION') return 'verification';
  if (state === 'PENDING_APPROVAL_NO_LEASING') return 'approval';
  return 'neutral';
}
