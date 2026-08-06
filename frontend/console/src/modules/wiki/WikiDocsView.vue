<script setup lang="ts">
import { ref, computed, onMounted, watch } from"vue";
import { settingsApi, wikiApi } from"@/api";
import MarkdownEditor from"@/components/MarkdownEditor.vue";
import { Icon } from"@iconify/vue";
import { VPageHeader, VCard, VButton, Toast, Dialog, VModal } from"@/components/ui";

// ---------- 类型 ----------
interface WikiProject {
 id: string;
 title: string;
 description?: string;
 icon?: string;
}

interface WikiDoc {
 id: number;
 project_id: string;
 slug: string;
 title: string;
 order: number;
 parent: number;
 content_md: string;
 content_html: string;
 status:"published" |"draft";
 created_at: string;
 updated_at: string;
}

interface TreeNode {
 doc: WikiDoc;
 children: TreeNode[];
}

interface FlatNode {
 doc: WikiDoc;
 depth: number;
 hasChildren: boolean;
}

// ---------- 项目（settings wiki 段） ----------
const projects = ref<WikiProject[]>([]);
const activeProject = ref("");
const settingsLoading = ref(true);

// ---------- 文档列表 / 树 ----------
const docs = ref<WikiDoc[]>([]);
const flatTree = ref<FlatNode[]>([]);
const docsLoading = ref(false);

// 树拍平（preorder + 层级缩进），供左侧列表渲染
// 后端 TreeNode.Children 带 omitempty，叶子节点不输出 children 字段（为 undefined），此处必须容错。
function flatten(nodes: TreeNode[], depth = 0): FlatNode[] {
 const out: FlatNode[] = [];
 for (const n of nodes) {
 const children = n.children || [];
 out.push({ doc: n.doc, depth, hasChildren: children.length > 0 });
 if (children.length) out.push(...flatten(children, depth + 1));
 }
 return out;
}

// ---------- 当前编辑文档 ----------
const form = ref<{
 title: string;
 slug: string;
 order: number;
 parent: number;
 status:"published" |"draft";
 content_md: string;
}>({ title:"", slug:"", order: 0, parent: 0, status:"draft", content_md:"" });
const currentId = ref(0);
const originalSlug = ref(""); // 加载时的 slug；保存改名后用于定位新文档
const selectedSlug = ref("");
const docLoading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);

const hasCurrent = computed(() => currentId.value > 0 || !!originalSlug.value);

// 递归收集某文档的全部后代 ID（编辑时不可选，否则会产生循环引用）
function collectDescendantIds(pid: number, acc = new Set<number>()): Set<number> {
 for (const d of docs.value) {
 if (d.parent === pid && !acc.has(d.id)) {
 acc.add(d.id);
 collectDescendantIds(d.id, acc);
 }
 }
 return acc;
}

// 父文档可选项：排除当前文档自身及其所有后代
const parentOptions = computed(() => {
 const excluded = new Set<number>();
 if (currentId.value > 0) {
 excluded.add(currentId.value);
 collectDescendantIds(currentId.value, excluded);
 }
 return docs.value.filter((d) => !excluded.has(d.id));
});

function resetEditor() {
 currentId.value = 0;
 originalSlug.value ="";
 selectedSlug.value ="";
 form.value = { title:"", slug:"", order: 0, parent: 0, status:"draft", content_md:"" };
}

// ---------- 新建文档弹窗 ----------
const createOpen = ref(false);
const createTitle = ref("");
const createSlug = ref("");
const slugManuallyEdited = ref(false);
const creating = ref(false);

// 与 PostEditor 对齐的 slug 自动推导（保留 Unicode 字母/数字）
function autoSlug(title: string): string {
 let s = (title ||"").trim().toLowerCase();
 s = s.replace(/\s+/g,"-");
 s = s.replace(/[^\p{L}\p{N}-]/gu,"");
 s = s.replace(/-{2,}/g,"-");
 s = s.replace(/^-+|-+$/g,"");
 return s;
}

watch(createTitle, (t) => {
 if (!slugManuallyEdited.value) createSlug.value = autoSlug(t);
});

function openCreate() {
 createTitle.value ="";
 createSlug.value ="";
 slugManuallyEdited.value = false;
 createOpen.value = true;
}

async function handleCreate() {
 const slug = createSlug.value.trim();
 if (!slug) {
 Toast.error("请输入 slug");
 return;
 }
 if (!activeProject.value) return;
 creating.value = true;
 try {
 await wikiApi.createDoc(activeProject.value, {
 slug,
 title: createTitle.value.trim() || slug,
 order: 0,
 parent: 0,
 content_md:"",
 status:"draft",
 });
 createOpen.value = false;
 Toast.success("文档已创建");
 await loadDocs();
 await selectDoc(slug);
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"创建失败");
 } finally {
 creating.value = false;
 }
}

