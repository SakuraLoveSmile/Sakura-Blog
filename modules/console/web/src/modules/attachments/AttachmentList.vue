<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { attachmentApi, attachmentGroupApi } from "@/api";
import { formatRelative, formatBytes } from "@/lib/format";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  VModal,
  VFloatingDock,
  Dialog,
  Toast,
} from "@/components/ui";
import ConsoleStatsCard from "@/components/ConsoleStatsCard.vue";

interface AttachmentItem {
  id: number;
  filename: string;
  url: string;
  mime: string;
  size: number;
  group_id?: number;
  created_at: string;
  width?: number;
  height?: number;
}

interface GroupItem {
  id: number;
  name: string;
  order: number;
  count: number;
}

interface AttachmentRefs {
  posts?: { title: string; slug: string }[];
  pages?: { title: string; slug: string }[];
  moments?: { id: number }[];
}

const queryClient = useQueryClient();

// 搜索关键词（debounce 300ms）
const keyword = ref("");
const debouncedKeyword = ref("");
const page = ref(1);
const size = ref(24);
const uploading = ref(false);
const importingURL = ref(false);
const importURLVisible = ref(false);
const importURL = ref("");
const viewMode = ref<"grid" | "table">("grid");

// 类型分段筛选
type TypeFilter = "all" | "image" | "document" | "video" | "audio" | "other";
const typeFilter = ref<TypeFilter>("all");
const typeFilters: { key: TypeFilter; label: string; icon: string }[] = [
  { key: "all", label: "全部", icon: "ri:apps-line" },
  { key: "image", label: "图片", icon: "ri:image-line" },
  { key: "document", label: "文档", icon: "ri:file-text-line" },
  { key: "video", label: "视频", icon: "ri:video-line" },
  { key: "audio", label: "音频", icon: "ri:music-line" },
  { key: "other", label: "其他", icon: "ri:more-line" },
];

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debouncedKeyword.value = keyword.value;
    page.value = 1;
  }, 300);
});

// --- 分组管理 ---
const activeGroup = ref<number | undefined>(undefined);
const groups = ref<GroupItem[]>([]);

async function loadGroups() {
  try {
    const { data } = await attachmentGroupApi.list();
    groups.value = data.items || [];
  } catch {}
}

function selectGroup(g: number | undefined) {
  activeGroup.value = g;
  page.value = 1;
  clearSelection();
}

const groupModalVisible = ref(false);
const groupModalMode = ref<"create" | "rename">("create");
const groupModalName = ref("");
const groupModalTarget = ref<GroupItem | null>(null);
const groupSaving = ref(false);

function openCreateGroup() {
  groupModalMode.value = "create";
  groupModalName.value = "";
  groupModalTarget.value = null;
  groupModalVisible.value = true;
}

function openRenameGroup(g: GroupItem) {
  groupModalMode.value = "rename";
  groupModalName.value = g.name;
  groupModalTarget.value = g;
  groupModalVisible.value = true;
}

async function submitGroup() {
  const name = groupModalName.value.trim();
  if (!name) {
    Toast.warning("请输入分组名称");
    return;
  }
  groupSaving.value = true;
  try {
    if (groupModalMode.value === "create") {
      await attachmentGroupApi.create({ name });
      Toast.success("分组已创建");
    } else if (groupModalTarget.value) {
      await attachmentGroupApi.update(groupModalTarget.value.id, { name });
      Toast.success("分组已重命名");
    }
    groupModalVisible.value = false;
    await loadGroups();
  } catch {
  } finally {
    groupSaving.value = false;
  }
}

function handleDeleteGroup(g: GroupItem) {
  Dialog.warning({
    title: `确定删除分组「${g.name}」吗？`,
    description: "该分组下的附件将自动移至「未分组」，文件本身不会被删除。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await attachmentGroupApi.delete(g.id);
        if (activeGroup.value === g.id) activeGroup.value = undefined;
        await loadGroups();
        queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
        Toast.success("分组已删除");
      } catch {}
    },
  });
}

