import { ApiError } from '@mrsmith/api-client';

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
    return error.message;
  }
  return fallback;
}

export function conformityConfirmationErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
      const upstreamMessage = body.error.toLowerCase();
      if (upstreamMessage.includes('ddt') || upstreamMessage.includes('delivery note')) {
        return 'Impossibile confermare la conformità. Carica il DDT per gli articoli di tipo bene e riprova.';
      }
    }
  }
  return apiErrorMessage(error, 'Conferma della conformità non riuscita');
}
