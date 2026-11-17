<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { statsApi } from "@/api";
import { useBreakpoint } from "../useBreakpoint";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";
import type { WidgetSizePreset } from "./layout";

interface Props {
  sizePreset?: WidgetSizePreset;
  config?: Record<string, unknown>;
}
interface PopularPost { path: string; title: string; pv: number; uv: number }

const props = withDefaults(defineProps<Props>(), { sizePreset: "medium", config: () => ({}) });
const sizePreset = useWidgetSize(props.sizePreset).preset;
const { breakpoint } = useBreakpoint();
const router = useRouter();

const days = ref<7 | 30>(7);
const items = ref<PopularPost[]>([]);
const loading = ref(true);
const error = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const response = await statsApi.popular(days.value, Math.max(5, configuredLimit.value));
    items.value = Array.isArray(response.data?.items) ? response.data.items : [];
  } catch (err) {
    error.value = err instanceof Error ? err.message : "热门文章加载失败";
  } finally {
    loading.value = false;
  }
}

const maxPv = computed(() => Math.max(...items.value.map((item) => item.pv), 1));
/** 配置条数优先，未配置时按档位默认；同时不超过后端返回的条数。 */
const configuredLimit = computed(() => {
  const raw = Number(props.config.itemLimit);
  return Number.isFinite(raw) && raw > 0 ? Math.min(8, Math.round(raw)) : 0;
});
const displayedItems = computed(() => {
  const presetLimit = sizePreset.value === "small" ? 2 : sizePreset.value === "medium" ? 4 : 5;
  const limit = configuredLimit.value || presetLimit;
  return items.value.slice(0, Math.min(limit, items.value.length));
});