// ---------- 数据加载 ----------
async function loadProjects() {
 settingsLoading.value = true;
 try {
 const { data } = await settingsApi.all();
 // 项目列表按 sort 降序排列（sort 相同或无值时按 id 字母序），与前台 /wiki 页顺序一致
 projects.value = [...(data.wiki || [])].sort((a: any, b: any) => {
 const sa = Number(a?.sort) || 0;
 const sb = Number(b?.sort) || 0;
 if (sa !== sb) return sb - sa;
 return String(a?.id ??"").localeCompare(String(b?.id ??""));
 });
 if (projects.value.length) {
 activeProject.value = projects.value[0].id;
 }
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载文库项目失败");
 } finally {
 settingsLoading.value = false;
 }
}

async function loadDocs() {
 if (!activeProject.value) return;
 docsLoading.value = true;
 try {
 const { data } = await wikiApi.listDocs(activeProject.value);
 docs.value = data.docs || [];
 flatTree.value = flatten(data.tree || []);
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载文档失败");
 } finally {
 docsLoading.value = false;
 }
}

// 切换项目：清空编辑区后重载文档
watch(activeProject, () => {
 resetEditor();
 loadDocs();
});

async function selectDoc(slug: string) {
 if (!activeProject.value) return;
 docLoading.value = true;
 try {
 const { data } = await wikiApi.getDoc(activeProject.value, slug);
 currentId.value = data.id;
 originalSlug.value = data.slug;
 selectedSlug.value = data.slug;
 form.value = {
 title: data.title,
 slug: data.slug,
 order: data.order,
 parent: data.parent,
 status: data.status ==="published" ?"published" :"draft",
 content_md: data.content_md,
 };
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"加载文档失败");
 } finally {
 docLoading.value = false;
 }
}

// ---------- 保存 / 删除 ----------
async function handleSave() {
 if (!activeProject.value || !hasCurrent.value) return;
 saving.value = true;
 try {
 const payload = {
 slug: form.value.slug.trim(),
 title: form.value.title.trim(),
 order: Number(form.value.order) || 0,
 parent: Number(form.value.parent) || 0,
 content_md: form.value.content_md,
 status: form.value.status,
 };
 await wikiApi.updateDoc(activeProject.value, originalSlug.value, payload);
 editorRef.value?.clearDraft();
 Toast.success("保存成功");
 // slug 改名后重新加载列表并选中新 slug
 const newSlug = payload.slug || originalSlug.value;
 await loadDocs();
 if (newSlug !== originalSlug.value) {
 await selectDoc(newSlug);
 } else {
 selectedSlug.value = newSlug;
 }
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"保存失败");
 } finally {
 saving.value = false;
 }
}

