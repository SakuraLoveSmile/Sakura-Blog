<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { attachmentApi, attachmentGroupApi } from "@/api";
import { formatRelative, formatBytes } from "@/lib/format";
import { themeColors } from "@/lib/theme-colors";
import { Icon } from "@iconify/vue";
import {
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  VModal,
  FilterTag,
  Dialog,
  Toast,
} from "@/components/ui";

const queryClient = useQueryClient();
const colors = themeColors({
  primary: "#2E8DFF",
  chart5: "#BF5AF2",
  warning: "#FF9F0A",
  success: "#30D158",
  tertiary: "#8E8E93",
});

// 搜索关键词（debounce 300ms）
const keyword = ref("");
const debouncedKeyword = ref("");
const page = ref(1);
const size = ref(24);
const uploading = ref(false);
// 视图：grid / list
const viewMode = ref<"grid" | "list">("grid");

// 类型分段筛选：all / image / document / video / audio / other
type TypeFilter = "all" | "image" | "document" | "video" | "audio" | "other";
const typeFilter = ref<TypeFilter>("all");
const typeFilters: { key: TypeFilter; label: string }[] = [
  { key: "all", label: "全部" },
  { key: "image", label: "图片" },
  { key: "document", label: "文档" },
  { key: "video", label: "视频" },
  { key: "audio", label: "音频" },
  { key: "other", label: "其他" },
];

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debouncedKeyword.value = keyword.value;
    page.value = 1;
  }, 300);
});

// --- 分组（Spec 44）---
const activeGroup = ref<number | undefined>(undefined);

interface GroupItem {
  id: number;
  name: string;
  order: number;
  count: number;
}
const groups = ref<GroupItem[]>([]);

async function loadGroups() {
  try {
    const { data } = await attachmentGroupApi.list();
    groups.value = data.items || [];
  } catch {}
}
loadGroups();

function selectGroup(g: number | undefined) {
  activeGroup.value = g;
  page.value = 1;
  clearSelection();
}

const activeGroupName = computed(() => {
  if (activeGroup.value === undefined) return "全部";
  if (activeGroup.value === 0) return "未分组";
  return groups.value.find((g) => g.id === activeGroup.value)?.name || "分组";
});

const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (debouncedKeyword.value.trim()) params.keyword = debouncedKeyword.value.trim();
  // 保留后端 kind 过滤：image / other（仅图片 / 非图片），其他分段在前端过滤
  if (typeFilter.value === "image") params.kind = "image";
  else if (typeFilter.value !== "all") params.kind = "other";
  if (activeGroup.value !== undefined) params.group = activeGroup.value;
  return params;
});

const { data: queryData, isLoading, refetch } = useQuery({
  queryKey: computed(() => queryKeys.attachments(queryParams.value)),
  queryFn: () => attachmentApi.list(queryParams.value).then(({ data }) => data),
});

const attachments = computed<any[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

// 前端按 mime 精确分类（后端 kind 只区分 image/other，细分在前端完成）
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
    mime.includes("powerpoint") ||
    mime.includes("document") ||
    mime.includes("sheet") ||
    mime.includes("presentation") ||
    mime.includes("markdown") ||
    mime.includes("json") ||
    mime.includes("xml")
  ) {
    return "document";
  }
  return "other";
}

const displayedAttachments = computed(() => {
  if (typeFilter.value === "all" || typeFilter.value === "image") return attachments.value;
  return attachments.value.filter((a) => categoryOf(a.mime) === typeFilter.value);
});

function invalidateAll() {
  queryClient.invalidateQueries({ queryKey: queryKeys.attachments() });
  loadGroups();
}

async function handleUpload(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  uploading.value = true;
  try {
    const gid = activeGroup.value && activeGroup.value > 0 ? activeGroup.value : undefined;
    await attachmentApi.upload(input.files[0], gid);
    invalidateAll();
    Toast.success("上传成功");
  } catch {} finally {
    uploading.value = false;
    input.value = "";
  }
}

// --- 分组管理 ---
const groupModalVisible = ref(false);
const groupModalMode = ref<"create" | "rename">("create");
const groupModalName = ref("");
const groupModalTarget = ref<GroupItem | null>(null);

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
  try {
    if (groupModalMode.value === "create") {
      await attachmentGroupApi.create({ name, order: groups.value.length });
      Toast.success("分组已创建");
    } else if (groupModalTarget.value) {
      await attachmentGroupApi.update(groupModalTarget.value.id, {
        name,
        order: groupModalTarget.value.order,
      });
      Toast.success("分组已重命名");
    }
    groupModalVisible.value = false;
    await loadGroups();
  } catch {}
}

function requestDeleteGroup(g: GroupItem) {
  Dialog.warning({
    title: `确定删除分组「${g.name}」吗？`,
    description: "组内附件将归入未分组，文件本身不会被删除。",
    confirmType: "danger",
    onConfirm: async () => {
      await attachmentGroupApi.delete(g.id);
      if (activeGroup.value === g.id) selectGroup(undefined);
      invalidateAll();
      Toast.success("分组已删除");
    },
  });
}

// --- 多选批量移动 ---
const selectedIds = ref<number[]>([]);
const checkedAll = ref(false);

watch(selectedIds, (val) => {
  checkedAll.value = val.length === displayedAttachments.value.length && val.length > 0;
});

