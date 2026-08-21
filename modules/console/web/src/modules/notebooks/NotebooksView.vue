<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { notebookApi } from "@/api";
import ContentEditor from "@/components/ContentEditor.vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEmpty,
  VLoading,
  Toast,
  Dialog,
  VModal,
} from "@/components/ui";

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

const route = useRoute();
const router = useRouter();

const notebooks = ref<Notebook[]>([]);
const noteCounts = ref<Record<number, number>>({});
const loading = ref(false);
const searchQuery = ref("");
const noteSearchQuery = ref("");

// 激活的笔记本与笔记
const activeNotebook = ref<Notebook | null>(null);
const currentId = ref(0);
const notes = ref<Note[]>([]);
const notesTotal = ref(0);
const notesPage = ref(1);
const perPage = 20;
const notesLoading = ref(false);

const noteForm = ref({ title: "", tags: "", content: "" });
const noteLoading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editorRef = ref<InstanceType<typeof ContentEditor> | null>(null);

const hasCurrent = computed(() => currentId.value > 0);
const totalPages = computed(() => Math.max(1, Math.ceil(notesTotal.value / perPage)));

// 笔记本弹窗
const nbModalOpen = ref(false);
const nbModalMode = ref<"create" | "edit">("create");
const nbEditingId = ref(0);
const nbForm = ref({ name: "", description: "", cover: "" });
const nbSaving = ref(false);

// 新建笔记弹窗
const createNoteOpen = ref(false);
const createNoteTitle = ref("");
const createNoteSaving = ref(false);

const allTags = ref<string[]>([]);

// ---------- 统计数据 ----------
const totalNotesCount = computed(() => {
  return Object.values(noteCounts.value).reduce((sum, n) => sum + (n || 0), 0);
});

const filteredNotebooks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return notebooks.value;
  return notebooks.value.filter(
    (nb) => nb.name.toLowerCase().includes(q) || (nb.description || "").toLowerCase().includes(q)
  );
});

const filteredNotes = computed(() => {
  const q = noteSearchQuery.value.trim().toLowerCase();
  if (!q) return notes.value;
  return notes.value.filter(
    (n) =>
      (n.title || "").toLowerCase().includes(q) ||
      (n.tags || "").toLowerCase().includes(q) ||
      (n.content || "").toLowerCase().includes(q)
  );
});

function notebookCoverClass(id: number): string {
  const gradients = [
    "avatar-gradient-1",
    "avatar-gradient-2",
    "avatar-gradient-3",
    "avatar-gradient-4",
    "avatar-gradient-5",
  ];
  return gradients[Math.abs(id) % gradients.length];
}

