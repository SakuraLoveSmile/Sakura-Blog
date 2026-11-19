<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VModal } from "@/components/ui";
import { stellarEditorContextKey } from "./stellar-context";
import { usePluginPreview } from "@/modules/plugins/preview/usePluginPreview";
import { PreviewCache, sharedPreviewQueue } from "@/modules/plugins/preview/render-gate";
import { withPreviewContentWidth } from "@/modules/plugins/preview/preview-document";

// 插入外部 HTML：左侧写源码，右侧实时预览（与发布同一渲染管线）。
// 作者即可信，源码原样保存，不做净化。
const props = withDefaults(defineProps<{ visible: boolean; raw?: string }>(), { raw: "" });
const emit = defineEmits<{
  (event: "update:visible", visible: boolean): void;
  (event: "submit", raw: string): void;
}>();

const stellarContext = inject(stellarEditorContextKey, undefined);
const draft = ref("");
const sourceRef = ref<HTMLTextAreaElement | null>(null);
const paneRef = ref<HTMLElement | null>(null);
const frameRef = ref<HTMLIFrameElement | null>(null);
const contentWidth = ref(0);

const preview = usePluginPreview(() => ({ capability: "static", height: 120 }), undefined, {
  delayMs: 300,
  queue: sharedPreviewQueue,
  cache: new PreviewCache(20),
  frame: { minHeight: 120, maxHeight: 4096, margin: 0 },
});
const srcdoc = computed(() => withPreviewContentWidth(preview.srcdoc.value, contentWidth.value));
const hasPreview = computed(() => Boolean(preview.srcdoc.value));
const charCount = computed(() => draft.value.length);
const canSubmit = computed(() => draft.value.trim().length > 0);

let paneObserver: ResizeObserver | undefined;
function measurePane() {
  contentWidth.value = paneRef.value?.clientWidth || 0;
}

// 高度回传沿用共享 composable 的 source/token 校验；监听 iframe 实际所属窗口。
let messageWindow: Window | null = null;
function bindFrameMessages() {
  const ownerWindow = frameRef.value?.ownerDocument.defaultView || null;
  if (messageWindow === ownerWindow) return;
  messageWindow?.removeEventListener("message", onFrameMessage);
  messageWindow = ownerWindow;
  messageWindow?.addEventListener("message", onFrameMessage);
}
function onFrameMessage(event: MessageEvent) {
  preview.onMessage(event, frameRef.value);
}
watch(frameRef, bindFrameMessages, { flush: "post" });

function schedule() {
  if (!draft.value.trim()) {
    preview.clearPreview();
    return;
  }
  preview.holdHeight();
  preview.scheduleRender(draft.value, {
    profile: null,
    metadata: stellarContext?.value as Record<string, unknown> | undefined,
  });
}

watch(
  () => props.visible,
  (open) => {
    if (open) {
      draft.value = props.raw || "";
      nextTick(() => {
        measurePane();
        if (paneRef.value) {
          paneObserver?.disconnect();
          paneObserver = new ResizeObserver(measurePane);
          paneObserver.observe(paneRef.value);
        }
        sourceRef.value?.focus();
        schedule();
      });
      return;
    }
    paneObserver?.disconnect();
    paneObserver = undefined;
    preview.clearPreview();
  },
);

watch(draft, () => {
  if (props.visible) schedule();
});

onBeforeUnmount(() => {
  paneObserver?.disconnect();
  messageWindow?.removeEventListener("message", onFrameMessage);
  messageWindow = null;
});

function close() {
  emit("update:visible", false);
}
function submit() {
  if (!canSubmit.value) return;
  emit("submit", draft.value);
  close();
}

// 起始片段：自包含、离线可渲染，点一下即可看到真实效果，再按需改源码。
const starters = [
  {
    label: "渐变卡片",
    html: '<div style="padding:20px 22px;border-radius:14px;background:linear-gradient(135deg,#5e5ce6,#0a84ff);color:#fff;font-family:system-ui,sans-serif">\n  <div style="font-size:17px;font-weight:600;letter-spacing:.01em">外部 HTML 卡片</div>\n  <p style="margin:8px 0 0;font-size:14px;line-height:1.6;opacity:.88">这段 markup 原样保存并渲染，内联样式与结构都不会被改写。</p>\n</div>',
  },
  {
    label: "样式表格",
    html: '<table style="width:100%;border-collapse:collapse;font-family:system-ui,sans-serif;font-size:14px">\n  <thead>\n    <tr style="background:#5e5ce6;color:#fff">\n      <th style="padding:10px 12px;text-align:left">项目</th>\n      <th style="padding:10px 12px;text-align:right">数值</th>\n    </tr>\n  </thead>\n  <tbody>\n    <tr><td style="padding:9px 12px;border-bottom:1px solid #e5e5ea">渲染保真</td><td style="padding:9px 12px;border-bottom:1px solid #e5e5ea;text-align:right">100%</td></tr>\n    <tr><td style="padding:9px 12px">往返无损</td><td style="padding:9px 12px;text-align:right">逐字节</td></tr>\n  </tbody>\n</table>',
  },
  {
    label: "折叠详情",
    html: '<details style="padding:14px 16px;border:1px solid #d1d1d6;border-radius:12px;font-family:system-ui,sans-serif">\n  <summary style="cursor:pointer;font-size:15px;font-weight:600;color:#5e5ce6">点开看细节</summary>\n  <p style="margin:10px 0 0;font-size:14px;line-height:1.6;color:#3a3a3c">原生 details/summary 交互在发布后依然可用，脚本与事件同样原样保留。</p>\n</details>',
  },
];
function useStarter(html: string) {
  draft.value = html;
  nextTick(() => sourceRef.value?.focus());
}
</script>

