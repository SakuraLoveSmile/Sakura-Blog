import axios, { type AxiosInstance } from "axios";

export interface ApiTransportOptions {
  baseURL?: string;
  timeout?: number;
  onUnauthorized?: () => void;
}

let onUnauthorized: (() => void) | undefined;

/**
 * The package owns the HTTP transport lifecycle but never imports the host
 * application's router. The root app injects the 401 behavior at startup.
 */
export const http: AxiosInstance = axios.create({
  baseURL: "/api/v1",
  timeout: 30_000,
});

http.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) onUnauthorized?.();
    return Promise.reject(error);
  },
);

export function configureApiTransport(options: ApiTransportOptions): void {
  if (options.baseURL !== undefined) http.defaults.baseURL = options.baseURL;
  if (options.timeout !== undefined) http.defaults.timeout = options.timeout;
  onUnauthorized = options.onUnauthorized;
}

export default http;
