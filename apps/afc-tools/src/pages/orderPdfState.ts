export const defaultOrderPdfNotReadyMessage = 'Il PDF non è ancora disponibile.';

interface OrderPdfErrorPayload {
  error?: unknown;
  message?: unknown;
}

export function getOrderPdfNotReadyMessage(status: number, body: unknown): string | null {
  if (status !== 404) return null;

  const payload = asOrderPdfErrorPayload(body);
  if (payload?.error !== 'pdf_not_ready') return null;

  return typeof payload.message === 'string' && payload.message.trim() !== ''
    ? payload.message
    : defaultOrderPdfNotReadyMessage;
}

function asOrderPdfErrorPayload(value: unknown): OrderPdfErrorPayload | null {
  return typeof value === 'object' && value !== null ? (value as OrderPdfErrorPayload) : null;
}
