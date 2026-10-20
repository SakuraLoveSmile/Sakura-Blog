<script setup lang="ts">
import { VButton } from "@/components/ui";
import type { StellarSettingsEditor } from "./useStellarSettings";

const props = defineProps<{ editor: StellarSettingsEditor; saving?: string }>();
const { loading, busy, migration, loadError, issues, validationMessage, showValidation, valid, pendingInput } =
  props.editor;
</script>

<template>
  <div v-if="loading" class="text-xs text-text-tertiary" role="status">正在读取 Stellar 2 配置...</div>
  <div v-else-if="migration" class="space-y-2 text-sm text-state-warning" role="status">
    <p>当前配置尚未迁移。可以编辑草稿并校验；完成迁移预览与确认后才能保存。</p>
    <p class="text-xs break-words">位置：{{ migration.source }}.{{ migration.path }}</p>
    <p class="text-xs break-words">现有配置段：{{ migration.sections.join("、") || "空" }}</p>
    <VButton type="secondary" :disabled="Boolean(busy) || Boolean(saving)" @click="editor.load()">重新读取</VButton>
  </div>
  <div v-else-if="loadError" class="space-y-2" role="alert">
    <p class="text-sm text-state-error break-words">{{ loadError }}</p>
    <VButton type="secondary" :disabled="Boolean(busy) || Boolean(saving)" @click="editor.load()">重新读取</VButton>
  </div>

  <p v-if="pendingInput" class="text-sm text-state-warning" role="status">部分设置还有未完成的输入，请修正后再保存。</p>
  <div v-else-if="showValidation" class="space-y-2" role="status" aria-live="polite">
    <p class="text-sm break-words" :class="valid ? 'text-state-success' : 'text-state-error'">
      {{ validationMessage }}
    </p>
    <ul v-if="issues.length" class="space-y-1 text-xs text-text-secondary">
      <li v-for="(issue, index) in issues" :key="index" class="break-words">
        <code>{{ issue.source }}.{{ issue.path }}</code
        >：{{ issue.code }}
        <span v-if="issue.expected">（需要 {{ issue.expected }}）</span>
      </li>
    </ul>
  </div>
</template>
