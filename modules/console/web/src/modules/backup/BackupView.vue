<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import {
  backupApi,
  backupExportUrl,
  backupExportMarkdownUrl,
} from "@/api";
import { formatBytes, formatRelative } from "@/lib/format";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, VLoading, VEmpty, VModal, Dialog, Toast } from "@/components/ui";

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
const importResult = ref<{
  imported: number;
  slugs: string[];
  skipped: string[];
} | null>(null);

// 整站恢复（双重确认：输入 RESTORE）
const restoring = ref(false);
const restoreConfirmText = ref("");
const pendingRestore = ref<{ kind: "upload" | "snapshot"; file?: File; name?: string } | null>(null);
const restoreStrict = ref(false);

function pickRestoreFile(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  pendingRestore.value = { kind: "upload", file: input.files[0] };
  restoreConfirmText.value = "";
  input.value = "";
}

function pickRestoreSnapshot(name: string) {
  pendingRestore.value = { kind: "snapshot", name };
  restoreConfirmText.value = "";
}

async function executeRestore() {
  const p = pendingRestore.value;
  if (!p || restoreConfirmText.value !== "RESTORE") return;
  restoring.value = true;
  try {
    if (p.kind === "upload" && p.file) {
      await backupApi.restore(p.file, restoreStrict.value);
    } else if (p.kind === "snapshot" && p.name) {
      await backupApi.restoreSnapshot(p.name);
    }
    Toast.success("恢复完成，前台与搜索索引已重新同步");
    pendingRestore.value = null;
    loadStatus();
  } catch (err: any) {
    Toast.error(err.response?.data?.error || "恢复失败");
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
  } catch {} finally {
    loading.value = false;
  }
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
      } catch (err: any) {
        Toast.error(err.response?.data?.error || "导入失败");
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

onMounted(loadStatus);
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

    <div class="mx-auto max-w-[1200px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <div class="stats-card">
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:database-2-line" />
          </div>
          <div>
            <div class="stats-label">本地快照备份</div>
            <div class="stats-value">{{ stats.totalSnapshots }} <span class="stats-sub">份快照文件</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:hard-drive-2-line" />
          </div>
          <div>
            <div class="stats-label">快照占用空间</div>
            <div class="stats-value text-[#30D158]">{{ stats.totalBytes }} <span class="stats-sub">磁盘已用</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30B0C7] bg-[#30B0C7]/10">
            <Icon icon="ri:timer-line" />
          </div>
          <div>
            <div class="stats-label">定时自动备份</div>
            <div class="stats-value text-[#30B0C7]">{{ stats.autoBackup }} <span class="stats-sub">每 24 小时执行</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
            <Icon icon="ri:history-line" />
          </div>
          <div>
            <div class="stats-label">快照轮转策略</div>
            <div class="stats-value">{{ stats.backupKeep }} <span class="stats-sub">自动清理旧版本</span></div>
          </div>
        </div>
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
              打包内容包含 SQLite 核心数据库（文章/评论/瞬间/页面/设置/标签等全部数据）、<code>attachments/</code> 媒体文件目录以及 <code>site.yml</code> 站点配置。
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
                    <td class="py-2.5 px-3 font-mono font-medium text-foreground truncate max-w-[200px]" :title="b.name">
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
                        :disabled="restoring"
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
              将全部文章（含草稿）打包为带 YAML Frontmatter 的纯 <code>.md</code> 文件压缩包，便于迁移至 Hexo、Hugo 等静态博客或外部备份。
            </p>
          </VCard>

          <!-- Markdown 文章导入 -->
          <VCard title="Markdown 文章导入">
            <p class="text-xs leading-relaxed text-text-tertiary mb-3">
              支持上传单个 <code>.md</code> 文件或包含多篇 Markdown 文章的 <code>.zip</code> 压缩包，系统将自动解析元数据并写入数据库。
            </p>

            <div class="flex items-center justify-between gap-3 border-t border-border/60 pt-3">
              <span class="text-[11px] text-[#FF9500] flex items-center gap-1">
                <Icon icon="ri:alert-line" /> 同名 Slug 将被直接覆盖
              </span>

              <label class="cursor-pointer">
                <input
                  type="file"
                  accept=".md,.zip"
                  class="hidden"
                  :disabled="importing"
                  @change="handleImport"
                />
                <VButton size="sm" :loading="importing" @click.prevent>
                  <template #icon><Icon icon="ri:upload-2-line" /></template>
                  选择文件导入
                </VButton>
              </label>
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
              <label class="cursor-pointer">
                <input
                  type="file"
                  accept=".zip"
                  class="hidden"
                  :disabled="restoring"
                  @change="pickRestoreFile"
                />
                <VButton size="sm" type="danger" @click.prevent>
                  <template #icon><Icon icon="ri:restart-line" /></template>
                  上传备份并恢复
                </VButton>
              </label>
            </div>
          </VCard>
        </div>
      </div>
    </div>

    <!-- ============ 危险操作：整站恢复双重确认弹窗 ============ -->
    <VModal
      :visible="!!pendingRestore"
      title="⚠️ 确认从备份恢复整站数据"
      :width="480"
      @update:visible="pendingRestore = null"
    >
      <div v-if="pendingRestore" class="space-y-4 text-xs">
        <div class="rounded-xl border border-state-error/30 bg-state-error/10 p-3.5 text-foreground leading-relaxed">
          <div class="font-bold text-state-error text-sm flex items-center gap-1.5 mb-1">
            <Icon icon="ri:error-warning-fill" /> 高风险操作警告
          </div>
          当前数据库中的所有文章、评论、瞬间和配置都将被备份中的数据<strong>全量覆盖</strong>，且此操作不可逆！
        </div>

        <div class="rounded-xl border border-border bg-card p-3 font-mono">
          <div class="text-text-quaternary">恢复目标来源：</div>
          <div class="font-semibold text-foreground mt-1">
            {{ pendingRestore.kind === 'upload' ? `本地上传文件：${pendingRestore.file?.name}` : `服务器快照：${pendingRestore.name}` }}
          </div>
        </div>

        <div>
          <label class="mb-1.5 block font-medium text-text-tertiary">
            请在下方输入框中完整输入 <strong class="text-state-error font-mono">RESTORE</strong> 以确认执行：
          </label>
          <input
            v-model="restoreConfirmText"
            class="apple-input font-mono text-sm"
            placeholder="RESTORE"
            autofocus
          />
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="pendingRestore = null">取消</VButton>
          <VButton
            type="danger"
            :loading="restoring"
            :disabled="restoreConfirmText !== 'RESTORE'"
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
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
