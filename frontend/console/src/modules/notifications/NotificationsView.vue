<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery, useMutation, useQueryClient } from "@tanstack/vue-query";
import { notificationApi, type NotificationItem } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  Toast,
} from "@/components/ui";

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
const unreadCount = computed(() => queryData.value?.unread || 0);

type FilterKey = "all" | "system" | "comment" | "interaction" | "reminder";

const filterTabs: { key: FilterKey; label: string }[] = [
  { key: "all", label: "全部" },
  { key: "system", label: "系统" },
  { key: "comment", label: "评论" },
  { key: "interaction", label: "互动" },
  { key: "reminder", label: "提醒" },
];

const activeFilter = ref<FilterKey>("all");

/** 通知类型 -> 分组（系统 / 评论 / 互动 / 提醒） */
function categoryOf(type: string): Exclude<FilterKey, "all"> {
  if (type === "comment_new" || type === "comment_reply" || type === "comment_approved") {
    return "comment";
  }
  if (type.startsWith("like_") || type.startsWith("follow_") || type === "moment_like") {
    return "interaction";
  }
  if (type.startsWith("remind_") || type === "backup" || type === "storage_warning") {
    return "reminder";
  }
  return "system";
}

const filteredItems = computed<NotificationItem[]>(() => {
  if (activeFilter.value === "all") return items.value;
  return items.value.filter((n) => categoryOf(n.type) === activeFilter.value);
});

type GroupKey = "today" | "yesterday" | "earlier";

interface NotifGroup {
  key: GroupKey;
  title: string;
  items: NotificationItem[];
}

function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

const groupedNotifications = computed<NotifGroup[]>(() => {
  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterdayStart = new Date(todayStart.getTime() - 24 * 60 * 60 * 1000);

  const buckets: Record<GroupKey, NotificationItem[]> = {
    today: [],
    yesterday: [],
    earlier: [],
  };

  for (const n of filteredItems.value) {
    const d = new Date(n.created_at);
    if (isNaN(d.getTime())) {
      buckets.earlier.push(n);
      continue;
    }
    if (isSameDay(d, todayStart)) {
      buckets.today.push(n);
    } else if (isSameDay(d, yesterdayStart)) {
      buckets.yesterday.push(n);
    } else {
      buckets.earlier.push(n);
    }
  }

  const groups: NotifGroup[] = [];
  (["today", "yesterday", "earlier"] as GroupKey[]).forEach((k) => {
    if (buckets[k].length > 0) {
      groups.push({
        key: k,
        title: k === "today" ? "今天" : k === "yesterday" ? "昨天" : "更早",
        items: buckets[k],
      });
    }
  });
  return groups;
});

/** 失效通知列表缓存（列表与未读数共用 key 前缀） */
function invalidate() {
  queryClient.invalidateQueries({ queryKey: queryKeys.notifications() });
}

/** 标记单条已读：成功后失效缓存，列表自动重排（未读优先） */
const readMutation = useMutation({
  mutationFn: (id: number) => notificationApi.markRead(id),
  onSuccess: () => {
    invalidate();
  },
  // Spec 62：失败反馈（原静默失败）
  onError: (e: any) => Toast.error(e?.response?.data?.error || "标记失败"),
});

/** 全部已读 */
const readAllMutation = useMutation({
  mutationFn: () => notificationApi.markAllRead(),
  onSuccess: () => {
    invalidate();
    Toast.success("已全部标记为已读");
  },
  // Spec 62：失败反馈（原静默失败）
  onError: (e: any) => Toast.error(e?.response?.data?.error || "标记失败"),
});

/** 点击条目：标记已读 + 新标签页打开目标页面 */
function handleOpen(n: NotificationItem) {
  if (!n.read) readMutation.mutate(n.id);
  if (n.link) window.open(n.link, "_blank");
}

/** 通知类型图标 */
function typeIcon(type: string): string {
  if (type === "comment_new") return "ri:chat-3-line";
  if (type === "comment_reply") return "ri:chat-follow-up-line";
  if (type === "comment_approved") return "ri:checkbox-circle-line";
  if (type.startsWith("like_") || type === "moment_like") return "ri:heart-3-line";
  if (type.startsWith("follow_")) return "ri:user-add-line";
  if (type === "backup" || type.startsWith("remind_")) return "ri:alarm-warning-line";
  if (type === "storage_warning") return "ri:database-2-line";
  return "ri:notification-3-line";
}

