<script setup lang="ts">
import { computed, inject, ref } from "vue";
import PackageRichTextEditor from "@blog/editor/RichTextEditor.vue";
import type { AssistantBridge } from "@blog/editor/assistant-bridge";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { StellarTag } from "@/components/editor/stellar-tag";
import StellarEditingCanvas from "@/components/editor/StellarEditingCanvas.vue";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";

const props = defineProps<{
  modelValue: string;
  storageKey?: string;
  followStorageKey?: boolean;
  pageAppearance?: boolean;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: string): void;
  (event: "save"): void;
  (event: "pending-change", pending: boolean): void;
}>();

const inner = ref<any>(null);
const stellarContext = inject(stellarEditorContextKey, undefined);
const canvasProps = computed(() => ({ markdown: props.modelValue, metadata: stellarContext?.value }));
const activeStates = computed<Record<string, boolean>>(() => inner.value?.activeStates ?? {});

function applyFormat(id: string) {
  inner.value?.applyFormat(id);
}
function insertMarkdown(text: string) {
  inner.value?.insertMarkdown(text);
}
function scrollToHeading(index: number) {
  inner.value?.scrollToHeading(index);
}
function openImageUpload() {
  inner.value?.openImageUpload();
}
function openImagePicker() {
  inner.value?.openImagePicker();
}
function clearDraft() {
  inner.value?.clearDraft();
}
async function flushPendingChanges() {
  await inner.value?.flushPendingChanges();
}
function openSearch() {
  inner.value?.openSearch();
}
/** 写作助手桥接；内层编辑器尚未挂载时为 null。 */
function getAssistantBridge(): AssistantBridge | null {
  return inner.value?.getAssistantBridge?.() ?? null;
}

defineExpose({
  flushPendingChanges,
  applyFormat,
  insertMarkdown,
  scrollToHeading,
  openImageUpload,
  openImagePicker,
  clearDraft,
  openSearch,
  getAssistantBridge,
  activeStates,
});
</script>

<template>
  <PackageRichTextEditor
    ref="inner"
    :model-value="props.modelValue"
    :storage-key="props.storageKey"
    :follow-storage-key="props.followStorageKey"
    :extensions="[StellarTag]"
    :attachment-picker="AttachmentPickerModal"
    :canvas="props.pageAppearance ? StellarEditingCanvas : undefined"
    :canvas-props="canvasProps"
    @update:model-value="emit('update:modelValue', $event)"
    @save="emit('save')"
    @pending-change="emit('pending-change', $event)"
  />
</template>
