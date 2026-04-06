import { ApiError } from '@mrsmith/api-client';

interface ApiErrorBody {
  code?: string;
  error?: string;
  message?: string;
}

function getBody(error: ApiError): ApiErrorBody | undefined {
  if (!error.body || typeof error.body !== 'object') return undefined;
  return error.body as ApiErrorBody;
}

export function isUpstreamAuthFailed(error: unknown): error is ApiError {
  return (
    error instanceof ApiError &&
    error.status === 502 &&
    getBody(error)?.code === 'UPSTREAM_AUTH_FAILED'
  );
}

export function getApiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = getBody(error);
    return body?.message ?? body?.error ?? error.statusText;
  }
  return 'Errore di connessione';
}
