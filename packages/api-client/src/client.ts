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
  onUnauthorized?: (error: ApiError) => Promise<void> | void;
}

export interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body: unknown) => Promise<T>;
  put: <T>(path: string, body: unknown) => Promise<T>;
  delete: <T>(path: string) => Promise<T>;
  getBlob: (path: string) => Promise<Blob>;
}

export function createApiClient({ baseUrl, getToken, onUnauthorized }: ApiClientOptions): ApiClient {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const token = await getToken();
    const res = await fetch(`${baseUrl}${path}`, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      let body: unknown;
      try { body = await res.json(); } catch { /* no body */ }
      const error = new ApiError(res.status, res.statusText, path, body);
      if (res.status === 401) {
        await onUnauthorized?.(error);
      }
      throw error;
    }

    return res.json() as Promise<T>;
  }

  async function getBlob(path: string): Promise<Blob> {
    const token = await getToken();
    const res = await fetch(`${baseUrl}${path}`, {
      method: 'GET',
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    });

    if (!res.ok) {
      let body: unknown;
      try { body = await res.json(); } catch { /* no body */ }
      const error = new ApiError(res.status, res.statusText, path, body);
      if (res.status === 401) {
        await onUnauthorized?.(error);
      }
      throw error;
    }

    return res.blob();
  }

  return {
    get: <T>(path: string) => request<T>('GET', path),
    post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
    put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
    delete: <T>(path: string) => request<T>('DELETE', path),
    getBlob,
  };
}
