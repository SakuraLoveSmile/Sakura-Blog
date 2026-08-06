<script setup lang="ts">
import { ref, computed, onMounted } from"vue";
import { notebookApi } from"@/api";
import MarkdownEditor from"@/components/MarkdownEditor.vue";
import { Icon } from"@iconify/vue";
import { VPageHeader, VCard, VButton, Toast, Dialog, VModal } from"@/components/ui";

// ---------- 类型 ----------
interface Notebook {
 id: number;
 name: string;
 description: string;
 cover: string;
 order: number;
 created_at: string;
}

interface Note {
 id: number;
 notebook_id: number;
 title: string;
 content: string;
 content_html: string;
 tags: string;
 created_at: string;
 updated_at: string;
}

// ---------- 第一级：笔记本列表 ----------
const notebooks = ref<Notebook[]>([]);
const noteCounts = ref<Record<number, number>>({}); // notebookId -> 笔记数（后端列表不返回，逐个取 total）
const loading = ref(false);

async function loadNotebooks() {
 loading.value = true;
 try {
 const { data } = await notebookApi.listNotebooks();
 notebooks.value = data.items || [];
 // 并行取各笔记本笔记数（per_page=1 仅用 total）
 const entries = await Promise.all(
 notebooks.value.map(async (nb) => {
 try {
 const { data: d } = await notebookApi.listNotes(nb.id, { page: 1, per_page: 1 });
 return [nb.id, d.total || 0] as const;
 } catch {
 return [nb.id, 0] as const;
 }
 })
 );
 noteCounts.value = Object.fromEntries(entries);
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载笔记本失败");
 } finally {
 loading.value = false;
 }
}

// ---------- 笔记本新建/编辑弹窗 ----------
const nbModalOpen = ref(false);
const nbModalMode = ref<"create" |"edit">("create");
const nbEditingId = ref(0);
const nbForm = ref({ name:"", description:"", cover:"" });
const nbSaving = ref(false);

function openCreateNotebook() {
 nbModalMode.value ="create";
 nbEditingId.value = 0;
 nbForm.value = { name:"", description:"", cover:"" };
 nbModalOpen.value = true;
}

function openEditNotebook(nb: Notebook) {
 nbModalMode.value ="edit";
 nbEditingId.value = nb.id;
 nbForm.value = { name: nb.name, description: nb.description ||"", cover: nb.cover ||"" };
 nbModalOpen.value = true;
}

async function handleSaveNotebook() {
 const name = nbForm.value.name.trim();
 if (!name) {
 Toast.error("请输入笔记本名称");
 return;
 }
 nbSaving.value = true;
 try {
 const payload = {
 name,
 description: nbForm.value.description.trim(),
 cover: nbForm.value.cover.trim(),
 };
 if (nbModalMode.value ==="create") {
 await notebookApi.createNotebook(payload);
 Toast.success("笔记本已创建");
 } else {
 await notebookApi.updateNotebook(nbEditingId.value, payload);
 Toast.success("已保存");
 }
 nbModalOpen.value = false;
 await loadNotebooks();
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"保存失败");
 } finally {
 nbSaving.value = false;
 }
}

function handleDeleteNotebook(nb: Notebook) {
 Dialog.warning({
 title:"确定要删除该笔记本吗？",
 description: `「${nb.name}」及其全部笔记将一并级联删除，不可恢复。`,
 confirmType:"danger",
 onConfirm: async () => {
 try {
 await notebookApi.deleteNotebook(nb.id);
 Toast.success("已删除");
 await loadNotebooks();
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"删除失败");
 }
 },
 });
}

// ---------- 第二级：笔记管理 ----------
const activeNotebook = ref<Notebook | null>(null); // 非空 = 进入笔记管理

function enterNotebook(nb: Notebook) {
 activeNotebook.value = nb;
 resetNoteEditor();
 loadNotes(1);
}

function backToNotebooks() {
 activeNotebook.value = null;
 loadNotebooks(); // 刷新第一级笔记数
}

// --- 左侧笔记列表（分页） ---
const notes = ref<Note[]>([]);
const notesTotal = ref(0);
const notesPage = ref(1);
const perPage = 20;
const notesLoading = ref(false);

const totalPages = computed(() => Math.max(1, Math.ceil(notesTotal.value / perPage)));

async function loadNotes(page = 1) {
 if (!activeNotebook.value) return;
 notesLoading.value = true;
 try {
 const { data } = await notebookApi.listNotes(activeNotebook.value.id, {
 page,
 per_page: perPage,
 });
 notes.value = data.notes || [];
 notesTotal.value = data.total || 0;
 notesPage.value = data.page || page;
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载笔记失败");
 } finally {
 notesLoading.value = false;
 }
}

