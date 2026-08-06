<script setup lang="ts">
import { ref, computed, watch } from"vue";
import { Icon } from"@iconify/vue";
import { revisionApi } from"@/api";
import { VButton, Dialog, Toast } from"@/components/ui";
import { myersDiff, type DiffHunk } from"./diff";

// 文章版本历史面板（Spec 22 + Spec 46）：
// 列表 → 选中版本（左）对比 当前正文/任一其他版本（右）→ 行级 diff（统一/并排、未变化折叠）→ 一键回滚到左侧版本。
const props = defineProps<{
 open: boolean;
 slug: string;
 currentContent: string;
 currentWordCount: number;
}>();
const emit = defineEmits<{
 (e:"close"): void;
 (e:"restored"): void;
}>();

interface RevisionMeta {
 id: number;
 title: string;
 word_count: number;
 created_at: string;
}

const metas = ref<RevisionMeta[]>([]);
const loadingList = ref(false);
const selected = ref<RevisionMeta | null>(null);
const loadingDiff = ref(false);
const restoring = ref(false);
const error = ref("");

// Spec 46：对比右侧目标（缺省当前正文，可选任一其他版本）；统一/并排视图；折叠展开态
const compareTarget = ref<"current" | number>("current");
const viewMode = ref<"unified" |"split">("unified");
const expandedFolds = ref<Set<number>>(new Set());
const hunks = ref<DiffHunk[]>([]);
// 版本内容缓存：切换对比目标时避免重复拉取
const contentCache = new Map<number, string>();

const FOLD_THRESHOLD = 10; // 未变化行数超过该值折叠
const FOLD_CONTEXT = 3; // 折叠时首尾保留的上下文行数

watch(
 () => props.open,
 (v) => {
 if (v) {
 selected.value = null;
 hunks.value = [];
 error.value ="";
 compareTarget.value ="current";
 viewMode.value ="unified";
 expandedFolds.value = new Set();
 contentCache.clear();
 loadList();
 }
 }
);

watch(compareTarget, () => {
 if (selected.value) rebuildDiff();
});

async function loadList() {
 if (!props.slug) return;
 loadingList.value = true;
 try {
 const { data } = await revisionApi.list(props.slug);
 metas.value = data.items || [];
 } catch (e: any) {
 error.value = e.response?.data?.error ||"加载版本列表失败";
 } finally {
 loadingList.value = false;
 }
}

async function fetchContent(id: number): Promise<string> {
 if (contentCache.has(id)) return contentCache.get(id)!;
 const { data } = await revisionApi.get(id);
 const content = data.content_md ||"";
 contentCache.set(id, content);
 return content;
}

async function selectRevision(m: RevisionMeta) {
 selected.value = m;
 // 左侧换版本后若右侧恰为同一版本，重置回当前正文
 if (compareTarget.value === m.id) compareTarget.value ="current";
 await rebuildDiff();
}

async function rebuildDiff() {
 if (!selected.value) return;
 loadingDiff.value = true;
 error.value ="";
 expandedFolds.value = new Set();
 try {
 const left = await fetchContent(selected.value.id);
 const right =
 compareTarget.value ==="current"
 ? props.currentContent ||""
 : await fetchContent(compareTarget.value);
 // 左（旧）→ 右（新）：新增绿、删除红
 hunks.value = myersDiff(left, right).hunks;
 } catch (e: any) {
 error.value = e.response?.data?.error ||"加载版本内容失败";
 } finally {
 loadingDiff.value = false;
 }
}

async function handleRestore() {
 if (!selected.value) return;
 const target = selected.value;
 Dialog.warning({
 title:"确认回滚到该版本？",
 description:"当前内容会先存为新快照，可随时再回滚。",
 confirmType:"danger",
 onConfirm: async () => {
 restoring.value = true;
 error.value ="";
 try {
 await revisionApi.restore(target.id);
 Toast.success("已回滚");
 emit("restored");
 emit("close");
 } catch (e: any) {
 error.value = e.response?.data?.error ||"回滚失败";
 } finally {
 restoring.value = false;
 }
 },
 });
}

