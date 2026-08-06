<script setup lang="ts">
import { ref, onMounted } from"vue";
import {
 backupApi,
 backupExportUrl,
 backupExportMarkdownUrl,
} from"@/api";
import { Icon } from"@iconify/vue";
import { VPageHeader, VCard, VButton, Dialog, Toast } from"@/components/ui";

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

// 整站恢复（Spec 41）：上传 zip / 服务器快照，双重确认（输入 RESTORE）
const restoring = ref(false);
const restoreConfirmText = ref("");
const pendingRestore = ref<{ kind:"upload" |"snapshot"; file?: File; name?: string } | null>(null);
const restoreStrict = ref(false);

function pickRestoreFile(e: Event) {
 const input = e.target as HTMLInputElement;
 if (!input.files?.length) return;
 pendingRestore.value = { kind:"upload", file: input.files[0] };
 restoreConfirmText.value ="";
 input.value ="";
}

function pickRestoreSnapshot(name: string) {
 pendingRestore.value = { kind:"snapshot", name };
 restoreConfirmText.value ="";
}

async function executeRestore() {
 const p = pendingRestore.value;
 if (!p || restoreConfirmText.value !=="RESTORE") return;
 restoring.value = true;
 try {
 if (p.kind ==="upload" && p.file) {
 await backupApi.restore(p.file, restoreStrict.value);
 } else if (p.kind ==="snapshot" && p.name) {
 await backupApi.restoreSnapshot(p.name);
 }
 Toast.success("恢复完成，前台与搜索索引已同步");
 pendingRestore.value = null;
 loadStatus();
 } catch (err: any) {
 Toast.error(err.response?.data?.error ||"恢复失败");
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
 title: `确定导入「${file.name}」？`,
 description:"同名 slug 的文章会被覆盖。",
 confirmType:"danger",
 onConfirm: async () => {
 importing.value = true;
 importResult.value = null;
 try {
 const { data } = await backupApi.importMarkdown(file);
 importResult.value = data;
 Toast.success(`成功导入 ${data.imported} 篇`);
 } catch (err: any) {
 Toast.error(err.response?.data?.error ||"导入失败");
 } finally {
 importing.value = false;
 input.value ="";
 }
 },
 onCancel: () => {
 input.value ="";
 },
 });
}

function formatSize(bytes: number) {
 if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) +" MB";
 return (bytes / 1024).toFixed(1) +" KB";
}

function formatTime(iso: string) {
 return new Date(iso).toLocaleString("zh-CN", { hour12: false });
}

onMounted(loadStatus);
</script>

