<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { commentUsersApi, commentApi, type CommentUserItem, type CommentStatus } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VCard, VEmpty, VLoading, VPagination, VModal, Dialog, Toast } from "@/components/ui";
import { formatRelative } from "@/lib/format";

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

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
  queryKey: computed(() => ["comment-users", queryParams.value] as const),
  queryFn: () => commentUsersApi.list(queryParams.value).then(({ data }) => data),
});

const users = computed<CommentUserItem[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);
const counts = computed(() => queryData.value?.counts || { all: 0, banned: 0 });

function invalidate() {
  queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] === "comment-users" });
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
  } catch {
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
      } catch {}
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
  post_slug: string;
  content: string;
  status: CommentStatus;
  created_at?: string;
  date?: string;
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
  } catch {
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
    if (action === "delete" && commentsTotal.value === 0) {
      commentsVisible.value = false;
    }
  } catch {
  } finally {
    commentsBusy.value = false;
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="评论用户">
      <template #icon>
        <Icon icon="ri:user-shared-line" class="text-primary" />
      </template>
      <template #subtitle>管理注册读者账户、评论活跃度画像、封禁控制与按用户批量清理</template>
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="keyword" placeholder="搜索读者昵称或用户名..." />
          <button v-if="keyword" type="button" class="text-text-quaternary hover:text-foreground" @click="keyword = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton :route="{ path: '/comments' }">
          <template #icon><Icon icon="ri:chat-3-line" /></template>
          返回评论列表
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <div class="stats-card">
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:user-community-line" />
          </div>
          <div>
            <div class="stats-label">注册读者总数</div>
            <div class="stats-value">{{ counts.all }} <span class="stats-sub">位注册读者</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:checkbox-circle-line" />
          </div>
          <div>
            <div class="stats-label">正常活跃读者</div>
            <div class="stats-value text-[#30D158]">
              {{ Math.max(0, counts.all - counts.banned) }} <span class="stats-sub">可正常互动</span>
            </div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#FF453A] bg-[#FF453A]/10">
            <Icon icon="ri:forbid-2-line" />
          </div>
          <div>
            <div class="stats-label">已封禁读者</div>
            <div class="stats-value text-[#FF453A]">
              {{ counts.banned }} <span class="stats-sub">禁止登录留言</span>
            </div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
            <Icon icon="ri:chat-voice-line" />
          </div>
          <div>
            <div class="stats-label">当前列表用户</div>
            <div class="stats-value">
              {{ users.length }} <span class="stats-sub">共 {{ total }} 条记录</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 用户列表卡片 -->
      <VCard>
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-3 w-full">
            <div class="apple-segmented">
              <button type="button" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">
                全部读者
              </button>
              <button type="button" :class="{ active: statusFilter === 'banned' }" @click="statusFilter = 'banned'">
                已封禁读者
              </button>
            </div>

            <VButton size="sm" @click="refetch()">
              <template #icon><Icon icon="ri:refresh-line" /></template>
              刷新数据
            </VButton>
          </div>
        </template>

        <VLoading v-if="isLoading" />
        <VEmpty v-else-if="!users.length" title="暂无对应读者记录" />

        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[760px] text-left">
            <thead>
              <tr class="border-b border-border text-xs font-medium text-text-quaternary">
                <th class="px-4 py-3">读者账号</th>
                <th class="px-4 py-3">发表评论数</th>
                <th class="px-4 py-3">最后活跃时间</th>
                <th class="px-4 py-3">账号状态</th>
                <th class="w-48 px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/60">
              <tr v-for="u in users" :key="u.id" class="transition-colors hover:bg-secondary/40">
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-3">
                    <img
                      v-if="u.avatar"
                      :src="u.avatar"
                      class="h-9 w-9 shrink-0 rounded-full border border-border object-cover bg-secondary"
                      alt=""
                    />
                    <div
                      v-else
                      class="avatar-gradient-1 flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-bold text-white"
                    >
                      {{ (u.display_name || u.username).charAt(0).toUpperCase() }}
                    </div>

                    <div class="min-w-0">
                      <div class="font-semibold text-foreground text-sm flex items-center gap-2">
                        <span>{{ u.display_name || u.username }}</span>
                        <span v-if="u.display_name" class="text-xs font-mono text-text-quaternary"
                          >(@{{ u.username }})</span
                        >
                      </div>
                      <div class="text-xs font-mono text-text-quaternary truncate max-w-[240px]">
                        {{ u.email || "未绑定邮箱" }}
                      </div>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3.5 font-mono text-sm font-semibold text-foreground">
                  {{ u.comment_count || 0 }}
                </td>
                <td class="px-4 py-3.5 text-xs text-text-tertiary">
                  {{ u.last_comment_at ? formatRelative(u.last_comment_at) : "暂无留言" }}
                </td>
                <td class="px-4 py-3.5 text-xs">
                  <span
                    class="rounded-full px-2 py-0.5 text-[10px] font-medium"
                    :class="u.status === 'banned' ? 'bg-[#FF453A]/10 text-[#FF453A]' : 'bg-[#30D158]/10 text-[#30D158]'"
                  >
                    {{ u.status === "banned" ? "已封禁" : "正常" }}
                  </span>
                </td>
                <td class="px-4 py-3.5 text-right">
                  <div class="flex items-center justify-end gap-1.5">
                    <button type="button" class="action-btn text-primary hover:bg-primary/10" @click="openComments(u)">
                      <Icon icon="ri:chat-3-line" />
                      <span>查看评论</span>
                    </button>

                    <button
                      v-if="u.status !== 'banned'"
                      type="button"
                      class="action-btn text-[#FF9500] hover:bg-[#FF9500]/10"
                      :disabled="banBusy === u.id"
                      @click="confirmBan(u)"
                    >
                      <Icon icon="ri:forbid-2-line" />
                      <span>封禁</span>
                    </button>
                    <button
                      v-else
                      type="button"
                      class="action-btn text-[#30D158] hover:bg-[#30D158]/10"
                      :disabled="banBusy === u.id"
                      @click="setStatus(u, 'active')"
                    >
                      <Icon icon="ri:checkbox-circle-line" />
                      <span>解封</span>
                    </button>

                    <button
                      type="button"
                      class="action-btn text-state-error hover:bg-state-error/10"
                      @click="confirmDelete(u)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 分页 -->
        <div v-if="total > size" class="flex justify-center pt-4">
          <VPagination
            :page="page"
            :size="size"
            :total="total"
            @update:page="page = $event"
            @update:size="size = $event"
          />
        </div>
      </VCard>
    </div>

    <!-- ============ 用户名下评论弹窗 ============ -->
    <VModal
      v-model:visible="commentsVisible"
      :title="`「${commentsTarget?.display_name || commentsTarget?.username}」的全部评论`"
      :width="680"
    >
      <div class="space-y-4">
        <div class="flex items-center justify-between border-b border-border/60 pb-3">
          <span class="text-xs text-text-tertiary">共 {{ commentsTotal }} 条记录</span>
          <div class="flex items-center gap-2">
            <VButton size="sm" :disabled="commentsBusy || !commentRows.length" @click="confirmBulk('spam')">
              全部标为垃圾
            </VButton>
            <VButton
              size="sm"
              type="danger"
              :disabled="commentsBusy || !commentRows.length"
              @click="confirmBulk('delete')"
            >
              全部删除
            </VButton>
          </div>
        </div>

        <VLoading v-if="commentsLoading" />
        <div v-else-if="!commentRows.length" class="py-12 text-center text-xs text-text-quaternary">
          该用户名下暂无评论
        </div>
        <div v-else class="space-y-2.5 max-h-[420px] overflow-y-auto pr-1">
          <div v-for="row in commentRows" :key="row.id" class="rounded-xl border border-border bg-card p-3 text-xs">
            <div class="flex items-center justify-between text-text-quaternary mb-1.5">
              <span
                >在文章: <strong class="text-foreground">{{ row.post_slug }}</strong></span
              >
              <span class="rounded px-1.5 py-0.2 bg-secondary text-[10px]">{{ statusLabel(row.status) }}</span>
            </div>
            <div class="text-foreground leading-relaxed">{{ row.content }}</div>
          </div>
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
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  /* highlight removed */
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

/* 操作小按钮 */
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  transition: background-color var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}

@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
