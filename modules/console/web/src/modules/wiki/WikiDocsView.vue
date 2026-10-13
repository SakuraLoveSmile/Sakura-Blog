<script setup lang="ts">
import { provide, ref, computed, onMounted, watch } from "vue";
import { wikiApi, type WikiProject } from "@/api";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import ContentEditor from "@/components/ContentEditor.vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VCard, VEmpty, VLoading, Toast, Dialog, VModal } from "@/components/ui";

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
  status: "published" | "draft";
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

// ---------- 项目 ----------
const projects = ref<WikiProject[]>([]);
const activeProject = ref("");
const settingsLoading = ref(true);

// ---------- 文档列表 / 树 ----------
const docs = ref<WikiDoc[]>([]);
const flatTree = ref<FlatNode[]>([]);
const docsLoading = ref(false);
const treeKeyword = ref("");

// 文件夹展开状态
const expanded = ref<Set<number>>(new Set());

function toggleFolder(id: number) {
  if (expanded.value.has(id)) expanded.value.delete(id);
  else expanded.value.add(id);
}

function isExpanded(id: number) {
  return expanded.value.has(id);
}

// 按展开状态与搜索过滤可见节点
const visibleTree = computed<FlatNode[]>(() => {
  const kw = treeKeyword.value.trim().toLowerCase();
  if (kw) {
    return flatTree.value.filter(
      (n) =>
        (n.doc.title && n.doc.title.toLowerCase().includes(kw)) ||
        (n.doc.slug && n.doc.slug.toLowerCase().includes(kw)),
    );
  }

  const collapsedAncestors = new Set<number>();
  const out: FlatNode[] = [];
  for (const node of flatTree.value) {
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

const childCountMap = computed<Record<number, number>>(() => {
  const m: Record<number, number> = {};
  for (const d of docs.value) {
    if (d.parent && d.parent > 0) m[d.parent] = (m[d.parent] || 0) + 1;
  }
  return m;
});

// ---------- 统计数据 ----------
const totalDocs = computed(() => docs.value.length);
const currentProject = computed(() => projects.value.find((p) => p.id === activeProject.value) || null);
const thisMonthUpdates = computed(() => {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth();
  return docs.value.filter((d) => {
    const t = new Date(d.updated_at);
    return !isNaN(t.getTime()) && t.getFullYear() === y && t.getMonth() === m;
  }).length;
});
const draftDocsCount = computed(() => docs.value.filter((d) => d.status === "draft").length);

// 面包屑
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
  stellar_version: number;
  stellar_config: string;
  blog_extensions: string;
  title: string;
  slug: string;
  order: number;
  parent: number;
  status: "published" | "draft";
  content_md: string;
}>({
  stellar_version: 2,
  stellar_config: "{}",
  blog_extensions: "{}",
  title: "",
  slug: "",
  order: 0,
  parent: 0,
  status: "draft",
  content_md: "",
});
provide(
  stellarEditorContextKey,
  computed(() => ({
    stellar_version: form.value.stellar_version,
    stellar_config: form.value.stellar_config,
    blog_extensions: form.value.blog_extensions,
  })),
);
const currentId = ref(0);
const originalSlug = ref("");
const selectedSlug = ref("");
const docLoading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editorRef = ref<InstanceType<typeof ContentEditor> | null>(null);

const hasCurrent = computed(() => currentId.value > 0 || !!originalSlug.value);

function collectDescendantIds(pid: number, acc = new Set<number>()): Set<number> {
  for (const d of docs.value) {
    if (d.parent === pid && !acc.has(d.id)) {
      acc.add(d.id);
      collectDescendantIds(d.id, acc);
    }
  }
  return acc;
}

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
  originalSlug.value = "";
  selectedSlug.value = "";
  form.value = {
    stellar_version: 2,
    stellar_config: "{}",
    blog_extensions: "{}",
    title: "",
    slug: "",
    order: 0,
    parent: 0,
    status: "draft",
    content_md: "",
  };
}

// ---------- 新建文档弹窗 ----------
const createOpen = ref(false);
const createTitle = ref("");
const createSlug = ref("");
const createParent = ref(0);
const slugManuallyEdited = ref(false);
const creating = ref(false);

function autoSlug(title: string): string {
  let s = (title || "").trim().toLowerCase();
  s = s.replace(/\s+/g, "-");
  s = s.replace(/[^\p{L}\p{N}-]/gu, "");
  s = s.replace(/-{2,}/g, "-");
  s = s.replace(/^-+|-+$/g, "");
  return s;
}

watch(createTitle, (t) => {
  if (!slugManuallyEdited.value) createSlug.value = autoSlug(t);
});

function openCreate(parentId = 0) {
  createTitle.value = "";
  createSlug.value = "";
  createParent.value = parentId;
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
      parent: createParent.value || 0,
      content_md: "",
      status: "draft",
    });
    createOpen.value = false;
    Toast.success("文档已创建");
    await loadDocs();
    await selectDoc(slug);
  } catch {
  } finally {
    creating.value = false;
  }
}

