<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { attachmentApi, getApiErrorMessage } from "@/api";
import AttachmentPickerModal from "./AttachmentPickerModal.vue";

let attachmentFieldSequence = 0;

// 图片 URL 字段：预览卡片 + Halo 风格操作菜单；不显示缩略图时保留传统 URL 输入行。
const props = withDefaults(
  defineProps<{
    modelValue?: string;
    placeholder?: string;
    showThumb?: boolean;
    /** modelValue 为空时缩略图回退显示的默认图（如 favicon 默认 /favicon.ico） */
    fallbackSrc?: string;
    /** 上传时传给浏览器文件选择器的 accept 值。 */
    accepts?: string;
  }>(),
  {
    modelValue: "",
    placeholder: "",
    showThumb: false,
    fallbackSrc: "",
    accepts: "image/*",
  },
);

const emit = defineEmits<{
  (e: "update:modelValue", v: string): void;
}>();

const fieldRef = ref<HTMLElement | null>(null);
const linkInputID = `attachment-link-input-${++attachmentFieldSequence}`;
const menuTriggerRef = ref<HTMLButtonElement | null>(null);
const firstMenuItemRef = ref<HTMLButtonElement | null>(null);
const linkInputRef = ref<HTMLInputElement | null>(null);
const uploadInputRef = ref<HTMLInputElement | null>(null);
const pickerOpen = ref(false);
const menuOpen = ref(false);
const linkOpen = ref(false);
const linkDraft = ref("");
const uploadBusy = ref(false);
const uploadError = ref("");
const thumbError = ref(false);

const previewSrc = computed(() => (props.modelValue || "").trim() || props.fallbackSrc);
const hasPreview = computed(() => !!previewSrc.value && !thumbError.value);

// 值变化后重置缩略图错误标记，重新尝试加载。
watch(
  () => props.modelValue,
  () => {
    thumbError.value = false;
    uploadError.value = "";
  },
);

function closeMenu(restoreFocus = false) {
  menuOpen.value = false;
  if (restoreFocus) nextTick(() => menuTriggerRef.value?.focus());
}

function toggleMenu() {
  if (uploadBusy.value) return;
  linkOpen.value = false;
  menuOpen.value = !menuOpen.value;
  if (menuOpen.value) nextTick(() => firstMenuItemRef.value?.focus());
}

function openUpload() {
  closeMenu();
  uploadInputRef.value?.click();
}

function openPicker() {
  closeMenu();
  pickerOpen.value = true;
}

function openLinkEditor() {
  closeMenu();
  linkDraft.value = props.modelValue || "";
  linkOpen.value = true;
  nextTick(() => linkInputRef.value?.focus());
}

function submitLink() {
  const url = linkDraft.value.trim();
  if (!url) return;
  emit("update:modelValue", url);
  linkOpen.value = false;
  nextTick(() => menuTriggerRef.value?.focus());
}

function cancelLink() {
  linkOpen.value = false;
  nextTick(() => menuTriggerRef.value?.focus());
}

function removeValue() {
  emit("update:modelValue", "");
  thumbError.value = false;
  uploadError.value = "";
  closeMenu();
  nextTick(() => menuTriggerRef.value?.focus());
}

async function onUploadChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  // 立即清空，保证再次选择同一个文件也能触发 change。
  input.value = "";
  if (!file) return;

  uploadBusy.value = true;
  uploadError.value = "";
  try {
    const { data } = await attachmentApi.upload(file);
    const url = data?.url;
    if (typeof url !== "string" || !url.trim()) throw new Error("上传未返回附件地址");
    emit("update:modelValue", url.trim());
  } catch (error) {
    uploadError.value = getApiErrorMessage(error, "图片上传失败");
  } finally {
    uploadBusy.value = false;
  }
}

function onSelect(url: string) {
  emit("update:modelValue", url);
  pickerOpen.value = false;
  uploadError.value = "";
  nextTick(() => menuTriggerRef.value?.focus());
}

function onDocumentPointerDown(event: PointerEvent) {
  if (fieldRef.value && !fieldRef.value.contains(event.target as Node)) {
    menuOpen.value = false;
    linkOpen.value = false;
  }
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape") return;
  if (pickerOpen.value) return;
  if (menuOpen.value || linkOpen.value) {
    event.preventDefault();
    menuOpen.value = false;
    linkOpen.value = false;
    nextTick(() => menuTriggerRef.value?.focus());
  }
}

