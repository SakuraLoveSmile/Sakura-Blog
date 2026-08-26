<script setup lang="ts">
import { ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import AttachmentPickerModal from "./AttachmentPickerModal.vue";

// 附件 URL 字段：手填 input + 可选缩略图 + 「从附件库选择」按钮。
// 内部 v-if 懒挂载单个 AttachmentPickerModal 实例（open 时创建，关闭即销毁）。
const props = defineProps<{
  modelValue?: string;
  placeholder?: string;
  showThumb?: boolean;
  /** modelValue 为空时缩略图回退显示的默认图（如 favicon 默认 /favicon.ico） */
  fallbackSrc?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: string): void;
}>();

const pickerOpen = ref(false);
const thumbError = ref(false);

// 值变化后重置缩略图错误标记，重新尝试加载
watch(
  () => props.modelValue,
  () => {
    thumbError.value = false;
  },
);

function onSelect(url: string) {
  emit("update:modelValue", url);
  pickerOpen.value = false;
}
</script>

<template>
  <div class="attachment-field">
    <input
      :value="modelValue"
      class="apple-input attachment-input"
      :placeholder="placeholder"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <img
      v-if="showThumb && (modelValue || fallbackSrc) && !thumbError"
      :src="modelValue || fallbackSrc"
      class="attachment-thumb"
      alt=""
      @error="thumbError = true"
    />
    <button
      type="button"
      class="attachment-btn"
      title="从附件库选择图片"
      aria-label="从附件库选择图片"
      @click="pickerOpen = true"
    >
      <Icon icon="ri:image-upload-line" />
    </button>
    <AttachmentPickerModal v-if="pickerOpen" :open="pickerOpen" @select="onSelect" @close="pickerOpen = false" />
  </div>
</template>

<style scoped>
/* 组件自包含：不依赖外部 scoped 类；视觉定义复制自 field-inline / icon-btn 少量样式 */
.attachment-field {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}
.attachment-input {
  flex: 1;
  min-width: 0;
}
.attachment-thumb {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  object-fit: cover;
  flex-shrink: 0;
  border: 1px solid var(--color-border);
  background: var(--color-secondary);
}
.attachment-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  border: none;
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-standard);
}
.attachment-btn:hover {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
}
</style>
