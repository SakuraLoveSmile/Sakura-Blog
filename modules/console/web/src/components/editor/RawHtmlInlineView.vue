<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { scanHtmlTags } from "./raw-html-syntax";

// 行内外部 HTML 的 chip：显示标签名，选中后可编辑源码或删除。
// 不在编辑器 DOM 内直接执行外部标记，真实效果由块级预览与发布页承担。
const props = defineProps(nodeViewProps);
const raw = computed(() => (props.node.attrs.raw as string) || "");
const tagLabel = computed(() => scanHtmlTags(raw.value).find((tag) => !tag.closing)?.name || "html");

const editing = ref(false);
const draft = ref("");
const sourceRef = ref<HTMLTextAreaElement | null>(null);

function startEdit() {
  draft.value = raw.value;
  editing.value = true;
  nextTick(() => sourceRef.value?.focus());
}
function cancelEdit() {
  editing.value = false;
}
function saveEdit() {
  const next = draft.value;
  editing.value = false;
  if (next !== raw.value) props.updateAttributes({ raw: next });
}
</script>

<template>
  <NodeViewWrapper as="span" class="raw-html-inline-node relative inline-flex align-baseline">
    <span
      v-if="!editing"
      :title="raw"
      :class="[
        'inline-flex max-w-40 items-center gap-1 truncate rounded border px-1.5 py-0.5 align-baseline font-mono text-[11px] transition-colors',
        selected
          ? 'border-primary bg-primary/15 text-primary'
          : 'border-border bg-secondary text-text-secondary hover:border-primary/50',
      ]"
    >
      <Icon icon="ri:html5-line" class="shrink-0 text-[12px]" />
      <span class="truncate">&lt;{{ tagLabel }}&gt;</span>
    </span>

    <span
      v-if="selected && !editing"
      contenteditable="false"
      class="absolute -top-9 left-0 z-50 flex items-center gap-0.5 rounded-[10px] border border-border bg-card px-1 py-0.5 shadow-lg"
    >
      <button
        type="button"
        aria-label="编辑行内 HTML 源码"
        class="rounded p-1 text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
        @mousedown.prevent
        @click="startEdit"
      >
        <Icon icon="ri:file-edit-line" class="text-sm" />
      </button>
      <button
        type="button"
        aria-label="删除行内 HTML"
        class="rounded p-1 text-state-error transition-colors hover:bg-state-error/10"
        @mousedown.prevent
        @click="deleteNode()"
      >
        <Icon icon="ri:delete-bin-line" class="text-sm" />
      </button>
    </span>

    <span
      v-if="editing"
      contenteditable="false"
      class="absolute left-0 top-full z-50 mt-1 block w-72 overflow-hidden rounded-lg border border-border bg-card shadow-xl"
    >
      <textarea
        ref="sourceRef"
        v-model="draft"
        spellcheck="false"
        aria-label="行内 HTML 源码"
        class="block h-24 w-full resize-y bg-transparent px-2.5 py-2 font-mono text-[12px] leading-relaxed text-foreground outline-none"
        @keydown.esc.stop="cancelEdit"
        @keydown.meta.enter.stop="saveEdit"
        @keydown.ctrl.enter.stop="saveEdit"
      ></textarea>
      <span class="flex items-center justify-end gap-2 border-t border-border px-2.5 py-1.5">
        <button
          type="button"
          class="rounded px-2 py-0.5 text-[11px] text-text-secondary transition-colors hover:bg-secondary"
          @mousedown.prevent
          @click="cancelEdit"
        >
          取消
        </button>
        <button
          type="button"
          class="rounded bg-primary px-2 py-0.5 text-[11px] font-medium text-primary-foreground transition-colors hover:bg-primary-hover"
          @mousedown.prevent
          @click="saveEdit"
        >
          保存
        </button>
      </span>
    </span>
  </NodeViewWrapper>
</template>