onMounted(() => {
  document.addEventListener("pointerdown", onDocumentPointerDown);
  document.addEventListener("keydown", onDocumentKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  document.removeEventListener("keydown", onDocumentKeydown);
});
</script>

<template>
  <div ref="fieldRef" class="attachment-field" :class="{ 'attachment-field-preview-mode': showThumb }">
    <template v-if="showThumb">
      <button
        v-if="!hasPreview"
        ref="menuTriggerRef"
        type="button"
        class="attachment-preview attachment-preview-empty"
        :class="{ 'attachment-preview-error': previewSrc }"
        :aria-expanded="menuOpen"
        aria-haspopup="menu"
        aria-label="添加图片"
        @click="toggleMenu"
      >
        <Icon :icon="previewSrc ? 'ri:error-warning-line' : 'ri:add-line'" class="attachment-preview-icon" />
        <span>{{ previewSrc ? "图片加载失败" : "添加图片" }}</span>
      </button>
      <div v-else class="attachment-preview" :class="{ 'attachment-preview-loading': uploadBusy }">
        <img :src="previewSrc" class="attachment-preview-image" alt="" @error="thumbError = true" />
        <span v-if="uploadBusy" class="attachment-preview-loading-mask" aria-live="polite">
          <Icon icon="ri:loader-4-line" class="animate-spin" />
          上传中
        </span>
        <button
          ref="menuTriggerRef"
          type="button"
          class="attachment-menu-trigger"
          :disabled="uploadBusy"
          :aria-expanded="menuOpen"
          aria-haspopup="menu"
          aria-label="附件操作"
          title="附件操作"
          @click="toggleMenu"
        >
          <Icon icon="ri:more-2-line" />
        </button>
      </div>
    </template>

    <template v-else>
      <input
        :value="modelValue"
        class="apple-input attachment-input"
        :placeholder="placeholder"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
      <button
        ref="menuTriggerRef"
        type="button"
        class="attachment-menu-trigger attachment-menu-trigger-inline"
        :disabled="uploadBusy"
        :aria-expanded="menuOpen"
        aria-haspopup="menu"
        aria-label="附件操作"
        title="附件操作"
        @click="toggleMenu"
      >
        <Icon icon="ri:more-2-line" />
      </button>
    </template>

    <div
      v-if="menuOpen"
      class="attachment-menu"
      :class="{
        'attachment-menu-preview': showThumb && hasPreview,
        'attachment-menu-empty': showThumb && !hasPreview,
        'attachment-menu-inline': !showThumb,
      }"
      role="menu"
      aria-label="附件操作"
      @keydown.esc.stop.prevent="closeMenu(true)"
    >
      <button ref="firstMenuItemRef" type="button" class="attachment-menu-item" role="menuitem" @click="openUpload">
        <Icon icon="ri:upload-2-line" />
        <span>上传</span>
      </button>
      <button type="button" class="attachment-menu-item" role="menuitem" @click="openPicker">
        <Icon icon="ri:image-line" />
        <span>从附件库选择</span>
      </button>
      <button type="button" class="attachment-menu-item" role="menuitem" @click="openLinkEditor">
        <Icon icon="ri:link" />
        <span>输入链接</span>
      </button>
      <div class="attachment-menu-divider" role="separator"></div>
      <button type="button" class="attachment-menu-item attachment-menu-item-danger" role="menuitem" @click="removeValue">
        <Icon icon="ri:delete-bin-line" />
        <span>移除</span>
      </button>
    </div>

    <div
      v-if="linkOpen"
      class="attachment-link-editor"
      role="dialog"
      aria-label="输入图片链接"
      @keydown.esc.stop.prevent="cancelLink"
    >
      <form @submit.prevent="submitLink">
        <label :for="linkInputID">图片链接</label>
        <input
          :id="linkInputID"
          ref="linkInputRef"
          v-model="linkDraft"
          class="apple-input"
          type="text"
          :placeholder="placeholder || 'https://...'"
          autocomplete="off"
        />
        <div class="attachment-link-actions">
          <button type="button" class="attachment-link-cancel" @click="cancelLink">取消</button>
          <button type="submit" class="attachment-link-submit" :disabled="!linkDraft.trim()">保存</button>
        </div>
      </form>
    </div>

    <input ref="uploadInputRef" class="attachment-upload-input" type="file" :accept="accepts" @change="onUploadChange" />
    <p v-if="uploadError" class="attachment-error" role="alert">{{ uploadError }}</p>

    <AttachmentPickerModal v-if="pickerOpen" :open="pickerOpen" @select="onSelect" @close="pickerOpen = false" />
  </div>
</template>

<style scoped>
/* 组件自包含：预览卡片、锚定菜单和链接编辑器只消费 Console 语义 token。 */
.attachment-field {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.attachment-field-preview-mode {
  display: inline-flex;
  align-items: flex-start;
  flex: 0 0 auto;
}

.attachment-input {
  flex: 1;
  min-width: 0;
}

.attachment-preview {
  position: relative;
  display: flex;
  width: 112px;
  height: 112px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-secondary);
  color: var(--color-text-quaternary);
}

.attachment-preview-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.attachment-preview-empty {
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border-style: dashed;
  color: var(--color-text-tertiary);
  font-size: 11px;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.attachment-preview-empty:hover,
.attachment-preview-empty:focus-visible {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-secondary));
  color: var(--color-primary);
}