function handleCheckAllChange(e: Event) {
  const { checked } = e.target as HTMLInputElement;
  selectedIds.value = checked ? displayedAttachments.value.map((a: any) => a.id) : [];
}

function checkSelection(id: number) {
  return selectedIds.value.includes(id);
}

function toggleSelection(id: number) {
  if (checkSelection(id)) {
    selectedIds.value = selectedIds.value.filter((i) => i !== id);
  } else {
    selectedIds.value = [...selectedIds.value, id];
  }
}

function clearSelection() {
  selectedIds.value = [];
}

// 批量移动弹窗
const moveVisible = ref(false);

async function moveTo(groupId: number) {
  if (!selectedIds.value.length) return;
  try {
    const { data } = await attachmentApi.move([...selectedIds.value], groupId);
    moveVisible.value = false;
    clearSelection();
    invalidateAll();
    Toast.success(`已移动 ${data.moved ?? 0} 项`);
  } catch {}
}

// 详情弹窗
const detailVisible = ref(false);
const detailItem = ref<any | null>(null);
const detailRefs = ref<any[]>([]);
const refsLoading = ref(false);

async function openDetail(att: any) {
  detailItem.value = att;
  detailRefs.value = [];
  detailVisible.value = true;
  refsLoading.value = true;
  try {
    const { data } = await attachmentApi.refs(att.id);
    detailRefs.value = data.refs || [];
  } catch {} finally {
    refsLoading.value = false;
  }
}

