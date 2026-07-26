<script setup lang="ts">
import { ref, onMounted } from "vue";
import { notesApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VLoading, Toast } from "@/components/ui";

// 笔记页编辑器：content/notes.md 原文读写，渲染于前台 /notes 页上半部分。
const content = ref("");
const loading = ref(true);
const saving = ref(false);

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
  try {
    await notesApi.update(content.value);
    Toast.success("已保存，前台 /notes 已生效");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

function openPreview() {
  window.open("/notes", "_blank");
}

onMounted(load);
</script>

<template>
  <div>
    <VPageHeader title="笔记页">
      <template #icon>
        <Icon icon="ri:booklet-line" class="mr-2 self-center text-xl" />
      </template>
      <template #subtitle>
        此内容渲染于前台 /notes 页上半部分（支持 Markdown 与站点 shortcode），下半部分为最近瞬间。
      </template>
      <template #actions>
        <div class="flex items-center gap-2">
          <VButton size="sm" @click="openPreview">
            <template #icon>
              <Icon icon="ri:external-link-line" class="h-full w-full" />
            </template>
            前台预览
          </VButton>
          <VButton type="secondary" :loading="saving" :disabled="loading" @click="save">保存</VButton>
        </div>
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VLoading v-if="loading" />
      <MarkdownEditor v-else v-model="content" storage-key="notes-page" @save="save" />
    </div>
  </div>
</template>
