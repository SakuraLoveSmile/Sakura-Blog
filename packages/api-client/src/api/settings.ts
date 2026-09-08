import { http } from "../transport";
import type { JsonValue } from "./contracts";
import type { StyleYAMLResponse, StyleYAMLUpdateResponse } from "../generated";

// --- 备份与导出 ---
// 下载接口走 cookie 认证，直接 <a href> 触发浏览器下载，不走 axios。
export const backupExportUrl = "/api/v1/backup/export";
export const backupExportMarkdownUrl = "/api/v1/backup/export/markdown";
export const backupApi = {
  status: () => http.get("/backup/status"),
  importMarkdown: (file: File) => {
    const form = new FormData();
    form.append("file", file);
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
};

// --- Settings ---
// 段名与后端 KnownSections 对齐；Spec 42 新增 seo（站点级 SEO）与 inject（代码注入 inject_head/inject_footer）两段。
export const settingsApi = {
  all: () => http.get("/settings"),
  updateSection: (section: string, data: JsonValue | undefined) => http.put(`/settings/${section}`, data),
  testNotify: () => http.post("/settings/notify/test"),
  // 样式 YAML 编辑器：style 段 YAML 读写 + 热更新
  getStyleYAML: () => http.get<StyleYAMLResponse>("/settings/style/yaml"),
  updateStyleYAML: (yamlText: string) =>
    http.put<StyleYAMLUpdateResponse>("/settings/style/yaml", yamlText, { headers: { "Content-Type": "text/yaml" } }),
};