// --- 数据查询 ---
const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (debouncedKeyword.value.trim()) params.keyword = debouncedKeyword.value.trim();
  if (typeFilter.value === "image") params.kind = "image";
  else if (typeFilter.value !== "all") params.kind = "other";
  if (activeGroup.value !== undefined) params.group = activeGroup.value;
  return params;
});

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
  queryKey: computed(() => queryKeys.attachments(queryParams.value)),
  queryFn: () => attachmentApi.list(queryParams.value).then(({ data }) => data),
});

const rawAttachments = computed<AttachmentItem[]>(() => (queryData.value?.items || []) as AttachmentItem[]);
const total = computed(() => queryData.value?.total || 0);

function categoryOf(mime: string): TypeFilter {
  if (!mime) return "other";
  if (mime.startsWith("image/")) return "image";
  if (mime.startsWith("video/")) return "video";
  if (mime.startsWith("audio/")) return "audio";
  if (
    mime.startsWith("text/") ||
    mime.includes("pdf") ||
    mime.includes("word") ||
    mime.includes("excel") ||
    mime.includes("presentation") ||
    mime.includes("markdown") ||
    mime.includes("json") ||
    mime.includes("xml")
  ) {
    return "document";
  }
  return "other";
}

const attachments = computed(() => {
  if (typeFilter.value === "all" || typeFilter.value === "image") return rawAttachments.value;
  return rawAttachments.value.filter((a) => categoryOf(a.mime) === typeFilter.value);
});

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = rawAttachments.value;
  let totalBytes = 0;
  let imgCount = 0;
  let mediaCount = 0;
  for (const a of all) {
    totalBytes += a.size || 0;
    const cat = categoryOf(a.mime);
    if (cat === "image") imgCount++;
    else if (cat === "video" || cat === "audio" || cat === "document") mediaCount++;
  }
  return {
    total: total.value,
    totalBytes: formatBytes(totalBytes),
    imgCount,
    mediaCount,
  };
});

// --- 上传管理 ---
const fileInput = ref<HTMLInputElement | null>(null);

async function handleUploadFiles(files: FileList | File[]) {
  if (!files.length) return;
  uploading.value = true;
  let okCount = 0;
  try {
    const gid = activeGroup.value && activeGroup.value > 0 ? activeGroup.value : undefined;
    for (const file of Array.from(files)) {
      await attachmentApi.upload(file, gid);
      okCount++;
    }
    Toast.success(`成功上传 ${okCount} 个文件`);
    await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
    await loadGroups();
    await refetch();
  } catch {
  } finally {
    uploading.value = false;
    if (fileInput.value) fileInput.value.value = "";
  }
}

async function submitURLImport() {
  const url = importURL.value.trim();
  if (!/^https?:\/\//i.test(url)) {
    Toast.warning("请输入 http:// 或 https:// 图片 URL");
    return;
  }
  importingURL.value = true;
  try {
    const gid = activeGroup.value && activeGroup.value > 0 ? activeGroup.value : undefined;
    await attachmentApi.importURL(url, gid);
    Toast.success("URL 图片已导入");
    importURL.value = "";
    importURLVisible.value = false;
    await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
    await loadGroups();
    await refetch();
  } catch {
  } finally {
    importingURL.value = false;
  }
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  if (input.files?.length) {
    handleUploadFiles(input.files);
  }
}

onMounted(() => {
  loadGroups();
});

// --- 批量选择 ---
const selectedIds = ref<Set<number>>(new Set());
const batchMoving = ref(false);
const batchTargetGroup = ref<number | "">("");

function toggleSelect(id: number) {
  if (selectedIds.value.has(id)) selectedIds.value.delete(id);
  else selectedIds.value.add(id);
}

function selectAll() {
  for (const a of attachments.value) selectedIds.value.add(a.id);
}

