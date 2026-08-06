<script setup lang="ts">
import { ref, computed } from"vue";
import { useQuery, useMutation, useQueryClient } from"@tanstack/vue-query";
import { notificationApi, type NotificationItem } from"@/api";
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
 Toast,
} from"@/components/ui";

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

const queryParams = computed(() => ({ page: page.value, size: size.value }));

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
 queryKey: computed(() => queryKeys.notifications(queryParams.value)),
 queryFn: () => notificationApi.list(queryParams.value).then(({ data }) => data),
});

const items = computed<NotificationItem[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);
const unread = computed(() => queryData.value?.unread || 0);

/** 失效通知列表缓存（列表与未读数共用 key 前缀） */
function invalidate() {
 queryClient.invalidateQueries({ queryKey: ["notifications"] });
}

/** 标记单条已读：成功后失效缓存，列表自动重排（未读优先） */
const readMutation = useMutation({
 mutationFn: (id: number) => notificationApi.markRead(id),
 onSuccess: () => {
 invalidate();
 },
});

/** 全部已读 */
const readAllMutation = useMutation({
 mutationFn: () => notificationApi.markAllRead(),
 onSuccess: () => {
 invalidate();
 Toast.success("已全部标记为已读");
 },
});

/** 点击条目：标记已读 + 新标签页打开目标页面 */
function handleOpen(n: NotificationItem) {
 if (!n.read) readMutation.mutate(n.id);
 if (n.link) window.open(n.link,"_blank");
}

/** 通知类型图标 */
function typeIcon(type: string): string {
 if (type ==="comment_new") return"ri:chat-3-line";
 if (type ==="comment_reply") return"ri:chat-reply-line";
 if (type ==="comment_approved") return"ri:checkbox-circle-line";
 return"ri:notification-3-line";
}

/** 格式化日期时间：2026-07-27T14:27:00Z -> 2026-07-27 14:27 */
function formatDateTime(dateStr: string): string {
 if (!dateStr) return"";
 return dateStr.slice(0, 16).replace("T","");
}
</script>

<template>
 <div>
 <VPageHeader title="通知中心">
 <template #icon>
 <Icon icon="ri:notification-3-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="m-0 md:m-4">
 <VCard :body-class="['!p-0']">
 <!-- 头部：未读提示 + 全部已读 + 刷新 -->
 <template #header>
 <div class="block w-full bg-secondary px-4 py-3">
 <div class="flex items-center justify-between">
 <div class="flex items-center gap-2 text-sm text-text-tertiary">
 <span class="flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-xs font-medium text-white">
 {{ unread }}
 </span>
 <span class="text-xs text-text-tertiary">条未读</span>
 </div>
 <div class="flex items-center gap-2">
 <VButton size="sm" :disabled="unread === 0" :loading="readAllMutation.isPending.value" @click="readAllMutation.mutate()">
 全部已读
 </VButton>
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
 </div>
 </template>

 <!-- Loading -->
 <VLoading v-if="isLoading" />

 <!-- Empty -->
 <Transition v-else-if="!items.length" appear name="fade">
 <VEmpty message="评论产生新通知时会显示在这里" title="暂无通知">
 <template #actions>
 <VButton @click="refetch">刷新</VButton>
 </template>
 </VEmpty>
 </Transition>

 <!-- 通知列表：未读高亮（左侧主题色竖条 + 图标） -->
 <Transition v-else appear name="fade">
 <ul class="box-border h-full w-full divide-y divide-[var(--color-border)]" role="list">
 <li
 v-for="n in items"
 :key="n.id"
 class="cursor-pointer transition-colors hover:bg-secondary hover:bg-accent"
 @click="handleOpen(n)"
 >
 <VEntity>
 <template #start>
 <VEntityField width="3rem">
 <template #description>
 <span
 class="flex h-9 w-9 items-center justify-center rounded-full"
 :class="n.read ? 'bg-secondary ' : 'bg-primary/15'"
 >
 <Icon
 :icon="typeIcon(n.type)"
 class="text-lg"
 :class="n.read ? 'text-text-quaternary ' : 'text-secondary text-primary'"
 />
 </span>
 </template>
 </VEntityField>
 <VEntityField :width="n.read ? undefined : '2.5rem'">
 <template #title>
 <span class="flex items-center gap-2 text-sm font-medium text-foreground">
 <span v-if="!n.read" class="h-2 w-2 flex-shrink-0 rounded-full bg-red-500" />
 {{ n.title }}
 </span>
 </template>
 <template #description>
 <span class="line-clamp-2 text-xs text-text-quaternary">
 {{ n.content }}
 </span>
 </template>
 </VEntityField>
 </template>
 <template #end>
 <!-- 跳转目标 -->
 <VEntityField v-if="n.link">
 <template #description>
 <span class="truncate text-xs text-text-quaternary">
 {{ n.link }}
 </span>
 </template>
 </VEntityField>
 <!-- 时间 -->
 <VEntityField>
 <template #description>
 <span class="truncate text-xs tabular-nums text-text-tertiary">
 {{ formatDateTime(n.created_at) }}
 </span>
 </template>
 </VEntityField>
 </template>
 </VEntity>
 </li>
 </ul>
 </Transition>

 <!-- Footer: pagination -->
 <template #footer>
 <div class="bg-card sm:flex sm:items-center sm:justify-end">
 <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 50, 100]" />
 </div>
 </template>
 </VCard>
 </div>
 </div>
</template>
