<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { dashboardApi, type StellarVersionResponse } from "@/api";
import { useBreakpoint } from "../useBreakpoint";

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
  <div class="apple-card stellar-version-card h-full" :data-bp="breakpoint">
    <div class="stellar-version-header">
      <div class="stellar-version-title-row">
        <div class="stellar-version-icon" aria-hidden="true">
          <Icon icon="ri:git-branch-line" class="w-5 h-5" />
        </div>
        <div>
          <h3 class="apple-section-title">Stellar 版本</h3>
          <p class="stellar-version-caption">主题同步状态</p>
        </div>
      </div>
      <span v-if="!loading" class="stellar-version-state" :class="`is-${state}`">
        <span class="stellar-version-dot" aria-hidden="true" />
        {{ stateLabel }}
      </span>
    </div>

    <div v-if="loading" class="stellar-version-loading" aria-label="正在检查版本">
      <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
      <span>正在检查</span>
    </div>
    <template v-else>
      <div class="stellar-version-values">
        <div>
          <span class="stellar-version-label">本地版本</span>
          <strong>{{ data?.local_version || "未知" }}</strong>
        </div>
        <div>
          <span class="stellar-version-label">GitHub 最新</span>
          <strong>{{ data?.upstream_version || "未获取" }}</strong>
        </div>
      </div>

      <div class="stellar-version-footer">
        <span class="stellar-version-message" :class="`is-${state}`">
          <Icon :icon="stateIcon" class="w-4 h-4" aria-hidden="true" />
          {{ data?.error || (state === "outdated" ? "建议查看最新 release" : "当前版本与上游一致") }}
        </span>
        <a
          v-if="data?.release_url"
          class="stellar-version-release"
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

<style scoped>
.stellar-version-card {
  display: flex;
  flex-direction: column;
  min-height: 184px;
}

.stellar-version-header,
.stellar-version-title-row,
.stellar-version-footer,
.stellar-version-state,
.stellar-version-message,
.stellar-version-loading {
  display: flex;
  align-items: center;
}

.stellar-version-header,
.stellar-version-footer {
  justify-content: space-between;
  gap: 12px;
}

.stellar-version-title-row {
  min-width: 0;
  gap: 10px;
}

.stellar-version-icon {
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

.stellar-version-caption,
.stellar-version-label {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.stellar-version-caption {
  margin: 2px 0 0;
}

.stellar-version-state {
  flex: 0 0 auto;
  gap: 6px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.stellar-version-state.is-synced,
.stellar-version-message.is-synced {
  color: var(--color-state-success);
}

.stellar-version-state.is-outdated,
.stellar-version-message.is-outdated {
  color: var(--color-state-warning);
}

.stellar-version-state.is-unavailable,
.stellar-version-message.is-unavailable {
  color: var(--color-muted-foreground);
}

.stellar-version-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.stellar-version-values {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 22px 0 18px;
}

.stellar-version-values > div {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stellar-version-values strong {
  overflow: hidden;
  color: var(--color-foreground);
  font-size: 20px;
  font-weight: 650;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stellar-version-footer {
  margin-top: auto;
  min-width: 0;
}

.stellar-version-message {
  min-width: 0;
  gap: 6px;
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stellar-version-release {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 4px;
  color: var(--color-accent);
  font-size: 12px;
  font-weight: 600;
  text-decoration: none;
}

.stellar-version-release:hover {
  color: var(--color-accent-hover);
}

.stellar-version-loading {
  flex: 1;
  justify-content: center;
  gap: 8px;
  color: var(--color-muted-foreground);
  font-size: 13px;
}

.stellar-version-card:is([data-bp="xs"], [data-bp="xxs"]) {
  min-height: 168px;
}

.stellar-version-card:is([data-bp="xs"], [data-bp="xxs"]) .stellar-version-values {
  margin: 16px 0 14px;
}
</style>
