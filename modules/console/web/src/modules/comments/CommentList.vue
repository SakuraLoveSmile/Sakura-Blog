<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { commentApi, type CommentStatus } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VButton,
  VEmpty,
  VLoading,
  VFloatingDock,
  VPagination,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";
import { formatRelative } from "@/lib/format";

interface CommentItem {
  id: number;
  post_slug: string;
  post_title?: string;
  parent_id?: number;
  user_id?: number;
  nick: string;
  email: string;
  website?: string;
  avatar?: string;
  ip?: string;
  browser?: string;
  os?: string;
  content: string;
  content_html?: string;
  status: CommentStatus;
  created_at: string;
}

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

// 批量模式
const selectedIds = ref<Set<number>>(new Set());

// 回复弹窗
const replyVisible = ref(false);
const replyTarget = ref<CommentItem | null>(null);
const replyContent = ref("");
const replySubmitting = ref(false);

// 键盘快捷流焦点与紧凑帮助入口
const focusedIndex = ref<number>(-1);
const shortcutHelpVisible = ref(false);

// 响应式查询参数
const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (selectedStatus.value !== "all") params.status = selectedStatus.value;
  if (debouncedKeyword.value) params.keyword = debouncedKeyword.value;
  return params;
});

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
  queryKey: computed(() => queryKeys.comments(queryParams.value)),
  queryFn: () => commentApi.list(queryParams.value).then(({ data }) => data),
});

const comments = computed<CommentItem[]>(() => (queryData.value?.items || []) as CommentItem[]);
const total = computed(() => queryData.value?.total || 0);

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
  { immediate: true },
);

function statusInfo(status: string): { cls: string; label: string } {
  switch (status) {
    case "pending":
      return { cls: "text-state-warning", label: "待审核" };
    case "approved":
      return { cls: "text-state-success", label: "已批准" };
    case "rejected":
      return { cls: "text-state-error", label: "已拒绝" };
    case "spam":
      return { cls: "text-text-tertiary", label: "垃圾评论" };
    default:
      return { cls: "text-text-tertiary", label: status };
  }
}

// ---------- 操作流 ----------
async function setCommentStatus(id: number, status: CommentStatus) {
  try {
    await commentApi.setStatus(id, status);
    await queryClient.invalidateQueries({ queryKey: queryKeys.comments() });
    await refetch();
    const map: Record<CommentStatus, string> = {
      approved: "已批准该评论",
      rejected: "已拒绝该评论",
      spam: "已标记为垃圾评论",
      pending: "已重置为待审核",
    };
    Toast.success(map[status] || "状态已更新");
  } catch {}
}

function handleDeleteSingle(c: CommentItem) {
  Dialog.warning({
    title: `确定删除来自「${c.nick}」的评论吗？`,
    description: "删除后该评论及其子回复将被永久移除。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await commentApi.delete(c.id);
        selectedIds.value.delete(c.id);
        await queryClient.invalidateQueries({ queryKey: queryKeys.comments() });
        await refetch();
        Toast.success("已删除");
      } catch {}
    },
  });
}

function openReply(c: CommentItem) {
  replyTarget.value = c;
  replyContent.value = "";
  replyVisible.value = true;
}

async function submitReply() {
  if (!replyTarget.value || !replyContent.value.trim()) {
    Toast.warning("请输入回复内容");
    return;
  }
  replySubmitting.value = true;
  try {
    await commentApi.reply({
      parent_id: replyTarget.value.id,
      content: replyContent.value.trim(),
    });
    replyVisible.value = false;
    await queryClient.invalidateQueries({ queryKey: queryKeys.comments() });
    await refetch();
    Toast.success("回复已发送");
  } catch {
  } finally {
    replySubmitting.value = false;
  }
}

// ---------- 批量操作 ----------
function toggleSelect(id: number) {
  if (selectedIds.value.has(id)) selectedIds.value.delete(id);
  else selectedIds.value.add(id);
}

