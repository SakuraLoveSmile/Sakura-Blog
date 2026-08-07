<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { pageApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import { Dialog, Toast } from "@/components/ui";

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
const route = useRoute();
const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);
const viewMode = ref<"list" | "grid">("list");

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

// Filters
const validStatuses = ["published", "draft", "trashed"];
function statusFromQuery(): string | undefined {
  const s = route.query.status;
  return typeof s === "string" && validStatuses.includes(s) ? s : undefined;
}
const selectedStatus = ref<string | undefined>(statusFromQuery());
watch(
  () => route.query.status,
  () => {
    const s = statusFromQuery();
    if (s && s !== selectedStatus.value) {
      selectedStatus.value = s;
      page.value = 1;
    }
  }
);

const statusItems = [
  { label: "全部状态", value: undefined },
  { label: "已发布", value: "published" },
  { label: "草稿", value: "draft" },
  { label: "回收站", value: "trashed" },
];

const sortItems = [
  { label: "最近更新", value: "newest" },
  { label: "最早更新", value: "oldest" },
  { label: "浏览最多", value: "views" },
];
const selectedSort = ref<string>("newest");

const statusFilterOpen = ref(false);
const sortFilterOpen = ref(false);

const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (selectedStatus.value) params.status = selectedStatus.value;
  if (debouncedKeyword.value.trim()) params.search = debouncedKeyword.value.trim();
  return params;
});

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
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
    items.sort((a, b) => b.views - a.views);
  }
  return items;
});
const total = computed(() => queryData.value?.total || 0);
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)));

// Selection
const selectedSlugs = ref<string[]>([]);
const menuOpenSlug = ref<string | null>(null);

const hasFilters = computed(
  () => selectedStatus.value !== undefined || keyword.value !== ""
);

const checkedAll = computed(() => pages.value.length > 0 && selectedSlugs.value.length === pages.value.length);

function handleCheckAllChange(checked: boolean) {
  selectedSlugs.value = checked ? pages.value.map((p) => p.slug) : [];
}

function toggleSelection(slug: string) {
  if (selectedSlugs.value.includes(slug)) {
    selectedSlugs.value = selectedSlugs.value.filter((s) => s !== slug);
  } else {
    selectedSlugs.value = [...selectedSlugs.value, slug];
  }
}

function handleClearFilters() {
  selectedStatus.value = undefined;
  keyword.value = "";
  debouncedKeyword.value = "";
  page.value = 1;
}

const statusLabel = computed(() => statusItems.find((i) => i.value === selectedStatus.value)?.label || "状态");
const sortLabel = computed(() => sortItems.find((i) => i.value === selectedSort.value)?.label || "排序");

const isTrashed = computed(() => selectedStatus.value === "trashed");

// Actions
function handleDelete(item: PageItem) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要删除该页面吗？",
    description: "该操作会将页面放入回收站，后续可以从回收站恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await pageApi.delete(item.slug);
      queryClient.invalidateQueries({ queryKey: ["pages"] });
      Toast.success("删除成功");
    },
  });
}

function handleDeleteInBatch() {
  Dialog.warning({
    title: "确定要删除选中的页面吗？",
    description: "该操作会将页面放入回收站，后续可以从回收站恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await Promise.all(selectedSlugs.value.map((slug) => pageApi.delete(slug)));
      selectedSlugs.value = [];
      queryClient.invalidateQueries({ queryKey: ["pages"] });
      Toast.success("删除成功");
    },
  });
}

function handleRestore(item: PageItem) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要恢复该页面吗？",
    onConfirm: async () => {
      await pageApi.restore(item.slug);
      queryClient.invalidateQueries({ queryKey: ["pages"] });
      Toast.success("恢复成功");
    },
  });
}

function handlePurge(item: PageItem) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要彻底删除该页面吗？",
    description: "该操作不可恢复！",
    confirmType: "danger",
    onConfirm: async () => {
      await pageApi.purge(item.slug);
      queryClient.invalidateQueries({ queryKey: ["pages"] });
      Toast.success("已彻底删除");
    },
  });
}

function formatDate(dateStr: string): string {
  if (!dateStr) return "";
  return dateStr.slice(0, 10);
}

function formatViews(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + "w";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return String(n);
}

function openPage(slug: string) {
  window.open(`/${slug}`, "_blank");
}

