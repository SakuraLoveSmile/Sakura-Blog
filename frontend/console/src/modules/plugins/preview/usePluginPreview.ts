// 标签插件预览组合式函数（Spec 63）：防抖渲染（component-library Fixture 模式）、
// 请求序列号防旧结果覆盖、主题切换、iframe 高度回传。
// PluginLibrary.vue 只负责选择插件/编辑示例/传入预览能力，预览管线全部收敛于此。

import { computed, onBeforeUnmount, ref } from "vue";
import { previewApi } from "@/api";
import type { PluginPreviewInfo } from "../plugins-data.ts";
import { buildPreviewDoc, type PreviewTheme } from "./preview-document.ts";
import { previewScriptsFor } from "./resources.ts";
import { RenderGate } from "./render-gate.ts";

const MIN_HEIGHT = 420;
const MAX_HEIGHT = 900;

export function usePluginPreview(getInfo: () => PluginPreviewInfo) {
  const previewHtml = ref("");
  const rendering = ref(false);
  const previewError = ref("");
  const theme = ref<PreviewTheme>("dark");
  const frameHeight = ref(MIN_HEIGHT);
  const gate = new RenderGate(500);

  // srcdoc：主题切换 / HTML 更新 / 能力脚本变化时自动重建（iframe 整体重载）
  const srcdoc = computed(() => {
    if (!previewHtml.value) return "";
    const info = getInfo();
    return buildPreviewDoc({
      theme: theme.value,
      bodyHtml: previewHtml.value,
      scripts: previewScriptsFor(info),
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

  function toggleTheme(): void {
    theme.value = theme.value === "dark" ? "light" : "dark";
  }

  /** iframe 高度回传处理（event.source 必须是当前预览 iframe）。 */
  function onMessage(event: MessageEvent, frame: HTMLIFrameElement | null): void {
    if (!frame || event.source !== frame.contentWindow) return;
    if (event.data?.type !== "component-preview-resize") return;
    const h = Number(event.data.height);
    if (Number.isFinite(h) && h > 0) {
      frameHeight.value = Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, Math.ceil(h) + 24));
    }
  }

  onBeforeUnmount(() => gate.dispose());

  return {
    previewHtml,
    rendering,
    previewError,
    theme,
    frameHeight,
    srcdoc,
    scheduleRender,
    toggleTheme,
    onMessage,
  };
}
