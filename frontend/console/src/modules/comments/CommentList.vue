<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRouter } from "vue-router";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { commentApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";

const router = useRouter();
const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

type StatusKey = "all" | "pending" | "approved" | "rejected" | "spam";
const selectedStatus = ref<StatusKey>("all");
const statusTabs: { key: StatusKey; label: string }[] = [
  { key: "all", label: "全部" },
  { key: "pending", label: "待审核" },
  { key: "approved", label: "已批准" },
  { key: "rejected", label: "已拒绝" },
  { key: "spam", label: "垃圾评论" },
];

const keyword = ref("");
const debouncedKeyword = ref("");
let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debouncedKeyword.value = keyword.value.trim().toLowerCase();
    page.value = 1;
  }, 300);
});

const menuOpenId = ref<number | null>(null);
const replyVisible = ref(false);
const replyTarget = ref<any>(null);
const replyContent = ref("");

// 批量模式：仅在批量模式下显示复选框与批量操作条
const batchMode = ref(false);

// 响应式查询参数：page/size/status 变化自动重取。
// 后端支持 pending/approved/rejected/spam 四种状态筛选，非法值回退全量。
const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (selectedStatus.value !== "all") params.status = selectedStatus.value;
  return params;
});

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
  queryKey: computed(() => queryKeys.comments(queryParams.value)),
  queryFn: () => commentApi.list(queryParams.value).then(({ data }) => data),
});

