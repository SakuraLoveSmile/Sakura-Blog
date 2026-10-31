<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRouter } from "vue-router";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { pageApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VEmpty, VLoading, VPagination, VFloatingDock, Dialog, Toast } from "@/components/ui";
import { formatRelative } from "@/lib/format";

interface PageItem {
  slug: string;
  title: string;
  status: "published" | "draft" | "trashed";
  is_system: boolean;
  views: number;
  created_at: string;
  updated_at: string;
}

const router = useRouter();
const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);
const viewMode = ref<"grid" | "table">("table");

// 搜索关键词（debounce 300ms）
const keyword = ref("");
const debouncedKeyword = ref("");
let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debouncedKeyword.value = keyword.value;
    page.value = 1;
  }, 300);
});

// 状态分段筛选
type StatusKey = "all" | "published" | "draft" | "trashed";
const selectedStatus = ref<StatusKey>("all");
const statusTabs: { key: StatusKey; label: string; icon: string }[] = [
  { key: "all", label: "全部页面", icon: "ri:pages-line" },
  { key: "published", label: "已发布", icon: "ri:checkbox-circle-line" },
  { key: "draft", label: "草稿", icon: "ri:draft-line" },
  { key: "trashed", label: "回收站", icon: "ri:delete-bin-line" },
];

const selectedSort = ref<"newest" | "oldest" | "views">("newest");

const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (selectedStatus.value !== "all") params.status = selectedStatus.value;
  if (debouncedKeyword.value.trim()) params.search = debouncedKeyword.value.trim();
  return params;
});

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
  queryKey: computed(() => queryKeys.pages(queryParams.value)),
  queryFn: () => pageApi.list(queryParams.value).then(({ data }) => data as { items: PageItem[]; total: number }),
});

// 客户端排序
const pages = computed(() => {
  const items = [...(queryData.value?.items || [])];
  if (selectedSort.value === "newest") {
    items.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
  } else if (selectedSort.value === "oldest") {
    items.sort((a, b) => new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime());
  } else if (selectedSort.value === "views") {
    items.sort((a, b) => (b.views || 0) - (a.views || 0));
  }
  return items;
});

const total = computed(() => queryData.value?.total || 0);

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = pages.value;
  let publishedCount = 0;
  let draftCount = 0;
  let systemCount = 0;
  for (const p of all) {
    if (p.status === "published") publishedCount++;
    else if (p.status === "draft") draftCount++;
    if (p.is_system) systemCount++;
  }
  return {
    total: total.value,
    publishedCount,
    draftCount,
    systemCount,
  };
});

// 批量选择
const selectedSlugs = ref<Set<string>>(new Set());

function toggleSelect(slug: string) {
  if (selectedSlugs.value.has(slug)) selectedSlugs.value.delete(slug);
  else selectedSlugs.value.add(slug);
}

function selectAll() {
  for (const p of pages.value) selectedSlugs.value.add(p.slug);
}

function clearSelection() {
  selectedSlugs.value.clear();
}

// 搜索、筛选或分页使旧选择失效时清除失效选择；加载中沿用上一批数据，不伪造空态
watch(
  () => (isLoading.value ? null : pages.value.map((p) => p.slug)),
  (slugs) => {
    if (!slugs) return;
    const visible = new Set(slugs);
    for (const slug of Array.from(selectedSlugs.value)) {
      if (!visible.has(slug)) selectedSlugs.value.delete(slug);
    }
  },
);

