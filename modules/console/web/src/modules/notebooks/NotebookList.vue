<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { notebookApi, getApiErrorMessage, type NotebookResponse } from "@/api";
import { VPageHeader, VButton, VCard, VLoading, VEmpty } from "@/components/ui";
import NotebookMigrationNotice from "./NotebookMigrationNotice.vue";
import { isMigrationError, notebookCountsLabel, notebookRoute } from "./notebook-form";
import "./notebooks.css";
const items = ref<NotebookResponse[]>([]),
  q = ref(""),
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
    const { data } = await notebookApi.list({ page: page.value, size: 20, q: q.value });
    if (current !== request) return;
    items.value = data.items;
    total.value = data.total;
  } catch (e) {
    if (current !== request) return;
    items.value = [];
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "加载笔记本失败");
  } finally {
    if (current === request) loading.value = false;
  }
}
function turn(delta: number) {
  page.value += delta;
  void load();
}
onMounted(() => load());
</script>
<template>
  <div>
    <VPageHeader title="笔记本" subtitle="把相关笔记整理成集合，发布后显示在博客的笔记本页面">
      <template #actions><VButton type="primary" route="/notebooks/new">新建笔记本</VButton></template>
    </VPageHeader>
    <div class="notebooks-body">
      <form class="notebooks-toolbar" @submit.prevent="load(true)">
        <input
          v-model="q"
          class="notebooks-control notebooks-toolbar-field"
          aria-label="搜索笔记本"
          placeholder="搜索名称、ID 或描述"
        /><VButton native-type="submit" :loading="loading">搜索</VButton>
      </form>
      <NotebookMigrationNotice v-if="migration" :detail="error"
        ><VButton :loading="loading" @click="load()">重试</VButton></NotebookMigrationNotice
      >
      <div v-else-if="error" class="notebooks-error" role="alert">
        {{ error }}
        <div class="notebooks-actions mt-3"><VButton @click="load()">重试</VButton></div>
      </div>
      <VLoading v-if="loading" />
      <VEmpty
        v-else-if="!error && !items.length"
        title="还没有笔记本"
        message="新建一个笔记本来整理笔记，或调整搜索条件。"
      />
      <div v-else-if="!error" class="notebooks-grid">
        <VCard v-for="item in items" :key="item.id">
          <div class="notebooks-card">
            <div>
              <h2 class="notebooks-card-title">{{ item.name || item.id }}</h2>
              <p class="notebooks-card-id">{{ item.id }}</p>
            </div>
            <p class="notebooks-card-desc">{{ item.description || "暂无描述" }}</p>
            <p class="notebooks-card-meta">{{ notebookCountsLabel(item) }}</p>
            <div class="notebooks-actions">
              <VButton type="primary" :route="notebookRoute(item.id)">管理笔记</VButton>
              <VButton type="ghost" :route="notebookRoute(item.id, 'edit')">编辑</VButton>
              <a
                v-if="item.public_path && item.published_count"
                class="notebooks-ext-link"
                :href="item.public_path"
                target="_blank"
                rel="noopener noreferrer"
                ><Icon icon="ri:external-link-line" aria-hidden="true" />在博客中查看</a
              >
            </div>
            <p v-if="!item.published_count" class="notebooks-card-meta">发布笔记后，博客前台才会显示这个笔记本。</p>
          </div>
        </VCard>
      </div>
      <div v-if="!error" class="notebooks-pager">
        <span>共 {{ total }} 个 · 第 {{ page }} 页</span
        ><VButton :disabled="loading || page <= 1" @click="turn(-1)">上一页</VButton
        ><VButton :disabled="loading || page * 20 >= total" @click="turn(1)">下一页</VButton>
      </div>
    </div>
  </div>
</template>