function closeMenus() {
  menuOpenSlug.value = null;
  statusFilterOpen.value = false;
  sortFilterOpen.value = false;
}

function getPageIconColor(p: PageItem) {
  if (p.status === "draft") return "orange";
  if (p.status === "trashed") return "red";
  if (p.is_system) return "blue";
  return "green";
}

function getStatusText(p: PageItem) {
  if (p.status === "published") return "已发布";
  if (p.status === "draft") return "草稿";
  return "已删除";
}

function getStatusClass(p: PageItem) {
  if (p.status === "published") return "text-green";
  if (p.status === "draft") return "text-orange";
  return "text-red";
}
</script>

<template>
  <div class="page-list-apple" @click="closeMenus">
    <!-- Header -->
    <div class="apple-page-header">
      <div class="header-left">
        <div class="header-icon">
          <Icon icon="ri:pages-line" />
        </div>
        <h1 class="header-title">页面</h1>
      </div>
      <div class="header-actions">
        <button class="apple-new-btn" @click="router.push('/pages/edit/new')">
          <Icon icon="ri:add-line" class="btn-icon" />
          新建页面
        </button>
      </div>
    </div>

    <!-- Content -->
    <div class="content-wrap">
      <div class="apple-card">
        <!-- Filter Bar -->
        <div class="card-header">
          <div class="filter-left">
            <div class="custom-checkbox" :class="{ checked: checkedAll }" @click.stop="handleCheckAllChange(!checkedAll)">
              <Icon v-if="checkedAll" icon="ri:check-line" />
            </div>
            <div v-if="!selectedSlugs.length" class="filter-search">
              <Icon icon="ri:search-line" class="search-icon" />
              <input
                v-model="keyword"
                type="text"
                class="search-input"
                placeholder="搜索页面标题或 slug..."
              />
            </div>
            <div v-else class="bulk-actions">
              <button class="bulk-delete-btn" @click="handleDeleteInBatch">
                <Icon icon="ri:delete-bin-line" />
                删除
              </button>
              <span class="bulk-count">已选择 {{ selectedSlugs.length }} 项</span>
            </div>
          </div>
          <div class="filter-right">
            <!-- Status Filter -->
            <div class="filter-dropdown-wrap" @click.stop>
              <button class="filter-btn" :class="{ active: selectedStatus !== undefined }" @click="statusFilterOpen = !statusFilterOpen; sortFilterOpen = false">
                {{ statusLabel }}
                <Icon icon="ri:arrow-down-s-line" />
              </button>
              <Transition name="dropdown">
                <div v-if="statusFilterOpen" class="filter-dropdown">
                  <button
                    v-for="item in statusItems"
                    :key="String(item.value)"
                    class="dropdown-item"
                    :class="{ active: selectedStatus === item.value }"
                    @click="selectedStatus = item.value; statusFilterOpen = false; page = 1"
                  >
                    {{ item.label }}
                    <Icon v-if="selectedStatus === item.value" icon="ri:check-line" class="check-icon" />
                  </button>
                </div>
              </Transition>
            </div>

            <!-- Sort Filter -->
            <div class="filter-dropdown-wrap" @click.stop>
              <button class="filter-btn" @click="sortFilterOpen = !sortFilterOpen; statusFilterOpen = false">
                {{ sortLabel }}
                <Icon icon="ri:arrow-down-s-line" />
              </button>
              <Transition name="dropdown">
                <div v-if="sortFilterOpen" class="filter-dropdown">
                  <button
                    v-for="item in sortItems"
                    :key="item.value"
                    class="dropdown-item"
                    :class="{ active: selectedSort === item.value }"
                    @click="selectedSort = item.value; sortFilterOpen = false"
                  >
                    {{ item.label }}
                    <Icon v-if="selectedSort === item.value" icon="ri:check-line" class="check-icon" />
                  </button>
                </div>
              </Transition>
            </div>

            <!-- Refresh -->
            <button class="icon-btn" @click="refetch()">
              <Icon icon="ri:refresh-line" :class="{ 'animate-spin': isFetching }" />
            </button>

            <!-- View Toggle -->
            <div class="view-toggle">
              <button class="toggle-btn" :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'" title="列表视图">
                <Icon icon="ri:list-check" />
              </button>
              <button class="toggle-btn" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'" title="网格视图">
                <Icon icon="ri:grid-line" />
              </button>
            </div>
          </div>
        </div>

        <!-- Active Filters Tags -->
        <div v-if="hasFilters" class="active-filters">
          <span v-if="selectedStatus !== undefined" class="filter-tag">
            状态：{{ statusLabel }}
            <button @click="selectedStatus = undefined; page = 1"><Icon icon="ri:close-line" /></button>
          </span>
          <span v-if="keyword" class="filter-tag">
            搜索：{{ keyword }}
            <button @click="keyword = ''"><Icon icon="ri:close-line" /></button>
          </span>
          <button class="clear-filters" @click="handleClearFilters">清除全部</button>
        </div>

        <!-- Loading -->
        <div v-if="isLoading" class="loading-wrap">
          <div class="apple-spinner"></div>
        </div>

        <!-- Empty -->
        <div v-else-if="!pages.length" class="empty-wrap">
          <div class="empty-icon">
            <Icon icon="ri:file-list-3-line" />
          </div>
          <h3 class="empty-title">暂无页面</h3>
          <p class="empty-desc">你可以尝试刷新或者创建新页面</p>
          <div class="empty-actions">
            <button class="apple-btn-secondary" @click="() => refetch()">刷新</button>
            <button class="apple-btn-primary" @click="router.push('/pages/edit/new')">
              <Icon icon="ri:add-line" class="btn-icon" />
              新建页面
            </button>
          </div>
        </div>

        <!-- List View -->
        <div v-else-if="viewMode === 'list'" class="page-list-view">
          <div
            v-for="p in pages"
            :key="p.slug"
            class="page-row"
            :class="{ selected: selectedSlugs.includes(p.slug) }"
            @click="router.push(`/pages/edit/${p.slug}`)"
          >
            <div class="row-left" @click.stop>
              <div
                class="custom-checkbox small"
                :class="{ checked: selectedSlugs.includes(p.slug) }"
                @click="toggleSelection(p.slug)"
              >
                <Icon v-if="selectedSlugs.includes(p.slug)" icon="ri:check-line" />
              </div>
            </div>
            <div class="page-icon-box" :class="getPageIconColor(p)">
              <Icon icon="ri:file-text-line" />
            </div>
            <div class="page-info">
              <div class="page-title-row">
                <span class="page-title">{{ p.title || p.slug }}</span>
                <button v-if="p.is_system" class="system-tag">系统页面</button>
              </div>
              <div class="page-meta">
                <span class="meta-item">
                  <Icon icon="ri:link" />
                  /{{ p.slug }}
                </span>
                <span class="meta-dot"></span>
                <span class="meta-item">
                  <Icon icon="ri:eye-line" />
                  {{ formatViews(p.views) }}
                </span>
              </div>
            </div>
            <div class="row-right">
              <span class="status-text" :class="getStatusClass(p)">{{ getStatusText(p) }}</span>
              <span class="page-date">{{ formatDate(p.updated_at) }}</span>
              <div class="row-actions" @click.stop>
                <button class="action-btn" @click="menuOpenSlug = menuOpenSlug === p.slug ? null : p.slug">
                  <Icon icon="ri:more-fill" />
                </button>
                <Transition name="dropdown">
                  <div v-if="menuOpenSlug === p.slug" class="action-menu">
                    <button class="menu-item" @click="openPage(p.slug)">
                      <Icon icon="ri:external-link-line" />
                      在前台打开
                    </button>
                    <template v-if="isTrashed">
                      <button class="menu-item" @click="handleRestore(p)">
                        <Icon icon="ri:arrow-go-back-line" />
                        恢复
                      </button>
                      <div class="menu-divider"></div>
                      <button class="menu-item danger" @click="handlePurge(p)">
                        <Icon icon="ri:delete-bin-line" />
                        彻底删除
                      </button>
                    </template>
                    <template v-else>
                      <div class="menu-divider"></div>
                      <button class="menu-item danger" @click="handleDelete(p)">
                        <Icon icon="ri:delete-bin-line" />
                        删除
                      </button>
                    </template>
                  </div>
                </Transition>
              </div>
            </div>
          </div>
        </div>

        <!-- Grid View -->
        <div v-else class="page-grid-view">
          <div
            v-for="p in pages"
            :key="p.slug"
            class="grid-card"
            :class="{ selected: selectedSlugs.includes(p.slug) }"
            @click="router.push(`/pages/edit/${p.slug}`)"
          >
            <div class="grid-checkbox" @click.stop>
              <div
                class="custom-checkbox"
                :class="{ checked: selectedSlugs.includes(p.slug) }"
                @click="toggleSelection(p.slug)"
              >
                <Icon v-if="selectedSlugs.includes(p.slug)" icon="ri:check-line" />
              </div>
            </div>
            <div class="grid-icon" :class="getPageIconColor(p)">
              <Icon icon="ri:file-text-line" />
            </div>
            <h3 class="grid-title">{{ p.title || p.slug }}</h3>
            <p class="grid-slug">/{{ p.slug }}</p>
            <div class="grid-footer">
              <span class="status-badge" :class="getStatusClass(p)">{{ getStatusText(p) }}</span>
              <span class="grid-views">
                <Icon icon="ri:eye-line" />
                {{ formatViews(p.views) }}
              </span>
            </div>
            <div class="grid-date">{{ formatDate(p.updated_at) }}</div>
            <div class="grid-actions" @click.stop>
              <button class="grid-action-btn" @click="openPage(p.slug)" title="在前台打开">
                <Icon icon="ri:external-link-line" />
              </button>
              <button v-if="!isTrashed" class="grid-action-btn danger" @click="handleDelete(p)" title="删除">
                <Icon icon="ri:delete-bin-line" />
              </button>
              <template v-else>
                <button class="grid-action-btn" @click="handleRestore(p)" title="恢复">
                  <Icon icon="ri:arrow-go-back-line" />
                </button>
                <button class="grid-action-btn danger" @click="handlePurge(p)" title="彻底删除">
                  <Icon icon="ri:delete-bin-7-line" />
                </button>
              </template>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div v-if="total > 0 && !isLoading" class="pagination-bar">
          <span class="pagination-info">
            共 <strong>{{ total }}</strong> 个页面
          </span>
          <div class="pagination-controls">
            <button class="page-btn" :disabled="page <= 1" @click="page--">
              <Icon icon="ri:arrow-left-s-line" />
            </button>
            <template v-for="n in totalPages" :key="n">
              <button
                v-if="n === 1 || n === totalPages || (n >= page - 1 && n <= page + 1)"
                class="page-btn"
                :class="{ active: page === n }"
                @click="page = n"
              >
                {{ n }}
              </button>
              <span v-else-if="n === page - 2 || n === page + 2" class="page-ellipsis">...</span>
            </template>
            <button class="page-btn" :disabled="page >= totalPages" @click="page++">
              <Icon icon="ri:arrow-right-s-line" />
            </button>
            <select v-model="size" class="page-size-select" @change="page = 1">
              <option :value="10">10/页</option>
              <option :value="20">20/页</option>
              <option :value="50">50/页</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-list-apple {
  background: var(--color-background);
  color: var(--color-foreground);
  min-height: 100%;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro Text", "PingFang SC", sans-serif;
  -webkit-font-smoothing: antialiased;
}

