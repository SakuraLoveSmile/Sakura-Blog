<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, VTag, Toast } from "@/components/ui";
import { systemApi, type UpdateCheckResponse } from "@/api";

const checkData = ref<UpdateCheckResponse | null>(null);
const checking = ref(false);

const hasUpdate = computed(() => checkData.value?.update_available ?? false);
// 三态：current 为 unknown 或 latest 缺失时，无法确定是否有更新（如本地 dev 构建恒为 unknown）。
const canCompare = computed(() => checkData.value?.current_version !== "unknown" && !!checkData.value?.latest_version);

const versionStatusText = computed(() => {
  if (hasUpdate.value) return "发现新版本";
  if (canCompare.value) return "已是最新版本";
  return "无法确定是否有更新";
});

const versionStatusClass = computed(() => {
  if (hasUpdate.value) return "border-primary/30 bg-primary/10 text-primary";
  if (canCompare.value) return "border-state-success/30 bg-state-success/10 text-state-success";
  return "border-border bg-secondary text-text-tertiary";
});

async function checkUpdate(refresh = false) {
  checking.value = true;
  try {
    const { data } = await systemApi.check(refresh);
    checkData.value = data;
    if (refresh) {
      Toast.success("已完成最新版本检查");
    }
  } catch {
  } finally {
    checking.value = false;
  }
}

async function copyText(text: string, label = "内容") {
  try {
    await navigator.clipboard.writeText(text);
    Toast.success(`已复制${label}到剪贴板`);
  } catch {
    Toast.error("复制失败，请手动复制");
  }
}

onMounted(() => {
  checkUpdate();
});
</script>

