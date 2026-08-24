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
const statusTabs: { key: StatusKey; label: string; icon: string }[] = [
  { key: "all", label: "全部", icon: "ri:chat-3-line" },
  { key: "pending", label: "待审核", icon: "ri:time-line" },
  { key: "approved", label: "已批准", icon: "ri:checkbox-circle-line" },
  { key: "rejected", label: "已拒绝", icon: "ri:close-circle-line" },
  { key: "spam", label: "垃圾评论", icon: "ri:spam-2-line" },
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

// 键盘快捷流焦点
const focusedIndex = ref<number>(-1);
const keyboardHelpVisible = ref(false);

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
      return { cls: "bg-[#FF9500]/10 text-[#FF9500] border-[#FF9500]/20", label: "待审核" };
    case "approved":
      return { cls: "bg-[#30D158]/10 text-[#30D158] border-[#30D158]/20", label: "已批准" };
    case "rejected":
      return { cls: "bg-[#FF453A]/10 text-[#FF453A] border-[#FF453A]/20", label: "已拒绝" };
    case "spam":
      return { cls: "bg-[#8E8E93]/10 text-[#8E8E93] border-[#8E8E93]/20", label: "垃圾评论" };
    default:
      return { cls: "bg-secondary text-text-tertiary border-border", label: status };
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
      <template #icon>
        <Icon icon="ri:chat-3-line" class="text-primary" />
      </template>
      <template #subtitle>审核读者留言与互动评论，支持键盘极速审核与批量处置</template>
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="keyword" placeholder="搜索昵称、邮箱或正文..." />
          <button v-if="keyword" type="button" class="text-text-quaternary hover:text-foreground" @click="keyword = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton :route="{ path: '/console/comment-users' }">
          <template #icon><Icon icon="ri:user-shared-line" /></template>
          评论用户
        </VButton>

        <button
          type="button"
          class="rounded-xl border border-border px-3 py-1.5 text-xs text-text-secondary hover:bg-secondary hover:text-foreground flex items-center gap-1.5"
          @click="keyboardHelpVisible = !keyboardHelpVisible"
        >
          <Icon icon="ri:keyboard-line" />
          <span>快捷键</span>
        </button>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 快捷键指南横幅 -->
      <div
        v-if="keyboardHelpVisible"
        class="rounded-2xl border border-primary/20 bg-primary/5 p-3.5 text-xs text-foreground flex flex-wrap items-center justify-between gap-3 animate-fade-in"
      >
        <div class="flex flex-wrap items-center gap-4">
          <span class="font-bold text-primary flex items-center gap-1">
            <Icon icon="ri:flashlight-line" /> 极速审核快捷键：
          </span>
          <span><kbd class="apple-kbd">J</kbd> / <kbd class="apple-kbd">K</kbd> 上下翻选</span>
          <span><kbd class="apple-kbd">A</kbd> 批准通过</span>
          <span><kbd class="apple-kbd">D</kbd> 拒绝</span>
          <span><kbd class="apple-kbd">S</kbd> 标为垃圾</span>
          <span><kbd class="apple-kbd">R</kbd> 快捷回复</span>
          <span><kbd class="apple-kbd">X</kbd> 勾选/取消</span>
        </div>
        <button type="button" class="text-text-quaternary hover:text-foreground" @click="keyboardHelpVisible = false">
          <Icon icon="ri:close-line" />
        </button>
      </div>

      <!-- 概览数据指标卡（点击直接作为过滤器） -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <div
          class="stats-card cursor-pointer transition-all hover:border-primary"
          :class="{ 'border-primary ring-1 ring-primary': selectedStatus === 'all' }"
          @click="
            selectedStatus = 'all';
            page = 1;
          "
        >
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:chat-3-line" />
          </div>
          <div>
            <div class="stats-label">全部评论</div>
            <div class="stats-value">{{ statusCounts.all }} <span class="stats-sub">条留言</span></div>
          </div>
        </div>

        <div
          class="stats-card cursor-pointer transition-all hover:border-[#FF9500]"
          :class="{ 'border-[#FF9500] ring-1 ring-[#FF9500]': selectedStatus === 'pending' }"
          @click="
            selectedStatus = 'pending';
            page = 1;
          "
        >
          <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
            <Icon icon="ri:time-line" />
          </div>
          <div>
            <div class="stats-label">待审核</div>
            <div class="stats-value text-[#FF9500]">
              {{ statusCounts.pending }} <span class="stats-sub">条需要处理</span>
            </div>
          </div>
        </div>

        <div
          class="stats-card cursor-pointer transition-all hover:border-[#30D158]"
          :class="{ 'border-[#30D158] ring-1 ring-[#30D158]': selectedStatus === 'approved' }"
          @click="
            selectedStatus = 'approved';
            page = 1;
          "
        >
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:checkbox-circle-line" />
          </div>
          <div>
            <div class="stats-label">已批准展示</div>
            <div class="stats-value text-[#30D158]">
              {{ statusCounts.approved }} <span class="stats-sub">条正常显示</span>
            </div>
          </div>
        </div>

        <div
          class="stats-card cursor-pointer transition-all hover:border-[#FF453A]"
          :class="{ 'border-[#FF453A] ring-1 ring-[#FF453A]': selectedStatus === 'spam' }"
          @click="
            selectedStatus = 'spam';
            page = 1;
          "
        >
          <div class="stats-icon text-[#FF453A] bg-[#FF453A]/10">
            <Icon icon="ri:spam-2-line" />
          </div>
          <div>
            <div class="stats-label">垃圾拦截</div>
            <div class="stats-value text-[#FF453A]">
              {{ statusCounts.spam }} <span class="stats-sub">条已拦截</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 状态分段栏与批量控制 -->
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-card p-3">
        <div class="apple-segmented">
          <button
            v-for="st in statusTabs"
            :key="st.key"
            type="button"
            :class="{ active: selectedStatus === st.key }"
            @click="
              selectedStatus = st.key;
              page = 1;
            "
          >
            <Icon :icon="st.icon" class="mr-1 inline-block" />
            {{ st.label }}
            <span v-if="statusCounts[st.key] > 0" class="ml-1 opacity-70 font-mono">
              ({{ statusCounts[st.key] }})
            </span>
          </button>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded-lg border border-border px-3 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground"
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

      <!-- 评论卡片列表 -->
      <div v-else class="space-y-3">
        <div
          v-for="(c, idx) in comments"
          :key="c.id"
          class="group relative rounded-2xl border border-border bg-card p-4 transition-all hover:border-border-strong sm:p-5"
          :class="[
            selectedIds.has(c.id) ? 'border-primary ring-1 ring-primary bg-primary/5' : '',
            focusedIndex === idx ? 'ring-2 ring-primary/60 border-primary' : '',
          ]"
        >
          <div class="flex items-start justify-between gap-3">
            <!-- 评论者信息 -->
            <div class="flex items-start gap-3 min-w-0">
              <input
                type="checkbox"
                :checked="selectedIds.has(c.id)"
                class="mt-1 h-4 w-4 rounded border-border text-primary cursor-pointer shrink-0"
                @change="toggleSelect(c.id)"
              />

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
                    class="text-sm font-semibold text-foreground truncate cursor-pointer hover:underline"
                    @click="openUserDrawer(c)"
                  >
                    {{ c.nick || "匿名访客" }}
                  </span>
                  <span
                    class="rounded-full border px-2 py-0.2 text-[10px] font-medium"
                    :class="statusInfo(c.status).cls"
                  >
                    {{ statusInfo(c.status).label }}
                  </span>
                  <span v-if="c.email" class="text-xs font-mono text-text-quaternary truncate">
                    &lt;{{ c.email }}&gt;
                  </span>
                </div>

                <!-- 关联文章 / 单页 / 来源 -->
                <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-text-quaternary">
                  <span v-if="c.post_title || c.post_slug" class="flex items-center gap-1 text-primary hover:underline">
                    <Icon icon="ri:article-line" />
                    <span>{{ c.post_title || c.post_slug }}</span>
                  </span>
                  <span>·</span>
                  <span>{{ formatRelative(c.created_at) }}</span>
                  <span v-if="c.ip" class="font-mono text-[11px]">IP: {{ c.ip }}</span>
                </div>
              </div>
            </div>

            <!-- 操作按钮组 -->
            <div class="flex items-center gap-1 shrink-0">
              <button
                v-if="c.status !== 'approved'"
                type="button"
                class="action-btn text-[#30D158] hover:bg-[#30D158]/10"
                title="批准通过 (A)"
                @click="setCommentStatus(c.id, 'approved')"
              >
                <Icon icon="ri:check-line" />
                <span class="hidden sm:inline">批准</span>
              </button>

              <button
                v-if="c.status !== 'rejected'"
                type="button"
                class="action-btn text-[#FF9500] hover:bg-[#FF9500]/10"
                title="拒绝显示 (D)"
                @click="setCommentStatus(c.id, 'rejected')"
              >
                <Icon icon="ri:close-line" />
                <span class="hidden sm:inline">拒绝</span>
              </button>

              <button
                v-if="c.status !== 'spam'"
                type="button"
                class="action-btn text-[#8E8E93] hover:bg-[#8E8E93]/10"
                title="标为垃圾 (S)"
                @click="setCommentStatus(c.id, 'spam')"
              >
                <Icon icon="ri:spam-2-line" />
              </button>

              <button
                type="button"
                class="action-btn text-primary hover:bg-primary/10"
                title="快捷回复 (R)"
                @click="openReply(c)"
              >
                <Icon icon="ri:reply-line" />
              </button>

              <button
                type="button"
                class="action-btn text-state-error hover:bg-state-error/10"
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
            class="comment-rich-content mt-3 text-sm leading-relaxed text-foreground pl-12"
            v-html="c.content_html"
          ></div>
          <p v-else class="mt-3 text-sm leading-relaxed text-foreground pl-12 whitespace-pre-wrap">
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
            class="avatar-gradient-1 flex h-12 w-12 items-center justify-center rounded-full text-base font-bold text-white"
          >
            {{ userDrawerTarget.nick ? userDrawerTarget.nick.charAt(0).toUpperCase() : "?" }}
          </div>
          <div>
            <div class="text-base font-bold text-foreground">{{ userDrawerTarget.nick }}</div>
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
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
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
  text-transform: uppercase;
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
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}

/* 快捷键 kbd */
.apple-kbd {
  display: inline-block;
  padding: 1px 5px;
  font-size: 10px;
  font-family: monospace;
  font-weight: 700;
  color: var(--color-foreground);
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: 4px;
  box-shadow: inset 0 -1px 0 rgba(0, 0, 0, 0.2);
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