const comments = computed(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

// 各状态计数：直接使用后端列表响应中的 counts（各状态真实总数）。
const statusCounts = ref<Record<StatusKey, number>>({
  all: 0,
  pending: 0,
  approved: 0,
  rejected: 0,
  spam: 0,
});
watch(
  () => queryData.value,
  (data) => {
    const c = (data?.counts || {}) as Record<string, number>;
    statusCounts.value = {
      all: c.all ?? 0,
      pending: c.pending ?? 0,
      approved: c.approved ?? 0,
      rejected: c.rejected ?? 0,
      spam: c.spam ?? 0,
    };
  },
  { immediate: true }
);

// 本地关键字过滤（作用于当前页）
const filteredComments = computed(() => {
  const kw = debouncedKeyword.value;
  if (!kw) return comments.value;
  return comments.value.filter((cm: any) => {
    const hay = `${cm.nick || ""} ${cm.email || ""} ${cm.content || ""}`.toLowerCase();
    return hay.includes(kw);
  });
});

const statCards = computed(() => [
  {
    label: "总评论数",
    value: statusCounts.value.all,
    color: "#2E8DFF",
    bg: "color-mix(in srgb, var(--color-primary) 10%, transparent)",
    icon: "ri:chat-3-line",
    badgeText: "全部评论",
    badgeColor: "#2E8DFF",
    badgeBg: "color-mix(in srgb, var(--color-primary) 12%, transparent)",
  },
  {
    label: "待审核",
    value: statusCounts.value.pending,
    color: "#FF9F0A",
    bg: "color-mix(in srgb, var(--color-state-warning) 10%, transparent)",
    icon: "ri:time-line",
    badgeText: "需要处理",
    badgeColor: "#FF9F0A",
    badgeBg: "color-mix(in srgb, var(--color-state-warning) 12%, transparent)",
  },
  {
    label: "已批准",
    value: statusCounts.value.approved,
    color: "#30D158",
    bg: "color-mix(in srgb, var(--color-state-success) 10%, transparent)",
    icon: "ri:checkbox-circle-line",
    badgeText: "正常显示",
    badgeColor: "#30D158",
    badgeBg: "color-mix(in srgb, var(--color-state-success) 12%, transparent)",
  },
  {
    label: "垃圾评论",
    value: statusCounts.value.spam,
    color: "#FF453A",
    bg: "color-mix(in srgb, var(--color-state-error) 10%, transparent)",
    icon: "ri:delete-bin-line",
    badgeText: "已拦截",
    badgeColor: "#FF453A",
    badgeBg: "color-mix(in srgb, var(--color-state-error) 12%, transparent)",
  },
]);

// 状态徽章：class 名 + 文案
function statusInfo(status: string): { cls: string; label: string } {
  if (status === "approved") return { cls: "approved", label: "已批准" };
  if (status === "pending") return { cls: "pending", label: "待审核" };
  if (status === "rejected") return { cls: "rejected", label: "已拒绝" };
  if (status === "spam") return { cls: "spam", label: "垃圾评论" };
  return { cls: "spam", label: "未知" };
}

// 按昵称 hash 取头像渐变（avatar-gradient-1..8）
function avatarGradient(name: string): string {
  const src = name || "?";
  let hash = 0;
  for (let i = 0; i < src.length; i++) {
    hash = (hash * 31 + src.charCodeAt(i)) & 0xffffffff;
  }
  const n = (Math.abs(hash) % 8) + 1;
  return `avatar-gradient-${n}`;
}
function avatarInitial(cm: any): string {
  const source = (cm?.nick || cm?.email || "?").trim();
  return source.charAt(0).toUpperCase() || "?";
}

// 批量选择（对齐 PostList 既有模式）
const checkedAll = ref(false);
const selectedIds = ref<number[]>([]);

watch(selectedIds, (val) => {
  checkedAll.value = val.length === filteredComments.value.length && val.length > 0;
});

// 翻页/筛选变化后清空选中
watch(queryParams, () => {
  selectedIds.value = [];
});

function handleCheckAllChange(e: Event) {
  const { checked } = e.target as HTMLInputElement;
  selectedIds.value = checked ? filteredComments.value.map((cm: any) => cm.id) : [];
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

function exitBatchMode() {
  batchMode.value = false;
  selectedIds.value = [];
}

// 批量状态操作：不触发邮件通知（后端批量端点不接 notify）
async function runBatchAction(action: "approve" | "reject" | "spam", label: string) {
  try {
    const { data } = await commentApi.batch({ ids: selectedIds.value, action });
    selectedIds.value = [];
    queryClient.invalidateQueries({ queryKey: ["comments"] });
    Toast.success(`批量${label}完成：成功 ${data.succeeded} 条，失败 ${data.failed} 条`);
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || `批量${label}失败`);
  }
}
function handleApproveInBatch() {
  runBatchAction("approve", "通过");
}
function handleRejectInBatch() {
  runBatchAction("reject", "拒绝");
}
function handleSpamInBatch() {
  runBatchAction("spam", "标记垃圾");
}

function handleDeleteInBatch() {
  const count = selectedIds.value.length;
  Dialog.warning({
    title: `确定要删除选中的 ${count} 条评论吗？`,
    description: "该操作不可恢复",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        const { data } = await commentApi.batch({ ids: selectedIds.value, action: "delete" });
        selectedIds.value = [];
        queryClient.invalidateQueries({ queryKey: ["comments"] });
        Toast.success(`批量删除完成：成功 ${data.succeeded} 条，失败 ${data.failed} 条`);
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "批量删除失败");
      }
    },
  });
}

function selectStatus(key: StatusKey) {
  if (selectedStatus.value === key) return;
  selectedStatus.value = key;
  page.value = 1;
  selectedIds.value = [];
}

function handleApprove(cm: any) {
  menuOpenId.value = null;
  Dialog.warning({
    title: "确定要通过该评论吗？",
    confirmType: "secondary",
    onConfirm: async () => {
      await commentApi.approve(cm.id);
      queryClient.invalidateQueries({ queryKey: ["comments"] });
      Toast.success("已通过");
    },
  });
}

// 拒绝 / 标垃圾：走通用状态端点（不发通知）
function handleSetStatus(cm: any, status: "rejected" | "spam", title: string, doneText: string, danger = false) {
  menuOpenId.value = null;
  Dialog.warning({
    title,
    confirmType: danger ? "danger" : "secondary",
    onConfirm: async () => {
      await commentApi.setStatus(cm.id, status);
      queryClient.invalidateQueries({ queryKey: ["comments"] });
      Toast.success(doneText);
    },
  });
}
function handleReject(cm: any) {
  handleSetStatus(cm, "rejected", "确定要拒绝该评论吗？", "已拒绝");
}
function handleSpam(cm: any) {
  handleSetStatus(cm, "spam", "确定要将该评论标为垃圾吗？", "已标为垃圾", true);
}

