<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { backupApi, backupExportUrl, backupExportMarkdownUrl, RestoreTaskHttpError, type RestoreTask } from "@/api";
import { formatBytes, formatRelative } from "@/lib/format";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VLoading,
  VEmpty,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";
import ConsoleStatsCard from "@/components/ConsoleStatsCard.vue";
import { useAuthStore } from "@/stores/auth";
import { queryClient } from "@/api/query-client";
import router from "@/router";

interface BackupInfo {
  name: string;
  size: number;
  mod_time: string;
}

const autoBackup = ref(false);
const backupKeep = ref(0);
const backups = ref<BackupInfo[]>([]);
const loading = ref(false);

const importing = ref(false);
const importFileInput = ref<HTMLInputElement | null>(null);
const importResult = ref<{
  imported: number;
  slugs: string[];
  skipped: string[];
} | null>(null);

// 整站恢复（双重确认：输入 RESTORE）
const restoring = ref(false);
const restoreFileInput = ref<HTMLInputElement | null>(null);
const restoreConfirmText = ref("");
const pendingRestore = ref<{
  kind: "upload" | "snapshot";
  file?: File;
  name?: string;
  idempotencyKey: string;
} | null>(null);
const restoreStrict = ref(false);
const restoreTask = ref<RestoreTask | null>(null);
const restoreTasks = ref<RestoreTask[]>([]);
const restorePollTimer = ref<ReturnType<typeof window.setTimeout> | null>(null);
const restoreTaskStorageKey = "console.restore.task_id";
const restorePollError = ref("");
const componentUnmounted = ref(false);
const auth = useAuthStore();

function restoreTaskMessage(task: RestoreTask): string {
  const labels: Record<string, string> = {
    preparing: "准备中",
    waiting: "等待维护窗口",
    applying: "恢复数据中",
    rebuilding: "重建索引中",
    succeeded: "已完成",
    rolled_back: "已回滚",
    recovery_required: "需要人工恢复",
  };
  return labels[task.status] || task.status;
}

function isRestoreTerminal(status: string): boolean {
  return ["succeeded", "rolled_back", "recovery_required"].includes(status);
}

const restoreBusy = computed(
  () => restoring.value || (!!restoreTask.value && !isRestoreTerminal(restoreTask.value.status)),
);

function stopRestorePolling() {
  if (restorePollTimer.value !== null) {
    window.clearTimeout(restorePollTimer.value);
    restorePollTimer.value = null;
  }
}

function scheduleRestorePoll() {
  if (componentUnmounted.value || !localStorage.getItem(restoreTaskStorageKey)) return;
  stopRestorePolling();
  restorePollTimer.value = window.setTimeout(pollRestoreTask, 2000);
}

async function readMaintenanceState(): Promise<boolean> {
  const response = await fetch("/api/maintenance", { credentials: "same-origin" });
  if (!response.ok) throw new Error(`维护状态请求失败（${response.status}）`);
  const body = (await response.json()) as { maintenance?: unknown };
  if (typeof body.maintenance !== "boolean") throw new Error("维护状态响应格式错误");
  return body.maintenance;
}

function handleRestoreTaskUnauthorized() {
  auth.clearSession();
  queryClient.clear();
  stopRestorePolling();
  Dialog.warning({
    title: "恢复任务需要重新登录",
    description: "请使用备份账号重新登录后继续查询恢复结果。任务仍会在后台运行。",
    confirmText: "去备份账号登录",
    cancelText: "稍后处理",
    onConfirm: async () => {
      await router.replace({ path: "/login", query: { redirect: router.currentRoute.value.fullPath } });
    },
  });
}

async function pollRestoreTask() {
  if (componentUnmounted.value) return;
  const taskID = localStorage.getItem(restoreTaskStorageKey);
  if (!taskID) return;
  try {
    restorePollError.value = "";
    const maintenance = await readMaintenanceState();
    if (componentUnmounted.value) return;
    if (!maintenance) {
      const task = await backupApi.getRestoreTask(taskID);
      if (componentUnmounted.value) return;
      restoreTask.value = task;
      if (isRestoreTerminal(task.status)) {
        localStorage.removeItem(restoreTaskStorageKey);
        stopRestorePolling();
        if (task.status === "succeeded") Toast.success("恢复完成，前台与搜索索引已重新同步");
        else Toast.error(task.error || `恢复${restoreTaskMessage(task)}`);
        if (!componentUnmounted.value) await loadRestoreTasks();
        return;
      }
    }
  } catch (error) {
    if (error instanceof RestoreTaskHttpError && error.status === 401) {
      handleRestoreTaskUnauthorized();
      return;
    }
    restorePollError.value = error instanceof Error ? error.message : "恢复状态暂时无法查询，将自动重试";
  }
  scheduleRestorePoll();
}