function handleDelete() {
 Dialog.warning({
 title:"确定要删除该文档吗？",
 description: `「${form.value.title || originalSlug.value}」删除后不可恢复。`,
 confirmType:"danger",
 onConfirm: async () => {
 if (!activeProject.value) return;
 deleting.value = true;
 try {
 await wikiApi.deleteDoc(activeProject.value, originalSlug.value);
 Toast.success("已删除");
 resetEditor();
 await loadDocs();
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

onMounted(loadProjects);
</script>

<template>
 <div>
 <VPageHeader title="文库管理">
 <template #icon>
 <Icon icon="ri:book-2-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="m-0 md:m-4">
 <div v-if="settingsLoading" class="py-12 text-center text-sm text-text-quaternary">加载中...</div>

 <div v-else-if="!projects.length" class="rounded border border-border bg-card py-16 text-center">
 <Icon icon="ri:book-2-line" class="mx-auto text-3xl text-text-quaternary" />
 <p class="mt-3 text-sm text-text-tertiary">暂无文库项目</p>
 <p class="mt-1 text-xs text-text-quaternary">请先到「设置 → 文库」添加项目，保存后回到本页即可管理文档。</p>
 <!-- 无项目时也提供前往设置的入口 -->
 <VButton
 type="secondary"
 size="sm"
 class="mt-4"
 :route="{ path: '/settings', query: { tab: 'wiki' } }"
 >
 <template #icon>
 <Icon icon="ri:settings-3-line" class="h-full w-full" />
 </template>
 前往文库设置
 </VButton>
 </div>

 <div v-else>
 <!-- 项目选择 + 新建文档 -->
 <div class="mb-4 flex flex-wrap items-center gap-2">
 <div class="flex items-center gap-2">
 <Icon icon="ri:folder-open-line" class="text-text-quaternary" />
 <select v-model="activeProject" :class="inputCls + ' w-56'" title="选择文库项目">
 <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.title || p.id }}</option>
 </select>
 <!-- 跳转到设置页文库 tab 管理项目（增删改） -->
 <VButton
 type="secondary"
 size="sm"
 :route="{ path: '/settings', query: { tab: 'wiki' } }"
 title="管理文库项目"
 >
 <template #icon>
 <Icon icon="ri:settings-3-line" class="h-full w-full" />
 </template>
 管理项目
 </VButton>
 </div>
 <VButton type="secondary" size="sm" @click="openCreate">
 <template #icon>
 <Icon icon="ri:add-line" class="h-full w-full" />
 </template>
 新建文档
 </VButton>
 </div>

 <div class="flex flex-col gap-4 lg:flex-row">
 <!-- 左侧：文档树 -->
 <VCard title="文档树" :bodyClass="['!p-0']" class="lg:w-[250px] shrink-0">
 <template #actions>
 <span class="text-xs text-text-quaternary">{{ docs.length }} 篇</span>
 </template>
 <div class="max-h-[70vh] overflow-y-auto p-2">
 <div v-if="docsLoading" class="py-8 text-center text-xs text-text-quaternary">加载中...</div>
 <div v-else-if="!flatTree.length" class="py-8 text-center text-xs text-text-quaternary">
 暂无文档<br />点击「新建文档」创建
 </div>
 <div v-else class="space-y-0.5">
 <button
 v-for="node in flatTree"
 :key="node.doc.id"
 type="button"
 @click="selectDoc(node.doc.slug)"
 :style="{ paddingLeft: 8 + node.depth * 16 + 'px' }"
 :class="[
 'flex w-full items-center gap-1.5 rounded px-2 py-1.5 text-left text-sm transition-colors',
 selectedSlug === node.doc.slug
 ? 'bg-primary/15 text-secondary bg-primary/20 text-primary'
 : 'text-text-secondary hover:bg-secondary hover:bg-accent',
 ]"
 >
 <Icon
 :icon="node.hasChildren ? 'ri:folder-line' : 'ri:file-text-line'"
 class="shrink-0 text-text-quaternary"
 />
 <span class="min-w-0 flex-1 truncate">{{ node.doc.title || node.doc.slug }}</span>
 <span v-if="node.doc.status === 'draft'" title="草稿" class="h-1.5 w-1.5 shrink-0 rounded-full bg-yellow-400"></span>
 </button>
 </div>
 </div>
 </VCard>

 <!-- 右侧：文档编辑区 -->
 <VCard class="min-w-0 flex-1">
 <div v-if="docLoading" class="py-16 text-center text-sm text-text-quaternary">加载中...</div>

 <div v-else-if="!hasCurrent" class="py-16 text-center">
 <Icon icon="ri:file-edit-line" class="mx-auto text-3xl text-text-quaternary" />
 <p class="mt-3 text-sm text-text-tertiary">选择左侧文档开始编辑，或点击「新建文档」</p>
 </div>

 <div v-else class="space-y-4">
 <!-- 元信息 -->
 <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
 <div>
 <label :class="labelCls">标题</label>
 <input v-model="form.title" :class="inputCls" placeholder="文档标题" />
 </div>
 <div>
 <label :class="labelCls">Slug（URL 别名）</label>
 <input v-model="form.slug" :class="inputCls" placeholder="用于 /wiki/:project/:slug" />
 </div>
 <div>
 <label :class="labelCls">排序（升序）</label>
 <input v-model.number="form.order" type="number" min="0" :class="inputCls" />
 </div>
 <div>
 <label :class="labelCls">父文档</label>
 <select v-model="form.parent" :class="inputCls">
 <option :value="0">无（顶级文档）</option>
 <option v-for="d in parentOptions" :key="d.id" :value="d.id">{{ d.title || d.slug }}</option>
 </select>
 </div>
 <div>
 <label :class="labelCls">状态</label>
 <select v-model="form.status" :class="inputCls">
 <option value="draft">草稿</option>
 <option value="published">已发布</option>
 </select>
 </div>
 </div>

 <!-- Markdown 编辑器 -->
 <MarkdownEditor
 ref="editorRef"
 v-model="form.content_md"
 :storage-key="'wiki:' + activeProject + ':' + (originalSlug || 'new')"
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
 <VButton type="danger" :loading="deleting" @click="handleDelete">
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
 </div>

 <!-- 新建文档弹窗 -->
 <VModal
 :visible="createOpen"
 title="新建文档"
 :width="440"
 @update:visible="createOpen = $event"
 @close="createOpen = false"
 >
 <div class="space-y-4">
 <div>
 <label :class="labelCls">标题</label>
 <input v-model="createTitle" :class="inputCls" placeholder="文档标题" autofocus />
 </div>
 <div>
 <label :class="labelCls">Slug（URL 别名）</label>
 <input
 v-model="createSlug"
 :class="inputCls"
 placeholder="自动从标题生成，可手动修改"
 @input="slugManuallyEdited = true"
 />
 </div>
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton @click="createOpen = false">取消</VButton>
 <VButton type="secondary" :loading="creating" @click="handleCreate">创建</VButton>
 </div>
 </template>
 </VModal>
 </div>
</template>
