<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { systemApi, type UpdateCheckResponse } from "@/api";
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
const router = useRouter();
const data = ref<UpdateCheckResponse | null>(null);
const loading = ref(true);

const hasUpdate = computed(() => data.value?.update_available ?? false);
const applySupported = computed(() => data.value?.apply_supported ?? false);
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

onMounted(async () => {
  try {
    const { data: resp } = await systemApi.check();
    data.value = resp;
  } catch {
    data.value = null;
  } finally {
    loading.value = false;
  }
});

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
    <span v-if="!loading" class="system-version-state text-xs ml-2 flex-shrink-0" :class="stateClass">
      <span class="system-version-dot" aria-hidden="true" />
      {{ stateLabel }}
    </span>
  </div>

  <!-- Medium / Large 尺寸：标准与扩展视图 -->
  <div
    v-else
    class="apple-card system-version-card h-full cursor-pointer flex flex-col justify-between"
    :class="`system-card-${sizePreset}`"
    :data-bp="breakpoint"
    @click="goToUpdate"
  >
    <div class="system-version-header">
      <div class="system-version-title-row">
        <div class="system-version-icon" aria-hidden="true">
          <Icon icon="ri:rocket-2-line" class="w-5 h-5" />
        </div>
        <div>
          <h3 class="apple-section-title">Blog 版本</h3>
          <p class="system-version-caption">系统更新状态</p>
        </div>
      </div>
      <span v-if="!loading" class="system-version-state" :class="stateClass">
        <span class="system-version-dot" aria-hidden="true" />
        {{ stateLabel }}
      </span>
    </div>

    <div v-if="loading" class="system-version-loading" aria-label="正在检查版本">
      <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
      <span>正在检查</span>
    </div>
    <template v-else>
      <div class="system-version-values" :class="{ 'grid-cols-3': sizePreset === 'large' }">
        <div>
          <span class="system-version-label">当前版本</span>
          <strong>{{ data?.current_version || "未知" }}</strong>
        </div>
        <div>
          <span class="system-version-label">GitHub 最新</span>
          <strong>{{ data?.latest_version || "未获取" }}</strong>
        </div>
        <div v-if="sizePreset === 'large'">
          <span class="system-version-label">自动应用</span>
          <strong class="text-xs text-muted-foreground font-normal mt-1">{{ applySupported ? "支持一键更新" : "手动更新" }}</strong>
        </div>
      </div>

      <div class="system-version-footer">
        <span class="system-version-message" :class="stateClass">
          <Icon
            :icon="hasUpdate ? 'ri:arrow-up-circle-line' : 'ri:checkbox-circle-line'"
            class="w-4 h-4"
            aria-hidden="true"
          />
          {{ data?.error || (hasUpdate ? "点击进入系统更新页面" : (canCompare ? "当前版本已是最新" : "版本未知，无法比较")) }}
        </span>
        <Icon
          v-if="applySupported && hasUpdate"
          icon="ri:arrow-right-line"
          class="w-4 h-4 text-primary"
          aria-hidden="true"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.system-version-card {
  display: flex;
  flex-direction: column;
  min-height: 184px;
}

.system-version-header,
.system-version-title-row,
.system-version-footer,
.system-version-state,
.system-version-message,
.system-version-loading {
  display: flex;
  align-items: center;
}

.system-version-header,
.system-version-footer {
  justify-content: space-between;
  gap: 12px;
}

.system-version-title-row {
  min-width: 0;
  gap: 10px;
}

.system-version-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  flex: 0 0 auto;
  border-radius: var(--radius-md);
  color: var(--color-accent);
  background: color-mix(in srgb, var(--color-accent) 14%, transparent);
}

.system-version-caption,
.system-version-label {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.system-version-caption {
  margin: 2px 0 0;
}

.system-version-state {
  flex: 0 0 auto;
  gap: 6px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.system-version-state.is-synced,
.system-version-message.is-synced {
  color: var(--color-state-success);
}

.system-version-state.is-outdated,
.system-version-message.is-outdated {
  color: var(--color-state-warning);
}

.system-version-state.is-unavailable,
.system-version-message.is-unavailable {
  color: var(--color-muted-foreground);
}

.system-version-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.system-version-values {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 22px 0 18px;
}

.system-version-values > div {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.system-version-values strong {
  overflow: hidden;
  color: var(--color-foreground);
  font-size: 20px;
  font-weight: 650;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.system-version-footer {
  margin-top: auto;
  min-width: 0;
}

.system-version-message {
  min-width: 0;
  gap: 6px;
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.system-version-loading {
  flex: 1;
  justify-content: center;
  gap: 8px;
  color: var(--color-muted-foreground);
  font-size: 13px;
}

.system-version-card:is([data-bp="xs"], [data-bp="xxs"]) {
  min-height: 168px;
}

.system-version-card:is([data-bp="xs"], [data-bp="xxs"]) .system-version-values {
  margin: 16px 0 14px;
}
</style>
