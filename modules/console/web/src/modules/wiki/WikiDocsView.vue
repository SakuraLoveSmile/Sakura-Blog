<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { wikiApi } from "@/api";
import type { WikiProject } from "@/api";
import ContentEditor from "@/components/ContentEditor.vue";
import { Icon } from "@iconify/vue";
import { VButton, Toast, Dialog, VModal } from "@/components/ui";

// ---------- 类型 ----------
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

// ---------- 项目（SQLite wiki_projects） ----------
const projects = ref<WikiProject[]>([]);
const activeProject = ref("");
const settingsLoading = ref(true);

// ---------- 文档列表 / 树 ----------
const docs = ref<WikiDoc[]>([]);
const flatTree = ref<FlatNode[]>([]);
const docsLoading = ref(false);

// 文件夹（含子文档的节点）展开状态，默认全部展开
const expanded = ref<Set<number>>(new Set());

function toggleFolder(id: number) {
  if (expanded.value.has(id)) {
    expanded.value.delete(id);
  } else {
    expanded.value.add(id);
  }
}

function isExpanded(id: number) {
  return expanded.value.has(id);
}

// 按展开状态过滤可见节点：若任一祖先被折叠，则该节点隐藏
const visibleTree = computed<FlatNode[]>(() => {
  const collapsedAncestors = new Set<number>();
  const out: FlatNode[] = [];
  for (const node of flatTree.value) {
    // 向上查找祖先链是否包含被折叠的文件夹
    let hidden = false;
    for (const cid of collapsedAncestors) {
      if (isAncestor(cid, node.doc, docs.value)) {
        hidden = true;
        break;
      }
    }
    if (hidden) continue;
    out.push(node);
    if (node.hasChildren && !expanded.value.has(node.doc.id)) {
      collapsedAncestors.add(node.doc.id);
    }
  }
  return out;
});

// 判断 ancestorId 是否是 target 的祖先（沿 parent 链）
function isAncestor(ancestorId: number, target: WikiDoc, all: WikiDoc[]): boolean {
  let pid = target.parent;
  const guard = new Set<number>();
  while (pid && pid > 0 && !guard.has(pid)) {
    if (pid === ancestorId) return true;
    guard.add(pid);
    const p = all.find((d) => d.id === pid);
    if (!p) break;
    pid = p.parent;
  }
  return false;
}

// 每个文件夹的直接子文档数量（用于 count-badge）
const childCountMap = computed<Record<number, number>>(() => {
  const m: Record<number, number> = {};
  for (const d of docs.value) {
    if (d.parent && d.parent > 0) m[d.parent] = (m[d.parent] || 0) + 1;
  }
  return m;
});

// ---------- 统计卡（数据均从现有 docs/projects 派生，不新增 API） ----------
const totalDocs = computed(() => docs.value.length);
const projectCount = computed(() => projects.value.length);
const thisMonthUpdates = computed(() => {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth();
  return docs.value.filter((d) => {
    const t = new Date(d.updated_at);
    return !isNaN(t.getTime()) && t.getFullYear() === y && t.getMonth() === m;
  }).length;
});

// 当前项目对象
const currentProject = computed(() =>
  projects.value.find((p) => p.id === activeProject.value) || null,
);

// 面包屑：项目名 / 当前文档的祖先链 / 当前文档标题
const breadcrumb = computed<{ title: string; slug?: string }[]>(() => {
  const crumbs: { title: string; slug?: string }[] = [];
  if (currentProject.value) {
    crumbs.push({ title: currentProject.value.title || currentProject.value.id });
  }
  if (hasCurrent.value) {
    const chain: WikiDoc[] = [];
    const byId = new Map(docs.value.map((d) => [d.id, d]));
    let cur = byId.get(currentId.value);
    const guard = new Set<number>();
    while (cur && cur.parent > 0 && !guard.has(cur.id)) {
      guard.add(cur.id);
      chain.unshift(cur);
      cur = byId.get(cur.parent);
    }
    for (const c of chain) {
      crumbs.push({ title: c.title || c.slug, slug: c.slug });
    }
  }
  return crumbs;
});

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
const editorRef = ref<InstanceType<typeof ContentEditor> | null>(null);

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
 const { data } = await wikiApi.listProjects();
 // 项目列表由后端按 pin/sort/id 稳定排序。
 projects.value = Array.isArray(data.items) ? data.items : [];
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
    // 默认展开所有含子文档的文件夹
    expanded.value = new Set(
      flatTree.value.filter((n) => n.hasChildren).map((n) => n.doc.id),
    );
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载文档失败");
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
const selectCls = "apple-select";
const labelCls = "block text-xs text-text-tertiary mb-1";

