<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, Dialog, Toast } from "@/components/ui";
import { systemApi, type UpdateCheckResponse, type UpdateStatusResponse } from "@/api";

const checkData = ref<UpdateCheckResponse | null>(null);
const statusData = ref<UpdateStatusResponse | null>(null);
const checking = ref(false);
const applying = ref(false);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const hasUpdate = computed(() => checkData.value?.update_available ?? false);
// 三态：current 为 unknown 或 latest 缺失时，无法确定是否有更新（如本地 dev 构建恒为 unknown）。
const canCompare = computed(
  () => checkData.value?.current_version !== "unknown" && !!checkData.value?.latest_version
);
const applySupported = computed(() => checkData.value?.apply_supported ?? false);
const currentState = computed(() => statusData.value?.state ?? "idle");

const stateLabel = computed(() => {
  switch (currentState.value) {
    case "running":
      return "更新中";
    case "succeeded":
      return "更新成功";
    case "failed":
      return "更新失败";
    default:
      return "空闲";
  }
});

const stateColor = computed(() => {
  switch (currentState.value) {
    case "running":
      return "text-state-warning";
    case "succeeded":
      return "text-state-success";
    case "failed":
      return "text-state-error";
    default:
      return "text-text-quaternary";
  }
});

const stateIcon = computed(() => {
  switch (currentState.value) {
    case "running":
      return "ri:loader-4-line";
    case "succeeded":
      return "ri:checkbox-circle-line";
    case "failed":
      return "ri:close-circle-line";
    default:
      return "ri:check-double-line";
  }
});

async function checkUpdate(refresh = false) {
  checking.value = true;
  try {
    const { data } = await systemApi.check(refresh);
    checkData.value = data;
  } catch (err: any) {
    Toast.error(err.response?.data?.error || "检查更新失败");
  } finally {
    checking.value = false;
  }
}

async function applyUpdate() {
  Dialog.warning({
    title: "一键更新",
    description: "将触发后台更新服务，更新期间博客会短暂重启。确认继续？",
    onConfirm: async () => {
      applying.value = true;
      try {
        const { data } = await systemApi.apply();
        Toast.success(`更新已触发，目标版本 ${data.target_version}`);
        startPolling();
      } catch (err: any) {
        Toast.error(err.response?.data?.error || "触发更新失败");
      } finally {
        applying.value = false;
      }
    },
  });
}

async function fetchStatus() {
  try {
    const { data } = await systemApi.status();
    statusData.value = data;
    if (data.state === "succeeded") {
      stopPolling();
      Toast.success("更新成功！当前版本已更新");
      checkUpdate(true);
    } else if (data.state === "failed") {
      stopPolling();
      Toast.error("更新失败，请查看日志");
    }
  } catch {
    // 静默失败，轮询继续
  }
}

