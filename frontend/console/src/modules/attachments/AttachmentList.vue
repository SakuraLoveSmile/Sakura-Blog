<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { attachmentApi, attachmentGroupApi } from "@/api";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  VModal,
  FilterDropdown,
  FilterTag,
  Dialog,
  Toast,
} from "@/components/ui";

const queryClient = useQueryClient();

// 搜索关键词（debounce 300ms）
const keyword = ref("");
const debouncedKeyword = ref("");
const page = ref(1);
const size = ref(24);
const uploading = ref(false);
const kindFilter = ref<string | undefined>(undefined);
const kindItems = [
  { label: "全部", value: undefined },
  { label: "仅图片", value: "image" },
  { label: "非图片", value: "other" },
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
// activeGroup: undefined=全部；0=未分组；>0=指定分组
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
  if (kindFilter.value) params.kind = kindFilter.value;
  if (activeGroup.value !== undefined) params.group = activeGroup.value;
  return params;
});

const { data: queryData, isLoading, refetch } = useQuery({
  queryKey: computed(() => queryKeys.attachments(queryParams.value)),
  queryFn: () => attachmentApi.list(queryParams.value).then(({ data }) => data),
});

const attachments = computed(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

function handleFilterChange() {
  page.value = 1;
}

function invalidateAll() {
  queryClient.invalidateQueries({ queryKey: ["attachments"] });
  loadGroups();
}

async function handleUpload(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  uploading.value = true;
  try {
    // 归入当前浏览分组（仅当为自定义分组时）
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

// --- 多选批量移动（复用 PostList 多选模式）---
const selectedIds = ref<number[]>([]);
const checkedAll = ref(false);

watch(selectedIds, (val) => {
  checkedAll.value = val.length === attachments.value.length && val.length > 0;
});

function handleCheckAllChange(e: Event) {
  const { checked } = e.target as HTMLInputElement;
  selectedIds.value = checked ? attachments.value.map((a: any) => a.id) : [];
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

function formatSize(bytes: number) {
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB";
  return (bytes / 1024).toFixed(1) + " KB";
}

function isImage(mime: string) {
  return mime?.startsWith("image/");
}

const kindLabel = ref("");
watch(kindFilter, (v) => {
  kindLabel.value = kindItems.find((i) => i.value === v)?.label || "";
});
</script>

<template>
  <div>
    <VPageHeader title="附件">
      <template #icon>
        <Icon icon="ri:folder-2-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <label class="inline-flex cursor-pointer items-center">
          <VButton type="secondary" :loading="uploading" @click.prevent>
            <template #icon>
              <Icon icon="ri:upload-2-line" class="h-full w-full" />
            </template>
            上传{{ activeGroup && activeGroup > 0 ? `到「${activeGroupName}」` : "" }}
          </VButton>
          <input type="file" accept="image/*" class="hidden" :disabled="uploading" @change="handleUpload" />
        </label>
      </template>
    </VPageHeader>

    <div class="m-0 flex flex-col gap-4 md:m-4 md:flex-row">
      <!-- 左侧分组栏 -->
      <div class="w-full shrink-0 md:w-56">
        <VCard :body-class="['!p-0']">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-gray-700">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-200">分组</span>
            <button
              class="rounded p-1 text-gray-500 hover:bg-gray-100 hover:text-primary dark:text-gray-400 dark:hover:bg-gray-700"
              title="新建分组"
              @click="openCreateGroup"
            >
              <Icon icon="ri:add-line" />
            </button>
          </div>
          <ul class="py-1 text-sm">
            <li>
              <button
                class="flex w-full items-center justify-between px-4 py-2 text-left transition-colors"
                :class="activeGroup === undefined ? 'bg-primary/10 text-primary' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50'"
                @click="selectGroup(undefined)"
              >
                <span class="flex items-center gap-2"><Icon icon="ri:apps-line" />全部</span>
              </button>
            </li>
            <li>
              <button
                class="flex w-full items-center justify-between px-4 py-2 text-left transition-colors"
                :class="activeGroup === 0 ? 'bg-primary/10 text-primary' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50'"
                @click="selectGroup(0)"
              >
                <span class="flex items-center gap-2"><Icon icon="ri:inbox-line" />未分组</span>
              </button>
            </li>
            <li v-for="g in groups" :key="g.id" class="group/item">
              <div
                class="flex w-full items-center justify-between px-4 py-2 transition-colors"
                :class="activeGroup === g.id ? 'bg-primary/10 text-primary' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50'"
              >
                <button class="flex min-w-0 flex-1 items-center gap-2 text-left" @click="selectGroup(g.id)">
                  <Icon icon="ri:folder-line" class="shrink-0" />
                  <span class="truncate">{{ g.name }}</span>
                  <span class="text-[10px] text-gray-400">{{ g.count }}</span>
                </button>
                <span class="ml-1 hidden shrink-0 gap-1 group-hover/item:flex">
                  <button
                    class="rounded p-0.5 text-gray-400 hover:text-primary"
                    title="重命名"
                    @click.stop="openRenameGroup(g)"
                  ><Icon icon="ri:edit-line" class="text-xs" /></button>
                  <button
                    class="rounded p-0.5 text-gray-400 hover:text-red-500"
                    title="删除"
                    @click.stop="requestDeleteGroup(g)"
                  ><Icon icon="ri:delete-bin-line" class="text-xs" /></button>
                </span>
              </div>
            </li>
          </ul>
        </VCard>
      </div>

      <!-- 右侧内容区 -->
      <div class="min-w-0 flex-1">
        <VCard :body-class="['!p-0']">
          <!-- 筛选头部 -->
          <template #header>
            <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
              <div class="relative flex flex-col items-start sm:flex-row sm:items-center">
                <!-- 批量操作条 -->
                <div v-if="selectedIds.length" class="flex w-full flex-1 items-center gap-3">
                  <span class="text-sm text-gray-500 dark:text-gray-400">已选择 {{ selectedIds.length }} 项</span>
                  <VButton size="sm" type="secondary" @click="moveVisible = true">
                    <template #icon><Icon icon="ri:folder-transfer-line" class="h-full w-full" /></template>
                    移动到分组
                  </VButton>
                  <button class="text-sm text-gray-400 hover:text-gray-600" @click="clearSelection">取消选择</button>
                </div>
                <!-- 常规筛选 -->
                <div v-else class="flex w-full flex-1 items-center sm:w-auto">
                  <div class="flex items-center gap-2">
                    <input
                      v-model="keyword"
                      placeholder="搜索文件名..."
                      type="text"
                      class="w-48 rounded-base border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    />
                    <FilterTag v-if="activeGroup !== undefined" @close="selectGroup(undefined)">
                      分组：{{ activeGroupName }}
                    </FilterTag>
                    <FilterTag v-if="keyword" @close="keyword = ''">关键词：{{ keyword }}</FilterTag>
                    <FilterTag v-if="kindFilter !== undefined" @close="kindFilter = undefined; handleFilterChange()">
                      类型：{{ kindLabel }}
                    </FilterTag>
                    <span class="text-xs text-gray-400 dark:text-gray-500">共 {{ total }} 项</span>
                  </div>
                </div>
                <div class="mt-4 flex sm:mt-0">
                  <div class="flex items-center gap-6">
                    <FilterDropdown v-model="kindFilter" :items="kindItems" label="类型" @change="handleFilterChange" />
                    <div class="group cursor-pointer rounded p-1 hover:bg-gray-200 dark:hover:bg-gray-600" @click="refetch()">
                      <Icon
                        icon="ri:refresh-line"
                        :class="{ 'animate-spin text-gray-900 dark:text-gray-100': isLoading }"
                        class="h-4 w-4 text-gray-600 group-hover:text-gray-900 dark:text-gray-300 dark:group-hover:text-gray-100"
                      />
                    </div>
                  </div>
                </div>
              </div>
              <!-- 全选行 -->
              <div v-if="attachments.length" class="mt-3 flex items-center gap-2 border-t border-gray-200 pt-2 dark:border-gray-600">
                <input type="checkbox" :checked="checkedAll" class="cursor-pointer" @change="handleCheckAllChange" />
                <span class="text-xs text-gray-400 dark:text-gray-500">全选本页</span>
              </div>
            </div>
          </template>

          <VLoading v-if="isLoading" />

          <Transition v-else-if="!attachments.length" appear name="fade">
            <VEmpty message="你可以尝试刷新或者上传附件" title="当前没有附件">
              <template #actions>
                <VButton @click="refetch">刷新</VButton>
              </template>
            </VEmpty>
          </Transition>

          <!-- Grid -->
          <Transition v-else appear name="fade">
            <div class="grid grid-cols-2 gap-4 p-4 md:grid-cols-4 lg:grid-cols-6">
              <div
                v-for="att in attachments"
                :key="att.id"
                class="group relative cursor-pointer overflow-hidden rounded-base border bg-white transition-shadow hover:shadow-md dark:bg-gray-800"
                :class="checkSelection(att.id) ? 'border-primary ring-2 ring-primary/30' : 'border-gray-200 dark:border-gray-700'"
                @click="openDetail(att)"
              >
                <!-- 选择框 -->
                <input
                  type="checkbox"
                  :checked="checkSelection(att.id)"
                  class="absolute left-2 top-2 z-10 cursor-pointer"
                  @click.stop="toggleSelection(att.id)"
                />
                <div class="flex aspect-square items-center justify-center overflow-hidden bg-gray-50 dark:bg-gray-700/50">
                  <img
                    v-if="isImage(att.mime)"
                    :src="att.thumb_url || att.url"
                    :alt="att.filename"
                    class="h-full w-full object-cover"
                    loading="lazy"
                  />
                  <div v-else class="flex flex-col items-center gap-1 text-gray-300 dark:text-gray-600">
                    <Icon icon="ri:file-3-line" class="text-4xl" />
                    <span class="text-[10px] uppercase">{{ att.mime?.split("/")[1] || "file" }}</span>
                  </div>
                </div>
                <div class="p-2">
                  <p class="truncate text-xs text-gray-600 dark:text-gray-300">{{ att.filename }}</p>
                  <div class="mt-0.5 flex items-center gap-2">
                    <span class="text-[10px] text-gray-400 dark:text-gray-500">{{ formatSize(att.size) }}</span>
                    <span v-if="att.width && att.height" class="text-[10px] text-gray-400 dark:text-gray-500">
                      {{ att.width }}×{{ att.height }}
                    </span>
                  </div>
                </div>
                <div class="absolute right-2 top-2 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                  <button
                    class="flex h-6 w-6 items-center justify-center rounded-full bg-red-500 text-xs text-white hover:bg-red-600"
                    @click.stop="requestDelete(att)"
                  >
                    <Icon icon="ri:delete-bin-line" />
                  </button>
                </div>
              </div>
            </div>
          </Transition>

          <template #footer>
            <div class="bg-white sm:flex sm:items-center sm:justify-end dark:bg-gray-800">
              <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[24, 48, 96]" />
            </div>
          </template>
        </VCard>
      </div>
    </div>

    <!-- 分组新建/重命名弹窗 -->
    <VModal v-model:visible="groupModalVisible" :title="groupModalMode === 'create' ? '新建分组' : '重命名分组'" :width="380">
      <input
        v-model="groupModalName"
        placeholder="分组名称"
        type="text"
        class="w-full rounded-base border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
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
      <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">将选中的 {{ selectedIds.length }} 项移动到：</p>
      <ul class="space-y-1 text-sm">
        <li>
          <button
            class="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50"
            @click="moveTo(0)"
          ><Icon icon="ri:inbox-line" />未分组</button>
        </li>
        <li v-for="g in groups" :key="g.id">
          <button
            class="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50"
            @click="moveTo(g.id)"
          ><Icon icon="ri:folder-line" />{{ g.name }}</button>
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
        <div class="mb-4 overflow-hidden rounded-base bg-gray-50 dark:bg-gray-700/50">
          <img
            v-if="isImage(detailItem.mime)"
            :src="detailItem.url"
            :alt="detailItem.filename"
            class="max-h-64 w-full object-contain"
          />
          <div v-else class="flex flex-col items-center py-12 text-gray-300 dark:text-gray-600">
            <Icon icon="ri:file-3-line" class="text-5xl" />
          </div>
        </div>

        <div class="mb-4 grid grid-cols-2 gap-3 text-sm">
          <div>
            <span class="text-gray-400 dark:text-gray-500">格式：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ detailItem.mime }}</span>
          </div>
          <div>
            <span class="text-gray-400 dark:text-gray-500">大小：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ formatSize(detailItem.size) }}</span>
          </div>
          <div v-if="detailItem.width">
            <span class="text-gray-400 dark:text-gray-500">尺寸：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ detailItem.width }}×{{ detailItem.height }} px</span>
          </div>
          <div>
            <span class="text-gray-400 dark:text-gray-500">上传：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ new Date(detailItem.created_at).toLocaleDateString() }}</span>
          </div>
        </div>

        <div class="mb-4">
          <label class="mb-1 block text-xs text-gray-400 dark:text-gray-500">引用 URL</label>
          <code
            class="block select-all break-all rounded-base bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-gray-700/50 dark:text-gray-300"
          >{{ detailItem.url }}</code>
        </div>

        <div>
          <label class="mb-1 block text-xs text-gray-400 dark:text-gray-500">
            引用来源
            <span v-if="refsLoading" class="ml-1">加载中...</span>
            <span v-else class="ml-1">({{ detailRefs.length }})</span>
          </label>
          <div v-if="detailRefs.length === 0 && !refsLoading" class="py-2 text-xs text-gray-400 dark:text-gray-500">无引用</div>
          <div v-else class="space-y-1.5">
            <div v-for="r in detailRefs" :key="r.type + r.slug" class="flex items-center gap-2 text-sm">
              <Icon :icon="r.type === 'post' ? 'ri:article-line' : 'ri:pages-line'" class="text-gray-400" />
              <span class="text-gray-600 dark:text-gray-300">{{ r.title || r.slug }}</span>
              <span
                class="rounded-base bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-400 dark:bg-gray-700 dark:text-gray-500"
              >{{ r.type === "post" ? "文章" : "页面" }}</span>
            </div>
          </div>
        </div>
      </template>
    </VModal>

    <!-- 删除确认弹窗 -->
    <VModal v-model:visible="deleteVisible" title="删除附件" :width="420">
      <template v-if="deleteTarget">
        <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">
          确定删除「{{ deleteTarget.filename }}」？此操作不可恢复。
        </p>
        <div v-if="deleteRefsLoading" class="mb-3 text-xs text-gray-400">检查引用中...</div>
        <div
          v-else-if="deleteRefs.length > 0"
          class="mb-3 rounded-base border border-red-200 bg-red-50 p-3 dark:border-red-500/30 dark:bg-red-500/10"
        >
          <p class="mb-1.5 text-xs font-medium text-red-600 dark:text-red-400">
            <Icon icon="ri:alert-line" class="mr-1 inline" />该附件被 {{ deleteRefs.length }} 处引用，删除后将导致裂图：
          </p>
          <ul class="space-y-0.5 text-xs text-red-500 dark:text-red-400">
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
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