function noteTags(note: Note): string[] {
 return (note.tags ||"").split(",").map((s) => s.trim()).filter(Boolean);
}

function fmtTime(s: string): string {
 if (!s) return"";
 const d = new Date(s);
 if (isNaN(d.getTime())) return s;
 const pad = (n: number) => String(n).padStart(2,"0");
 return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// --- 右侧笔记编辑区 ---
const currentId = ref(0);
const noteForm = ref({ title:"", tags:"", content:"" });
const noteLoading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);

const hasCurrent = computed(() => currentId.value > 0);

// 常用标签（来自后端 note-tags 聚合，供输入提示）
const allTags = ref<string[]>([]);

async function loadTags() {
 try {
 const { data } = await notebookApi.getTags();
 allTags.value = data.tags || [];
 } catch {
 allTags.value = [];
 }
}

function resetNoteEditor() {
 currentId.value = 0;
 noteForm.value = { title:"", tags:"", content:"" };
}

async function selectNote(id: number) {
 noteLoading.value = true;
 try {
 const { data } = await notebookApi.getNote(id);
 currentId.value = data.id;
 noteForm.value = {
 title: data.title ||"",
 tags: data.tags ||"",
 content: data.content ||"",
 };
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载笔记失败");
 } finally {
 noteLoading.value = false;
 }
}

async function handleSave() {
 if (!hasCurrent.value) return;
 saving.value = true;
 try {
 await notebookApi.updateNote(currentId.value, {
 title: noteForm.value.title.trim(),
 tags: noteForm.value.tags.trim(),
 content: noteForm.value.content,
 });
 editorRef.value?.clearDraft();
 Toast.success("保存成功");
 await loadNotes(notesPage.value); // 刷新列表（标题/更新时间/标签）
 await loadTags(); // 新增标签后刷新提示
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"保存失败");
 } finally {
 saving.value = false;
 }
}

// --- 新建笔记弹窗 ---
const createNoteOpen = ref(false);
const createNoteTitle = ref("");
const createNoteSaving = ref(false);

function openCreateNote() {
 createNoteTitle.value ="";
 createNoteOpen.value = true;
}

async function handleCreateNote() {
 if (!activeNotebook.value) return;
 const title = createNoteTitle.value.trim();
 if (!title) {
 Toast.error("请输入笔记标题");
 return;
 }
 createNoteSaving.value = true;
 try {
 const { data } = await notebookApi.createNote(activeNotebook.value.id, {
 title,
 content:"",
 tags:"",
 });
 createNoteOpen.value = false;
 Toast.success("笔记已创建");
 await loadNotes(1);
 await selectNote(data.id);
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"创建失败");
 } finally {
 createNoteSaving.value = false;
 }
}

function handleDeleteNote() {
 Dialog.warning({
 title:"确定要删除该笔记吗？",
 description: `「${noteForm.value.title ||"未命名笔记"}」删除后不可恢复。`,
 confirmType:"danger",
 onConfirm: async () => {
 if (!hasCurrent.value) return;
 deleting.value = true;
 try {
 await notebookApi.deleteNote(currentId.value);
 Toast.success("已删除");
 resetNoteEditor();
 // 当前页删空则回退一页
 let page = notesPage.value;
 if (notes.value.length === 1 && page > 1) page -= 1;
 await loadNotes(page);
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"删除失败");
 } finally {
 deleting.value = false;
 }
 },
 });
}

// ---------- 样式 ----------
const inputCls = "apple-input";
const labelCls ="block text-xs text-text-tertiary mb-1";

onMounted(() => {
 loadNotebooks();
 loadTags();
});
</script>