<template>
 <div>
 <VPageHeader title="备份与导出">
 <template #icon>
 <Icon icon="ri:archive-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="m-0 max-w-4xl space-y-4 md:m-4">
 <!-- 全量备份 -->
 <VCard>
 <template #header>
 <span class="flex items-center gap-2 text-base font-bold text-foreground">
 <Icon icon="ri:database-2-line" class="text-lg text-primary" />
 全量备份
 </span>
 </template>
 <div class="flex items-start justify-between gap-4">
 <p class="text-xs text-text-tertiary">
 打包下载数据库快照（文章/评论/瞬间/页面/设置等全部数据）+ 附件目录 + site.yml。
 </p>
 <a :href="backupExportUrl" class="shrink-0">
 <VButton type="secondary">
 <template #icon>
 <Icon icon="ri:download-2-line" class="h-full w-full" />
 </template>
 下载备份
 </VButton>
 </a>
 </div>

 <div class="mt-4 border-t border-border pt-4">
 <p class="mb-2 text-xs text-text-tertiary">
 自动备份：{{ autoBackup ? `已开启（每 24 小时一次，保留最近 ${backupKeep} 份，存放于服务器 data/backups/）` :"已关闭（可在 config/app.yml 的 auto_backup 开启）" }}
 </p>
 <table v-if="backups.length" class="w-full text-sm">
 <thead>
 <tr class="text-left text-xs text-text-quaternary">
 <th class="py-1.5 font-medium">文件名</th>
 <th class="py-1.5 font-medium">大小</th>
 <th class="py-1.5 font-medium">时间</th>
 <th class="py-1.5"></th>
 </tr>
 </thead>
 <tbody>
 <tr v-for="b in backups" :key="b.name" class="border-t border-border text-text-tertiary /50">
 <td class="py-1.5 font-mono text-xs">{{ b.name }}</td>
 <td class="py-1.5 text-xs">{{ formatSize(b.size) }}</td>
 <td class="py-1.5 text-xs">{{ formatTime(b.mod_time) }}</td>
 <td class="py-1.5 text-right">
 <button
 class="text-xs text-red-500 hover:text-red-600 hover:underline text-red-400"
 :disabled="restoring"
 @click="pickRestoreSnapshot(b.name)"
 >恢复</button>
 </td>
 </tr>
 </tbody>
 </table>
 <p v-else-if="!loading" class="text-xs text-text-quaternary">服务器上暂无自动备份。</p>
 </div>
 </VCard>

 <!-- Markdown 导出 -->
 <VCard>
 <template #header>
 <span class="flex items-center gap-2 text-base font-bold text-foreground">
 <Icon icon="ri:markdown-line" class="text-lg text-primary" />
 Markdown 导出
 </span>
 </template>
 <div class="flex items-start justify-between gap-4">
 <p class="text-xs text-text-tertiary">
 把全部文章（含草稿）导出为带 frontmatter 的 Markdown 文件打包，可用于迁移或再导入。
 </p>
 <a :href="backupExportMarkdownUrl" class="shrink-0">
 <VButton type="secondary">
 <template #icon>
 <Icon icon="ri:download-2-line" class="h-full w-full" />
 </template>
 导出文章
 </VButton>
 </a>
 </div>
 </VCard>

 <!-- Markdown 导入 -->
 <VCard>
 <template #header>
 <span class="flex items-center gap-2 text-base font-bold text-foreground">
 <Icon icon="ri:upload-cloud-2-line" class="text-lg text-primary" />
 Markdown 导入
 </span>
 </template>
 <div class="flex items-start justify-between gap-4">
 <div>
 <p class="text-xs text-text-tertiary">
 上传单个 .md 文件或 Markdown 导出的 .zip，按文件名（slug）导入文章。
 </p>
 <p class="mt-1 text-xs text-red-500 text-red-400">
 注意：同名 slug 的文章会被上传内容覆盖。
 </p>
 </div>
 <label class="shrink-0 cursor-pointer">
 <VButton type="secondary" :loading="importing" @click.prevent>
 <template #icon>
 <Icon icon="ri:folder-open-line" class="h-full w-full" />
 </template>
 选择文件
 </VButton>
 <input type="file" accept=".md,.zip" class="hidden" :disabled="importing" @change="handleImport" />
 </label>
 </div>

 <div
 v-if="importResult"
 class="mt-4 rounded-base bg-green-50 p-3 text-xs text-green-700 bg-green-500/10 text-green-400"
 >
 成功导入 {{ importResult.imported }} 篇：{{ importResult.slugs.join("、") }}
 <template v-if="importResult.skipped.length">
 <br />跳过 {{ importResult.skipped.length }} 项：{{ importResult.skipped.join("；") }}
 </template>
 </div>
 </VCard>

 <!-- 整站恢复（Spec 41） -->
 <VCard>
 <template #header>
 <span class="flex items-center gap-2 text-base font-bold text-foreground">
 <Icon icon="ri:restart-line" class="text-lg text-red-500" />
 从备份恢复
 </span>
 </template>
 <div class="flex items-start justify-between gap-4">
 <div>
 <p class="text-xs text-text-tertiary">
 上传全量备份 zip（blog-backup-*.zip）一键还原数据库、附件与站点配置；也可在上方列表对服务器留存的备份直接点「恢复」。
 </p>
 <p class="mt-1 text-xs text-red-500 text-red-400">
 恢复会覆盖当前全部数据（执行前自动留存 .pre-restore 兜底快照）。
 </p>
 <label class="mt-2 flex items-center gap-1.5 text-xs text-text-tertiary">
 <input v-model="restoreStrict" type="checkbox" class="h-3.5 w-3.5" />
 严格模式：先清空附件目录再解包（默认保留包外既有附件）
 </label>
 </div>
 <label class="shrink-0 cursor-pointer">
 <VButton type="danger" :loading="restoring" @click.prevent>
 <template #icon>
 <Icon icon="ri:upload-cloud-2-line" class="h-full w-full" />
 </template>
 选择备份包
 </VButton>
 <input type="file" accept=".zip" class="hidden" :disabled="restoring" @change="pickRestoreFile" />
 </label>
 </div>

 <!-- 双重确认：输入 RESTORE 才可执行 -->
 <div
 v-if="pendingRestore"
 class="mt-4 rounded-base border border-red-200 bg-red-50 p-4 border-red-500/30 bg-red-500/10"
 >
 <p class="text-xs font-medium text-red-700 text-red-300">
 即将恢复：{{ pendingRestore.kind ==="upload" ? pendingRestore.file?.name : pendingRestore.name }}
 </p>
 <p class="mt-1 text-xs text-red-600 text-red-400">该操作覆盖当前数据，请输入 <b>RESTORE</b> 确认：</p>
 <div class="mt-2 flex items-center gap-2">
 <input
 v-model="restoreConfirmText"
 type="text"
 placeholder="RESTORE"
 class="w-40 rounded-base border border-red-300 bg-card px-2 py-1 text-xs font-mono outline-none transition-colors focus:border-red-500 border-red-500/40"
 />
 <VButton
 type="danger"
 size="sm"
 :disabled="restoreConfirmText !== 'RESTORE'"
 :loading="restoring"
 @click="executeRestore"
 >执行恢复</VButton>
 <VButton size="sm" @click="pendingRestore = null">取消</VButton>
 </div>
 </div>
 </VCard>

 <!-- 全量恢复说明 -->
 <div class="rounded-base border border-dashed border-[var(--color-border-strong)] p-5">
 <h3 class="flex items-center gap-2 text-sm font-semibold text-text-tertiary">
 <Icon icon="ri:tools-line" />
 离线恢复（备选）
 </h3>
 <p class="mt-1 text-xs leading-relaxed text-text-tertiary">
 服务无法启动时的手动步骤：停止服务 → 解开备份 zip，将 blog.db 覆盖到 data/blog.db、uploads/ 覆盖到
 data/uploads/（如有需要将 site.yml 放回 config/）→ 重新启动服务。
 </p>
 </div>
 </div>
 </div>
</template>
