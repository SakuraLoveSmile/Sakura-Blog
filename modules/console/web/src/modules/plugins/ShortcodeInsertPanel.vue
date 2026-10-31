<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useId, watch } from "vue";
import { pluginApi } from "@/api";
import { VButton } from "@/components/ui";
import PluginParamForm from "./PluginParamForm.vue";
import { PLUGIN_CATEGORIES, previewInfoFor, type PluginCategory, type PluginMeta } from "./plugin-types";
import { catalogError, catalogStatus, loadPluginCatalog, retryLoadCatalog, runtimePlugins } from "./usePluginCatalog";
import { useShortcodeDraft } from "./useShortcodeDraft";
import { usePluginPreview } from "./preview/usePluginPreview";
import { previewInvalidNoticeVisible } from "./preview/preview-document";

const emit = defineEmits<{ insert: [code: string] }>();
const sourceID = useId();
const category = ref<PluginCategory | "全部">("全部");
const selected = ref<PluginMeta | null>(null);
const plugins = computed(() =>
  category.value === "全部" ? runtimePlugins.value : runtimePlugins.value.filter((p) => p.category === category.value),
);
const draft = useShortcodeDraft(
  () => selected.value,
  async (raw) => (await pluginApi.parseShortcode(raw)).data,
);
const preview = usePluginPreview(() => (selected.value ? previewInfoFor(selected.value) : { capability: "static" }));
const frame = ref<HTMLIFrameElement | null>(null);
// 参数无效/校验中时保留旧预览，但必须说明它没有更新。
const stalePreview = computed(() =>
  previewInvalidNoticeVisible(draft.canApply.value, draft.parsing.value, Boolean(preview.srcdoc.value)),
);

function selectPlugin(event: Event) {
  selected.value =
    runtimePlugins.value.find((plugin) => plugin.name === (event.target as HTMLSelectElement).value) || null;
  preview.clearPreview();
  preview.resetHeight();
  draft.startInsertion();
}

// 输入过程中保留旧预览：raw 为空才清空；参数解析完成（canApply）后再调度渲染。
watch(draft.raw, (raw) => {
  if (!raw.trim()) {
    preview.clearPreview();
    return;
  }
  // 输入已变化：新文档 ready 前冻结高度，避免旧文档兜底回传先收缩。
  preview.holdHeight();
  if (draft.canApply.value) preview.scheduleRender(raw);
});
watch(draft.canApply, (valid) => {
  if (valid) preview.scheduleRender(draft.raw.value);
});
function insert() {
  if (draft.canApply.value) emit("insert", "\n" + draft.raw.value + "\n");
}
function onMessage(event: MessageEvent) {
  preview.onMessage(event, frame.value);
}
onMounted(() => {
  void loadPluginCatalog();
  window.addEventListener("message", onMessage);
});
onBeforeUnmount(() => window.removeEventListener("message", onMessage));
</script>

