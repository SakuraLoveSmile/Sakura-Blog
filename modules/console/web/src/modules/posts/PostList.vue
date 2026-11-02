<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useQueryClient } from "@tanstack/vue-query";
import { postApi, shareApi, taxonomyApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { usePagedList } from "@/composables/usePagedList";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VButton,
  VEmpty,
  VLoading,
  VFloatingDock,
  VPagination,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";
import { formatRelative, formatDateTime } from "@/lib/format";

interface PostItem {
  id?: number;
  slug: string;
  title: string;
  cover?: string;
  excerpt?: string;
  category?: string;
  tags?: string | string[];
  status: "published" | "draft" | "trashed";
  pinned?: boolean;
  views?: number;
  created_at: string;
  updated_at: string;
}

const router = useRouter();
const route = useRoute();
const queryClient = useQueryClient();

const validStatuses = ["published", "draft", "trashed"];
function statusFromQuery(): string | undefined {
  const s = route.query.status;
  return typeof s === "string" && validStatuses.includes(s) ? s : undefined;
}

function categoryFromQuery(): string | undefined {
  const category = route.query.category;
  return typeof category === "string" && category.trim() ? category : undefined;
}

const selectedStatus = ref<string | undefined>(statusFromQuery());
const selectedSort = ref<string>("newest");
const selectedCategory = ref<string | undefined>(categoryFromQuery());
const selectedTag = ref<string | undefined>(undefined);
const VIEW_KEY = "console:posts-view";
const viewMode = ref<"grid" | "table">(localStorage.getItem(VIEW_KEY) === "grid" ? "grid" : "table");
watch(viewMode, (value) => localStorage.setItem(VIEW_KEY, value));

const {
  page,
  size,
  keyword,
  items: rawItems,
  total,
  isLoading,
  isError,
  refetch,
} = usePagedList<PostItem>({
  apiFn: (params: Record<string, unknown>) => postApi.list(params),
  keyFn: (params: Record<string, unknown>) => queryKeys.posts(params),
  keywordParam: "search",
  extraParams: () => {
    const params: Record<string, unknown> = {};
    if (selectedStatus.value) params.status = selectedStatus.value;
    if (selectedCategory.value) params.category = selectedCategory.value;
    if (selectedTag.value) params.tag = selectedTag.value;
    return params;
  },
});

watch(
  () => route.query.status,
  () => {
    const s = statusFromQuery();
    if (s !== selectedStatus.value) {
      selectedStatus.value = s;
      page.value = 1;
    }
  },
);

watch(
  () => route.query.category,
  () => {
    const category = categoryFromQuery();
    if (category !== selectedCategory.value) {
      selectedCategory.value = category;
      page.value = 1;
    }
  },
);

// URL 是状态筛选的真值，返回文章列表和浏览器前进后退保持一致。
function selectStatus(status: string | undefined) {
  router.push({ query: { ...route.query, status } });
}
const hasFilters = computed(
  () => !!(keyword.value || selectedStatus.value || selectedCategory.value || selectedTag.value),
);
function clearFilters() {
  keyword.value = "";
  selectedTag.value = undefined;
  selectedCategory.value = undefined;
  selectedStatus.value = undefined;
  page.value = 1;
  router.replace({ query: { ...route.query, status: undefined, category: undefined } });
}
watch([selectedCategory, selectedTag], () => {
  page.value = 1;
});

// 分类与标签选项
const categories = ref<string[]>([]);
const tags = ref<string[]>([]);

onMounted(async () => {
  try {
    const [cRes, tRes] = await Promise.all([taxonomyApi.categories(), taxonomyApi.tags()]);
    categories.value = (cRes.data.items || []).map((i: any) => i.name);
    tags.value = (tRes.data.items || []).map((i: any) => i.name);
  } catch {}
});

// 客户端排序
const posts = computed(() => {
  const items = [...rawItems.value];
  if (selectedSort.value === "newest") {
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  } else if (selectedSort.value === "oldest") {
    items.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  } else if (selectedSort.value === "updated") {
    items.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
  }
  return items;
});

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = posts.value;
  let pubCount = 0;
  let draftCount = 0;
  let pinCount = 0;
  for (const p of all) {
    if (p.status === "published") pubCount++;
    else if (p.status === "draft") draftCount++;
    if (p.pinned) pinCount++;
  }
  return {
    total: total.value,
    pubCount,
    draftCount,
    pinCount,
  };
});

