<script setup lang="ts">
import { ref, onMounted } from "vue";
import { notesApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import { Icon } from "@iconify/vue";

// 笔记页编辑器：content/notes.md 原文读写，渲染于前台 /notes 页上半部分。
const content = ref("");
const loading = ref(true);
const saving = ref(false);
const message = ref("");
const isError = ref(false);

async function load() {
  loading.value = true;
  try {
    const { data } = await notesApi.get();
    content.value = data.content || "";
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  message.value = "";
  isError.value = false;
  try {
    await notesApi.update(content.value);
    message.value = "已保存，前台 /notes 已生效";
  } catch (e: any) {
    isError.value = true;
    message.value = e?.response?.data?.error || e?.message || "保存失败";
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">笔记页</h2>
        <p class="text-xs text-gray-400 mt-1 dark:text-gray-500">此内容渲染于前台 /notes 页上半部分（支持 Markdown 与站点 shortcode），下半部分为最近瞬间。</p>
      </div>
      <div class="flex items-center gap-3">
        <span v-if="message" :class="['text-sm px-3 py-1.5 rounded-lg', isError ? 'text-red-600 bg-red-50 dark:text-red-400 dark:bg-red-500/10' : 'text-green-700 bg-green-50 dark:text-green-400 dark:bg-green-500/10']">{{ message }}</span>
        <a href="/notes" target="_blank" class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400"><Icon icon="ri:external-link-line" /> 前台预览</a>
        <button @click="save" :disabled="saving || loading" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存</button>
      </div>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center dark:text-gray-500">加载中...</div>
    <MarkdownEditor v-else v-model="content" storage-key="notes-page" @save="save" />
  </div>
</template>
