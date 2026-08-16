<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  commentUsersApi,
  commentApi,
  type CommentUserItem,
  type CommentStatus,
} from "@/api";
import { Icon } from "@iconify/vue";
import {
  VButton,
  VCard,
  VEmpty,
  VLoading,
  VPagination,
  VStatusDot,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";

// Spec 71 评论用户管理：注册读者（role=reader）列表 + 评论聚合 +
// 封禁/解封 + 按用户查看评论与批量处置。视觉体系对齐 /console/users（UserListView）。

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);
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

// 状态筛选走后端（空串=全部 / banned）
const statusFilter = ref<"all" | "banned">("all");
watch(statusFilter, () => {
  page.value = 1;
});

const queryParams = computed(() => ({
  page: page.value,
  size: size.value,
  keyword: debouncedKeyword.value,
  status: statusFilter.value === "all" ? "" : "banned",
}));

const { data: queryData, isLoading, refetch } = useQuery({
  queryKey: computed(() => ["comment-users", queryParams.value] as const),
  queryFn: () => commentUsersApi.list(queryParams.value).then(({ data }) => data),
});

const users = computed<CommentUserItem[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);
const counts = computed(() => queryData.value?.counts || { all: 0, banned: 0 });

// 当前页评论总数（统计卡用）
const pageCommentSum = computed(() =>
  users.value.reduce((sum, u) => sum + (u.comment_count || 0), 0)
);

function invalidate() {
  queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] === "comment-users" });
}

function errMsg(e: any, fallback: string): string {
  return e?.response?.data?.error || e?.response?.data?.detail || fallback;
}

// ---------- 封禁 / 解封 ----------
const banBusy = ref(0);

function confirmBan(u: CommentUserItem) {
  Dialog.warning({
    title: `确定要封禁用户「${u.display_name || u.username}」吗？`,
    description: "封禁后该用户将立即无法登录与评论，其已有评论保留",
    confirmText: "封禁",
    confirmType: "danger",
    onConfirm: () => setStatus(u, "banned"),
  });
}

async function setStatus(u: CommentUserItem, status: "active" | "banned") {
  banBusy.value = u.id;
  try {
    await commentUsersApi.setStatus(u.id, status);
    invalidate();
    Toast.success(status === "banned" ? "已封禁" : "已解封");
  } catch (e: any) {
    Toast.error(errMsg(e, "操作失败"));
  } finally {
    banBusy.value = 0;
  }
}

// ---------- 删除用户 ----------
function confirmDelete(u: CommentUserItem) {
  Dialog.warning({
    title: `确定要删除用户「${u.username}」吗？`,
    description:
      u.comment_count > 0
        ? `该用户名下还有 ${u.comment_count} 条评论；删除账号不会删除评论，如需清理请先「查看评论」批量处置`
        : "删除后该用户将无法登录，其名下评论保留",
    confirmText: "删除",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await commentUsersApi.remove(u.id);
        invalidate();
        Toast.success("已删除");
      } catch (e: any) {
        Toast.error(errMsg(e, "删除失败"));
      }
    },
  });
}

// ---------- 查看评论弹窗 ----------
const commentsVisible = ref(false);
const commentsTarget = ref<CommentUserItem | null>(null);
const commentsPage = ref(1);
const commentsLoading = ref(false);
const commentsBusy = ref(false);

interface CommentRow {
  id: number;
  page_key: string;
  content: string;
  status: CommentStatus;
  date: string;
}
const commentRows = ref<CommentRow[]>([]);
const commentsTotal = ref(0);

async function openComments(u: CommentUserItem) {
  commentsTarget.value = u;
  commentsPage.value = 1;
  commentsVisible.value = true;
  await loadComments();
}