/** 通知类型 -> 图标配色变体：系统蓝 / 评论绿 / 互动粉 / 提醒橙 */
function typeVariant(type: string): "blue" | "green" | "pink" | "orange" {
  const cat = categoryOf(type);
  if (cat === "comment") return "green";
  if (cat === "interaction") return "pink";
  if (cat === "reminder") return "orange";
  return "blue";
}

/** 来源标签（meta 行） */
function sourceLabel(type: string): string {
  if (type === "comment_new") return "新评论";
  if (type === "comment_reply") return "评论回复";
  if (type === "comment_approved") return "评论通过";
  if (type.startsWith("like_") || type === "moment_like") return "点赞";
  if (type.startsWith("follow_")) return "关注";
  if (type === "backup") return "数据备份";
  if (type === "storage_warning") return "存储空间";
  return "系统通知";
}

/** 相对时间：今天显示「X 分钟前 / X 小时前」，其余显示绝对日期 */
function formatRelative(dateStr: string): string {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr.slice(0, 16).replace("T", " ");

  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterdayStart = new Date(todayStart.getTime() - 24 * 60 * 60 * 1000);

  const pad = (n: number) => String(n).padStart(2, "0");
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;

  if (isSameDay(d, todayStart)) {
    const diffMin = Math.floor((now.getTime() - d.getTime()) / 60000);
    if (diffMin < 1) return "刚刚";
    if (diffMin < 60) return `${diffMin} 分钟前`;
    const diffHr = Math.floor(diffMin / 60);
    return `${diffHr} 小时前`;
  }
  if (isSameDay(d, yesterdayStart)) {
    return `昨天 ${hm}`;
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm}`;
}
</script>

<template>
  <div class="notifications-page">
    <VPageHeader title="通知中心">
      <template #icon>
        <Icon icon="ri:notification-3-line" class="mr-2 self-center text-xl" />
      </template>
      <template #subtitle>
        管理你收到的所有通知与提醒
      </template>
      <template #actions>
        <VButton
          size="sm"
          :disabled="unreadCount === 0"
          :loading="readAllMutation.isPending.value"
          @click="readAllMutation.mutate()"
        >
          <template #icon>
            <Icon icon="ri:check-double-line" class="h-full w-full" />
          </template>
          全部已读
        </VButton>
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-[10px] border border-border bg-card text-text-tertiary transition-colors hover:border-border-strong hover:bg-secondary hover:text-foreground"
          title="刷新"
          @click="refetch()"
        >
          <Icon
            icon="ri:refresh-line"
            :class="{ 'animate-spin text-foreground': isFetching }"
            class="h-[18px] w-[18px]"
          />
        </button>
      </template>
    </VPageHeader>

    <div class="notifications-content px-5 py-5 md:px-10">
      <!-- 未读摘要横幅 -->
      <div
        v-if="unreadCount > 0"
        class="mb-5 flex items-center gap-3 rounded-[14px] border border-primary/18 bg-primary/8 px-4 py-3.5"
      >
        <div
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px] bg-primary/18 text-primary"
        >
          <Icon icon="ri:notification-3-line" class="h-[18px] w-[18px]" />
        </div>
        <div class="text-[13px] text-text-secondary">
          你有 <strong class="font-bold text-primary">{{ unreadCount }} 条未读通知</strong>，点击卡片查看详情
        </div>
      </div>

      <!-- 胶囊分段筛选 -->
      <div class="apple-segmented mb-5" role="tablist" aria-label="通知分类">
        <button
          v-for="tab in filterTabs"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="activeFilter === tab.key"
          :class="{ active: activeFilter === tab.key }"
          @click="activeFilter = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Loading -->
      <VLoading v-if="isLoading" />

      <!-- Empty -->
      <Transition v-else-if="filteredItems.length === 0" appear name="fade">
        <VEmpty message="评论产生新通知时会显示在这里" title="暂无通知">
          <template #actions>
            <VButton @click="refetch">刷新</VButton>
          </template>
        </VEmpty>
      </Transition>

      <!-- 按日期分组的通知列表 -->
      <Transition v-else appear name="fade">
        <div>
          <section v-for="group in groupedNotifications" :key="group.key" class="mb-7 last:mb-0">
            <div class="mb-2 flex items-center justify-between px-1">
              <div class="text-[13px] font-bold tracking-tight text-text-secondary">
                {{ group.title }}
              </div>
              <div class="text-[12px] font-medium text-text-tertiary">
                {{ group.items.length }} 条
              </div>
            </div>

            <div>
              <article
                v-for="n in group.items"
                :key="n.id"
                class="notif-card group relative mb-2 flex cursor-pointer items-start gap-3.5 rounded-[14px] border border-border bg-card p-4 transition-all duration-200 last:mb-0 hover:-translate-y-px hover:border-border-strong"
                :class="{
                  'notif-card--unread': !n.read,
                  'notif-card--read': n.read,
                }"
                @click="handleOpen(n)"
              >
                <!-- 图标：按类型取色 -->
                <div
                  class="notif-icon flex h-[42px] w-[42px] shrink-0 items-center justify-center rounded-[11px] text-white"
                  :class="`notif-icon--${typeVariant(n.type)}`"
                >
                  <Icon :icon="typeIcon(n.type)" class="h-[22px] w-[22px]" />
                </div>

                <!-- 内容 -->
                <div class="min-w-0 flex-1">
                  <div class="mb-0.5 text-[14px] font-semibold leading-snug text-foreground">
                    {{ n.title }}
                  </div>
                  <div class="mb-1.5 text-[13px] leading-snug text-text-secondary">
                    {{ n.content }}
                  </div>
                  <div class="flex items-center gap-2 text-[12px] text-text-tertiary">
                    <span class="font-medium">{{ sourceLabel(n.type) }}</span>
                    <span class="notif-meta-dot h-[3px] w-[3px] rounded-full bg-text-quaternary" />
                    <span class="truncate">{{ n.link || "站内通知" }}</span>
                  </div>
                </div>

                <!-- 右侧：时间 + 未读圆点 -->
                <div class="flex shrink-0 flex-col items-end gap-2 pt-0.5">
                  <div class="whitespace-nowrap text-[12px] font-medium tabular-nums text-text-tertiary">
                    {{ formatRelative(n.created_at) }}
                  </div>
                  <span
                    v-if="!n.read"
                    class="h-2 w-2 rounded-full bg-primary"
                    style="box-shadow: 0 0 8px color-mix(in srgb, var(--color-primary) 60%, transparent);"
                  />
                  <span v-else class="h-2 w-2 rounded-full bg-transparent" />
                </div>
              </article>
            </div>
          </section>

          <!-- 分页 -->
          <div class="mt-6 flex justify-end border-t border-border pt-4">
            <VPagination
              v-model:page="page"
              v-model:size="size"
              :total="total"
              :size-options="[20, 50, 100]"
            />
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.notifications-content {
  min-height: calc(100vh - 56px);
}

/* 未读卡：蓝色左边框 3px + 略深背景 */
.notif-card--unread {
  border-left: 3px solid var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 4%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 22%, transparent);
  padding-left: calc(1rem - 3px);
}

.notif-card--unread:hover {
  background: color-mix(in srgb, var(--color-primary) 7%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 35%, transparent);
}

/* 已读卡：降低对比度 */
.notif-card--read {
  opacity: 0.78;
}

.notif-card--read:hover {
  opacity: 1;
}

/* 图标配色变体 */
.notif-icon--blue {
  background: linear-gradient(135deg, #66ABFF, #2E8DFF);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-primary) 30%, transparent);
}

.notif-icon--green {
  background: linear-gradient(135deg, #2E8DFF, #007AFF);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-state-success) 30%, transparent);
}

.notif-icon--pink {
  background: linear-gradient(135deg, #9FCBFF, #66ABFF);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-state-error) 30%, transparent);
}

.notif-icon--orange {
  background: linear-gradient(135deg, #007AFF, #0064D6);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-state-warning) 30%, transparent);
}
</style>
