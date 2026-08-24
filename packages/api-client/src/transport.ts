import axios, { AxiosError, type AxiosInstance } from "axios";

export interface ApiErrorEnvelope {
  type?: string;
  title?: string;
  status?: number;
  detail?: string;
  instance?: string;
  error?: string;
  [key: string]: unknown;
}

export interface ApiTransportOptions {
  baseURL?: string;
  timeout?: number;
  onUnauthorized?: (error: unknown) => void;
  onError?: (error: unknown, message: string) => void;
}

let onUnauthorized: ((error: unknown) => void) | undefined;
let onError: ((error: unknown, message: string) => void) | undefined;

declare module "axios" {
  interface AxiosRequestConfig {
    /** Suppress the root application's global error toast for background work. */
    mute?: boolean;
  }
}

function envelopeMessage(data: unknown): string | undefined {
  if (typeof data === "string" && data.trim() !== "") return data.trim();
  if (!data || typeof data !== "object") return undefined;
  const envelope = data as ApiErrorEnvelope;
  for (const key of ["error", "detail", "title"]) {
    const value = envelope[key];
    if (typeof value === "string" && value.trim() !== "") return value.trim();
  }
  return undefined;
}

/** Extract the stable user-facing message from the backend Problem envelope. */
export function getApiErrorMessage(error: unknown, fallback = "请求失败"): string {
  if (error instanceof AxiosError) {
    return envelopeMessage(error.response?.data) || error.message || fallback;
  }
  if (error && typeof error === "object") {
    const candidate = error as { response?: { data?: unknown }; message?: unknown };
    return (
      envelopeMessage(candidate.response?.data) ||
      (typeof candidate.message === "string" && candidate.message.trim() ? candidate.message.trim() : fallback)
    );
  }
  return fallback;
}

function handleResponseError(error: unknown): Promise<never> {
  const axiosError = error as AxiosError<ApiErrorEnvelope>;
  if (axiosError.response?.status === 401) {
    onUnauthorized?.(error);
  } else if (!axiosError.config?.mute) {
    onError?.(error, getApiErrorMessage(error));
  }
  return Promise.reject(error);
}

/** Install the package-owned response policy on a transport instance. */
export function installApiInterceptors(instance: AxiosInstance): void {
  instance.interceptors.response.use((response) => response, handleResponseError);
}

/**
 * The package owns the HTTP transport lifecycle but never imports the host
 * application's router. The root app injects the 401 behavior at startup.
 */
export const http: AxiosInstance = axios.create({
  baseURL: "/api/v1",
  timeout: 30_000,
});

installApiInterceptors(http);

export function configureApiTransport(options: ApiTransportOptions): void {
  if (options.baseURL !== undefined) http.defaults.baseURL = options.baseURL;
  if (options.timeout !== undefined) http.defaults.timeout = options.timeout;
  onUnauthorized = options.onUnauthorized;
  onError = options.onError;
}

export default http;
