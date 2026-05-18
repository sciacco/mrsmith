const requesterDeleteStates = new Set([
  'DRAFT',
  'PENDING_APPROVAL_PROVIDER',
  'PENDING_APPROVAL',
  'PENDING_APPROVAL_PAYMENT_METHOD',
  'PENDING_BUDGET_INCREMENT',
]);

export function isRequesterDeletablePOState(state?: string | null): boolean {
  return Boolean(state && requesterDeleteStates.has(state.trim()));
}