// ---------- 数据加载 ----------
async function loadProjects() {
  settingsLoading.value = true;
  try {
    const { data } = await wikiApi.listProjects();
    projects.value = Array.isArray(data.items) ? data.items : [];
    if (projects.value.length && !activeProject.value) {
      activeProject.value = projects.value[0].id;
    }
  } catch {
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
    expanded.value = new Set(flatTree.value.filter((n) => n.hasChildren).map((n) => n.doc.id));
  } catch {
  } finally {
    docsLoading.value = false;
  }
}

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
      stellar_version: data.stellar_version ?? 0,
      stellar_config: data.stellar_config || "{}",
      blog_extensions: data.blog_extensions || "{}",
      title: data.title,
      slug: data.slug,
      order: data.order,
      parent: data.parent,
      status: data.status === "published" ? "published" : "draft",
      content_md: data.content_md,
    };
  } catch {
  } finally {
    docLoading.value = false;
  }
}

// ---------- 保存 / 删除 ----------
async function handleSave() {
  await editorRef.value?.flushPendingChanges();
  if (!activeProject.value || !hasCurrent.value) return;
  saving.value = true;
  try {
    const payload = {
      stellar_version: form.value.stellar_version,
      stellar_config: form.value.stellar_config,
      blog_extensions: form.value.blog_extensions,
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
    const newSlug = payload.slug || originalSlug.value;
    await loadDocs();
    if (newSlug !== originalSlug.value) {
      await selectDoc(newSlug);
    } else {
      selectedSlug.value = newSlug;
    }
  } catch {
  } finally {
    saving.value = false;
  }
}

function handleDelete() {
  Dialog.warning({
    title: "确定要删除该文档吗？",
    description: `「${form.value.title || originalSlug.value}」删除后不可恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      if (!activeProject.value) return;
      deleting.value = true;
      try {
        await wikiApi.deleteDoc(activeProject.value, originalSlug.value);
        Toast.success("已删除");
        resetEditor();
        await loadDocs();
      } catch {
      } finally {
        deleting.value = false;
      }
    },
  });
}

function selectAllDocs() {
  resetEditor();
}

onMounted(loadProjects);
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文库管理">
      <template #icon>
        <Icon icon="ri:book-2-line" class="text-primary" />
      </template>
      <template #subtitle>编排多级技术文档与知识库大纲，支持树形导轨与 Markdown 沉浸写作</template>
      <template #actions>
        <!-- 项目选择器 -->
        <select v-model="activeProject" class="apple-select min-w-[160px]" title="选择文库项目">
          <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.title || p.name || p.id }}</option>
        </select>

        <VButton :route="{ path: '/wiki/projects' }">
          <template #icon><Icon icon="ri:settings-3-line" /></template>
          项目设置
        </VButton>

        <VButton type="primary" @click="openCreate(0)">
          <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
          新建文档
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1500px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <div v-if="settingsLoading" class="py-20 text-center text-sm text-text-quaternary">
        <VLoading />
      </div>

      <VEmpty
        v-else-if="!projects.length"
        title="暂无文库项目"
        message="请先前往「文库项目」创建你的第一个技术知识库合辑"
      >
        <template #actions>
          <VButton type="primary" :route="{ path: '/wiki/projects' }"> 前往新建项目 </VButton>
        </template>
      </VEmpty>

      <template v-else>
        <!-- 概览数据指标卡 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:file-list-3-line" />
            </div>
            <div>
              <div class="stats-label">当前项目文档</div>
              <div class="stats-value">{{ totalDocs }} <span class="stats-sub">篇文档</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:checkbox-circle-line" />
            </div>
            <div>
              <div class="stats-label">已公开发布</div>
              <div class="stats-value text-[#30D158]">
                {{ totalDocs - draftDocsCount }} <span class="stats-sub">篇可在线查阅</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
              <Icon icon="ri:draft-line" />
            </div>
            <div>
              <div class="stats-label">草稿状态</div>
              <div class="stats-value text-[#FF9500]">
                {{ draftDocsCount }} <span class="stats-sub">篇暂未发布</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
              <Icon icon="ri:calendar-event-line" />
            </div>
            <div>
              <div class="stats-label">本月更新</div>
              <div class="stats-value">{{ thisMonthUpdates }} <span class="stats-sub">篇更新记录</span></div>
            </div>
          </div>
        </div>

        <!-- 主工作区：左侧文档树 + 右侧内容编辑器 -->
        <div class="flex flex-col gap-5 lg:flex-row lg:items-start">
          <!-- 左侧文档层级树 -->
          <VCard class="w-full lg:w-[320px] shrink-0" :bodyClass="['!p-2.5']">
            <!-- 树顶搜索与全部文档 -->
            <div class="space-y-2 pb-2">
              <div class="apple-search !py-1">
                <Icon icon="ri:search-line" class="text-text-quaternary" />
                <input v-model="treeKeyword" placeholder="过滤文档标题或 Slug..." />
                <button
                  v-if="treeKeyword"
                  type="button"
                  class="text-text-quaternary hover:text-foreground"
                  @click="treeKeyword = ''"
                >
                  <Icon icon="ri:close-line" />
                </button>
              </div>

              <button
                type="button"
                class="flex w-full items-center justify-between rounded-xl px-3 py-2 text-xs font-semibold transition-colors hover:bg-secondary"
                :class="{ 'bg-secondary text-primary': !hasCurrent }"
                @click="selectAllDocs"
              >
                <div class="flex items-center gap-2">
                  <Icon icon="ri:book-open-line" class="text-base text-primary" />
                  <span>全部文档</span>
                </div>
                <span class="rounded-full bg-secondary px-2 py-0.5 font-mono text-[10px] text-text-tertiary">
                  {{ docs.length }} 篇
                </span>
              </button>
            </div>

            <div class="my-1 h-px bg-border/60"></div>

            <VLoading v-if="docsLoading" />

            <div v-else-if="!flatTree.length" class="py-12 text-center text-xs text-text-quaternary">
              暂无文档<br />点击右上角「新建文档」创建
            </div>

            <!-- 树形导轨节点列表 -->
            <div v-else class="space-y-0.5 max-h-[calc(100vh-320px)] overflow-y-auto pr-1">
              <template v-for="node in visibleTree" :key="node.doc.id">
                <!-- 文件夹节点 -->
                <div
                  v-if="node.hasChildren"
                  class="group flex items-center justify-between rounded-xl px-2 py-1.5 text-xs font-medium text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
                  :style="{ paddingLeft: `${8 + node.depth * 14}px` }"
                >
                  <div
                    class="flex items-center gap-1.5 min-w-0 flex-1 cursor-pointer"
                    @click="toggleFolder(node.doc.id)"
                  >
                    <Icon
                      icon="ri:arrow-right-s-line"
                      class="shrink-0 text-text-quaternary transition-transform duration-200"
                      :class="{ 'rotate-90 text-foreground': isExpanded(node.doc.id) }"
                    />
                    <Icon icon="ri:folder-line" class="shrink-0 text-warning" />
                    <span class="truncate">{{ node.doc.title || node.doc.slug }}</span>
                  </div>

                  <div class="flex items-center gap-1">
                    <span
                      v-if="childCountMap[node.doc.id]"
                      class="rounded bg-secondary px-1.5 py-0.2 font-mono text-[10px] text-text-quaternary"
                    >
                      {{ childCountMap[node.doc.id] }}
                    </span>
                    <button
                      type="button"
                      class="hidden p-0.5 text-text-quaternary hover:text-primary group-hover:inline-block"
                      title="新建子文档"
                      @click.stop="openCreate(node.doc.id)"
                    >
                      <Icon icon="ri:add-line" />
                    </button>
                  </div>
                </div>

                <!-- 叶子节点 -->
                <div
                  v-else
                  class="group flex items-center justify-between rounded-xl px-2 py-1.5 text-xs font-medium transition-colors duration-150 cursor-pointer"
                  :class="[
                    selectedSlug === node.doc.slug
                      ? 'bg-primary text-white font-semibold'
                      : 'text-text-secondary hover:bg-secondary hover:text-foreground',
                  ]"
                  :style="{ paddingLeft: `${18 + node.depth * 14}px` }"
                  @click="selectDoc(node.doc.slug)"
                >
                  <div class="flex items-center gap-2 min-w-0 flex-1">
                    <Icon
                      :icon="node.doc.status === 'draft' ? 'ri:file-text-line' : 'ri:article-line'"
                      class="shrink-0"
                      :class="selectedSlug === node.doc.slug ? 'text-white' : 'text-text-tertiary'"
                    />
                    <span class="truncate">{{ node.doc.title || node.doc.slug }}</span>
                  </div>

                  <div class="flex items-center gap-1.5">
                    <span
                      v-if="node.doc.status === 'draft'"
                      class="h-1.5 w-1.5 rounded-full"
                      :class="selectedSlug === node.doc.slug ? 'bg-white' : 'bg-warning'"
                      title="草稿"
                    ></span>
                  </div>
                </div>
              </template>
            </div>
          </VCard>

          <!-- 右侧沉浸式内容编辑区 -->
          <div class="min-w-0 flex-1 space-y-4">
            <VLoading v-if="docLoading" />

            <!-- 未选择文档状态 -->
            <VCard v-else-if="!hasCurrent" class="py-16 text-center">
              <div
                class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-secondary text-3xl text-text-tertiary"
              >
                <Icon icon="ri:file-edit-line" />
              </div>
              <h3 class="mt-4 text-base font-semibold text-foreground">选择左侧文档开始编排</h3>
              <p class="mt-1 text-xs text-text-quaternary">可在左侧树形大纲中点击任意文档，或点击右上角「新建文档」</p>
              <div class="mt-5 flex justify-center gap-3">
                <VButton type="primary" @click="openCreate(0)">
                  <template #icon><Icon icon="ri:add-line" /></template>
                  新建文档
                </VButton>
              </div>
            </VCard>

            <!-- 文档编辑表单 -->
            <VCard v-else>
              <!-- 面包屑导航 -->
              <div class="mb-4 flex items-center gap-1.5 text-xs text-text-tertiary border-b border-border/60 pb-3">
                <Icon icon="ri:folder-2-line" class="text-primary" />
                <template v-for="(crumb, idx) in breadcrumb" :key="idx">
                  <span
                    class="cursor-pointer hover:text-foreground"
                    :class="{ 'font-semibold text-foreground': idx === breadcrumb.length - 1 }"
                    @click="idx === 0 ? selectAllDocs() : crumb.slug ? selectDoc(crumb.slug) : null"
                  >
                    {{ crumb.title }}
                  </span>
                  <span v-if="idx < breadcrumb.length - 1" class="text-text-quaternary">/</span>
                </template>
              </div>

              <!-- 文档属性行 -->
              <div class="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
                <div class="lg:col-span-2">
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">文档标题 *</label>
                  <input v-model="form.title" class="apple-input" placeholder="输入文档标题..." />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">Slug（URL 别名）*</label>
                  <input v-model="form.slug" class="apple-input" placeholder="url-slug" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">归属父文档</label>
                  <select v-model="form.parent" class="apple-select w-full">
                    <option :value="0">无（根目录文档）</option>
                    <option v-for="d in parentOptions" :key="d.id" :value="d.id">{{ d.title || d.slug }}</option>
                  </select>
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">发布状态</label>
                  <select v-model="form.status" class="apple-select w-full">
                    <option value="draft">草稿</option>
                    <option value="published">已发布</option>
                  </select>
                </div>
              </div>

              <!-- Markdown 编辑器 -->
              <StellarContentConfig
                v-model:config="form.stellar_config"
                v-model:blog="form.blog_extensions"
                :version="form.stellar_version"
                profile="wiki"
                :content="form.content_md"
                class="mb-3"
              />
              <ContentEditor
                ref="editorRef"
                v-model="form.content_md"
                :storage-key="'wiki:' + activeProject + ':' + (originalSlug || 'new')"
                min-height="500px"
                @save="handleSave"
              />

              <template #footer>
                <div class="flex items-center justify-between w-full">
                  <div class="flex items-center gap-2">
                    <VButton type="primary" :loading="saving" @click="handleSave">
                      <template #icon><Icon icon="ri:save-line" /></template>
                      保存文档
                    </VButton>
                    <VButton type="danger" :loading="deleting" @click="handleDelete">
                      <template #icon><Icon icon="ri:delete-bin-line" /></template>
                      删除
                    </VButton>
                  </div>
                  <span class="text-xs text-text-quaternary"> 快捷键 ⌘ + S 实时保存 </span>
                </div>
              </template>
            </VCard>
          </div>
        </div>
      </template>
    </div>

    <!-- 新建文档弹窗 -->
    <VModal v-model:visible="createOpen" title="新建 Wiki 文档" :width="440">
      <div class="space-y-4">
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">文档标题 *</label>
          <input v-model="createTitle" class="apple-input" placeholder="如：快速入门指南" autofocus />
        </div>
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">Slug（URL 路径）*</label>
          <input
            v-model="createSlug"
            class="apple-input"
            placeholder="自动生成，也可手动指定"
            @input="slugManuallyEdited = true"
          />
        </div>
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">父节点</label>
          <select v-model="createParent" class="apple-select w-full">
            <option :value="0">无（根目录）</option>
            <option v-for="d in docs" :key="d.id" :value="d.id">{{ d.title || d.slug }}</option>
          </select>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="createOpen = false">取消</VButton>
          <VButton type="primary" :loading="creating" @click="handleCreate">立即创建</VButton>
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
  /* highlight removed */
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
