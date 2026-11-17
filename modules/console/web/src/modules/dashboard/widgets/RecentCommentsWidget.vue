<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { commentApi, dashboardApi } from "@/api";
import { Toast, VButton } from "@/components/ui";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

interface Props {
  sizePreset?: WidgetSizePreset;
  config?: Record<string, unknown>;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
  config: () => ({}),
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const router = useRouter();

interface CommentItem {
  id: number;
  nick: string;
  email: string;
  content: string;
  page_key: string;
  status: string;
  date: string;
  avatar: string;
}

interface CommentsResp {
  items: CommentItem[];
}

const items = ref<CommentItem[]>([]);
const loading = ref(true);
const error = ref("");
const actionBusy = ref<number | null>(null);
// 正在执行的审核动作；与 actionBusy（哪一行）配合，只让被点的那枚图标转起来，其余只是禁用变淡
const busyAction = ref<"approved" | "spam" | "delete" | null>(null);
const replyingId = ref<number | null>(null);
const replyText = ref("");
const replyBusy = ref<number | null>(null);
const deleteConfirmId = ref<number | null>(null);
const avatarFailed = ref(new Set<number>());

function getInitials(name: string): string {
  return name ? name.slice(0, 1).toUpperCase() : "?";
}

function avatarClass(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  return `avatar-gradient-${(Math.abs(hash) % 8) + 1}`;
}

function handleAvatarError(item: CommentItem) {
  avatarFailed.value = new Set(avatarFailed.value).add(item.id);
}

/** 跳转到前台文章中该评论所在位置（新标签打开，带锚点） */
function openComment(item: CommentItem) {
  const base = item.page_key || "/";
  const url = `${base}#comment-${item.id}`;
  window.open(url, "_blank", "noopener,noreferrer");
}

function isBusy(item: CommentItem, action: "approved" | "spam" | "delete"): boolean {
  return actionBusy.value === item.id && busyAction.value === action;
}

async function moderate(item: CommentItem, action: "approved" | "spam" | "delete") {
  if (actionBusy.value !== null) return;
  actionBusy.value = item.id;
  busyAction.value = action;
  try {
    if (action === "delete") {
      await commentApi.delete(item.id);
      items.value = items.value.filter((current) => current.id !== item.id);
      deleteConfirmId.value = null;
      Toast.success("评论已删除");
    } else {
      await commentApi.setStatus(item.id, action);
      item.status = action;
      Toast.success(action === "approved" ? "评论已通过" : "已标记为垃圾评论");
    }
  } catch (err: any) {
    Toast.error(err?.response?.data?.error || "评论操作失败");
  } finally {
    actionBusy.value = null;
    busyAction.value = null;
  }
}

function startReply(item: CommentItem) {
  if (actionBusy.value !== null || replyBusy.value !== null) return;
  deleteConfirmId.value = null;
  replyingId.value = item.id;
  replyText.value = "";
}

function cancelReply() {
  replyingId.value = null;
  replyText.value = "";
}

async function sendReply(item: CommentItem) {
  const content = replyText.value.trim();
  if (!content || replyBusy.value !== null) return;
  replyBusy.value = item.id;
  try {
    await commentApi.reply({ parent_id: item.id, content });
    Toast.success("回复已发送");
    cancelReply();
  } catch (err: any) {
    Toast.error(err?.response?.data?.error || "回复失败");
  } finally {
    replyBusy.value = null;
  }
}

function confirmDelete(item: CommentItem) {
  if (actionBusy.value !== null) return;
  deleteConfirmId.value = deleteConfirmId.value === item.id ? null : item.id;
  replyingId.value = null;
}

function loadComments() {
  loading.value = true;
  error.value = "";
  dashboardApi.recentComments()
    .then((r: { data: CommentsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e instanceof Error ? e.message : "评论加载失败"))
    .finally(() => (loading.value = false));
}

onMounted(() => {
  loadComments();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadComments);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadComments));

const displayedItems = computed(() => {
  // 实例配置的显示条数优先，但不超过当前档位的盒子高度上限。
  const presetMax = sizePreset.value === "small" ? 2 : sizePreset.value === "medium" ? 4 : 8;
  const raw = Number(props.config.itemLimit);
  const limit = Number.isFinite(raw) && raw > 0 ? Math.min(Math.round(raw), presetMax) : presetMax;
  return items.value.slice(0, limit);
});
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`recent-comments-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3 recent-header">
      <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近评论</h3>
      <button type="button" class="apple-link card-link card-hit text-xs" @click="router.push('/comments')">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载评论">
      <span v-for="index in (sizePreset === 'small' ? 2 : sizePreset === 'medium' ? 4 : 5)" :key="index" class="dashboard-skeleton-row">
        <i class="dashboard-skeleton-avatar" />
        <i class="dashboard-skeleton-copy is-wide" />
      </span>
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1">
      暂无评论
    </div>

    <!-- Small 尺寸：超紧凑单行/双行评论 -->
    <div v-else-if="sizePreset === 'small'" class="recent-mini-list flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.id"
        class="recent-mini-row w-full flex items-center gap-2 py-1.5 border-t border-border text-left transition-colors duration-150"
        type="button"
        @click="openComment(item)"
      >
        <div v-if="item.avatar && !avatarFailed.has(item.id)" class="w-6 h-6 rounded-full overflow-hidden flex-shrink-0">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full object-cover" loading="lazy" @error="handleAvatarError(item)" />
        </div>
        <div v-else :class="['w-6 h-6 rounded-full flex items-center justify-center text-[11px] font-bold text-foreground flex-shrink-0', avatarClass(item.nick)]">
          {{ getInitials(item.nick) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between text-xs">
            <span class="font-medium text-foreground truncate">{{ item.nick }}</span>
            <span class="text-[12px] text-muted-foreground ml-1">{{ item.date }}</span>
          </div>
          <p class="text-[12px] text-muted-foreground truncate mt-0.5">{{ item.content }}</p>
        </div>
      </button>
    </div>

    <!-- Large 尺寸：评论流 + 内联审核动作 -->
    <div v-else-if="sizePreset === 'large'" class="apple-comment-list flex-1 overflow-y-auto pr-0.5">
      <div
        v-for="item in displayedItems"
        :key="item.id"
        class="apple-comment-item group"
      >
        <button class="flex min-w-0 flex-1 gap-3 text-left" type="button" @click="openComment(item)">
          <div v-if="item.avatar && !avatarFailed.has(item.id)" class="apple-avatar">
            <img :src="item.avatar" :alt="item.nick" class="w-full h-full rounded-full object-cover" loading="lazy" @error="handleAvatarError(item)" />
          </div>
          <div v-else :class="['apple-avatar-fallback', avatarClass(item.nick)]">
            {{ getInitials(item.nick) }}
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-foreground text-sm font-medium">{{ item.nick }}</span>
              <span v-if="item.status === 'pending'" class="apple-pending-badge">待审</span>
              <span class="text-text-quaternary text-xs ml-auto">{{ item.date }}</span>
            </div>
            <p class="comment-content-bubble text-muted-foreground text-sm mt-1 line-clamp-2">{{ item.content }}</p>
            <div v-if="item.page_key" class="flex items-center gap-1 mt-1.5 text-xs text-text-quaternary">
              <Icon icon="ri:article-line" class="w-3 h-3" />
              <span class="truncate">{{ item.page_key }}</span>
            </div>
          </div>
        </button>
        <!-- 处理中：这一行的四枚都禁用变淡，但只有被点的那枚换成转动的 loader（回复在弹窗里发送，不占用图标） -->
        <div class="comment-inline-actions" role="group" aria-label="评论操作">
          <button
            class="action-approve card-hit"
            type="button"
            title="通过"
            :aria-label="`通过 ${item.nick} 的评论`"
            :aria-busy="isBusy(item, 'approved')"
            :disabled="actionBusy === item.id"
            @click.stop="moderate(item, 'approved')"
          >
            <Icon :icon="isBusy(item, 'approved') ? 'ri:loader-4-line' : 'ri:check-line'" :class="{ 'animate-spin': isBusy(item, 'approved') }" />
          </button>
          <button
            class="action-reply card-hit"
            type="button"
            title="回复"
            :aria-label="`回复 ${item.nick}`"
            :disabled="actionBusy === item.id"
            @click.stop="startReply(item)"
          >
            <Icon icon="ri:reply-line" />
          </button>
          <button
            class="action-spam card-hit"
            type="button"
            title="标记垃圾"
            :aria-label="`标记 ${item.nick} 的评论为垃圾`"
            :aria-busy="isBusy(item, 'spam')"
            :disabled="actionBusy === item.id"
            @click.stop="moderate(item, 'spam')"
          >
            <Icon :icon="isBusy(item, 'spam') ? 'ri:loader-4-line' : 'ri:spam-2-line'" :class="{ 'animate-spin': isBusy(item, 'spam') }" />
          </button>
          <button
            class="action-delete card-hit"
            type="button"
            title="删除"
            :aria-label="`删除 ${item.nick} 的评论`"
            :aria-busy="isBusy(item, 'delete')"
            :disabled="actionBusy === item.id"
            @click.stop="confirmDelete(item)"
          >
            <Icon :icon="isBusy(item, 'delete') ? 'ri:loader-4-line' : 'ri:delete-bin-6-line'" :class="{ 'animate-spin': isBusy(item, 'delete') }" />
          </button>
        </div>
        <div v-if="replyingId === item.id" class="comment-reply-popover" @click.stop>
          <div class="flex items-center justify-between gap-2 mb-1.5">
            <span class="text-xs font-medium text-foreground">回复 {{ item.nick }}</span>
            <span class="text-[11px] text-text-quaternary tabular-nums">{{ replyText.length }}/500</span>
          </div>
          <textarea
            v-model="replyText"
            class="comment-reply-input"
            rows="2"
            maxlength="500"
            placeholder="写下你的回复..."
            aria-label="回复内容"
            @keydown.esc="cancelReply"
          />
          <div class="flex justify-end gap-2 mt-2">
            <VButton class="card-hit" type="ghost" size="xs" @click="cancelReply">取消</VButton>
            <VButton class="card-hit" type="primary" size="xs" :disabled="!replyText.trim()" :loading="replyBusy === item.id" @click="sendReply(item)">
              <template #icon><Icon icon="ri:send-plane-line" class="w-3 h-3" /></template>
              发送
            </VButton>
          </div>
        </div>
        <div v-if="deleteConfirmId === item.id" class="comment-delete-popover" role="alertdialog" @click.stop>
          <span>删除这条评论？</span>
          <div class="flex gap-1.5">
            <VButton class="card-hit" type="ghost" size="xs" @click="deleteConfirmId = null">取消</VButton>
            <VButton class="card-hit" type="danger" size="xs" :loading="isBusy(item, 'delete')" :disabled="actionBusy === item.id" @click="moderate(item, 'delete')">确认删除</VButton>
          </div>
        </div>
      </div>
    </div>

    <!-- Medium 尺寸：标准评论流 -->
    <div v-else class="apple-comment-list flex-1 overflow-y-auto pr-0.5">
      <button
        v-for="item in displayedItems"
        :key="item.id"
        class="apple-comment-item group"
        type="button"
        @click="openComment(item)"
      >
        <div v-if="item.avatar && !avatarFailed.has(item.id)" class="apple-avatar">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full rounded-full object-cover" loading="lazy" @error="handleAvatarError(item)" />
        </div>
        <div v-else :class="['apple-avatar-fallback', avatarClass(item.nick)]">
          {{ getInitials(item.nick) }}
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-foreground text-sm font-medium">{{ item.nick }}</span>
            <span
              v-if="item.status === 'pending'"
              class="apple-pending-badge"
            >
              待审
            </span>
            <span class="text-text-quaternary text-xs ml-auto">{{ item.date }}</span>
          </div>
          <p class="comment-content-bubble text-muted-foreground text-sm mt-1 line-clamp-2">{{ item.content }}</p>
          <div v-if="item.page_key" class="flex items-center gap-1 mt-1.5 text-xs text-text-quaternary">
            <Icon icon="ri:article-line" class="w-3 h-3" />
            <span class="truncate">{{ item.page_key }}</span>
          </div>
        </div>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 小档 107px 高：只留昵称一行与头像，正文行在更矮的盒子里会挤掉时间。 */
.recent-comments-small .recent-header {
  margin-bottom: 4px;
}
.recent-mini-list {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.recent-comments-small .recent-mini-row {
  padding-block: 4px;
}
.recent-comments-small .recent-mini-row p {
  display: none;
}

.recent-placeholder {
  min-height: 0;
}

.comment-content-bubble {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

/* 评论行操作：与头部、文章行同一套幽灵语言——中性浅底，只有图标变语义色；悬停只在有悬停能力的设备上生效。
   手机上间隔 16px（--card-rail-gap，见 dashboard.css），每枚 28px 的按钮各自补成 44px 的点击区且互不抢点击。 */
.comment-inline-actions {
  display: flex;
  align-items: center;
  gap: var(--card-rail-gap, 2px);
  flex-shrink: 0;
  margin-right: var(--card-comment-inset, 0px);
  transition: opacity var(--duration-fast) var(--ease-standard);
}

@media (hover: hover) and (min-width: 1024px) {
  .comment-inline-actions { opacity: 0; }
  .apple-comment-item:hover .comment-inline-actions,
  .apple-comment-item:focus-within .comment-inline-actions { opacity: 1; }
}

.comment-inline-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-muted-foreground);
  background: transparent;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

/* 按钮总是落在悬停中的行上（行悬停底就是 accent），悬停 / 按下各再亮一阶，才看得出落在了哪枚上 */
.comment-inline-actions button:active:not(:disabled) {
  color: var(--color-foreground);
  background: var(--color-border-strong);
}

.comment-inline-actions button:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

@media (hover: hover) {
  .comment-inline-actions button:hover:not(:disabled) {
    color: var(--color-foreground);
    background: var(--color-secondary-700);
  }
  .comment-inline-actions button.action-approve:hover:not(:disabled) { color: var(--color-state-success); }
  .comment-inline-actions button.action-reply:hover:not(:disabled) { color: var(--color-primary); }
  .comment-inline-actions button.action-spam:hover:not(:disabled) { color: var(--color-state-warning); }
  .comment-inline-actions button.action-delete:hover:not(:disabled) { color: var(--color-state-error); }
}

.comment-inline-actions button:disabled {
  cursor: progress;
  opacity: 0.5;
}

/* 被点的那枚在处理中：保持不透明，让转动的 loader 看得清；同一行其余三枚仍是禁用变淡 */
.comment-inline-actions button:disabled[aria-busy="true"] {
  opacity: 1;
}

/* 大档行带操作与弹窗：弹窗另起一行并对齐正文起点（头像 32px + 间距 12px）。
   否则它会作为第四个弹性子项挤在操作右侧，手机上正文会被压成一条缝、按钮文字逐字折行。 */
.apple-card.recent-comments-large .apple-comment-item {
  flex-wrap: wrap;
  row-gap: 0;
}

.comment-reply-popover,
.comment-delete-popover {
  flex: 1 1 100%;
  min-width: 0;
  max-width: 480px;
  margin: 8px 0 2px 44px;
  padding: 10px;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-2xl);
  background: var(--color-popover);
  box-shadow: var(--shadow-lg);
}

.comment-delete-popover {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.comment-reply-input {
  width: 100%;
  resize: vertical;
  min-height: 48px;
  padding: 8px 9px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  outline: none;
  background: color-mix(in srgb, var(--color-background) 70%, transparent);
  color: var(--color-foreground);
  font: inherit;
  font-size: 12px;
}

.comment-reply-input:focus {
  border-color: var(--color-primary);
}

.comment-inline-actions svg {
  width: 14px;
  height: 14px;
}
</style>