function selectAll() {
  for (const c of comments.value) selectedIds.value.add(c.id);
}

function clearSelection() {
  selectedIds.value.clear();
}

// 搜索、筛选或分页使旧选择失效时清除失效选择；加载中沿用上一批数据，不伪造空态
watch(
  () => (isLoading.value ? null : comments.value.map((c) => c.id)),
  (ids) => {
    if (!ids) return;
    const visible = new Set(ids);
    for (const id of Array.from(selectedIds.value)) {
      if (!visible.has(id)) selectedIds.value.delete(id);
    }
  },
);

async function handleBatch(action: "approve" | "reject" | "spam" | "delete") {
  const ids = Array.from(selectedIds.value);
  if (!ids.length) return;

  const labels = {
    approve: "批量批准",
    reject: "批量拒绝",
    spam: "批量标为垃圾",
    delete: "批量删除",
  };

  const execute = async () => {
    try {
      await commentApi.batch({ ids, action });
      clearSelection();
      await queryClient.invalidateQueries({ queryKey: queryKeys.comments() });
      await refetch();
      Toast.success(`已${labels[action]} ${ids.length} 条评论`);
    } catch {}
  };

  if (action === "delete" || action === "spam") {
    Dialog.warning({
      title: `确定要${labels[action]}选中的 ${ids.length} 条评论吗？`,
      description: action === "delete" ? "删除后不可恢复。" : "标记为垃圾后将不再前台展示。",
      confirmType: "danger",
      onConfirm: execute,
    });
  } else {
    execute();
  }
}

// ---------- 用户信息抽屉 ----------
const userDrawerTarget = ref<CommentItem | null>(null);

function openUserDrawer(c: CommentItem) {
  userDrawerTarget.value = c;
}

