<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { dashboardApi, type StellarVersionResponse } from "@/api";
import { useBreakpoint } from "../useBreakpoint";
import { useWidgetSize } from "../useWidgetSize";
import type { WidgetSizePreset } from "./layout";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const data = ref<StellarVersionResponse | null>(null);
const loading = ref(true);

const state = computed(() => {
  if (data.value?.error) return "unavailable";
  if (data.value?.in_sync) return "synced";
  return "outdated";
});

const stateLabel = computed(() => {
  switch (state.value) {
    case "synced":
      return "已同步";
    case "outdated":
      return "有新版本";
    default:
      return "暂不可用";
  }
});

const stateIcon = computed(() => {
  switch (state.value) {
    case "synced":
      return "ri:checkbox-circle-line";
    case "outdated":
      return "ri:arrow-up-circle-line";
    default:
      return "ri:cloud-off-line";
  }
});

onMounted(async () => {
  try {
    const response = await dashboardApi.stellarVersion();
    data.value = response.data;
  } catch (error) {
    data.value = {
      local_version: "",
      upstream_version: "",
      in_sync: false,
      release_url: "",
      checked_at: "",
      error: error instanceof Error ? error.message : "版本检查失败",
    };
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <!-- Small 尺寸：单行紧凑胶囊 -->
  <div
    v-if="sizePreset === 'small'"
    class="apple-card h-full flex items-center justify-between p-3"
    :data-bp="breakpoint"
  >
    <div class="flex items-center gap-2.5 min-w-0">
      <div class="w-8 h-8 rounded-lg bg-accent/15 flex items-center justify-center text-accent flex-shrink-0">
        <Icon icon="ri:git-branch-line" class="w-4 h-4" />
      </div>
      <div class="min-w-0">
        <div class="text-xs font-semibold text-foreground truncate">Stellar {{ data?.local_version || "未知" }}</div>
        <div class="text-[10px] text-muted-foreground truncate">最新: {{ data?.upstream_version || "--" }}</div>
      </div>
    </div>
    <span v-if="!loading" class="version-card-state text-xs ml-2 flex-shrink-0" :class="`is-${state}`">
      <span class="version-card-dot" aria-hidden="true" />
      {{ stateLabel }}
    </span>
  </div>

  <!-- Medium / Large 尺寸：标准与扩展视图（样式走 dashboard.css 共享 .version-card-*） -->
  <div
    v-else
    class="apple-card version-card h-full"
    :data-bp="breakpoint"
  >
    <div class="version-card-header">
      <div class="version-card-title-row">
        <div class="version-card-icon" aria-hidden="true">
          <Icon icon="ri:git-branch-line" class="w-5 h-5" />
        </div>
        <div>
          <h3 class="apple-section-title">Stellar 版本</h3>
          <p class="version-card-caption">主题同步状态</p>
        </div>
      </div>
      <span v-if="!loading" class="version-card-state" :class="`is-${state}`">
        <span class="version-card-dot" aria-hidden="true" />
        {{ stateLabel }}
      </span>
    </div>

    <div v-if="loading" class="version-card-loading" aria-label="正在检查版本">
      <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
      <span>正在检查</span>
    </div>
    <template v-else>
      <div class="version-card-values" :class="{ 'grid-cols-3': sizePreset === 'large' }">
        <div>
          <span class="version-card-label">本地版本</span>
          <strong>{{ data?.local_version || "未知" }}</strong>
        </div>
        <div>
          <span class="version-card-label">GitHub 最新</span>
          <strong>{{ data?.upstream_version || "未获取" }}</strong>
        </div>
        <div v-if="sizePreset === 'large'">
          <span class="version-card-label">检查时间</span>
          <strong class="text-xs text-muted-foreground font-normal mt-1">{{ data?.checked_at ? new Date(data.checked_at).toLocaleDateString() : '刚刚' }}</strong>
        </div>
      </div>

      <div class="version-card-footer">
        <span class="version-card-message" :class="`is-${state}`">
          <Icon :icon="stateIcon" class="w-4 h-4" aria-hidden="true" />
          {{ data?.error || (state === "outdated" ? "建议查看最新 release 变更" : "当前版本与上游一致") }}
        </span>
        <a
          v-if="data?.release_url"
          class="version-card-release"
          :href="data.release_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          Release
          <Icon icon="ri:external-link-line" class="w-3.5 h-3.5" aria-hidden="true" />
        </a>
      </div>
    </template>
  </div>
</template>