async function loadComments() {
  const target = commentsTarget.value;
  if (!target) return;
  commentsLoading.value = true;
  try {
    const { data } = await commentApi.list({
      user_id: target.id,
      page: commentsPage.value,
      size: 10,
    });
    commentRows.value = (data.items || []) as CommentRow[];
    commentsTotal.value = data.total || 0;
  } catch (e: any) {
    Toast.error(errMsg(e, "加载评论失败"));
  } finally {
    commentsLoading.value = false;
  }
}

watch(commentsPage, () => {
  if (commentsVisible.value) loadComments();
});

function statusLabel(s: string): string {
  return { pending: "待审", approved: "已通过", rejected: "已拒绝", spam: "垃圾" }[s] || s;
}

// 批量处置：全部标为垃圾 / 全部删除
function confirmBulk(action: "spam" | "delete") {
  const target = commentsTarget.value;
  if (!target) return;
  const isDelete = action === "delete";
  Dialog.warning({
    title: isDelete
      ? `删除「${target.display_name || target.username}」的全部评论？`
      : `将「${target.display_name || target.username}」的全部评论标为垃圾？`,
    description: isDelete
      ? "该用户名下所有评论（含其根评论下的回复）将被永久删除，不可恢复"
      : "该用户名下所有评论将被标记为垃圾，不再前台展示",
    confirmText: isDelete ? "全部删除" : "全部标为垃圾",
    confirmType: "danger",
    onConfirm: () => doBulk(action),
  });
}

async function doBulk(action: "spam" | "delete") {
  const target = commentsTarget.value;
  if (!target) return;
  commentsBusy.value = true;
  try {
    const { data } = await commentUsersApi.bulkComments(target.id, action);
    Toast.success(`已处置 ${data.affected} 条评论`);
    invalidate();
    await loadComments();
    // 全部评论被删后可直接关闭弹窗
    if (action === "delete" && commentsTotal.value === 0) {
      commentsVisible.value = false;
    }
  } catch (e: any) {
    Toast.error(errMsg(e, "批量处置失败"));
  } finally {
    commentsBusy.value = false;
  }
}

// ---------- 展示辅助 ----------
function avatarGradient(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) & 0xffffffff;
  }
  const n = (Math.abs(hash) % 8) + 1;
  return `avatar-gradient-${n}`;
}

function avatarInitial(u: CommentUserItem): string {
  const source = u.display_name?.trim() || u.username || "?";
  return source.charAt(0).toUpperCase();
}
</script>