function clearSelection() {
  selectedIds.value.clear();
  batchTargetGroup.value = "";
}

async function batchDelete() {
  const ids = Array.from(selectedIds.value);
  if (ids.length === 0) return;
  Dialog.warning({
    title: `确定批量删除选中的 ${ids.length} 个附件吗？`,
    description: "删除后磁盘中的实体文件将被清理，不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await Promise.all(ids.map((id) => attachmentApi.delete(id)));
        clearSelection();
        await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
        await loadGroups();
        await refetch();
        Toast.success("批量删除成功");
      } catch {}
    },
  });
}

async function batchMove() {
  if (batchTargetGroup.value === "") {
    Toast.warning("请选择目标分组");
    return;
  }
  const ids = Array.from(selectedIds.value);
  batchMoving.value = true;
  try {
    await attachmentApi.move(ids, Number(batchTargetGroup.value));
    clearSelection();
    await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
    await loadGroups();
    await refetch();
    Toast.success(`已将 ${ids.length} 个附件移至新分组`);
  } catch {
  } finally {
    batchMoving.value = false;
  }
}

// --- 详情抽屉 ---
const detailItem = ref<AttachmentItem | null>(null);
const detailRefs = ref<AttachmentRefs | null>(null);
const detailRefsLoading = ref(false);
const detailGroupID = ref(0);
const detailGroupSaving = ref(false);
const detailGroupChanged = computed(
  () => detailItem.value !== null && detailGroupID.value !== (detailItem.value.group_id ?? 0),
);

async function openDetail(item: AttachmentItem) {
  detailItem.value = item;
  detailGroupID.value = item.group_id ?? 0;
  detailRefs.value = null;
  detailRefsLoading.value = true;
  try {
    const { data } = await attachmentApi.refs(item.id);
    detailRefs.value = data;
  } catch {
  } finally {
    detailRefsLoading.value = false;
  }
}

function closeDetail() {
  detailItem.value = null;
}

async function saveDetailGroup() {
  const item = detailItem.value;
  if (!item || !detailGroupChanged.value) return;

  detailGroupSaving.value = true;
  try {
    await attachmentApi.move([item.id], detailGroupID.value);
    detailItem.value = { ...item, group_id: detailGroupID.value };
    await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
    await loadGroups();
    await refetch();
    Toast.success("附件分组已更新");
  } catch {
  } finally {
    detailGroupSaving.value = false;
  }
}

function copyText(text: string, msg = "已复制到剪贴板") {
  navigator.clipboard.writeText(text).then(() => Toast.success(msg));
}

function handleDeleteSingle(item: AttachmentItem) {
  Dialog.warning({
    title: `确定删除附件「${item.filename}」吗？`,
    description: "删除后不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await attachmentApi.delete(item.id);
        selectedIds.value.delete(item.id);
        if (detailItem.value?.id === item.id) closeDetail();
        await queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
        await loadGroups();
        await refetch();
        Toast.success("已删除");
      } catch {}
    },
  });
}