// 批量选择
const selectedSlugs = ref<Set<string>>(new Set());

function toggleSelect(slug: string) {
  if (selectedSlugs.value.has(slug)) selectedSlugs.value.delete(slug);
  else selectedSlugs.value.add(slug);
}

function selectAll() {
  for (const p of posts.value) selectedSlugs.value.add(p.slug);
}

function clearSelection() {
  selectedSlugs.value.clear();
}

// 搜索、筛选或分页使旧选择失效时清除失效选择；加载中沿用上一批数据，不伪造空态
watch(
  () => (isLoading.value ? null : posts.value.map((p) => p.slug)),
  (slugs) => {
    if (!slugs) return;
    const visible = new Set(slugs);
    for (const slug of Array.from(selectedSlugs.value)) {
      if (!visible.has(slug)) selectedSlugs.value.delete(slug);
    }
  },
);

// ---------- 操作流 ----------
function handleDelete(p: PostItem) {
  Dialog.warning({
    title: "确定将文章移入回收站吗？",
    description: `「${p.title}」将被移入回收站，可在回收站中随时恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.delete(p.slug);
        selectedSlugs.value.delete(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("已移入回收站");
      } catch {}
    },
  });
}

function handleRestore(p: PostItem) {
  Dialog.warning({
    title: "确定恢复该文章吗？",
    description: `「${p.title}」将恢复至草稿/发布状态。`,
    onConfirm: async () => {
      try {
        await postApi.restore(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("文章已恢复");
      } catch {}
    },
  });
}

function handlePurge(p: PostItem) {
  Dialog.warning({
    title: "确定彻底删除该文章吗？",
    description: `「${p.title}」将被永久物理删除，不可恢复！`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.purge(p.slug);
        selectedSlugs.value.delete(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("已彻底删除");
      } catch {}
    },
  });
}

async function handleBatchDelete() {
  const slugs = Array.from(selectedSlugs.value);
  if (!slugs.length) return;
  Dialog.warning({
    title: `确定将选中的 ${slugs.length} 篇文章移入回收站吗？`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await Promise.all(slugs.map((s) => postApi.delete(s)));
        clearSelection();
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("批量操作成功");
      } catch {}
    },
  });
}

// ---------- 分享链接创建弹窗 ----------
const shareModalOpen = ref(false);
const shareTarget = ref<PostItem | null>(null);
const shareDuration = ref<"1" | "7" | "30" | "0">("7");
const shareCreating = ref(false);
const generatedShareUrl = ref("");

function openShareModal(p: PostItem) {
  shareTarget.value = p;
  shareDuration.value = "7";
  generatedShareUrl.value = "";
  shareModalOpen.value = true;
}

async function createShare() {
  if (!shareTarget.value) return;
  shareCreating.value = true;
  try {
    const days = Number(shareDuration.value);
    const expiresAt = days > 0 ? new Date(Date.now() + days * 86400000).toISOString() : "";
    const { data } = await shareApi.create(shareTarget.value.slug, {
      share_type: shareTarget.value.status === "draft" ? "draft" : "published",
      expires_at: expiresAt,
    });
    generatedShareUrl.value = window.location.origin + data.url;
    Toast.success("分享链接已生成");
  } catch {
  } finally {
    shareCreating.value = false;
  }
}

function copyShareUrl() {
  navigator.clipboard.writeText(generatedShareUrl.value).then(() => {
    Toast.success("已复制到剪贴板");
  });
}

function openPostPreview(slug: string) {
  window.open(`/${slug}`, "_blank");
}

function formatViews(views: number): string {
  if (views < 1000) return String(views);
  const value = views / 1000;
  return `${value.toFixed(value >= 10 ? 0 : 1).replace(/\.0$/, "")}k`;
}

function parseTags(tags: string | string[] | undefined | null): string[] {
  if (!tags) return [];
  if (Array.isArray(tags)) return tags.filter(Boolean);
  return tags
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

function statusBadge(status: string) {
  switch (status) {
    case "published":
      return { cls: "text-state-success", label: "已发布" };
    case "draft":
      return { cls: "text-state-warning", label: "草稿" };
    case "trashed":
      return { cls: "text-state-error", label: "回收站" };
    default:
      return { cls: "text-text-tertiary", label: status };
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文章管理" :hide-on-scroll="false" actions-layout="stack-below-lg">
      <template #subtitle>{{ total }} 篇文章</template>
      <template #actions>
        <div class="flex w-full flex-col gap-2.5 lg:w-auto lg:flex-row lg:items-center">
          <div class="apple-search w-full lg:min-w-[200px] lg:max-w-[280px]">
            <Icon icon="ri:search-line" class="text-text-quaternary" />
            <input v-model="keyword" aria-label="搜索文章" placeholder="搜索文章标题或 Slug..." />
            <button
              v-if="keyword"
              type="button"
              aria-label="清除搜索"
              class="text-text-quaternary hover:text-foreground"
              @click="keyword = ''"
            >
              <Icon icon="ri:close-line" />
            </button>
          </div>

          <div class="flex w-full flex-wrap items-center gap-2 lg:w-auto lg:flex-nowrap sm:gap-2.5">
            <VButton :route="{ path: '/posts/shares' }">
              <template #icon><Icon icon="ri:share-forward-line" /></template>
              分享链接
            </VButton>

            <VButton type="primary" :route="{ path: '/posts/new' }">
              <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
              撰写文章
            </VButton>
          </div>
        </div>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 工具栏：状态分段 + 筛选 + 视图模式 -->
      <div class="console-toolbar flex flex-wrap items-center justify-between gap-3">
        <!-- 状态分段 -->
        <div class="apple-segmented" role="group" aria-label="文章状态">
          <button
            type="button"
            :class="{ active: selectedStatus === undefined }"
            @click="selectStatus(undefined)"
            :aria-pressed="selectedStatus === undefined"
          >
            全部
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'published' }"
            @click="selectStatus('published')"
            :aria-pressed="selectedStatus === 'published'"
          >
            已发布
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'draft' }"
            @click="selectStatus('draft')"
            :aria-pressed="selectedStatus === 'draft'"
          >
            草稿
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'trashed' }"
            @click="selectStatus('trashed')"
            :aria-pressed="selectedStatus === 'trashed'"
          >
            回收站
          </button>
        </div>

        <!-- 筛选器与视图 -->
        <div class="flex flex-wrap items-center gap-2">
          <!-- 分类下拉 -->
          <select v-model="selectedCategory" aria-label="分类筛选" class="apple-select text-xs h-8">
            <option :value="undefined">全部分类</option>
            <option v-for="c in categories" :key="c" :value="c">{{ c }}</option>
          </select>

          <!-- 标签下拉 -->
          <select v-model="selectedTag" aria-label="标签筛选" class="apple-select text-xs h-8">
            <option :value="undefined">全部标签</option>
            <option v-for="t in tags" :key="t" :value="t">{{ t }}</option>
          </select>

          <!-- 排序 -->
          <select v-model="selectedSort" aria-label="本页排序" class="apple-select text-xs h-8">
            <option value="newest">本页 · 最近创建</option>
            <option value="updated">本页 · 最近更新</option>
            <option value="oldest">本页 · 最早创建</option>
          </select>

          <button
            v-if="posts.length > 0"
            type="button"
            class="apple-filter-btn rounded-lg border border-border px-2.5 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            @click="selectedSlugs.size === posts.length ? clearSelection() : selectAll()"
          >
            {{ selectedSlugs.size === posts.length ? "取消全选" : "全选本页" }}
          </button>

          <div class="apple-segmented">
            <button
              type="button"
              class="segmented-icon-btn"
              :class="{ active: viewMode === 'grid' }"
              title="海报卡片视图"
              aria-label="海报卡片视图"
              :aria-pressed="viewMode === 'grid'"
              @click="viewMode = 'grid'"
            >
              <Icon icon="ri:layout-grid-line" />
            </button>
            <button
              type="button"
              class="segmented-icon-btn"
              :class="{ active: viewMode === 'table' }"
              title="列表视图"
              aria-label="列表视图"
              :aria-pressed="viewMode === 'table'"
              @click="viewMode = 'table'"
            >
              <Icon icon="ri:table-line" />
            </button>
          </div>
        </div>
      </div>

      <div class="console-summary" aria-label="文章摘要">
        <div class="stats-card">
          <span class="stats-label">匹配文章</span><span class="stats-value">{{ stats.total }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页已发布</span><span class="stats-value">{{ stats.pubCount }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页草稿</span><span class="stats-value">{{ stats.draftCount }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页置顶</span><span class="stats-value">{{ stats.pinCount }}</span>
        </div>
      </div>

      <div
        v-if="hasFilters"
        class="flex flex-wrap items-center justify-between gap-2 text-sm text-text-tertiary"
        aria-live="polite"
      >
        <span>已筛选 · {{ total }} 篇文章</span>
        <VButton size="sm" @click="clearFilters">清除筛选</VButton>
      </div>

      <VLoading v-if="isLoading" />
      <VEmpty v-else-if="isError" title="文章加载失败" message="请稍后重试，已有文章不会受影响。">
        <template #actions><VButton @click="refetch()">重新加载</VButton></template>
      </VEmpty>

      <VEmpty
        v-else-if="posts.length === 0"
        :title="hasFilters ? '未找到匹配的文章' : '暂无文章'"
        :message="hasFilters ? '可以调整筛选条件，或清除筛选查看全部文章。' : '点击“撰写文章”创建草稿。'"
      >
        <template #actions>
          <VButton v-if="hasFilters" @click="clearFilters">清除筛选</VButton>
          <VButton v-else type="primary" :route="{ path: '/posts/new' }">创建第一篇文章</VButton>
        </template>
      </VEmpty>

      <!-- ============ 模式 1：海报卡片网格（保留功能与偏好，只简化视觉） ============ -->
      <div v-else-if="viewMode === 'grid'" class="post-grid">
        <article
          v-for="p in posts"
          :key="p.slug"
          class="post-card group"
          :class="{ 'is-selected': selectedSlugs.has(p.slug) }"
        >
          <div class="post-cover" @click="router.push(`/posts/edit/${p.slug}`)">
            <img
              v-if="p.cover"
              :src="p.cover"
              alt=""
              loading="lazy"
              decoding="async"
              class="h-full w-full object-cover"
            />
            <div v-else class="post-card-placeholder flex h-full w-full items-center justify-center">
              <Icon icon="ri:article-line" />
            </div>

            <label class="post-select" :aria-label="`选择文章：${p.title}`" @click.stop>
              <input type="checkbox" :checked="selectedSlugs.has(p.slug)" class="sr-only" @change="toggleSelect(p.slug)" />
              <span class="post-select-knob" :class="{ 'is-checked': selectedSlugs.has(p.slug) }">
                <Icon v-if="selectedSlugs.has(p.slug)" icon="ri:check-line" />
              </span>
            </label>
          </div>

          <div class="post-card-body">
            <div class="post-card-meta">
              <span class="post-status" :class="statusBadge(p.status).cls">{{ statusBadge(p.status).label }}</span>
              <span v-if="p.pinned" class="post-pinned">置顶</span>
              <button
                v-if="p.category"
                type="button"
                class="post-meta-link"
                :title="`按分类「${p.category}」筛选`"
                @click.stop="selectedCategory = p.category; page = 1"
              >
                {{ p.category }}
              </button>
              <button
                v-for="t in parseTags(p.tags).slice(0, 2)"
                :key="t"
                type="button"
                class="post-meta-link"
                :title="`按标签「#${t}」筛选`"
                @click.stop="selectedTag = t; page = 1"
              >
                #{{ t }}
              </button>
              <span class="post-card-views">{{ formatViews(p.views ?? 0) }} 浏览</span>
            </div>

            <h3 class="post-title-inner" :title="p.title" @click="router.push(`/posts/edit/${p.slug}`)">
              {{ p.title }}
            </h3>
            <p v-if="p.excerpt" class="post-excerpt">{{ p.excerpt }}</p>

            <div class="post-card-footer">
              <span class="post-card-time" :title="`创建：${formatDateTime(p.created_at)}\n更新：${formatDateTime(p.updated_at)}`">
                {{ formatRelative(p.created_at) }}
              </span>
              <div class="post-card-actions">
                <button type="button" class="post-card-action" title="生成临时分享链接" @click="openShareModal(p)">
                  <Icon icon="ri:share-forward-line" />
                </button>
                <button
                  v-if="p.status === 'published'"
                  type="button"
                  class="post-card-action"
                  title="前台查阅"
                  @click="openPostPreview(p.slug)"
                >
                  <Icon icon="ri:external-link-line" />
                </button>
                <button
                  v-if="p.status !== 'trashed'"
                  type="button"
                  class="post-card-action"
                  title="编辑博文"
                  @click="router.push(`/posts/edit/${p.slug}`)"
                >
                  <Icon icon="ri:edit-line" />
                </button>
                <button
                  v-if="p.status === 'trashed'"
                  type="button"
                  class="post-card-action"
                  title="恢复博文"
                  @click="handleRestore(p)"
                >
                  <Icon icon="ri:restart-line" />
                </button>
                <button
                  v-if="p.status !== 'trashed'"
                  type="button"
                  class="post-card-action post-card-action-danger"
                  title="移入回收站"
                  @click="handleDelete(p)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>
                <button
                  v-if="p.status === 'trashed'"
                  type="button"
                  class="post-card-action post-card-action-danger"
                  title="彻底删除"
                  @click="handlePurge(p)"
                >
                  <Icon icon="ri:delete-bin-7-line" />
                </button>
              </div>
            </div>
          </div>
        </article>
      </div>

      <!-- 行列表在窄屏内折行，保留同一份选择与操作状态。 -->
      <ul v-else class="console-records content-records" aria-label="文章列表">
        <li
          v-for="p in posts"
          :key="p.slug"
          class="content-record post-record"
          :class="{ 'is-selected': selectedSlugs.has(p.slug) }"
        >
          <label class="record-selection" :aria-label="`选择文章：${p.title}`">
            <input
              type="checkbox"
              :aria-label="`选择文章：${p.title}`"
              :checked="selectedSlugs.has(p.slug)"
              @change="toggleSelect(p.slug)"
            />
          </label>
          <div class="record-main">
            <div class="record-title-line">
              <button type="button" class="record-title" :title="p.title" @click="router.push(`/posts/edit/${p.slug}`)">
                {{ p.title }}
              </button>
              <span v-if="p.pinned" class="record-note">置顶</span>
            </div>
            <div class="record-metadata">
              <span class="record-slug" :title="`/${p.slug}`">/{{ p.slug }}</span>
              <button
                v-if="p.category"
                type="button"
                :title="`按分类「${p.category}」筛选`"
                @click="
                  selectedCategory = p.category;
                  page = 1;
                "
              >
                {{ p.category }}
              </button>
              <button
                v-for="tag in parseTags(p.tags).slice(0, 2)"
                :key="tag"
                type="button"
                :title="`按标签「#${tag}」筛选`"
                @click="
                  selectedTag = tag;
                  page = 1;
                "
              >
                #{{ tag }}
              </button>
            </div>
          </div>
          <div class="record-state">
            <span class="record-status" :class="statusBadge(p.status).cls">{{ statusBadge(p.status).label }}</span>
            <span class="record-updated" :title="`更新时间：${formatDateTime(p.updated_at)}`">{{
              formatRelative(p.updated_at)
            }}</span>
            <span class="record-views">{{ formatViews(p.views ?? 0) }} 浏览</span>
          </div>
          <div class="record-actions">
            <button type="button" class="action-icon-btn" title="生成临时分享链接" @click="openShareModal(p)">
              <Icon icon="ri:share-forward-line" />
            </button>
            <button
              v-if="p.status === 'published'"
              type="button"
              class="action-icon-btn"
              title="前台查阅"
              @click="openPostPreview(p.slug)"
            >
              <Icon icon="ri:external-link-line" />
            </button>
            <button
              v-if="p.status !== 'trashed'"
              type="button"
              class="action-icon-btn"
              title="编辑博文"
              @click="router.push(`/posts/edit/${p.slug}`)"
            >
              <Icon icon="ri:edit-line" />
            </button>
            <button
              v-if="p.status === 'trashed'"
              type="button"
              class="action-icon-btn text-primary"
              title="恢复博文"
              @click="handleRestore(p)"
            >
              <Icon icon="ri:restart-line" />
            </button>
            <button
              v-if="p.status !== 'trashed'"
              type="button"
              class="action-icon-btn record-danger"
              title="移入回收站"
              @click="handleDelete(p)"
            >
              <Icon icon="ri:delete-bin-line" />
            </button>
            <button
              v-if="p.status === 'trashed'"
              type="button"
              class="action-icon-btn record-danger text-state-error"
              title="彻底删除"
              @click="handlePurge(p)"
            >
              <Icon icon="ri:delete-bin-7-line" />
            </button>
          </div>
        </li>
      </ul>

      <!-- 分页 -->
      <div v-if="total > size" class="flex justify-center pt-4">
        <VPagination
          :page="page"
          :size="size"
          :total="total"
          @update:page="page = $event"
          @update:size="size = $event"
        />
      </div>
    </div>

    <!-- ============ 底部批量操作浮动栏 ============ -->
    <VFloatingDock
      :visible="selectedSlugs.size > 0"
      :count="selectedSlugs.size"
      title="已选文章"
      @clear="clearSelection"
    >
      <div class="flex items-center gap-2">
        <VButton size="sm" type="danger" @click="handleBatchDelete"> 批量移入回收站 </VButton>
      </div>
    </VFloatingDock>

    <!-- ============ 生成分享链接弹窗 ============ -->
    <VModal v-model:visible="shareModalOpen" title="生成临时分享链接" :width="460">
      <div class="space-y-4 text-xs">
        <div class="rounded-xl border border-border bg-secondary/40 p-3">
          <div class="text-text-quaternary">目标文章：</div>
          <div class="font-bold text-foreground text-sm mt-0.5">{{ shareTarget?.title }}</div>
        </div>

        <div>
          <label class="mb-1.5 block font-medium text-text-tertiary">链接有效期</label>
          <select v-model="shareDuration" class="apple-select w-full">
            <option value="1">1 天有效</option>
            <option value="7">7 天有效</option>
            <option value="30">30 天有效</option>
            <option value="0">永久有效</option>
          </select>
        </div>

        <div v-if="generatedShareUrl" class="space-y-1.5">
          <label class="block font-medium text-text-tertiary">已生成的分享直链</label>
          <div class="flex items-center gap-2">
            <input :value="generatedShareUrl" readonly class="apple-input font-mono text-xs" />
            <VButton size="sm" @click="copyShareUrl">复制</VButton>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="shareModalOpen = false">关闭</VButton>
          <VButton v-if="!generatedShareUrl" type="primary" :loading="shareCreating" @click="createShare">
            生成链接
          </VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
/* 海报视图保留封面、摘要与操作，去掉玻璃浮层、彩色胶囊和重阴影。 */
.post-grid {
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 24px 32px;
}
@media (min-width: 640px) {
  .post-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (min-width: 1024px) {
  .post-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (min-width: 1280px) {
  .post-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

.post-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  border-bottom: 1px solid var(--color-border);
  padding-bottom: 16px;
  transition: background-color var(--duration-fast) var(--ease-standard);
}
.post-card.is-selected {
  background: var(--color-accent);
}

.post-cover {
  position: relative;
  aspect-ratio: 16 / 9;
  width: 100%;
  overflow: hidden;
  background: var(--color-secondary);
  border-radius: var(--radius-md);
  cursor: pointer;
}

.post-card-placeholder {
  color: var(--color-text-quaternary);
  font-size: 24px;
}

.post-select {
  position: absolute;
  top: 4px;
  left: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  cursor: pointer;
}
.post-select-knob {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: 1px solid var(--color-border-strong);
  border-radius: 50%;
  background: var(--color-background);
  color: var(--color-background);
  font-size: 12px;
}
.post-select-knob.is-checked {
  background: var(--color-foreground);
  border-color: var(--color-foreground);
}

.post-pinned {
  color: var(--color-text-secondary);
  font-size: 12px;
}

.post-card-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 6px;
  padding-top: 12px;
}

.post-card-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 20px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.post-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}
.post-status::before {
  content: "";
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}
.post-meta-link {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-tertiary);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.post-meta-link:hover {
  color: var(--color-foreground);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.post-card-views {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}

.post-title-inner {
  overflow: hidden;
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}
.post-title-inner:hover {
  text-decoration: underline;
  text-underline-offset: 3px;
}
.post-excerpt {
  display: -webkit-box;
  overflow: hidden;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.post-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: auto;
  padding-top: 8px;
  border-top: 1px solid var(--color-border);
}
.post-card-time {
  color: var(--color-text-tertiary);
  font-size: 12px;
}

.post-card-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  transition: opacity var(--duration-fast) var(--ease-standard);
}

.post-card-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.post-card-action:hover {
  border-color: var(--color-border);
  background: var(--color-accent);
  color: var(--color-foreground);
}

.post-card-action:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.post-card-action-danger:hover {
  border-color: var(--color-state-error);
  color: var(--color-state-error);
}

/* 操作小按钮（表格视图沿用） */
.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

@media (hover: hover) and (min-width: 1024px) {
  .post-card-actions { opacity: 0; }
  .post-card:hover .post-card-actions,
  .post-card:focus-within .post-card-actions,
  .post-card.is-selected .post-card-actions { opacity: 1; }
}

@media (max-width: 1023px) {
  .action-icon-btn, .post-card-action { width: 44px; height: 44px; }
  .post-select { top: 0; left: 0; width: 44px; height: 44px; }
}
</style>
