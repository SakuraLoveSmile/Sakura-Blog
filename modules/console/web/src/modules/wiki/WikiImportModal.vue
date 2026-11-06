<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, postApi, wikiApi, type WikiDocImportResponse } from "@/api";
import { VButton, VLoading, VModal, Toast, VPagination } from "@/components/ui";
import { formatRelative } from "@/lib/format";
import {
  buildImportPayload,
  importIssueText,
  importSummary,
  postImportState,
  selectableSlugs,
  toggleSelection,
} from "./wiki-import";

// 文库「导入文章」：从「文章」多选复制为当前项目的文档。
// 只读原文章（标题 + Markdown 正文），不改文章本身；同名 Slug 已存在时后端跳过，不覆盖。
interface PostItem {
  slug: string;
  title?: string;
  status: string;
  updated_at?: string;
}

const props = defineProps<{
  open: boolean;
  projectId: string;
  projectTitle: string;
  /** 当前项目已有文档 slug，用于标记「已导入」。 */
  existingSlugs: string[];
  /** 父文档候选（当前项目全部文档）。 */
  docs: { id: number; title: string; slug: string }[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "imported", result: WikiDocImportResponse): void;
}>();

const page = ref(1);
const size = ref(20);
const keyword = ref("");
const posts = ref<PostItem[]>([]);
const total = ref(0);
const loading = ref(false);
const selected = ref<Set<string>>(new Set());
const parent = ref(0);
const status = ref<"draft" | "published">("draft");
const importing = ref(false);
const result = ref<WikiDocImportResponse | null>(null);

const done = computed(() => result.value !== null);
const selectedCount = computed(() => selected.value.size);
const pageSelectable = computed(() => selectableSlugs(posts.value, props.existingSlugs));
const pageAllSelected = computed(
  () => pageSelectable.value.length > 0 && pageSelectable.value.every((slug) => selected.value.has(slug)),
);

let searchTimer: ReturnType<typeof setTimeout> | undefined;

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    reset();
    void loadPosts();
  },
);

watch(keyword, () => {
  if (!props.open || done.value) return;
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    page.value = 1;
    void loadPosts();
  }, 250);
});

watch([page, size], () => {
  if (props.open && !done.value) void loadPosts();
});

onBeforeUnmount(() => clearTimeout(searchTimer));

function reset() {
  clearTimeout(searchTimer);
  page.value = 1;
  size.value = 20;
  keyword.value = "";
  posts.value = [];
  total.value = 0;
  selected.value = new Set();
  parent.value = 0;
  status.value = "draft";
  importing.value = false;
  result.value = null;
}

async function loadPosts() {
  if (!props.projectId) return;
  loading.value = true;
  try {
    const { data } = await postApi.list({
      page: page.value,
      size: size.value,
      search: keyword.value.trim() || undefined,
    });
    posts.value = Array.isArray(data.items) ? (data.items as PostItem[]) : [];
    total.value = Number(data.total) || 0;
  } catch {
    posts.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function postState(post: PostItem) {
  return postImportState(post, props.existingSlugs);
}

function postStatusLabel(post: PostItem): string {
  if (post.status === "trashed") return "回收站";
  return post.status === "published" ? "已发布" : "草稿";
}

function postStatusDot(post: PostItem): string {
  if (post.status === "trashed") return "bg-state-error";
  return post.status === "published" ? "bg-state-success" : "bg-state-warning";
}

function toggle(post: PostItem, checked: boolean) {
  if (postState(post) !== "importable") return;
  selected.value = toggleSelection(selected.value, post.slug, checked);
}

function togglePage() {
  const next = new Set(selected.value);
  if (pageAllSelected.value) {
    for (const slug of pageSelectable.value) next.delete(slug);
  } else {
    for (const slug of pageSelectable.value) next.add(slug);
  }
  selected.value = next;
}

function clearSelection() {
  selected.value = new Set();
}

async function handleImport() {
  const payload = buildImportPayload(selected.value, parent.value, status.value);
  if (!payload.slugs.length || !props.projectId) return;
  importing.value = true;
  try {
    const { data } = await wikiApi.importDocs(props.projectId, payload);
    result.value = data;
    const next = new Set(selected.value);
    for (const item of data.imported) next.delete(item.slug);
    selected.value = next;
    if (data.imported.length) {
      Toast.success("导入完成", { description: importSummary(data) });
    } else {
      Toast.error("没有导入任何文档", { description: importSummary(data) });
    }
    emit("imported", data);
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "导入失败，请稍后重试"));
  } finally {
    importing.value = false;
  }
}
</script>

