import { http } from "../transport";
import type { JsonObject, PostPayload, StellarContentMetadata } from "./contracts";
import type { MarkdownImportResponse, StellarMarkdownImportRequest, StellarMarkdownImportResponse } from "../generated";

// --- Markdown 预览 ---
export const previewApi = {
  /** 渲染 Markdown。previewProfile=component-library（Spec 63）时后端进入本地 Fixture 模式 */
  render: (
    markdown: string,
    previewProfile?: string,
    metadata?: Partial<StellarContentMetadata> & { title?: string; layout?: string; path?: string; language?: string; mode?: "document" | "content" },
  ) =>
    http.post<{ html: string; document: string; preview_token: string }>("/preview/markdown", {
      markdown,
      preview_profile: previewProfile,
      mode: metadata?.mode,
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
      ...metadata,
      preview_token: Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
        byte.toString(16).padStart(2, "0"),
      ).join(""),
    }),
  /** 发布前预览：提交当前表单拿临时 token，新标签页打开 /preview/draft/:token */
  createPostPreview: (form: PostPayload) => http.post("/preview/post", form),
  /** 站点配置预览（设置页所见即所得）：草稿分段 → 前台整页 HTML */
  site: (sections: JsonObject, page: string) => http.post<{ html: string }>("/preview/site", { sections, page }),
};

// --- Markdown 导入（仅解析，不落库） ---
export const editorImportApi = {
  stellar: (payload: StellarMarkdownImportRequest) =>
    http.post<StellarMarkdownImportResponse>("/editor/import/stellar", payload),
  exportStellar: (payload: { front_matter: string; blog_extensions: string; content_md: string }) =>
    http.post<{ content: string }>("/editor/export/stellar", payload),
  markdown: (payload: { filename: string; content: string; kind: "post" | "page" }) =>
    http.post<MarkdownImportResponse>("/editor/import/markdown", payload),
};

// --- AI 属性生成（OpenAI 兼容代理，配置在 设置 → AI） ---
export interface AiGeneratePayload {
  fields: string[];
  title: string;
  content: string;
  categories: string[];
  tags: string[];
}

export const aiApi = {
  generate: (payload: AiGeneratePayload) => http.post("/ai/generate", payload),
};