// ---------- 键盘流极速审核 ----------
function onKeydown(e: KeyboardEvent) {
  // 如果在输入框中打字，不拦截快捷键
  const tag = (e.target as HTMLElement)?.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || replyVisible.value) {
    return;
  }
  if (e.key === "Escape" && shortcutHelpVisible.value) {
    shortcutHelpVisible.value = false;
    return;
  }

  const list = comments.value;
  if (!list.length) return;

  if (e.key === "j" || e.key === "J" || e.key === "ArrowDown") {
    e.preventDefault();
    if (focusedIndex.value < list.length - 1) focusedIndex.value++;
    else focusedIndex.value = 0;
  } else if (e.key === "k" || e.key === "K" || e.key === "ArrowUp") {
    e.preventDefault();
    if (focusedIndex.value > 0) focusedIndex.value--;
    else focusedIndex.value = list.length - 1;
  } else if (focusedIndex.value >= 0 && focusedIndex.value < list.length) {
    const target = list[focusedIndex.value];
    if (e.key === "a" || e.key === "A") {
      e.preventDefault();
      setCommentStatus(target.id, "approved");
    } else if (e.key === "d" || e.key === "D") {
      e.preventDefault();
      setCommentStatus(target.id, "rejected");
    } else if (e.key === "s" || e.key === "S") {
      e.preventDefault();
      setCommentStatus(target.id, "spam");
    } else if (e.key === "r" || e.key === "R") {
      e.preventDefault();
      openReply(target);
    } else if (e.key === "x" || e.key === "X") {
      e.preventDefault();
      toggleSelect(target.id);
    }
  }
}

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="评论管理">
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="keyword" placeholder="搜索昵称、邮箱或正文..." />
          <button v-if="keyword" type="button" class="text-text-quaternary hover:text-foreground" @click="keyword = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton :route="{ path: '/comment-users' }">
          <template #icon><Icon icon="ri:user-shared-line" /></template>
          评论用户
        </VButton>

        <div class="shortcut-help-wrap">
          <button
            type="button"
            class="shortcut-help-trigger"
            aria-label="评论快捷键"
            aria-controls="comment-shortcuts"
            :aria-expanded="shortcutHelpVisible"
            @click="shortcutHelpVisible = !shortcutHelpVisible"
          >
            <Icon icon="ri:keyboard-line" />
            <span class="hidden sm:inline">快捷键</span>
          </button>
          <div
            v-if="shortcutHelpVisible"
            id="comment-shortcuts"
            class="shortcut-help-popover"
            role="dialog"
            aria-label="评论快捷键"
          >
            <div class="shortcut-help-head">
              <span>评论快捷键</span>
              <button type="button" aria-label="关闭快捷键帮助" @click="shortcutHelpVisible = false">
                <Icon icon="ri:close-line" />
              </button>
            </div>
            <dl>
              <div><dt>J / K</dt><dd>上下翻选评论</dd></div>
              <div><dt>A</dt><dd>批准通过</dd></div>
              <div><dt>D</dt><dd>拒绝显示</dd></div>
              <div><dt>S</dt><dd>标为垃圾</dd></div>
              <div><dt>R</dt><dd>快捷回复</dd></div>
              <div><dt>X</dt><dd>勾选 / 取消</dd></div>
            </dl>
          </div>
        </div>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 按状态筛选评论 -->
      <div class="console-summary" role="group" aria-label="评论状态摘要">
        <button
          type="button"
          class="stats-card"
          :aria-pressed="selectedStatus === 'all'"
          :class="{ active: selectedStatus === 'all' }"
          @click="
            selectedStatus = 'all';
            page = 1;
          "
        >
          <span>
            <span class="stats-label">全部评论</span>
            <span class="stats-value">{{ statusCounts.all }}</span>
          </span>
        </button>

        <button
          type="button"
          class="stats-card"
          :aria-pressed="selectedStatus === 'pending'"
          :class="{ active: selectedStatus === 'pending' }"
          @click="
            selectedStatus = 'pending';
            page = 1;
          "
        >
          <span>
            <span class="stats-label">待审核</span>
            <span class="stats-value">{{ statusCounts.pending }}</span>
          </span>
        </button>

        <button
          type="button"
          class="stats-card"
          :aria-pressed="selectedStatus === 'approved'"
          :class="{ active: selectedStatus === 'approved' }"
          @click="
            selectedStatus = 'approved';
            page = 1;
          "
        >
          <span>
            <span class="stats-label">已批准展示</span>
            <span class="stats-value">{{ statusCounts.approved }}</span>
          </span>
        </button>

        <button
          type="button"
          class="stats-card"
          :aria-pressed="selectedStatus === 'spam'"
          :class="{ active: selectedStatus === 'spam' }"
          @click="
            selectedStatus = 'spam';
            page = 1;
          "
        >
          <span>
            <span class="stats-label">垃圾拦截</span>
            <span class="stats-value">{{ statusCounts.spam }}</span>
          </span>
        </button>
      </div>

      <!-- 状态分段栏与批量控制 -->
      <div class="console-toolbar">
        <div class="apple-segmented" role="group" aria-label="评论状态">
          <button
            v-for="st in statusTabs"
            :key="st.key"
            type="button"
            :class="{ active: selectedStatus === st.key }"
            :aria-pressed="selectedStatus === st.key"
            @click="
              selectedStatus = st.key;
              page = 1;
            "
          >
            {{ st.label }}
            <span v-if="statusCounts[st.key] > 0" class="ml-1 opacity-70 font-mono">
              ({{ statusCounts[st.key] }})
            </span>
          </button>
        </div>

        <div class="flex items-center gap-2">
          <button
            v-if="comments.length > 0"
            type="button"
            class="apple-filter-btn rounded-lg border border-border px-3 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            @click="selectedIds.size === comments.length ? clearSelection() : selectAll()"
          >
            {{ selectedIds.size === comments.length ? "取消全选" : "全选本页" }}
          </button>
        </div>
      </div>

      <VLoading v-if="isLoading" />

      <VEmpty
        v-else-if="comments.length === 0"
        :title="keyword ? '未找到匹配的评论' : '暂无对应状态的评论'"
        :message="keyword ? '换个搜索关键词试试' : '当读者在前台或文章中留言时将在此处列出'"
      />

      <!-- 评论分隔线行 -->
      <div v-else class="comment-rows">
        <div
          v-for="(c, idx) in comments"
          :key="c.id"
          class="comment-row group"
          :class="{
            'is-selected': selectedIds.has(c.id),
            'is-focused': focusedIndex === idx,
          }"
        >
          <div class="comment-row-main">
            <!-- 评论者信息 -->
            <div class="flex items-start gap-3 min-w-0">
              <label class="comment-select">
                <input
                  type="checkbox"
                  :checked="selectedIds.has(c.id)"
                  class="h-4 w-4 rounded-sm border-border text-text-secondary cursor-pointer shrink-0"
                  @change="toggleSelect(c.id)"
                />
              </label>

              <!-- 头像 -->
              <img
                v-if="c.avatar"
                :src="c.avatar"
                alt=""
                class="h-9 w-9 rounded-full border border-border object-cover bg-secondary shrink-0 cursor-pointer"
                @click="openUserDrawer(c)"
              />
              <div
                v-else
                class="avatar-gradient-1 flex h-9 w-9 items-center justify-center rounded-full text-xs font-bold text-white shrink-0 cursor-pointer"
                @click="openUserDrawer(c)"
              >
                {{ c.nick ? c.nick.charAt(0).toUpperCase() : "?" }}
              </div>

              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    class="text-[14px] font-medium text-foreground truncate cursor-pointer hover:underline"
                    @click="openUserDrawer(c)"
                  >
                    {{ c.nick || "匿名访客" }}
                  </span>
                  <span class="comment-status" :class="statusInfo(c.status).cls">
                    {{ statusInfo(c.status).label }}
                  </span>
                  <span v-if="c.email" class="min-w-0 text-xs font-mono text-text-quaternary truncate">
                    &lt;{{ c.email }}&gt;
                  </span>
                </div>

                <!-- 关联文章 / 单页 / 来源 -->
                <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-text-quaternary">
                  <span v-if="c.post_title || c.post_slug" class="flex items-center gap-1 text-primary hover:underline">
                    <Icon icon="ri:article-line" />
                    <span>{{ c.post_title || c.post_slug }}</span>
                  </span>
                  <span v-if="(c.post_title || c.post_slug) && c.created_at">·</span>
                  <span v-if="c.created_at">{{ formatRelative(c.created_at) }}</span>
                  <span v-if="c.ip" class="font-mono text-[11px]">IP: {{ c.ip }}</span>
                </div>
              </div>
            </div>

            <!-- 操作按钮组 -->
            <div class="comment-actions">
              <button
                v-if="c.status !== 'approved'"
                type="button"
                class="action-btn text-state-success"
                title="批准通过 (A)"
                @click="setCommentStatus(c.id, 'approved')"
              >
                <Icon icon="ri:check-line" />
                <span class="hidden sm:inline">批准</span>
              </button>

              <button
                v-if="c.status !== 'rejected'"
                type="button"
                class="action-btn text-state-warning"
                title="拒绝显示 (D)"
                @click="setCommentStatus(c.id, 'rejected')"
              >
                <Icon icon="ri:close-line" />
                <span class="hidden sm:inline">拒绝</span>
              </button>

              <button
                v-if="c.status !== 'spam'"
                type="button"
                class="action-btn text-text-tertiary"
                title="标为垃圾 (S)"
                @click="setCommentStatus(c.id, 'spam')"
              >
                <Icon icon="ri:spam-2-line" />
              </button>

              <button
                type="button"
                class="action-btn text-text-secondary"
                title="快捷回复 (R)"
                @click="openReply(c)"
              >
                <Icon icon="ri:reply-line" />
              </button>

              <button
                type="button"
                class="action-btn text-state-error"
                title="删除评论"
                @click="handleDeleteSingle(c)"
              >
                <Icon icon="ri:delete-bin-line" />
              </button>
            </div>
          </div>

          <!-- 评论富文本正文 -->
          <div
            v-if="c.content_html"
            class="comment-rich-content mt-2 text-[13px] leading-relaxed text-foreground pl-11"
            v-html="c.content_html"
          ></div>
          <p v-else class="mt-2 text-[13px] leading-relaxed text-foreground pl-11 whitespace-pre-wrap">
            {{ c.content }}
          </p>
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
      </div>
    </div>

    <!-- ============ 批量操作浮动栏 ============ -->
    <VFloatingDock :visible="selectedIds.size > 0" :count="selectedIds.size" title="已选评论" @clear="clearSelection">
      <div class="flex items-center gap-2">
        <VButton size="sm" @click="handleBatch('approve')"> 批量批准 </VButton>
        <VButton size="sm" @click="handleBatch('reject')"> 批量拒绝 </VButton>
        <VButton size="sm" @click="handleBatch('spam')"> 标为垃圾 </VButton>
        <VButton size="sm" type="danger" @click="handleBatch('delete')"> 批量删除 </VButton>
      </div>
    </VFloatingDock>

    <!-- ============ 快捷回复弹窗 ============ -->
    <VModal v-model:visible="replyVisible" :title="`回复给：${replyTarget?.nick || '访客'}`" :width="480">
      <div class="space-y-3">
        <div class="rounded-xl border border-border bg-secondary/40 p-3 text-xs text-text-tertiary">
          <div class="font-medium text-foreground mb-1">原始评论：</div>
          <div class="line-clamp-3">{{ replyTarget?.content }}</div>
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-text-tertiary">博主回复内容 *</label>
          <textarea
            v-model="replyContent"
            rows="4"
            class="apple-textarea"
            placeholder="写下你的回复..."
            autofocus
          ></textarea>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="replyVisible = false">取消</VButton>
          <VButton type="primary" :loading="replySubmitting" @click="submitReply">发送回复</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 访客/用户画像抽屉 ============ -->
    <VModal :visible="!!userDrawerTarget" title="评论者画像详情" :width="440" @update:visible="userDrawerTarget = null">
      <div v-if="userDrawerTarget" class="space-y-4 text-xs">
        <div class="flex items-center gap-3 border-b border-border/60 pb-3">
          <img
            v-if="userDrawerTarget.avatar"
            :src="userDrawerTarget.avatar"
            class="h-12 w-12 rounded-full border border-border object-cover"
            alt=""
          />
          <div
            v-else
            class="avatar-gradient-1 flex h-12 w-12 items-center justify-center rounded-full text-sm font-bold text-white"
          >
            {{ userDrawerTarget.nick ? userDrawerTarget.nick.charAt(0).toUpperCase() : "?" }}
          </div>
          <div>
            <div class="text-sm font-bold text-foreground">{{ userDrawerTarget.nick }}</div>
            <div class="text-text-quaternary font-mono">{{ userDrawerTarget.email || "未填邮箱" }}</div>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2.5">
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">IP 地址</div>
            <div class="font-mono font-medium text-foreground mt-0.5">{{ userDrawerTarget.ip || "未知" }}</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">客户端环境</div>
            <div class="font-mono font-medium text-foreground mt-0.5 truncate" :title="userDrawerTarget.browser">
              {{ userDrawerTarget.browser || userDrawerTarget.os || "未知浏览器" }}
            </div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">个人主页</div>
            <div class="font-medium text-foreground mt-0.5 truncate">
              <a
                v-if="userDrawerTarget.website"
                :href="userDrawerTarget.website"
                target="_blank"
                class="text-primary hover:underline"
              >
                {{ userDrawerTarget.website }}
              </a>
              <span v-else class="text-text-quaternary">无</span>
            </div>
          </div>
          <div class="rounded-xl border border-border bg-card p-3">
            <div class="text-text-quaternary">留言时间</div>
            <div class="font-medium text-foreground mt-0.5">{{ formatRelative(userDrawerTarget.created_at) }}</div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <VButton @click="userDrawerTarget = null">关闭</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
