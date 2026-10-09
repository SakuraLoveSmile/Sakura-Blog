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
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VFloatingDock,
  VPagination,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";
import ConsoleStatsCard from "@/components/ConsoleStatsCard.vue";
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
const viewMode = ref<"grid" | "table">(localStorage.getItem(VIEW_KEY) === "table" ? "table" : "grid");
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

function handleTitleEnter(e: MouseEvent) {
  const container = e.currentTarget as HTMLElement | null;
  if (!container) return;
  const inner = container.querySelector(".post-title-inner") as HTMLElement | null;
  if (!inner) return;
  const overflow = inner.scrollWidth - container.clientWidth;
  if (overflow > 2) {
    container.style.setProperty("--scroll-dist", `-${overflow + 8}px`);
    container.style.setProperty("--scroll-duration", `${Math.max(3, overflow / 30)}s`);
    container.classList.add("is-overflowing");
  }
}

function handleTitleLeave(e: MouseEvent) {
  const container = e.currentTarget as HTMLElement | null;
  if (!container) return;
  container.classList.remove("is-overflowing");
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文章管理" :hide-on-scroll="false" actions-layout="stack-below-lg">
      <template #icon>
        <Icon icon="ri:article-line" class="text-primary" />
      </template>
      <template #subtitle>撰写与管理博客博文，支持分类标签筛选、版本追踪与临时分享链接</template>
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
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <ConsoleStatsCard
          label="匹配文章"
          :value="stats.total"
          sub="当前筛选下的总数"
          icon="ri:article-line"
          color="primary"
        />
        <ConsoleStatsCard
          label="本页已发布"
          :value="stats.pubCount"
          sub="篇在线查阅"
          icon="ri:checkbox-circle-line"
          color="success"
          value-class="text-state-success"
        />
        <ConsoleStatsCard
          label="本页草稿"
          :value="stats.draftCount"
          sub="篇未发布"
          icon="ri:draft-line"
          color="warning"
          value-class="text-state-warning"
        />
        <ConsoleStatsCard
          label="本页置顶"
          :value="stats.pinCount"
          sub="篇精选置顶"
          icon="ri:pushpin-line"
          color="purple"
        />
      </div>

      <!-- 工具栏：状态分段 + 筛选 + 视图模式 -->
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-card p-3">
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
              title="表格视图"
              aria-label="表格视图"
              :aria-pressed="viewMode === 'table'"
              @click="viewMode = 'table'"
            >
              <Icon icon="ri:table-line" />
            </button>
          </div>
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
        :title="hasFilters ? '未找到匹配的文章' : '开始你的第一篇文章'"
        :message="hasFilters ? '可以调整筛选条件，或清除筛选查看全部文章。' : '记录一个想法，从草稿开始慢慢写。'"
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
          class="post-card group relative flex flex-col overflow-hidden rounded-2xl border border-border bg-card hover:-translate-y-0.5 hover:border-border-strong hover:shadow-lg"
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
              class="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-105"
            />
            <div
              v-else
              class="post-card-placeholder flex h-full w-full items-center justify-center transition-transform duration-500 ease-out group-hover:scale-105"
            >
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
                    class="flex h-7 w-7 items-center justify-center rounded-full border border-primary bg-primary text-white shadow-[0_0_10px_rgba(46,141,255,0.6)] transition-colors duration-200"
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
                <span
                  class="h-1.5 w-1.5 rounded-full shadow-[0_0_6px_currentColor]"
                  :class="statusBadge(p.status).dotCls"
                ></span>
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

              <!-- 2. 单行自适应溢出平滑滚动标题 -->
              <div
                class="post-title-wrapper mb-2"
                :title="p.title"
                @mouseenter="handleTitleEnter($event)"
                @mouseleave="handleTitleLeave($event)"
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

            <!-- 底部时间、阅读量与操作：默认半透明弱化呈现，悬停/聚焦时高亮 -->
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

              <div
                class="post-card-actions flex items-center gap-1 opacity-60 transition-opacity duration-200 sm:opacity-40 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100"
              >
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

      <!-- ============ 模式 2：表格列表视图 ============ -->
      <VCard v-else>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[800px] text-left">
            <thead>
              <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                <th class="w-12 px-4 py-3 text-center">
                  <!-- 空数据时不渲染选择入口；占位单元格保留以避免列布局跳动 -->
                  <input
                    v-if="posts.length > 0"
                    type="checkbox"
                    :checked="selectedSlugs.size === posts.length"
                    class="h-4 w-4 rounded border-border text-primary"
                    @change="selectedSlugs.size === posts.length ? clearSelection() : selectAll()"
                  />
                </th>
                <th class="px-4 py-3">文章标题</th>
                <th class="px-4 py-3">分类 / 标签</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">更新时间</th>
                <th class="w-40 px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/60">
              <tr
                v-for="p in posts"
                :key="p.slug"
                class="cursor-pointer transition-colors hover:bg-secondary/40"
                :class="{ 'bg-primary/5': selectedSlugs.has(p.slug) }"
                @click="router.push(`/posts/edit/${p.slug}`)"
              >
                <td class="px-4 py-3.5 text-center" @click.stop>
                  <input
                    type="checkbox"
                    :checked="selectedSlugs.has(p.slug)"
                    class="h-4 w-4 rounded border-border text-primary"
                    @change="toggleSelect(p.slug)"
                  />
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-3">
                    <img
                      v-if="p.cover"
                      :src="p.cover"
                      class="h-10 w-14 shrink-0 rounded-lg border border-border object-cover bg-secondary"
                      alt=""
                    />
                    <div class="min-w-0">
                      <div class="font-semibold text-foreground text-sm flex items-center gap-1.5">
                        <span class="truncate max-w-[320px]">{{ p.title }}</span>
                        <span v-if="p.pinned" class="rounded bg-primary px-1.5 py-0.2 text-[9px] font-bold text-white"
                          >置顶</span
                        >
                      </div>
                      <div class="text-xs font-mono text-text-quaternary">/{{ p.slug }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-text-secondary">
                  <div class="flex flex-wrap items-center gap-1">
                    <span v-if="p.category" class="rounded bg-secondary px-1.5 py-0.5 text-[10px]">
                      {{ p.category }}
                    </span>
                    <span v-for="t in parseTags(p.tags).slice(0, 2)" :key="t" class="text-text-quaternary text-[11px]">
                      #{{ t }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs">
                  <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="statusBadge(p.status).cls">
                    {{ statusBadge(p.status).label }}
                  </span>
                </td>
                <td class="px-4 py-3.5 text-xs text-text-quaternary">
                  {{ formatRelative(p.updated_at) }}
                </td>
                <td class="px-4 py-3.5 text-right" @click.stop>
                  <div class="flex items-center justify-end gap-1">
                    <button type="button" class="action-icon-btn" title="分享链接" @click="openShareModal(p)">
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
                      title="编辑"
                      @click="router.push(`/posts/edit/${p.slug}`)"
                    >
                      <Icon icon="ri:edit-line" />
                    </button>
                    <button
                      v-if="p.status === 'trashed'"
                      type="button"
                      class="action-icon-btn text-primary"
                      title="恢复"
                      @click="handleRestore(p)"
                    >
                      <Icon icon="ri:restart-line" />
                    </button>
                    <button
                      v-if="p.status !== 'trashed'"
                      type="button"
                      class="action-icon-btn hover:text-state-error"
                      title="移入回收站"
                      @click="handleDelete(p)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                    <button
                      v-if="p.status === 'trashed'"
                      type="button"
                      class="action-icon-btn text-state-error"
                      title="彻底删除"
                      @click="handlePurge(p)"
                    >
                      <Icon icon="ri:delete-bin-7-line" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </VCard>

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
/* Apple 深色卡片微光边框与选中光效 */
.post-card {
  position: relative;
  transition:
    transform var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}

.post-card::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  box-shadow: inset 0 1px 0 0 rgba(255, 255, 255, 0.08);
  z-index: 10;
}

.post-card.is-selected {
  border-color: var(--color-primary);
  box-shadow:
    0 0 0 1px var(--color-primary),
    0 0 24px rgba(46, 141, 255, 0.18);
  background: color-mix(in srgb, var(--color-primary) 4%, var(--color-card));
}

/* 标题单行与自适应溢出滚动 */
.post-title-wrapper {
  position: relative;
  overflow: hidden;
  white-space: nowrap;
  cursor: pointer;
  mask-image: linear-gradient(90deg, #000 calc(100% - 1.2rem), transparent 100%);
}

.post-title-wrapper.is-overflowing {
  mask-image: none;
}

.post-title-scroll {
  display: inline-block;
  white-space: nowrap;
}

.post-title-wrapper.is-overflowing .post-title-inner {
  animation: title-marquee var(--scroll-duration, 4s) cubic-bezier(0.45, 0.05, 0.55, 0.95) infinite alternate;
  will-change: transform;
}

@keyframes title-marquee {
  0%,
  20% {
    transform: translateX(0);
  }
  80%,
  100% {
    transform: translateX(var(--scroll-dist, 0px));
  }
}

.post-title-inner {
  display: inline-block;
  white-space: nowrap;
}

/* 海报卡片媒体占位：克制的暗色渐变承托图标，不抢标题层级 */
.post-card-placeholder {
  background: linear-gradient(135deg, var(--color-secondary), var(--color-background));
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

@media (max-width: 640px) {
  .post-card-action {
    width: 32px;
    height: 32px;
  }
}
</style>
