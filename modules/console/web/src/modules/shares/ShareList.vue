<script setup lang="ts">
import { computed } from"vue";
import { useMutation, useQueryClient } from"@tanstack/vue-query";
import { usePagedList } from"@/composables/usePagedList";
import { shareApi } from"@/api";
import { queryKeys } from"@/api/query-client";
import { Icon } from"@iconify/vue";
import {
 VPageHeader,
 VCard,
 VButton,
 VEntity,
 VEntityField,
 VEmpty,
 VLoading,
 VPagination,
 Dialog,
 Toast,
} from"@/components/ui";

/** 分享链接响应模型 */
interface ShareRow {
 id: number;
 token: string;
 post_slug: string;
 post_title: string;
 share_type: string;
 expires_at: string;
 created_at: string;
 url: string;
}

const queryClient = useQueryClient();

// Spec 62：分页样板收敛到 usePagedList（示范迁移，与 PostList 同构）
const { page, size, items: rawShares, total, isLoading, isFetching, refetch } = usePagedList<ShareRow>({
 apiFn: (params: Record<string, unknown>) => shareApi.list(params),
 keyFn: (params: Record<string, unknown>) => queryKeys.shares(params),
});

const shares = computed<ShareRow[]>(() => rawShares.value);

/** 删除 mutation：成功后失效分享列表缓存 */
const deleteMutation = useMutation({
 mutationFn: (id: number) => shareApi.delete(id),
 onSuccess: () => {
 queryClient.invalidateQueries({ queryKey: queryKeys.shares() });
 Toast.success("删除成功");
 },
 // Spec 62：失败反馈（原静默失败）
 onError: (e: any) => Toast.error(e?.response?.data?.error || "删除失败"),
});

/** 删除前弹出确认对话框 */
function handleDelete(share: ShareRow) {
 Dialog.warning({
 title:"确定要删除该分享链接吗？",
 description:"删除后该链接将立即失效，无法再用于访问文章",
 confirmType:"danger",
 onConfirm: () => deleteMutation.mutate(share.id),
 });
}

/** 拼接完整可访问的分享 URL */
function buildFullUrl(url: string): string {
 return window.location.origin + url;
}

/** 复制分享链接到剪贴板 */
async function handleCopy(url: string) {
 const fullUrl = buildFullUrl(url);
 try {
 await navigator.clipboard.writeText(fullUrl);
 Toast.success("链接已复制到剪贴板");
 } catch {
 Toast.error("复制失败，请手动复制");
 }
}

/** 格式化日期时间：2026-07-27T14:27:00Z -> 2026-07-27 14:27 */
function formatDateTime(dateStr: string): string {
 if (!dateStr) return"";
 return dateStr.slice(0, 16).replace("T","");
}

/** 分享类型对应的中文标签 */
function shareTypeLabel(shareType: string): string {
 return shareType ==="draft" ?"草稿" :"已发布";
}
</script>

<template>
 <div>
 <VPageHeader title="分享链接">
 <template #icon>
 <Icon icon="ri:share-forward-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="m-0 md:m-4">
 <VCard :body-class="['!p-0']">
 <!-- 筛选头部：刷新按钮 -->
 <template #header>
 <div class="block w-full bg-secondary px-4 py-3">
 <div class="flex items-center justify-end">
 <div
 class="group cursor-pointer rounded p-1 hover:bg-accent hover:bg-accent"
 @click="refetch()"
 >
 <Icon
 icon="ri:refresh-line"
 :class="{ 'animate-spin text-foreground ': isFetching }"
 class="h-4 w-4 text-text-tertiary group-hover:text-foreground group-hover:text-foreground"
 />
 </div>
 </div>
 </div>
 </template>

 <!-- Loading -->
 <VLoading v-if="isLoading" />

 <!-- Empty -->
 <Transition v-else-if="!shares.length" appear name="fade">
 <VEmpty message="你可以在文章列表中为文章创建分享链接" title="暂无分享链接">
 <template #actions>
 <VButton @click="refetch">刷新</VButton>
 </template>
 </VEmpty>
 </Transition>

 <!-- Entity list -->
 <Transition v-else appear name="fade">
 <div class="overflow-x-auto">
 <ul class="box-border h-full w-full min-w-max md:min-w-0 divide-y divide-[var(--color-border)]" role="list">
 <li v-for="share in shares" :key="share.id">
 <VEntity>
 <template #start>
 <VEntityField :title="share.post_title || share.post_slug" width="20rem">
 <template #title>
 <span class="truncate text-sm font-medium text-foreground">
 {{ share.post_title || share.post_slug }}
 </span>
 </template>
 <template #description>
 <span class="truncate text-xs text-text-quaternary">
 /{{ share.post_slug }}
 </span>
 </template>
 </VEntityField>
 </template>
 <template #end>
 <!-- 分享链接 -->
 <VEntityField width="22rem">
 <template #description>
 <div class="flex items-center gap-1.5">
 <a
 :href="buildFullUrl(share.url)"
 target="_blank"
 class="truncate text-xs text-primary hover:underline text-primary"
 :title="buildFullUrl(share.url)"
 >
 {{ share.token }}
 </a>
 <span
 class="cursor-pointer shrink-0 text-text-quaternary transition-all hover:text-primary"
 title="复制链接"
 @click="handleCopy(share.url)"
 >
 <Icon icon="ri:file-copy-line" class="h-3.5 w-3.5" />
 </span>
 </div>
 </template>
 </VEntityField>

 <!-- 分享类型 -->
 <VEntityField>
 <template #description>
 <span
 class="inline-block rounded-base px-1.5 py-0.5 text-xs font-medium"
 :class="share.share_type === 'draft'
 ? 'bg-state-warning/10 text-state-warning bg-state-warning/20 text-state-warning'
 : 'bg-state-success/10 text-state-success bg-state-success/100/20 text-state-success'"
 >
 {{ shareTypeLabel(share.share_type) }}
 </span>
 </template>
 </VEntityField>

 <!-- 创建时间 -->
 <VEntityField>
 <template #description>
 <span class="truncate text-xs tabular-nums text-text-tertiary">
 {{ formatDateTime(share.created_at) }}
 </span>
 </template>
 </VEntityField>

 <!-- 操作：删除 -->
 <VEntityField>
 <template #description>
 <span
 class="cursor-pointer text-sm text-text-quaternary transition-all hover:text-state-error"
 title="删除"
 @click="handleDelete(share)"
 >
 <Icon icon="ri:delete-bin-line" />
 </span>
 </template>
 </VEntityField>
 </template>
 </VEntity>
 </li>
 </ul>
 </div>
 </Transition>

 <!-- Footer: pagination -->
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
 transition: opacity var(--duration-fast) var(--ease-standard);
}
.fade-enter-from,
.fade-leave-to {
 opacity: 0;
}
</style>