function handleDelete(cm: any) {
  menuOpenId.value = null;
  Dialog.warning({
    title: "确定要删除该评论吗？",
    description: "该操作不可恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await commentApi.delete(cm.id);
      queryClient.invalidateQueries({ queryKey: ["comments"] });
      Toast.success("删除成功");
    },
  });
}

function openReply(cm: any) {
  menuOpenId.value = null;
  replyTarget.value = cm;
  replyContent.value = "";
  replyVisible.value = true;
}

async function handleReply() {
  if (!replyTarget.value || !replyContent.value.trim()) return;
  await commentApi.reply({ parent_id: replyTarget.value.id, content: replyContent.value });
  replyVisible.value = false;
  replyTarget.value = null;
  replyContent.value = "";
  queryClient.invalidateQueries({ queryKey: ["comments"] });
  Toast.success("回复成功");
}

function closeMenus() {
  menuOpenId.value = null;
}
</script>

<template>
  <div class="min-h-full" @click="closeMenus">
    <!-- ===== Page Header ===== -->
    <header
      class="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-2 px-4 py-4 backdrop-blur-xl sm:gap-3 sm:px-5 sm:py-5 md:px-8"
      style="background: linear-gradient(180deg, color-mix(in srgb, #000 90%, transparent) 0%, color-mix(in srgb, #000 60%, transparent) 80%, transparent 100%);"
    >
      <div class="flex flex-col gap-0.5 sm:gap-1">
        <h1 class="text-xl font-bold leading-tight tracking-tight text-white sm:text-[28px]">评论管理</h1>
        <p class="text-[12px] text-text-tertiary sm:text-[13px]">管理所有访客评论，审核、回复或删除不当内容</p>
      </div>
      <div class="flex items-center gap-1.5 sm:gap-2.5">
        <button
          type="button"
          title="刷新"
          class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-card text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-white sm:h-9 sm:w-9 sm:rounded-[10px]"
          @click="refetch()"
        >
          <Icon
            icon="ri:refresh-line"
            :class="{ 'animate-spin text-foreground': isFetching }"
            class="text-base sm:text-[18px]"
          />
        </button>
        <button
          type="button"
          class="inline-flex h-8 items-center gap-1 rounded-full border border-border bg-card px-2.5 text-xs font-semibold text-foreground transition-colors hover:border-border-strong hover:bg-secondary sm:h-9 sm:gap-1.5 sm:px-4 sm:text-[13px]"
          :class="{ '!border-primary !bg-primary/15 !text-primary': batchMode }"
          @click="batchMode = !batchMode"
        >
          <Icon icon="ri:list-check-2" class="text-sm sm:text-[15px]" />
          <span class="hidden sm:inline">{{ batchMode ? "退出批量" : "批量操作" }}</span>
        </button>
        <button
          type="button"
          class="inline-flex h-8 items-center justify-center gap-1.5 rounded-full bg-primary px-3.5 text-[13px] font-semibold text-primary-foreground transition-colors hover:bg-primary-hover active:scale-[0.97] sm:h-9 sm:px-5"
          title="评论设置"
          @click="router.push({ path: '/settings', query: { tab: 'comments' } })"
        >
          <Icon icon="ri:settings-3-line" class="text-base sm:text-[15px]" />
          <span class="hidden sm:inline">评论设置</span>
        </button>
      </div>
    </header>

    <div class="px-4 pb-8 sm:px-5 md:px-8">
      <!-- ===== Stat Cards ===== -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="stat in statCards"
          :key="stat.label"
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">{{ stat.label }}</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px]"
              :style="{ color: stat.color, backgroundColor: stat.bg }"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon :icon="stat.icon" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[34px] font-bold leading-none tracking-tight text-white"
            style="font-variant-numeric: tabular-nums; letter-spacing: -1px;"
          >
            {{ stat.value }}
          </div>
          <div
            class="mt-2.5 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-semibold"
            :style="{ color: stat.badgeColor, backgroundColor: stat.badgeBg }"
          >
            <span class="h-1.5 w-1.5 rounded-full" style="background: currentColor;"></span>
            {{ stat.badgeText }}
          </div>
        </div>
      </div>

      <!-- ===== Comments Card ===== -->
      <VCard class="mt-5" :body-class="['!p-0']">
        <!-- Card Header: 分段筛选 + 搜索 + 批量操作条 -->
        <template #header>
          <div class="flex min-w-0 flex-col gap-3 p-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:p-4">
            <div class="tabs-scroll apple-segmented -mx-1 px-1 sm:mx-0 sm:px-0">
              <button
                v-for="tab in statusTabs"
                :key="tab.key"
                type="button"
                :class="['shrink-0 whitespace-nowrap', { active: selectedStatus === tab.key }]"
                @click="selectStatus(tab.key)"
              >
                {{ tab.label }}
                <span
                  class="ml-1.5 rounded-full px-1.5 py-0.5 text-[11px] font-semibold"
                  :class="
                    selectedStatus === tab.key
                      ? 'bg-primary/20 text-primary'
                      : 'bg-white/10 text-text-secondary'
                  "
                >
                  {{ statusCounts[tab.key] }}
                </span>
              </button>
            </div>

            <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center sm:gap-2.5">
              <!-- 批量操作条：选中后显示 -->
              <template v-if="batchMode && selectedIds.length">
                <div class="flex flex-wrap items-center gap-2">
                  <VButton type="secondary" size="sm" @click="handleApproveInBatch">批量通过</VButton>
                  <VButton type="secondary" size="sm" @click="handleRejectInBatch">批量拒绝</VButton>
                  <VButton type="secondary" size="sm" @click="handleSpamInBatch">批量垃圾</VButton>
                  <VButton type="danger" size="sm" @click="handleDeleteInBatch">批量删除</VButton>
                  <span class="text-xs text-text-tertiary">已选择 {{ selectedIds.length }} 项</span>
                </div>
              </template>
              <div v-else-if="batchMode" class="flex items-center gap-2">
                <input
                  :checked="checkedAll"
                  class="h-4 w-4 rounded border-border-strong text-primary"
                  type="checkbox"
                  @change="handleCheckAllChange"
                />
                <span class="text-xs text-text-tertiary">全选当前页</span>
                <button
                  type="button"
                  class="text-xs text-text-quaternary underline hover:text-foreground"
                  @click="exitBatchMode"
                >
                  取消
                </button>
              </div>

              <div class="apple-search flex-1 sm:w-60 sm:flex-none">
                <Icon icon="ri:search-line" class="text-text-quaternary" />
                <input v-model="keyword" placeholder="搜索评论内容、作者..." />
              </div>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />

        <Transition v-else-if="!filteredComments.length" appear name="fade">
          <VEmpty message="你可以尝试刷新或者查看评论筛选" title="当前没有评论">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <!-- ===== Comment List (邮件式布局) ===== -->
        <Transition v-else appear name="fade">
          <div class="border-t border-border">
            <div
              v-for="cm in filteredComments"
              :key="cm.id"
              class="border-b border-border px-3 py-3.5 transition-colors last:border-b-0 hover:bg-white/[0.02] sm:px-5 sm:py-5"
            >
              <div class="flex items-start gap-3 sm:gap-3.5">
                <!-- 批量复选框 -->
                <input
                  v-if="batchMode"
                  :checked="checkSelection(cm.id)"
                  class="mt-1 h-4 w-4 shrink-0 rounded border-border-strong text-primary sm:mt-1.5"
                  type="checkbox"
                  @change="toggleSelection(cm.id)"
                />

                <!-- 头像 -->
                <img
                  v-if="cm.avatar"
                  :src="cm.avatar"
                  :alt="cm.nick"
                  class="h-9 w-9 shrink-0 rounded-full border border-border object-cover sm:h-[42px] sm:w-[42px]"
                />
                <div
                  v-else
                  :class="['flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white sm:h-[42px] sm:w-[42px] sm:text-[15px]', avatarGradient(cm.nick || cm.email)]"
                >
                  {{ avatarInitial(cm) }}
                </div>

                <!-- 主体 -->
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5 sm:gap-2.5">
                    <span class="text-sm font-semibold text-white sm:text-[14px]">{{ cm.nick }}</span>
                    <span
                      v-if="cm.is_admin"
                      class="rounded-full bg-primary/15 px-1.5 py-0.5 text-[10px] font-semibold text-primary sm:px-2"
                    >
                      博主
                    </span>
                    <span
                      class="comment-status-badge"
                      :class="statusInfo(cm.status).cls"
                    >
                      {{ statusInfo(cm.status).label }}
                    </span>
                    <span class="ml-auto shrink-0 text-[11px] text-text-tertiary sm:text-[12px]">{{ cm.date }}</span>
                  </div>
                  <div v-if="cm.email" class="mt-0.5 text-[11px] text-text-tertiary sm:hidden">
                    {{ cm.email }}
                  </div>
                  <span v-if="cm.email" class="hidden truncate text-[12px] text-text-tertiary sm:inline">{{ cm.email }}</span>

                  <p v-if="cm.reply_nick" class="mt-1.5 text-xs text-text-quaternary">
                    回复 @{{ cm.reply_nick }}
                  </p>
                  <p class="mt-2 line-clamp-3 text-sm leading-[1.6] text-text-secondary sm:text-[14px]">
                    {{ cm.content }}
                  </p>

                  <div v-if="cm.page_key" class="mt-2.5 flex items-center gap-1.5 text-[12px]">
                    <span class="text-text-tertiary">评论于：</span>
                    <span class="cursor-pointer truncate font-medium text-primary transition-opacity hover:opacity-75">
                      {{ cm.page_key }}
                    </span>
                  </div>

                  <div class="mt-3 flex flex-wrap items-center gap-1.5 sm:mt-3.5 sm:gap-2">
                    <button
                      v-if="cm.status !== 'approved'"
                      type="button"
                      class="action-btn approve"
                      @click="handleApprove(cm)"
                    >
                      <Icon icon="ri:check-line" />
                      通过
                    </button>
                    <button
                      v-if="cm.status === 'pending'"
                      type="button"
                      class="action-btn reject"
                      @click="handleReject(cm)"
                    >
                      <Icon icon="ri:close-line" />
                      拒绝
                    </button>
                    <button
                      v-if="cm.status === 'pending' || cm.status === 'approved'"
                      type="button"
                      class="action-btn spam"
                      @click="handleSpam(cm)"
                    >
                      <Icon icon="ri:spam-2-line" />
                      垃圾
                    </button>
                    <button type="button" class="action-btn reply" @click="openReply(cm)">
                      <Icon icon="ri:corner-up-left-line" />
                      回复
                    </button>
                    <button type="button" class="action-btn delete" @click="handleDelete(cm)">
                      <Icon icon="ri:delete-bin-line" />
                      删除
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </Transition>

        <template #footer>
          <div class="border-t border-border bg-black/15 px-3 py-3 sm:px-5 sm:py-4">
            <div class="flex flex-col items-center gap-2 sm:flex-row sm:items-center sm:justify-between">
              <span class="text-xs text-text-tertiary sm:text-[13px]">
                共 <strong class="font-semibold text-foreground">{{ total }}</strong> 条评论
              </span>
              <VPagination
                v-model:page="page"
                v-model:size="size"
                :total="total"
                :size-options="[20, 30, 50, 100]"
              />
            </div>
          </div>
        </template>
      </VCard>
    </div>

    <!-- Reply modal -->
    <VModal v-model:visible="replyVisible" :title="`回复 @${replyTarget?.nick || ''}`" :width="480">
      <textarea
        v-model="replyContent"
        rows="4"
        placeholder="回复内容..."
        class="apple-textarea w-full"
      ></textarea>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="replyVisible = false">取消</VButton>
          <VButton type="secondary" @click="handleReply">发送</VButton>
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