function startPolling() {
  stopPolling();
  fetchStatus();
  pollTimer = setInterval(() => fetchStatus(), 3000);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

onMounted(() => {
  checkUpdate();
  fetchStatus();
});

onBeforeUnmount(() => {
  stopPolling();
});
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <VPageHeader title="系统更新" description="检查 Blog 新版本并一键更新" />

    <div class="mt-6 space-y-4">
      <!-- 当前版本状态 -->
      <VCard>
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="flex h-12 w-12 items-center justify-center rounded-base bg-accent/15">
              <Icon icon="ri:rocket-2-line" class="text-2xl text-primary" />
            </div>
            <div>
              <h3 class="text-base font-semibold text-foreground">版本信息</h3>
              <p class="text-sm text-text-quaternary">
                当前版本：
                <span class="font-medium text-foreground">{{ checkData?.current_version || "未知" }}</span>
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <span class="flex items-center gap-1.5 text-sm" :class="stateColor">
              <Icon :icon="stateIcon" :class="{ 'animate-spin': currentState === 'running' }" />
              {{ stateLabel }}
            </span>
          </div>
        </div>
      </VCard>

      <!-- 检查更新结果 -->
      <VCard>
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-foreground">检查更新</h3>
          <div class="flex gap-2">
            <VButton size="sm" variant="ghost" :loading="checking" @click="checkUpdate(false)">
              <Icon icon="ri:refresh-line" class="mr-1" />
              检查
            </VButton>
            <VButton size="sm" variant="ghost" :loading="checking" @click="checkUpdate(true)">
              <Icon icon="ri:refresh-fill" class="mr-1" />
              强制刷新
            </VButton>
          </div>
        </div>

        <!-- Loading -->
        <div v-if="checking && !checkData" class="flex items-center justify-center py-12 text-text-quaternary">
          <Icon icon="ri:loader-4-line" class="mr-2 animate-spin text-xl" />
          正在检查更新…
        </div>

        <!-- Error -->
        <div v-else-if="checkData?.error && !checkData.latest_version" class="py-6">
          <div class="flex items-start gap-3 rounded-base bg-state-error/10 p-4">
            <Icon icon="ri:error-warning-line" class="mt-0.5 text-xl text-state-error" />
            <div>
              <p class="text-sm font-medium text-state-error">检查更新失败</p>
              <p class="mt-1 text-sm text-text-secondary">{{ checkData.error }}</p>
            </div>
          </div>
        </div>

        <!-- Update available -->
        <div v-else-if="checkData" class="mt-4 space-y-4">
          <div class="flex items-center gap-3 rounded-base p-4" :class="hasUpdate ? 'bg-state-success/10' : 'bg-accent/10'">
            <Icon
              :icon="hasUpdate ? 'ri:arrow-up-circle-fill' : (canCompare ? 'ri:checkbox-circle-fill' : 'ri:question-line')"
              class="text-2xl"
              :class="hasUpdate ? 'text-state-success' : 'text-text-quaternary'"
            />
            <div class="flex-1">
              <p class="text-sm font-medium text-foreground">
                {{ hasUpdate ? "发现新版本" : (canCompare ? "已是最新版本" : "无法确定是否有更新") }}
              </p>
              <p class="mt-0.5 text-sm text-text-quaternary">
                GitHub 最新版本：
                <span class="font-medium text-foreground">{{ checkData.latest_version || "未获取" }}</span>
                <span v-if="checkData.published_at" class="ml-2">
                  发布于 {{ new Date(checkData.published_at).toLocaleDateString("zh-CN") }}
                </span>
              </p>
            </div>
            <a
              v-if="checkData.release_url"
              :href="checkData.release_url"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-1 text-sm text-primary hover:text-primary-hover"
            >
              查看详情
              <Icon icon="ri:external-link-line" />
            </a>
          </div>

          <!-- Release notes -->
          <div v-if="checkData.release_notes" class="rounded-base border border-border p-4">
            <h4 class="mb-2 text-sm font-semibold text-foreground">更新日志</h4>
            <div class="max-h-60 overflow-y-auto text-sm text-text-secondary whitespace-pre-line">{{ checkData.release_notes }}</div>
          </div>

          <!-- Unsupported notice -->
          <div v-if="!applySupported" class="flex items-start gap-3 rounded-base bg-state-warning/10 p-4">
            <Icon icon="ri:information-line" class="mt-0.5 text-xl text-state-warning" />
            <div>
              <p class="text-sm font-medium text-state-warning">一键更新不可用</p>
              <p class="mt-1 text-sm text-text-secondary">{{ checkData.apply_note || "当前环境不支持一键更新（非 systemd 部署）" }}</p>
            </div>
          </div>
        </div>
      </VCard>

      <!-- 一键更新操作 -->
      <VCard>
        <h3 class="mb-4 text-base font-semibold text-foreground">一键更新</h3>
        <div class="flex items-center justify-between">
          <p class="text-sm text-text-secondary">
            点击后将触发后台更新服务，自动拉取最新版本并重启。更新期间博客会短暂不可用。
          </p>
          <VButton
            variant="primary"
            :disabled="!applySupported || !hasUpdate || applying || currentState === 'running'"
            :loading="applying"
            @click="applyUpdate"
          >
            <Icon icon="ri:download-cloud-2-line" class="mr-1.5" />
            {{ currentState === "running" ? "更新中…" : "一键更新" }}
          </VButton>
        </div>
      </VCard>

      <!-- 更新状态详情 -->
      <VCard v-if="statusData && currentState !== 'idle'">
        <h3 class="mb-4 text-base font-semibold text-foreground">更新状态</h3>
        <div class="space-y-3">
          <div class="grid grid-cols-2 gap-4">
            <div>
              <span class="text-sm text-text-quaternary">目标版本</span>
              <p class="mt-1 font-medium text-foreground">{{ statusData.target_version || "—" }}</p>
            </div>
            <div>
              <span class="text-sm text-text-quaternary">开始时间</span>
              <p class="mt-1 font-medium text-foreground">
                {{ statusData.started_at ? new Date(statusData.started_at).toLocaleString("zh-CN") : "—" }}
              </p>
            </div>
          </div>
          <div v-if="statusData.message" class="flex items-center gap-2 text-sm" :class="stateColor">
            <Icon :icon="stateIcon" :class="{ 'animate-spin': currentState === 'running' }" />
            {{ statusData.message }}
          </div>
          <!-- 日志尾部 -->
          <div v-if="statusData.log_tail" class="rounded-base bg-black/30 p-3">
            <h4 class="mb-2 text-xs font-medium text-text-quaternary">更新日志（尾部）</h4>
            <pre class="max-h-48 overflow-y-auto text-xs text-text-secondary whitespace-pre-wrap font-mono">{{ statusData.log_tail }}</pre>
          </div>
        </div>
      </VCard>
    </div>
  </div>
</template>
