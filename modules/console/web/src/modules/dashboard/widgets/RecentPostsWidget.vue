<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { dashboardApi } from "@/api";
import { formatRelative } from "@/lib/format";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

interface Props {
  sizePreset?: WidgetSizePreset;
  config?: Record<string, unknown>;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
  config: () => ({}),
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const router = useRouter();

interface PostItem {
  slug: string;
  title: string;
  category: string;
  status: string;
  updated_at: string;
}

interface PostsResp {
  items: PostItem[];
}

const items = ref<PostItem[]>([]);
const loading = ref(true);
const error = ref("");

function loadPosts() {
  loading.value = true;
  error.value = "";
  dashboardApi.recentPosts()
    .then((r: { data: PostsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e instanceof Error ? e.message : "文章加载失败"))
    .finally(() => (loading.value = false));
}

onMounted(() => {
  loadPosts();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadPosts);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadPosts));

const displayedItems = computed(() => {
  // 同一档位盒子的可用高度固定：小档只放两条单行，中档四条，大档七条；
  // 实例配置的显示条数优先，但仍受盒子高度上限约束，避免溢出。
  const presetMax = sizePreset.value === "small" ? 2 : sizePreset.value === "medium" ? 4 : 7;
  const raw = Number(props.config.itemLimit);
  const limit = Number.isFinite(raw) && raw > 0 ? Math.min(Math.round(raw), presetMax) : presetMax;
  return items.value.slice(0, limit);
});

function editPost(item: PostItem) {
  router.push(`/posts/edit/${item.slug}`);
}

function previewPost(item: PostItem) {
  window.open(`/post/${encodeURIComponent(item.slug)}`, "_blank", "noopener,noreferrer");
}
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`recent-posts-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3 recent-header">
      <div class="flex items-baseline gap-2.5">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近文章</h3>
        <!-- 状态点图例 -->
        <span v-if="sizePreset !== 'small'" class="recent-legend">
          <span class="recent-legend-item"><span class="apple-status-dot published" />已发布</span>
          <span class="recent-legend-item"><span class="apple-status-dot draft" />草稿</span>
        </span>
      </div>
      <button type="button" class="apple-link card-link card-hit text-xs" @click="router.push('/posts')">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载文章">
      <span v-for="index in (sizePreset === 'small' ? 2 : sizePreset === 'medium' ? 4 : 5)" :key="index" class="dashboard-skeleton-row">
        <i class="dashboard-skeleton-avatar" />
        <i class="dashboard-skeleton-copy" />
      </span>
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1">
      暂无文章
    </div>

    <!-- Small 尺寸：单行精简列表 -->
    <div v-else-if="sizePreset === 'small'" class="recent-mini-list flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="recent-mini-row w-full flex items-center justify-between py-1.5 border-t border-border text-left transition-colors duration-150"
        @click="editPost(item)"
      >
        <div class="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
          <span
            class="apple-status-dot"
            :class="item.status === 'published' ? 'published' : 'draft'"
            :aria-label="item.status === 'published' ? '已发布' : '草稿'"
          />
          <span class="text-foreground text-xs font-medium truncate">{{ item.title }}</span>
        </div>
        <span class="text-[12px] text-muted-foreground whitespace-nowrap">{{ formatRelative(item.updated_at) }}</span>
      </button>
    </div>

    <!-- Large 尺寸：富列表 + 编辑/预览快捷入口 -->
    <div v-else-if="sizePreset === 'large'" class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <div v-for="item in displayedItems" :key="item.slug" class="apple-list-item group">
        <button class="recent-post-main" type="button" @click="editPost(item)">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2.5">
              <span
                class="apple-status-dot"
                :class="item.status === 'published' ? 'published' : 'draft'"
                :title="item.status === 'published' ? '已发布' : '草稿'"
              />
              <span class="text-foreground text-sm font-medium tracking-tight truncate">{{ item.title }}</span>
            </div>
            <div class="flex items-center gap-2 mt-1.5 text-xs text-muted-foreground">
              <span v-if="item.category" class="inline-flex items-center gap-1 text-[12px] text-muted-foreground">
                <Icon icon="ri:folder-3-line" class="w-3 h-3" />
                {{ item.category }}
              </span>
              <span class="flex items-center gap-1 text-[12px]">
                <Icon icon="ri:time-line" class="w-3 h-3" />
                {{ formatRelative(item.updated_at) }}
              </span>
            </div>
          </div>
        </button>
        <div class="recent-post-actions" role="group" aria-label="文章快捷操作">
          <button type="button" class="post-action card-hit" title="编辑" :aria-label="`编辑 ${item.title}`" @click="editPost(item)">
            <Icon icon="ri:edit-line" />
          </button>
          <button type="button" class="post-action card-hit" title="前台预览" :aria-label="`前台预览 ${item.title}`" @click="previewPost(item)">
            <Icon icon="ri:external-link-line" />
          </button>
        </div>
      </div>
    </div>

    <!-- Medium 尺寸：标准富列表 -->
    <div v-else class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="apple-list-item group"
        @click="editPost(item)"
      >
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2.5">
            <span
              class="apple-status-dot"
              :class="item.status === 'published' ? 'published' : 'draft'"
              :title="item.status === 'published' ? '已发布' : '草稿'"
            />
            <span class="text-foreground text-sm font-medium tracking-tight truncate">{{ item.title }}</span>
          </div>
          <div class="flex items-center gap-2 mt-1.5 text-xs text-muted-foreground">
            <span v-if="item.category" class="inline-flex items-center gap-1 text-[12px] text-muted-foreground">
              <Icon icon="ri:folder-3-line" class="w-3 h-3" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1 text-[12px]">
              <Icon icon="ri:time-line" class="w-3 h-3" />
              {{ formatRelative(item.updated_at) }}
            </span>
          </div>
        </div>
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4 text-text-quaternary apple-list-arrow" />
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 小档 107px 高：标题与两条单行记录必须正好放下，不留空间给 gap。 */
.recent-posts-small .recent-header {
  margin-bottom: 4px;
}
.recent-mini-list {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.recent-posts-small .recent-mini-row {
  padding-block: 4px;
}
.recent-placeholder {
  min-height: 0;
}
.recent-legend {
  display: inline-flex;
  align-items: center;
  gap: 0.8rem;
  font-size: 11px;
  color: var(--color-muted-foreground);
}
.recent-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  white-space: nowrap;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-legend { display: none; }

.recent-post-main {
  display: flex;
  min-width: 0;
  flex: 1;
  color: inherit;
  font: inherit;
  text-align: left;
  background: transparent;
  border: 0;
  cursor: pointer;
}

/* 行操作：与评论行同一条显隐规则——只有「能悬停且视口 ≥1024px」才藏到悬停 / 聚焦时，
   其余（触屏、平板、手机）常显，避免触屏上看不见却点得到的隐形热区。
   手机上两枚间隔 16px、整组左移 8px（--card-rail-*，见 dashboard.css），给各自 44px 的点击区留出不抢邻居的空白。 */
.recent-post-actions {
  display: flex;
  align-items: center;
  gap: var(--card-rail-gap-post, 6px);
  flex-shrink: 0;
  margin-right: var(--card-post-inset, 0px);
  transition: opacity var(--duration-fast) var(--ease-standard);
}

@media (hover: hover) and (min-width: 1024px) {
  .recent-post-actions {
    opacity: 0;
  }
  .apple-list-item:hover .recent-post-actions,
  .apple-list-item:focus-within .recent-post-actions {
    opacity: 1;
  }
}

.post-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-muted-foreground);
  background: transparent;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

/* 按钮总是落在悬停中的行上（行悬停底就是 accent），悬停 / 按下各再亮一阶，才看得出落在了哪枚上 */
.post-action:active {
  color: var(--color-foreground);
  background: var(--color-border-strong);
}

.post-action:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

@media (hover: hover) {
  .post-action:hover {
    color: var(--color-foreground);
    background: var(--color-secondary-700);
  }
}

.post-action svg {
  width: 14px;
  height: 14px;
}
</style>
