import { ApiError } from '@mrsmith/api-client';

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
    return error.message;
  }
  return fallback;
}