// Actions
function handleDelete(item: PageItem) {
  Dialog.warning({
    title: "确定将该页面移入回收站吗？",
    description: `「${item.title || item.slug}」将被移入回收站，后续可在回收站中恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await pageApi.delete(item.slug);
        selectedSlugs.value.delete(item.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.pages() });
        await refetch();
        Toast.success("已移入回收站");
      } catch {}
    },
  });
}

function handleRestore(item: PageItem) {
  Dialog.warning({
    title: "确定恢复该页面吗？",
    description: `「${item.title || item.slug}」将从回收站恢复至草稿/发布状态。`,
    onConfirm: async () => {
      try {
        await pageApi.restore(item.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.pages() });
        await refetch();
        Toast.success("恢复成功");
      } catch {}
    },
  });
}

function handlePurge(item: PageItem) {
  Dialog.warning({
    title: "确定彻底删除该页面吗？",
    description: `「${item.title || item.slug}」将被物理清除，此操作不可撤销！`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await pageApi.purge(item.slug);
        selectedSlugs.value.delete(item.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.pages() });
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
    title: `确定批量移入回收站选中的 ${slugs.length} 个页面吗？`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await Promise.all(slugs.map((s) => pageApi.delete(s)));
        clearSelection();
        await queryClient.invalidateQueries({ queryKey: queryKeys.pages() });
        await refetch();
        Toast.success("批量操作成功");
      } catch {}
    },
  });
}

function openPagePreview(slug: string) {
  window.open(`/${slug}`, "_blank");
}

function formatViews(n: number): string {
  if (!n) return "0";
  if (n >= 10000) return (n / 10000).toFixed(1) + "w";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return String(n);
}

function statusBadge(status: string) {
  switch (status) {
    case "published":
      return { cls: "bg-[#30D158]/10 text-[#30D158]", label: "已发布" };
    case "draft":
      return { cls: "bg-[#FF9500]/10 text-[#FF9500]", label: "草稿" };
    case "trashed":
      return { cls: "bg-[#FF453A]/10 text-[#FF453A]", label: "回收站" };
    default:
      return { cls: "bg-secondary text-text-tertiary", label: status };
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="独立页面" actions-layout="stack-below-lg">
      <template #icon>
        <Icon icon="ri:pages-line" class="text-primary" />
      </template>
      <template #subtitle>{{ total }} 个页面</template>
      <template #actions>
        <div class="flex w-full flex-col gap-2.5 lg:w-auto lg:flex-row lg:items-center">
          <div class="apple-search w-full lg:min-w-[200px] lg:max-w-[280px]">
            <Icon icon="ri:search-line" class="text-text-quaternary" />
            <input v-model="keyword" aria-label="搜索页面" placeholder="搜索页面标题或 Slug..." />
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
            <VButton type="primary" :route="{ path: '/pages/edit/new' }">
              <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
              新建页面
            </VButton>
          </div>
        </div>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 工具栏：状态分段 + 排序 + 视图模式 -->
      <div class="console-toolbar flex flex-wrap items-center justify-between gap-3">
        <!-- 状态分段 -->
        <div class="apple-segmented" role="group" aria-label="页面状态">
          <button
            v-for="st in statusTabs"
            :key="st.key"
            type="button"
            :class="{ active: selectedStatus === st.key }"
            :aria-pressed="selectedStatus === st.key"
            @click="
              selectedStatus = st.key;
              page = 1;
            "
          >
            <Icon :icon="st.icon" class="mr-1 inline-block" />
            {{ st.label }}
          </button>
        </div>

        <!-- 排序与视图 -->
        <div class="flex flex-wrap items-center gap-2">
          <select v-model="selectedSort" aria-label="本页排序" class="apple-select text-xs h-8">
            <option value="newest">最近更新</option>
            <option value="oldest">最早更新</option>
            <option value="views">浏览最多</option>
          </select>

          <button
            v-if="pages.length > 0"
            type="button"
            class="apple-filter-btn rounded-lg border border-border px-2.5 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            @click="selectedSlugs.size === pages.length ? clearSelection() : selectAll()"
          >
            {{ selectedSlugs.size === pages.length ? "取消全选" : "全选本页" }}
          </button>

          <div class="apple-segmented">
            <button
              type="button"
              :class="{ active: viewMode === 'grid' }"
              title="卡片视图"
              aria-label="卡片视图"
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

      <div class="console-summary" aria-label="页面摘要">
        <div class="stats-card">
          <span class="stats-label">匹配页面</span><span class="stats-value">{{ stats.total }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页已发布</span><span class="stats-value">{{ stats.publishedCount }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页草稿</span><span class="stats-value">{{ stats.draftCount }}</span>
        </div>
        <div class="stats-card">
          <span class="stats-label">本页系统内置</span><span class="stats-value">{{ stats.systemCount }}</span>
        </div>
      </div>

      <VLoading v-if="isLoading" />

      <VEmpty
        v-else-if="pages.length === 0"
        :title="keyword ? '未找到匹配的页面' : '暂无对应页面'"
        :message="keyword ? '换个搜索关键词试试' : '点击“新建页面”创建独立页面。'"
      />

      <!-- ============ 模式 1：卡片网格视图 ============ -->
      <div v-else-if="viewMode === 'grid'" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div
          v-for="p in pages"
          :key="p.slug"
          class="group relative flex flex-col justify-between rounded-2xl border border-border bg-card p-4 transition-colors duration-150 hover:border-border-strong sm:p-5"
          :class="{ 'border-primary ring-1 ring-primary bg-primary/5': selectedSlugs.has(p.slug) }"
        >
          <div>
            <!-- 顶部类型与状态徽章 -->
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <input
                  type="checkbox"
                  :checked="selectedSlugs.has(p.slug)"
                  class="h-4 w-4 rounded border-border text-primary cursor-pointer"
                  @change="toggleSelect(p.slug)"
                />
                <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="statusBadge(p.status).cls">
                  {{ statusBadge(p.status).label }}
                </span>
                <span
                  v-if="p.is_system"
                  class="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary"
                >
                  系统内置
                </span>
              </div>

              <!-- 浏览量徽标 -->
              <span class="flex items-center gap-1 font-mono text-xs text-text-quaternary">
                <Icon icon="ri:eye-line" />
                {{ formatViews(p.views) }}
              </span>
            </div>

            <!-- 标题与 Slug -->
            <div class="mt-3">
              <h3
                class="truncate text-base font-bold text-foreground transition-colors group-hover:text-primary cursor-pointer"
                @click="router.push(`/pages/edit/${p.slug}`)"
              >
                {{ p.title || p.slug }}
              </h3>
              <div class="mt-1 flex items-center gap-1.5 font-mono text-xs text-text-tertiary truncate">
                <span>/{{ p.slug }}</span>
              </div>
            </div>
          </div>

          <!-- 底部更新时间与操作 -->
          <div class="mt-5 flex items-center justify-between border-t border-border/60 pt-3 text-xs">
            <span class="text-text-quaternary">{{ formatRelative(p.updated_at) }}</span>

            <div class="flex items-center gap-1">
              <button
                v-if="p.status === 'published'"
                type="button"
                class="action-icon-btn"
                title="前台预览"
                @click="openPagePreview(p.slug)"
              >
                <Icon icon="ri:external-link-line" />
              </button>

              <button
                v-if="p.status !== 'trashed'"
                type="button"
                class="action-icon-btn"
                title="编辑页面"
                @click="router.push(`/pages/edit/${p.slug}`)"
              >
                <Icon icon="ri:edit-line" />
              </button>

              <button
                v-if="p.status === 'trashed'"
                type="button"
                class="action-icon-btn text-primary"
                title="从回收站恢复"
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
          </div>
        </div>
      </div>

      <!-- 行列表在窄屏内折行，保留同一份选择与操作状态。 -->
      <ul v-else class="console-records content-records" aria-label="页面列表">
        <li
          v-for="p in pages"
          :key="p.slug"
          class="content-record page-record"
          :class="{ 'is-selected': selectedSlugs.has(p.slug) }"
        >
          <label class="record-selection" :aria-label="`选择页面：${p.title || p.slug}`">
            <input
              type="checkbox"
              :aria-label="`选择页面：${p.title || p.slug}`"
              :checked="selectedSlugs.has(p.slug)"
              @change="toggleSelect(p.slug)"
            />
          </label>
          <div class="record-main">
            <div class="record-title-line">
              <button
                type="button"
                class="record-title"
                :title="p.title || p.slug"
                @click="router.push(`/pages/edit/${p.slug}`)"
              >
                {{ p.title || p.slug }}
              </button>
              <span v-if="p.is_system" class="record-note">系统内置</span>
            </div>
            <div class="record-metadata">
              <span class="record-slug" :title="`/${p.slug}`">/{{ p.slug }}</span>
            </div>
          </div>
          <div class="record-state">
            <span class="record-status" :class="statusBadge(p.status).cls">{{ statusBadge(p.status).label }}</span>
            <span class="record-updated" :title="`更新时间：${p.updated_at}`">{{ formatRelative(p.updated_at) }}</span>
            <span class="record-views">{{ formatViews(p.views) }} 浏览</span>
          </div>
          <div class="record-actions">
            <button
              v-if="p.status === 'published'"
              type="button"
              class="action-icon-btn"
              title="前台预览"
              @click="openPagePreview(p.slug)"
            >
              <Icon icon="ri:external-link-line" />
            </button>
            <button
              v-if="p.status !== 'trashed'"
              type="button"
              class="action-icon-btn"
              title="编辑页面"
              @click="router.push(`/pages/edit/${p.slug}`)"
            >
              <Icon icon="ri:edit-line" />
            </button>
            <button
              v-if="p.status === 'trashed'"
              type="button"
              class="action-icon-btn text-primary"
              title="从回收站恢复"
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
      title="已选页面"
      @clear="clearSelection"
    >
      <div class="flex items-center gap-2">
        <VButton size="sm" type="danger" @click="handleBatchDelete"> 批量移入回收站 </VButton>
      </div>
    </VFloatingDock>
  </div>
</template>

<style scoped>
/* 操作小按钮 */
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