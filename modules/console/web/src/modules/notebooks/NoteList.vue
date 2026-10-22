<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { notebookApi, getApiErrorMessage, type NotebookNoteResponse, type NotebookResponse } from "@/api";
import { VPageHeader, VButton, VCard, VLoading, VEmpty } from "@/components/ui";
import { isMigrationError, notebookRoute } from "./notebook-form";
import "./notebooks.css";
const route = useRoute(),
  notebook = computed(() => String(route.params.notebook || ""));
const collection = ref<NotebookResponse>(),
  items = ref<NotebookNoteResponse[]>([]);
const q = ref(""),
  tag = ref(""),
  status = ref<"" | "draft" | "published">(""),
  page = ref(1),
  total = ref(0);
const loading = ref(false),
  error = ref(""),
  migration = ref(false);
let request = 0;
async function load(reset = false) {
  if (reset) page.value = 1;
  const current = ++request;
  loading.value = true;
  error.value = "";
  migration.value = false;
  try {
    const [book, notes] = await Promise.all([
      notebookApi.get(notebook.value),
      notebookApi.listNotes(notebook.value, {
        page: page.value,
        size: 20,
        q: q.value,
        tag: tag.value.trim(),
        status: status.value || undefined,
      }),
    ]);
    if (current !== request) return;
    collection.value = book.data;
    items.value = notes.data.items;
    total.value = notes.data.total;
  } catch (e) {
    if (current !== request) return;
    items.value = [];
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "加载笔记失败");
  } finally {
    if (current === request) loading.value = false;
  }
}
function turn(delta: number) {
  page.value += delta;
  void load();
}
watch(
  notebook,
  () => {
    q.value = "";
    tag.value = "";
    status.value = "";
    collection.value = undefined;
    void load(true);
  },
  { immediate: true },
);
</script>
<template>
  <div>
    <VPageHeader :title="collection?.name || notebook" subtitle="笔记管理">
      <template #actions
        ><VButton route="/notebooks">笔记本列表</VButton
        ><VButton :route="notebookRoute(notebook, 'edit')">集合设置</VButton
        ><VButton type="primary" :route="notebookRoute(notebook, 'notes/new')">新建笔记</VButton></template
      >
    </VPageHeader>
    <div class="notebooks-body">
      <form class="notebooks-toolbar" @submit.prevent="load(true)">
        <input
          v-model="q"
          class="notebooks-control"
          style="flex: 1; width: auto"
          aria-label="搜索笔记"
          placeholder="搜索标题、Slug 或正文"
        />
        <input
          v-model="tag"
          class="notebooks-control"
          style="flex: 1; width: auto"
          aria-label="标签筛选"
          placeholder="标签路径，例如 工作/记录"
        />
        <select v-model="status" class="notebooks-control" style="width: auto" aria-label="状态筛选">
          <option value="">全部状态</option>
          <option value="draft">草稿</option>
          <option value="published">已发布</option>
        </select>
        <VButton native-type="submit" :loading="loading">筛选</VButton>
      </form>
      <div v-if="error" class="notebooks-error" role="alert">
        <strong v-if="migration">需要迁移到 Stellar 2.0。先查看迁移预览。<br /></strong>{{ error }}
        <div class="notebooks-actions mt-3"><VButton @click="load()">重试</VButton></div>
      </div>
      <VLoading v-if="loading" />
      <VEmpty v-else-if="!error && !items.length" title="没有笔记" message="新建一篇笔记，或调整筛选条件。" />
      <VCard v-else-if="!error">
        <article v-for="note in items" :key="note.id" class="notebooks-row">
          <div class="notebooks-row-main">
            <RouterLink class="notebooks-link" :to="notebookRoute(notebook, `notes/${note.id}/edit`)">{{
              note.title || note.slug
            }}</RouterLink>
            <p class="notebooks-help mb-0">
              {{ note.status === "published" ? "已发布" : "草稿" }} · {{ note.slug }}<br />{{ note.tags.join(" · ") }}
            </p>
          </div>
          <VButton :route="notebookRoute(notebook, `notes/${note.id}/edit`)">编辑</VButton>
        </article>
      </VCard>
      <div v-if="!error" class="notebooks-pager">
        <span>共 {{ total }} 篇 · 第 {{ page }} 页</span
        ><VButton :disabled="loading || page <= 1" @click="turn(-1)">上一页</VButton
        ><VButton :disabled="loading || page * 20 >= total" @click="turn(1)">下一页</VButton>
      </div>
    </div>
  </div>
</template>