.comment-rows {
  display: flex;
  flex-direction: column;
}

.comment-row {
  padding: 16px 0;
  border-bottom: 1px solid var(--color-border);
  transition: background-color var(--duration-fast) var(--ease-standard);
}
.comment-row.is-selected {
  background: var(--color-accent);
}
.comment-row.is-focused {
  box-shadow: inset 2px 0 0 var(--color-foreground);
}

.comment-row-main {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.comment-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.comment-select {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-top: -4px;
  flex-shrink: 0;
  cursor: pointer;
}

.comment-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  white-space: nowrap;
  font-size: 12px;
}
.comment-status::before {
  content: "";
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  transition: background-color var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-btn:hover {
  background: var(--color-accent);
  border-color: var(--color-border);
  color: var(--color-foreground);
}

.comment-actions {
  transition: opacity var(--duration-fast) var(--ease-standard);
}
@media (hover: hover) and (min-width: 1024px) {
  .comment-actions { opacity: 0; }
  .comment-row:hover .comment-actions,
  .comment-row:focus-within .comment-actions,
  .comment-row.is-focused .comment-actions,
  .comment-row.is-selected .comment-actions { opacity: 1; }
}

/* 紧凑快捷键帮助：入口 44px，弹层只展示键位，不再恢复整宽横幅。 */
.shortcut-help-wrap {
  position: relative;
  display: inline-flex;
}

.shortcut-help-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 44px;
  min-height: 44px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 13px;
  transition: border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

.shortcut-help-trigger:hover,
.shortcut-help-trigger[aria-expanded="true"] {
  border-color: var(--color-border-strong);
  color: var(--color-foreground);
}

.shortcut-help-trigger:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.shortcut-help-popover {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 30;
  width: 232px;
  padding: 10px 12px;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-md);
  background: var(--color-popover);
  box-shadow: var(--shadow-lg);
}

