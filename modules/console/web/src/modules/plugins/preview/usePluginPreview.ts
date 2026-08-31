// 标签插件预览组合式函数（Spec 63 + Spec 64 暗色单模式）：防抖渲染
// （component-library Fixture 模式）、请求序列号防旧结果覆盖、推荐高度 +
// 内容高度回传。暗色固定，无主题切换。
// PluginLibrary.vue 只负责选择插件/编辑示例/传入预览能力，预览管线全部收敛于此。

import { computed, onBeforeUnmount, ref } from "vue";
import { previewApi } from "@/api";
import type { PluginPreviewInfo } from "../plugin-types.ts";
import { buildPreviewDoc, resolvePreviewHeight, clampPreviewHeight, type MockFixtureConfig } from "./preview-document.ts";
import { previewScriptsFor } from "./resources.ts";
import { RenderGate } from "./render-gate.ts";

export function usePluginPreview(
  getInfo: () => PluginPreviewInfo,
  getOverrides?: () => Record<string, MockFixtureConfig> | undefined,
) {
  const previewHtml = ref("");
  const rendering = ref(false);
  const previewError = ref("");
  // 初始高度 = 当前插件的推荐高度（选中标签先应用推荐高度，再按 iframe 内容回传调整）
  const frameHeight = ref(resolvePreviewHeight(getInfo()));
  const gate = new RenderGate(500);

  // srcdoc：HTML 更新 / 能力脚本变化时自动重建（iframe 整体重载，主题固定暗色）
  const srcdoc = computed(() => {
    if (!previewHtml.value) return "";
    const info = getInfo();
    return buildPreviewDoc({
      bodyHtml: previewHtml.value,
      scripts: previewScriptsFor(info),
      mockFixtures: getOverrides ? getOverrides() : undefined,
    });
  });

  /** 调度一次渲染；immediate=true 立即执行（选中插件/重新渲染按钮）。 */
  function scheduleRender(markdown: string, immediate = false): void {
    const seq = gate.schedule(async () => {
      rendering.value = true;
      previewError.value = "";
      try {
        const { data } = await previewApi.render(markdown, "component-library");
        if (!gate.isCurrent(seq)) return; // 旧请求结果丢弃
        previewHtml.value = data.html;
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
    if (event.data?.type !== "component-preview-resize") return;
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
    onMessage,
  };
}