<template>
  <div>
    <!-- 顶部固定吸顶导航栏 -->
    <VPageHeader title="系统更新">
      <template #subtitle> 检查 GHCR 镜像的最新版本 </template>
      <template #actions>
        <VButton size="sm" type="default" :loading="checking" @click="checkUpdate(false)">
          <template #icon>
            <Icon icon="ri:refresh-line" />
          </template>
          检查更新
        </VButton>
        <VButton size="sm" type="secondary" :loading="checking" @click="checkUpdate(true)">
          <template #icon>
            <Icon icon="ri:refresh-fill" />
          </template>
          强制刷新
        </VButton>
      </template>
    </VPageHeader>

    <div class="m-0 max-w-4xl space-y-4 p-4 md:m-4 md:p-0">
      <!-- 1. 版本概览卡片 -->
      <VCard>
        <template #header>
          <div class="flex items-center gap-2 text-[14px] font-semibold text-foreground">
            <Icon icon="ri:shield-check-line" class="text-lg text-primary" />
            <span>版本概览</span>
          </div>
          <div class="flex items-center gap-2">
            <span
              class="flex items-center gap-1.5 rounded-sm border px-2.5 py-0.5 text-xs font-medium"
              :class="versionStatusClass"
            >
              <Icon
                :icon="
                  hasUpdate ? 'ri:arrow-up-circle-fill' : canCompare ? 'ri:checkbox-circle-fill' : 'ri:question-line'
                "
                class="text-sm"
              />
              {{ versionStatusText }}
            </span>
          </div>
        </template>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <!-- 当前版本 -->
          <div
            class="flex flex-col justify-between rounded-base border border-border bg-card/60 p-4 transition-colors hover:border-border-strong"
          >
            <div class="flex items-center justify-between text-xs text-text-tertiary">
              <span class="flex items-center gap-1.5">
                <Icon icon="ri:database-2-line" class="text-primary" />
                当前环境
              </span>
              <VTag theme="default" rounded>运行中</VTag>
            </div>
            <div class="mt-3">
              <div class="font-mono text-xl font-bold tracking-tight text-foreground">
                {{ checkData?.current_version || "未知" }}
              </div>
              <p class="mt-1 text-xs text-text-quaternary">
                {{ checkData?.current_version === "unknown" ? "本地开发 / 自编译构建" : "已部署正式版本" }}
              </p>
            </div>
          </div>

          <!-- GHCR 最新镜像标签 -->
          <div
            class="flex flex-col justify-between rounded-base border border-border bg-card/60 p-4 transition-colors hover:border-border-strong"
          >
            <div class="flex items-center justify-between text-xs text-text-tertiary">
              <span class="flex items-center gap-1.5">
                <Icon icon="ri:github-fill" class="text-text-secondary" />
                GHCR 最新镜像
              </span>
              <a
                v-if="checkData?.release_url"
                :href="checkData.release_url"
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center gap-0.5 text-xs text-primary hover:underline"
              >
                镜像包
                <Icon icon="ri:external-link-line" class="text-[11px]" />
              </a>
            </div>
            <div class="mt-3">
              <div class="font-mono text-xl font-bold tracking-tight text-foreground">
                {{ checkData?.latest_version || "未获取" }}
              </div>
              <p class="mt-1 text-xs text-text-quaternary">
                <span v-if="checking">正在检索 GHCR 镜像标签…</span>
                <span v-else-if="checkData?.latest_version">GHCR 镜像版本标签</span>
                <span v-else>暂无版本标签</span>
              </p>
            </div>
          </div>
        </div>
      </VCard>

      <!-- 2. 检查异常错误提示（若有） -->
      <div
        v-if="checkData?.error && !checkData.latest_version"
        class="flex items-start gap-3 rounded-base border border-state-error/30 bg-state-error/10 p-4 text-sm text-state-error"
      >
        <Icon icon="ri:error-warning-line" class="mt-0.5 shrink-0 text-xl text-state-error" />
        <div class="min-w-0 flex-1">
          <div class="font-semibold text-state-error">检查更新遇到错误</div>
          <p class="mt-1 break-words text-xs leading-relaxed text-text-secondary">{{ checkData.error }}</p>
          <p class="mt-2 text-xs text-text-tertiary">
            提示：检查更新读取公开的 GHCR 镜像标签；服务器无法访问 ghcr.io，或镜像包不是公开的，都会在此显示具体原因。
          </p>
        </div>
        <VButton size="xs" type="danger" :loading="checking" @click="checkUpdate(true)">
          <template #icon>
            <Icon icon="ri:refresh-line" />
          </template>
          重试
        </VButton>
      </div>

      <!-- 3. 更新日志卡片（有新版本或已获取 Release Notes 时展示） -->
      <VCard v-if="checkData?.release_notes || hasUpdate">
        <template #header>
          <div class="flex items-center gap-2 text-[14px] font-semibold text-foreground">
            <Icon icon="ri:file-list-3-line" class="text-lg text-primary" />
            <span>版本更新日志</span>
            <VTag v-if="checkData?.latest_version" theme="primary" rounded>
              {{ checkData.latest_version }}
            </VTag>
          </div>
          <div class="flex items-center gap-2">
            <VButton
              v-if="checkData?.release_notes"
              size="xs"
              type="default"
              @click="copyText(checkData.release_notes, '更新日志')"
            >
              <template #icon>
                <Icon icon="ri:file-copy-line" />
              </template>
              复制日志
            </VButton>
            <a v-if="checkData?.release_url" :href="checkData.release_url" target="_blank" rel="noopener noreferrer">
              <VButton size="xs" type="secondary">
                <template #icon>
                  <Icon icon="ri:external-link-line" />
                </template>
                镜像包页面
              </VButton>
            </a>
          </div>
        </template>

        <div
          v-if="hasUpdate"
          class="mb-4 flex items-center justify-between rounded-base border border-primary/30 bg-primary/10 p-3.5 text-xs text-primary"
        >
          <div class="flex items-center gap-2.5">
            <Icon icon="ri:arrow-up-circle-fill" class="text-base text-primary" />
            <span
              >发现新版本 <strong>{{ checkData?.latest_version }}</strong
              >，在服务器执行 <code>docker compose pull && docker compose up -d</code> 完成升级。</span
            >
          </div>
        </div>

        <div v-if="checkData?.release_notes" class="rounded-base border border-border bg-secondary p-4">
          <div
            class="custom-scrollbar max-h-72 overflow-y-auto font-mono text-xs leading-relaxed text-text-secondary whitespace-pre-wrap"
          >
            {{ checkData.release_notes }}
          </div>
        </div>
        <div v-else class="py-4 text-center text-xs text-text-quaternary">暂无详细更新日志</div>
      </VCard>

    </div>
  </div>
</template>

<style scoped>
/* 自定义细滚动条 */
.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: var(--color-accent);
  border-radius: 6px;
}
.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: var(--color-border-strong);
}

@media (max-width: 640px) {
  .max-w-4xl {
    margin: 0 !important;
  }
}
</style>
