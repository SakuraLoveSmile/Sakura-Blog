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
      return { cls: "bg-state-success/10 text-state-success", dotCls: "bg-state-success", label: "已发布" };
    case "draft":
      return { cls: "bg-state-warning/10 text-state-warning", dotCls: "bg-state-warning", label: "草稿" };
    case "trashed":
      return { cls: "bg-state-error/10 text-state-error", dotCls: "bg-state-error", label: "回收站" };
    default:
      return { cls: "bg-secondary text-text-tertiary", dotCls: "bg-text-quaternary", label: status };
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文章管理" :hide-on-scroll="false" actions-layout="stack-below-lg">
      <template #icon>
        <Icon icon="ri:article-line" class="text-primary" />
      </template>
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

      <!-- ============ 模式 1：海报卡片网格 ============ -->
      <div v-else-if="viewMode === 'grid'" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <article
          v-for="p in posts"
          :key="p.slug"
          class="post-card group relative flex flex-col overflow-hidden rounded-lg border border-border bg-card hover:border-border-strong"
          :class="{ 'is-selected': selectedSlugs.has(p.slug) }"
        >
          <!-- 封面图：只保留媒体与轻量浮层，点击直达编辑，悬停平滑缩放 -->
          <div
            class="relative aspect-video w-full cursor-pointer overflow-hidden bg-secondary"
            @click="router.push(`/posts/edit/${p.slug}`)"
          >
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

            <!-- 顶部浮动栏：多选、置顶与状态各自保持清晰热区与高对比浮层 -->
            <div class="pointer-events-none absolute inset-x-2.5 top-2.5 flex items-center justify-between">
              <div class="pointer-events-auto flex items-center gap-1.5" @click.stop>
                <label
                  class="group/cb relative flex h-7 w-7 cursor-pointer items-center justify-center rounded-full transition-transform duration-150 active:scale-90"
                  :aria-label="`选择文章：${p.title}`"
                >
                  <input
                    type="checkbox"
                    :checked="selectedSlugs.has(p.slug)"
                    class="sr-only"
                    @change="toggleSelect(p.slug)"
                  />
                  <!-- 未选中：高对比圆环 + 悬停微光点 -->
                  <div
                    v-if="!selectedSlugs.has(p.slug)"
                    class="flex h-7 w-7 items-center justify-center rounded-full border border-white/30 bg-black/60 shadow-sm transition-colors duration-200 group-hover/cb:border-white/70 group-hover/cb:bg-black/80"
                  >
                    <div
                      class="h-1.5 w-1.5 rounded-full bg-white/0 transition-colors duration-200 group-hover/cb:bg-white/50"
                    ></div>
                  </div>
                  <!-- 已选中：Apple 蓝实心底板 + 锐利白色勾选 -->
                  <div
                    v-else
                    class="flex h-7 w-7 items-center justify-center rounded-full border border-primary bg-primary text-primary-foreground transition-colors duration-200"
                  >
                    <Icon icon="ri:check-line" class="text-sm font-bold" />
                  </div>
                </label>
                <span
                  v-if="p.pinned"
                  class="inline-flex items-center gap-1 rounded-full border border-white/20 bg-primary px-2 py-0.5 text-[10px] font-medium text-white shadow-sm"
                >
                  <Icon icon="ri:pushpin-fill" class="text-[10px]" /> 置顶
                </span>
              </div>

              <span
                class="inline-flex items-center gap-1.5 rounded-full border border-white/20 bg-black/60 px-2.5 py-0.5 text-[11px] font-medium text-white/90 shadow-sm"
              >
                <span class="h-1.5 w-1.5 rounded-full" :class="statusBadge(p.status).dotCls"></span>
                {{ statusBadge(p.status).label }}
              </span>
            </div>
          </div>

          <!-- 卡片正文信息：分类与标签作为 Eyebrow 前置，单行滚动标题主导 -->
          <div class="flex flex-1 flex-col justify-between p-4">
            <div>
              <!-- 1. 分类与标签（Eyebrow 前置元信息，保齐网格行高，点击即快捷筛选） -->
              <div class="mb-2 flex min-h-[22px] flex-wrap items-center gap-1.5">
                <span
                  v-if="p.category"
                  class="inline-flex cursor-pointer items-center gap-1 rounded-md border border-white/5 bg-white/[0.06] px-2 py-0.5 text-[11px] font-medium text-text-secondary transition-colors hover:border-primary/40 hover:bg-white/[0.12] hover:text-foreground active:scale-95"
                  :title="`按分类「${p.category}」筛选`"
                  @click.stop="
                    selectedCategory = p.category;
                    page = 1;
                  "
                >
                  <Icon icon="ri:folder-3-line" class="text-xs text-text-tertiary" />
                  {{ p.category }}
                </span>
                <span
                  v-for="t in parseTags(p.tags).slice(0, 2)"
                  :key="t"
                  class="inline-flex cursor-pointer items-center rounded-md border border-white/5 bg-white/[0.04] px-1.5 py-0.5 text-[10px] text-text-tertiary transition-colors hover:border-primary/40 hover:bg-white/[0.1] hover:text-foreground active:scale-95"
                  :title="`按标签「#${t}」筛选`"
                  @click.stop="
                    selectedTag = t;
                    page = 1;
                  "
                >
                  #{{ t }}
                </span>
                <span
                  v-if="!p.category && parseTags(p.tags).length === 0"
                  class="inline-flex items-center rounded-md border border-white/[0.04] bg-white/[0.02] px-2 py-0.5 text-[10px] text-text-quaternary"
                >
                  未分类
                </span>
              </div>

              <!-- 完整标题通过提示可查看，点击进入编辑。 -->
              <div
                class="post-title-wrapper mb-2"
                :title="p.title"
                @click="router.push(`/posts/edit/${p.slug}`)"
              >
                <div class="post-title-scroll">
                  <h3
                    class="post-title-inner text-[15px] font-semibold leading-snug text-foreground transition-colors group-hover:text-primary"
                  >
                    {{ p.title }}
                  </h3>
                </div>
              </div>

              <!-- 3. 摘要信息（若有） -->
              <p v-if="p.excerpt" class="mb-2.5 line-clamp-2 text-xs leading-relaxed text-text-tertiary">
                {{ p.excerpt }}
              </p>
            </div>

            <!-- 时间、阅读量与常显操作。 -->
            <div class="flex items-center justify-between border-t border-white/[0.06] pt-3 text-xs">
              <div
                class="flex items-center gap-1.5 text-[11px] text-text-tertiary transition-colors hover:text-text-secondary"
                :title="`创建时间：${formatDateTime(p.created_at)}\n更新时间：${formatDateTime(p.updated_at)}`"
              >
                <span>{{ formatRelative(p.created_at) }}</span>
                <template v-if="p.views !== undefined">
                  <span class="text-text-quaternary" aria-hidden="true">·</span>
                  <span>{{ formatViews(p.views) }} 浏览</span>
                </template>
              </div>

              <div class="post-card-actions flex items-center gap-1">
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
                  class="post-card-action text-primary"
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
                  class="post-card-action post-card-action-danger text-state-error"
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
/* 封面视图保留内容预览，选中状态只用边界区分。 */
.post-card {
  transition: border-color var(--duration-fast) var(--ease-standard);
}
.post-card.is-selected {
  border-color: var(--color-primary);
}
.post-title-wrapper {
  overflow: hidden;
  cursor: pointer;
}
.post-title-inner {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 封面缺省图使用中性背景。 */
.post-card-placeholder {
  background: var(--color-secondary);
  color: color-mix(in srgb, var(--color-foreground) 22%, transparent);
  font-size: 1.5rem;
}

.post-card-actions {
  transition: opacity var(--duration-fast) var(--ease-standard);
}

.post-card-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: color-mix(in srgb, var(--color-secondary) 78%, transparent);
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.post-card-action:hover {
  border-color: var(--color-border-strong);
  background: var(--color-accent);
  color: var(--color-foreground);
}

.post-card-action:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.post-card-action-danger:hover {
  border-color: color-mix(in srgb, var(--color-state-error) 40%, transparent);
  background: color-mix(in srgb, var(--color-state-error) 12%, transparent);
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
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

@media (max-width: 1023px) {
  .action-icon-btn, .post-card-action { width: 44px; height: 44px; }
}
</style>
