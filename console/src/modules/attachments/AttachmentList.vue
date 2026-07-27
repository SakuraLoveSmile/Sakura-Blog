<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { attachmentApi } from "@/api";
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

const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (debouncedKeyword.value.trim()) params.keyword = debouncedKeyword.value.trim();
  if (kindFilter.value) params.kind = kindFilter.value;
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

async function handleUpload(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  uploading.value = true;
  try {
    await attachmentApi.upload(input.files[0]);
    queryClient.invalidateQueries({ queryKey: ["attachments"] });
    Toast.success("上传成功");
  } catch {} finally {
    uploading.value = false;
    input.value = "";
  }
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
  queryClient.invalidateQueries({ queryKey: ["attachments"] });
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
            上传
          </VButton>
          <input type="file" accept="image/*" class="hidden" :disabled="uploading" @change="handleUpload" />
        </label>
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <!-- 筛选头部 -->
        <template #header>
          <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
            <div class="relative flex flex-col items-start sm:flex-row sm:items-center">
              <div class="flex w-full flex-1 items-center sm:w-auto">
                <div class="flex items-center gap-2">
                  <input
                    v-model="keyword"
                    placeholder="搜索文件名..."
                    type="text"
                    class="w-48 border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    style="border-radius: 4px"
                  />
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
              class="group relative cursor-pointer overflow-hidden rounded border border-gray-200 bg-white transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-800"
              style="border-radius: 4px"
              @click="openDetail(att)"
            >
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

    <!-- 详情弹窗 -->
    <VModal v-model:visible="detailVisible" :title="detailItem?.filename" :width="560">
      <template v-if="detailItem">
        <div class="mb-4 overflow-hidden rounded bg-gray-50 dark:bg-gray-700/50" style="border-radius: 4px">
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
            class="block select-all break-all rounded bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-gray-700/50 dark:text-gray-300"
            style="border-radius: 4px"
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
                class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-400 dark:bg-gray-700 dark:text-gray-500"
                style="border-radius: 4px"
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
          class="mb-3 rounded border border-red-200 bg-red-50 p-3 dark:border-red-500/30 dark:bg-red-500/10"
          style="border-radius: 4px"
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
