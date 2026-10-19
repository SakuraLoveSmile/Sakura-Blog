import { http } from "../transport";
import type { JsonValue } from "./contracts";
import type {
  StellarAuthorCatalogResponse,
  StyleYAMLResponse,
  StyleYAMLUpdateResponse,
  RestoreTaskResponse,
  RestoreTasksResponse,
  FeedbackCheckResponse,
} from "../generated";

// --- 备份与导出 ---
// 下载接口走 cookie 认证，直接 <a href> 触发浏览器下载，不走 axios。
export const backupExportUrl = "/api/v1/backup/export";
export const backupExportMarkdownUrl = "/api/v1/backup/export/markdown";

export type RestoreTask = RestoreTaskResponse;

export class RestoreTaskHttpError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "RestoreTaskHttpError";
    this.status = status;
  }
}

async function readRestoreTaskResponse(response: Response): Promise<RestoreTaskResponse> {
  const body = (await response.json().catch(() => ({}))) as RestoreTask & { error?: string; detail?: string };
  if (!response.ok) {
    throw new RestoreTaskHttpError(response.status, body.error || body.detail || "读取恢复任务失败");
  }
  return body;
}

export const backupApi = {
  status: () => http.get("/backup/status"),
  importMarkdown: (file: File, legacy = false) => {
    const form = new FormData();
    form.append("file", file);
    if (legacy) form.append("legacy", "true");
    return http.post("/backup/import/markdown", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  // 整站恢复（Spec 41）：上传全量备份 zip / 从服务器留存备份还原
  restore: (file: File, strict: boolean) => {
    const form = new FormData();
    form.append("file", file);
    form.append("strict", strict ? "true" : "false");
    return http.post("/backup/restore", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  restoreSnapshot: (name: string) => http.post("/backup/restore-snapshot", { name }),
  startRestore: (file: File, strict: boolean, idempotencyKey: string) => {
    const form = new FormData();
    form.append("file", file);
    form.append("strict", strict ? "true" : "false");
    return http.post<RestoreTask>("/backup/restores", form, {
      headers: { "Content-Type": "multipart/form-data", "Idempotency-Key": idempotencyKey },
    });
  },
  startSnapshotRestore: (snapshot: string, strict: boolean, idempotencyKey: string) =>
    http.post<RestoreTask>(
      "/backup/restores",
      { snapshot, strict },
      { headers: { "Idempotency-Key": idempotencyKey } },
    ),
  listRestoreTasks: () => http.get<RestoreTasksResponse>("/backup/restores"),
  getRestoreTask: (id: string) =>
    fetch(`/api/v1/backup/restores/${encodeURIComponent(id)}`, { credentials: "same-origin" }).then(
      readRestoreTaskResponse,
    ),
};

// --- Settings ---
// 段名与后端 KnownSections 对齐；Spec 42 新增 seo（站点级 SEO）与 inject（代码注入 inject_head/inject_footer）两段。
export const settingsApi = {
  all: () => http.get("/settings"),
  authorCatalog: () => http.get<StellarAuthorCatalogResponse>("/authors/catalog"),
  updateSection: (section: string, data: JsonValue | undefined) => http.put(`/settings/${section}`, data),
  testNotify: () => http.post("/settings/notify/test"),
  // Feedback 服务连通性自测：提交当前草稿 api_base，后端探测其 /healthz
  checkFeedback: (apiBase: string) =>
    http.post<FeedbackCheckResponse>("/settings/feedback/check", { api_base: apiBase }),
  // 样式 YAML 编辑器：style 段 YAML 读写 + 热更新
  getStyleYAML: () => http.get<StyleYAMLResponse>("/settings/style/yaml"),
  updateStyleYAML: (yamlText: string) =>
    http.put<StyleYAMLUpdateResponse>("/settings/style/yaml", yamlText, { headers: { "Content-Type": "text/yaml" } }),
};