/* 状态徽章：系统色 12% 透明底 + 同色文字 + ::before 圆点 */
.comment-status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: var(--radius-full, 9999px);
  letter-spacing: 0.2px;
}
.comment-status-badge::before {
  content: "";
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.comment-status-badge.approved {
  color: #30d158;
  background-color: color-mix(in srgb, var(--color-state-success) 12%, transparent);
}
.comment-status-badge.pending {
  color: #ff9f0a;
  background-color: color-mix(in srgb, var(--color-state-warning) 12%, transparent);
}
.comment-status-badge.rejected {
  color: #ff453a;
  background-color: color-mix(in srgb, var(--color-state-error) 12%, transparent);
}
.comment-status-badge.spam {
  color: var(--color-text-tertiary);
  background-color: color-mix(in srgb, var(--color-muted-foreground) 15%, transparent);
}

/* 操作按钮：语义色低透明胶囊 */
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 500;
  border-radius: var(--radius-full, 9999px);
  border: 1px solid transparent;
  background-color: transparent;
  color: var(--color-text-secondary);
  transition: all 0.18s var(--ease-standard);
}
.action-btn:hover {
  background-color: color-mix(in srgb, #fff 5%, transparent);
  color: #fff;
}
.action-btn :deep(svg) {
  width: 13px;
  height: 13px;
}
.action-btn.approve {
  color: #30d158;
  background-color: color-mix(in srgb, var(--color-state-success) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-state-success) 15%, transparent);
}
.action-btn.approve:hover {
  background-color: color-mix(in srgb, var(--color-state-success) 20%, transparent);
  border-color: color-mix(in srgb, var(--color-state-success) 25%, transparent);
}
.action-btn.reply {
  color: var(--color-primary);
  background-color: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 15%, transparent);
}
.action-btn.reply:hover {
  background-color: color-mix(in srgb, var(--color-primary) 20%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 25%, transparent);
}
.action-btn.reject {
  color: #ff9f0a;
  background-color: color-mix(in srgb, var(--color-state-warning) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-state-warning) 15%, transparent);
}
.action-btn.reject:hover {
  background-color: color-mix(in srgb, var(--color-state-warning) 20%, transparent);
  border-color: color-mix(in srgb, var(--color-state-warning) 25%, transparent);
}
.action-btn.spam {
  color: var(--color-text-tertiary);
  background-color: color-mix(in srgb, var(--color-muted-foreground) 15%, transparent);
  border-color: color-mix(in srgb, var(--color-muted-foreground) 18%, transparent);
}
.action-btn.spam:hover {
  background-color: color-mix(in srgb, var(--color-muted-foreground) 25%, transparent);
  border-color: color-mix(in srgb, var(--color-muted-foreground) 30%, transparent);
}
.action-btn.delete {
  color: #ff453a;
  background-color: color-mix(in srgb, var(--color-state-error) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-state-error) 15%, transparent);
}
.action-btn.delete:hover {
  background-color: color-mix(in srgb, var(--color-state-error) 20%, transparent);
  border-color: color-mix(in srgb, var(--color-state-error) 25%, transparent);
}

/* 分段 Tab 横向滚动，不显示滚动条（无条件生效：任何宽度下标签溢出都可横滚） */
.tabs-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.tabs-scroll::-webkit-scrollbar {
  display: none;
}

/* ===== 移动端适配 ===== */
@media (max-width: 640px) {
  /* 操作按钮更紧凑 */
  .action-btn {
    padding: 4px 10px;
    font-size: 11px;
  }
  .action-btn :deep(svg) {
    width: 12px;
    height: 12px;
  }

  /* 状态徽章更紧凑 */
  .comment-status-badge {
    font-size: 10px;
    padding: 2px 8px;
  }
}
</style>
