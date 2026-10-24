<script setup lang="ts">
import { computed, ref } from "vue";
import PackageRichTextEditor from "@blog/editor/RichTextEditor.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { StellarTag } from "@/components/editor/stellar-tag";

const props = defineProps<{
  modelValue: string;
  storageKey?: string;
  followStorageKey?: boolean;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: string): void;
  (event: "save"): void;
  (event: "pending-change", pending: boolean): void;
}>();

const inner = ref<any>(null);
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

defineExpose({
  flushPendingChanges,
  applyFormat,
  insertMarkdown,
  scrollToHeading,
  openImageUpload,
  openImagePicker,
  clearDraft,
  openSearch,
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
    @update:model-value="emit('update:modelValue', $event)"
    @save="emit('save')"
    @pending-change="emit('pending-change', $event)"
  />
</template>