.attachment-preview-error {
  border-color: color-mix(in srgb, var(--color-state-error) 45%, var(--color-border));
  color: var(--color-state-error);
}

.attachment-preview-icon {
  font-size: 24px;
}

.attachment-preview-loading-mask {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  background: color-mix(in srgb, var(--color-background) 58%, transparent);
  color: var(--color-foreground);
  font-size: 11px;
}

.attachment-menu-trigger {
  position: absolute;
  top: 6px;
  right: 6px;
  display: inline-flex;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-primary) 85%, transparent);
  color: #ffffff;
  font-size: 14px;
  cursor: pointer;
  opacity: 0;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
  -webkit-backdrop-filter: blur(8px);
  backdrop-filter: blur(8px);
  transition:
    opacity var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}

.attachment-preview:hover .attachment-menu-trigger,
.attachment-menu-trigger:focus-visible,
.attachment-menu-trigger[aria-expanded="true"] {
  opacity: 1;
}

.attachment-menu-trigger:hover:not(:disabled),
.attachment-menu-trigger[aria-expanded="true"] {
  background: var(--color-primary);
  color: #ffffff;
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-primary) 45%, transparent);
}

.attachment-menu-trigger:disabled {
  cursor: wait;
  opacity: 0.6;
}

.attachment-menu-trigger-inline {
  position: relative;
  top: auto;
  right: auto;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 24%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  opacity: 1;
  box-shadow: none;
}

.attachment-menu-trigger-inline:hover:not(:disabled),
.attachment-menu-trigger-inline[aria-expanded="true"] {
  border-color: var(--color-primary);
  background: var(--color-primary);
  color: #ffffff;
}

.attachment-menu {
  position: absolute;
  z-index: 50;
  width: min(180px, calc(100vw - 32px));
  padding: 6px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-popover);
  color: var(--color-popover-foreground);
  box-shadow: var(--shadow-xl);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.attachment-menu::before {
  position: absolute;
  top: -6px;
  width: 11px;
  height: 11px;
  border-top: 1px solid var(--color-border);
  border-left: 1px solid var(--color-border);
  background: var(--color-popover);
  content: "";
  transform: rotate(45deg);
}

/* 缩略图已有图片：气泡锚定在右上角 22px 按钮正下方，箭头对齐按钮中心 */
.attachment-menu-preview {
  top: 34px;
  right: 0;
  left: auto;
}

.attachment-menu-preview::before {
  right: 11px;
  left: auto;
}

/* 缩略图空状态：气泡在 112px 卡片正下方 */
.attachment-menu-empty {
  top: calc(100% + 8px);
  right: auto;
  left: 0;
}

.attachment-menu-empty::before {
  right: auto;
  left: 24px;
}

/* 单行输入框模式：气泡在右侧操作按钮下方 */
.attachment-menu-inline {
  top: calc(100% + 6px);
  right: 0;
  left: auto;
}

.attachment-menu-inline::before {
  right: 12px;
  left: auto;
}

.attachment-menu-item {
  display: flex;
  width: 100%;
  min-height: 36px;
  align-items: center;
  gap: 10px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.attachment-menu-item:hover,
.attachment-menu-item:focus-visible {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.attachment-menu-item-danger {
  color: var(--color-state-error);
}

.attachment-menu-item-danger:hover,
.attachment-menu-item-danger:focus-visible {
  background: color-mix(in srgb, var(--color-state-error) 12%, transparent);
  color: var(--color-state-error);
}

.attachment-menu-divider {
  height: 1px;
  margin: 8px 4px;
  background: var(--color-border);
}

.attachment-link-editor {
  position: absolute;
  top: calc(100% + 10px);
  left: 0;
  z-index: 45;
  width: min(340px, calc(100vw - 32px));
  padding: 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-popover);
  color: var(--color-popover-foreground);
  box-shadow: var(--shadow-xl);
}

.attachment-link-editor label {
  display: block;
  margin-bottom: 8px;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.attachment-link-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.attachment-link-cancel,
.attachment-link-submit {
  min-height: 40px;
  padding: 0 14px;
  border: 0;
  border-radius: var(--radius-full);
  font-size: 13px;
  cursor: pointer;
}

.attachment-link-cancel {
  background: transparent;
  color: var(--color-text-tertiary);
}

.attachment-link-cancel:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.attachment-link-submit {
  background: var(--color-primary);
  color: var(--color-primary-foreground);
  font-weight: 600;
}

.attachment-link-submit:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.attachment-upload-input {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
}

.attachment-error {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  max-width: min(340px, calc(100vw - 32px));
  margin: 0;
  color: var(--color-state-error);
  font-size: 11px;
  line-height: 1.4;
}

@media (max-width: 640px) {
  .attachment-preview {
    width: 88px;
    height: 88px;
  }

  .attachment-menu-trigger {
    top: 6px;
    right: 6px;
  }
}
</style>
