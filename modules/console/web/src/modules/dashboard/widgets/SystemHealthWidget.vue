<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { attachmentApi, perfApi, type PerfOverview } from "@/api";
import { useBreakpoint } from "../useBreakpoint";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";
import type { WidgetSizePreset } from "./layout";

interface Props { sizePreset?: WidgetSizePreset }
interface AttachmentItem { mime?: string; size?: number }
interface AttachmentResponse { items?: AttachmentItem[]; total?: number }
interface UsageItem { label: string; bytes: number; count: number; tone: string }

const props = withDefaults(defineProps<Props>(), { sizePreset: "medium" });
const sizePreset = useWidgetSize(props.sizePreset).preset;
const { breakpoint } = useBreakpoint();
const router = useRouter();

const overview = ref<PerfOverview | null>(null);
const usage = ref<UsageItem[]>([]);
const loading = ref(true);
const error = ref("");

function classify(mime: string): number {
  if (mime.startsWith("image/")) return 0;
  if (mime.startsWith("video/") || mime.startsWith("audio/")) return 2;
  return 1;
}

async function load() {
  loading.value = true;
  error.value = "";
  const [perf, attachments] = await Promise.allSettled([
    perfApi.overview(),
    attachmentApi.list({ page: 1, size: 100 }),
  ]);
  if (perf.status === "fulfilled") overview.value = perf.value.data;
  if (attachments.status === "fulfilled") {
    const firstPage = attachments.value.data as AttachmentResponse;
    let attachmentItems = firstPage.items || [];
    const total = Math.max(Number(firstPage.total) || attachmentItems.length, attachmentItems.length);
    const pageCount = Math.ceil(total / 100);
    if (pageCount > 1) {
      try {
        const rest = await Promise.all(
          Array.from({ length: pageCount - 1 }, (_, index) => attachmentApi.list({ page: index + 2, size: 100 })),
        );
        attachmentItems = attachmentItems.concat(...rest.map((response) => (response.data as AttachmentResponse).items || []));
      } catch {
        // 保留第一页汇总；性能指标和已成功返回的附件仍可用。
      }
    }
    const buckets = [
      { label: "图片", bytes: 0, count: 0, tone: "primary" },
      { label: "文档", bytes: 0, count: 0, tone: "chart-5" },
      { label: "音视频", bytes: 0, count: 0, tone: "chart-1" },
    ];
    for (const item of attachmentItems) {
      const bucket = buckets[classify(item.mime || "")];
      bucket.bytes += Number(item.size) || 0;
      bucket.count += 1;
    }
    usage.value = buckets;
  }
  if (perf.status === "rejected" && attachments.status === "rejected") {
    error.value = "系统状态暂不可用";
  }
  loading.value = false;
}

