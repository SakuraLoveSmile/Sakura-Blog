import { http, reportUnauthorized } from "../transport";
import { createSSEParser } from "../sse";
import type { JsonObject, PostPayload, StellarContentMetadata } from "./contracts";
import type {
  AIAssistEvent,
  AIAssistRequest,
  MarkdownImportResponse,
  StellarMarkdownImportRequest,
  StellarMarkdownImportResponse,
} from "../generated";

// --- Markdown 预览 ---
export const previewApi = {
  /** 渲染 Markdown。previewProfile=component-library（Spec 63）时后端进入本地 Fixture 模式 */
  render: (
    markdown: string,
    previewProfile?: string,
    metadata?: Partial<StellarContentMetadata> & {
      title?: string;
      layout?: string;
      path?: string;
      language?: string;
      mode?: "document" | "content";
    },
  ) =>
    http.post<{ html: string; document: string; preview_token: string }>(
      "/preview/markdown",
      {
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
      },
      { mute: metadata?.mode === "content" },
    ),
  /** 发布前预览：提交当前表单拿临时 token，新标签页打开 /preview/draft/:token */
  createPostPreview: (form: PostPayload) => http.post("/preview/post", form),
  /** 站点配置预览（设置页所见即所得）：草稿分段 → 前台整页 HTML */
  site: (sections: JsonObject | string, page: string) =>
    typeof sections === "string"
      ? http.post<{ html: string }>("/preview/site", `{"sections":${sections},"page":${JSON.stringify(page)}}`, {
          headers: { "Content-Type": "application/json" },
          transformRequest: [(data: string) => data],
          mute: true,
        })
      : http.post<{ html: string }>("/preview/site", { sections, page }),
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

/** 写作助手请求在流开始前就失败（校验、权限、并发、网络）。message 可以直接展示给作者。 */
export class AssistRequestError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "AssistRequestError";
    this.status = status;
  }
}

export interface AssistStreamOptions {
  signal?: AbortSignal;
  /** 每个 SSE 事件回调一次；流以 done 或 error 事件结束。 */
  onEvent: (event: AIAssistEvent) => void;
}

const assistStatusMessages: Record<number, string> = {
  401: "登录已过期，请重新登录",
  403: "没有权限使用写作助手",
  413: "选中的内容太长，请缩小范围后再试",
};

/** 取后端 Problem 信封里能直接展示的文字；响应不是 JSON 时返回空串。 */
async function readProblemMessage(response: Response): Promise<string> {
  try {
    const body: unknown = await response.json();
    if (body && typeof body === "object") {
      for (const key of ["error", "detail", "title"]) {
        const value = (body as Record<string, unknown>)[key];
        if (typeof value === "string" && value.trim() !== "") return value.trim();
      }
    }
  } catch {
    // 不是 JSON，交给按状态码的默认文案。
  }
  return "";
}

const assistInterrupted = "连接中断，结果可能不完整";

/**
 * 以 SSE 流式调用写作助手。EventSource 只支持 GET，所以用 fetch 读响应流。
 * 流开始前的失败（校验、权限、并发、网络）抛出 AssistRequestError；
 * 取消请求时重新抛出 AbortError，由调用方自己判断；流开始后的失败以 error 事件送达。
 */
async function assistStream(payload: AIAssistRequest, options: AssistStreamOptions): Promise<void> {
  const { signal, onEvent } = options;
  const base = String(http.defaults.baseURL ?? "").replace(/\/+$/, "");
  let response: Response;
  try {
    response = await fetch(`${base}/ai/assist`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
      body: JSON.stringify(payload),
      credentials: "same-origin",
      signal,
    });
  } catch (error) {
    if (signal?.aborted) throw error;
    throw new AssistRequestError(0, "网络连接失败，请检查网络后重试");
  }

  if (!response.ok) {
    const detail = await readProblemMessage(response);
    const failure = new AssistRequestError(
      response.status,
      detail || assistStatusMessages[response.status] || `请求失败（HTTP ${response.status}）`,
    );
    if (response.status === 401) reportUnauthorized(failure);
    throw failure;
  }
  if (!response.body) throw new AssistRequestError(0, "当前浏览器不支持流式读取");

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let finished = false;
  const parser = createSSEParser(({ data }) => {
    let event: AIAssistEvent;
    try {
      event = JSON.parse(data) as AIAssistEvent;
    } catch {
      return; // 无法解析的数据行不是事件，跳过。
    }
    if (event.type === "done" || event.type === "error") finished = true;
    onEvent(event);
  });

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      parser.push(decoder.decode(value, { stream: true }));
    }
    parser.push(decoder.decode());
    parser.end();
  } catch (error) {
    if (signal?.aborted) throw error;
    if (!finished) onEvent({ type: "error", message: assistInterrupted });
    return;
  } finally {
    reader.cancel().catch(() => undefined);
  }
  // 连接被对端正常关闭，却没有 done/error：结果不能当作完整。
  if (!finished) onEvent({ type: "error", message: assistInterrupted });
}

export const aiApi = {
  generate: (payload: AiGeneratePayload) => http.post("/ai/generate", payload),
  assistStream,
};