/* Header */
.apple-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
  background: color-mix(in srgb, #000 80%, transparent);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-bottom: 1px solid var(--color-border);
  position: sticky;
  top: 0;
  z-index: 50;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-icon {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-primary);
  font-size: 22px;
}

.header-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.3px;
  margin: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.apple-new-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 18px;
  height: 32px;
  border-radius: var(--radius-full);
  background: var(--color-primary);
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s var(--ease-standard);
  box-shadow: var(--shadow-md);
}

.apple-new-btn:hover {
  background: var(--color-primary-hover);
  transform: translateY(-0.5px);
  box-shadow: var(--shadow-lg);
}

.apple-new-btn:active {
  transform: translateY(0);
}

.btn-icon {
  width: 14px;
  height: 14px;
}

/* Content */
.content-wrap {
  padding: 20px 24px 24px;
}

.apple-card {
  background: color-mix(in srgb, var(--color-card) 72%, transparent);
  backdrop-filter: blur(40px) saturate(180%);
  -webkit-backdrop-filter: blur(40px) saturate(180%);
  border-radius: var(--radius-lg);
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  overflow: hidden;
  position: relative;
  animation: fadeInUp 0.4s var(--ease-standard) both;
}

.apple-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 6%, transparent), transparent);
  pointer-events: none;
}

