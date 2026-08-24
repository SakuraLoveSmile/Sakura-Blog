<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { ucApi, type UCPostItem } from "@blog/api-client";

const router = useRouter();
const posts = ref<UCPostItem[]>([]);
const total = ref(0);
const page = ref(1);
const size = 12;
type PostStatus = "" | "draft" | "published" | "trashed";
const status = ref<PostStatus>("");
const loading = ref(false);
const error = ref("");
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size)));

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const { data } = await ucApi.posts.list({ page: page.value, size, status: status.value || undefined });
    posts.value = data.items;
    total.value = data.total;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "文章加载失败";
  } finally {
    loading.value = false;
  }
}

function setStatus(next: PostStatus) {
  status.value = next;
  page.value = 1;
  load();
}

async function removePost(post: UCPostItem) {
  if (!window.confirm(`删除「${post.title || post.slug}」？`)) return;
  await ucApi.posts.delete(post.slug);
  await load();
}

onMounted(load);
</script>

<template>
  <div>
    <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
      <div>
        <p class="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-primary">My writing</p>
        <h1 class="text-3xl font-semibold tracking-tight">我的文章</h1>
        <p class="mt-2 text-sm text-text-tertiary">只管理署名属于你的文章。</p>
      </div>
      <button type="button" class="uc-primary-button" @click="router.push('/posts/new')">
        <Icon icon="ri:add-line" />新建文章
      </button>
    </div>
    <div class="mb-5 flex flex-wrap items-center gap-2" role="tablist" aria-label="文章状态">
      <button
        v-for="item in [
          { key: '', label: '全部' },
          { key: 'draft', label: '草稿' },
          { key: 'published', label: '已发布' },
          { key: 'trashed', label: '回收站' },
        ]"
        :key="item.key"
        type="button"
        class="uc-tab"
        :class="{ active: status === item.key }"
        @click="setStatus(item.key as PostStatus)"
      >
        {{ item.label }}
      </button>
    </div>
    <div
      v-if="error"
      class="mb-4 rounded-xl border border-state-error/30 bg-state-error/10 px-4 py-3 text-sm text-state-error"
    >
      {{ error }}
    </div>
    <div
      v-if="loading"
      class="rounded-2xl border border-border bg-card px-5 py-12 text-center text-sm text-text-tertiary"
    >
      加载中…
    </div>
    <div v-else-if="!posts.length" class="rounded-2xl border border-border bg-card px-5 py-14 text-center">
      <Icon icon="ri:draft-line" class="mx-auto h-9 w-9 text-text-quaternary" />
      <p class="mt-3 text-sm text-text-secondary">还没有文章</p>
      <button type="button" class="mt-4 text-sm text-primary hover:underline" @click="router.push('/posts/new')">
        写下第一篇
      </button>
    </div>
    <div v-else class="grid gap-3">
      <article
        v-for="post in posts"
        :key="post.slug"
        class="rounded-2xl border border-border bg-card p-4 transition-colors hover:border-border-strong sm:p-5"
      >
        <div class="flex flex-wrap items-start justify-between gap-4">
          <button type="button" class="min-w-0 flex-1 text-left" @click="router.push(`/posts/${post.slug}`)">
            <h2 class="truncate text-base font-semibold text-foreground">{{ post.title || "无标题" }}</h2>
            <p class="mt-1 line-clamp-2 text-sm leading-6 text-text-tertiary">
              {{ post.excerpt || post.content_md?.slice(0, 120) || "暂无摘要" }}
            </p>
          </button>
          <div class="flex shrink-0 items-center gap-2">
            <span class="uc-status" :class="post.status === 'published' ? 'published' : 'draft'">{{
              post.status === "published" ? "已发布" : "草稿"
            }}</span>
            <button type="button" class="uc-icon-button" title="编辑" @click="router.push(`/posts/${post.slug}`)">
              <Icon icon="ri:edit-line" />
            </button>
            <button type="button" class="uc-icon-button danger" title="删除" @click="removePost(post)">
              <Icon icon="ri:delete-bin-line" />
            </button>
          </div>
        </div>
        <div class="mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-quaternary">
          <span>{{ new Date(post.updated_at || post.created_at).toLocaleString() }}</span>
          <span v-if="post.views > 0">{{ post.views }} 次阅读</span>
          <span v-if="post.tags">{{ post.tags }}</span>
        </div>
      </article>
    </div>
    <div v-if="totalPages > 1" class="mt-5 flex items-center justify-between text-sm text-text-tertiary">
      <span>共 {{ total }} 篇</span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="uc-secondary-button"
          :disabled="page <= 1"
          @click="
            page--;
            load();
          "
        >
          上一页
        </button>
        <span class="min-w-16 text-center">{{ page }} / {{ totalPages }}</span>
        <button
          type="button"
          class="uc-secondary-button"
          :disabled="page >= totalPages"
          @click="
            page++;
            load();
          "
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.uc-primary-button,
.uc-secondary-button,
.uc-tab,
.uc-icon-button {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border-radius: 9999px;
  padding: 0.65rem 1rem;
  font-size: 0.8rem;
  font-weight: 600;
  transition: 150ms var(--ease-standard);
}
.uc-primary-button {
  background: var(--color-primary);
  color: var(--color-primary-foreground);
}
.uc-primary-button:hover {
  background: var(--color-primary-hover);
}
.uc-secondary-button,
.uc-tab {
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
}
.uc-secondary-button:hover,
.uc-tab:hover,
.uc-tab.active {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.uc-secondary-button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}
.uc-icon-button {
  min-width: 44px;
  padding: 0;
  color: var(--color-text-tertiary);
}
.uc-icon-button:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.uc-icon-button.danger:hover {
  color: var(--color-state-error);
}
.uc-status {
  border-radius: 9999px;
  padding: 0.25rem 0.55rem;
  font-size: 0.7rem;
}
.uc-status.published {
  background: color-mix(in srgb, var(--color-state-success) 14%, transparent);
  color: var(--color-state-success);
}
.uc-status.draft {
  background: color-mix(in srgb, var(--color-state-warning) 14%, transparent);
  color: var(--color-state-warning);
}
</style>