function beginRestorePolling(task: RestoreTask) {
  restoreTask.value = task;
  localStorage.setItem(restoreTaskStorageKey, task.id);
  restorePollError.value = "";
  stopRestorePolling();
  restorePollTimer.value = window.setTimeout(pollRestoreTask, 500);
}

async function loadRestoreTasks() {
  try {
    const { data } = await backupApi.listRestoreTasks();
    restoreTasks.value = data.tasks || [];
  } catch {
    restoreTasks.value = [];
  }
}

function openRestorePicker() {
  const input = restoreFileInput.value;
  if (!input || restoreBusy.value) return;
  input.value = "";
  input.click();
}

function pickRestoreFile(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  pendingRestore.value = { kind: "upload", file: input.files[0], idempotencyKey: crypto.randomUUID() };
  restoreConfirmText.value = "";
  input.value = "";
}

function pickRestoreSnapshot(name: string) {
  if (restoreBusy.value) return;
  pendingRestore.value = { kind: "snapshot", name, idempotencyKey: crypto.randomUUID() };
  restoreConfirmText.value = "";
}

async function executeRestore() {
  const p = pendingRestore.value;
  if (!p || restoreBusy.value || restoreConfirmText.value !== "RESTORE") return;
  restoring.value = true;
  try {
    let response;
    if (p.kind === "upload" && p.file) {
      response = await backupApi.startRestore(p.file, restoreStrict.value, p.idempotencyKey);
    } else if (p.kind === "snapshot" && p.name) {
      response = await backupApi.startSnapshotRestore(p.name, restoreStrict.value, p.idempotencyKey);
    } else {
      return;
    }
    beginRestorePolling(response.data);
    Toast.success("恢复任务已提交，系统将在维护完成后更新结果");
    pendingRestore.value = null;
  } catch (error) {
    Toast.error(error instanceof Error ? error.message : "恢复任务提交失败，请重试");
  } finally {
    restoring.value = false;
  }
}

async function loadStatus() {
  loading.value = true;
  try {
    const { data } = await backupApi.status();
    autoBackup.value = data.auto_backup;
    backupKeep.value = data.backup_keep;
    backups.value = data.backups || [];
    await loadRestoreTasks();
  } catch {
  } finally {
    loading.value = false;
  }
}

function openImportPicker() {
  const input = importFileInput.value;
  if (!input || importing.value) return;
  input.value = "";
  input.click();
}

async function handleImport(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  const file = input.files[0];
  Dialog.warning({
    title: `确定导入「${file.name}」吗？`,
    description: "同名 Slug 的文章将被导入的文件内容直接覆盖，请确认后再操作。",
    confirmType: "danger",
    onConfirm: async () => {
      importing.value = true;
      importResult.value = null;
      try {
        const { data } = await backupApi.importMarkdown(file);
        importResult.value = data;
        Toast.success(`成功导入 ${data.imported} 篇文章`);
      } catch {
      } finally {
        importing.value = false;
        input.value = "";
      }
    },
    onCancel: () => {
      input.value = "";
    },
  });
}

// ---------- 统计数据 ----------
const stats = computed(() => {
  const totalSnapshots = backups.value.length;
  let totalBytes = 0;
  for (const b of backups.value) totalBytes += b.size || 0;
  return {
    totalSnapshots,
    totalBytes: formatBytes(totalBytes),
    autoBackup: autoBackup.value ? "已启用" : "未开启",
    backupKeep: backupKeep.value > 0 ? `保留 ${backupKeep.value} 份` : "不限",
  };
});