// 点击「全部文档」：清空当前编辑态
function selectAllDocs() {
  resetEditor();
}

onMounted(loadProjects);
</script>

<template>
  <div class="wiki-docs">
    <!-- 顶部标题栏 -->
    <header class="wiki-header">
      <div class="min-w-0 flex-1">
        <h1 class="wiki-title">
          <Icon icon="ri:book-2-line" class="mr-2 self-center text-[22px] text-primary" />
          文库管理
        </h1>
        <p class="wiki-subtitle">管理文档树结构、编辑内容与发布状态</p>
      </div>
      <div class="flex shrink-0 items-center gap-2.5">
        <select v-model="activeProject" :class="selectCls + ' wiki-project-select'" title="选择文库项目">
          <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.title || p.id }}</option>
        </select>
        <VButton
          type="secondary"
          size="sm"
          :route="{ path: '/wiki/projects' }"
          title="管理文库项目"
        >
          <template #icon>
            <Icon icon="ri:settings-3-line" class="h-full w-full" />
          </template>
          管理项目
        </VButton>
        <VButton type="primary" size="sm" @click="openCreate">
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          新建文档
        </VButton>
      </div>
    </header>

    <div class="wiki-body">
      <div v-if="settingsLoading" class="py-16 text-center text-sm text-text-quaternary">加载中...</div>

      <div v-else-if="!projects.length" class="wiki-empty-state">
        <div class="wiki-empty-icon">
          <Icon icon="ri:book-2-line" class="text-3xl" />
        </div>
        <p class="mt-4 text-sm font-medium text-text-tertiary">暂无文库项目</p>
        <p class="mt-1 text-xs text-text-quaternary">请先到「设置 → 文库」添加项目，保存后回到本页即可管理文档。</p>
        <VButton
          type="secondary"
          size="sm"
          class="mt-4"
          :route="{ path: '/wiki/projects' }"
        >
          <template #icon>
            <Icon icon="ri:settings-3-line" class="h-full w-full" />
          </template>
          前往文库设置
        </VButton>
      </div>

      <div v-else class="wiki-content-wrapper">
        <!-- 左侧：文档树 -->
        <aside class="wiki-doc-tree">
          <button type="button" class="wiki-tree-all" @click="selectAllDocs">
            <Icon icon="ri:book-open-line" class="text-base text-primary" />
            <span>全部文档</span>
            <span class="wiki-tree-count">{{ docs.length }} 篇</span>
          </button>

          <div v-if="docsLoading" class="py-8 text-center text-xs text-text-quaternary">加载中...</div>
          <div v-else-if="!flatTree.length" class="py-8 text-center text-xs text-text-quaternary">
            暂无文档<br />点击「新建文档」创建
          </div>

          <div v-else class="wiki-tree-list">
            <template v-for="node in visibleTree" :key="node.doc.id">
              <!-- 含子文档的文件夹节点 -->
              <button
                v-if="node.hasChildren"
                type="button"
                class="wiki-tree-folder"
                :class="{ expanded: isExpanded(node.doc.id) }"
                :style="{ paddingLeft: 12 + node.depth * 14 + 'px' }"
                @click="toggleFolder(node.doc.id)"
              >
                <Icon icon="ri:arrow-right-s-line" class="wiki-tree-chevron" />
                <Icon icon="ri:folder-line" class="wiki-tree-folder-icon text-warning" />
                <span class="min-w-0 flex-1 truncate text-left">{{ node.doc.title || node.doc.slug }}</span>
                <span v-if="childCountMap[node.doc.id]" class="wiki-count-badge">
                  {{ childCountMap[node.doc.id] }}
                </span>
              </button>
              <!-- 叶子文档节点 -->
              <button
                v-else
                type="button"
                class="wiki-tree-child"
                :class="{ active: selectedSlug === node.doc.slug }"
                :style="{ paddingLeft: 12 + node.depth * 14 + 'px' }"
                @click="selectDoc(node.doc.slug)"
              >
                <Icon
                  :icon="node.doc.status === 'draft' ? 'ri:file-text-line' : 'ri:article-line'"
                  class="wiki-tree-doc-icon"
                />
                <span class="min-w-0 flex-1 truncate text-left">{{ node.doc.title || node.doc.slug }}</span>
                <span v-if="node.doc.status === 'draft'" title="草稿" class="h-1.5 w-1.5 shrink-0 rounded-full bg-warning"></span>
              </button>
            </template>
          </div>
        </aside>

        <!-- 右侧：内容区 -->
        <div class="wiki-doc-content">
          <!-- 面包屑 -->
          <nav class="wiki-breadcrumb">
            <template v-for="(crumb, idx) in breadcrumb" :key="idx">
              <span
                class="wiki-breadcrumb-item"
                :class="{ current: idx === breadcrumb.length - 1 && hasCurrent }"
                @click="idx === 0 ? selectAllDocs() : (crumb.slug ? selectDoc(crumb.slug) : null)"
              >{{ crumb.title }}</span>
              <svg
                v-if="idx < breadcrumb.length - 1"
                class="wiki-breadcrumb-sep"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </template>
          </nav>

          <!-- 统计卡 -->
          <div class="wiki-stats-row">
            <div class="wiki-stat-card">
              <div class="wiki-stat-icon blue">
                <Icon icon="ri:file-list-3-line" class="text-lg" />
              </div>
              <div class="wiki-stat-value">{{ totalDocs }}</div>
              <div class="wiki-stat-label">总文档数</div>
            </div>
            <div class="wiki-stat-card">
              <div class="wiki-stat-icon green">
                <Icon icon="ri:folder-3-line" class="text-lg" />
              </div>
              <div class="wiki-stat-value">{{ projectCount }}</div>
              <div class="wiki-stat-label">项目数</div>
            </div>
            <div class="wiki-stat-card">
              <div class="wiki-stat-icon orange">
                <Icon icon="ri:calendar-event-line" class="text-lg" />
              </div>
              <div class="wiki-stat-value">{{ thisMonthUpdates }}</div>
              <div class="wiki-stat-label">本月更新</div>
            </div>
          </div>

          <!-- 编辑卡片 -->
          <div class="wiki-editor-card">
            <div v-if="docLoading" class="py-16 text-center text-sm text-text-quaternary">加载中...</div>

            <div v-else-if="!hasCurrent" class="wiki-editor-empty">
              <div class="wiki-editor-empty-icon">
                <Icon icon="ri:file-edit-line" class="text-3xl" />
              </div>
              <p class="mt-4 text-sm text-text-tertiary">选择左侧文档开始编辑，或点击「新建文档」</p>
            </div>

            <div v-else class="wiki-editor-body">
              <!-- 元信息表单 -->
              <div class="wiki-form-grid">
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
                  <select v-model="form.parent" :class="selectCls + ' w-full'">
                    <option :value="0">无（顶级文档）</option>
                    <option v-for="d in parentOptions" :key="d.id" :value="d.id">{{ d.title || d.slug }}</option>
                  </select>
                </div>
                <div>
                  <label :class="labelCls">状态</label>
                  <select v-model="form.status" :class="selectCls + ' w-full'">
                    <option value="draft">草稿</option>
                    <option value="published">已发布</option>
                  </select>
                </div>
              </div>

              <!-- 内容编辑器（富文本/Markdown 双模式） -->
              <ContentEditor
                ref="editorRef"
                v-model="form.content_md"
                :storage-key="'wiki:' + activeProject + ':' + (originalSlug || 'new')"
                min-height="450px"
                @save="handleSave"
              />

              <!-- 操作按钮 -->
              <div class="wiki-editor-actions">
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
          </div>
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

