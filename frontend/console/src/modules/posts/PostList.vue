<script setup lang="ts">
import { ref, computed, watch, onMounted } from"vue";
import { useRouter, useRoute } from"vue-router";
import { useQuery, useMutation, useQueryClient } from"@tanstack/vue-query";
import { postApi, shareApi, taxonomyApi, settingsApi } from"@/api";
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
 VModal,
 Dialog,
 Toast,
} from"@/components/ui";

const router = useRouter();
const route = useRoute();
const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

// 搜索关键词（debounce 300ms，避免每次按键都触发请求）
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

// 筛选条件
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
const selectedCategory = ref<string | undefined>(undefined);
const selectedTag = ref<string | undefined>(undefined);

const statusItems = [
 { label:"全部", value: undefined },
 { label:"已发布", value:"published" },
 { label:"草稿", value:"draft" },
 { label:"回收站", value:"trashed" },
];

const sortItems = [
 { label:"默认", value: undefined },
 { label:"较近创建", value:"newest" },
 { label:"较早创建", value:"oldest" },
 { label:"较近更新", value:"updated" },
];

// 分类/标签筛选项（taxonomy API）+ 站长头像（站点设置，作者列展示）
const categoryItems = ref<{ label: string; value: string | undefined }[]>([{ label:"全部", value: undefined }]);
const tagItems = ref<{ label: string; value: string | undefined }[]>([{ label:"全部", value: undefined }]);
const ownerAvatar = ref("");
const ownerName = ref("");

onMounted(async () => {
 try {
 const [cRes, tRes] = await Promise.all([taxonomyApi.categories(), taxonomyApi.tags()]);
 categoryItems.value = [
 { label:"全部", value: undefined },
 ...(cRes.data.items || []).map((i: any) => ({ label: i.name, value: i.name })),
 ];
 tagItems.value = [
 { label:"全部", value: undefined },
 ...(tRes.data.items || []).map((i: any) => ({ label: i.name, value: i.name })),
 ];
 } catch {
 /* 静默：筛选项缺失不阻塞列表 */
 }
 try {
 const { data } = await settingsApi.all();
 ownerAvatar.value = data?.site?.avatar ||"";
 ownerName.value = data?.site?.title ||"站长";
 } catch {
 /* 静默 */
 }
});

// 响应式查询参数：page/size/status/category/tag/keyword 变化自动重取
const queryParams = computed(() => {
 const params: Record<string, unknown> = { page: page.value, size: size.value };
 if (selectedStatus.value) params.status = selectedStatus.value;
 if (selectedCategory.value) params.category = selectedCategory.value;
 if (selectedTag.value) params.tag = selectedTag.value;
 if (debouncedKeyword.value.trim()) params.search = debouncedKeyword.value.trim();
 return params;
});

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
 queryKey: computed(() => queryKeys.posts(queryParams.value)),
 queryFn: () => postApi.list(queryParams.value).then(({ data }) => data),
});

