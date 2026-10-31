// 标签插件共用完整前台预览文档；component-library Fixture 不写真实投票/评分。
// BLG-20261031-03：调度不再清空当前文档；命中缓存同步应用；迟到响应丢弃；
// 刷新期间保留旧 iframe/高度，失败保留上次成功预览并可重试。

import { computed, onBeforeUnmount, ref } from "vue";
import { previewApi } from "@/api";
import type { PluginPreviewInfo } from "../plugin-types.ts";
import {
  MAX_PREVIEW_HEIGHT,
  preparePreviewDocument,
  previewHeightFromMessage,
  resolvePreviewHeight,
  type MockFixtureConfig,
} from "./preview-document.ts";
import {
  PreviewScheduler,
  sharedPreviewCache,
  type PreviewCache,
  type PreviewQueue,
  type PreviewStatus,
} from "./render-gate.ts";

export interface PluginPreviewFrameLimits {
  /** iframe 高度下限（缺省用插件推荐高度） */
  minHeight?: number;
  /** iframe 高度上限（缺省 MAX_PREVIEW_HEIGHT） */
  maxHeight?: number;
  /** 内容高度额外留白（缺省 24px；NodeView 传 0 保持原始语义） */
  margin?: number;
}

export interface PluginPreviewOptions {
  /** 合并防抖（缺省 400ms） */
  delayMs?: number;
  /** 可选模块级并发闸（正文 NodeView 使用 sharedPreviewQueue） */
  queue?: PreviewQueue;
  /** 缓存实例（缺省共享 LRU 20 条） */
  cache?: PreviewCache;
  frame?: PluginPreviewFrameLimits;
}

export interface PluginPreviewRenderOptions {
  immediate?: boolean;
  delayMs?: number;
  /** null = 真实站点上下文（正文预览）；缺省 component-library */
  profile?: string | null;
  metadata?: Record<string, unknown>;
  cacheKey?: string;
  bypassCache?: boolean;
}

export function usePluginPreview(
  getInfo: () => PluginPreviewInfo,
  getOverrides?: () => Record<string, MockFixtureConfig> | undefined,
  options: PluginPreviewOptions = {},
) {
  const previewHtml = ref("");
  const document = ref("");
  const previewToken = ref("");
  const status = ref<PreviewStatus>("idle");
  const previewError = ref("");
  function frameLimits() {
    // 缺省下限沿用插件推荐高度（旧语义）；只有显式 frame.minHeight 才覆盖。
    return {
      min: options.frame?.minHeight ?? resolvePreviewHeight(getInfo()),
      max: options.frame?.maxHeight ?? MAX_PREVIEW_HEIGHT,
    };
  }
  function recommendedFrameHeight(): number {
    const { min, max } = frameLimits();
    return Math.min(max, Math.max(min, resolvePreviewHeight(getInfo())));
  }

  // 初始高度 = 当前插件的推荐高度（选中标签先应用推荐高度，再按 iframe 内容回传调整）
  const frameHeight = ref(recommendedFrameHeight());
  // 输入已变化但新请求还没发出（例如等待短code解析）时冻结收缩，避免旧文档兜底回传把高度先压小。
  let heightHold = false;
  const rendering = computed(() => status.value === "loading" || status.value === "updating");
  const hasDocument = computed(() => Boolean(previewHtml.value && document.value));

  // iframe直接消费前台生成文档，既有可选Fixture覆盖交由前台预览宿主实现。
  const srcdoc = computed(() => document.value);
  void getOverrides;

  const scheduler = new PreviewScheduler({
    delayMs: options.delayMs ?? 400,
    cache: options.cache ?? sharedPreviewCache,
    queue: options.queue,
    run: async (markdown, profile, metadata) => {
      const { data } = await previewApi.render(markdown, profile, { ...metadata, mode: "content" });
      // 写守卫始终注入；内存 Storage 只在 component-library 预览注入，
      // 真实站上下文（NodeView）保持 rating/vote 不可交互的旧行为。
      return { html: data.html, document: preparePreviewDocument(data.document, profile), token: data.preview_token };
    },
    onChange: (state) => {
      status.value = state.status;
      previewError.value = state.error;
      if (state.result) {
        previewHtml.value = state.result.html;
        document.value = state.result.document;
        previewToken.value = state.result.token;
      } else if (state.status === "idle") {
        previewHtml.value = "";
        document.value = "";
        previewToken.value = "";
      }
      // 失败时也释放冻结：旧文档继续展示，允许其后续高度回传恢复正常收缩。
      if (state.status === "ready" || state.status === "idle" || state.status === "error") heightHold = false;
    },
  });

  /** 调度一次渲染：不清空当前预览；空输入等同显式 clearPreview。 */
  function scheduleRender(
    markdown: string,
    immediateOrOptions: boolean | PluginPreviewRenderOptions = false,
  ): void {
    const opts = typeof immediateOrOptions === "boolean" ? { immediate: immediateOrOptions } : immediateOrOptions;
    scheduler.schedule({
      markdown,
      // 未显式给出 profile 时走组件库 Fixture；NodeView 传 null 使用真实上下文。
      profile: opts.profile === undefined ? "component-library" : opts.profile,
      metadata: opts.metadata,
      cacheKey: opts.cacheKey,
      delayMs: opts.delayMs,
      immediate: opts.immediate,
      bypassCache: opts.bypassCache,
    });
  }

  /** 清空或无效参数时销毁旧文档，并使在途响应失效。 */
  function clearPreview(): void {
    scheduler.clear();
  }

  /** 失败后按上次请求重试（跳过缓存）。 */
  function retry(): void {
    scheduler.retry();
  }

  /** 主动失效缓存；不传 key 时清空共享缓存。 */
  function invalidateCache(key?: string): void {
    scheduler.invalidate(key);
  }

  /** 输入变化时调用：新文档就绪前不把高度收缩。 */
  function holdHeight(): void {
    heightHold = true;
  }

  /** 选中插件时重置为推荐高度（受 options.frame 的 min/max 钳制，避免先小后跳）。 */
  function resetHeight(): void {
    heightHold = false;
    frameHeight.value = recommendedFrameHeight();
  }

  /** iframe 高度回传处理（event.source 必须是当前预览 iframe）。 */
  function onMessage(event: MessageEvent, frame: HTMLIFrameElement | null): void {
    if (!frame || event.source !== frame.contentWindow) return;
    const height = previewHeightFromMessage(event.data, previewToken.value);
    if (height === null) return;
    const { min, max } = frameLimits();
    const margin = options.frame?.margin ?? 24;
    const next = Math.min(max, Math.max(min, Math.ceil(height) + margin));
    // 输入待处理/刷新期间旧文档仍可能在 1s/3s 兜底回传时收缩；此时保持旧高度，等新文档 ready 后再接受。
    const pending = heightHold || status.value === "updating" || status.value === "loading";
    if (pending && hasDocument.value && next < frameHeight.value) return;
    frameHeight.value = next;
  }

  onBeforeUnmount(() => scheduler.dispose());

  return {
    previewHtml,
    document,
    previewToken,
    rendering,
    status,
    hasDocument,
    previewError,
    frameHeight,
    srcdoc,
    scheduleRender,
    resetHeight,
    holdHeight,
    clearPreview,
    retry,
    invalidateCache,
    onMessage,
  };
}
