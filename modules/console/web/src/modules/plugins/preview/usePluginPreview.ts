// 标签插件共用完整前台预览文档；component-library Fixture 不写真实投票/评分。
// 防抖和序列号在排队时立即使旧预览失效，iframe 高度只接收当前文档 token。

import { computed, onBeforeUnmount, ref } from "vue";
import { previewApi } from "@/api";
import type { PluginPreviewInfo } from "../plugin-types.ts";
import { resolvePreviewHeight, clampPreviewHeight, type MockFixtureConfig } from "./preview-document.ts";
import { RenderGate } from "./render-gate.ts";

export function usePluginPreview(
  getInfo: () => PluginPreviewInfo,
  getOverrides?: () => Record<string, MockFixtureConfig> | undefined,
) {
  const previewHtml = ref("");
  const document = ref("");
  const previewToken = ref("");
  const rendering = ref(false);
  const previewError = ref("");
  // 初始高度 = 当前插件的推荐高度（选中标签先应用推荐高度，再按 iframe 内容回传调整）
  const frameHeight = ref(resolvePreviewHeight(getInfo()));
  const gate = new RenderGate(500);

  // iframe直接消费前台生成文档，既有可选Fixture覆盖交由前台预览宿主实现。
  const srcdoc = computed(() => document.value);
  void getOverrides;

  /** 清空或无效参数时立即销毁旧文档，并使在途响应失效。 */
  function clearPreview(): void {
    gate.invalidate();
    previewHtml.value = "";
    document.value = "";
    previewToken.value = "";
    previewError.value = "";
    rendering.value = false;
  }

  /** 调度一次渲染；immediate=true 立即执行（选中插件/重新渲染按钮）。 */
  function scheduleRender(markdown: string, immediate = false): void {
    clearPreview();
    if (!markdown.trim()) return;
    rendering.value = true;
    const seq = gate.schedule(async () => {
      rendering.value = true;
      previewError.value = "";
      try {
        const { data } = await previewApi.render(markdown, "component-library", { mode: "content" });
        if (!gate.isCurrent(seq)) return; // 旧请求结果丢弃
        previewHtml.value = data.html;
        document.value = data.document;
        previewToken.value = data.preview_token;
      } catch {
        if (gate.isCurrent(seq)) previewError.value = "预览渲染失败";
      } finally {
        if (gate.isCurrent(seq)) rendering.value = false;
      }
    }, immediate);
  }

  /** 选中插件时重置为推荐高度（新 iframe 内容高度回传后再调整）。 */
  function resetHeight(): void {
    frameHeight.value = resolvePreviewHeight(getInfo());
  }

  /** iframe 高度回传处理（event.source 必须是当前预览 iframe）。 */
  function onMessage(event: MessageEvent, frame: HTMLIFrameElement | null): void {
    if (!frame || event.source !== frame.contentWindow) return;
    if (event.data?.type !== "component-preview-resize" || event.data?.token !== previewToken.value) return;
    const h = Number(event.data.height);
    if (Number.isFinite(h) && h > 0) {
      frameHeight.value = clampPreviewHeight(h, resolvePreviewHeight(getInfo()));
    }
  }

  onBeforeUnmount(() => gate.dispose());

  return {
    previewHtml,
    rendering,
    previewError,
    frameHeight,
    srcdoc,
    scheduleRender,
    resetHeight,
    clearPreview,
    onMessage,
  };
}