<style scoped>
.wiki-docs {
  position: relative;
  min-height: 100vh;
}

/* ===== 顶部标题栏 ===== */
.wiki-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 24px 40px 18px;
  border-bottom: 1px solid var(--color-border);
  background: linear-gradient(180deg, color-mix(in srgb, #000 90%, transparent) 0%, color-mix(in srgb, #000 60%, transparent) 80%, transparent 100%);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
}
.wiki-title {
  display: flex;
  align-items: center;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--color-foreground);
}
.wiki-subtitle {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-tertiary);
}
.wiki-project-select {
  min-width: 180px;
}

/* ===== 主体 ===== */
.wiki-body {
  position: relative;
  z-index: 1;
  padding: 24px 40px 40px;
}
.wiki-content-wrapper {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

/* ===== 空状态 ===== */
.wiki-empty-state {
  max-width: 480px;
  margin: 40px auto;
  padding: 48px 32px;
  text-align: center;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}
.wiki-empty-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin: 0 auto;
  color: var(--color-text-quaternary);
  background: var(--color-secondary);
  border-radius: 16px;
}

/* ===== 左侧文档树 ===== */
.wiki-doc-tree {
  position: sticky;
  top: 96px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 240px;
  flex-shrink: 0;
  max-height: calc(100vh - 120px);
  padding: 12px;
  overflow-y: auto;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

/* 全部文档按钮 */
.wiki-tree-all {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
  cursor: pointer;
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary) 25%, transparent);
  border-radius: var(--radius-md);
  transition: background var(--duration-normal) var(--ease-standard);
}
.wiki-tree-all:hover {
  background: color-mix(in srgb, var(--color-primary) 22%, transparent);
}
.wiki-tree-count {
  margin-left: auto;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-tertiary);
}

