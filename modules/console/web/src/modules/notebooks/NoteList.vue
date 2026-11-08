<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { Icon } from "@iconify/vue";
import { notebookApi, getApiErrorMessage, type NotebookNoteResponse, type NotebookResponse } from "@/api";
import { VPageHeader, VButton, VCard, VLoading, VEmpty, VTag } from "@/components/ui";
import NotebookMigrationNotice from "./NotebookMigrationNotice.vue";
import { formatNoteTime, isMigrationError, notebookCountsLabel, noteStatusLabel, notebookRoute } from "./notebook-form";
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
const subtitle = computed(() => (collection.value ? notebookCountsLabel(collection.value) : "管理这个笔记本里的笔记"));
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
    <VPageHeader :title="collection?.name || notebook" :subtitle="subtitle">
      <template #actions
        ><a
          v-if="collection?.public_path && collection.published_count"
          class="notebooks-ext-link"
          :href="collection.public_path"
          target="_blank"
          rel="noopener noreferrer"
          ><Icon icon="ri:external-link-line" aria-hidden="true" />在博客中查看</a
        ><VButton route="/notebooks">笔记本列表</VButton
        ><VButton :route="notebookRoute(notebook, 'edit')">集合设置</VButton
        ><VButton type="primary" :route="notebookRoute(notebook, 'notes/new')">新建笔记</VButton></template
      >
    </VPageHeader>
    <div class="notebooks-body">
      <form class="notebooks-toolbar" @submit.prevent="load(true)">
        <input
          v-model="q"
          class="notebooks-control notebooks-toolbar-field"
          aria-label="搜索笔记"
          placeholder="搜索标题、Slug 或正文"
        />
        <input
          v-model="tag"
          class="notebooks-control notebooks-toolbar-field"
          aria-label="标签筛选"
          placeholder="标签路径，例如 工作/记录"
        />
        <select v-model="status" class="notebooks-control notebooks-toolbar-select" aria-label="状态筛选">
          <option value="">全部状态</option>
          <option value="draft">草稿</option>
          <option value="published">已发布</option>
        </select>
        <VButton native-type="submit" :loading="loading">筛选</VButton>
      </form>
      <NotebookMigrationNotice v-if="migration" :detail="error"
        ><VButton :loading="loading" @click="load()">重试</VButton></NotebookMigrationNotice
      >
      <div v-else-if="error" class="notebooks-error" role="alert">
        {{ error }}
        <div class="notebooks-actions mt-3"><VButton @click="load()">重试</VButton></div>
      </div>
      <VLoading v-if="loading" />
      <VEmpty v-else-if="!error && !items.length" title="没有笔记" message="新建一篇笔记，或调整筛选条件。" />
      <VCard v-else-if="!error">
        <article v-for="note in items" :key="note.id" class="notebooks-row">
          <div class="notebooks-row-main">
            <div class="notebooks-row-title">
              <RouterLink class="notebooks-link" :to="notebookRoute(notebook, `notes/${note.id}/edit`)">{{
                note.title || note.slug
              }}</RouterLink>
              <VTag :theme="note.status === 'published' ? 'primary' : 'default'">{{
                noteStatusLabel(note.status)
              }}</VTag>
            </div>
            <p class="notebooks-help notebooks-row-meta">
              <span class="notebooks-mono">{{ note.slug }}</span>
              <span v-if="formatNoteTime(note.updated_at)">更新于 {{ formatNoteTime(note.updated_at) }}</span>
            </p>
            <div v-if="note.tags.length" class="notebooks-tags">
              <VTag v-for="item in note.tags" :key="item">{{ item }}</VTag>
            </div>
          </div>
          <div class="notebooks-actions">
            <a
              v-if="note.public_path"
              class="notebooks-ext-link"
              :href="note.public_path"
              target="_blank"
              rel="noopener noreferrer"
              ><Icon icon="ri:external-link-line" aria-hidden="true" />在博客中查看</a
            >
            <VButton :route="notebookRoute(notebook, `notes/${note.id}/edit`)">编辑</VButton>
          </div>
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
