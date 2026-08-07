<script setup lang="ts">
import { ref, computed } from"vue";
import { useQuery, useQueryClient } from"@tanstack/vue-query";
import { queryKeys } from"@/api/query-client";
import { linkApi } from"@/api";
import { Icon } from"@iconify/vue";
import {
 VPageHeader,
 VButton,
 VEmpty,
 VLoading,
 VModal,
 Dialog,
 Toast,
} from"@/components/ui";

interface LinkRow {
 id: number;
 group_title: string;
 name: string;
 url: string;
 avatar: string;
 description: string;
 rss: string;
 label: string;
 color: string;
 sort: number;
}

const queryClient = useQueryClient();

const { data: queryData, isLoading } = useQuery({
 queryKey: queryKeys.friends(),
 queryFn: () => linkApi.list().then(({ data }) => data),
});

const links = computed<LinkRow[]>(() => queryData.value?.items || []);

const showModal = ref(false);
const saving = ref(false);
const error = ref("");
const editing = ref<Partial<LinkRow>>({});
const menuOpenId = ref<number | null>(null);

// 连通性检测状态（批注：对齐 Halo“检测全部”）：存 {ok, status}，status 为 HTTP 状态码（0=无法访问）
const checkStatus = ref<Record<number, { ok: boolean; status: number }>>({});
const checking = ref(false);
async function runCheck() {
 checking.value = true;
 try {
 const { data } = await linkApi.check();
 const map: Record<number, { ok: boolean; status: number }> = {};
 let okN = 0;
 let errN = 0;
 for (const r of data.items || []) {
 map[r.id] = { ok: r.ok, status: r.status };
 r.ok ? okN++ : errN++;
 }
 checkStatus.value = map;
 Toast.success("检测完成", { description: `正常 ${okN} · 异常 ${errN}` });
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"检测失败");
 } finally {
 checking.value = false;
 }
}

/** 异常原因提示：HTTP 状态码或“无法访问” */
function errorReason(id: number): string {
 const s = checkStatus.value[id]?.status;
 return s && s > 0 ? `HTTP ${s}` :"无法访问";
}

// 状态筛选：全部 / 正常 / 异常（仅在检测后有意义）
const statusFilter = ref<"all" |"ok" |"error">("all");

// 按 group_title 分组展示（保留服务端返回顺序），并应用状态筛选
const grouped = computed(() => {
 const map: Record<string, LinkRow[]> = {};
 const order: string[] = [];
 for (const l of links.value) {
 if (statusFilter.value !=="all" && (checkStatus.value[l.id]?.ok ?"ok" : checkStatus.value[l.id] ?"error" : undefined) !== statusFilter.value) continue;
 const g = l.group_title ||"未分组";
 if (!map[g]) {
 map[g] = [];
 order.push(g);
 }
 map[g].push(l);
 }
 return order.map((title) => ({ title, items: map[title] }));
});

// GitHub 友链导入（Spec 54）：弹窗 + 分组输入 + 结果 Toast
const showImportModal = ref(false);
const importGroup = ref("GitHub 友链");
const importing = ref(false);
const importError = ref("");

function openImport() {
 importGroup.value ="GitHub 友链";
 importError.value ="";
 showImportModal.value = true;
}

async function handleImport() {
 importing.value = true;
 importError.value ="";
 try {
 const { data } = await linkApi.importFromGitHub({ group: importGroup.value.trim() });
 const failed = data?.failed || [];
 const msg = `导入 ${data.imported ?? 0} 条 · 跳过 ${data.skipped ?? 0} 条`;
 if (failed.length > 0) {
 Toast.error("部分导入失败", { description: `${msg}，失败 ${failed.length} 条（${failed.slice(0, 3).join("；")}${failed.length > 3 ?"…" :""}）` });
 } else {
 Toast.success("导入完成", { description: msg });
 }
 showImportModal.value = false;
 queryClient.invalidateQueries({ queryKey: ["friends"] });
 } catch (e: any) {
 importError.value = e?.response?.data?.error ||"导入失败，请稍后重试";
 } finally {
 importing.value = false;
 }
}

function openCreate() {
 editing.value = { group_title:"", name:"", url:"", avatar:"", description:"", rss:"", label:"", color:"", sort: 0 };
 error.value ="";
 showModal.value = true;
}
function openEdit(l: LinkRow) {
 menuOpenId.value = null;
 editing.value = { ...l };
 error.value ="";
 showModal.value = true;
}