function noteTags(note: Note): string[] {
  return (note.tags || "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function fmtTime(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// ---------- 笔记本操作 ----------
async function loadNotebooks() {
  loading.value = true;
  try {
    const { data } = await notebookApi.listNotebooks();
    notebooks.value = data.items || [];
    noteCounts.value = Object.fromEntries(
      (data.items || []).map((nb: any) => [nb.id, nb.note_count || 0])
    );
    // 从路由参数恢复激活的笔记本
    const nbIdParam = Number(route.query.nb);
    if (nbIdParam > 0 && !activeNotebook.value) {
      const target = notebooks.value.find((n) => n.id === nbIdParam);
      if (target) {
        enterNotebook(target, false);
      }
    }
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载笔记本失败");
  } finally {
    loading.value = false;
  }
}

async function loadTags() {
  try {
    const { data } = await notebookApi.getTags();
    allTags.value = data.tags || [];
  } catch {
    allTags.value = [];
  }
}

function openCreateNotebook() {
  nbModalMode.value = "create";
  nbEditingId.value = 0;
  nbForm.value = { name: "", description: "", cover: "" };
  nbModalOpen.value = true;
}

function openEditNotebook(nb: Notebook) {
  nbModalMode.value = "edit";
  nbEditingId.value = nb.id;
  nbForm.value = { name: nb.name, description: nb.description || "", cover: nb.cover || "" };
  nbModalOpen.value = true;
}

async function handleSaveNotebook() {
  const name = nbForm.value.name.trim();
  if (!name) {
    Toast.warning("请输入笔记本名称");
    return;
  }
  nbSaving.value = true;
  try {
    const payload = {
      name,
      description: nbForm.value.description.trim(),
      cover: nbForm.value.cover.trim(),
    };
    if (nbModalMode.value === "create") {
      await notebookApi.createNotebook(payload);
      Toast.success("笔记本已创建");
    } else {
      await notebookApi.updateNotebook(nbEditingId.value, payload);
      if (activeNotebook.value?.id === nbEditingId.value) {
        activeNotebook.value = { ...activeNotebook.value, ...payload };
      }
      Toast.success("笔记本已保存");
    }
    nbModalOpen.value = false;
    await loadNotebooks();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    nbSaving.value = false;
  }
}

function handleDeleteNotebook(nb: Notebook) {
  Dialog.warning({
    title: `确定删除笔记本「${nb.name}」吗？`,
    description: "其包含的全部笔记将一并级联删除，不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await notebookApi.deleteNotebook(nb.id);
        Toast.success("笔记本已删除");
        if (activeNotebook.value?.id === nb.id) {
          backToNotebooks();
        }
        await loadNotebooks();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

function enterNotebook(nb: Notebook, updateRoute = true) {
  activeNotebook.value = nb;
  resetNoteEditor();
  if (updateRoute) {
    router.replace({ query: { ...route.query, nb: nb.id, note: undefined } });
  }
  loadNotes(1);
}

function backToNotebooks() {
  activeNotebook.value = null;
  resetNoteEditor();
  router.replace({ query: { ...route.query, nb: undefined, note: undefined } });
  loadNotebooks();
}

// ---------- 笔记操作 ----------
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
    if (activeNotebook.value) {
      noteCounts.value[activeNotebook.value.id] = data.total || 0;
    }

    // 从路由恢复激活笔记
    const noteIdParam = Number(route.query.note);
    if (noteIdParam > 0 && !hasCurrent.value) {
      const match = notes.value.find((n) => n.id === noteIdParam);
      if (match) {
        selectNote(match.id, false);
      }
    }
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载笔记失败");
  } finally {
    notesLoading.value = false;
  }
}

function resetNoteEditor() {
  currentId.value = 0;
  noteForm.value = { title: "", tags: "", content: "" };
}

async function selectNote(id: number, updateRoute = true) {
  noteLoading.value = true;
  try {
    const { data } = await notebookApi.getNote(id);
    currentId.value = data.id;
    noteForm.value = {
      title: data.title || "",
      tags: data.tags || "",
      content: data.content || "",
    };
    if (updateRoute && activeNotebook.value) {
      router.replace({ query: { ...route.query, nb: activeNotebook.value.id, note: id } });
    }
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载笔记内容失败");
  } finally {
    noteLoading.value = false;
  }
}

function openCreateNote() {
  createNoteTitle.value = "";
  createNoteOpen.value = true;
}

async function handleCreateNote() {
  if (!activeNotebook.value) return;
  const title = createNoteTitle.value.trim();
  if (!title) {
    Toast.warning("请输入笔记标题");
    return;
  }
  createNoteSaving.value = true;
  try {
    const { data } = await notebookApi.createNote(activeNotebook.value.id, {
      title,
      content: "",
      tags: "",
    });
    createNoteOpen.value = false;
    Toast.success("笔记已创建");
    await loadNotes(1);
    await selectNote(data.id);
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "创建失败");
  } finally {
    createNoteSaving.value = false;
  }
}

async function handleSave() {
  if (!hasCurrent.value) return;
  if (!noteForm.value.title.trim()) {
    Toast.warning("笔记标题不能为空");
    return;
  }
  saving.value = true;
  try {
    await notebookApi.updateNote(currentId.value, {
      title: noteForm.value.title.trim(),
      tags: noteForm.value.tags.trim(),
      content: noteForm.value.content,
    });
    editorRef.value?.clearDraft();
    Toast.success("笔记已保存");
    await loadNotes(notesPage.value);
    await loadTags();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

function handleDeleteNote() {
  Dialog.warning({
    title: `确定删除笔记「${noteForm.value.title || "未命名笔记"}」吗？`,
    description: "删除后不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      if (!hasCurrent.value) return;
      deleting.value = true;
      try {
        await notebookApi.deleteNote(currentId.value);
        Toast.success("笔记已删除");
        resetNoteEditor();
        let page = notesPage.value;
        if (notes.value.length === 1 && page > 1) page -= 1;
        await loadNotes(page);
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      } finally {
        deleting.value = false;
      }
    },
  });
}

onMounted(() => {
  loadNotebooks();
  loadTags();
});
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader :title="activeNotebook ? activeNotebook.name : '笔记本'">
      <template #icon>
        <button
          v-if="activeNotebook"
          type="button"
          class="mr-2 flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-card text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
          title="返回笔记本列表"
          @click="backToNotebooks"
        >
          <Icon icon="ri:arrow-left-line" class="text-lg" />
        </button>
        <Icon v-else icon="ri:book-open-line" class="text-primary" />
      </template>
      <template #subtitle>
        {{ activeNotebook ? (activeNotebook.description || "管理该笔记本下的全部笔记") : "管理你的所有笔记本与 Markdown 备忘录" }}
      </template>
      <template #actions>
        <template v-if="!activeNotebook">
          <div class="apple-search min-w-[200px] max-w-[280px]">
            <Icon icon="ri:search-line" class="text-text-quaternary" />
            <input v-model="searchQuery" placeholder="搜索笔记本..." />
            <button v-if="searchQuery" type="button" class="text-text-quaternary hover:text-foreground" @click="searchQuery = ''">
              <Icon icon="ri:close-line" />
            </button>
          </div>

          <VButton type="primary" @click="openCreateNotebook">
            <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
            新建笔记本
          </VButton>
        </template>

        <template v-else>
          <VButton size="sm" @click="openEditNotebook(activeNotebook)">
            <template #icon><Icon icon="ri:edit-line" /></template>
            编辑本本
          </VButton>
          <VButton type="primary" size="sm" @click="openCreateNote">
            <template #icon><Icon icon="ri:add-line" /></template>
            新建笔记
          </VButton>
        </template>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />

      <!-- ============ 视图 1：笔记本卡片网格 ============ -->
      <template v-else-if="!activeNotebook">
        <!-- 统计指标 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:book-2-line" />
            </div>
            <div>
              <div class="stats-label">全部笔记本</div>
              <div class="stats-value">{{ notebooks.length }} <span class="stats-sub">个合辑分类</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:file-text-line" />
            </div>
            <div>
              <div class="stats-label">笔记总数</div>
              <div class="stats-value text-[#30D158]">{{ totalNotesCount }} <span class="stats-sub">篇归档笔记</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30B0C7] bg-[#30B0C7]/10">
              <Icon icon="ri:price-tag-3-line" />
            </div>
            <div>
              <div class="stats-label">笔记标签</div>
              <div class="stats-value">{{ allTags.length }} <span class="stats-sub">个聚合标签</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
              <Icon icon="ri:sparkling-line" />
            </div>
            <div>
              <div class="stats-label">前台呈现</div>
              <div class="stats-value">/notes <span class="stats-sub">即时同步展示</span></div>
            </div>
          </div>
        </div>

        <VEmpty
          v-if="filteredNotebooks.length === 0"
          :title="searchQuery ? '未找到匹配的笔记本' : '暂无笔记本'"
          :message="searchQuery ? '换个搜索关键词试试' : '点击右上角「新建笔记本」创建首个合辑'"
        >
          <template #actions>
            <VButton v-if="!searchQuery" size="sm" type="primary" @click="openCreateNotebook">
              新建首个笔记本
            </VButton>
          </template>
        </VEmpty>

        <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="nb in filteredNotebooks"
            :key="nb.id"
            class="notebook-card group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-border bg-card transition-all hover:-translate-y-1 hover:border-border-strong hover:shadow-lg"
          >
            <!-- 封面区 -->
            <div
              class="relative aspect-[2.4/1] w-full overflow-hidden cursor-pointer"
              :class="notebookCoverClass(nb.id)"
              @click="enterNotebook(nb)"
            >
              <img
                v-if="nb.cover"
                :src="nb.cover"
                :alt="nb.name"
                class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
              />
              <div
                v-else
                class="flex h-full w-full items-center justify-center text-3xl text-white/90"
              >
                <Icon icon="ri:book-open-line" />
              </div>

              <!-- 篇数药丸 -->
              <span class="absolute bottom-3 right-3 rounded-full bg-black/60 px-2.5 py-0.5 text-xs font-semibold text-white backdrop-blur-md">
                {{ noteCounts[nb.id] || 0 }} 篇笔记
              </span>
            </div>

            <!-- 卡片主体 -->
            <div class="flex flex-1 flex-col justify-between p-4">
              <div class="cursor-pointer" @click="enterNotebook(nb)">
                <h3 class="truncate text-base font-bold text-foreground group-hover:text-primary transition-colors">
                  {{ nb.name }}
                </h3>
                <p class="mt-1.5 line-clamp-2 text-xs leading-relaxed text-text-tertiary">
                  {{ nb.description || "未填写笔记本描述信息" }}
                </p>
              </div>

              <div class="mt-4 flex items-center justify-between border-t border-border/50 pt-3 text-xs text-text-quaternary">
                <span class="flex items-center gap-1">
                  <Icon icon="ri:time-line" />
                  {{ fmtTime(nb.created_at) || "刚刚创建" }}
                </span>

                <div class="flex items-center gap-1">
                  <button
                    type="button"
                    class="action-icon-btn"
                    title="编辑笔记本"
                    @click.stop="openEditNotebook(nb)"
                  >
                    <Icon icon="ri:pencil-line" />
                  </button>
                  <button
                    type="button"
                    class="action-icon-btn hover:text-[var(--color-state-error)]"
                    title="删除笔记本"
                    @click.stop="handleDeleteNotebook(nb)"
                  >
                    <Icon icon="ri:delete-bin-line" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- ============ 视图 2：Apple Notes 风格双栏/三栏笔记编辑器 ============ -->
      <template v-else>
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start">
          <!-- 栏 1：笔记列表面板 -->
          <VCard class="w-full lg:w-[320px] shrink-0" :bodyClass="['!p-0']">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-semibold text-sm text-foreground">笔记列表</span>
                <span class="rounded-full bg-secondary px-2 py-0.5 text-xs text-text-tertiary">
                  {{ notesTotal }} 篇
                </span>
              </div>
            </template>

            <!-- 搜索框 -->
            <div class="p-3 border-b border-border/60">
              <div class="apple-search h-8 text-xs">
                <Icon icon="ri:search-line" class="text-text-quaternary" />
                <input v-model="noteSearchQuery" placeholder="过滤当前笔记..." class="text-xs" />
              </div>
            </div>

            <!-- 笔记项列表 -->
            <div class="max-h-[65vh] overflow-y-auto p-2 space-y-1">
              <div v-if="notesLoading" class="py-8 text-center text-xs text-text-quaternary">
                加载笔记中...
              </div>

              <div v-else-if="filteredNotes.length === 0" class="py-12 text-center text-xs text-text-quaternary">
                {{ noteSearchQuery ? "未找到匹配的笔记" : "该笔记本暂无笔记" }}
              </div>

              <button
                v-for="note in filteredNotes"
                :key="note.id"
                type="button"
                :class="[
                  'w-full text-left p-3 rounded-xl transition-all',
                  currentId === note.id
                    ? 'bg-primary/15 border border-primary/40 text-foreground shadow-xs'
                    : 'border border-transparent hover:bg-secondary/60 text-text-secondary hover:text-foreground',
                ]"
                @click="selectNote(note.id)"
              >
                <div class="truncate text-sm font-semibold text-foreground">
                  {{ note.title || "未命名笔记" }}
                </div>
                <div class="mt-1 flex items-center justify-between text-[11px] text-text-quaternary">
                  <span>{{ fmtTime(note.updated_at) }}</span>
                </div>
                <div v-if="noteTags(note).length" class="mt-2 flex flex-wrap gap-1">
                  <span
                    v-for="t in noteTags(note)"
                    :key="t"
                    class="rounded-full bg-secondary px-2 py-0.5 text-[10px] text-text-tertiary"
                  >
                    #{{ t }}
                  </span>
                </div>
              </button>
            </div>

            <!-- 分页条 -->
            <div
              v-if="totalPages > 1"
              class="flex items-center justify-between border-t border-border px-3 py-2 text-xs text-text-quaternary"
            >
              <button
                type="button"
                :disabled="notesPage <= 1"
                class="rounded p-1 hover:bg-secondary disabled:opacity-30"
                @click="loadNotes(notesPage - 1)"
              >
                <Icon icon="ri:arrow-left-s-line" class="text-base" />
              </button>
              <span>{{ notesPage }} / {{ totalPages }}</span>
              <button
                type="button"
                :disabled="notesPage >= totalPages"
                class="rounded p-1 hover:bg-secondary disabled:opacity-30"
                @click="loadNotes(notesPage + 1)"
              >
                <Icon icon="ri:arrow-right-s-line" class="text-base" />
              </button>
            </div>
          </VCard>

          <!-- 栏 2：右侧沉浸式写作区 -->
          <VCard class="min-w-0 flex-1">
            <div v-if="noteLoading" class="py-24 text-center text-sm text-text-quaternary">
              加载笔记详情中...
            </div>

            <div v-else-if="!hasCurrent" class="flex flex-col items-center justify-center py-24 text-center">
              <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-secondary text-3xl text-text-tertiary">
                <Icon icon="ri:file-edit-line" />
              </div>
              <h3 class="mt-4 text-base font-semibold text-foreground">未选择笔记</h3>
              <p class="mt-1 text-xs text-text-tertiary">
                从左侧列表选择一篇笔记开始编辑，或点击上方「新建笔记」
              </p>
              <VButton type="primary" size="sm" class="mt-4" @click="openCreateNote">
                新建笔记
              </VButton>
            </div>

            <div v-else class="space-y-4">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1.5 block text-xs font-medium text-text-tertiary">笔记标题 *</label>
                  <input v-model="noteForm.title" class="apple-input font-semibold text-sm" placeholder="输入标题..." />
                </div>
                <div>
                  <label class="mb-1.5 block text-xs font-medium text-text-tertiary">标签（逗号分隔）</label>
                  <input
                    v-model="noteForm.tags"
                    class="apple-input text-xs"
                    list="notebook-tags"
                    placeholder="如：技术, 前端, 备忘"
                  />
                  <datalist id="notebook-tags">
                    <option v-for="t in allTags" :key="t" :value="t"></option>
                  </datalist>
                </div>
              </div>

              <!-- 内嵌 Markdown / 富文本编辑器 -->
              <ContentEditor
                ref="editorRef"
                v-model="noteForm.content"
                :storage-key="'note:' + currentId"
                min-height="420px"
                @save="handleSave"
              />

              <!-- 保存/删除底栏 -->
              <div class="flex items-center justify-between border-t border-border/60 pt-3">
                <div class="flex items-center gap-2">
                  <VButton type="primary" :loading="saving" @click="handleSave">
                    <template #icon><Icon icon="ri:save-line" /></template>
                    保存修改
                  </VButton>
                  <VButton type="danger" :loading="deleting" @click="handleDeleteNote">
                    <template #icon><Icon icon="ri:delete-bin-line" /></template>
                    删除笔记
                  </VButton>
                </div>

                <span class="text-xs text-text-quaternary hidden sm:inline-block">
                  支持快捷键 <kbd class="rounded bg-secondary px-1.5 py-0.5 font-mono text-[11px]">⌘+S</kbd> 保存
                </span>
              </div>
            </div>
          </VCard>
        </div>
      </template>
    </div>

    <!-- ============ 新建 / 编辑笔记本弹窗 ============ -->
    <VModal
      v-model:visible="nbModalOpen"
      :title="nbModalMode === 'create' ? '新建笔记本' : '编辑笔记本'"
      :width="500"
    >
      <form class="space-y-4" @submit.prevent="handleSaveNotebook">
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">笔记本名称 *</label>
          <input
            v-model="nbForm.name"
            class="apple-input"
            required
            placeholder="如：Golang 实践 / 读书笔记"
          />
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">笔记本描述</label>
          <textarea
            v-model="nbForm.description"
            class="apple-textarea"
            rows="2"
            placeholder="简要说明此笔记本的归档内容"
          ></textarea>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">封面图片</label>
          <AttachmentField v-model="nbForm.cover" placeholder="图片 URL 或从附件库选择" show-thumb />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="nbModalOpen = false">取消</VButton>
          <VButton type="primary" :loading="nbSaving" @click="handleSaveNotebook">保存</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 新建笔记弹窗 ============ -->
    <VModal v-model:visible="createNoteOpen" title="新建笔记" :width="440">
      <div>
        <label class="mb-1.5 block text-xs font-medium text-text-tertiary">笔记标题 *</label>
        <input
          v-model="createNoteTitle"
          class="apple-input"
          placeholder="例如：关于 Vue3 的性能优化思考"
          autofocus
          @keyup.enter="handleCreateNote"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="createNoteOpen = false">取消</VButton>
          <VButton type="primary" :loading="createNoteSaving" @click="handleCreateNote">创建</VButton>
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

/* 操作小按钮 */
.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

/* 移动端适配 */
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
