<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { notebookApi } from "@/api";
import ContentEditor from "@/components/ContentEditor.vue";
import { Icon } from "@iconify/vue";
import { VCard, VButton, Toast, Dialog, VModal } from "@/components/ui";

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

const notebooks = ref<Notebook[]>([]);
const noteCounts = ref<Record<number, number>>({});
const loading = ref(false);

async function loadNotebooks() {
  loading.value = true;
  try {
    const { data } = await notebookApi.listNotebooks();
    notebooks.value = data.items || [];
    // Spec 62：note_count 由后端单条 GROUP BY 查询随列表返回（阶段 1），
    // 替代每笔记本一次 listNotes 计数的 N+1（20 笔记本 = 21 请求 → 1 请求）
    noteCounts.value = Object.fromEntries(
      (data.items || []).map((nb: any) => [nb.id, nb.note_count || 0])
    );
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载笔记本失败");
  } finally {
    loading.value = false;
  }
}

const nbModalOpen = ref(false);
const nbModalMode = ref<"create" | "edit">("create");
const nbEditingId = ref(0);
const nbForm = ref({ name: "", description: "", cover: "" });
const nbSaving = ref(false);

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
    if (nbModalMode.value === "create") {
      await notebookApi.createNotebook(payload);
      Toast.success("笔记本已创建");
    } else {
      await notebookApi.updateNotebook(nbEditingId.value, payload);
      Toast.success("已保存");
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
    title: "确定要删除该笔记本吗？",
    description: `「${nb.name}」及其全部笔记将一并级联删除，不可恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await notebookApi.deleteNotebook(nb.id);
        Toast.success("已删除");
        await loadNotebooks();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

const activeNotebook = ref<Notebook | null>(null);

function enterNotebook(nb: Notebook) {
  activeNotebook.value = nb;
  resetNoteEditor();
  loadNotes(1);
}

function backToNotebooks() {
  activeNotebook.value = null;
  loadNotebooks();
}

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
    Toast.error(e?.response?.data?.error || "加载笔记失败");
  } finally {
    notesLoading.value = false;
  }
}

function noteTags(note: Note): string[] {
  return (note.tags || "").split(",").map((s) => s.trim()).filter(Boolean);
}

function fmtTime(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

const currentId = ref(0);
const noteForm = ref({ title: "", tags: "", content: "" });
const noteLoading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const editorRef = ref<InstanceType<typeof ContentEditor> | null>(null);

const hasCurrent = computed(() => currentId.value > 0);

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
  noteForm.value = { title: "", tags: "", content: "" };
}

async function selectNote(id: number) {
  noteLoading.value = true;
  try {
    const { data } = await notebookApi.getNote(id);
    currentId.value = data.id;
    noteForm.value = {
      title: data.title || "",
      tags: data.tags || "",
      content: data.content || "",
    };
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载笔记失败");
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
    await loadNotes(notesPage.value);
    await loadTags();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

const createNoteOpen = ref(false);
const createNoteTitle = ref("");
const createNoteSaving = ref(false);

function openCreateNote() {
  createNoteTitle.value = "";
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

function handleDeleteNote() {
  Dialog.warning({
    title: "确定要删除该笔记吗？",
    description: `「${noteForm.value.title || "未命名笔记"}」删除后不可恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      if (!hasCurrent.value) return;
      deleting.value = true;
      try {
        await notebookApi.deleteNote(currentId.value);
        Toast.success("已删除");
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

const inputCls = "apple-input";
const labelCls = "block text-xs text-text-tertiary mb-1";

const searchQuery = ref("");
const viewMode = ref<"grid" | "list">("grid");
const menuOpenId = ref<number | null>(null);

function notebookCoverClass(id: number): string {
  const colors = ["blue", "purple", "green", "orange", "pink", "teal"];
  const key = String(id);
  let hash = 0;
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0;
  }
  return `notebook-cover-${colors[hash % colors.length]}`;
}

const filteredNotebooks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return notebooks.value;
  return notebooks.value.filter((nb) => nb.name.toLowerCase().includes(q));
});

