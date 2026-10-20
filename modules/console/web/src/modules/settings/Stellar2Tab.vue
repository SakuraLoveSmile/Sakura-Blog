<script setup lang="ts">
import { computed } from "vue";
import { VButton, Toast } from "@/components/ui";
import StellarSettingsStatus from "./StellarSettingsStatus.vue";
import type { StellarSettingsEditor } from "./useStellarSettings";

const props = defineProps<{ editor: StellarSettingsEditor; saving?: string }>();
const { draft, loading, busy, loadError, migration, dirty, pendingInput } = props.editor;
const disabled = computed(() => loading.value || Boolean(busy.value) || Boolean(props.saving));

async function submit(action: "save" | "validate") {
  if (disabled.value || pendingInput.value) return;
  if ((await props.editor.submit(action)) && action === "save") Toast.success("Stellar 2 配置已保存，前台已热更新");
}
</script>

<template>
  <div class="stellar2-form space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-text-primary">Stellar 2 完整配置</h2>
        <p id="stellar2-help" class="text-xs text-text-tertiary">
          theme 使用上游配置键名；blog_extensions 保存原生评论等本地功能。直接编辑完整 JSON，保存后前台即时生效。
        </p>
        <p class="text-xs text-text-quaternary">敏感字段显示为 ****；保留占位即可保持原值，显式空值可清除。</p>
      </div>
      <span v-if="dirty" class="text-xs text-state-warning whitespace-nowrap">（有未保存修改）</span>
    </div>

    <StellarSettingsStatus :editor="editor" :saving="saving" />

    <label for="stellar2-document" class="block text-sm text-text-secondary">
      {{ migration ? "配置校验草稿" : "配置文档" }}
    </label>
    <p v-if="pendingInput" class="text-xs text-state-warning">请先修正其他标签中未完成的输入，或重新读取放弃草稿。</p>
    <textarea
      id="stellar2-document"
      v-model="draft"
      spellcheck="false"
      :disabled="loading || pendingInput"
      aria-describedby="stellar2-help"
      class="apple-textarea code-style w-full font-mono text-xs"
      style="min-height: 420px"
    ></textarea>

    <div class="flex flex-wrap gap-2">
      <VButton
        type="primary"
        :disabled="disabled || Boolean(loadError) || Boolean(migration) || pendingInput"
        @click="submit('save')"
      >
        {{ busy === "save" ? "保存中..." : "保存 Stellar 2 配置" }}
      </VButton>
      <VButton type="secondary" :disabled="disabled || pendingInput" @click="submit('validate')">
        {{ busy === "validate" ? "校验中..." : "校验配置" }}
      </VButton>
      <VButton type="secondary" :disabled="disabled" @click="editor.load()">重新读取</VButton>
    </div>
  </div>
</template>

<style scoped>
/* 与现有 BasicSettings 的 section 对齐，外层 VCard body 本身没有 padding。 */
.stellar2-form {
  padding: 24px 28px 28px;
}
@media (max-width: 767px) {
  .stellar2-form {
    padding-left: 16px;
    padding-right: 16px;
  }
}
</style>
