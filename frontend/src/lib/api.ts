// API client para comunicação com o backend
import type { ResponseError } from '../types/errors';
import { isResponseError, getErrorMessage } from '../types/errors';

const API_BASE_URL = '/v1';
// Worker-backed endpoints can legitimately take much longer than a typical
// API call: the backend retries qBittorrent logins up to 5x with exponential
// backoff (each attempt itself bounded by its own request timeout), so a
// single request can take 60-90s+ before failing. Keep this comfortably above
// that worst case to avoid aborting requests the backend would have answered.
const DEFAULT_TIMEOUT_MS = 120_000;

export interface ApiResponse<T> {
  data?: T;
  error?: string;
  errorDetails?: ResponseError;
  status?: number;
}

export interface RequestConfig {
  // Lets callers cancel in-flight requests (e.g. on unmount/navigation) so a
  // stale response can't land and overwrite newer state.
  signal?: AbortSignal;
}

class ApiClient {
  private readonly baseURL: string;

  constructor(baseURL: string = API_BASE_URL) {
    this.baseURL = baseURL;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {},
    config: RequestConfig = {}
  ): Promise<ApiResponse<T>> {
    const url = `${this.baseURL}${endpoint}`;

    // Se headers está vazio (FormData), não adicionar Content-Type
    const shouldAddContentType = options.headers === undefined ||
      (typeof options.headers === 'object' && Object.keys(options.headers).length > 0);

    // Merge computed headers with caller-provided ones instead of letting a
    // later spread of `options` silently drop the computed Content-Type.
    const headers = {
      ...(shouldAddContentType ? { 'Content-Type': 'application/json' } : {}),
      ...options.headers,
    };

    // Bound every request with a timeout so an unreachable backend/worker
    // can't hang a caller forever, and honor a caller-supplied signal so
    // components can cancel on unmount or when a newer request supersedes
    // this one.
    const timeoutController = new AbortController();
    const timeoutId = setTimeout(() => timeoutController.abort(), DEFAULT_TIMEOUT_MS);

    const signal = config.signal && typeof AbortSignal.any === 'function'
      ? AbortSignal.any([timeoutController.signal, config.signal])
      : config.signal ?? timeoutController.signal;

    try {
      const response = await fetch(url, {
        ...options,
        headers,
        credentials: 'include', // Include cookies in requests
        signal,
      });

      // Handle 204 No Content responses (no body to parse)
      if (response.status === 204) {
        return { data: null as T, status: response.status };
      }

      // Check if response has content to parse
      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        const data = await response.json();

        // Handle error responses
        if (!response.ok) {
          // Check if it's a structured ResponseError from backend
          if (isResponseError(data)) {
            return {
              error: getErrorMessage(data) || `HTTP error! status: ${response.status}`,
              errorDetails: data,
              data: undefined,
              status: response.status,
            };
          }

          // Fallback for other error formats
          return {
            error: getErrorMessage(data) || `HTTP error! status: ${response.status}`,
            data: undefined,
            status: response.status,
          };
        }

        return { data, status: response.status };
      } else {
        if (!response.ok) {
          const text = await response.text();
          return {
            error: text || `HTTP error! status: ${response.status}`,
            data: undefined,
            status: response.status,
          };
        }
        // For non-JSON responses, return the response text or null
        const text = await response.text();
        return { data: (text || null) as T, status: response.status };
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        if (config.signal?.aborted) {
          return { error: 'Request cancelled' };
        }
        return { error: 'Request timed out' };
      }
      console.error('API request failed:', error);
      return {
        error: error instanceof Error ? error.message : 'Unknown error'
      };
    } finally {
      clearTimeout(timeoutId);
    }
  }

  // GET request
  async get<T>(endpoint: string, config?: RequestConfig): Promise<ApiResponse<T>> {
    return this.request<T>(endpoint, { method: 'GET' }, config);
  }

  // POST request
  async post<T>(endpoint: string, data?: unknown, config?: RequestConfig): Promise<ApiResponse<T>> {
    // Se data é FormData, não fazer stringify e remover Content-Type para o browser definir
    if (data instanceof FormData) {
      return this.request<T>(endpoint, {
        method: 'POST',
        body: data,
        headers: {}, // Remove Content-Type header para FormData
      }, config);
    }

    return this.request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    }, config);
  }

  // PUT request
  async put<T>(endpoint: string, data?: unknown, config?: RequestConfig): Promise<ApiResponse<T>> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    }, config);
  }

  // DELETE request
  async delete<T>(endpoint: string, config?: RequestConfig): Promise<ApiResponse<T>> {
    return this.request<T>(endpoint, { method: 'DELETE' }, config);
  }
}

// Instância padrão do cliente API
export const api = new ApiClient();

// Exemplo de uso:
// const response = await api.get('/health');
// if (response.data) {
//   console.log('Health check:', response.data);
// } else {
//   console.error('Error:', response.error);
// }