@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Filter Bar */
.card-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border);
  gap: 12px;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
}

.filter-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

/* Checkbox */
.custom-checkbox {
  width: 18px;
  height: 18px;
  border-radius: var(--radius-xs);
  border: 1.5px solid var(--color-border-strong);
  background: var(--color-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.18s var(--ease-standard);
  font-size: 12px;
  color: var(--color-foreground);
}

.custom-checkbox.small {
  width: 16px;
  height: 16px;
  font-size: 10px;
}

.custom-checkbox:hover {
  border-color: var(--color-primary);
}

.custom-checkbox.checked {
  background: var(--color-primary);
  border-color: var(--color-primary);
}

/* Search */
.filter-search {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--color-secondary);
  border: 1px solid transparent;
  border-radius: var(--radius-base);
  padding: 7px 12px;
  width: 320px;
  transition: all 0.2s var(--ease-standard);
}

.filter-search:focus-within {
  border-color: color-mix(in srgb, var(--color-ring) 40%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-ring) 12%, transparent);
}

.search-icon {
  width: 14px;
  height: 14px;
  color: var(--color-muted-foreground);
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--color-foreground);
  font-size: 13px;
  width: 100%;
  font-weight: 400;
}

.search-input::placeholder {
  color: var(--color-muted-foreground);
}