// 下载附件
function handleDownload(att: any) {
  if (!att?.url) return;
  const a = document.createElement("a");
  a.href = att.url;
  a.download = att.filename || "";
  a.target = "_blank";
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

// 删除（带引用警告）
const deleteVisible = ref(false);
const deleteTarget = ref<any | null>(null);
const deleteRefs = ref<any[]>([]);
const deleteRefsLoading = ref(false);

async function requestDelete(att: any) {
  deleteTarget.value = att;
  deleteRefs.value = [];
  deleteVisible.value = true;
  deleteRefsLoading.value = true;
  try {
    const { data } = await attachmentApi.refs(att.id);
    deleteRefs.value = data.refs || [];
  } catch {} finally {
    deleteRefsLoading.value = false;
  }
}

async function confirmDelete() {
  if (!deleteTarget.value) return;
  const target = deleteTarget.value;
  await attachmentApi.delete(target.id);
  deleteVisible.value = false;
  if (detailItem.value?.id === target.id) detailVisible.value = false;
  deleteTarget.value = null;
  invalidateAll();
  Toast.success("删除成功");
}


function isImage(mime: string) {
  return mime?.startsWith("image/");
}

function isVideo(mime: string) {
  return mime?.startsWith("video/");
}

// 文件扩展名（大写展示）
function extOf(att: any): string {
  const name: string = att?.filename || "";
  const dot = name.lastIndexOf(".");
  if (dot >= 0 && dot < name.length - 1) return name.slice(dot + 1).toUpperCase();
  if (att?.mime) {
    const sub = att.mime.split("/")[1];
    return sub ? sub.toUpperCase().slice(0, 4) : "FILE";
  }
  return "FILE";
}


// 预览背景复用全局头像渐变，确保附件、评论和通知的色板一致。
function previewGradientClass(id: number): string {
  return `avatar-gradient-${(Math.abs(Number(id) || 0) % 8) + 1}`;
}

// --- 存储空间统计（基于当前页附件 computed 派生） ---
type StorageCat = "image" | "document" | "video" | "audio" | "other";
const STORAGE_META: Record<StorageCat, { label: string; color: string }> = {
  image: { label: "图片", color: colors.primary },
  document: { label: "文档", color: colors.success },
  video: { label: "视频", color: colors.chart5 },
  audio: { label: "音频", color: colors.warning },
  other: { label: "其他", color: colors.tertiary },
};

const storageStats = computed(() => {
  const buckets: Record<StorageCat, number> = {
    image: 0,
    document: 0,
    video: 0,
    audio: 0,
    other: 0,
  };
  for (const a of attachments.value) {
    const cat = categoryOf(a.mime) as StorageCat;
    buckets[cat] += Number(a.size) || 0;
  }
  const total = Object.values(buckets).reduce((s, v) => s + v, 0);
  // 估算总容量：已用的 4 倍，下限 1GB（无后端总容量字段时的占位）
  const estimatedQuota = Math.max(total * 4, 1073741824);
  return { buckets, total, quota: estimatedQuota };
});

const storagePercent = computed(() => {
  const { total, quota } = storageStats.value;
  if (!quota) return 0;
  return Math.min(100, Math.round((total / quota) * 1000) / 10);
});

// 多色渐变进度条：按各分类占比分段
const storageGradient = computed(() => {
  const { buckets, total } = storageStats.value;
  if (!total) return "var(--color-accent)";
  const order: StorageCat[] = ["image", "document", "video", "audio", "other"];
  let acc = 0;
  const stops: string[] = [];
  for (const cat of order) {
    const v = buckets[cat];
    if (!v) continue;
    const start = (acc / total) * 100;
    acc += v;
    const end = (acc / total) * 100;
    stops.push(`${STORAGE_META[cat].color} ${start.toFixed(2)}%, ${STORAGE_META[cat].color} ${end.toFixed(2)}%`);
  }
  return `linear-gradient(90deg, ${stops.join(", ")})`;
});

const storageBreakdown = computed(() => {
  const { buckets, total } = storageStats.value;
  const order: StorageCat[] = ["image", "document", "video", "audio", "other"];
  return order
    .map((cat) => ({
      key: cat,
      label: STORAGE_META[cat].label,
      color: STORAGE_META[cat].color,
      size: buckets[cat],
      percent: total ? Math.round((buckets[cat] / total) * 1000) / 10 : 0,
    }))
    .filter((x) => x.size > 0);
});

function handleFilterChangeLocal() {
  page.value = 1;
  clearSelection();
}

function typeLabel(mime: string): string {
  if (!mime) return "其他";
  if (mime.startsWith("image/")) return "图片";
  if (mime.startsWith("video/")) return "视频";
  if (mime.startsWith("audio/")) return "音频";
  if (
    mime.startsWith("text/") ||
    mime.includes("pdf") ||
    mime.includes("word") ||
    mime.includes("document")
  ) {
    return "文档";
  }
  return "其他";
}

function typePillStyle(mime: string) {
  let color = colors.tertiary;
  if (mime?.startsWith("image/")) color = colors.primary;
  else if (mime?.startsWith("video/")) color = colors.chart5;
  else if (mime?.startsWith("audio/")) color = colors.warning;
  else if (
    mime?.startsWith("text/") ||
    mime?.includes("pdf") ||
    mime?.includes("document")
  ) {
    color = colors.success;
  }
  return {
    backgroundColor: `${color}1A`,
    color,
  };
}
</script>

<template>
  <div class="attachments-page min-h-full">
    <!-- ===== Page Header ===== -->
    <header class="attachments-page-header !px-4 !py-4 sm:!px-5 sm:!py-5">
      <div class="flex flex-col gap-0.5 sm:gap-1">
        <h1 class="text-xl font-bold leading-tight tracking-tight text-foreground sm:text-[26px]">附件管理</h1>
        <p class="text-[12px] text-text-tertiary sm:text-[13px]">
          共 {{ total }} 个文件 · 已使用 {{ formatBytes(storageStats.total) }}
        </p>
      </div>
      <div class="flex items-center gap-1.5 sm:gap-2.5">
        <label class="apple-upload-btn !h-8 !px-2.5 !text-xs sm:!h-auto sm:!px-4 sm:!text-[13px]">
          <span class="flex items-center gap-1.5">
            <Icon icon="ri:upload-2-line" class="text-sm sm:text-[15px]" />
            <span class="sm:inline hidden">上传附件{{ activeGroup && activeGroup > 0 ? `到「${activeGroupName}」` : "" }}</span>
            <span class="sm:hidden">上传</span>
          </span>
          <input type="file" class="hidden" :disabled="uploading" @change="handleUpload" />
        </label>
      </div>
    </header>

    <!-- ===== Content Area ===== -->
    <div class="attachments-content-area !px-4 sm:!px-5">
      <div class="flex flex-col gap-3 sm:gap-4 lg:flex-row">
        <!-- 左侧分组栏（桌面端侧边栏，移动端紧凑横排） -->
        <aside class="w-full shrink-0 lg:w-56">
          <VCard :body-class="['!p-0']">
            <div class="flex items-center justify-between border-b border-border px-3 py-2.5 sm:px-4 sm:py-3">
              <span class="text-xs font-semibold text-foreground sm:text-[13px]">分组</span>
              <button
                class="flex h-6 w-6 items-center justify-center rounded-md text-text-tertiary transition-colors hover:bg-accent hover:text-primary"
                title="新建分组"
                @click="openCreateGroup"
              >
                <Icon icon="ri:add-line" />
              </button>
            </div>
            <!-- 移动端：横向滚动分组列表 -->
            <div class="group-scroll lg:hidden">
              <ul class="flex gap-1 p-2 text-xs">
                <li>
                  <button
                    class="group-sidebar-item-mobile"
                    :class="{ active: activeGroup === undefined }"
                    @click="selectGroup(undefined)"
                  >
                    <Icon icon="ri:apps-line" class="text-sm" />
                    <span>全部</span>
                  </button>
                </li>
                <li>
                  <button
                    class="group-sidebar-item-mobile"
                    :class="{ active: activeGroup === 0 }"
                    @click="selectGroup(0)"
                  >
                    <Icon icon="ri:inbox-line" class="text-sm" />
                    <span>未分组</span>
                  </button>
                </li>
                <li v-for="g in groups" :key="g.id">
                  <button
                    class="group-sidebar-item-mobile"
                    :class="{ active: activeGroup === g.id }"
                    @click="selectGroup(g.id)"
                  >
                    <Icon icon="ri:folder-line" class="shrink-0 text-sm" />
                    <span class="truncate">{{ g.name }}</span>
                  </button>
                </li>
              </ul>
            </div>
            <!-- 桌面端：垂直列表 -->
            <ul class="hidden py-1 text-[13px] lg:block">
              <li>
                <button
                  class="group-sidebar-item"
                  :class="{ active: activeGroup === undefined }"
                  @click="selectGroup(undefined)"
                >
                  <span class="flex items-center gap-2">
                    <Icon icon="ri:apps-line" />
                    <span>全部</span>
                  </span>
                </button>
              </li>
              <li>
                <button
                  class="group-sidebar-item"
                  :class="{ active: activeGroup === 0 }"
                  @click="selectGroup(0)"
                >
                  <span class="flex items-center gap-2">
                    <Icon icon="ri:inbox-line" />
                    <span>未分组</span>
                  </span>
                </button>
              </li>
              <li v-for="g in groups" :key="g.id" class="group/item">
                <div
                  class="group-sidebar-item"
                  :class="{ active: activeGroup === g.id }"
                >
                  <button class="flex min-w-0 flex-1 items-center gap-2 text-left" @click="selectGroup(g.id)">
                    <Icon icon="ri:folder-line" class="shrink-0" />
                    <span class="truncate">{{ g.name }}</span>
                    <span class="ml-auto text-[10px] opacity-60">{{ g.count }}</span>
                  </button>
                  <span class="ml-1 hidden shrink-0 gap-0.5 group-hover/item:flex">
                    <button
                      class="flex h-6 w-6 items-center justify-center rounded-md text-text-quaternary hover:bg-accent hover:text-primary"
                      title="重命名"
                      @click.stop="openRenameGroup(g)"
                    >
                      <Icon icon="ri:edit-line" class="text-xs" />
                    </button>
                    <button
                      class="flex h-6 w-6 items-center justify-center rounded-md text-text-quaternary hover:bg-accent hover:text-[var(--color-destructive)]"
                      title="删除"
                      @click.stop="requestDeleteGroup(g)"
                    >
                      <Icon icon="ri:delete-bin-line" class="text-xs" />
                    </button>
                  </span>
                </div>
              </li>
            </ul>
          </VCard>
        </aside>

        <!-- 右侧主内容 -->
        <div class="min-w-0 flex-1">
          <!-- 存储卡 -->
          <div class="apple-card-highlight storage-card">
            <div class="flex items-center justify-between">
              <div>
                <div class="storage-title">存储空间</div>
                <div class="storage-info">
                  已使用 <strong>{{ formatBytes(storageStats.total) }}</strong>
                  <span class="opacity-60"> / 估算 {{ formatBytes(storageStats.quota) }} ({{ storagePercent }}%)</span>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="apple-icon-btn"
                  :class="{ active: isLoading }"
                  title="刷新"
                  @click="refetch()"
                >
                  <Icon icon="ri:refresh-line" :class="{ 'animate-spin': isLoading }" />
                </button>
              </div>
            </div>
            <div class="storage-progress">
              <div
                class="storage-progress-fill"
                :style="{ width: storagePercent + '%', background: storageGradient }"
              ></div>
            </div>
            <div class="storage-breakdown">
              <div v-for="item in storageBreakdown" :key="item.key" class="storage-item">
                <span class="storage-dot" :style="{ backgroundColor: item.color }"></span>
                <span class="storage-label">
                  <strong>{{ item.label }}</strong>
                  {{ formatBytes(item.size) }}
                </span>
              </div>
              <div v-if="!storageBreakdown.length" class="storage-label">暂无数据</div>
            </div>
          </div>

          <!-- 工具栏：分段筛选 + 视图切换 -->
          <div class="attachments-toolbar">
            <div class="tabs-scroll apple-segmented -mx-1 px-1 sm:mx-0 sm:px-0" role="tablist" aria-label="附件类型筛选">
              <button
                v-for="f in typeFilters"
                :key="f.key"
                type="button"
                role="tab"
                :aria-selected="typeFilter === f.key"
                :class="{ active: typeFilter === f.key }"
                @click="typeFilter = f.key; page = 1; clearSelection()"
              >
                {{ f.label }}
              </button>
            </div>

            <div class="flex items-center gap-2">
              <!-- 胶囊搜索框 -->
              <label class="apple-search apple-search-compact w-[160px] sm:w-[200px]">
                <Icon icon="ri:search-line" class="text-[15px] text-text-quaternary" />
                <input v-model="keyword" placeholder="搜索附件..." type="text" />
                <button
                  v-if="keyword"
                  type="button"
                  class="flex h-4 w-4 items-center justify-center rounded-full text-text-quaternary hover:bg-accent hover:text-foreground"
                  @click="keyword = ''"
                >
                  <Icon icon="ri:close-line" class="text-xs" />
                </button>
              </label>

              <!-- 视图切换 -->
              <div class="apple-view-toggle">
                <button
                  type="button"
                  title="网格视图"
                  :class="{ active: viewMode === 'grid' }"
                  @click="viewMode = 'grid'"
                >
                  <Icon icon="ri:grid-line" />
                </button>
                <button
                  type="button"
                  title="列表视图"
                  :class="{ active: viewMode === 'list' }"
                  @click="viewMode = 'list'"
                >
                  <Icon icon="ri:list-check-2" />
                </button>
              </div>
            </div>
          </div>

          <!-- 已选筛选标签 + 批量操作 -->
          <div v-if="selectedIds.length || activeGroup !== undefined || keyword || typeFilter !== 'all'" class="mb-3 flex flex-wrap items-center gap-2">
            <FilterTag v-if="activeGroup !== undefined" @close="selectGroup(undefined)">
              分组：{{ activeGroupName }}
            </FilterTag>
            <FilterTag v-if="typeFilter !== 'all'" @close="typeFilter = 'all'; handleFilterChangeLocal()">
              类型：{{ typeFilters.find((f) => f.key === typeFilter)?.label }}
            </FilterTag>
            <FilterTag v-if="keyword" @close="keyword = ''">关键词：{{ keyword }}</FilterTag>
            <template v-if="selectedIds.length">
              <span class="text-[13px] text-text-tertiary">已选择 {{ selectedIds.length }} 项</span>
              <VButton size="sm" type="secondary" @click="moveVisible = true">
                <template #icon><Icon icon="ri:folder-transfer-line" class="h-full w-full" /></template>
                移动到分组
              </VButton>
              <button
                class="text-[13px] text-text-quaternary hover:text-text-tertiary"
                @click="clearSelection"
              >
                取消选择
              </button>
            </template>
          </div>

          <VLoading v-if="isLoading" />

          <Transition v-else-if="!displayedAttachments.length" appear name="fade">
            <VEmpty message="你可以尝试刷新或者上传附件" title="当前没有附件">
              <template #actions>
                <VButton @click="refetch">刷新</VButton>
              </template>
            </VEmpty>
          </Transition>

          <!-- ===== Grid View ===== -->
          <Transition v-else-if="viewMode === 'grid'" appear name="fade">
            <div class="attachment-grid">
              <div
                v-for="att in displayedAttachments"
                :key="att.id"
                class="apple-card-highlight attachment-card"
                :class="{ selected: checkSelection(att.id) }"
                @click="openDetail(att)"
              >
                <div class="attachment-preview">
                  <img
                    v-if="isImage(att.mime) && (att.thumb_url || att.url)"
                    :src="att.thumb_url || att.url"
                    :alt="att.filename"
                    class="attachment-preview-img"
                    loading="lazy"
                  />
                  <div
                    v-else
                    :class="['attachment-preview-fallback', previewGradientClass(att.id)]"
                  >
                    <template v-if="isVideo(att.mime)">
                      <div class="play-button">
                        <Icon icon="ri:play-fill" />
                      </div>
                    </template>
                    <template v-else>
                      <Icon
                        :icon="isImage(att.mime)
                          ? 'ri:image-line'
                          : att.mime?.startsWith('audio/')
                            ? 'ri:music-2-line'
                            : 'ri:file-3-line'"
                        class="text-[40px] text-foreground/85"
                      />
                      <span class="preview-ext">{{ extOf(att) }}</span>
                    </template>
                  </div>

                  <!-- 选择框 -->
                  <input
                    type="checkbox"
                    :checked="checkSelection(att.id)"
                    class="attachment-checkbox"
                    @click.stop="toggleSelection(att.id)"
                  />

                  <!-- Hover 遮罩 -->
                  <div class="attachment-hover-overlay" @click.stop>
                    <button class="attachment-action" title="预览" @click="openDetail(att)">
                      <Icon icon="ri:eye-line" />
                    </button>
                    <button class="attachment-action" title="下载" @click="handleDownload(att)">
                      <Icon icon="ri:download-2-line" />
                    </button>
                    <button class="attachment-action delete" title="删除" @click="requestDelete(att)">
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>
                <div class="attachment-info">
                  <div class="attachment-name" :title="att.filename">{{ att.filename }}</div>
                  <div class="attachment-meta">
                    <span>{{ formatBytes(att.size) }}</span>
                    <span>{{ formatRelative(att.created_at) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </Transition>

          <!-- ===== List View ===== -->
          <Transition v-else appear name="fade">
            <VCard :body-class="['!p-0']">
              <div class="attachment-list">
                <div class="attachment-list-head">
                  <span class="col-check">
                    <input
                      type="checkbox"
                      :checked="checkedAll"
                      class="cursor-pointer"
                      @change="handleCheckAllChange"
                    />
                  </span>
                  <span class="col-name">文件名</span>
                  <span class="col-size">大小</span>
                  <span class="col-type">类型</span>
                  <span class="col-time">上传时间</span>
                  <span class="col-actions"></span>
                </div>
                <div
                  v-for="att in displayedAttachments"
                  :key="att.id"
                  class="attachment-list-row"
                  :class="{ selected: checkSelection(att.id) }"
                  @click="openDetail(att)"
                >
                  <span class="col-check" @click.stop>
                    <input
                      type="checkbox"
                      :checked="checkSelection(att.id)"
                      class="cursor-pointer"
                      @change="toggleSelection(att.id)"
                    />
                  </span>
                  <span class="col-name">
                    <span class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-[8px] bg-secondary">
                      <img
                        v-if="isImage(att.mime) && (att.thumb_url || att.url)"
                        :src="att.thumb_url || att.url"
                        :alt="att.filename"
                        class="h-full w-full object-cover"
                        loading="lazy"
                      />
                      <Icon
                        v-else
                        :icon="isVideo(att.mime)
                          ? 'ri:video-line'
                          : att.mime?.startsWith('audio/')
                            ? 'ri:music-2-line'
                            : 'ri:file-3-line'"
                        class="text-[18px] text-text-tertiary"
                      />
                    </span>
                    <span class="truncate text-[13px] text-foreground" :title="att.filename">
                      {{ att.filename }}
                    </span>
                  </span>
                  <span class="col-size text-[12px] text-text-tertiary">{{ formatBytes(att.size) }}</span>
                  <span class="col-type text-[12px] text-text-tertiary">
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium"
                      :style="typePillStyle(att.mime)"
                    >
                      {{ typeLabel(att.mime) }}
                    </span>
                  </span>
                  <span class="col-time text-[12px] text-text-tertiary">{{ formatRelative(att.created_at) }}</span>
                  <span class="col-actions flex items-center gap-1" @click.stop>
                    <button class="apple-icon-btn-sm" title="预览" @click="openDetail(att)">
                      <Icon icon="ri:eye-line" />
                    </button>
                    <button class="apple-icon-btn-sm" title="下载" @click="handleDownload(att)">
                      <Icon icon="ri:download-2-line" />
                    </button>
                    <button class="apple-icon-btn-sm danger" title="删除" @click="requestDelete(att)">
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </span>
                </div>
              </div>
            </VCard>
          </Transition>

          <!-- 分页 -->
          <div
            v-if="total > size"
            class="mt-5 flex items-center justify-center border-t border-border pt-4"
          >
            <VPagination
              v-model:page="page"
              v-model:size="size"
              :total="total"
              :size-options="[24, 48, 96]"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- 分组新建/重命名弹窗 -->
    <VModal v-model:visible="groupModalVisible" :title="groupModalMode === 'create' ? '新建分组' : '重命名分组'" :width="380">
      <input
        v-model="groupModalName"
        placeholder="分组名称"
        type="text"
        class="apple-input"
        @keyup.enter="submitGroup"
      />
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="groupModalVisible = false">取消</VButton>
          <VButton type="primary" @click="submitGroup">确定</VButton>
        </div>
      </template>
    </VModal>

    <!-- 批量移动到分组弹窗 -->
    <VModal v-model:visible="moveVisible" title="移动到分组" :width="380">
      <p class="mb-3 text-[13px] text-text-tertiary">将选中的 {{ selectedIds.length }} 项移动到：</p>
      <ul class="space-y-1 text-sm">
        <li>
          <button
            class="flex w-full items-center gap-2 rounded-[8px] px-3 py-2 text-left text-text-tertiary transition-colors hover:bg-accent"
            @click="moveTo(0)"
          >
            <Icon icon="ri:inbox-line" />未分组
          </button>
        </li>
        <li v-for="g in groups" :key="g.id">
          <button
            class="flex w-full items-center gap-2 rounded-[8px] px-3 py-2 text-left text-text-tertiary transition-colors hover:bg-accent"
            @click="moveTo(g.id)"
          >
            <Icon icon="ri:folder-line" />{{ g.name }}
          </button>
        </li>
      </ul>
      <template #footer>
        <div class="flex justify-end">
          <VButton @click="moveVisible = false">取消</VButton>
        </div>
      </template>
    </VModal>

    <!-- 详情弹窗 -->
    <VModal v-model:visible="detailVisible" :title="detailItem?.filename" :width="560">
      <template v-if="detailItem">
        <div class="mb-4 overflow-hidden rounded-[10px] bg-secondary">
          <img
            v-if="isImage(detailItem.mime)"
            :src="detailItem.url"
            :alt="detailItem.filename"
            class="max-h-64 w-full object-contain"
          />
          <div v-else class="flex flex-col items-center py-12 text-text-quaternary">
            <Icon icon="ri:file-3-line" class="text-5xl" />
          </div>
        </div>

        <div class="mb-4 grid grid-cols-2 gap-3 text-sm">
          <div>
            <span class="text-text-quaternary">格式：</span>
            <span class="text-text-secondary">{{ detailItem.mime }}</span>
          </div>
          <div>
            <span class="text-text-quaternary">大小：</span>
            <span class="text-text-secondary">{{ formatBytes(detailItem.size) }}</span>
          </div>
          <div v-if="detailItem.width">
            <span class="text-text-quaternary">尺寸：</span>
            <span class="text-text-secondary">{{ detailItem.width }}×{{ detailItem.height }} px</span>
          </div>
          <div>
            <span class="text-text-quaternary">上传：</span>
            <span class="text-text-secondary">{{ new Date(detailItem.created_at).toLocaleDateString() }}</span>
          </div>
        </div>

        <div class="mb-4">
          <label class="mb-1 block text-xs text-text-quaternary">引用 URL</label>
          <code
            class="block select-all break-all rounded-[8px] bg-secondary px-3 py-2 text-xs text-text-tertiary"
          >{{ detailItem.url }}</code>
        </div>

        <div>
          <label class="mb-1 block text-xs text-text-quaternary">
            引用来源
            <span v-if="refsLoading" class="ml-1">加载中...</span>
            <span v-else class="ml-1">({{ detailRefs.length }})</span>
          </label>
          <div v-if="detailRefs.length === 0 && !refsLoading" class="py-2 text-xs text-text-quaternary">无引用</div>
          <div v-else class="space-y-1.5">
            <div v-for="r in detailRefs" :key="r.type + r.slug" class="flex items-center gap-2 text-sm">
              <Icon :icon="r.type === 'post' ? 'ri:article-line' : 'ri:pages-line'" class="text-text-quaternary" />
              <span class="text-text-tertiary">{{ r.title || r.slug }}</span>
              <span
                class="rounded-[6px] bg-secondary px-1.5 py-0.5 text-[10px] text-text-quaternary"
              >{{ r.type === "post" ? "文章" : "页面" }}</span>
            </div>
          </div>
        </div>
      </template>
    </VModal>

    <!-- 删除确认弹窗 -->
    <VModal v-model:visible="deleteVisible" title="删除附件" :width="420">
      <template v-if="deleteTarget">
        <p class="mb-3 text-[13px] text-text-tertiary">
          确定删除「{{ deleteTarget.filename }}」？此操作不可恢复。
        </p>
        <div v-if="deleteRefsLoading" class="mb-3 text-xs text-text-quaternary">检查引用中...</div>
        <div
          v-else-if="deleteRefs.length > 0"
          class="mb-3 rounded-[10px] border border-[var(--color-destructive)]/30 bg-[var(--color-destructive)]/10 p-3"
        >
          <p class="mb-1.5 text-xs font-medium text-[var(--color-destructive)]">
            <Icon icon="ri:alert-line" class="mr-1 inline" />该附件被 {{ deleteRefs.length }} 处引用，删除后将导致裂图：
          </p>
          <ul class="space-y-0.5 text-xs text-[var(--color-destructive)]/90">
            <li v-for="r in deleteRefs" :key="r.type + r.slug">
              · {{ r.title || r.slug }}（{{ r.type === "post" ? "文章" : "页面" }}）
            </li>
          </ul>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="deleteVisible = false">取消</VButton>
          <VButton type="danger" @click="confirmDelete">确认删除</VButton>
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

/* ===== Page Header ===== */
.attachments-page {
  min-height: 100%;
  background-color: var(--color-background);
}
.attachments-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
  padding: 22px 28px 16px;
  background: linear-gradient(180deg, color-mix(in srgb, #000 90%, transparent) 0%, color-mix(in srgb, #000 60%, transparent) 80%, transparent 100%);
  backdrop-filter: saturate(180%) blur(16px);
  -webkit-backdrop-filter: saturate(180%) blur(16px);
  position: sticky;
  top: 0;
  z-index: 20;
}
.attachments-page-title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.8px;
  line-height: 1.15;
  color: #fff;
  margin: 0;
}
.attachments-page-subtitle {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}
.attachments-content-area {
  padding: 8px 28px 40px;
}

/* 上传按钮 */
.apple-upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 9px 20px;
  border-radius: var(--radius-full, 9999px);
  background-color: var(--color-primary);
  color: var(--color-primary-foreground);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.1px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s var(--ease-standard);
}
.apple-upload-btn:hover {
  background-color: var(--color-primary-hover);
}
.apple-upload-btn:active {
  transform: translateY(0);
}

/* ===== Sidebar group item ===== */
.group-sidebar-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  padding: 8px 14px;
  margin: 1px 8px;
  border-radius: 9px;
  color: var(--color-text-tertiary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s var(--ease-standard);
}
.group-sidebar-item:hover {
  background-color: color-mix(in srgb, #fff 5%, transparent);
  color: var(--color-foreground);
}
.group-sidebar-item.active {
  background-color: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  font-weight: 600;
}

/* ===== Storage Card ===== */
.storage-card {
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 20px 22px;
  margin-bottom: 16px;
  position: relative;
  overflow: hidden;
}
.storage-title {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
}
.storage-info {
  font-size: 12.5px;
  color: var(--color-text-tertiary);
  margin-top: 3px;
}
.storage-info strong {
  color: #fff;
  font-weight: 600;
}
.storage-progress {
  width: 100%;
  height: 8px;
  background-color: var(--color-accent);
  border-radius: 4px;
  overflow: hidden;
  margin: 14px 0;
}
.storage-progress-fill {
  height: 100%;
  border-radius: 4px;
  transition: width var(--duration-slow) var(--ease-standard);
}
.storage-breakdown {
  display: flex;
  align-items: center;
  gap: 22px;
  flex-wrap: wrap;
}
.storage-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.storage-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.storage-label {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.storage-label strong {
  color: color-mix(in srgb, #F5F5F7 88%, transparent);
  font-weight: 600;
  margin-right: 4px;
}

/* ===== Toolbar ===== */
.attachments-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.apple-search-compact {
  height: 34px;
  width: 220px;
  padding: 0 12px;
}

/* 视图切换 */
.apple-view-toggle {
  display: inline-flex;
  align-items: center;
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-full, 9999px);
  padding: 3px;
  gap: 2px;
}
.apple-view-toggle button {
  width: 30px;
  height: 30px;
  border-radius: var(--radius-full, 9999px);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-tertiary);
  transition: all var(--duration-fast) var(--ease-standard);
}
.apple-view-toggle button:hover {
  color: #fff;
}
.apple-view-toggle button.active {
  background-color: var(--color-accent);
  color: #fff;
}
.apple-view-toggle :deep(svg) {
  width: 16px;
  height: 16px;
}

/* Icon button (storage refresh) */
.apple-icon-btn {
  width: 34px;
  height: 34px;
  border-radius: var(--radius-base);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  transition: all var(--duration-fast) var(--ease-standard);
}
.apple-icon-btn:hover {
  background-color: var(--color-secondary);
  color: #fff;
  border-color: var(--color-border-strong, var(--color-border));
}
.apple-icon-btn :deep(svg) {
  width: 16px;
  height: 16px;
}

/* ===== Attachment Grid ===== */
.attachment-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 1200px) {
  .attachment-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 900px) {
  .attachment-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 600px) {
  .attachment-grid {
    grid-template-columns: 1fr;
  }
  .apple-search-compact {
    width: 100%;
  }
}

.attachment-card {
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  overflow: hidden;
  transition: all 0.25s var(--ease-standard);
  position: relative;
  cursor: pointer;
}
.attachment-card:hover {
  border-color: var(--color-border);
  transform: translateY(-2px);
  box-shadow: 0 12px 32px -8px color-mix(in srgb, #000 60%, transparent);
}
.attachment-card.selected {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 25%, transparent);
}
.attachment-preview {
  aspect-ratio: 4 / 3;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--color-secondary);
}
.attachment-preview-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.attachment-preview-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.preview-ext {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.8px;
  color: color-mix(in srgb, #fff 90%, transparent);
  background: color-mix(in srgb, #000 25%, transparent);
  padding: 3px 10px;
  border-radius: 9999px;
  font-family: "SF Mono", "JetBrains Mono", monospace;
}
.play-button {
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: color-mix(in srgb, #000 50%, transparent);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, #fff 15%, transparent);
  color: #fff;
}
.play-button :deep(svg) {
  width: 22px;
  height: 22px;
}

.attachment-checkbox {
  position: absolute;
  left: 10px;
  top: 10px;
  z-index: 4;
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--color-primary);
}

.attachment-hover-overlay {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, #000 55%, transparent);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  opacity: 0;
  transition: opacity 0.2s var(--ease-standard);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  z-index: 3;
}
.attachment-card:hover .attachment-hover-overlay {
  opacity: 1;
}
.attachment-action {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: color-mix(in srgb, #fff 15%, transparent);
  border: 1px solid color-mix(in srgb, #fff 20%, transparent);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  cursor: pointer;
  transition: all var(--duration-normal) var(--ease-standard);
}
.attachment-action:hover {
  background: color-mix(in srgb, #fff 28%, transparent);
  transform: scale(1.08);
}
.attachment-action.delete:hover {
  background: color-mix(in srgb, var(--color-state-error) 35%, transparent);
  border-color: color-mix(in srgb, var(--color-state-error) 50%, transparent);
}
.attachment-action :deep(svg) {
  width: 18px;
  height: 18px;
}

.attachment-info {
  padding: 12px 14px 14px;
}
.attachment-name {
  font-size: 13px;
  font-weight: 500;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 4px;
}
.attachment-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: var(--color-text-tertiary);
}

/* ===== Attachment List ===== */
.attachment-list {
  width: 100%;
}
.attachment-list-head,
.attachment-list-row {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr) 100px 90px 130px 130px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
}
.attachment-list-head {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--color-border);
  background-color: var(--color-secondary);
}
.attachment-list-row {
  border-bottom: 1px solid var(--color-border);
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard);
}
.attachment-list-row:last-child {
  border-bottom: none;
}
.attachment-list-row:hover {
  background-color: color-mix(in srgb, #fff 3%, transparent);
}
.attachment-list-row.selected {
  background-color: color-mix(in srgb, var(--color-primary) 8%, transparent);
}
.col-name {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.col-actions {
  justify-content: flex-end;
}
.apple-icon-btn-sm {
  width: 30px;
  height: 30px;
  border-radius: var(--radius-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-tertiary);
  transition: all var(--duration-fast) var(--ease-standard);
}
.apple-icon-btn-sm:hover {
  background-color: var(--color-accent);
  color: var(--color-foreground);
}
.apple-icon-btn-sm.danger:hover {
  background-color: color-mix(in srgb, var(--color-state-error) 15%, transparent);
  color: var(--color-destructive);
}
.apple-icon-btn-sm :deep(svg) {
  width: 16px;
  height: 16px;
}

@media (max-width: 900px) {
  .attachment-list-head,
  .attachment-list-row {
    grid-template-columns: 32px minmax(0, 1fr) 80px 100px;
  }
  .col-type,
  .col-time,
  .col-actions {
    display: none;
  }
}

/* ===== 移动端补充优化 ===== */
@media (max-width: 640px) {
  /* 类型分段 Tab 横向滚动，不显示滚动条 */
  .tabs-scroll {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
    flex-shrink: 0;
  }
  .tabs-scroll::-webkit-scrollbar {
    display: none;
  }

  /* 工具栏更紧凑 */
  .attachments-toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
  .attachments-toolbar > :last-child {
    justify-content: space-between;
  }
  .apple-search-compact {
    width: 100% !important;
  }

  /* 存储卡更紧凑 */
  .storage-title {
    font-size: 13px;
  }
  .storage-info {
    font-size: 11px;
  }
  .storage-breakdown {
    flex-wrap: wrap;
    gap: 8px 14px;
  }
  .storage-label {
    font-size: 11px;
  }

  /* 列表视图在 640px 以下：更紧凑，去掉 size 列 */
  .attachment-list-head,
  .attachment-list-row {
    grid-template-columns: 32px minmax(0, 1fr) !important;
  }
  .col-size {
    display: none;
  }
}

/* ===== 移动端分组栏：横向滚动胶囊 ===== */
.group-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.group-scroll::-webkit-scrollbar {
  display: none;
}
.group-scroll ul {
  display: flex;
  gap: 6px;
  padding: 8px;
  list-style: none;
}
.group-sidebar-item-mobile {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  background: var(--color-secondary);
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: all 0.15s ease;
  cursor: pointer;
  border: 1px solid transparent;
}
.group-sidebar-item-mobile:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}
.group-sidebar-item-mobile.active {
  background: color-mix(in srgb, var(--color-primary) 18%, transparent);
  color: var(--color-primary);
  border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
  font-weight: 600;
}
</style>
