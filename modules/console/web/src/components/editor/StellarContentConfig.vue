<script setup lang="ts">
import { ref } from "vue";
import { http } from "@blog/api-client/transport";
import { editorImportApi } from "@/api";
import { Toast, VButton } from "@/components/ui";
import { getApiErrorMessage } from "@/api";

const props = withDefaults(
  defineProps<{
    syncError?: string;
    version: number;
    config: string;
    blog: string;
    profile: string;
    collection?: boolean;
    content?: string;
    summary?: string;
  }>(),
  { collection: false, content: "", summary: "Stellar 2 原文配置" },
);
const emit = defineEmits<{ "update:config": [string]; "update:blog": [string] }>();
const busy = ref(false);
const diagnostic = ref("");

async function validate() {
  busy.value = true;
  diagnostic.value = "";
  try {
    const { data } = await http.post<{ issues: { path: string; code: string }[] }>("/editor/stellar/validate", {
      stellar_version: props.version,
      stellar_config: props.config,
      blog_extensions: props.blog,
      profile: props.profile,
      collection: props.collection,
    });
    diagnostic.value = data.issues.length
      ? data.issues.map((issue) => `${issue.path}: ${issue.code}`).join("\n")
      : "校验通过";
  } catch (error) {
    diagnostic.value = getApiErrorMessage(error);
  } finally {
    busy.value = false;
  }
}

async function exportDocument() {
  busy.value = true;
  try {
    const { data } = await editorImportApi.exportStellar({
      front_matter: props.config,
      blog_extensions: props.blog,
      content_md: props.content,
    });
    const url = URL.createObjectURL(new Blob([data.content], { type: "text/markdown;charset=utf-8" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "stellar-document.md";
    a.click();
    URL.revokeObjectURL(url);
  } catch (error) {
    Toast.error(getApiErrorMessage(error));
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <details class="rounded-[var(--radius)] border border-border bg-card p-4">
    <summary class="cursor-pointer text-sm font-medium">{{ summary }}</summary>
    <div class="mt-4 space-y-3">
      <p v-if="syncError" role="alert" class="text-sm text-destructive">{{ syncError }}</p>
      <p v-if="version !== 2" class="text-sm text-destructive" role="status">
        这条旧内容尚未迁移，仅可预览。请先审查迁移候选；保存不会自动转换旧字段。
      </p>
      <p class="text-xs text-text-tertiary">
        完整 JSON 按原文保存；Stellar 字段与 Blog 扩展分开。合法原文修改会同步基本控件，基本控件修改会同步对应 JSON
        字段；仅编辑的字段被重写。未知字段和归属冲突会明确报错。
      </p>
      <label class="block text-xs text-text-tertiary"
        >Stellar {{ collection ? "集合" : "内容" }}配置
        <textarea
          :value="config"
          @input="emit('update:config', ($event.target as HTMLTextAreaElement).value)"
          rows="9"
          spellcheck="false"
          class="apple-textarea mt-2 w-full font-mono text-xs"
        />
      </label>
      <label class="block text-xs text-text-tertiary"
        >Blog 扩展
        <textarea
          :value="blog"
          @input="emit('update:blog', ($event.target as HTMLTextAreaElement).value)"
          rows="5"
          spellcheck="false"
          class="apple-textarea mt-2 w-full font-mono text-xs"
        />
      </label>
      <div class="flex flex-wrap gap-2">
        <VButton :loading="busy" :disabled="!!syncError" @click="validate">只校验</VButton>
        <VButton
          v-if="!collection && (profile === 'post' || profile === 'page')"
          :disabled="busy || version !== 2 || !!syncError"
          @click="exportDocument"
          >导出 Markdown</VButton
        >
      </div>
      <pre v-if="diagnostic" class="whitespace-pre-wrap break-words text-xs text-text-secondary" role="status">{{
        diagnostic
      }}</pre>
    </div>
  </details>
</template>
