export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public path: string,
    public body?: unknown,
  ) {
    super(`API ${status} ${statusText}: ${path}`);
  }
}

export interface ApiClientOptions {
  baseUrl: string;
  getToken: () => Promise<string | undefined> | string | undefined;
  /**
   * Forces a token refresh. Called after a 401 so we can retry once with a fresh token.
   * When omitted, the 401 retry path is skipped and the original error is thrown.
   */
  forceRefreshToken?: () => Promise<string | undefined> | string | undefined;
  /**
   * Invoked when a 401 is received and the refresh-and-retry failed. Typical use:
   * trigger an interactive login redirect.
   */
  onUnauthorized?: (error: ApiError) => Promise<void> | void;
}

export interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body?: unknown) => Promise<T>;
  put: <T>(path: string, body?: unknown) => Promise<T>;
  patch: <T>(path: string, body?: unknown) => Promise<T>;
  delete: <T>(path: string) => Promise<T>;
  getBlob: (path: string) => Promise<Blob>;
  postBlob: (path: string, body?: unknown) => Promise<Blob>;
}

type BodyMode = 'json' | 'blob';

export function createApiClient({
  baseUrl,
  getToken,
  forceRefreshToken,
  onUnauthorized,
}: ApiClientOptions): ApiClient {
  // Single-flight guard: if multiple requests receive 401 concurrently, only the
  // first triggers a refresh; the others await the same promise and reuse the result.
  let pendingRefresh: Promise<string | undefined> | null = null;

  async function refreshOnce(): Promise<string | undefined> {
    if (!forceRefreshToken) return undefined;
    if (pendingRefresh) return pendingRefresh;
    pendingRefresh = Promise.resolve(forceRefreshToken()).finally(() => {
      pendingRefresh = null;
    });
    return pendingRefresh;
  }

  function buildHeaders(token: string | undefined, mode: BodyMode): HeadersInit {
    const headers: Record<string, string> = {};
    if (mode === 'json') headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    return headers;
  }

  async function doFetch(
    path: string,
    method: string,
    token: string | undefined,
    body: unknown,
    mode: BodyMode,
  ): Promise<Response> {
    const init: RequestInit = {
      method,
      headers: buildHeaders(token, mode),
    };
    if (body !== undefined) {
      init.body = mode === 'json' ? JSON.stringify(body) : (body as BodyInit);
    }
    return fetch(`${baseUrl}${path}`, init);
  }

  async function sendWithRetry(
    path: string,
    method: string,
    body: unknown,
    mode: BodyMode,
  ): Promise<Response> {
    const token = await getToken();
    let res = await doFetch(path, method, token, body, mode);

    if (res.status === 401 && forceRefreshToken) {
      const fresh = await refreshOnce();
      if (fresh) {
        res = await doFetch(path, method, fresh, body, mode);
      }
    }
    return res;
  }

  async function readErrorBody(res: Response): Promise<unknown> {
    try {
      return await res.clone().json();
    } catch {
      return undefined;
    }
  }

  async function requestJSON<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await sendWithRetry(path, method, body, 'json');
    if (!res.ok) {
      const payload = await readErrorBody(res);
      const error = new ApiError(res.status, res.statusText, path, payload);
      if (res.status === 401) await onUnauthorized?.(error);
      throw error;
    }
    if (res.status === 204) return undefined as T;
    return res.json() as Promise<T>;
  }

  async function requestBlob(method: string, path: string, body?: unknown): Promise<Blob> {
    const res = await sendWithRetry(path, method, body, body === undefined ? 'blob' : 'json');
    if (!res.ok) {
      const payload = await readErrorBody(res);
      const error = new ApiError(res.status, res.statusText, path, payload);
      if (res.status === 401) await onUnauthorized?.(error);
      throw error;
    }
    return res.blob();
  }

  // POST/PUT/PATCH without an explicit body default to `{}` so the backend
  // always receives valid JSON when Content-Type: application/json is set.
  const emptyIfMissing = (body: unknown) => (body === undefined ? {} : body);

  return {
    get: <T>(path: string) => requestJSON<T>('GET', path),
    post: <T>(path: string, body?: unknown) => requestJSON<T>('POST', path, emptyIfMissing(body)),
    put: <T>(path: string, body?: unknown) => requestJSON<T>('PUT', path, emptyIfMissing(body)),
    patch: <T>(path: string, body?: unknown) => requestJSON<T>('PATCH', path, emptyIfMissing(body)),
    delete: <T>(path: string) => requestJSON<T>('DELETE', path),
    getBlob: (path: string) => requestBlob('GET', path),
    postBlob: (path: string, body?: unknown) => requestBlob('POST', path, emptyIfMissing(body)),
  };
}