<template>
  <div class="min-h-full">
    <!-- ===== Page Header ===== -->
    <header
      class="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-2 px-4 py-4 backdrop-blur-xl sm:gap-3 sm:px-5 sm:py-5 md:px-8"
      style="background: linear-gradient(180deg, color-mix(in srgb, #000 90%, transparent) 0%, color-mix(in srgb, #000 60%, transparent) 80%, transparent 100%);"
    >
      <div class="flex flex-col gap-0.5 sm:gap-1">
        <h1 class="text-xl font-bold leading-tight tracking-tight text-foreground sm:text-[28px]">评论用户</h1>
        <p class="text-[12px] text-text-tertiary sm:text-[13px]">管理注册读者：评论统计、封禁解封、按用户批量处置</p>
      </div>
    </header>

    <div class="px-4 pb-8 sm:px-5 md:px-8">
      <!-- ===== Stat Cards ===== -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <!-- 评论用户总数 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">评论用户总数</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-primary"
              style="background-color: color-mix(in srgb, var(--color-primary) 10%, transparent);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:user-community-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-foreground"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ counts.all }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span class="text-[12px] text-text-tertiary">注册读者账户</span>
          </div>
        </div>

        <!-- 已封禁 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">已封禁</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-state-error"
              style="background-color: color-mix(in srgb, var(--color-state-error) 10%, transparent);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:forbid-2-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-foreground"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ counts.banned }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span class="text-[12px] text-text-tertiary">无法登录与评论</span>
          </div>
        </div>

        <!-- 当前页评论数 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">当前页评论数</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-chart-5"
              style="background-color: color-mix(in srgb, var(--color-chart-5) 10%, transparent);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:chat-3-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-foreground"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ pageCommentSum }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span class="text-[12px] text-text-tertiary">本页用户合计</span>
          </div>
        </div>

        <!-- 当前页用户 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">当前页用户</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-state-success"
              style="background-color: color-mix(in srgb, var(--color-state-success) 10%, transparent);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:eye-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-foreground"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ users.length }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span class="text-[12px] text-text-tertiary">共 {{ total }} 条</span>
          </div>
        </div>
      </div>

      <!-- ===== User Table Card ===== -->
      <VCard class="mt-5" :body-class="['!p-0']">
        <template #header>
          <div class="flex flex-col gap-3 p-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:p-4">
            <div class="tabs-scroll apple-segmented -mx-1 px-1 sm:mx-0 sm:px-0">
              <button
                type="button"
                :class="{ active: statusFilter === 'all' }"
                @click="statusFilter = 'all'"
              >
                全部
              </button>
              <button
                type="button"
                :class="{ active: statusFilter === 'banned' }"
                @click="statusFilter = 'banned'"
              >
                已封禁
              </button>
            </div>
            <div class="flex w-full items-center gap-2 sm:w-auto">
              <div class="flex-1 sm:w-[220px] sm:flex-none">
                <div class="apple-search w-full">
                  <Icon icon="ri:search-line" class="text-text-quaternary" />
                  <input v-model="keyword" placeholder="搜索用户..." />
                </div>
              </div>
              <button
                type="button"
                title="刷新"
                class="hidden h-9 items-center gap-1.5 rounded-full border border-border bg-card px-3.5 text-[13px] font-semibold text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-foreground sm:inline-flex"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" />
                刷新
              </button>
              <button
                type="button"
                title="刷新"
                class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-border bg-card text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-foreground sm:hidden"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" class="text-base" />
              </button>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />
        <VEmpty v-else-if="!users.length" title="暂无评论用户" />

        <template v-else>
          <!-- 桌面端表格 -->
          <div class="hidden sm:block">
            <div class="overflow-x-auto">
              <table class="w-full border-collapse">
                <thead>
                  <tr>
                    <th
                      class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                    >
                      用户
                    </th>
                    <th
                      class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                    >
                      评论数
                    </th>
                    <th
                      class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                    >
                      最后评论
                    </th>
                    <th
                      class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                    >
                      状态
                    </th>
                    <th
                      class="whitespace-nowrap border-b border-border px-5 py-3 text-right text-[12px] font-semibold tracking-wide text-text-tertiary"
                    >
                      操作
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="u in users"
                    :key="u.id"
                    class="transition-colors duration-150 hover:bg-white/[0.03]"
                  >
                    <td class="border-b border-border px-5 py-3.5 align-middle">
                      <div class="flex items-center gap-3">
                        <img
                          v-if="u.avatar"
                          :src="u.avatar"
                          :alt="u.username"
                          loading="lazy"
                          class="h-[38px] w-[38px] shrink-0 rounded-full object-cover"
                        />
                        <div
                          v-else
                          :class="avatarGradient(u.username)"
                          class="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full text-[14px] font-semibold text-foreground"
                        >
                          {{ avatarInitial(u) }}
                        </div>
                        <div class="flex min-w-0 flex-col gap-0.5">
                          <span class="text-[14px] font-semibold leading-tight text-foreground">
                            {{ u.display_name || u.username }}
                          </span>
                          <span
                            class="max-w-[220px] truncate text-[12px] leading-tight text-text-tertiary"
                          >
                            {{ u.email || u.username }}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td
                      class="border-b border-border px-5 py-3.5 align-middle text-[13px] text-text-secondary"
                      style="font-variant-numeric: tabular-nums;"
                    >
                      {{ u.comment_count }}
                    </td>
                    <td
                      class="border-b border-border px-5 py-3.5 align-middle text-[13px] text-text-secondary"
                      style="font-variant-numeric: tabular-nums;"
                    >
                      {{ u.last_comment_at || "—" }}
                    </td>
                    <td class="border-b border-border px-5 py-3.5 align-middle">
                      <span
                        v-if="u.status === 'banned'"
                        class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-medium"
                        style="background-color: color-mix(in srgb, var(--color-state-error) 12%, transparent); color: var(--color-state-error);"
                      >
                        <VStatusDot state="error" />
                        已封禁
                      </span>
                      <span
                        v-else
                        class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-medium"
                        style="background-color: color-mix(in srgb, var(--color-state-success) 12%, transparent); color: var(--color-state-success);"
                      >
                        <VStatusDot state="success" />
                        活跃
                      </span>
                    </td>
                    <td class="border-b border-border px-5 py-3.5 text-right align-middle">
                      <div class="flex items-center justify-end gap-1">
                        <button
                          type="button"
                          title="查看评论"
                          class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-foreground"
                          @click="openComments(u)"
                        >
                          <Icon icon="ri:chat-3-line" class="text-[16px]" />
                        </button>
                        <button
                          v-if="u.status !== 'banned'"
                          type="button"
                          title="封禁"
                          :disabled="banBusy === u.id"
                          class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-[var(--color-state-error)] disabled:opacity-50"
                          @click="confirmBan(u)"
                        >
                          <Icon icon="ri:forbid-2-line" class="text-[16px]" />
                        </button>
                        <button
                          v-else
                          type="button"
                          title="解封"
                          :disabled="banBusy === u.id"
                          class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-[var(--color-state-success)] disabled:opacity-50"
                          @click="setStatus(u, 'active')"
                        >
                          <Icon icon="ri:lock-unlock-line" class="text-[16px]" />
                        </button>
                        <button
                          type="button"
                          title="删除"
                          class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-[var(--color-state-error)]"
                          @click="confirmDelete(u)"
                        >
                          <Icon icon="ri:delete-bin-line" class="text-[16px]" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- 移动端卡片列表 -->
          <div class="sm:hidden user-card-list">
            <div
              v-for="u in users"
              :key="u.id"
              class="user-card border-b border-border px-3 py-3.5 last:border-b-0"
            >
              <div class="flex items-start gap-3">
                <img
                  v-if="u.avatar"
                  :src="u.avatar"
                  :alt="u.username"
                  loading="lazy"
                  class="h-10 w-10 shrink-0 rounded-full object-cover"
                />
                <div
                  v-else
                  :class="avatarGradient(u.username)"
                  class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-foreground"
                >
                  {{ avatarInitial(u) }}
                </div>

                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="truncate text-sm font-semibold text-foreground">
                      {{ u.display_name || u.username }}
                    </span>
                    <span
                      v-if="u.status === 'banned'"
                      class="inline-flex items-center rounded-full px-1.5 py-0.5 text-[11px] font-medium"
                      style="background-color: color-mix(in srgb, var(--color-state-error) 15%, transparent); color: var(--color-state-error);"
                    >
                      已封禁
                    </span>
                    <span
                      v-else
                      class="inline-flex items-center rounded-full px-1.5 py-0.5 text-[11px] font-medium"
                      style="background-color: color-mix(in srgb, var(--color-state-success) 15%, transparent); color: var(--color-state-success);"
                    >
                      活跃
                    </span>
                    <span class="ml-auto text-[11px] text-text-tertiary">
                      {{ u.comment_count }} 条评论
                    </span>
                  </div>
                  <div class="mt-0.5 truncate text-[12px] text-text-tertiary">
                    {{ u.email || u.username }}
                  </div>
                  <div class="mt-1 text-[11px] text-text-quaternary">
                    最后评论：{{ u.last_comment_at || "—" }}
                  </div>

                  <div class="mt-2.5 flex items-center gap-1">
                    <button
                      type="button"
                      title="查看评论"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
                      @click="openComments(u)"
                    >
                      <Icon icon="ri:chat-3-line" class="text-sm" />
                    </button>
                    <button
                      v-if="u.status !== 'banned'"
                      type="button"
                      title="封禁"
                      :disabled="banBusy === u.id"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-secondary transition-colors hover:bg-secondary hover:text-state-error disabled:opacity-50"
                      @click="confirmBan(u)"
                    >
                      <Icon icon="ri:forbid-2-line" class="text-sm" />
                    </button>
                    <button
                      v-else
                      type="button"
                      title="解封"
                      :disabled="banBusy === u.id"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-secondary transition-colors hover:bg-secondary hover:text-state-success disabled:opacity-50"
                      @click="setStatus(u, 'active')"
                    >
                      <Icon icon="ri:lock-unlock-line" class="text-sm" />
                    </button>
                    <button
                      type="button"
                      title="删除"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-tertiary transition-colors hover:bg-secondary hover:text-state-error"
                      @click="confirmDelete(u)"
                    >
                      <Icon icon="ri:delete-bin-line" class="text-sm" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="total > size" class="border-t border-border px-3 py-3 sm:px-4 sm:py-4">
            <div class="flex flex-col items-center gap-2 sm:flex-row sm:items-center sm:justify-end">
              <VPagination :page="page" :size="size" :total="total" @update:page="page = $event" />
            </div>
          </div>
        </template>
      </VCard>
    </div>

    <!-- 查看评论弹窗 -->
    <VModal
      v-model:visible="commentsVisible"
      :title="`「${commentsTarget?.display_name || commentsTarget?.username || ''}」的评论`"
      :width="720"
    >
      <!-- 批量操作条 -->
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <span class="text-[12px] text-text-tertiary">共 {{ commentsTotal }} 条评论</span>
        <div class="ml-auto flex items-center gap-2">
          <VButton size="sm" :loading="commentsBusy" :disabled="commentsTotal === 0" @click="confirmBulk('spam')">
            全部标为垃圾
          </VButton>
          <VButton size="sm" type="danger" :loading="commentsBusy" :disabled="commentsTotal === 0" @click="confirmBulk('delete')">
            全部删除
          </VButton>
        </div>
      </div>

      <VLoading v-if="commentsLoading" />
      <VEmpty v-else-if="!commentRows.length" title="该用户暂无评论" />
      <div v-else class="space-y-2">
        <div
          v-for="cm in commentRows"
          :key="cm.id"
          class="rounded-xl border border-border p-3"
        >
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
              :style="
                cm.status === 'approved'
                  ? { backgroundColor: 'color-mix(in srgb, var(--color-state-success) 12%, transparent)', color: 'var(--color-state-success)' }
                  : cm.status === 'spam'
                    ? { backgroundColor: 'color-mix(in srgb, var(--color-state-error) 12%, transparent)', color: 'var(--color-state-error)' }
                    : { backgroundColor: 'color-mix(in srgb, var(--color-state-warning) 15%, transparent)', color: 'var(--color-state-warning)' }
              "
            >
              {{ statusLabel(cm.status) }}
            </span>
            <span class="truncate text-[12px] text-text-quaternary">{{ cm.page_key }}</span>
            <span class="ml-auto shrink-0 text-[12px] text-text-quaternary">{{ cm.date }}</span>
          </div>
          <p class="mt-1.5 line-clamp-3 text-[13px] leading-relaxed text-text-secondary">{{ cm.content }}</p>
        </div>
        <div v-if="commentsTotal > 10" class="flex justify-end pt-2">
          <VPagination
            :page="commentsPage"
            :size="10"
            :total="commentsTotal"
            @update:page="commentsPage = $event"
          />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <VButton @click="commentsVisible = false">关闭</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
/* 分段 Tab 横向滚动，不显示滚动条（对齐 UserListView） */
.tabs-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.tabs-scroll::-webkit-scrollbar {
  display: none;
}
</style>