.shortcut-help-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  font-size: 12px;
}

.shortcut-help-head button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
}

.shortcut-help-head button:hover {
  color: var(--color-foreground);
  background: var(--color-accent);
}

.shortcut-help-popover dl {
  display: grid;
  gap: 4px;
  margin: 8px 0 0;
}

.shortcut-help-popover dl > div {
  display: grid;
  grid-template-columns: 58px minmax(0, 1fr);
  gap: 8px;
  align-items: baseline;
}

.shortcut-help-popover dt {
  color: var(--color-foreground);
  font-family: var(--font-mono);
  font-size: 11px;
}

.shortcut-help-popover dd {
  margin: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
}

/* 宽屏时入口靠右，弹层向右对齐；窄屏入口换行到左侧，改为向左展开。 */
@media (min-width: 1024px) {
  .shortcut-help-popover {
    left: auto;
    right: 0;
  }
}

/* 表情图片样式 */
:deep(.comment-rich-content img[alt="emoji"]),
:deep(.comment-rich-content img[src*="artalk"]),
:deep(.comment-rich-content img[src*="emoticons"]) {
  display: inline-block !important;
  height: 1.45em !important;
  width: 1.45em !important;
  vertical-align: -0.22em !important;
  margin: 0 2px !important;
  object-fit: contain !important;
}

@media (max-width: 1023px) {
  .action-btn {
    min-width: 44px;
    min-height: 44px;
    justify-content: center;
  }
  .comment-select {
    width: 44px;
    height: 44px;
    margin-top: -14px;
    margin-left: -10px;
  }
}

@media (max-width: 767px) {
  /* 窄屏把 5 个 44px 操作按钮移到信息行下方，避免作者/状态/邮箱被挤到不可读。 */
  .comment-row-main {
    flex-wrap: wrap;
  }
  .comment-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
