<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { systemApi, type UpdateCheckResponse } from "@/api";
import { useBreakpoint } from "../useBreakpoint";
import { useWidgetSize } from "../useWidgetSize";
import type { WidgetSizePreset } from "./layout";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const router = useRouter();
const data = ref<UpdateCheckResponse | null>(null);
const loading = ref(true);

const hasUpdate = computed(() => data.value?.update_available ?? false);
// 三态：current 为 unknown 或 latest 缺失时无法确定是否有更新，归「未知」，不再误判为「最新版本」。
const canCompare = computed(() => {
  const cv = data.value?.current_version;
  const lv = data.value?.latest_version;
  return !!cv && cv !== "unknown" && !!lv;
});

const stateLabel = computed(() => {
  if (data.value?.error && !data.value?.latest_version) return "检查失败";
  if (hasUpdate.value) return "有新版本";
  if (canCompare.value) return "最新版本";
  return "未知";
});

const stateClass = computed(() => {
  if (data.value?.error && !data.value?.latest_version) return "is-unavailable";
  if (hasUpdate.value) return "is-outdated";
  if (canCompare.value) return "is-synced";
  return "is-unavailable";
});

async function loadVersion() {
  loading.value = true;
  try {
    const { data: resp } = await systemApi.check();
    data.value = resp;
  } catch {
    data.value = null;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadVersion();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadVersion);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadVersion));

function goToUpdate() {
  router.push("/system-update");
}
</script>

<template>
  <!-- Small 尺寸：单行紧凑胶囊 -->
  <div
    v-if="sizePreset === 'small'"
    class="apple-card h-full flex items-center justify-between p-3 cursor-pointer"
    :data-bp="breakpoint"
    @click="goToUpdate"
  >
    <div class="flex items-center gap-2.5 min-w-0">
      <div class="w-8 h-8 rounded-lg bg-accent/15 flex items-center justify-center text-accent flex-shrink-0">
        <Icon icon="ri:rocket-2-line" class="w-4 h-4" />
      </div>
      <div class="min-w-0">
        <div class="text-xs font-semibold text-foreground truncate">Blog {{ data?.current_version || "未知" }}</div>
        <div class="text-[10px] text-muted-foreground truncate">最新: {{ data?.latest_version || "--" }}</div>
      </div>
    </div>
    <span v-if="!loading" class="version-card-state text-xs ml-2 flex-shrink-0" :class="stateClass">
      <span class="version-card-dot" aria-hidden="true" />
      {{ stateLabel }}
    </span>
  </div>

  <!-- Medium / Large 尺寸：标准与扩展视图（样式走 dashboard.css 共享 .version-card-*） -->
  <div
    v-else
    class="apple-card version-card h-full cursor-pointer"
    :class="{ 'is-attention': hasUpdate }"
    :data-bp="breakpoint"
    @click="goToUpdate"
  >
    <div class="version-card-header">
      <div class="version-card-title-row">
        <div class="version-card-icon" aria-hidden="true">
          <Icon icon="ri:rocket-2-line" class="w-5 h-5" />
        </div>
        <div>
          <h3 class="apple-section-title">Blog 版本</h3>
          <p class="version-card-caption">系统更新状态</p>
        </div>
      </div>
      <span v-if="!loading" class="version-card-state" :class="stateClass">
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
          <span class="version-card-label">当前版本</span>
          <strong class="font-mono">{{ data?.current_version || "未知" }}</strong>
        </div>
        <div>
          <span class="version-card-label">GitHub 最新</span>
          <strong class="font-mono">{{ data?.latest_version || "未获取" }}</strong>
        </div>
        <div v-if="sizePreset === 'large'">
          <span class="version-card-label">升级方式</span>
          <strong class="text-xs text-muted-foreground font-normal mt-1 font-mono">手动更新</strong>
        </div>
      </div>

      <div class="version-card-footer">
        <span class="version-card-message" :class="stateClass">
          <Icon
            :icon="hasUpdate ? 'ri:arrow-up-circle-line' : 'ri:checkbox-circle-line'"
            class="w-4 h-4"
            aria-hidden="true"
          />
          {{ data?.error || (hasUpdate ? "点击进入系统更新页面" : (canCompare ? "当前版本已是最新" : "版本未知，无法比较")) }}
        </span>
        <Icon
          v-if="hasUpdate"
          icon="ri:arrow-right-line"
          class="w-4 h-4 text-primary"
          aria-hidden="true"
        />
      </div>
    </template>
  </div>
</template>
