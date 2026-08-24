<script setup lang="ts">
import { computed, ref } from "vue";
import PackageRichTextEditor from "@blog/editor/RichTextEditor.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { StellarTag } from "@/components/editor/stellar-tag";

const props = defineProps<{
  modelValue: string;
  storageKey?: string;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: string): void;
  (event: "save"): void;
}>();

const inner = ref<any>(null);
const activeStates = computed<Record<string, boolean>>(() => inner.value?.activeStates ?? {});

function applyFormat(id: string) { inner.value?.applyFormat(id); }
function insertMarkdown(text: string) { inner.value?.insertMarkdown(text); }
function scrollToHeading(index: number) { inner.value?.scrollToHeading(index); }
function openImageUpload() { inner.value?.openImageUpload(); }
function openImagePicker() { inner.value?.openImagePicker(); }
function clearDraft() { inner.value?.clearDraft(); }
function openSearch() { inner.value?.openSearch(); }

defineExpose({
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
    :extensions="[StellarTag]"
    :attachment-picker="AttachmentPickerModal"
    @update:model-value="emit('update:modelValue', $event)"
    @save="emit('save')"
  />
</template>