async function resumeRestoreTask(taskID: string): Promise<boolean> {
  restoreTask.value = { id: taskID, status: "waiting", created_at: "", updated_at: "", has_preimage: false };
  try {
    const maintenance = await readMaintenanceState();
    if (componentUnmounted.value) return true;
    if (maintenance) {
      scheduleRestorePoll();
      return true;
    }
    const task = await backupApi.getRestoreTask(taskID);
    if (componentUnmounted.value) return true;
    restoreTask.value = task;
    if (isRestoreTerminal(task.status)) {
      localStorage.removeItem(restoreTaskStorageKey);
      restoreTask.value = null;
      return false;
    }
    beginRestorePolling(task);
    return true;
  } catch (error) {
    if (error instanceof RestoreTaskHttpError && error.status === 401) {
      handleRestoreTaskUnauthorized();
      return true;
    }
    restorePollError.value = error instanceof Error ? error.message : "恢复状态暂时无法查询，将自动重试";
    scheduleRestorePoll();
    return true;
  }
}

onMounted(async () => {
  componentUnmounted.value = false;
  const taskID = localStorage.getItem(restoreTaskStorageKey);
  if (taskID) {
    const active = await resumeRestoreTask(taskID);
    if (active || componentUnmounted.value) return;
  }
  await loadStatus();
});
onBeforeUnmount(() => {
  componentUnmounted.value = true;
  stopRestorePolling();
});
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="备份与导出">
      <template #icon>
        <Icon icon="ri:archive-line" class="text-primary" />
      </template>
      <template #subtitle>打包全站数据快照、Markdown 文章导入导出及一键灾备还原</template>
      <template #actions>
        <VButton @click="loadStatus">
          <template #icon><Icon icon="ri:refresh-line" /></template>
          刷新状态
        </VButton>
      </template>
    </VPageHeader>

    <div
      v-if="restoreTask && !isRestoreTerminal(restoreTask.status)"
      class="mx-auto max-w-[1200px] px-4 pt-5 sm:px-6 md:px-8"
    >
      <div class="rounded-xl border border-primary/30 bg-primary/10 px-4 py-3 text-sm text-foreground">
        <div class="flex items-center gap-2 font-semibold">
          <Icon icon="ri:loader-4-line" class="animate-spin text-primary" />
          整站恢复：{{ restoreTaskMessage(restoreTask) }}
        </div>
        <div class="mt-1 text-xs text-text-tertiary">恢复期间请保持页面打开，维护窗口结束后会自动查询最终结果。</div>
        <div v-if="restorePollError" class="mt-2 text-xs text-state-error">{{ restorePollError }}，正在重试。</div>
      </div>
    </div>

    <div class="mx-auto max-w-[1200px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <ConsoleStatsCard
          label="本地快照备份"
          :value="stats.totalSnapshots"
          sub="份快照文件"
          icon="ri:database-2-line"
          color="primary"
        />
        <ConsoleStatsCard
          label="快照占用空间"
          :value="stats.totalBytes"
          sub="磁盘已用"
          icon="ri:hard-drive-2-line"
          color="success"
          value-class="text-[#30D158]"
        />
        <ConsoleStatsCard
          label="定时自动备份"
          :value="stats.autoBackup"
          sub="每 24 小时执行"
          icon="ri:timer-line"
          color="cyan"
        />
        <ConsoleStatsCard
          label="快照轮转策略"
          :value="stats.backupKeep"
          sub="自动清理旧版本"
          icon="ri:history-line"
          color="purple"
        />
      </div>

      <div class="grid grid-cols-1 gap-5 lg:grid-cols-12 lg:items-start">
        <!-- 左侧：全量导出与快照管理 -->
        <div class="space-y-5 lg:col-span-7">
          <!-- 全量数据导出 -->
          <VCard title="全量数据打包下载">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-bold text-foreground text-sm flex items-center gap-2">
                  <Icon icon="ri:folder-zip-line" class="text-primary text-base" />
                  全量整站备份 (Full Backup)
                </span>
                <a :href="backupExportUrl" class="shrink-0" download>
                  <VButton type="primary" size="sm">
                    <template #icon><Icon icon="ri:download-2-line" /></template>
                    立即打包下载
                  </VButton>
                </a>
              </div>
            </template>
            <p class="text-xs leading-relaxed text-text-tertiary">
              打包内容包含 SQLite 核心数据库（文章/评论/瞬间/页面/设置/标签等全部数据）、<code>attachments/</code>
              媒体文件目录以及 <code>site.yml</code> 站点配置。
            </p>
          </VCard>

          <!-- 服务器自动快照列表 -->
          <VCard title="服务器快照列表">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-bold text-foreground text-sm flex items-center gap-2">
                  <Icon icon="ri:server-line" class="text-primary text-base" />
                  服务器快照留存 (data/backups/)
                </span>
                <span class="text-xs font-mono text-text-quaternary">{{ backups.length }} 份</span>
              </div>
            </template>

            <VLoading v-if="loading" />
            <VEmpty
              v-else-if="!backups.length"
              title="暂无服务器快照"
              message="可在 config/app.yml 中开启 auto_backup，系统将每 24 小时自动生成快照"
            />

            <div v-else class="overflow-x-auto">
              <table class="w-full text-left text-xs">
                <thead>
                  <tr class="border-b border-border text-text-quaternary font-medium">
                    <th class="py-2.5 px-3">快照文件名</th>
                    <th class="py-2.5 px-3">文件大小</th>
                    <th class="py-2.5 px-3">创建时间</th>
                    <th class="py-2.5 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border/60">
                  <tr v-for="b in backups" :key="b.name" class="hover:bg-secondary/40 transition-colors">
                    <td
                      class="py-2.5 px-3 font-mono font-medium text-foreground truncate max-w-[200px]"
                      :title="b.name"
                    >
                      {{ b.name }}
                    </td>
                    <td class="py-2.5 px-3 font-mono text-text-tertiary">
                      {{ formatBytes(b.size) }}
                    </td>
                    <td class="py-2.5 px-3 text-text-quaternary">
                      {{ formatRelative(b.mod_time) }}
                    </td>
                    <td class="py-2.5 px-3 text-right">
                      <button
                        type="button"
                        class="rounded-lg px-2.5 py-1 text-xs font-medium text-[#FF453A] hover:bg-[#FF453A]/10 transition-colors"
                        :disabled="restoreBusy"
                        @click="pickRestoreSnapshot(b.name)"
                      >
                        还原快照
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </VCard>

          <VCard title="恢复任务记录">
            <VEmpty v-if="!restoreTasks.length" title="暂无恢复任务" message="提交整站恢复后，任务状态会显示在这里" />
            <div v-else class="overflow-x-auto">
              <table class="w-full text-left text-xs">
                <thead>
                  <tr class="border-b border-border text-text-quaternary font-medium">
                    <th class="py-2.5 px-3">任务</th>
                    <th class="py-2.5 px-3">状态</th>
                    <th class="py-2.5 px-3">时间</th>
                    <th class="py-2.5 px-3">恢复前快照</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border/60">
                  <tr v-for="task in restoreTasks" :key="task.id">
                    <td class="py-2.5 px-3 font-mono text-text-tertiary">{{ task.id.slice(0, 12) }}</td>
                    <td class="py-2.5 px-3 font-medium">{{ restoreTaskMessage(task) }}</td>
                    <td class="py-2.5 px-3 text-text-quaternary">
                      {{ task.updated_at ? formatRelative(task.updated_at) : "-" }}
                    </td>
                    <td class="py-2.5 px-3">{{ task.has_preimage ? "已生成" : "-" }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </VCard>
        </div>

        <!-- 右侧：Markdown 导入导出与外部恢复 -->
        <div class="space-y-5 lg:col-span-5">
          <!-- Markdown 文章导出 -->
          <VCard title="Markdown 文章归档导出">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-bold text-foreground text-sm flex items-center gap-2">
                  <Icon icon="ri:markdown-line" class="text-primary text-base" />
                  文章 Markdown 导出
                </span>
                <a :href="backupExportMarkdownUrl" class="shrink-0" download>
                  <VButton size="sm">
                    <template #icon><Icon icon="ri:file-download-line" /></template>
                    导出全部文章
                  </VButton>
                </a>
              </div>
            </template>
            <p class="text-xs leading-relaxed text-text-tertiary">
              将全部文章（含草稿）打包为带 YAML Frontmatter 的纯 <code>.md</code> 文件压缩包，便于迁移至 Hexo、Hugo
              等静态博客或外部备份。
            </p>
          </VCard>

          <!-- Markdown 文章导入 -->
          <VCard title="Markdown 文章导入">
            <p class="text-xs leading-relaxed text-text-tertiary mb-3">
              支持上传单个 <code>.md</code> 文件或包含多篇 Markdown 文章的
              <code>.zip</code> 压缩包，系统将自动解析元数据并写入数据库。
            </p>

            <div class="flex items-center justify-between gap-3 border-t border-border/60 pt-3">
              <span class="text-[11px] text-[#FF9500] flex items-center gap-1">
                <Icon icon="ri:alert-line" /> 同名 Slug 将被直接覆盖
              </span>

              <input
                ref="importFileInput"
                type="file"
                accept=".md,.zip"
                class="hidden"
                :disabled="importing"
                @change="handleImport"
              />
              <VButton size="sm" :loading="importing" @click="openImportPicker">
                <template #icon><Icon icon="ri:upload-2-line" /></template>
                选择文件导入
              </VButton>
            </div>

            <!-- 导入结果卡片 -->
            <div
              v-if="importResult"
              class="mt-3 rounded-xl border border-[#30D158]/30 bg-[#30D158]/10 p-3 text-xs text-foreground"
            >
              <div class="font-bold text-[#30D158] flex items-center gap-1">
                <Icon icon="ri:checkbox-circle-fill" /> 成功导入 {{ importResult.imported }} 篇文章
              </div>
              <div v-if="importResult.slugs.length" class="mt-1 font-mono text-[11px] text-text-tertiary truncate">
                {{ importResult.slugs.join("、") }}
              </div>
              <div v-if="importResult.skipped.length" class="mt-1 text-[11px] text-[#FF9500]">
                跳过 {{ importResult.skipped.length }} 篇：{{ importResult.skipped.join("；") }}
              </div>
            </div>
          </VCard>

          <!-- 上传 Zip 恢复全站 -->
          <VCard title="上传本地备份包整站恢复">
            <p class="text-xs leading-relaxed text-text-tertiary mb-3">
              可选择先前下载的 <code>blog-backup-*.zip</code> 完整备份包上传并直接覆盖恢复全站。
            </p>

            <div class="flex items-center justify-between border-t border-border/60 pt-3">
              <span class="text-xs text-text-quaternary">支持全量覆盖还原</span>
              <input
                ref="restoreFileInput"
                type="file"
                accept=".zip"
                class="hidden"
                :disabled="restoreBusy"
                @change="pickRestoreFile"
              />
              <VButton size="sm" type="danger" :disabled="restoreBusy" @click="openRestorePicker">
                <template #icon><Icon icon="ri:restart-line" /></template>
                上传备份并恢复
              </VButton>
            </div>
          </VCard>
        </div>
      </div>
    </div>

    <!-- ============ 危险操作：整站恢复双重确认弹窗 ============ -->
    <VModal
      :visible="!!pendingRestore"
      title="确认从备份恢复整站数据"
      :width="480"
      @update:visible="pendingRestore = null"
    >
      <div v-if="pendingRestore" class="space-y-4 text-xs">
        <div class="rounded-xl border border-state-error/30 bg-state-error/10 p-3.5 text-foreground leading-relaxed">
          <div class="font-bold text-state-error text-sm flex items-center gap-1.5 mb-1">
            <Icon icon="ri:error-warning-fill" /> 高风险操作警告
          </div>
          当前数据库中的所有文章、评论、瞬间和配置都将被备份中的数据<strong>全量覆盖</strong>，恢复期间站点将暂停访问。系统会保存恢复前快照；失败时自动补偿，补偿未完成时继续保持维护。
        </div>

        <div class="rounded-xl border border-border bg-card p-3 font-mono">
          <div class="text-text-quaternary">恢复目标来源：</div>
          <div class="font-semibold text-foreground mt-1">
            {{
              pendingRestore.kind === "upload"
                ? `本地上传文件：${pendingRestore.file?.name}`
                : `服务器快照：${pendingRestore.name}`
            }}
          </div>
        </div>

        <div>
          <label class="mb-1.5 block font-medium text-text-tertiary">
            请在下方输入框中完整输入 <strong class="text-state-error font-mono">RESTORE</strong> 以确认执行：
          </label>
          <input v-model="restoreConfirmText" class="apple-input font-mono text-sm" placeholder="RESTORE" autofocus />
        </div>

        <label class="flex items-center gap-2 text-text-tertiary">
          <input v-model="restoreStrict" type="checkbox" class="accent-primary" />
          严格恢复：仅保留备份中的附件，移除备份之外的附件
        </label>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="pendingRestore = null">取消</VButton>
          <VButton
            type="danger"
            :loading="restoring"
            :disabled="restoreBusy || restoreConfirmText !== 'RESTORE'"
            @click="executeRestore"
          >
            确认覆盖并恢复
          </VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
</style>