function formatMimeBadge(mime: string): string {
  if (!mime) return "FILE";
  const sub = mime.split("/")[1] || mime;
  return sub.toUpperCase().slice(0, 5);
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="媒体库" actions-layout="stack-below-lg">
      <template #icon>
        <Icon icon="ri:image-line" class="text-primary" />
      </template>
      <template #subtitle>管理博客全部图片、音视频与文档附件，支持分组合辑与快速直链</template>
      <template #actions>
        <div class="flex w-full flex-col gap-2.5 lg:w-auto lg:flex-row lg:items-center">
          <div class="apple-search w-full lg:min-w-[200px] lg:max-w-[280px]">
            <Icon icon="ri:search-line" class="text-text-quaternary" />
            <input v-model="keyword" placeholder="搜索文件名或 URL..." />
            <button v-if="keyword" type="button" class="text-text-quaternary hover:text-foreground" @click="keyword = ''">
              <Icon icon="ri:close-line" />
            </button>
          </div>

          <div class="flex w-full flex-wrap items-center gap-2 lg:w-auto lg:flex-nowrap sm:gap-2.5">
            <VButton @click="openCreateGroup">
              <template #icon><Icon icon="ri:folder-add-line" /></template>
              新建分组
            </VButton>

            <input ref="fileInput" type="file" multiple class="hidden" @change="onFileChange" />

            <VButton type="primary" :loading="uploading" @click="fileInput?.click()">
              <template #icon><Icon icon="ri:upload-cloud-2-line" class="h-full w-full" /></template>
              上传附件
            </VButton>
            <VButton :disabled="uploading || importingURL" @click="importURLVisible = true">
              <template #icon><Icon icon="ri:link-m" /></template>
              从 URL 导入
            </VButton>
          </div>
        </div>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <ConsoleStatsCard
          label="全部附件"
          :value="stats.total"
          sub="个实体文件"
          icon="ri:attachment-line"
          color="primary"
        />
        <ConsoleStatsCard
          label="当前列表占用"
          :value="stats.totalBytes"
          sub="磁盘已用空间"
          icon="ri:hard-drive-2-line"
          color="success"
          value-class="text-[#30D158]"
        />
        <ConsoleStatsCard
          label="图片素材"
          :value="stats.imgCount"
          sub="张配图/封面"
          icon="ri:image-line"
          color="cyan"
        />
        <ConsoleStatsCard
          label="分组合辑"
          :value="groups.length"
          sub="个自定义分组"
          icon="ri:folder-zip-line"
          color="purple"
        />
      </div>

      <!-- 主工作区：左侧分组侧栏 + 右侧瀑布流/表格 -->
      <div class="flex flex-col gap-5 lg:flex-row lg:items-start">
        <!-- 左侧分组侧边栏 -->
        <VCard class="w-full lg:w-[260px] shrink-0" :bodyClass="['!p-2']">
          <template #header>
            <div class="flex items-center justify-between w-full">
              <span class="text-xs font-bold uppercase tracking-wider text-text-tertiary">附件分组</span>
              <button type="button" class="text-xs text-primary hover:underline" @click="openCreateGroup">
                + 新建
              </button>
            </div>
          </template>

          <div class="space-y-1">
            <button
              type="button"
              class="group-item-btn"
              :class="{ active: activeGroup === undefined }"
              @click="selectGroup(undefined)"
            >
              <div class="flex items-center gap-2 truncate">
                <Icon icon="ri:folder-line" />
                <span class="truncate">全部附件</span>
              </div>
              <span class="group-count-badge">{{ total }}</span>
            </button>

            <button type="button" class="group-item-btn" :class="{ active: activeGroup === 0 }" @click="selectGroup(0)">
              <div class="flex items-center gap-2 truncate">
                <Icon icon="ri:folder-unknow-line" />
                <span class="truncate">未分组</span>
              </div>
            </button>

            <div class="my-1.5 h-px bg-border/60"></div>

            <div
              v-for="g in groups"
              :key="g.id"
              class="group-item-btn group"
              :class="{ active: activeGroup === g.id }"
              @click="selectGroup(g.id)"
            >
              <div class="flex items-center gap-2 truncate flex-1 min-w-0">
                <Icon icon="ri:folder-2-line" class="text-primary" />
                <span class="truncate">{{ g.name }}</span>
              </div>
              <div class="flex items-center gap-1">
                <span class="group-count-badge">{{ g.count || 0 }}</span>
                <button
                  type="button"
                  class="hidden p-0.5 text-text-quaternary hover:text-foreground group-hover:inline-block"
                  title="重命名"
                  @click.stop="openRenameGroup(g)"
                >
                  <Icon icon="ri:edit-line" />
                </button>
                <button
                  type="button"
                  class="hidden p-0.5 text-text-quaternary hover:text-state-error group-hover:inline-block"
                  title="删除分组"
                  @click.stop="handleDeleteGroup(g)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>
              </div>
            </div>
          </div>
        </VCard>

        <!-- 右侧主内容区 -->
        <div class="min-w-0 flex-1 space-y-4">
          <!-- 工具栏：类型分段 + 视图模式 -->
          <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-card p-3">
            <!-- 类型筛选 -->
            <div class="apple-segmented max-w-full flex-wrap">
              <button
                v-for="tf in typeFilters"
                :key="tf.key"
                type="button"
                :class="{ active: typeFilter === tf.key }"
                @click="
                  typeFilter = tf.key;
                  page = 1;
                "
              >
                <Icon :icon="tf.icon" class="mr-1 inline-block" />
                {{ tf.label }}
              </button>
            </div>

            <!-- 视图与全选 -->
            <div class="flex items-center gap-2">
              <button
                v-if="attachments.length > 0"
                type="button"
                class="apple-filter-btn rounded-lg border border-border px-2.5 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
                @click="selectedIds.size === attachments.length ? clearSelection() : selectAll()"
              >
                {{ selectedIds.size === attachments.length ? "取消全选" : "全选本页" }}
              </button>

              <div class="apple-segmented">
                <button
                  type="button"
                  :class="{ active: viewMode === 'grid' }"
                  title="网格视图"
                  @click="viewMode = 'grid'"
                >
                  <Icon icon="ri:apps-line" />
                </button>
                <button
                  type="button"
                  :class="{ active: viewMode === 'table' }"
                  title="表格视图"
                  @click="viewMode = 'table'"
                >
                  <Icon icon="ri:list-check" />
                </button>
              </div>
            </div>
          </div>

          <VLoading v-if="isLoading" />

          <VEmpty
            v-else-if="attachments.length === 0"
            :title="keyword ? '未找到匹配的附件' : '暂无附件'"
            :message="keyword ? '换个搜索关键词试试' : '点击右上角「上传附件」选择文件'"
          />

          <!-- ============ 模式 1：网格卡片视图 ============ -->
          <div
            v-else-if="viewMode === 'grid'"
            class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6"
          >
            <div
              v-for="a in attachments"
              :key="a.id"
              class="attachment-card group relative flex flex-col justify-between overflow-hidden rounded-xl border border-border bg-card transition-all hover:border-border-strong hover:shadow-sm"
              :class="{ 'border-primary ring-1 ring-primary': selectedIds.has(a.id) }"
              @click="openDetail(a)"
            >
              <!-- 缩略图容器 -->
              <div class="relative aspect-square w-full overflow-hidden bg-secondary">
                <img
                  v-if="a.mime && a.mime.startsWith('image/')"
                  :src="a.url"
                  loading="lazy"
                  class="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
                  :alt="a.filename"
                />
                <div v-else class="flex h-full w-full flex-col items-center justify-center text-3xl text-text-tertiary">
                  <Icon
                    :icon="
                      categoryOf(a.mime) === 'video'
                        ? 'ri:video-line'
                        : categoryOf(a.mime) === 'audio'
                          ? 'ri:music-line'
                          : 'ri:file-text-line'
                    "
                  />
                </div>

                <!-- 格式角标 -->
                <span
                  class="absolute bottom-1.5 left-1.5 rounded bg-black/60 px-1.5 py-0.5 font-mono text-[9px] font-bold text-white/90 backdrop-blur-md"
                >
                  {{ formatMimeBadge(a.mime) }}
                </span>

                <!-- 多选框 -->
                <div class="absolute left-2 top-2 z-10" @click.stop>
                  <input
                    type="checkbox"
                    :checked="selectedIds.has(a.id)"
                    class="h-4 w-4 rounded border-border text-primary"
                    @change="toggleSelect(a.id)"
                  />
                </div>

                <!-- 悬浮操作按钮组 -->
                <div
                  class="absolute inset-0 flex items-center justify-center gap-1.5 bg-black/50 opacity-0 backdrop-blur-xs transition-opacity group-hover:opacity-100"
                  @click.stop
                >
                  <button
                    type="button"
                    class="rounded-full bg-white/20 p-2 text-white hover:bg-primary"
                    title="复制 Markdown"
                    @click.stop="copyText(`![${a.filename}](${a.url})`, '已复制 Markdown 语法')"
                  >
                    <Icon icon="ri:markdown-line" class="text-sm" />
                  </button>
                  <button
                    type="button"
                    class="rounded-full bg-white/20 p-2 text-white hover:bg-primary"
                    title="复制直链 URL"
                    @click.stop="copyText(a.url, '已复制直链 URL')"
                  >
                    <Icon icon="ri:link" class="text-sm" />
                  </button>
                  <button
                    type="button"
                    class="rounded-full bg-white/20 p-2 text-white hover:bg-state-error"
                    title="删除"
                    @click.stop="handleDeleteSingle(a)"
                  >
                    <Icon icon="ri:delete-bin-line" class="text-sm" />
                  </button>
                </div>
              </div>

              <!-- 底部信息 -->
              <div class="p-2.5">
                <div class="truncate text-xs font-semibold text-foreground" :title="a.filename">
                  {{ a.filename }}
                </div>
                <div class="mt-1 flex items-center justify-between text-[11px] text-text-quaternary">
                  <span>{{ formatBytes(a.size) }}</span>
                  <span>{{ formatRelative(a.created_at) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- ============ 模式 2：表格列表视图 ============ -->
          <VCard v-else>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] text-left">
                <thead>
                  <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                    <th class="w-12 px-4 py-3 text-center">
                      <input
                        type="checkbox"
                        :checked="attachments.length > 0 && selectedIds.size === attachments.length"
                        :disabled="attachments.length === 0"
                        class="h-4 w-4 rounded border-border text-primary disabled:cursor-not-allowed disabled:opacity-40"
                        @change="attachments.length > 0 && selectedIds.size === attachments.length ? clearSelection() : selectAll()"
                      />
                    </th>
                    <th class="px-4 py-3">文件名称</th>
                    <th class="px-4 py-3">格式 / 类型</th>
                    <th class="px-4 py-3">文件大小</th>
                    <th class="px-4 py-3">上传时间</th>
                    <th class="w-32 px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border/60">
                  <tr
                    v-for="a in attachments"
                    :key="a.id"
                    class="cursor-pointer transition-colors hover:bg-secondary/40"
                    :class="{ 'bg-primary/5': selectedIds.has(a.id) }"
                    @click="openDetail(a)"
                  >
                    <td class="px-4 py-3 text-center" @click.stop>
                      <input
                        type="checkbox"
                        :checked="selectedIds.has(a.id)"
                        class="h-4 w-4 rounded border-border text-primary"
                        @change="toggleSelect(a.id)"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <div class="flex items-center gap-3">
                        <img
                          v-if="a.mime && a.mime.startsWith('image/')"
                          :src="a.url"
                          class="h-9 w-9 shrink-0 rounded-lg border border-border object-cover bg-secondary"
                          alt=""
                        />
                        <div
                          v-else
                          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-base text-text-tertiary"
                        >
                          <Icon :icon="categoryOf(a.mime) === 'video' ? 'ri:video-line' : 'ri:file-text-line'" />
                        </div>
                        <div class="min-w-0">
                          <div class="font-semibold text-foreground text-sm truncate max-w-[280px]">
                            {{ a.filename }}
                          </div>
                          <div class="text-xs font-mono text-text-quaternary truncate max-w-[280px]">{{ a.url }}</div>
                        </div>
                      </div>
                    </td>
                    <td class="px-4 py-3 text-xs font-mono text-text-secondary">
                      <span class="rounded bg-secondary px-2 py-0.5 font-bold uppercase">
                        {{ formatMimeBadge(a.mime) }}
                      </span>
                    </td>
                    <td class="px-4 py-3 font-mono text-xs text-text-tertiary">
                      {{ formatBytes(a.size) }}
                    </td>
                    <td class="px-4 py-3 text-xs text-text-quaternary">
                      {{ formatRelative(a.created_at) }}
                    </td>
                    <td class="px-4 py-3 text-right" @click.stop>
                      <div class="flex justify-end gap-1">
                        <button
                          type="button"
                          class="action-icon-btn"
                          title="复制链接"
                          @click="copyText(a.url, '已复制直链 URL')"
                        >
                          <Icon icon="ri:link" />
                        </button>
                        <button
                          type="button"
                          class="action-icon-btn hover:text-[var(--color-state-error)]"
                          title="删除"
                          @click="handleDeleteSingle(a)"
                        >
                          <Icon icon="ri:delete-bin-line" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </VCard>

          <!-- 分页 -->
          <div v-if="total > size" class="flex justify-center pt-4">
            <VPagination
              :page="page"
              :size="size"
              :total="total"
              @update:page="page = $event"
              @update:size="size = $event"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- ============ 分组管理弹窗 ============ -->
    <VModal
      v-model:visible="groupModalVisible"
      :title="groupModalMode === 'create' ? '新建分组' : '重命名分组'"
      :width="400"
    >
      <div>
        <label class="mb-1.5 block text-xs font-medium text-text-tertiary">分组名称 *</label>
        <input
          v-model="groupModalName"
          class="apple-input"
          placeholder="如：文章插图 / 封面库"
          autofocus
          @keyup.enter="submitGroup"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="groupModalVisible = false">取消</VButton>
          <VButton type="primary" :loading="groupSaving" @click="submitGroup">保存</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ URL 导入弹窗 ============ -->
    <VModal v-model:visible="importURLVisible" title="从 URL 导入图片" :width="520">
      <div class="space-y-3">
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">图片 URL</label>
          <input
            v-model="importURL"
            class="apple-input"
            type="url"
            placeholder="https://example.com/image.png"
            autofocus
            @keyup.enter="submitURLImport"
          />
        </div>
        <p class="text-xs text-text-quaternary">仅支持 HTTP/HTTPS 图片；服务端会校验目标地址并限制文件大小。</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="importURLVisible = false">取消</VButton>
          <VButton type="primary" :loading="importingURL" @click="submitURLImport">导入</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 附件详情抽屉 ============ -->
    <VModal :visible="!!detailItem" title="附件属性详情" :width="540" @update:visible="closeDetail">
      <div v-if="detailItem" class="space-y-4">
        <!-- 大图 / 媒体预览 -->
        <div
          class="flex items-center justify-center rounded-2xl border border-border bg-secondary/50 p-4 max-h-[300px] overflow-hidden"
        >
          <img
            v-if="detailItem.mime && detailItem.mime.startsWith('image/')"
            :src="detailItem.url"
            class="max-h-[260px] rounded-xl object-contain shadow-sm"
            :alt="detailItem.filename"
          />
          <div v-else class="py-12 text-center text-4xl text-text-tertiary">
            <Icon icon="ri:file-text-line" />
          </div>
        </div>

        <!-- 基础元数据 -->
        <div class="grid grid-cols-2 gap-3 text-xs">
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">文件名</div>
            <div class="font-semibold text-foreground truncate mt-1" :title="detailItem.filename">
              {{ detailItem.filename }}
            </div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">文件大小</div>
            <div class="font-semibold text-foreground font-mono mt-1">{{ formatBytes(detailItem.size) }}</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">MIME 类型</div>
            <div class="font-semibold text-foreground font-mono mt-1 truncate">{{ detailItem.mime }}</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">上传时间</div>
            <div class="font-semibold text-foreground mt-1">{{ formatRelative(detailItem.created_at) }}</div>
          </div>
        </div>

        <!-- 所属分组 -->
        <div class="rounded-xl border border-border bg-card p-3">
          <label for="attachment-detail-group" class="mb-1.5 block text-xs font-medium text-text-tertiary"
            >所属分组</label
          >
          <select
            id="attachment-detail-group"
            v-model.number="detailGroupID"
            class="apple-select w-full"
            data-testid="attachment-detail-group"
            aria-label="附件分组"
          >
            <option :value="0">未分组</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>

        <!-- 快捷复制 -->
        <div class="space-y-2">
          <label class="block text-xs font-medium text-text-tertiary">直链 URL</label>
          <div class="flex items-center gap-2">
            <input :value="detailItem.url" readonly class="apple-input font-mono text-xs" />
            <VButton size="sm" @click="copyText(detailItem.url)">复制</VButton>
          </div>
        </div>

        <!-- 前台引用情况 -->
        <div class="rounded-xl border border-border bg-secondary/30 p-3 text-xs">
          <div class="mb-2 font-semibold text-foreground flex items-center justify-between">
            <span>使用排查与引用关联</span>
            <Icon v-if="detailRefsLoading" icon="ri:loader-4-line" class="animate-spin text-primary" />
          </div>

          <div v-if="detailRefsLoading" class="text-text-quaternary py-2 text-center">正在排查全站引用...</div>
          <div
            v-else-if="
              !detailRefs || (!detailRefs.posts?.length && !detailRefs.pages?.length && !detailRefs.moments?.length)
            "
            class="text-text-quaternary py-1"
          >
            暂未在已发布的文章、单页或瞬间中检测到直接引用。
          </div>
          <div v-else class="space-y-1.5">
            <div v-if="detailRefs.posts?.length" class="text-text-secondary">
              <span class="font-medium text-foreground">文章：</span>
              <span
                v-for="p in detailRefs.posts"
                :key="p.slug"
                class="ml-1 rounded bg-secondary px-1.5 py-0.5 text-[11px]"
              >
                {{ p.title }}
              </span>
            </div>
            <div v-if="detailRefs.pages?.length" class="text-text-secondary">
              <span class="font-medium text-foreground">单页：</span>
              <span
                v-for="p in detailRefs.pages"
                :key="p.slug"
                class="ml-1 rounded bg-secondary px-1.5 py-0.5 text-[11px]"
              >
                {{ p.title }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-between w-full">
          <VButton type="danger" size="sm" @click="detailItem && handleDeleteSingle(detailItem)"> 删除附件 </VButton>
          <div class="flex items-center gap-2">
            <VButton
              type="primary"
              size="sm"
              data-testid="save-attachment-group"
              :loading="detailGroupSaving"
              :disabled="!detailGroupChanged"
              @click="saveDetailGroup"
            >
              保存分组
            </VButton>
            <VButton @click="closeDetail">关闭</VButton>
          </div>
        </div>
      </template>
    </VModal>

    <!-- ============ 底部批量操作浮动栏 (VFloatingDock) ============ -->
    <VFloatingDock :visible="selectedIds.size > 0" :count="selectedIds.size" title="已选附件" @clear="clearSelection">
      <div class="flex items-center gap-2">
        <select v-model="batchTargetGroup" class="apple-select h-8 text-xs w-36">
          <option value="">迁移至分组...</option>
          <option :value="0">未分组</option>
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
        <VButton size="sm" :loading="batchMoving" @click="batchMove"> 迁移 </VButton>
        <VButton size="sm" type="danger" @click="batchDelete"> 批量删除 </VButton>
      </div>
    </VFloatingDock>
  </div>
</template>

<style scoped>
/* 分组项按钮 */
.group-item-btn {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.group-item-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.group-item-btn.active {
  background: var(--color-primary);
  color: #fff;
}
.group-item-btn.active .text-primary {
  color: #fff !important;
}
.group-count-badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: var(--radius-full);
  background: color-mix(in srgb, currentColor 15%, transparent);
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
}
</style>
