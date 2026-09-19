<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import PluginParamForm from "@/modules/plugins/PluginParamForm.vue";
import type { PluginMeta, PluginVariant } from "@/modules/plugins/plugin-types";

// initialValues 故意不给默认值：undefined 代表「插入模式」用 catalog default，
// 有值代表「编辑模式」只用 raw 解析结果，见 PluginParamForm。
const props = withDefaults(
  defineProps<{
    plugin: PluginMeta | null;
    initialValues?: Record<string, unknown>;
    variant?: PluginVariant;
    raw: string;
    loading?: boolean;
    error?: string;
  }>(),
  { loading: false, error: "" },
);

const emit = defineEmits<{
  close: [];
  "update:raw": [value: string];
}>();

const popoverRef = ref<HTMLElement | null>(null);
const sourceMode = ref(!props.plugin);
const draft = ref(props.raw);
const structuredValid = ref(true);
const structuredError = ref("");
const originalRaw = props.raw;
let updateTimer: ReturnType<typeof setTimeout> | undefined;

watch(
  () => props.plugin,
  (plugin) => {
    sourceMode.value = !plugin;
    structuredValid.value = !plugin;
    structuredError.value = "";
  },
  { immediate: true },
);

watch(
  () => props.raw,
  (next) => {
    draft.value = next;
  },
);

function clearUpdateTimer() {
  if (updateTimer !== undefined) {
    clearTimeout(updateTimer);
    updateTimer = undefined;
  }
}

function emitDraft() {
  if (draft.value !== props.raw) emit("update:raw", draft.value);
}

function scheduleStructuredUpdate(value: string) {
  draft.value = value;
  clearUpdateTimer();
  updateTimer = setTimeout(() => {
    updateTimer = undefined;
    emitDraft();
  }, 200);
}

function onStructuredValidation(state: { valid: boolean; error?: string }) {
  structuredValid.value = state.valid;
  structuredError.value = state.error || "";
  if (!state.valid) clearUpdateTimer();
}

function enterSourceMode() {
  clearUpdateTimer();
  emitDraft();
  sourceMode.value = true;
  nextTick(() => popoverRef.value?.querySelector<HTMLTextAreaElement>("textarea")?.focus());
}

function save() {
  clearUpdateTimer();
  if (sourceMode.value || structuredValid.value) emitDraft();
  emit("close");
}

function cancel() {
  clearUpdateTimer();
  if (draft.value !== originalRaw) emit("update:raw", originalRaw);
  emit("close");
}

function closeOnPointerDown(event: PointerEvent) {
  if (popoverRef.value && !popoverRef.value.contains(event.target as Node)) {
    clearUpdateTimer();
    if (sourceMode.value || structuredValid.value) emitDraft();
    emit("close");
  }
}

function closeOnKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    event.preventDefault();
    clearUpdateTimer();
    if (sourceMode.value || structuredValid.value) emitDraft();
    emit("close");
  }
}

onMounted(() => {
  document.addEventListener("pointerdown", closeOnPointerDown, true);
  document.addEventListener("keydown", closeOnKeydown, true);
});

onBeforeUnmount(() => {
  clearUpdateTimer();
  document.removeEventListener("pointerdown", closeOnPointerDown, true);
  document.removeEventListener("keydown", closeOnKeydown, true);
});
</script>

<template>
  <section
    ref="popoverRef"
    class="stellar-property-popover absolute left-1/2 top-8 z-50 flex max-w-[calc(100vw-1.5rem)] -translate-x-1/2 flex-col overflow-hidden rounded-[var(--radius)] border border-border bg-popover text-popover-foreground shadow-xl"
    contenteditable="false"
    role="dialog"
    aria-label="编辑组件属性"
  >
    <header class="flex shrink-0 items-start gap-3 border-b border-border px-4 py-3">
      <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Icon icon="ri:settings-3-line" class="text-lg" />
      </span>
      <div class="min-w-0 flex-1">
        <h3 class="text-sm font-semibold text-foreground">
          {{ plugin && !sourceMode ? `${plugin.title} 属性` : "源码编辑" }}
        </h3>
        <p class="mt-0.5 text-xs leading-relaxed text-text-tertiary">
          {{ loading ? "正在解析当前 shortcode…" : structuredError || error || "修改后实时更新当前组件预览。" }}
        </p>
      </div>
      <button
        type="button"
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
        aria-label="关闭属性编辑"
        @click="save"
      >
        <Icon icon="ri:close-line" class="text-lg" />
      </button>
    </header>

    <div class="stellar-property-scroll min-h-0 flex-1 overflow-y-auto px-4 py-3">
      <div v-if="loading" class="flex items-center gap-2 py-8 text-xs text-text-tertiary">
        <Icon icon="ri:loader-4-line" class="animate-spin text-base" />
        正在读取属性
      </div>
      <!-- 零参数组件（如 links）也算可编辑：没有属性可填，但仍保留源码编辑通道 -->
      <p
        v-else-if="plugin && !sourceMode && !plugin.form?.length"
        class="py-8 text-center text-xs text-text-quaternary"
      >
        该组件没有可编辑属性
      </p>
      <PluginParamForm
        v-else-if="plugin && !sourceMode"
        :plugin="plugin"
        :model-value="draft"
        :initial-values="initialValues"
        :variant="variant"
        @update:model-value="scheduleStructuredUpdate"
        @validation="onStructuredValidation"
      />
      <div v-else class="space-y-2">
        <label class="block text-xs text-text-tertiary" for="stellar-shortcode-source">Shortcode 源码</label>
        <textarea
          id="stellar-shortcode-source"
          v-model="draft"
          rows="7"
          spellcheck="false"
          class="apple-input w-full resize-y font-mono text-xs leading-relaxed"
          @keydown.stop
        ></textarea>
        <p class="text-[11px] leading-relaxed text-text-quaternary">
          当前组件保留原始 Markdown shortcode；保存后不会改变正文数据模型。
        </p>
      </div>
    </div>

    <footer class="flex shrink-0 items-center justify-between gap-2 border-t border-border px-4 py-3">
      <button
        v-if="plugin && !sourceMode"
        type="button"
        class="min-h-9 rounded-full px-3 text-xs text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
        @click="enterSourceMode"
      >
        源码编辑
      </button>
      <span v-else class="text-[11px] text-text-quaternary">Esc 关闭</span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="min-h-9 rounded-full px-3 text-xs text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
          @click="cancel"
        >
          取消
        </button>
        <button
          type="button"
          :disabled="!!plugin && !sourceMode && !structuredValid"
          class="min-h-9 rounded-full bg-primary px-3.5 text-xs font-semibold text-primary-foreground transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          @click="save"
        >
          应用
        </button>
      </div>
    </footer>
  </section>
</template>

<style scoped>
.stellar-property-popover {
  width: min(340px, calc(100vw - 1.5rem));
  max-height: min(72vh, 520px);
}
</style>