function toggleMenu(id: number, event: MouseEvent) {
  event.stopPropagation();
  menuOpenId.value = menuOpenId.value === id ? null : id;
}

function closeMenu() {
  menuOpenId.value = null;
}

onMounted(() => {
  loadNotebooks();
  loadTags();
});
</script>

<template>
  <div>
    <header
      class="sticky top-0 z-20 flex items-center justify-between px-10 pb-5 pt-7 backdrop-blur-xl"
      style="background: linear-gradient(180deg, color-mix(in srgb, #000 90%, transparent) 0%, color-mix(in srgb, #000 60%, transparent) 80%, transparent 100%);"
    >
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-3">
          <button
            v-if="activeNotebook"
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-md border border-border bg-card text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
            @click="backToNotebooks"
          >
            <Icon icon="ri:arrow-left-line" class="text-base" />
          </button>
          <div class="min-w-0">
            <h1
              class="truncate text-[28px] font-bold leading-tight tracking-[-0.8px] text-foreground"
            >
              {{ activeNotebook ? activeNotebook.name : "笔记本" }}
            </h1>
            <p class="mt-1 text-[13px] text-text-tertiary">
              {{ activeNotebook ? "管理该笔记本下的全部笔记" : "管理你的所有笔记本文档" }}
            </p>
          </div>
        </div>
      </div>

      <div v-if="!activeNotebook" class="flex shrink-0 items-center gap-2.5">
        <div class="apple-search w-60 rounded-md border-border bg-card">
          <Icon icon="ri:search-line" class="text-base text-text-quaternary" />
          <input v-model="searchQuery" type="text" placeholder="搜索笔记本..." />
        </div>

        <div class="flex gap-0.5 rounded-md border border-border bg-card p-0.5">
          <button
            type="button"
            title="网格视图"
            :class="[
              'flex h-[30px] w-8 items-center justify-center rounded-[7px] text-text-quaternary transition-colors',
              viewMode === 'grid' ? 'bg-accent text-foreground' : 'hover:text-foreground',
            ]"
            @click="viewMode = 'grid'"
          >
            <Icon icon="ri:grid-line" class="text-base" />
          </button>
          <button
            type="button"
            title="列表视图"
            :class="[
              'flex h-[30px] w-8 items-center justify-center rounded-[7px] text-text-quaternary transition-colors',
              viewMode === 'list' ? 'bg-accent text-foreground' : 'hover:text-foreground',
            ]"
            @click="viewMode = 'list'"
          >
            <Icon icon="ri:list-check" class="text-base" />
          </button>
        </div>

        <VButton type="primary" @click="openCreateNotebook">
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          新建笔记本
        </VButton>
      </div>

      <div v-else class="flex shrink-0 items-center gap-2">
        <VButton type="secondary" @click="openCreateNote">
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          新建笔记
        </VButton>
      </div>
    </header>

    <div class="px-10 pb-10 pt-2">
      <div v-if="!activeNotebook">
        <div v-if="loading" class="py-16 text-center text-sm text-text-quaternary">
          加载中...
        </div>

        <div
          v-else-if="!filteredNotebooks.length"
          class="rounded-lg border border-border bg-card py-16 text-center"
        >
          <Icon icon="ri:book-open-line" class="mx-auto text-3xl text-text-quaternary" />
          <p class="mt-3 text-sm text-text-tertiary">
            {{ searchQuery ? "未找到匹配的笔记本" : "暂无笔记本" }}
          </p>
          <p v-if="!searchQuery" class="mt-1 text-xs text-text-quaternary">
            点击「新建笔记本」创建，前台 /notebooks 即时可见。
          </p>
        </div>

        <div
          v-else
          :class="[
            'grid gap-5',
            viewMode === 'grid'
              ? 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3'
              : 'grid-cols-1',
          ]"
        >
          <div
            v-for="nb in filteredNotebooks"
            :key="nb.id"
            :class="[
              'group relative overflow-hidden rounded-[14px] border border-border bg-card transition-all duration-250 ease-standard hover:-translate-y-1 hover:border-border-strong hover:shadow-lg',
              viewMode === 'list' ? 'flex' : 'flex flex-col',
            ]"
          >
            <button
              type="button"
              :class="[
                'relative overflow-hidden',
                viewMode === 'list' ? 'h-[120px] w-56 shrink-0' : 'h-[120px] w-full',
                notebookCoverClass(nb.id),
              ]"
              @click="enterNotebook(nb)"
            >
              <div class="pointer-events-none absolute inset-0 z-[1]">
                <div
                  class="absolute left-[20%] top-[15%] h-24 w-24 rounded-full bg-white/10 blur-2xl"
                />
                <div
                  class="absolute bottom-[-30%] right-[-10%] h-28 w-28 rounded-full bg-white/10 blur-2xl"
                />
              </div>
              <div class="absolute inset-0 bg-gradient-to-b from-transparent to-black/15" />

              <div
                class="absolute left-[18px] top-[18px] z-[2] flex h-11 w-11 items-center justify-center rounded-xl border border-white/15 bg-white/20 text-foreground backdrop-blur-md"
              >
                <Icon icon="ri:book-open-line" class="text-[22px]" />
              </div>

              <span
                class="absolute bottom-3.5 right-4 z-[2] rounded-full bg-background/30 px-2.5 py-1 text-[11px] font-medium text-foreground/90 backdrop-blur-md"
              >
                {{ noteCounts[nb.id] || 0 }} 篇笔记
              </span>
            </button>

            <button
              type="button"
              title="更多操作"
              class="absolute right-3.5 top-3.5 z-[3] flex h-[30px] w-[30px] items-center justify-center rounded-lg bg-background/25 text-foreground/80 backdrop-blur-md transition-colors hover:bg-background/40 hover:text-foreground"
              @click="toggleMenu(nb.id, $event)"
            >
              <Icon icon="ri:more-2-fill" class="text-base" />
            </button>

            <div
              v-if="menuOpenId === nb.id"
              class="absolute right-3 top-12 z-20 w-32 overflow-hidden rounded-lg border border-border bg-popover py-1 shadow-lg"
              @click.stop
            >
              <button
                type="button"
                class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-text-secondary transition-colors hover:bg-accent hover:text-foreground"
                @click="openEditNotebook(nb); closeMenu()"
              >
                <Icon icon="ri:edit-line" />
                编辑
              </button>
              <button
                type="button"
                class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-state-error transition-colors hover:bg-state-error/10"
                @click="handleDeleteNotebook(nb); closeMenu()"
              >
                <Icon icon="ri:delete-bin-line" />
                删除
              </button>
            </div>

            <button
              type="button"
              :class="[
                'flex min-w-0 flex-1 text-left',
                viewMode === 'list' ? 'flex-row items-center justify-between gap-4 px-5' : 'flex-col p-4',
              ]"
              @click="enterNotebook(nb)"
            >
              <div class="min-w-0 flex-1">
                <div class="truncate text-[15px] font-semibold leading-snug text-foreground">
                  {{ nb.name }}
                </div>
                <p
                  v-if="nb.description"
                  :class="[
                    'mt-1.5 text-[13px] leading-5 text-text-tertiary',
                    viewMode === 'list' ? 'line-clamp-1' : 'line-clamp-2',
                  ]"
                >
                  {{ nb.description }}
                </p>
              </div>
              <div
                :class="[
                  'flex items-center gap-1.5 whitespace-nowrap text-[13px] text-text-tertiary',
                  viewMode === 'list' ? '' : 'mt-2',
                ]"
              >
                <Icon icon="ri:time-line" class="text-sm" />
                <span>{{ fmtTime(nb.created_at) || "刚刚创建" }}</span>
              </div>
            </button>
          </div>
        </div>
      </div>

      <div v-else class="flex flex-col gap-4 lg:flex-row">
        <VCard title="笔记列表" :bodyClass="['!p-0']" class="lg:w-[300px] shrink-0">
          <template #actions>
            <span class="text-xs text-text-quaternary">{{ notesTotal }} 篇</span>
          </template>
          <div class="max-h-[70vh] overflow-y-auto p-2">
            <div v-if="notesLoading" class="py-8 text-center text-xs text-text-quaternary">
              加载中...
            </div>
            <div
              v-else-if="!notes.length"
              class="py-8 text-center text-xs text-text-quaternary"
            >
              暂无笔记<br />点击「新建笔记」创建
            </div>
            <div v-else class="space-y-1">
              <button
                v-for="note in notes"
                :key="note.id"
                type="button"
                @click="selectNote(note.id)"
                :class="[
                  'block w-full rounded-lg px-3 py-2.5 text-left transition-colors',
                  currentId === note.id
                    ? 'bg-primary/20 text-primary'
                    : 'text-text-secondary hover:bg-accent',
                ]"
              >
                <span class="block truncate text-sm font-semibold">{{ note.title || "未命名笔记" }}</span>
                <span class="mt-1 flex items-center gap-1 text-[11px] text-text-quaternary">
                  <Icon icon="ri:time-line" />
                  {{ fmtTime(note.updated_at) }}
                </span>
                <span v-if="noteTags(note).length" class="mt-2 flex flex-wrap gap-1">
                  <span
                    v-for="t in noteTags(note)"
                    :key="t"
                    class="rounded-full bg-secondary px-2 py-px text-[10px] text-text-tertiary"
                  >{{ t }}</span>
                </span>
              </button>
            </div>
          </div>
          <div
            v-if="totalPages > 1"
            class="flex items-center justify-between border-t border-border px-3 py-2"
          >
            <button
              type="button"
              :disabled="notesPage <= 1"
              class="flex h-7 w-7 items-center justify-center rounded-md text-text-quaternary transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent"
              @click="loadNotes(notesPage - 1)"
            >
              <Icon icon="ri:arrow-left-s-line" class="text-base" />
            </button>
            <span class="text-xs text-text-quaternary">{{ notesPage }} / {{ totalPages }}</span>
            <button
              type="button"
              :disabled="notesPage >= totalPages"
              class="flex h-7 w-7 items-center justify-center rounded-md text-text-quaternary transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-30 disabled:hover:bg-transparent"
              @click="loadNotes(notesPage + 1)"
            >
              <Icon icon="ri:arrow-right-s-line" class="text-base" />
            </button>
          </div>
        </VCard>

        <VCard class="min-w-0 flex-1">
          <template #actions>
            <VButton type="secondary" size="sm" @click="openCreateNote">
              <template #icon>
                <Icon icon="ri:add-line" class="h-full w-full" />
              </template>
              新建笔记
            </VButton>
          </template>

          <div v-if="noteLoading" class="py-16 text-center text-sm text-text-quaternary">
            加载中...
          </div>

          <div v-else-if="!hasCurrent" class="py-16 text-center">
            <div
              class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-secondary text-text-quaternary"
            >
              <Icon icon="ri:file-edit-line" class="text-3xl" />
            </div>
            <p class="mt-4 text-sm text-text-tertiary">
              选择左侧笔记开始编辑，或点击「新建笔记」
            </p>
          </div>

          <div v-else class="space-y-4">
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

            <!-- 内容编辑器（富文本/Markdown 双模式） -->
            <ContentEditor
              ref="editorRef"
              v-model="noteForm.content"
              :storage-key="'note:' + currentId"
              min-height="400px"
              @save="handleSave"
            />

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

    <div v-if="menuOpenId !== null" class="fixed inset-0 z-10" @click="closeMenu" @scroll="closeMenu" />

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

    <VModal
      :visible="createNoteOpen"
      title="新建笔记"
      :width="440"
      @update:visible="createNoteOpen = $event"
      @close="createNoteOpen = false"
    >
      <div>
        <label :class="labelCls">标题</label>
        <input
          v-model="createNoteTitle"
          :class="inputCls"
          placeholder="笔记标题"
          autofocus
          @keyup.enter="handleCreateNote"
        />
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
