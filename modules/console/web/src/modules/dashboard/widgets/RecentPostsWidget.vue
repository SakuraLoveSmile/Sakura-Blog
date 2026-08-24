<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { dashboardApi } from "@/api";
import { formatRelative } from "@/lib/format";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
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

onMounted(() => {
  dashboardApi
    .recentPosts()
    .then((r: { data: PostsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});

const displayedItems = computed(() => {
  if (sizePreset.value === "small") return items.value.slice(0, 3);
  if (sizePreset.value === "medium") return items.value.slice(0, 5);
  return items.value.slice(0, 8);
});

function editPost(item: PostItem) {
  router.push(`/posts/edit/${item.slug}`);
}

function previewPost(item: PostItem) {
  window.open(`/post/${encodeURIComponent(item.slug)}`, "_blank", "noopener,noreferrer");
}
</script>

<template>
  <div
    class="apple-card h-full flex flex-col justify-between"
    :class="`recent-posts-${sizePreset}`"
    :data-bp="breakpoint"
  >
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-baseline gap-2.5">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近文章</h3>
        <!-- 状态点图例 -->
        <span v-if="sizePreset !== 'small'" class="recent-legend">
          <span class="recent-legend-item"><span class="apple-status-dot published" />已发布</span>
          <span class="recent-legend-item"><span class="apple-status-dot draft" />草稿</span>
        </span>
      </div>
      <button @click="router.push('/posts')" class="apple-link text-xs">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="recent-placeholder flex items-center justify-center text-muted-foreground flex-1">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">
      {{ error }}
    </div>
    <div
      v-else-if="items.length === 0"
      class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1"
    >
      暂无文章
    </div>

    <!-- Small 尺寸：单行精简列表 -->
    <div v-else-if="sizePreset === 'small'" class="space-y-1 flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="w-full flex items-center justify-between py-1.5 px-2 rounded-lg hover:bg-white/5 text-left transition-colors"
        @click="editPost(item)"
      >
        <div class="flex items-center gap-2 min-w-0 flex-1 pr-2">
          <span class="apple-status-dot" :class="item.status === 'published' ? 'published' : 'draft'" />
          <span class="text-foreground text-xs truncate">{{ item.title }}</span>
        </div>
        <span class="text-[10px] text-muted-foreground whitespace-nowrap">{{ formatRelative(item.updated_at) }}</span>
      </button>
    </div>

    <!-- Large 尺寸：富列表 + 编辑/预览快捷入口 -->
    <div v-else-if="sizePreset === 'large'" class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <div v-for="item in displayedItems" :key="item.slug" class="apple-list-item group">
        <button class="recent-post-main" type="button" @click="editPost(item)">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span
                class="apple-status-dot"
                :class="item.status === 'published' ? 'published' : 'draft'"
                :title="item.status === 'published' ? '已发布' : '草稿'"
              />
              <span class="text-foreground text-sm truncate">{{ item.title }}</span>
            </div>
            <div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
              <span v-if="item.category" class="flex items-center gap-1">
                <Icon icon="ri:folder-3-line" class="w-3 h-3" />
                {{ item.category }}
              </span>
              <span class="flex items-center gap-1">
                <Icon icon="ri:time-line" class="w-3 h-3" />
                {{ formatRelative(item.updated_at) }}
              </span>
            </div>
          </div>
        </button>
        <div class="recent-post-actions" aria-label="文章快捷操作">
          <button type="button" title="编辑" @click="editPost(item)">
            <Icon icon="ri:edit-line" />
          </button>
          <button type="button" title="前台预览" @click="previewPost(item)">
            <Icon icon="ri:external-link-line" />
          </button>
        </div>
      </div>
    </div>

    <!-- Medium 尺寸：标准富列表 -->
    <div v-else class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <button v-for="item in displayedItems" :key="item.slug" class="apple-list-item group" @click="editPost(item)">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span
              class="apple-status-dot"
              :class="item.status === 'published' ? 'published' : 'draft'"
              :title="item.status === 'published' ? '已发布' : '草稿'"
            />
            <span class="text-foreground text-sm truncate">{{ item.title }}</span>
          </div>
          <div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
            <span v-if="item.category" class="flex items-center gap-1">
              <Icon icon="ri:folder-3-line" class="w-3 h-3" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1">
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
.recent-placeholder {
  min-height: 120px;
}
.recent-legend {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  font-size: 11px;
  color: var(--color-text-muted, var(--text-muted, #8e8e93));
}
.recent-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  white-space: nowrap;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-legend {
  display: none;
}

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

.recent-post-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0;
  transition: opacity 150ms var(--ease-standard);
}

.apple-list-item:hover .recent-post-actions,
.apple-list-item:focus-within .recent-post-actions {
  opacity: 1;
}

.recent-post-actions button {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  color: var(--color-muted-foreground);
  background: transparent;
  cursor: pointer;
}

.recent-post-actions button:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}

.recent-post-actions svg {
  width: 14px;
  height: 14px;
}
</style>