/* Bulk Actions */
.bulk-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.bulk-delete-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-state-error) 15%, transparent);
  color: var(--color-state-error);
  font-size: 12px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.18s;
}

.bulk-delete-btn:hover {
  background: color-mix(in srgb, var(--color-state-error) 25%, transparent);
}

.bulk-count {
  font-size: 13px;
  color: var(--color-muted-foreground);
}

/* Filter Buttons */
.filter-dropdown-wrap {
  position: relative;
}

.filter-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 7px 12px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: var(--color-secondary);
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s;
  border: 1px solid transparent;
}

.filter-btn:hover,
.filter-btn.active {
  background: var(--color-accent);
  color: var(--color-foreground);
}

.filter-btn svg {
  width: 13px;
  height: 13px;
}

.icon-btn {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--color-muted-foreground);
  border: none;
  cursor: pointer;
  transition: all 0.18s;
}

.icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.icon-btn svg {
  width: 16px;
  height: 16px;
}

.animate-spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* View Toggle */
.view-toggle {
  display: flex;
  background: var(--color-secondary);
  border-radius: var(--radius-sm);
  padding: 2px;
}

.toggle-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: var(--color-muted-foreground);
  cursor: pointer;
  transition: all 0.18s;
  background: transparent;
  border: none;
}

.toggle-btn:hover {
  color: var(--color-text-secondary);
}

.toggle-btn.active {
  background: var(--color-accent);
  color: var(--color-foreground);
}

.toggle-btn svg {
  width: 15px;
  height: 15px;
}

/* Dropdown */
.filter-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 140px;
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-base);
  padding: 6px;
  z-index: 40;
  box-shadow: 0 12px 32px -8px color-mix(in srgb, #000 60%, transparent);
}

.dropdown-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 10px;
  border-radius: 6px;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 13px;
  border: none;
  cursor: pointer;
  transition: all 0.15s;
  text-align: left;
}

.dropdown-item:hover {
  background: color-mix(in srgb, #fff 6%, transparent);
  color: var(--color-foreground);
}

.dropdown-item.active {
  color: var(--color-primary);
}

.check-icon {
  width: 14px;
  height: 14px;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity var(--duration-fast) var(--ease-standard), transform var(--duration-fast) var(--ease-standard);
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

/* Active Filters */
.active-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--color-border);
  flex-wrap: wrap;
}

.filter-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  font-size: 11px;
  font-weight: 500;
}

.filter-tag button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: transparent;
  border: none;
  color: inherit;
  cursor: pointer;
  padding: 0;
  font-size: 12px;
  opacity: 0.7;
  transition: opacity 0.15s;
}

.filter-tag button:hover {
  opacity: 1;
}

.clear-filters {
  font-size: 12px;
  color: var(--color-muted-foreground);
  background: transparent;
  border: none;
  cursor: pointer;
  text-decoration: underline;
  padding: 4px 0;
}

