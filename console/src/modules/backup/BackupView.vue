<script setup lang="ts">
import { ref, onMounted } from "vue";
import {
  backupApi,
  backupExportUrl,
  backupExportMarkdownUrl,
} from "@/api";
import { Icon } from "@iconify/vue";

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
const importError = ref("");

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
  if (!confirm(`确定导入「${file.name}」？同名 slug 的文章会被覆盖。`)) {
    input.value = "";
    return;
  }
  importing.value = true;
  importResult.value = null;
  importError.value = "";
  try {
    const { data } = await backupApi.importMarkdown(file);
    importResult.value = data;
  } catch (err: any) {
    importError.value = err.response?.data?.error || "导入失败";
  } finally {
    importing.value = false;
    input.value = "";
  }
}

function formatSize(bytes: number) {
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + " MB";
  return (bytes / 1024).toFixed(1) + " KB";
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString("zh-CN", { hour12: false });
}

onMounted(loadStatus);
</script>

<template>
  <div class="p-6 max-w-4xl">
    <h2 class="text-xl font-semibold text-gray-800 mb-6 dark:text-gray-100">备份与导出</h2>

    <div class="space-y-6">
      <!-- 全量备份 -->
      <section class="bg-white rounded-xl border border-gray-200 p-5 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-800 flex items-center gap-2 dark:text-gray-100">
              <Icon icon="ri:database-2-line" class="text-indigo-500" />
              全量备份
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              打包下载数据库快照（文章/评论/瞬间/页面/设置等全部数据）+ 附件目录 + site.yml。
            </p>
          </div>
          <a
            :href="backupExportUrl"
            class="shrink-0 inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
          >
            <Icon icon="ri:download-2-line" />
            下载备份
          </a>
        </div>

        <!-- 自动备份留存 -->
        <div class="mt-4 pt-4 border-t border-gray-100 dark:border-gray-700">
          <p class="text-xs text-gray-500 mb-2 dark:text-gray-400">
            自动备份：{{ autoBackup ? `已开启（每 24 小时一次，保留最近 ${backupKeep} 份，存放于服务器 data/backups/）` : "已关闭（可在 config/app.yml 的 auto_backup 开启）" }}
          </p>
          <table v-if="backups.length" class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-400 dark:text-gray-500">
                <th class="py-1.5 font-medium">文件名</th>
                <th class="py-1.5 font-medium">大小</th>
                <th class="py-1.5 font-medium">时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in backups" :key="b.name" class="border-t border-gray-50 text-gray-600 dark:border-gray-700/50 dark:text-gray-300">
                <td class="py-1.5 font-mono text-xs">{{ b.name }}</td>
                <td class="py-1.5 text-xs">{{ formatSize(b.size) }}</td>
                <td class="py-1.5 text-xs">{{ formatTime(b.mod_time) }}</td>
              </tr>
            </tbody>
          </table>
          <p v-else-if="!loading" class="text-xs text-gray-400 dark:text-gray-500">服务器上暂无自动备份。</p>
        </div>
      </section>

      <!-- Markdown 导出 -->
      <section class="bg-white rounded-xl border border-gray-200 p-5 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-800 flex items-center gap-2 dark:text-gray-100">
              <Icon icon="ri:markdown-line" class="text-indigo-500" />
              Markdown 导出
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              把全部文章（含草稿）导出为带 frontmatter 的 Markdown 文件打包，可用于迁移或再导入。
            </p>
          </div>
          <a
            :href="backupExportMarkdownUrl"
            class="shrink-0 inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
          >
            <Icon icon="ri:download-2-line" />
            导出文章
          </a>
        </div>
      </section>

      <!-- Markdown 导入 -->
      <section class="bg-white rounded-xl border border-gray-200 p-5 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-800 flex items-center gap-2 dark:text-gray-100">
              <Icon icon="ri:upload-cloud-2-line" class="text-indigo-500" />
              Markdown 导入
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              上传单个 .md 文件或 Markdown 导出的 .zip，按文件名（slug）导入文章。
            </p>
            <p class="mt-1 text-xs text-red-500 dark:text-red-400">
              注意：同名 slug 的文章会被上传内容覆盖。
            </p>
          </div>
          <label
            class="shrink-0 inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 cursor-pointer transition-colors"
            :class="{ 'opacity-60 pointer-events-none': importing }"
          >
            <Icon icon="ri:folder-open-line" />
            {{ importing ? "导入中..." : "选择文件" }}
            <input type="file" accept=".md,.zip" class="hidden" @change="handleImport" :disabled="importing" />
          </label>
        </div>

        <!-- 导入结果 -->
        <div v-if="importResult" class="mt-4 p-3 rounded-lg bg-green-50 text-xs text-green-700 dark:bg-green-500/10 dark:text-green-400">
          成功导入 {{ importResult.imported }} 篇：{{ importResult.slugs.join("、") }}
          <template v-if="importResult.skipped.length">
            <br />跳过 {{ importResult.skipped.length }} 项：{{ importResult.skipped.join("；") }}
          </template>
        </div>
        <div v-if="importError" class="mt-4 p-3 rounded-lg bg-red-50 text-xs text-red-600 dark:bg-red-500/10 dark:text-red-400">
          {{ importError }}
        </div>
      </section>

      <!-- 全量恢复说明 -->
      <section class="rounded-xl border border-dashed border-gray-300 p-5 dark:border-gray-600">
        <h3 class="text-sm font-semibold text-gray-600 flex items-center gap-2 dark:text-gray-300">
          <Icon icon="ri:tools-line" />
          如何全量恢复？
        </h3>
        <p class="mt-1 text-xs text-gray-500 leading-relaxed dark:text-gray-400">
          出于安全考虑不提供在线恢复。手动步骤：停止服务 → 解开备份 zip，将 blog.db 覆盖到 data/blog.db、uploads/ 覆盖到
          data/uploads/（如有需要将 site.yml 放回 config/）→ 重新启动服务。
        </p>
      </section>
    </div>
  </div>
</template>