async function handleSave() {
 saving.value = true;
 error.value ="";
 try {
 const payload = { ...editing.value };
 if (editing.value.id) {
 await linkApi.update(editing.value.id, payload);
 } else {
 await linkApi.create(payload);
 }
 showModal.value = false;
 queryClient.invalidateQueries({ queryKey: ["friends"] });
 Toast.success("保存成功");
 } catch (e: any) {
 error.value = e?.response?.data?.error ||"保存失败";
 } finally {
 saving.value = false;
 }
}

function handleDelete(l: LinkRow) {
 menuOpenId.value = null;
 Dialog.warning({
 title: `确定删除友链「${l.name}」？`,
 confirmType:"danger",
 onConfirm: async () => {
 await linkApi.delete(l.id);
 queryClient.invalidateQueries({ queryKey: ["friends"] });
 Toast.success("删除成功");
 },
 });
}

const inputCls = "apple-input";

function closeMenus() {
 menuOpenId.value = null;
}
</script>

<template>
 <div @click="closeMenus">
 <VPageHeader title="链接">
 <template #icon>
 <Icon icon="ri:links-line" class="mr-2 self-center text-xl" />
 </template>
 <template #actions>
 <router-link
 to="/subscriptions"
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent"
 >
 <Icon icon="ri:rss-line" />链接订阅
 </router-link>
 </template>
 </VPageHeader>

 <div class="m-0 space-y-4 md:m-4">
 <VLoading v-if="isLoading" />

 <VEmpty v-else-if="links.length === 0" message="点击右上角新增第一条友链" title="当前没有友链">
 <template #actions>
 <VButton type="primary" @click="openCreate">
 <template #icon>
 <Icon icon="ri:add-circle-line" class="h-full w-full" />
 </template>
 新增友链
 </VButton>
 </template>
 </VEmpty>

 <template v-else>
 <!-- 工具条（对齐 Halo 链接页） -->
 <div class="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-card px-3 py-2.5">
 <button class="rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent" @click="openCreate">新建分组</button>
 <button class="rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent" @click="openCreate">新建链接</button>
 <button
 :disabled="checking"
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary disabled:opacity-50 hover:bg-accent"
 @click="runCheck"
 >
 <Icon :icon="checking ? 'ri:loader-4-line' : 'ri:pulse-line'" :class="checking && 'animate-spin'" />检测全部
 </button>
 <button
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent"
 @click="openImport"
 >
 <Icon icon="ri:github-fill" />从 GitHub 导入
 </button>
 <span class="flex-1"></span>
 <span class="text-sm text-text-quaternary">状态：</span>
 <select v-model="statusFilter" class="rounded-lg border border-border bg-card px-2 py-1.5 text-sm text-text-tertiary outline-none">
 <option value="all">全部</option>
 <option value="ok">正常</option>
 <option value="error">异常</option>
 </select>
 </div>

 <!-- 分组卡片网格 -->
 <div v-for="g in grouped" :key="g.title" class="rounded-xl border border-border bg-card p-4">
 <div class="mb-3 flex items-center justify-between">
 <span class="text-sm font-semibold text-text-secondary">{{ g.title }} <span class="text-text-quaternary">({{ g.items.length }})</span></span>
 <button class="rounded-lg bg-secondary px-3 py-1 text-xs text-white hover:opacity-90" @click="openCreate">新建</button>
 </div>
 <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
 <div
 v-for="l in g.items"
 :key="l.id"
 class="group relative rounded-lg border border-border p-3 transition-shadow hover:shadow-sm"
 >
 <div class="flex items-center gap-2.5">
 <img v-if="l.avatar" :src="l.avatar" class="h-9 w-9 shrink-0 rounded-full bg-secondary object-cover" alt="" />
 <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-secondary text-text-quaternary"><Icon icon="ri:link" /></span>
 <div class="min-w-0 flex-1">
 <div class="flex items-center gap-1.5">
 <span class="truncate text-sm font-medium text-foreground">{{ l.name }}</span>
 <span v-if="l.label" class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium leading-none text-white" :style="{ backgroundColor: l.color ? '#' + l.color : '#9ca3af' }">{{ l.label }}</span>
 </div>
 <div class="truncate text-xs text-text-quaternary">{{ l.url }}</div>
 </div>
 <!-- 悬停操作 -->
 <div class="relative shrink-0 opacity-0 transition-opacity group-hover:opacity-100">
 <div class="cursor-pointer rounded p-1 hover:bg-secondary hover:bg-accent" @click.stop="menuOpenId = menuOpenId === l.id ? null : l.id">
 <Icon icon="ri:more-fill" class="text-text-quaternary" />
 </div>
 <Transition name="fade">
 <div v-if="menuOpenId === l.id" class="absolute right-0 top-full z-30 mt-1 w-28 rounded-lg border border-border bg-card p-2 shadow-lg" @click.stop>
 <VButton block type="secondary" size="sm" @click="openEdit(l)">编辑</VButton>
 <div class="mt-2"><VButton block type="danger" size="sm" @click="handleDelete(l)">删除</VButton></div>
 </div>
 </Transition>
 </div>
 </div>
 <!-- 底部徐标行：状态（药丸）/ RSS / 访问 -->
 <div class="mt-2.5 flex items-center gap-2 text-xs">
 <span v-if="checkStatus[l.id]?.ok" class="inline-flex items-center gap-1 rounded-full bg-state-success/10 px-2 py-0.5 text-state-success bg-state-success/100/15 text-state-success"><Icon icon="ri:checkbox-circle-fill" />正常</span>
 <span v-else-if="checkStatus[l.id]" :title="errorReason(l.id)" class="inline-flex items-center gap-1 rounded-full bg-state-error/10 px-2 py-0.5 text-state-error bg-state-error/100/15 text-state-error"><Icon icon="ri:error-warning-fill" />异常·{{ errorReason(l.id) }}</span>
 <span v-else class="inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-0.5 text-text-quaternary"><Icon icon="ri:pulse-line" />未检测</span>
 <span v-if="l.rss" class="inline-flex items-center gap-1 text-text-quaternary"><Icon icon="ri:rss-line" />RSS</span>
 <a :href="l.url" target="_blank" class="inline-flex items-center gap-1 text-text-quaternary hover:text-primary"><Icon icon="ri:external-link-line" />访问</a>
 </div>
 </div>
 </div>
 </div>
 </template>
 </div>

 <!-- GitHub 导入弹窗（Spec 54） -->
 <VModal v-model:visible="showImportModal" title="从 GitHub 导入友链" :width="480">
 <p class="mb-3 rounded-base bg-secondary px-3 py-2 text-sm text-text-tertiary">
 将从 <code class="rounded bg-secondary px-1 py-0.5 text-xs">sakurasep/Friends</code> 仓库拉取全部 open issues，解析其中的友链信息并批量写入，已存在的链接自动跳过。
 </p>
 <p v-if="importError" class="mb-3 rounded-base bg-state-error/10 px-3 py-2 text-sm text-state-error bg-state-error/10 text-state-error">
 {{ importError }}
 </p>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">导入分组</label>
 <input v-model="importGroup" :class="inputCls" placeholder="GitHub 友链" />
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton @click="showImportModal = false">取消</VButton>
 <VButton type="secondary" :loading="importing" @click="handleImport">开始导入</VButton>
 </div>
 </template>
 </VModal>

 <!-- Edit/Create modal -->
 <VModal v-model:visible="showModal" :title="editing.id ? '编辑友链' : '新增友链'" :width="560">
 <p v-if="error" class="mb-3 rounded-base bg-state-error/10 px-3 py-2 text-sm text-state-error bg-state-error/10 text-state-error">
 {{ error }}
 </p>
 <div class="space-y-3">
 <div class="grid grid-cols-2 gap-3">
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">分组</label>
 <input v-model="editing.group_title" :class="inputCls" placeholder="我的朋友" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">名称 *</label>
 <input v-model="editing.name" :class="inputCls" />
 </div>
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">链接 *</label>
 <input v-model="editing.url" :class="inputCls" placeholder="https://" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">头像 URL</label>
 <input v-model="editing.avatar" :class="inputCls" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">描述</label>
 <input v-model="editing.description" :class="inputCls" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">RSS/Atom（动态聚合用）</label>
 <input v-model="editing.rss" :class="inputCls" />
 </div>
 <div class="grid grid-cols-3 gap-3">
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">标签</label>
 <input v-model="editing.label" :class="inputCls" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">配色（不带 #）</label>
 <input v-model="editing.color" :class="inputCls" placeholder="1BCDFC" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">排序（大在前）</label>
 <input v-model.number="editing.sort" type="number" :class="inputCls" />
 </div>
 </div>
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton @click="showModal = false">取消</VButton>
 <VButton type="secondary" :loading="saving" @click="handleSave">保存</VButton>
 </div>
 </template>
 </VModal>
 </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
 transition: opacity var(--duration-fast) var(--ease-standard);
}
.fade-enter-from,
.fade-leave-to {
 opacity: 0;
}
</style>