.clear-filters:hover {
  color: var(--color-text-secondary);
}

/* Loading */
.loading-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
}

.apple-spinner {
  width: 24px;
  height: 24px;
  border: 2.5px solid color-mix(in srgb, var(--color-muted-foreground) 20%, transparent);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

/* Empty */
.empty-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 60px 20px;
}

.empty-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, #fff 6%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  color: var(--color-muted-foreground);
  margin-bottom: 16px;
}

.empty-title {
  font-size: 17px;
  font-weight: 600;
  color: var(--color-foreground);
  margin: 0 0 6px;
}

.empty-desc {
  font-size: 13px;
  color: var(--color-muted-foreground);
  margin: 0 0 20px;
}

.empty-actions {
  display: flex;
  gap: 10px;
}

.apple-btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border-radius: var(--radius-full);
  background: var(--color-primary);
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
}

.apple-btn-primary:hover {
  background: var(--color-primary-hover);
}

.apple-btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border-radius: var(--radius-full);
  background: color-mix(in srgb, #fff 10%, transparent);
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 500;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
}

.apple-btn-secondary:hover {
  background: color-mix(in srgb, #fff 18%, transparent);
}

/* List View */
.page-list-view {
  padding: 0;
}

.page-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--color-border);
  cursor: pointer;
  transition: all 0.18s var(--ease-standard);
  position: relative;
}

.page-row:last-child {
  border-bottom: none;
}

.page-row:hover {
  background: color-mix(in srgb, #fff 4%, transparent);
}

.page-row.selected {
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}

.page-row.selected::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--color-primary);
}

.row-left {
  display: flex;
  align-items: center;
}

.page-icon-box {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-base);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
  font-size: 20px;
}

.page-icon-box::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, color-mix(in srgb, #fff 4%, transparent) 0%, transparent 50%);
  pointer-events: none;
  border-radius: var(--radius-base);
}

.page-icon-box svg {
  position: relative;
  z-index: 1;
}

.page-icon-box.blue { color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 12%, transparent); }
.page-icon-box.green { color: var(--color-state-success); background: color-mix(in srgb, var(--color-state-success) 12%, transparent); }
.page-icon-box.orange { color: var(--color-state-warning); background: color-mix(in srgb, var(--color-state-warning) 12%, transparent); }
.page-icon-box.red { color: var(--color-state-error); background: color-mix(in srgb, var(--color-state-error) 12%, transparent); }
.page-icon-box.purple { color: #BF5AF2; background: color-mix(in srgb, var(--color-chart-5) 12%, transparent); }

.page-info {
  flex: 1;
  min-width: 0;
}

.page-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.page-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.3;
}

.system-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: var(--radius-full);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  font-size: 10px;
  font-weight: 600;
  border: none;
  line-height: 1.5;
  flex-shrink: 0;
}

.page-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.meta-item svg {
  width: 12px;
  height: 12px;
}

.meta-dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--color-text-quaternary);
  flex-shrink: 0;
}

.row-right {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-shrink: 0;
}

.status-text {
  font-size: 12px;
  font-weight: 500;
  flex-shrink: 0;
  min-width: 48px;
  text-align: right;
}

.status-text.text-green { color: var(--color-state-success); }
.status-text.text-orange { color: var(--color-state-warning); }
.status-text.text-red { color: var(--color-state-error); }

.page-date {
  font-size: 12px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
  min-width: 78px;
  text-align: right;
}

.row-actions {
  position: relative;
}

.action-btn {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-muted-foreground);
  cursor: pointer;
  transition: all 0.18s;
  background: transparent;
  border: none;
  opacity: 0;
}

.page-row:hover .action-btn {
  opacity: 1;
}

.action-btn:hover {
  background: color-mix(in srgb, #fff 8%, transparent);
  color: var(--color-foreground);
}

.action-btn svg {
  width: 16px;
  height: 16px;
}

.action-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  min-width: 140px;
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-base);
  padding: 6px;
  z-index: 40;
  box-shadow: 0 12px 32px -8px color-mix(in srgb, #000 60%, transparent);
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border-radius: 6px;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 13px;
  border: none;
  cursor: pointer;
  transition: all 0.15s;
  text-align: left;
}