<template>
 <div>
 <VPageHeader :title="activeNotebook ? activeNotebook.name : '笔记本管理'">
 <template #icon>
 <Icon icon="ri:book-open-line" class="mr-2 self-center text-xl" />
 </template>
 <template v-if="activeNotebook" #actions>
 <VButton type="secondary" size="sm" @click="backToNotebooks">
 <template #icon>
 <Icon icon="ri:arrow-left-line" class="h-full w-full" />
 </template>
 返回笔记本列表
 </VButton>
 </template>
 </VPageHeader>

 <div class="m-0 md:m-4">
 <!-- ============ 第一级：笔记本卡片列表 ============ -->
 <div v-if="!activeNotebook">
 <div class="mb-4 flex items-center justify-between">
 <div class="flex items-center gap-2 text-sm text-text-tertiary">
 <Icon icon="ri:folder-open-line" />
 <span>共 {{ notebooks.length }} 个笔记本</span>
 </div>
 <VButton type="secondary" size="sm" @click="openCreateNotebook">
 <template #icon>
 <Icon icon="ri:add-line" class="h-full w-full" />
 </template>
 新建笔记本
 </VButton>
 </div>

 <div v-if="loading" class="py-12 text-center text-sm text-text-quaternary">加载中...</div>

 <div
 v-else-if="!notebooks.length"
 class="rounded border border-border bg-card py-16 text-center"
 >
 <Icon icon="ri:book-open-line" class="mx-auto text-3xl text-text-quaternary" />
 <p class="mt-3 text-sm text-text-tertiary">暂无笔记本</p>
 <p class="mt-1 text-xs text-text-quaternary">点击「新建笔记本」创建，前台 /notebooks 即时可见。</p>
 </div>

 <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
 <div
 v-for="nb in notebooks"
 :key="nb.id"
 class="group flex flex-col overflow-hidden rounded-lg border border-border bg-card transition-shadow hover:shadow-md"
 >
 <!-- 封面 -->
 <button type="button" class="relative h-28 w-full overflow-hidden bg-secondary" @click="enterNotebook(nb)">
 <img
 v-if="nb.cover"
 :src="nb.cover"
 :alt="nb.name"
 class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
 />
 <div v-else class="flex h-full w-full items-center justify-center">
 <Icon icon="ri:book-open-line" class="text-4xl text-text-quaternary" />
 </div>
 <span
 class="absolute right-2 top-2 rounded bg-black/50 px-1.5 py-0.5 text-[10px] text-white"
 >{{ noteCounts[nb.id] || 0 }} 篇笔记</span>
 </button>

 <!-- 信息 -->
 <div class="flex flex-1 flex-col p-4">
 <button
 type="button"
 class="text-left text-base font-medium text-foreground hover:text-primary hover:text-primary"
 @click="enterNotebook(nb)"
 >
 {{ nb.name }}
 </button>
 <p class="mt-1 line-clamp-2 min-h-[2.5rem] flex-1 text-xs leading-5 text-text-tertiary">
 {{ nb.description ||"暂无描述" }}
 </p>
 <!-- 操作 -->
 <div class="mt-3 flex items-center gap-2 border-t border-border pt-3">
 <VButton type="secondary" size="sm" class="flex-1" @click="enterNotebook(nb)">
 <template #icon>
 <Icon icon="ri:file-list-3-line" class="h-full w-full" />
 </template>
 管理笔记
 </VButton>
 <button
 type="button"
 title="编辑"
 class="rounded p-1.5 text-text-quaternary transition-colors hover:bg-secondary hover:text-text-tertiary hover:bg-accent hover:text-foreground"
 @click="openEditNotebook(nb)"
 >
 <Icon icon="ri:edit-line" />
 </button>
 <button
 type="button"
 title="删除"
 class="rounded p-1.5 text-text-quaternary transition-colors hover:bg-red-50 hover:text-red-500 hover:bg-red-900/20 hover:text-red-400"
 @click="handleDeleteNotebook(nb)"
 >
 <Icon icon="ri:delete-bin-line" />
 </button>
 </div>
 </div>
 </div>
 </div>
 </div>

 <!-- ============ 第二级：笔记管理（左右分栏） ============ -->
 <div v-else class="flex flex-col gap-4 lg:flex-row">
 <!-- 左侧：笔记列表 -->
 <VCard title="笔记列表" :bodyClass="['!p-0']" class="lg:w-[280px] shrink-0">
 <template #actions>
 <span class="text-xs text-text-quaternary">{{ notesTotal }} 篇</span>
 </template>
 <div class="max-h-[70vh] overflow-y-auto p-2">
 <div v-if="notesLoading" class="py-8 text-center text-xs text-text-quaternary">加载中...</div>
 <div v-else-if="!notes.length" class="py-8 text-center text-xs text-text-quaternary">
 暂无笔记<br />点击「新建笔记」创建
 </div>
 <div v-else class="space-y-0.5">
 <button
 v-for="note in notes"
 :key="note.id"
 type="button"
 @click="selectNote(note.id)"
 :class="[
 'block w-full rounded px-2 py-2 text-left transition-colors',
 currentId === note.id
 ? 'bg-primary/15 text-secondary bg-primary/20 text-primary'
 : 'text-text-secondary hover:bg-secondary hover:bg-accent',
 ]"
 >
 <span class="block truncate text-sm font-medium">{{ note.title ||"未命名笔记" }}</span>
 <span class="mt-0.5 block text-[10px] text-text-quaternary">{{ fmtTime(note.updated_at) }}</span>
 <span v-if="noteTags(note).length" class="mt-1 flex flex-wrap gap-1">
 <span
 v-for="t in noteTags(note)"
 :key="t"
 class="rounded bg-secondary px-1 py-px text-[10px] text-text-tertiary"
 >{{ t }}</span>
 </span>
 </button>
 </div>
 </div>
 <!-- 分页 -->
 <div v-if="totalPages > 1" class="flex items-center justify-between border-t border-border px-3 py-2">
 <button
 type="button"
 :disabled="notesPage <= 1"
 class="text-xs text-text-quaternary transition-colors hover:text-text-tertiary disabled:opacity-30 hover:text-foreground"
 @click="loadNotes(notesPage - 1)"
 >
 <Icon icon="ri:arrow-left-s-line" class="text-base" />
 </button>
 <span class="text-xs text-text-quaternary">{{ notesPage }} / {{ totalPages }}</span>
 <button
 type="button"
 :disabled="notesPage >= totalPages"
 class="text-xs text-text-quaternary transition-colors hover:text-text-tertiary disabled:opacity-30 hover:text-foreground"
 @click="loadNotes(notesPage + 1)"
 >
 <Icon icon="ri:arrow-right-s-line" class="text-base" />
 </button>
 </div>
 </VCard>

 <!-- 右侧：笔记编辑区 -->
 <VCard class="min-w-0 flex-1">
 <template #actions>
 <VButton type="secondary" size="sm" @click="openCreateNote">
 <template #icon>
 <Icon icon="ri:add-line" class="h-full w-full" />
 </template>
 新建笔记
 </VButton>
 </template>

 <div v-if="noteLoading" class="py-16 text-center text-sm text-text-quaternary">加载中...</div>

 <div v-else-if="!hasCurrent" class="py-16 text-center">
 <Icon icon="ri:file-edit-line" class="mx-auto text-3xl text-text-quaternary" />
 <p class="mt-3 text-sm text-text-tertiary">选择左侧笔记开始编辑，或点击「新建笔记」</p>
 </div>

 <div v-else class="space-y-4">
 <!-- 元信息 -->
 <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
 <div>
 <label :class="labelCls">标题</label>
 <input v-model="noteForm.title" :class="inputCls" placeholder="笔记标题" />
 </div>
 <div>
 <label :class="labelCls">标签（逗号分隔）</label>
 <input
 v-model="noteForm.tags"
 :class="inputCls"
 list="notebook-tag-options"
 placeholder="Go, 前端"
 />
 <datalist id="notebook-tag-options">
 <option v-for="t in allTags" :key="t" :value="t" />
 </datalist>
 </div>
 </div>

 <!-- Markdown 编辑器 -->
 <MarkdownEditor
 ref="editorRef"
 v-model="noteForm.content"
 :storage-key="'note:' + currentId"
 @save="handleSave"
 />

 <!-- 操作按钮 -->
 <div class="flex items-center gap-2">
 <VButton type="secondary" :loading="saving" @click="handleSave">
 <template #icon>
 <Icon icon="ri:save-line" class="h-full w-full" />
 </template>
 保存
 </VButton>
 <VButton type="danger" :loading="deleting" @click="handleDeleteNote">
 <template #icon>
 <Icon icon="ri:delete-bin-line" class="h-full w-full" />
 </template>
 删除
 </VButton>
 <span class="ml-auto text-xs text-text-quaternary">Ctrl/⌘ + S 快速保存</span>
 </div>
 </div>
 </VCard>
 </div>
 </div>

 <!-- 新建/编辑笔记本弹窗 -->
 <VModal
 :visible="nbModalOpen"
 :title="nbModalMode === 'create' ? '新建笔记本' : '编辑笔记本'"
 :width="460"
 @update:visible="nbModalOpen = $event"
 @close="nbModalOpen = false"
 >
 <div class="space-y-4">
 <div>
 <label :class="labelCls">名称（前台 URL 别名，需唯一）</label>
 <input v-model="nbForm.name" :class="inputCls" placeholder="如 go / 读书笔记" autofocus />
 </div>
 <div>
 <label :class="labelCls">描述</label>
 <input v-model="nbForm.description" :class="inputCls" placeholder="一句话介绍" />
 </div>
 <div>
 <label :class="labelCls">封面 URL</label>
 <input v-model="nbForm.cover" :class="inputCls" placeholder="https://... 或 /uploads/..." />
 </div>
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton @click="nbModalOpen = false">取消</VButton>
 <VButton type="secondary" :loading="nbSaving" @click="handleSaveNotebook">保存</VButton>
 </div>
 </template>
 </VModal>

 <!-- 新建笔记弹窗 -->
 <VModal
 :visible="createNoteOpen"
 title="新建笔记"
 :width="440"
 @update:visible="createNoteOpen = $event"
 @close="createNoteOpen = false"
 >
 <div>
 <label :class="labelCls">标题</label>
 <input v-model="createNoteTitle" :class="inputCls" placeholder="笔记标题" autofocus @keyup.enter="handleCreateNote" />
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton @click="createNoteOpen = false">取消</VButton>
 <VButton type="secondary" :loading="createNoteSaving" @click="handleCreateNote">创建</VButton>
 </div>
 </template>
 </VModal>
 </div>
</template>