// 客户端排序（排序变化不触发网络请求）
const posts = computed(() => {
 const items = [...(queryData.value?.items || [])];
 if (selectedSort.value ==="newest") {
 items.sort((a: any, b: any) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
 } else if (selectedSort.value ==="oldest") {
 items.sort((a: any, b: any) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
 } else if (selectedSort.value ==="updated") {
 items.sort((a: any, b: any) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
 }
 return items;
});
const total = computed(() => queryData.value?.total || 0);

// 批量选择
const checkedAll = ref(false);
const selectedSlugs = ref<string[]>([]);
const menuOpenSlug = ref<string | null>(null);

const hasFilters = computed(
 () =>
 selectedStatus.value !== undefined ||
 selectedSort.value !== undefined ||
 selectedCategory.value !== undefined ||
 selectedTag.value !== undefined ||
 keyword.value !==""
);

watch(selectedSlugs, (val) => {
 checkedAll.value = val.length === posts.value.length && val.length > 0;
});

function handleCheckAllChange(e: Event) {
 const { checked } = e.target as HTMLInputElement;
 selectedSlugs.value = checked ? posts.value.map((p) => p.slug) : [];
}

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
 selectedCategory.value = undefined;
 selectedTag.value = undefined;
 keyword.value ="";
 debouncedKeyword.value ="";
 page.value = 1;
}

const statusLabel = computed(() => statusItems.find((i) => i.value === selectedStatus.value)?.label ||"");
const sortLabel = computed(() => sortItems.find((i) => i.value === selectedSort.value)?.label ||"");
const isTrashed = computed(() => selectedStatus.value ==="trashed");

// Mutations：操作成功后自动失效文章列表缓存
const deleteMutation = useMutation({
 mutationFn: (slug: string) => postApi.delete(slug),
 onSuccess: () => {
 queryClient.invalidateQueries({ queryKey: ["posts"] });
 Toast.success("删除成功");
 },
});

function handleDelete(post: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要删除该文章吗？",
 description:"该操作会将文章放入回收站，后续可以从回收站恢复",
 confirmType:"danger",
 onConfirm: () => deleteMutation.mutate(post.slug),
 });
}

function handleDeleteInBatch() {
 Dialog.warning({
 title:"确定要删除选中的文章吗？",
 description:"该操作会将文章放入回收站，后续可以从回收站恢复",
 confirmType:"danger",
 onConfirm: async () => {
 await Promise.all(selectedSlugs.value.map((slug) => postApi.delete(slug)));
 selectedSlugs.value = [];
 queryClient.invalidateQueries({ queryKey: ["posts"] });
 Toast.success("删除成功");
 },
 });
}

function handleRestore(post: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要恢复该文章吗？",
 onConfirm: async () => {
 await postApi.restore(post.slug);
 queryClient.invalidateQueries({ queryKey: ["posts"] });
 Toast.success("恢复成功");
 },
 });
}

function handlePurge(post: any) {
 menuOpenSlug.value = null;
 Dialog.warning({
 title:"确定要彻底删除该文章吗？",
 description:"该操作不可恢复！",
 confirmType:"danger",
 onConfirm: async () => {
 await postApi.purge(post.slug);
 queryClient.invalidateQueries({ queryKey: ["posts"] });
 Toast.success("已彻底删除");
 },
 });
}

// --- 创建分享链接 ---
const showShareModal = ref(false);
const shareCreating = ref(false);
const shareForm = ref({ share_type:"published", expires_at:"" });
const shareTargetSlug = ref("");
const shareResultUrl = ref("");

/** 打开创建分享链接弹窗 */
function handleCreateShare(post: any) {
 menuOpenSlug.value = null;
 shareTargetSlug.value = post.slug;
 shareForm.value = { share_type:"published", expires_at:"" };
 shareResultUrl.value ="";
 showShareModal.value = true;
}

/** 提交创建分享链接请求 */
async function handleShareCreate() {
 shareCreating.value = true;
 try {
 const { data } = await shareApi.create(shareTargetSlug.value, {
 share_type: shareForm.value.share_type,
 expires_at: shareForm.value.expires_at,
 });
 shareResultUrl.value = window.location.origin + data.url;
 queryClient.invalidateQueries({ queryKey: ["shares"] });
 Toast.success("分享链接创建成功");
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"创建失败");
 } finally {
 shareCreating.value = false;
 }
}

/** 复制生成的分享链接到剪贴板 */
async function handleCopyShareUrl() {
 try {
 await navigator.clipboard.writeText(shareResultUrl.value);
 Toast.success("链接已复制到剪贴板");
 } catch {
 Toast.error("复制失败，请手动复制");
 }
}

const inputCls ="apple-input";

// Helpers
function parseTags(tags: string): string[] {
 if (!tags) return [];
 return tags.split(",").map((t) => t.trim()).filter(Boolean);
}

function formatDate(dateStr: string): string {
 if (!dateStr) return"";
 return dateStr.slice(0, 10);
}

function openPost(slug: string) {
 window.open(`/posts/${slug}`,"_blank");
}