.menu-item:hover {
  background: color-mix(in srgb, #fff 6%, transparent);
  color: var(--color-foreground);
}

.menu-item.danger {
  color: var(--color-state-error);
}

.menu-item.danger:hover {
  background: color-mix(in srgb, var(--color-state-error) 12%, transparent);
  color: var(--color-state-error);
}

.menu-item svg {
  width: 15px;
  height: 15px;
}

.menu-divider {
  height: 1px;
  background: var(--color-border);
  margin: 4px 0;
}

/* Grid View */
.page-grid-view {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  padding: 20px;
}

.grid-card {
  background: color-mix(in srgb, var(--color-accent) 60%, transparent);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px;
  cursor: pointer;
  transition: all 0.25s var(--ease-standard);
  position: relative;
  overflow: hidden;
}

.grid-card:hover {
  background: color-mix(in srgb, var(--color-accent) 60%, transparent);
  transform: translateY(-2px);
  border-color: var(--color-border);
  box-shadow: 0 8px 24px -8px color-mix(in srgb, #000 50%, transparent);
}

.grid-card.selected {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
}

.grid-checkbox {
  position: absolute;
  top: 12px;
  left: 12px;
  opacity: 0;
  transition: opacity 0.15s;
}

.grid-card:hover .grid-checkbox,
.grid-card.selected .grid-checkbox {
  opacity: 1;
}

.grid-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  margin-bottom: 14px;
}

.grid-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-foreground);
  margin: 0 0 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.grid-slug {
  font-size: 12px;
  color: var(--color-muted-foreground);
  margin: 0 0 16px;
}

.grid-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.status-badge {
  font-size: 11px;
  font-weight: 500;
}

.status-badge.text-green { color: var(--color-state-success); }
.status-badge.text-orange { color: var(--color-state-warning); }
.status-badge.text-red { color: var(--color-state-error); }

.grid-views {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.grid-views svg {
  width: 12px;
  height: 12px;
}

.grid-date {
  font-size: 11px;
  color: var(--color-text-quaternary);
}

.grid-actions {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}

.grid-card:hover .grid-actions {
  opacity: 1;
}

.grid-action-btn {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, #000 30%, transparent);
  backdrop-filter: blur(8px);
  color: var(--color-text-secondary);
  border: none;
  cursor: pointer;
  transition: all 0.15s;
}

.grid-action-btn:hover {
  background: color-mix(in srgb, #000 50%, transparent);
  color: var(--color-foreground);
}

.grid-action-btn.danger:hover {
  background: color-mix(in srgb, var(--color-state-error) 80%, transparent);
  color: var(--color-foreground);
}

.grid-action-btn svg {
  width: 14px;
  height: 14px;
}

/* Pagination */
.pagination-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-top: 1px solid var(--color-border);
}

.pagination-info {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.pagination-info strong {
  color: var(--color-text-secondary);
  font-weight: 600;
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 4px;
}

.page-btn {
  min-width: 30px;
  height: 30px;
  padding: 0 8px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.18s;
  background: transparent;
  border: 1px solid transparent;
}

.page-btn:hover:not(.active):not(:disabled) {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.page-btn.active {
  background: var(--color-primary);
  color: var(--color-foreground);
  font-weight: 600;
}

.page-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.page-btn svg {
  width: 14px;
  height: 14px;
}

.page-ellipsis {
  color: var(--color-text-quaternary);
  font-size: 12px;
  padding: 0 4px;
}

.page-size-select {
  padding: 6px 10px;
  border-radius: 7px;
  background: var(--color-secondary);
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s;
  border: none;
  margin-left: 8px;
  outline: none;
}

.page-size-select:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}

/* Responsive */
@media (max-width: 768px) {
  .content-wrap {
    padding: 16px;
  }

  .apple-page-header {
    padding: 0 16px;
  }

  .filter-search {
    width: 100%;
    min-width: 0;
  }

  .card-header {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .filter-left {
    width: 100%;
  }

  .filter-right {
    justify-content: flex-end;
    flex-wrap: wrap;
  }

  .page-row {
    flex-wrap: wrap;
    gap: 10px;
  }

  .row-right {
    width: 100%;
    justify-content: flex-end;
  }

  .page-grid-view {
    grid-template-columns: 1fr;
    padding: 16px;
  }

  .pagination-bar {
    flex-direction: column;
    gap: 12px;
  }
}
</style>
