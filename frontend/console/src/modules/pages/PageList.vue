<script setup lang="ts">
import { ref, computed, watch } from"vue";
import { useRouter, useRoute } from"vue-router";
import { useQuery, useQueryClient } from"@tanstack/vue-query";
import { pageApi } from"@/api";
import { queryKeys } from"@/api/query-client";
import { Icon } from"@iconify/vue";
import {
 VPageHeader,
 VCard,
 VButton,
 VEntityContainer,
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
} from"@/components/ui";

const router = useRouter();
const route = useRoute();
const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

// 搜索关键词（debounce 300ms）
const keyword = ref("");
const debouncedKeyword = ref("");
let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
 if (debounceTimer) clearTimeout(debounceTimer);
 debounceTimer = setTimeout(() => {
 debouncedKeyword.value = keyword.value;
 page.value = 1;
 }, 300);
});

// Filters
// 状态筛选支持 ?status= 查询参数预置（Spec 39：⌘K 搜索回收站项跳转带筛选）
const validStatuses = ["published","draft","trashed"];
function statusFromQuery(): string | undefined {
 const s = route.query.status;
 return typeof s ==="string" && validStatuses.includes(s) ? s : undefined;
}
const selectedStatus = ref<string | undefined>(statusFromQuery());
watch(
 () => route.query.status,
 () => {
 const s = statusFromQuery();
 if (s && s !== selectedStatus.value) {
 selectedStatus.value = s;
 page.value = 1;
 }
 }
);
const selectedSort = ref<string | undefined>(undefined);

const statusItems = [
 { label:"全部", value: undefined },
 { label:"已发布", value:"published" },
 { label:"草稿", value:"draft" },
 { label:"回收站", value:"trashed" },
];

const sortItems = [
 { label:"默认", value: undefined },
 { label:"较近更新", value:"newest" },
 { label:"较早更新", value:"oldest" },
];

const queryParams = computed(() => {
 const params: Record<string, unknown> = { page: page.value, size: size.value };
 if (selectedStatus.value) params.status = selectedStatus.value;
 if (debouncedKeyword.value.trim()) params.search = debouncedKeyword.value.trim();
 return params;
});

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
 queryKey: computed(() => queryKeys.pages(queryParams.value)),
 queryFn: () => pageApi.list(queryParams.value).then(({ data }) => data),
});

// 客户端排序（排序变化不触发网络请求）
const pages = computed(() => {
 const items = [...(queryData.value?.items || [])];
 if (selectedSort.value ==="newest") {
 items.sort((a: any, b: any) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
 } else if (selectedSort.value ==="oldest") {
 items.sort((a: any, b: any) => new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime());
 }
 return items;
});
const total = computed(() => queryData.value?.total || 0);

// Selection
const checkedAll = ref(false);
const selectedSlugs = ref<string[]>([]);
const menuOpenSlug = ref<string | null>(null);

const hasFilters = computed(
 () => selectedStatus.value !== undefined || selectedSort.value !== undefined || keyword.value !==""
);

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

// 筛选变化时重置到第一页
function handleFilterChange() {
 page.value = 1;
}

function handleClearFilters() {
 selectedStatus.value = undefined;
 selectedSort.value = undefined;
 keyword.value ="";
 debouncedKeyword.value ="";
 page.value = 1;
}

const statusLabel = computed(() => statusItems.find((i) => i.value === selectedStatus.value)?.label ||"");
const sortLabel = computed(() => sortItems.find((i) => i.value === selectedSort.value)?.label ||"");

const isTrashed = computed(() => selectedStatus.value ==="trashed");

// Actions
function handleDelete(item: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要删除该页面吗？",
 description:"该操作会将页面放入回收站，后续可以从回收站恢复",
 confirmType:"danger",
 onConfirm: async () => {
 await pageApi.delete(item.slug);
 queryClient.invalidateQueries({ queryKey: ["pages"] });
 Toast.success("删除成功");
 },
 });
}

function handleDeleteInBatch() {
 Dialog.warning({
 title:"确定要删除选中的页面吗？",
 description:"该操作会将页面放入回收站，后续可以从回收站恢复",
 confirmType:"danger",
 onConfirm: async () => {
 await Promise.all(selectedSlugs.value.map((slug) => pageApi.delete(slug)));
 selectedSlugs.value = [];
 queryClient.invalidateQueries({ queryKey: ["pages"] });
 Toast.success("删除成功");
 },
 });
}

function handleRestore(item: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要恢复该页面吗？",
 onConfirm: async () => {
 await pageApi.restore(item.slug);
 queryClient.invalidateQueries({ queryKey: ["pages"] });
 Toast.success("恢复成功");
 },
 });
}