function closeMenus() {
 menuOpenSlug.value = null;
}
</script>

<template>
 <div @click="closeMenus">
 <!-- VPageHeader -->
 <VPageHeader title="文章">
 <template #icon>
 <Icon icon="ri:book-read-line" class="mr-2 self-center text-xl" />
 </template>
 <template #actions>
 <div class="flex items-center gap-2">
 <VButton size="sm" @click="router.push('/taxonomy')">分类</VButton>
 <VButton size="sm" @click="router.push('/taxonomy')">标签</VButton>
 <VButton size="sm" @click="selectedStatus = 'trashed'; page = 1">回收站</VButton>
 <VButton type="secondary" @click="router.push('/posts/new')">
 <template #icon>
 <Icon icon="ri:add-circle-line" class="h-full w-full" />
 </template>
 新建
 </VButton>
 </div>
 </template>
 </VPageHeader>

 <div class="m-0 md:m-4">
 <VCard :body-class="['!p-0']">
 <!-- 筛选头部（bg-secondary） -->
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
 <!-- Normal mode -->
 <div v-if="!selectedSlugs.length" class="flex items-center gap-2">
 <input
 v-model="keyword"
 placeholder="输入关键词搜索"
 type="text"
 class="apple-input w-48 py-1.5"
 />
 <FilterTag v-if="keyword" @close="keyword = ''">关键词：{{ keyword }}</FilterTag>
 <FilterTag v-if="selectedStatus !== undefined" @close="selectedStatus = undefined; handleFilterChange()">
 状态：{{ statusLabel }}
 </FilterTag>
 <FilterTag v-if="selectedCategory !== undefined" @close="selectedCategory = undefined; handleFilterChange()">
 分类：{{ selectedCategory }}
 </FilterTag>
 <FilterTag v-if="selectedTag !== undefined" @close="selectedTag = undefined; handleFilterChange()">
 标签：{{ selectedTag }}
 </FilterTag>
 <FilterTag v-if="selectedSort !== undefined" @close="selectedSort = undefined; handleFilterChange()">
 排序：{{ sortLabel }}
 </FilterTag>
 <button
 v-if="hasFilters"
 class="text-xs text-text-tertiary underline hover:text-foreground"
 @click="handleClearFilters"
 >
 清除筛选
 </button>
 </div>
 <!-- Batch mode -->
 <div v-else class="flex items-center gap-3">
 <VButton type="danger" size="sm" @click="handleDeleteInBatch">删除</VButton>
 <span class="text-sm text-text-tertiary">已选择 {{ selectedSlugs.length }} 项</span>
 </div>
 </div>
 <div class="mt-4 flex sm:mt-0">
 <div class="flex items-center gap-6">
 <FilterDropdown v-model="selectedStatus" :items="statusItems" label="状态" @change="handleFilterChange" />
 <FilterDropdown v-model="selectedCategory" :items="categoryItems" label="分类" @change="handleFilterChange" />
 <FilterDropdown v-model="selectedTag" :items="tagItems" label="标签" @change="handleFilterChange" />
 <FilterDropdown v-model="selectedSort" :items="sortItems" label="排序" @change="handleFilterChange" />
 <div class="group cursor-pointer rounded p-1 hover:bg-accent" @click="refetch()">
 <Icon
 icon="ri:refresh-line"
 :class="{ 'animate-spin text-foreground': isFetching }"
 class="h-4 w-4 text-text-tertiary group-hover:text-foreground"
 />
 </div>
 </div>
 </div>
 </div>
 </div>
 </template>

 <!-- Loading -->
 <VLoading v-if="isLoading" />

 <!-- Empty -->
 <Transition v-else-if="!posts.length" appear name="fade">
 <VEmpty message="你可以尝试刷新或者新建文章" title="当前没有文章">
 <template #actions>
 <div class="flex items-center gap-2">
 <VButton @click="refetch">刷新</VButton>
 <VButton type="primary" @click="router.push('/posts/new')">
 <template #icon>
 <Icon icon="ri:add-circle-line" class="h-full w-full" />
 </template>
 新建文章
 </VButton>
 </div>
 </template>
 </VEmpty>
 </Transition>

 <!-- Entity list -->
 <Transition v-else appear name="fade">
 <VEntityContainer>
 <VEntity v-for="post in posts" :key="post.slug" :is-selected="checkSelection(post.slug)">
 <template #checkbox>
 <input
 :checked="checkSelection(post.slug)"
 class="h-4 w-4 rounded border-[var(--color-border-strong)] text-primary"
 type="checkbox"
 @change="toggleSelection(post.slug)"
 />
 </template>
 <template #start>
 <VEntityField v-if="post.cover">
 <template #description>
 <div class="w-20 overflow-hidden rounded-md" style="aspect-ratio: 3/2">
 <img class="h-full w-full object-cover" :src="post.cover" :alt="post.title" />
 </div>
 </template>
 </VEntityField>
 <VEntityField :title="post.title || post.slug" max-width="30rem">
 <template #title>
 <div class="flex items-center gap-1.5">
 <Icon v-if="post.pinned" icon="ri:pushpin-2-fill" class="shrink-0 text-xs text-red-400" />
 <span
 class="truncate text-sm font-medium text-foreground hover:text-primary"
 @click.stop="router.push(`/posts/edit/${post.slug}`)"
 >
 {{ post.title || post.slug }}
 </span>
 <!-- 小绿点：已保存但未发布的草稿头（对齐 Halo） -->
 <span
 v-if="post.has_draft && post.status === 'published'"
 class="group/dot relative flex shrink-0 items-center"
 >
 <span class="inline-block h-1.5 w-1.5 rounded-full bg-green-500"></span>
 <span
 class="pointer-events-none absolute left-1/2 top-full z-30 mt-1.5 hidden -translate-x-1/2 whitespace-nowrap rounded bg-foreground px-2 py-1 text-xs text-background shadow-lg group-hover/dot:block"
 >
 当前有内容已保存，但还未发布。
 </span>
 </span>
 <span
 class="hidden cursor-pointer text-text-tertiary transition-all hover:text-foreground group-hover:inline-block"
 title="在前台打开"
 @click.stop="openPost(post.slug)"
 >
 <Icon icon="ri:external-link-line" class="h-3.5 w-3.5" />
 </span>
 </div>
 </template>
 <template #description>
 <div class="flex flex-col gap-1.5">
 <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
 <span v-if="post.category" class="text-xs text-text-tertiary">
 分类：{{ post.category }}
 </span>
 <span class="text-xs text-text-tertiary">访问量 {{ post.views || 0 }}</span>
 <span v-if="post.pinned" class="text-xs text-text-tertiary">已置顶</span>
 </div>
 <div v-if="parseTags(post.tags).length" class="flex flex-wrap gap-1.5">
 <span
 v-for="tag in parseTags(post.tags)"
 :key="tag"
 class="rounded-base bg-secondary px-1.5 py-0.5 text-xs text-text-tertiary"
 >
 {{ tag }}
 </span>
 </div>
 </div>
 </template>
 </VEntityField>
 </template>
 <template #end>
 <!-- 作者头像（单作者：站长，对齐 Halo 作者列） -->
 <VEntityField v-if="ownerAvatar">
 <template #description>
 <span class="group/avatar relative flex items-center">
 <img
 :src="ownerAvatar"
 :alt="ownerName"
 class="h-6 w-6 rounded-full border border-border object-cover"
 />
 <span
 class="pointer-events-none absolute left-1/2 top-full z-30 mt-1.5 hidden -translate-x-1/2 whitespace-nowrap rounded bg-foreground px-2 py-1 text-xs text-background shadow-lg group-hover/avatar:block"
 >
 {{ ownerName }}
 </span>
 </span>
 </template>
 </VEntityField>
 <VEntityField v-if="post.status === 'draft'">
 <template #description>
 <VStatusDot state="warning" text="未发布" />
 </template>
 </VEntityField>
 <VEntityField v-else-if="post.status === 'trashed'">
 <template #description>
 <VStatusDot state="error" text="已删除" />
 </template>
 </VEntityField>
 <VEntityField v-else description="已发布" />
 <VEntityField>
 <template #description>
 <Icon
 icon="ri:eye-line"
 class="cursor-pointer text-sm text-text-quaternary transition-all hover:text-primary"
 title="公开访问"
 />
 </template>
 </VEntityField>
 <VEntityField>
 <template #description>
 <span class="truncate text-xs tabular-nums text-text-tertiary">
 {{ formatDate(post.created_at) }}
 </span>
 </template>
 </VEntityField>
 </template>
 <template #dropdownItems>
 <div class="relative">
 <div
 class="cursor-pointer rounded p-1 transition-all hover:bg-accent hover:text-primary"
 @click.stop="menuOpenSlug = menuOpenSlug === post.slug ? null : post.slug"
 >
 <Icon icon="ri:more-fill" />
 </div>
 <Transition name="fade">
 <div
 v-if="menuOpenSlug === post.slug"
 class="absolute right-0 top-full z-30 mt-1 w-32 rounded-base border border-border bg-card p-2 shadow-lg"
 @click.stop
 >
 <template v-if="isTrashed">
 <VButton block type="secondary" size="sm" @click="handleRestore(post)">恢复</VButton>
 <div class="mt-2">
 <VButton block type="danger" size="sm" @click="handlePurge(post)">彻底删除</VButton>
 </div>
 </template>
 <template v-else>
 <VButton block type="secondary" size="sm" @click="menuOpenSlug = null; router.push(`/posts/edit/${post.slug}`)">
 编辑
 </VButton>
 <div class="mt-2">
 <VButton block type="secondary" size="sm" @click="handleCreateShare(post)">创建分享链接</VButton>
 </div>
 <div class="mt-2">
 <VButton block type="danger" size="sm" @click="handleDelete(post)">删除</VButton>
 </div>
 </template>
 </div>
 </Transition>
 </div>
 </template>
 </VEntity>
 </VEntityContainer>
 </Transition>

 <!-- Footer: pagination -->
 <template #footer>
 <div class="bg-card sm:flex sm:items-center sm:justify-end">
 <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" />
 </div>
 </template>
 </VCard>
 </div>

 <!-- 创建分享链接弹窗 -->
 <VModal
 v-model:visible="showShareModal"
 :title="shareResultUrl ? '分享链接已创建' : '创建分享链接'"
 :width="520"
 >
 <!-- 表单状态 -->
 <div v-if="!shareResultUrl" class="space-y-4">
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">分享类型</label>
 <select v-model="shareForm.share_type" :class="inputCls">
 <option value="published">已发布版本</option>
 <option value="draft">草稿版本</option>
 </select>
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">过期时间（可选）</label>
 <input v-model="shareForm.expires_at" type="datetime-local" :class="inputCls" />
 </div>
 </div>
 <!-- 结果状态：展示生成的链接 -->
 <div v-else class="space-y-3">
 <p class="text-sm text-text-tertiary">分享链接已生成，可复制或直接打开：</p>
 <div
 class="flex items-center gap-2 rounded-base border border-border bg-secondary px-3 py-2"
 >
 <span class="flex-1 truncate text-sm text-primary">{{ shareResultUrl }}</span>
 <VButton size="sm" type="secondary" @click="handleCopyShareUrl">复制</VButton>
 </div>
 <a :href="shareResultUrl" target="_blank" class="inline-block text-xs text-primary hover:underline">
 在新标签页打开 →
 </a>
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton v-if="!shareResultUrl" @click="showShareModal = false">取消</VButton>
 <VButton v-if="!shareResultUrl" type="secondary" :loading="shareCreating" @click="handleShareCreate">创建</VButton>
 <VButton v-else type="secondary" @click="showShareModal = false">完成</VButton>
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