function editPost(item: PopularPost) {
  const slug = item.path.replace(/^\/post\//, "");
  if (slug) router.push(`/posts/edit/${decodeURIComponent(slug)}`);
}

function rankTone(index: number): string {
  return `rank-${Math.min(index + 1, 4)}`;
}

const handleRefresh = () => { void load(); };
watch([days, configuredLimit], load);
onMounted(() => {
  load();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh));
</script>

<template>
  <div class="apple-card h-full flex flex-col" :class="`top-posts-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between gap-3 mb-3">
      <div class="min-w-0">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">热门文章</h3>
        <p v-if="sizePreset !== 'small'" class="top-posts-subtitle">近 {{ days }} 天阅读排行</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="card-range" role="group" aria-label="热门文章统计范围">
          <button type="button" class="card-range-btn card-hit" :class="{ 'is-active': days === 7 }" :aria-pressed="days === 7" @click="days = 7">
            <span class="card-range-label">7 天</span>
          </button>
          <button type="button" class="card-range-btn card-hit" :class="{ 'is-active': days === 30 }" :aria-pressed="days === 30" @click="days = 30">
            <span class="card-range-label">30 天</span>
          </button>
        </div>
        <Icon v-if="sizePreset === 'large'" icon="ri:fire-line" class="w-4 h-4 text-state-warning" />
      </div>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载热门文章">
      <span v-for="index in (sizePreset === 'small' ? 2 : sizePreset === 'medium' ? 4 : 5)" :key="index" class="dashboard-skeleton-row"><i class="dashboard-skeleton-avatar" /><i class="dashboard-skeleton-copy is-wide" /></span>
    </div>
    <div v-else-if="error" class="top-posts-empty flex-1 text-state-error">{{ error }}</div>
    <div v-else-if="displayedItems.length === 0" class="top-posts-empty flex-1">暂无访问数据</div>
    <div v-else class="top-posts-list flex-1" :class="{ 'is-large': sizePreset === 'large' }">
      <button v-for="(item, index) in displayedItems" :key="item.path" type="button" class="top-post-row" @click="editPost(item)">
        <span class="top-post-rank" :class="rankTone(index)">{{ index + 1 }}</span>
        <span class="top-post-copy">
          <span class="top-post-title">{{ item.title || item.path }}</span>
          <span class="top-post-track"><i :style="{ width: `${Math.max(6, (item.pv / maxPv) * 100)}%` }" /></span>
        </span>
        <span class="top-post-metrics">
          <strong class="tabular-nums">{{ item.pv.toLocaleString() }}</strong>
          <small v-if="sizePreset !== 'small'" class="tabular-nums">PV · {{ item.uv.toLocaleString() }} UV</small>
        </span>
        <Icon v-if="sizePreset !== 'small'" icon="ri:arrow-right-s-line" class="top-post-arrow" />
      </button>
    </div>
    <div v-if="sizePreset === 'large' && displayedItems.length" class="top-posts-footnote">
      <Icon icon="ri:bar-chart-grouped-line" class="w-3.5 h-3.5" />
      热门内容贡献 {{ Math.round((displayedItems.reduce((sum, item) => sum + item.pv, 0) / Math.max(items.reduce((sum, item) => sum + item.pv, 0), 1)) * 100) }}%
    </div>
  </div>
</template>

<style scoped>
.top-posts-subtitle { margin-top: 3px; color: var(--color-muted-foreground); font-size: 11px; }
.top-posts-list { display: flex; flex-direction: column; gap: 4px; min-height: 0; overflow-y: auto; }
/* 行悬停与头部、其它列表同一语言：只出中性浅底，不描边、不位移；悬停只在有悬停能力的设备上生效，触屏点按后不残留 */
.top-post-row { display: flex; align-items: center; gap: 9px; width: 100%; min-width: 0; padding: 8px 5px; border: 1px solid transparent; border-radius: var(--radius-md); color: inherit; background: transparent; font: inherit; text-align: left; cursor: pointer; transition: background-color var(--duration-fast) var(--ease-standard); }
.top-post-row:active { background: var(--color-secondary-700); }
.top-post-row:focus-visible { outline: 2px solid var(--color-ring); outline-offset: -2px; }
@media (hover: hover) {
  .top-post-row:hover { background: var(--color-accent); }
  .top-post-row:hover .top-post-arrow { opacity: 1; }
}
.top-post-rank { display: inline-flex; align-items: center; justify-content: center; width: 21px; height: 21px; flex: 0 0 auto; border-radius: var(--radius-sm); color: var(--color-muted-foreground); background: color-mix(in srgb, var(--color-muted-foreground) 10%, transparent); font: 700 11px var(--font-mono); }
.top-post-rank.rank-1 { color: var(--color-state-warning); background: color-mix(in srgb, var(--color-state-warning) 13%, transparent); }
.top-post-rank.rank-2 { color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 12%, transparent); }
.top-post-rank.rank-3 { color: var(--color-chart-5); background: color-mix(in srgb, var(--color-chart-5) 12%, transparent); }
.top-post-copy { display: flex; flex: 1; flex-direction: column; gap: 6px; min-width: 0; }
.top-post-title { overflow: hidden; color: var(--color-foreground); font-size: 12px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.top-post-track { display: block; width: 100%; height: 4px; overflow: hidden; border-radius: var(--radius-sm); background: color-mix(in srgb, var(--color-muted-foreground) 12%, transparent); }
.top-post-track i { display: block; height: 100%; border-radius: inherit; background: var(--color-primary); transition: width 350ms var(--ease-standard); }
.top-post-metrics { display: flex; flex: 0 0 auto; flex-direction: column; align-items: flex-end; gap: 2px; color: var(--color-foreground); font-size: 12px; }
.top-post-metrics small { color: var(--color-muted-foreground); font-size: 11px; }
.top-post-arrow { flex: 0 0 auto; color: var(--color-muted-foreground); opacity: 0; transition: opacity var(--duration-fast) var(--ease-standard); }
.top-post-row:focus-visible .top-post-arrow { opacity: 1; }
.top-posts-empty { display: flex; align-items: center; justify-content: center; min-height: 0; color: var(--color-muted-foreground); font-size: 12px; }
.top-posts-footnote { display: flex; align-items: center; gap: 5px; margin-top: 9px; padding-top: 9px; border-top: 1px solid color-mix(in srgb, var(--color-muted-foreground) 12%, transparent); color: var(--color-muted-foreground); font-size: 11px; }
.top-posts-small .card-range { display: none; }
.top-posts-small .top-post-row { padding-block: 6px; }
.top-posts-small .top-post-copy { gap: 4px; }
.top-posts-small .top-post-title { font-size: 11px; }
.top-posts-small .top-post-metrics { font-size: 11px; }
</style>