function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / (1024 ** index)).toFixed(index ? 1 : 0)} ${units[index]}`;
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days) return `${days}天 ${hours}小时`;
  if (hours) return `${hours}小时 ${minutes}分`;
  return `${minutes}分钟`;
}

const totalUsage = computed(() => usage.value.reduce((sum, item) => sum + item.bytes, 0));
const memoryLabel = computed(() => overview.value ? `${overview.value.heap_alloc_mb.toFixed(1)} MB` : "--");
const healthReady = computed(() => Boolean(overview.value || usage.value.length));
const handleRefresh = () => { void load(); };

onMounted(() => {
  load();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh));
</script>

<template>
  <div class="apple-card health-card h-full flex flex-col" :class="`system-health-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between gap-3 mb-2.5 health-header">
      <div>
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">系统运行状态</h3>
        <p v-if="sizePreset !== 'small'" class="health-subtitle">Runtime & Storage</p>
      </div>
      <button type="button" class="health-detail-link" @click="router.push('/perf')">
        详情 <Icon icon="ri:arrow-right-up-line" class="w-3.5 h-3.5" />
      </button>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载系统状态">
      <span v-for="index in 4" :key="index" class="dashboard-skeleton-row"><i class="dashboard-skeleton-avatar" /><i class="dashboard-skeleton-copy is-wide" /></span>
    </div>
    <div v-else-if="error && !healthReady" class="health-empty flex-1 text-state-error">{{ error }}</div>
    <template v-else>
      <div class="health-metric-grid" :class="{ 'is-large': sizePreset === 'large' }">
        <div class="health-metric">
          <span class="health-metric-icon is-success"><Icon icon="ri:timer-flash-line" /></span>
          <span><small>运行时长</small><strong>{{ overview ? formatUptime(overview.uptime_sec) : '--' }}</strong></span>
        </div>
        <div class="health-metric">
          <span class="health-metric-icon is-primary"><Icon icon="ri:database-2-line" /></span>
          <span><small>数据库</small><strong>{{ overview ? formatBytes(overview.db_size_bytes) : '--' }}</strong></span>
        </div>
        <div class="health-metric">
          <span class="health-metric-icon is-purple"><Icon icon="ri:cpu-line" /></span>
          <span><small>堆内存</small><strong>{{ memoryLabel }}</strong></span>
        </div>
        <div v-if="sizePreset !== 'small'" class="health-metric">
          <span class="health-metric-icon is-warning"><Icon icon="ri:links-line" /></span>
          <span><small>协程</small><strong>{{ overview?.goroutines ?? '--' }}</strong></span>
        </div>
      </div>

      <div v-if="sizePreset === 'large'" class="health-storage">
        <div class="flex items-center justify-between mb-2">
          <span class="health-section-label">附件占用</span>
          <span class="tabular-nums text-xs text-muted-foreground">{{ formatBytes(totalUsage) }}</span>
        </div>
        <div class="health-storage-grid">
          <div v-for="item in usage" :key="item.label" class="health-storage-col">
            <div class="flex items-center justify-between text-xs mb-1.5">
              <span>{{ item.label }}</span>
              <span class="tabular-nums text-[11px] text-muted-foreground">{{ item.count }} 个 · {{ formatBytes(item.bytes) }}</span>
            </div>
            <div class="health-storage-track">
              <i :class="`is-${item.tone}`" :style="{ width: `${totalUsage ? Math.max(4, (item.bytes / totalUsage) * 100) : 4}%` }" />
            </div>
          </div>
        </div>
      </div>
      <div class="health-runtime-note">
        <Icon icon="ri:shield-check-line" class="w-3.5 h-3.5 text-state-success flex-shrink-0" />
        <span class="truncate">{{ overview ? `${overview.platform} · ${overview.go_version}` : '运行时模块未启用' }}</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.health-card {
  padding: 18px 20px;
}
.health-subtitle { margin-top: 2px; color: var(--color-muted-foreground); font-size: 11px; }
.health-detail-link { display: inline-flex; align-items: center; gap: 3px; border: 0; color: var(--color-muted-foreground); background: transparent; font: inherit; font-size: 11px; cursor: pointer; transition: color var(--duration-fast, 150ms) ease; }
.health-detail-link:hover { color: var(--color-primary); }
.health-metric-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.health-metric-grid.is-large { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.health-metric { display: flex; align-items: center; gap: 8px; min-width: 0; padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--color-muted-foreground) 12%, transparent); border-radius: 12px; background: color-mix(in srgb, var(--color-muted-foreground) 4%, transparent); }
.health-metric > span:last-child { display: flex; flex-direction: column; min-width: 0; gap: 2px; }
.health-metric small { color: var(--color-muted-foreground); font-size: 10px; line-height: 1.2; }
.health-metric strong { overflow: hidden; color: var(--color-foreground); font-size: 13px; font-weight: 600; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
.health-metric-icon { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; flex: 0 0 auto; border-radius: 9px; }
.health-metric-icon.is-success { color: var(--color-state-success); background: color-mix(in srgb, var(--color-state-success) 12%, transparent); }
.health-metric-icon.is-primary { color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 12%, transparent); }
.health-metric-icon.is-purple { color: var(--color-chart-5); background: color-mix(in srgb, var(--color-chart-5) 12%, transparent); }
.health-metric-icon.is-warning { color: var(--color-state-warning); background: color-mix(in srgb, var(--color-state-warning) 12%, transparent); }
.health-storage { margin-top: 14px; padding-top: 12px; border-top: 1px solid color-mix(in srgb, var(--color-muted-foreground) 12%, transparent); }
.health-section-label { color: var(--color-foreground); font-size: 11px; font-weight: 600; }
.health-storage-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.health-storage-track { height: 5px; overflow: hidden; border-radius: 99px; background: color-mix(in srgb, var(--color-muted-foreground) 12%, transparent); }
.health-storage-track i { display: block; height: 100%; min-width: 4px; border-radius: inherit; background: var(--color-primary); }
.health-storage-track i.is-chart-5 { background: var(--color-chart-5); }
.health-storage-track i.is-chart-1 { background: var(--color-chart-1); }
.health-runtime-note { display: flex; align-items: center; gap: 6px; margin-top: auto; padding-top: 10px; padding-bottom: 2px; color: var(--color-muted-foreground); font-size: 11px; line-height: 1.3; }
.health-empty { display: flex; align-items: center; justify-content: center; min-height: 110px; font-size: 12px; }

.system-health-small.health-card { padding: 13px 15px; }
.system-health-small .health-header { margin-bottom: 6px; }
.system-health-small .health-metric-grid { gap: 6px; }
.system-health-small .health-metric { padding: 6px 8px; border-radius: 10px; gap: 6px; }
.system-health-small .health-metric-icon { width: 24px; height: 24px; border-radius: 7px; }
.system-health-small .health-metric strong { font-size: 11.5px; }
.system-health-small .health-metric small { font-size: 9.5px; }
.system-health-small .health-runtime-note { padding-top: 6px; padding-bottom: 0; font-size: 10px; }

.system-health-large.health-card { padding: 18px 22px; }
.system-health-large .health-header { margin-bottom: 8px; }
.system-health-large .health-storage { margin-top: 12px; padding-top: 10px; }
.system-health-large .health-runtime-note { padding-top: 10px; padding-bottom: 0; }

@media (max-width: 640px) {
  .health-metric-grid.is-large { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .health-storage-grid { grid-template-columns: repeat(1, minmax(0, 1fr)); gap: 8px; }
}
</style>
