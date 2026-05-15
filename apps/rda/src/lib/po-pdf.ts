const poPDFDownloadStates = new Set([
  'APPROVED',
  'PENDING_SEND',
  'SENT',
  'PENDING_VERIFICATION',
  'PENDING_DISPUTE',
  'DELIVERED_AND_COMPLIANT',
  'CLOSED',
]);

export function canDownloadPOPDF(state?: string | null): boolean {
  return Boolean(state && poPDFDownloadStates.has(state.trim()));
}
