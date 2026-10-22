<script setup lang="ts">
import type { StellarMarkdownImportResponse } from "@/api";
import { VButton, VModal } from "@/components/ui";

defineProps<{ data: StellarMarkdownImportResponse | null }>();
const emit = defineEmits<{ confirm: []; close: [] }>();
</script>

<template>
  <VModal :visible="!!data" :title="data?.legacy_converted ? '审查 1.x 转换预览' : '审查 Markdown 导入预览'" :width="720" @update:visible="emit('close')">
    <div v-if="data" class="space-y-4 text-sm text-text-secondary">
      <p>确认后将替换当前正文与内容配置。导入不会自动保存或发布，保存草稿后可继续检查。</p>
      <ul v-if="data.warnings?.length" class="list-disc space-y-1 pl-5 text-xs text-state-warning">
        <li v-for="warning in data.warnings" :key="warning">{{ warning }}</li>
      </ul>
      <p v-if="data.slug_suggestion" class="break-all text-xs">新建时建议使用的 Slug：{{ data.slug_suggestion }}</p>
      <p v-if="data.stellar_tags.length" class="text-xs">识别的 Stellar 标签：{{ data.stellar_tags.join('、') }}</p>
      <details>
        <summary class="cursor-pointer">Stellar 配置与 Blog 扩展</summary>
        <pre class="mt-2 overflow-x-auto whitespace-pre-wrap break-all rounded-lg bg-secondary p-3 text-xs">{{ data.front_matter }}
{{ data.blog_extensions }}</pre>
      </details>
      <div>
        <h4 class="mb-2 font-medium text-text-primary">正文预览</h4>
        <pre class="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-secondary p-3 font-mono text-xs">{{ data.content_md }}</pre>
      </div>
    </div>
    <template #footer>
      <VButton @click="emit('close')">取消</VButton>
      <VButton type="primary" @click="emit('confirm')">确认回填</VButton>
    </template>
  </VModal>
</template>