<template>
  <div class="min-w-0 space-y-3" data-shortcode-insert-panel>
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="item in ['全部', ...PLUGIN_CATEGORIES] as const"
        :key="item"
        type="button"
        :class="[
          'inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors',
          category === item ? 'bg-primary/15 text-primary' : 'bg-secondary text-text-secondary hover:bg-accent',
        ]"
        @click="category = item"
      >
        {{ item }}
      </button>
    </div>
    <p v-if="catalogStatus === 'loading'" class="text-xs text-text-quaternary">正在加载标签目录…</p>
    <div v-else-if="catalogError" class="space-y-2 text-xs text-state-error">
      <p>{{ catalogError }}</p>
      <button type="button" class="text-primary" @click="retryLoadCatalog">重新加载</button>
    </div>
    <select :value="selected?.name || ''" class="apple-select" aria-label="选择标签插件" @change="selectPlugin">
      <option value="" disabled>选择插件…</option>
      <option v-for="plugin in plugins" :key="plugin.name" :value="plugin.name">{{ plugin.title }}</option>
    </select>
    <div v-if="selected" class="min-w-0 space-y-3">
      <PluginParamForm
        v-if="draft.formPlugin.value"
        :key="selected.name"
        :plugin="draft.formPlugin.value"
        :model-value="draft.raw.value"
        :initial-values="draft.initialValues.value"
        :present="draft.present.value"
        :variant="draft.variant.value"
        @update:model-value="draft.updateForm"
        @validation="draft.updateValidation"
      />
      <div>
        <label class="mb-1.5 block text-[12.5px] font-medium text-text-secondary" :for="sourceID">Shortcode 源码</label>
        <textarea
          :id="sourceID"
          :value="draft.raw.value"
          rows="4"
          spellcheck="false"
          class="apple-textarea resize-y font-mono text-xs leading-relaxed"
          @input="draft.updateSource(($event.target as HTMLTextAreaElement).value)"
        ></textarea>
        <p v-if="draft.parsing.value" class="mt-1 text-xs text-text-quaternary">正在同步参数…</p>
        <p v-else-if="draft.sourceReason.value" class="mt-1 text-xs text-text-quaternary">
          {{ draft.sourceReason.value }}
        </p>
      </div>
      <div>
        <label class="mb-1.5 block text-[12.5px] font-medium text-text-secondary"
          >预览<span v-if="preview.rendering.value" class="ml-1 text-text-quaternary">{{
            preview.status.value === "updating" ? "更新中…" : "渲染中…"
          }}</span></label
        >
        <div class="relative overflow-hidden rounded-lg border border-border">
          <p
            v-if="!preview.srcdoc.value && preview.status.value === 'error'"
            class="px-3 py-4 text-center text-xs text-state-error"
          >
            {{ preview.previewError.value }}
            <button type="button" class="ml-1 text-primary" @click="preview.retry()">重试</button>
          </p>
          <p
            v-else-if="!preview.srcdoc.value"
            class="px-3 py-4 text-center text-xs text-text-quaternary"
          >
            {{ preview.status.value === "loading" ? "正在生成预览…" : "填写参数后预览" }}
          </p>
          <iframe
            v-if="preview.srcdoc.value"
            ref="frame"
            :srcdoc="preview.srcdoc.value"
            title="标签插件预览"
            class="block w-full border-0 transition-[height] duration-150"
            :style="{ height: preview.frameHeight.value + 'px' }"
            sandbox="allow-scripts"
          ></iframe>
          <!-- 渲染期间保留旧 iframe，只叠加轻量更新状态 -->
          <p
            v-if="preview.srcdoc.value && preview.status.value === 'updating' && !stalePreview"
            class="pointer-events-none absolute right-2 top-2 rounded bg-card/90 px-1.5 py-0.5 text-[10px] text-text-quaternary"
          >
            更新中…
          </p>
          <p
            v-if="stalePreview"
            class="pointer-events-none absolute right-2 top-2 rounded bg-card/90 px-1.5 py-0.5 text-[10px] text-text-quaternary"
          >
            参数无效，预览未更新
          </p>
          <div
            v-if="preview.srcdoc.value && preview.status.value === 'error'"
            class="absolute inset-x-0 bottom-0 flex items-center justify-between gap-2 border-t border-state-error/30 bg-card/95 px-2 py-1 text-[11px] text-state-error"
          >
            <span>预览更新失败，仍显示上一次结果</span>
            <button type="button" class="text-primary" @click="preview.retry()">重试</button>
          </div>
        </div>
      </div>
      <p
        v-if="draft.formError.value && draft.structured.value && !draft.parsing.value"
        class="text-xs text-state-error"
      >
        {{ draft.formError.value }}
      </p>
      <VButton type="secondary" block :disabled="!draft.canApply.value" @click="insert">插入到正文</VButton>
    </div>
    <p v-else class="text-xs text-text-quaternary">选择插件后可编辑参数并插入正文。</p>
  </div>
</template>
