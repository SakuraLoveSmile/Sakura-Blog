<script setup lang="ts">
import Stellar2Tab from "./Stellar2Tab.vue";
import type { StellarSettingsEditor } from "./useStellarSettings";

// 完整配置原文的折叠入口。表单无法使用时（语法错误、读取失败）仍要能打开它修正，所以独立于表单渲染。
defineProps<{ id: string; editor: StellarSettingsEditor; saving?: string; open: boolean }>();
const emit = defineEmits<{ "update:open": [value: boolean] }>();
</script>

<template>
  <details
    :id="id"
    class="settings-raw-editor"
    :open="open"
    @toggle="emit('update:open', ($event.target as HTMLDetailsElement).open)"
  >
    <summary>完整配置原文（高级）</summary>
    <Stellar2Tab :editor="editor" :saving="saving" />
  </details>
</template>

<style scoped>
.settings-raw-editor {
  margin-top: 8px;
  border-top: 1px solid var(--color-border);
  scroll-margin-top: calc(var(--settings-sticky-offset, 72px) + 12px);
}
.settings-raw-editor > summary {
  min-height: 44px;
  align-content: center;
  padding: 8px 0;
  color: var(--color-text-secondary);
  font-size: 14px;
  cursor: pointer;
}
.settings-raw-editor > summary:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
</style>
