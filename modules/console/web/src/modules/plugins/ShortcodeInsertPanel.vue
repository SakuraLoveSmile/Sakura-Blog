<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useId, watch } from "vue";
import { pluginApi } from "@/api";
import { VButton } from "@/components/ui";
import PluginParamForm from "./PluginParamForm.vue";
import { PLUGIN_CATEGORIES, previewInfoFor, type PluginCategory, type PluginMeta } from "./plugin-types";
import { catalogError, catalogStatus, loadPluginCatalog, retryLoadCatalog, runtimePlugins } from "./usePluginCatalog";
import { useShortcodeDraft } from "./useShortcodeDraft";
import { usePluginPreview } from "./preview/usePluginPreview";

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

function selectPlugin(event: Event) {
  selected.value =
    runtimePlugins.value.find((plugin) => plugin.name === (event.target as HTMLSelectElement).value) || null;
  preview.clearPreview();
  preview.resetHeight();
  draft.startInsertion();
}

watch([draft.raw, draft.canApply], ([raw, valid]) => {
  if (valid) preview.scheduleRender(raw);
  else preview.clearPreview();
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
          >预览<span v-if="preview.rendering.value" class="ml-1 text-text-quaternary">渲染中…</span></label
        >
        <div class="overflow-hidden rounded-lg border border-border">
          <p v-if="preview.previewError.value" class="px-3 py-4 text-center text-xs text-state-error">
            {{ preview.previewError.value }}
          </p>
          <iframe
            v-else-if="preview.srcdoc.value"
            ref="frame"
            :srcdoc="preview.srcdoc.value"
            title="标签插件预览"
            class="block w-full border-0"
            :style="{ height: preview.frameHeight.value + 'px' }"
            sandbox="allow-scripts"
          ></iframe>
          <p v-else class="px-3 py-4 text-center text-xs text-text-quaternary">
            {{ preview.rendering.value ? "正在生成预览…" : "填写参数后预览" }}
          </p>
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