function fmtTime(s: string): string {
 const d = new Date(s);
 if (isNaN(d.getTime())) return s;
 return d.toLocaleString("zh-CN", { hour12: false });
}

function wordDelta(m: RevisionMeta): string {
 const d = props.currentWordCount - m.word_count;
 if (d > 0) return `+${d}`;
 return `${d}`;
}

function toggleFold(key: number) {
 const next = new Set(expandedFolds.value);
 if (next.has(key)) next.delete(key);
 else next.add(key);
 expandedFolds.value = next;
}

// --- 统一视图行模型（含行号与折叠占位） ---
interface UnifiedRow {
 kind:"line" |"fold";
 type?:"add" |"del" |"ctx";
 text?: string;
 leftNo?: number | null;
 rightNo?: number | null;
 foldKey?: number;
 foldCount?: number;
}

const unifiedRows = computed<UnifiedRow[]>(() => {
 const rows: UnifiedRow[] = [];
 let leftNo = 1;
 let rightNo = 1;
 hunks.value.forEach((h, hi) => {
 if (h.type ==="same") {
 const foldable = h.lines.length > FOLD_THRESHOLD && !expandedFolds.value.has(hi);
 h.lines.forEach((text, li) => {
 const inFoldMiddle =
 foldable && li >= FOLD_CONTEXT && li < h.lines.length - FOLD_CONTEXT;
 if (inFoldMiddle) {
 // 折叠区中间只放一次占位行
 if (li === FOLD_CONTEXT) {
 rows.push({
 kind:"fold",
 foldKey: hi,
 foldCount: h.lines.length - FOLD_CONTEXT * 2,
 });
 }
 } else {
 rows.push({ kind:"line", type:"ctx", text, leftNo, rightNo });
 }
 leftNo++;
 rightNo++;
 });
 } else if (h.type ==="delete") {
 h.lines.forEach((text) => {
 rows.push({ kind:"line", type:"del", text, leftNo, rightNo: null });
 leftNo++;
 });
 } else {
 h.lines.forEach((text) => {
 rows.push({ kind:"line", type:"add", text, leftNo: null, rightNo });
 rightNo++;
 });
 }
 });
 return rows;
});

// --- 并排视图行模型：delete/add 相邻 hunk 逐行配对 ---
interface SplitCell {
 type:"add" |"del" |"ctx";
 text: string;
 no: number;
}
interface SplitRow {
 kind:"line" |"fold";
 left?: SplitCell | null;
 right?: SplitCell | null;
 foldKey?: number;
 foldCount?: number;
}

const splitRows = computed<SplitRow[]>(() => {
 const rows: SplitRow[] = [];
 let leftNo = 1;
 let rightNo = 1;
 const hs = hunks.value;
 for (let hi = 0; hi < hs.length; hi++) {
 const h = hs[hi];
 if (h.type ==="same") {
 const foldable = h.lines.length > FOLD_THRESHOLD && !expandedFolds.value.has(hi);
 h.lines.forEach((text, li) => {
 const inFoldMiddle =
 foldable && li >= FOLD_CONTEXT && li < h.lines.length - FOLD_CONTEXT;
 if (inFoldMiddle) {
 if (li === FOLD_CONTEXT) {
 rows.push({
 kind:"fold",
 foldKey: hi,
 foldCount: h.lines.length - FOLD_CONTEXT * 2,
 });
 }
 } else {
 rows.push({
 kind:"line",
 left: { type:"ctx", text, no: leftNo },
 right: { type:"ctx", text, no: rightNo },
 });
 }
 leftNo++;
 rightNo++;
 });
 } else if (h.type ==="delete") {
 // delete 紧跟 add 时逐行配对为「修改」
 const next = hs[hi + 1];
 const addLines = next && next.type ==="add" ? next.lines : [];
 const max = Math.max(h.lines.length, addLines.length);
 for (let i = 0; i < max; i++) {
 const l = i < h.lines.length ? { type:"del" as const, text: h.lines[i], no: leftNo++ } : null;
 const r = i < addLines.length ? { type:"add" as const, text: addLines[i], no: rightNo++ } : null;
 rows.push({ kind:"line", left: l, right: r });
 }
 if (addLines.length > 0) hi++; // 消费掉配对的 add hunk
 } else {
 h.lines.forEach((text) => {
 rows.push({ kind:"line", left: null, right: { type:"add", text, no: rightNo++ } });
 });
 }
 }
 return rows;
});