.wiki-tree-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 4px;
}

/* 文件夹节点 */
.wiki-tree-folder {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 7px 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  transition: background var(--duration-normal) var(--ease-standard);
  user-select: none;
}
.wiki-tree-folder:hover {
  background: var(--color-secondary);
}
.wiki-tree-chevron {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  color: var(--color-text-quaternary);
  transition: transform 0.2s var(--ease-standard);
}
.wiki-tree-folder.expanded .wiki-tree-chevron {
  transform: rotate(90deg);
}
.wiki-tree-folder-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
}
.wiki-count-badge {
  margin-left: auto;
  padding: 1px 8px;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-tertiary);
  background: var(--color-accent);
  border-radius: var(--radius-full);
}

/* 叶子文档节点 */
.wiki-tree-child {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 10px;
  font-size: 13px;
  color: var(--color-text-tertiary);
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: none;
  border-left: 3px solid transparent;
  border-radius: 6px;
  transition: all var(--duration-normal) var(--ease-standard);
}
.wiki-tree-child:hover {
  color: var(--color-foreground);
  background: var(--color-secondary);
}
.wiki-tree-child.active {
  font-weight: 500;
  color: var(--color-foreground);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
  border-left-color: var(--color-primary);
}
.wiki-tree-doc-icon {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  color: var(--color-primary);
}

/* ===== 右侧内容区 ===== */
.wiki-doc-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

/* 面包屑 */
.wiki-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--color-text-tertiary);
}
.wiki-breadcrumb-item {
  cursor: pointer;
  transition: color var(--duration-normal) var(--ease-standard);
}
.wiki-breadcrumb-item:hover {
  color: var(--color-foreground);
}
.wiki-breadcrumb-item.current {
  font-weight: 500;
  color: var(--color-foreground);
  cursor: default;
}
.wiki-breadcrumb-item.current:hover {
  color: var(--color-foreground);
}
.wiki-breadcrumb-sep {
  width: 12px;
  height: 12px;
  color: var(--color-text-quaternary);
  opacity: 0.5;
}

/* 统计卡 */
.wiki-stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.wiki-stat-card {
  padding: 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  transition: all var(--duration-normal) var(--ease-standard);
}
.wiki-stat-card:hover {
  border-color: var(--color-border-strong);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}
.wiki-stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  margin-bottom: 12px;
  border-radius: var(--radius-base);
}
.wiki-stat-icon.blue {
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
}
.wiki-stat-icon.green {
  color: var(--color-success);
  background: color-mix(in srgb, var(--color-state-success) 15%, transparent);
}
.wiki-stat-icon.orange {
  color: var(--color-warning);
  background: color-mix(in srgb, var(--color-state-warning) 15%, transparent);
}
.wiki-stat-value {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.02em;
  color: var(--color-foreground);
}
.wiki-stat-label {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* 编辑卡片 */
.wiki-editor-card {
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.wiki-editor-empty {
  padding: 64px 24px;
  text-align: center;
}
.wiki-editor-empty-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin: 0 auto;
  color: var(--color-text-quaternary);
  background: var(--color-secondary);
  border-radius: 16px;
}
.wiki-editor-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px;
}
.wiki-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.wiki-editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 900px) {
  .wiki-content-wrapper {
    flex-direction: column;
  }
  .wiki-doc-tree {
    position: static;
    width: 100%;
    max-height: none;
  }
  .wiki-stats-row {
    grid-template-columns: 1fr;
  }
  .wiki-form-grid {
    grid-template-columns: 1fr;
  }
  .wiki-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
