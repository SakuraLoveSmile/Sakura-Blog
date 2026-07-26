<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { pageApi } from "@/api";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEntity,
  VEntityField,
  VEmpty,
  VLoading,
  VPagination,
  VStatusDot,
  FilterDropdown,
  FilterTag,
  Dialog,
  Toast,
} from "@/components/ui";

const router = useRouter();
const pages = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const size = ref(20);
const loading = ref(false);
const isFetching = ref(false);

// Filters
const keyword = ref("");
const selectedStatus = ref<string | undefined>(undefined);
const selectedSort = ref<string | undefined>(undefined);

const statusItems = [
  { label: "全部", value: undefined },
  { label: "已发布", value: "published" },
  { label: "草稿", value: "draft" },
  { label: "回收站", value: "trashed" },
];

const sortItems = [
  { label: "默认", value: undefined },
  { label: "较近更新", value: "newest" },
  { label: "较早更新", value: "oldest" },
];

// Selection
const checkedAll = ref(false);
const selectedSlugs = ref<string[]>([]);
const menuOpenSlug = ref<string | null>(null);

const hasFilters = computed(
  () => selectedStatus.value !== undefined || selectedSort.value !== undefined || keyword.value !== ""
);

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    page.value = 1;
    loadPages();
  }, 300);
});

async function loadPages() {
  loading.value = true;
  isFetching.value = true;
  try {
    const params: Record<string, unknown> = { page: page.value, size: size.value };
    if (selectedStatus.value) params.status = selectedStatus.value;
    if (keyword.value.trim()) params.search = keyword.value.trim();
    const { data } = await pageApi.list(params);
    let items = data.items || [];
    if (selectedSort.value === "newest") {
      items = [...items].sort((a: any, b: any) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
    } else if (selectedSort.value === "oldest") {
      items = [...items].sort((a: any, b: any) => new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime());
    }
    pages.value = items;
    total.value = data.total || 0;
  } catch {} finally {
    loading.value = false;
    isFetching.value = false;
  }
}

function refetch() {
  loadPages();
}

// Selection
function handleCheckAllChange(e: Event) {
  const { checked } = e.target as HTMLInputElement;
  selectedSlugs.value = checked ? pages.value.map((p) => p.slug) : [];
}

watch(selectedSlugs, (val) => {
  checkedAll.value = val.length === pages.value.length && val.length > 0;
});

function checkSelection(slug: string) {
  return selectedSlugs.value.includes(slug);
}

function toggleSelection(slug: string) {
  if (checkSelection(slug)) {
    selectedSlugs.value = selectedSlugs.value.filter((s) => s !== slug);
  } else {
    selectedSlugs.value = [...selectedSlugs.value, slug];
  }
}

// Filter handlers
function handleFilterChange() {
  page.value = 1;
  loadPages();
}

function handleClearFilters() {
  selectedStatus.value = undefined;
  selectedSort.value = undefined;
  keyword.value = "";
  page.value = 1;
  loadPages();
}

const statusLabel = computed(() => statusItems.find((i) => i.value === selectedStatus.value)?.label || "");
const sortLabel = computed(() => sortItems.find((i) => i.value === selectedSort.value)?.label || "");

const isTrashed = computed(() => selectedStatus.value === "trashed");

// Actions
function handleDelete(item: any) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要删除该页面吗？",
    description: "该操作会将页面放入回收站，后续可以从回收站恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await pageApi.delete(item.slug);
      await loadPages();
      Toast.success("删除成功");
    },
  });
}

function handleDeleteInBatch() {
  Dialog.warning({
    title: "确定要删除选中的页面吗？",
    description: "该操作会将页面放入回收站，后续可以从回收站恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await Promise.all(selectedSlugs.value.map((slug) => pageApi.delete(slug)));
      selectedSlugs.value = [];
      await loadPages();
      Toast.success("删除成功");
    },
  });
}

function handleRestore(item: any) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要恢复该页面吗？",
    onConfirm: async () => {
      await pageApi.restore(item.slug);
      await loadPages();
      Toast.success("恢复成功");
    },
  });
}