<template>
  <VModal :visible="props.visible" title="插入外部 HTML" :width="960" @update:visible="emit('update:visible', $event)">
    <div class="grid gap-4 lg:grid-cols-2">
      <div class="flex min-w-0 flex-col gap-1.5">
        <div class="flex items-center justify-between gap-2">
          <label for="raw-html-source" class="flex items-center gap-1.5 text-xs font-medium text-text-secondary">
            <Icon icon="ri:html5-line" class="text-sm text-primary" />
            HTML 源码
          </label>
          <span class="text-[10px] tabular-nums text-text-quaternary">{{ charCount }} 字符</span>
        </div>
        <div class="flex flex-wrap items-center gap-1.5">
          <span class="text-[10px] text-text-quaternary">起始片段</span>
          <button
            v-for="starter in starters"
            :key="starter.label"
            type="button"
            class="rounded-full border border-border px-2.5 py-0.5 text-[11px] text-text-secondary transition-colors hover:border-primary/60 hover:bg-primary/10 hover:text-primary"
            @click="useStarter(starter.html)"
          >
            {{ starter.label }}
          </button>
        </div>
        <textarea
          id="raw-html-source"
          ref="sourceRef"
          v-model="draft"
          spellcheck="false"
          autocapitalize="off"
          autocomplete="off"
          placeholder="粘贴外部 HTML，例如第三方卡片、嵌入播放器或自定义排版片段…"
          class="h-64 w-full resize-y rounded-lg border border-border bg-secondary/40 px-3 py-2 font-mono text-[12px] leading-relaxed text-foreground outline-none transition-colors placeholder:text-text-quaternary focus:border-primary lg:h-80"
          @keydown.meta.enter.stop="submit"
          @keydown.ctrl.enter.stop="submit"
        ></textarea>
        <p class="text-[11px] leading-relaxed text-text-tertiary">
          源码原样保存并渲染，<code class="font-mono">&lt;script&gt;</code>、<code class="font-mono">&lt;iframe&gt;</code>
          与内联样式都会保留。给链接加上 <code class="font-mono">class</code> 或
          <code class="font-mono">role</code> 可跳过站点的链接图标增强。
        </p>
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <div class="flex items-center justify-between gap-2">
          <span class="flex items-center gap-1.5 text-xs font-medium text-text-secondary">
            <Icon icon="ri:eye-line" class="text-sm" />
            实时预览
          </span>
          <span v-if="preview.status.value === 'loading'" class="text-[10px] text-text-quaternary">渲染中…</span>
          <span v-else-if="preview.status.value === 'updating'" class="text-[10px] text-text-quaternary">更新中…</span>
          <button
            v-else-if="preview.status.value === 'error'"
            type="button"
            class="text-[10px] text-primary"
            @click="preview.retry()"
          >
            重试
          </button>
        </div>
        <div
          ref="paneRef"
          class="relative h-64 overflow-auto rounded-lg border border-border bg-card lg:h-80"
        >
          <iframe
            v-if="hasPreview"
            ref="frameRef"
            :srcdoc="srcdoc"
            :style="{ height: preview.frameHeight.value + 'px' }"
            class="block w-full border-0 bg-transparent transition-[height] duration-150"
            sandbox="allow-scripts"
            title="外部 HTML 预览"
            @load="bindFrameMessages"
          ></iframe>
          <p
            v-else-if="preview.status.value === 'error'"
            class="px-3 py-8 text-center text-xs text-state-error"
          >
            {{ preview.previewError.value }}
          </p>
          <p v-else class="px-3 py-8 text-center text-xs text-text-quaternary">
            {{ draft.trim() ? "渲染中…" : "在左侧写入 HTML 后即可看到渲染效果" }}
          </p>
        </div>
        <p class="text-[11px] leading-relaxed text-text-tertiary">
          预览与发布使用同一渲染管线，在隔离沙箱中运行；发布后脚本在页面内直接执行。
        </p>
      </div>
    </div>

    <template #footer>
      <VButton @click="close">取消</VButton>
      <VButton type="primary" :disabled="!canSubmit" @click="submit">插入</VButton>
    </template>
  </VModal>
</template>