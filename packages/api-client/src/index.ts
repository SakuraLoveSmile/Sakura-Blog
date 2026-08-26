// Package entrypoint: keep the historical @/api barrel stable while the
// implementation is organized by API domain under src/api/.
export { configureApiTransport, getApiErrorMessage, http, installApiInterceptors } from "./transport";
export type { ApiErrorEnvelope, ApiTransportOptions } from "./transport";
export type { Breakpoint, DashboardResponsiveLayout, DashboardWidget } from "./types";
export type * from "./generated";

export * from "./api/auth";
export * from "./api/contracts";
export * from "./api/content";
export * from "./api/dashboard";
export * from "./api/editor";
export * from "./api/links";
export * from "./api/media";
export * from "./api/monitoring";
export * from "./api/plugins";
export * from "./api/rag";
export * from "./api/settings";
export * from "./api/wiki";
export * from "./api/uc";

export { default } from "./transport";
