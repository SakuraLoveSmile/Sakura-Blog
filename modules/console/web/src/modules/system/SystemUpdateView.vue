<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, VTag, Toast } from "@/components/ui";
import { systemApi, type UpdateCheckResponse } from "@/api";
import { formatDate } from "@/lib/format";

const checkData = ref<UpdateCheckResponse | null>(null);
const checking = ref(false);

// GitHub Token（可选）：用于私有仓库鉴权或提升速率上限。
// 仅保存在本浏览器 localStorage，随请求以 X-GitHub-Token 头发送，不存储到服务端。
const githubToken = ref<string>(localStorage.getItem("blog:github_token") || "");
const showToken = ref(false);

function onTokenInput(e: Event) {
  githubToken.value = (e.target as HTMLInputElement).value;
  if (githubToken.value) localStorage.setItem("blog:github_token", githubToken.value);
  else localStorage.removeItem("blog:github_token");
}

function clearToken() {
  githubToken.value = "";
  localStorage.removeItem("blog:github_token");
  Toast.success("已清除 GitHub Token");
}

// 统一把当前 Token 转成请求参数（空则不携带，回落到服务端配置 / 匿名）。
function tokenParam(): string | undefined {
  const t = githubToken.value.trim();
  return t ? t : undefined;
}

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
    const { data } = await systemApi.check(refresh, tokenParam());
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
      <template #icon>
        <Icon icon="ri:rocket-2-line" class="text-primary" />
      </template>
      <template #subtitle> 检查 Blog 新版本并查看更新日志 </template>
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
          <div class="flex items-center gap-2 text-base font-semibold text-foreground">
            <Icon icon="ri:shield-check-line" class="text-lg text-primary" />
            <span>版本概览</span>
          </div>
          <div class="flex items-center gap-2">
            <span
              class="flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium"
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

          <!-- GitHub 最新 Release -->
          <div
            class="flex flex-col justify-between rounded-base border border-border bg-card/60 p-4 transition-colors hover:border-border-strong"
          >
            <div class="flex items-center justify-between text-xs text-text-tertiary">
              <span class="flex items-center gap-1.5">
                <Icon icon="ri:github-fill" class="text-text-secondary" />
                GitHub 最新
              </span>
              <a
                v-if="checkData?.release_url"
                :href="checkData.release_url"
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center gap-0.5 text-xs text-primary hover:underline"
              >
                Release
                <Icon icon="ri:external-link-line" class="text-[11px]" />
              </a>
            </div>
            <div class="mt-3">
              <div class="font-mono text-xl font-bold tracking-tight text-foreground">
                {{ checkData?.latest_version || "未获取" }}
              </div>
              <p class="mt-1 text-xs text-text-quaternary">
                <span v-if="checkData?.published_at"> 发布于 {{ formatDate(checkData.published_at) }} </span>
                <span v-else-if="checking">正在检索最新发布…</span>
                <span v-else>暂无发布数据</span>
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
        <div class="flex-1">
          <div class="font-semibold text-state-error">检查更新遇到错误</div>
          <p class="mt-1 text-xs leading-relaxed text-text-secondary">{{ checkData.error }}</p>
          <p class="mt-2 text-xs text-text-tertiary">
            提示：若为私有仓库或触碰 GitHub 匿名 API 速率限制（60次/小时），请在下方配置 GitHub Token。
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
          <div class="flex items-center gap-2 text-base font-semibold text-foreground">
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
                GitHub 发布页
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
              >，按部署方式手动升级：Docker 镜像部署执行 <code>docker compose pull && up -d</code>；二进制部署执行
              <code>deploy/update.sh</code> 或替换 Release 中的新二进制。</span
            >
          </div>
          <span v-if="checkData?.published_at" class="text-text-tertiary">
            发布于 {{ formatDate(checkData.published_at) }}
          </span>
        </div>

        <div v-if="checkData?.release_notes" class="rounded-base border border-border bg-[#0f0f10] p-4">
          <div
            class="custom-scrollbar max-h-72 overflow-y-auto font-mono text-xs leading-relaxed text-text-secondary whitespace-pre-wrap"
          >
            {{ checkData.release_notes }}
          </div>
        </div>
        <div v-else class="py-4 text-center text-xs text-text-quaternary">暂无详细更新日志</div>
      </VCard>

      <!-- 4. GitHub Token 配置卡片 -->
      <VCard>
        <template #header>
          <div class="flex items-center gap-2 text-base font-semibold text-foreground">
            <Icon icon="ri:key-2-line" class="text-lg text-primary" />
            <span>GitHub Token（可选鉴权）</span>
          </div>
          <div v-if="githubToken" class="flex items-center gap-2">
            <VButton size="xs" type="danger" @click="clearToken">
              <template #icon>
                <Icon icon="ri:delete-bin-line" />
              </template>
              清除 Token
            </VButton>
          </div>
        </template>

        <div class="space-y-3">
          <p class="text-xs leading-relaxed text-text-secondary">
            私有仓库需鉴权才能检查更新；填入 Personal Access Token 可访问私有仓库并提升速率上限（从 60次/小时 提升至
            5,000次/小时）。Token 仅保存在本浏览器 localStorage，随请求以 X-GitHub-Token 头发送，不会存储到服务端。
          </p>
          <div class="relative flex items-center">
            <div class="pointer-events-none absolute left-3 flex items-center text-text-quaternary">
              <Icon icon="ri:lock-line" class="text-base" />
            </div>
            <input
              :type="showToken ? 'text' : 'password'"
              :value="githubToken"
              @input="onTokenInput"
              placeholder="ghp_xxx…（留空则使用服务端已配置的 github_token / 匿名）"
              class="w-full rounded-base border border-border bg-card py-2 pl-9 pr-10 text-xs font-mono text-foreground outline-none transition-colors placeholder:font-sans placeholder:text-text-quaternary focus:border-primary"
            />
            <button
              type="button"
              class="absolute right-2.5 flex h-6 w-6 items-center justify-center rounded text-text-quaternary transition-colors hover:bg-accent hover:text-foreground"
              :title="showToken ? '隐藏 Token' : '显示 Token'"
              @click="showToken = !showToken"
            >
              <Icon :icon="showToken ? 'ri:eye-off-line' : 'ri:eye-line'" class="text-sm" />
            </button>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
            <p v-if="githubToken" class="flex items-center gap-1 text-state-success">
              <Icon icon="ri:checkbox-circle-line" />
              已填入，「检查更新」将携带该 Token
            </p>
            <p v-else class="text-text-quaternary">未配置 Token，当前采用匿名请求</p>
            <a
              href="https://github.com/settings/tokens/new?scopes=repo&description=Blog%20System%20Update"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-0.5 text-primary hover:underline"
            >
              去 GitHub 创建 Token
              <Icon icon="ri:external-link-line" class="text-[11px]" />
            </a>
          </div>
        </div>
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
  border-radius: 4px;
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