function handlePurge(item: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要彻底删除该页面吗？",
 description:"该操作不可恢复！",
 confirmType:"danger",
 onConfirm: async () => {
 await pageApi.purge(item.slug);
 queryClient.invalidateQueries({ queryKey: ["pages"] });
 Toast.success("已彻底删除");
 },
 });
}

function formatDate(dateStr: string): string {
 if (!dateStr) return"";
 return dateStr.slice(0, 10);
}

function openPage(slug: string) {
 window.open(`/${slug}`,"_blank");
}

function closeMenus() {
 menuOpenSlug.value = null;
}
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
 <div class="block w-full bg-secondary px-4 py-3">
 <div class="relative flex flex-col items-start sm:flex-row sm:items-center">
 <div class="mr-4 hidden items-center sm:flex">
 <input
 :checked="checkedAll"
 class="h-4 w-4 rounded border-[var(--color-border-strong)] text-primary"
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
 class="w-48 rounded-base border border-[var(--color-border-strong)] bg-card px-3 py-1.5 text-sm text-foreground outline-none transition-colors focus:border-primary"
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
 class="text-xs text-text-quaternary underline hover:text-text-tertiary hover:text-foreground"
 @click="handleClearFilters"
 >
 清除筛选
 </button>
 </div>
 <div v-else class="flex items-center gap-3">
 <VButton type="danger" size="sm" @click="handleDeleteInBatch">删除</VButton>
 <span class="text-sm text-text-tertiary">已选择 {{ selectedSlugs.length }} 项</span>
 </div>
 </div>
 <div class="mt-4 flex sm:mt-0">
 <div class="flex items-center gap-6">
 <FilterDropdown v-model="selectedStatus" :items="statusItems" label="状态" @change="handleFilterChange" />
 <FilterDropdown v-model="selectedSort" :items="sortItems" label="排序" @change="handleFilterChange" />
 <div class="group cursor-pointer rounded p-1 hover:bg-accent hover:bg-accent" @click="refetch()">
 <Icon
 icon="ri:refresh-line"
 :class="{ 'animate-spin text-foreground ': isFetching }"
 class="h-4 w-4 text-text-tertiary group-hover:text-foreground group-hover:text-foreground"
 />
 </div>
 </div>
 </div>
 </div>
 </div>
 </template>

 <VLoading v-if="isLoading" />

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
 <VEntityContainer>
 <VEntity v-for="p in pages" :key="p.slug" :is-selected="checkSelection(p.slug)">
 <template #checkbox>
 <input
 :checked="checkSelection(p.slug)"
 class="h-4 w-4 rounded border-[var(--color-border-strong)] text-primary"
 type="checkbox"
 @change="toggleSelection(p.slug)"
 />
 </template>
 <template #start>
 <VEntityField :title="p.title || p.slug" max-width="30rem">
 <template #title>
 <div class="flex items-center gap-1.5">
 <span
 class="truncate text-sm font-medium text-foreground hover:text-blue-600 hover:text-blue-400"
 @click.stop="router.push(`/pages/edit/${p.slug}`)"
 >
 {{ p.title || p.slug }}
 </span>
 <span
 class="hidden cursor-pointer text-text-tertiary transition-all hover:text-foreground group-hover:inline-block hover:text-foreground"
 title="在前台打开"
 @click.stop="openPage(p.slug)"
 >
 <Icon icon="ri:external-link-line" class="h-3.5 w-3.5" />
 </span>
 </div>
 </template>
 <template #description>
 <span class="text-xs text-text-tertiary">Slug：{{ p.slug }}</span>
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
 <span class="truncate text-xs tabular-nums text-text-tertiary">
 {{ formatDate(p.updated_at) }}
 </span>
 </template>
 </VEntityField>
 </template>
 <template #dropdownItems>
 <div class="relative">
 <div
 class="cursor-pointer rounded p-1 transition-all hover:bg-accent/60 hover:text-blue-600"
 @click.stop="menuOpenSlug = menuOpenSlug === p.slug ? null : p.slug"
 >
 <Icon icon="ri:more-fill" />
 </div>
 <Transition name="fade">
 <div
 v-if="menuOpenSlug === p.slug"
 class="absolute right-0 top-full z-30 mt-1 w-32 rounded-base border border-border bg-card p-2 shadow-lg"
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
 </VEntityContainer>
 </Transition>

 <template #footer>
 <div class="bg-card sm:flex sm:items-center sm:justify-end">
 <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" />
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