<template>
  <VModal :visible="open" :title="`导入文章到「${projectTitle}」`" :width="760" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-xs leading-relaxed text-text-quaternary">
        从「文章」中多选并复制到当前文库项目：只复制标题与 Markdown 正文，不改动原文章；同名 Slug
        文档已存在时跳过，不覆盖。
      </p>

      <!-- 结果面板：本批导入后展示，保留失败/跳过原因 -->
      <div v-if="result" class="space-y-3 rounded-xl border border-border bg-secondary/40 p-4">
        <div class="flex items-center gap-2 text-sm font-semibold text-foreground">
          <Icon
            :icon="result.imported.length ? 'ri:checkbox-circle-line' : 'ri:error-warning-line'"
            class="text-base"
            :class="result.imported.length ? 'text-state-success' : 'text-state-warning'"
          />
          {{ importSummary(result) }}
        </div>
        <div v-if="result.skipped.length" class="space-y-1">
          <div class="text-xs font-medium text-text-tertiary">已跳过</div>
          <ul class="space-y-1 text-xs text-text-quaternary">
            <li v-for="item in result.skipped" :key="'skipped-' + item.slug" class="flex gap-1.5">
              <Icon icon="ri:forbid-2-line" class="mt-0.5 shrink-0" />
              <span class="min-w-0 truncate" :title="importIssueText(item)">{{ importIssueText(item) }}</span>
            </li>
          </ul>
        </div>
        <div v-if="result.failed.length" class="space-y-1">
          <div class="text-xs font-medium text-text-tertiary">导入失败</div>
          <ul class="space-y-1 text-xs text-text-quaternary">
            <li v-for="item in result.failed" :key="'failed-' + item.slug" class="flex gap-1.5">
              <Icon icon="ri:error-warning-line" class="mt-0.5 shrink-0" />
              <span class="min-w-0 truncate" :title="importIssueText(item)">{{ importIssueText(item) }}</span>
            </li>
          </ul>
        </div>
      </div>

      <template v-else>
        <!-- 搜索与批量选择 -->
        <div class="space-y-2">
          <div class="apple-search w-full !py-1">
            <Icon icon="ri:search-line" class="text-text-quaternary" />
            <input v-model="keyword" placeholder="搜索文章标题或 Slug..." />
            <button
              v-if="keyword"
              type="button"
              class="text-text-quaternary hover:text-foreground"
              @click="keyword = ''"
            >
              <Icon icon="ri:close-line" />
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <VButton size="sm" :disabled="!pageSelectable.length" @click="togglePage">
              {{ pageAllSelected ? "取消本页" : "全选本页" }}
            </VButton>
            <VButton size="sm" :disabled="!selectedCount" @click="clearSelection">清空选择</VButton>
            <span class="text-xs text-text-quaternary">已选 {{ selectedCount }} 篇</span>
          </div>
        </div>

        <!-- 文章多选列表 -->
        <div class="max-h-[240px] overflow-y-auto rounded-xl border border-border sm:max-h-[340px]">
          <div v-if="loading" class="py-10 text-center text-sm text-text-quaternary"><VLoading /></div>
          <div v-else-if="!posts.length" class="py-10 text-center text-xs text-text-quaternary">没有匹配的文章</div>
          <ul v-else class="divide-y divide-border/60" data-wiki-import-list>
            <li v-for="post in posts" :key="post.slug">
              <label
                class="flex items-center gap-3 px-3 py-2.5 text-sm transition-colors"
                :class="
                  postState(post) === 'importable'
                    ? 'cursor-pointer hover:bg-secondary'
                    : 'cursor-not-allowed opacity-60'
                "
              >
                <input
                  type="checkbox"
                  class="shrink-0 rounded-sm"
                  :checked="selected.has(post.slug)"
                  :disabled="postState(post) !== 'importable'"
                  @change="toggle(post, ($event.target as HTMLInputElement).checked)"
                />
                <span class="min-w-0 flex-1">
                  <span class="block truncate font-medium text-foreground">{{ post.title || post.slug }}</span>
                  <span class="block truncate font-mono text-[11px] text-text-quaternary">{{ post.slug }}</span>
                </span>
                <span
                  v-if="postState(post) !== 'importable'"
                  class="shrink-0 text-[11px] text-text-quaternary"
                  :title="postState(post) === 'imported' ? '当前项目已有同名 Slug 文档' : '回收站文章不能导入'"
                >
                  {{ postState(post) === "imported" ? "已导入" : "回收站" }}
                </span>
                <span v-else class="flex shrink-0 items-center gap-1.5 text-[11px] text-text-quaternary">
                  <span class="h-1.5 w-1.5 rounded-full" :class="postStatusDot(post)"></span>
                  {{ postStatusLabel(post) }}
                </span>
                <span v-if="post.updated_at" class="hidden shrink-0 text-[11px] text-text-quaternary sm:block">
                  {{ formatRelative(post.updated_at) }}
                </span>
              </label>
            </li>
          </ul>
        </div>

        <!-- 导入选项与分页：分页独占一行，避免长父文档名把「共 N 条」挤成竖排 -->
        <div class="space-y-3">
          <div class="flex flex-wrap items-end gap-3">
            <label class="text-xs text-text-tertiary">
              <span class="mb-1 block">归属父文档</span>
              <select v-model.number="parent" class="apple-select">
                <option :value="0">无（根目录）</option>
                <option v-for="doc in docs" :key="doc.id" :value="doc.id">{{ doc.title }}</option>
              </select>
            </label>
            <label class="text-xs text-text-tertiary">
              <span class="mb-1 block">导入后状态</span>
              <select v-model="status" class="apple-select">
                <option value="draft">草稿（不公开）</option>
                <option value="published">已发布</option>
              </select>
            </label>
          </div>
          <div class="flex justify-end">
            <VPagination v-model:page="page" v-model:size="size" :total="total" />
          </div>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-2">
        <span class="truncate text-xs text-text-quaternary">
          {{ done ? `${result?.imported.length ?? 0} 篇已写入当前项目` : "导入不会自动发布或覆盖已有文档" }}
        </span>
        <div class="flex shrink-0 gap-2">
          <template v-if="!done">
            <VButton @click="emit('close')">取消</VButton>
            <VButton type="primary" :loading="importing" :disabled="!selectedCount" @click="handleImport">
              导入 {{ selectedCount }} 篇
            </VButton>
          </template>
          <VButton v-else type="primary" @click="emit('close')">完成</VButton>
        </div>
      </div>
    </template>
  </VModal>
</template>
