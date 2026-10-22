import { http } from "../transport";
import type { StellarSettingsProblem, StellarSettingsValidationResponse } from "../generated";

// 配置文档只作为文本运输。Axios 的默认 JSON 转换会损坏大整数和开放袋参数。
const rawDocument = {
  responseType: "text" as const,
  transformResponse: [(data: string) => data],
  mute: true,
};
const rawRequest = {
  headers: { "Content-Type": "application/json" },
  transformRequest: [(data: string) => data],
  mute: true,
};

export const stellarSettingsApi = {
  get: () => http.get<string>("/settings/stellar", rawDocument),
  save: (document: string) => http.put<string>("/settings/stellar", document, { ...rawDocument, ...rawRequest }),
  validate: (document: string) =>
    http.post<StellarSettingsValidationResponse>("/settings/stellar/validate", document, rawRequest),
};

// 仅诊断响应可解析为 JS 对象；主题及扩展文档始终保持原始文本。
export function readStellarSettingsProblem(error: unknown): StellarSettingsProblem | undefined {
  const response = (error as { response?: { data?: unknown } })?.response;
  let data = response?.data;
  if (typeof data === "string") {
    try {
      data = JSON.parse(data);
    } catch {
      return undefined;
    }
  }
  if (!data || typeof data !== "object" || !("issues" in data) || !("status" in data)) return undefined;
  return data as StellarSettingsProblem;
}