const compareOptions = computed(() =>
 metas.value.filter((m) => m.id !== selected.value?.id)
);
</script>

<template>
 <div
 v-if="open"
 class="fixed inset-0 z-50 flex justify-end bg-black/40"
 @click.self="emit('close')"
 >
 <div
 class="w-full h-full flex flex-col bg-card shadow-xl"
 :class="viewMode === 'split' ? 'max-w-6xl' : 'max-w-3xl'"
 >
 <div class="flex items-center justify-between px-5 py-3 border-b border-border">
 <h3 class="text-sm font-medium text-foreground">版本历史</h3>
 <button @click="emit('close')" class="text-text-quaternary hover:text-foreground">
 <Icon icon="ri:close-line" class="text-xl" />
 </button>
 </div>

 <p v-if="error" class="mx-5 mt-3 text-sm text-state-error bg-state-error/10 px-3 py-2 rounded-lg">{{ error }}</p>

 <div class="flex-1 grid grid-cols-1 md:grid-cols-5 gap-0 overflow-hidden">
 <!-- 版本列表 -->
 <div class="md:col-span-2 overflow-y-auto border-r border-border" :class="viewMode === 'split' ? 'md:col-span-1' : ''">
 <p v-if="loadingList" class="p-4 text-sm text-text-tertiary">加载中...</p>
 <p v-else-if="metas.length === 0" class="p-4 text-sm text-text-tertiary">暂无历史版本</p>
 <button
 v-for="m in metas"
 :key="m.id"
 @click="selectRevision(m)"
 class="w-full text-left px-4 py-3 border-b border-border/50 hover:bg-accent"
 :class="{ 'bg-primary/10': selected?.id === m.id }"
 >
 <div class="flex items-center justify-between">
 <span class="text-sm font-medium text-foreground truncate">{{ m.title ||"（无标题）" }}</span>
 <span
 class="ml-2 shrink-0 text-xs tabular-nums"
 :class="wordDelta(m).startsWith('+') ? 'text-state-success' : 'text-text-quaternary'"
 >{{ wordDelta(m) }}</span>
 </div>
 <div class="mt-0.5 text-xs text-text-quaternary">{{ fmtTime(m.created_at) }}</div>
 </button>
 </div>

 <!-- diff 视图 -->
 <div class="overflow-y-auto" :class="viewMode === 'split' ? 'md:col-span-4' : 'md:col-span-3'">
 <div v-if="!selected" class="p-6 text-sm text-text-quaternary">选择左侧版本查看差异</div>
 <div v-else-if="loadingDiff" class="p-6 text-sm text-text-tertiary">加载差异中...</div>
 <div v-else>
 <div class="sticky top-0 z-10 flex flex-wrap items-center gap-2 px-4 py-2 bg-secondary border-b border-border">
 <span class="text-xs text-text-tertiary shrink-0">对比</span>
 <!-- 右侧目标选择（Spec 46：任选两版） -->
 <select
 v-model="compareTarget"
 class="apple-select text-xs py-1"
 >
 <option value="current">当前正文</option>
 <option v-for="m in compareOptions" :key="m.id" :value="m.id">
 {{ fmtTime(m.created_at) }}
 </option>
 </select>
 <span class="text-xs text-text-quaternary shrink-0">（<span class="text-state-success">新增</span> / <span class="text-state-error">删除</span>）</span>
 <div class="flex-1"></div>
 <!-- 统一/并排切换（移动端隐藏并排，退化统一视图） -->
 <div class="hidden md:flex rounded-md border border-border overflow-hidden">
 <button
 class="px-2 py-1 text-xs"
 :class="viewMode === 'unified' ? 'bg-primary/10 text-primary' : 'text-text-tertiary'"
 @click="viewMode = 'unified'"
 >统一</button>
 <button
 class="px-2 py-1 text-xs border-l border-border"
 :class="viewMode === 'split' ? 'bg-primary/10 text-primary' : 'text-text-tertiary'"
 @click="viewMode = 'split'"
 >并排</button>
 </div>
 <VButton type="danger" size="sm" :loading="restoring" @click="handleRestore">恢复到左侧版本</VButton>
 </div>

 <!-- 统一视图 -->
 <div v-if="viewMode === 'unified'" class="text-xs leading-5 font-mono">
 <template v-for="(row, idx) in unifiedRows" :key="idx">
 <button
 v-if="row.kind === 'fold'"
 class="block w-full px-4 py-1 text-center text-text-quaternary bg-secondary hover:bg-accent hover:text-text-tertiary"
 @click="toggleFold(row.foldKey!)"
 >⋯ 展开 {{ row.foldCount }} 行 ⋯</button>
 <div
 v-else
 class="flex px-2 whitespace-pre-wrap break-words"
 :class="{
 'bg-state-success/10 text-state-success': row.type === 'add',
 'bg-state-error/10 text-state-error': row.type === 'del',
 'text-text-tertiary': row.type === 'ctx'
 }"
 >
 <span class="w-10 shrink-0 text-right pr-2 text-text-quaternary select-none tabular-nums">{{ row.leftNo ?? '' }}</span>
 <span class="w-10 shrink-0 text-right pr-2 text-text-quaternary select-none tabular-nums">{{ row.rightNo ?? '' }}</span>
 <span class="flex-1">{{ row.type === 'add' ? '+ ' : row.type === 'del' ? '- ' : ' ' }}{{ row.text }}</span>
 </div>
 </template>
 </div>

 <!-- 并排视图（仅 md+，移动端由切换按钮隐藏保证不可进入） -->
 <div v-else class="text-xs leading-5 font-mono">
 <template v-for="(row, idx) in splitRows" :key="idx">
 <button
 v-if="row.kind === 'fold'"
 class="block w-full px-4 py-1 text-center text-text-quaternary bg-secondary hover:bg-accent hover:text-text-tertiary"
 @click="toggleFold(row.foldKey!)"
 >⋯ 展开 {{ row.foldCount }} 行 ⋯</button>
 <div v-else class="grid grid-cols-2">
 <div
 class="flex px-2 whitespace-pre-wrap break-words border-r border-border"
 :class="row.left?.type === 'del' ? 'bg-state-error/10 text-state-error' : 'text-text-tertiary'"
 >
 <span class="w-10 shrink-0 text-right pr-2 text-text-quaternary select-none tabular-nums">{{ row.left?.no ?? '' }}</span>
 <span class="flex-1">{{ row.left ? (row.left.type === 'del' ? '- ' : ' ') + row.left.text : '' }}</span>
 </div>
 <div
 class="flex px-2 whitespace-pre-wrap break-words"
 :class="row.right?.type === 'add' ? 'bg-state-success/10 text-state-success' : 'text-text-tertiary'"
 >
 <span class="w-10 shrink-0 text-right pr-2 text-text-quaternary select-none tabular-nums">{{ row.right?.no ?? '' }}</span>
 <span class="flex-1">{{ row.right ? (row.right.type === 'add' ? '+ ' : ' ') + row.right.text : '' }}</span>
 </div>
 </div>
 </template>
 </div>
 </div>
 </div>
 </div>
 </div>
 </div>
</template>