function handlePurge(item: any) {
  menuOpenSlug.value = null;
  Dialog.warning({
    title: "确定要彻底删除该页面吗？",
    description: "该操作不可恢复！",
    confirmType: "danger",
    onConfirm: async () => {
      await pageApi.purge(item.slug);
      await loadPages();
      Toast.success("已彻底删除");
    },
  });
}

function formatDate(dateStr: string): string {
  if (!dateStr) return "";
  return dateStr.slice(0, 10);
}

function openPage(slug: string) {
  window.open(`/${slug}`, "_blank");
}

function closeMenus() {
  menuOpenSlug.value = null;
}

onMounted(loadPages);
</script>

<template>
  <div @click="closeMenus">
    <VPageHeader title="页面">
      <template #icon>
        <Icon icon="ri:pages-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <VButton type="secondary" @click="router.push('/pages/edit/new')">
          <template #icon>
            <Icon icon="ri:add-circle-line" class="h-full w-full" />
          </template>
          新建
        </VButton>
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <template #header>
          <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
            <div class="relative flex flex-col items-start sm:flex-row sm:items-center">
              <div class="mr-4 hidden items-center sm:flex">
                <input
                  :checked="checkedAll"
                  class="h-4 w-4 rounded border-gray-300 text-primary"
                  type="checkbox"
                  @change="handleCheckAllChange"
                />
              </div>
              <div class="flex w-full flex-1 items-center sm:w-auto">
                <div v-if="!selectedSlugs.length" class="flex items-center gap-2">
                  <input
                    v-model="keyword"
                    placeholder="输入关键词搜索"
                    type="text"
                    class="w-48 border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                    style="border-radius: 4px"
                  />
                  <FilterTag v-if="keyword" @close="keyword = ''">关键词：{{ keyword }}</FilterTag>
                  <FilterTag v-if="selectedStatus !== undefined" @close="selectedStatus = undefined; handleFilterChange()">
                    状态：{{ statusLabel }}
                  </FilterTag>
                  <FilterTag v-if="selectedSort !== undefined" @close="selectedSort = undefined; handleFilterChange()">
                    排序：{{ sortLabel }}
                  </FilterTag>
                  <button
                    v-if="hasFilters"
                    class="text-xs text-gray-400 underline hover:text-gray-600 dark:hover:text-gray-300"
                    @click="handleClearFilters"
                  >
                    清除筛选
                  </button>
                </div>
                <div v-else class="flex items-center gap-3">
                  <VButton type="danger" size="sm" @click="handleDeleteInBatch">删除</VButton>
                  <span class="text-sm text-gray-500 dark:text-gray-400">已选择 {{ selectedSlugs.length }} 项</span>
                </div>
              </div>
              <div class="mt-4 flex sm:mt-0">
                <div class="flex items-center gap-6">
                  <FilterDropdown v-model="selectedStatus" :items="statusItems" label="状态" @change="handleFilterChange" />
                  <FilterDropdown v-model="selectedSort" :items="sortItems" label="排序" @change="handleFilterChange" />
                  <div class="group cursor-pointer rounded p-1 hover:bg-gray-200 dark:hover:bg-gray-600" @click="refetch()">
                    <Icon
                      icon="ri:refresh-line"
                      :class="{ 'animate-spin text-gray-900 dark:text-gray-100': isFetching }"
                      class="h-4 w-4 text-gray-600 group-hover:text-gray-900 dark:text-gray-300 dark:group-hover:text-gray-100"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>

        <VLoading v-if="loading" />

        <Transition v-else-if="!pages.length" appear name="fade">
          <VEmpty message="你可以尝试刷新或者新建页面" title="当前没有页面">
            <template #actions>
              <div class="flex items-center gap-2">
                <VButton @click="refetch">刷新</VButton>
                <VButton type="primary" @click="router.push('/pages/edit/new')">
                  <template #icon>
                    <Icon icon="ri:add-circle-line" class="h-full w-full" />
                  </template>
                  新建页面
                </VButton>
              </div>
            </template>
          </VEmpty>
        </Transition>

        <Transition v-else appear name="fade">
          <ul class="box-border h-full w-full divide-y divide-gray-100 dark:divide-gray-700" role="list">
            <li v-for="p in pages" :key="p.slug">
              <VEntity :is-selected="checkSelection(p.slug)">
                <template #checkbox>
                  <input
                    :checked="checkSelection(p.slug)"
                    class="h-4 w-4 rounded border-gray-300 text-primary"
                    type="checkbox"
                    @change="toggleSelection(p.slug)"
                  />
                </template>
                <template #start>
                  <VEntityField :title="p.title || p.slug" width="27rem">
                    <template #title>
                      <div class="flex items-center gap-1.5">
                        <span
                          class="truncate text-sm font-medium text-gray-900 hover:text-blue-600 dark:text-gray-100 dark:hover:text-blue-400"
                          @click.stop="router.push(`/pages/edit/${p.slug}`)"
                        >
                          {{ p.title || p.slug }}
                        </span>
                        <span
                          class="hidden cursor-pointer text-gray-600 transition-all hover:text-gray-900 group-hover:inline-block dark:text-gray-400 dark:hover:text-gray-200"
                          title="在前台打开"
                          @click.stop="openPage(p.slug)"
                        >
                          <Icon icon="ri:external-link-line" class="h-3.5 w-3.5" />
                        </span>
                      </div>
                    </template>
                    <template #description>
                      <span class="text-xs text-gray-500 dark:text-gray-400">Slug：{{ p.slug }}</span>
                    </template>
                  </VEntityField>
                </template>
                <template #end>
                  <VEntityField v-if="p.status === 'draft'">
                    <template #description>
                      <VStatusDot state="warning" text="未发布" />
                    </template>
                  </VEntityField>
                  <VEntityField v-else-if="p.status === 'trashed'">
                    <template #description>
                      <VStatusDot state="error" text="已删除" />
                    </template>
                  </VEntityField>
                  <VEntityField v-else description="已发布" />
                  <VEntityField>
                    <template #description>
                      <span class="truncate text-xs tabular-nums text-gray-500 dark:text-gray-400">
                        {{ formatDate(p.updated_at) }}
                      </span>
                    </template>
                  </VEntityField>
                </template>
                <template #dropdownItems>
                  <div class="relative">
                    <div
                      class="cursor-pointer rounded p-1 transition-all hover:bg-gray-200/60 hover:text-blue-600"
                      @click.stop="menuOpenSlug = menuOpenSlug === p.slug ? null : p.slug"
                    >
                      <Icon icon="ri:more-fill" />
                    </div>
                    <Transition name="fade">
                      <div
                        v-if="menuOpenSlug === p.slug"
                        class="absolute right-0 top-full z-30 mt-1 w-32 rounded border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
                        style="border-radius: 4px"
                        @click.stop
                      >
                        <template v-if="isTrashed">
                          <VButton block type="secondary" size="sm" @click="handleRestore(p)">恢复</VButton>
                          <div class="mt-2">
                            <VButton block type="danger" size="sm" @click="handlePurge(p)">彻底删除</VButton>
                          </div>
                        </template>
                        <template v-else>
                          <VButton block type="secondary" size="sm" @click="menuOpenSlug = null; router.push(`/pages/edit/${p.slug}`)">
                            编辑
                          </VButton>
                          <div class="mt-2">
                            <VButton block type="danger" size="sm" @click="handleDelete(p)">删除</VButton>
                          </div>
                        </template>
                      </div>
                    </Transition>
                  </div>
                </template>
              </VEntity>
            </li>
          </ul>
        </Transition>

        <template #footer>
          <div class="bg-white sm:flex sm:items-center sm:justify-end dark:bg-gray-800">
            <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" @change="loadPages" />
          </div>
        </template>
      </VCard>
    </div>
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
