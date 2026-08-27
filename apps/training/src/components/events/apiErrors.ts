// Lettura uniforme degli errori applicativi del backend Training (#156): gli
// errori 4xx vanno mostrati per intero (vincolo di slice), mai riassorbiti da
// una state machine locale. { error: codice, message: testo italiano }.

import { ApiError } from '@mrsmith/api-client';

export interface TrainingApiErrorBody {
  error?: string;
  message?: string;
}

export function apiErrorBody(error: unknown): TrainingApiErrorBody | undefined {
  if (error instanceof ApiError && error.body && typeof error.body === 'object') {
    return error.body as TrainingApiErrorBody;
  }
  return undefined;
}

export function apiErrorCode(error: unknown): string | undefined {
  return apiErrorBody(error)?.error;
}

export function describeApiError(error: unknown, fallback: string): string {
  const body = apiErrorBody(error);
  if (body?.message) return body.message;
  if (error instanceof ApiError) return `${fallback} (${error.status})`;
  return fallback;
}
