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
  getToken: () => string | undefined;
}

export interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body: unknown) => Promise<T>;
  put: <T>(path: string, body: unknown) => Promise<T>;
  delete: <T>(path: string) => Promise<T>;
}

export function createApiClient({ baseUrl, getToken }: ApiClientOptions): ApiClient {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const token = getToken();
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
      throw new ApiError(res.status, res.statusText, path, body);
    }

    return res.json() as Promise<T>;
  }

  return {
    get: <T>(path: string) => request<T>('GET', path),
    post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
    put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
    delete: <T>(path: string) => request<T>('DELETE', path),
  };
}
