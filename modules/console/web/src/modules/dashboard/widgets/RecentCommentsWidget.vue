<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { commentApi, dashboardApi } from "@/api";
import { Toast } from "@/components/ui";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
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

async function moderate(item: CommentItem, action: "approved" | "spam" | "delete") {
  if (actionBusy.value !== null) return;
  actionBusy.value = item.id;
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
  if (sizePreset.value === "small") return items.value.slice(0, 2);
  if (sizePreset.value === "medium") return items.value.slice(0, 4);
  return items.value.slice(0, 8);
});
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`recent-comments-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3">
      <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近评论</h3>
      <button @click="router.push('/comments')" class="apple-link text-xs">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载评论">
      <span v-for="index in (sizePreset === 'large' ? 4 : 2)" :key="index" class="dashboard-skeleton-row">
        <i class="dashboard-skeleton-avatar" />
        <i class="dashboard-skeleton-copy is-wide" />
      </span>
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1">
      暂无评论
    </div>

    <!-- Small 尺寸：超紧凑单行/双行评论 -->
    <div v-else-if="sizePreset === 'small'" class="space-y-2 flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.id"
        class="w-full flex items-start gap-2 p-1.5 rounded-xl border border-transparent hover:border-white/5 hover:bg-white/5 text-left transition-all"
        type="button"
        @click="openComment(item)"
      >
        <div v-if="item.avatar && !avatarFailed.has(item.id)" class="w-6 h-6 rounded-full overflow-hidden flex-shrink-0">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full object-cover" loading="lazy" @error="handleAvatarError(item)" />
        </div>
        <div v-else :class="['w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold text-foreground flex-shrink-0', avatarClass(item.nick)]">
          {{ getInitials(item.nick) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between text-xs">
            <span class="font-medium text-foreground truncate">{{ item.nick }}</span>
            <span class="text-[10px] text-muted-foreground ml-1">{{ item.date }}</span>
          </div>
          <p class="text-[11px] text-muted-foreground truncate mt-0.5">{{ item.content }}</p>
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
        <div class="comment-inline-actions" aria-label="评论操作">
          <button class="action-approve" type="button" title="通过" :disabled="actionBusy === item.id" @click.stop="moderate(item, 'approved')">
            <Icon :icon="actionBusy === item.id ? 'ri:loader-4-line' : 'ri:check-line'" :class="{ 'animate-spin': actionBusy === item.id }" />
          </button>
          <button class="action-reply" type="button" title="回复" :disabled="actionBusy === item.id" @click.stop="startReply(item)">
            <Icon :icon="actionBusy === item.id ? 'ri:loader-4-line' : 'ri:reply-line'" :class="{ 'animate-spin': actionBusy === item.id }" />
          </button>
          <button class="action-spam" type="button" title="标记垃圾" :disabled="actionBusy === item.id" @click.stop="moderate(item, 'spam')">
            <Icon :icon="actionBusy === item.id ? 'ri:loader-4-line' : 'ri:spam-2-line'" :class="{ 'animate-spin': actionBusy === item.id }" />
          </button>
          <button class="action-delete" type="button" title="删除" :disabled="actionBusy === item.id" @click.stop="confirmDelete(item)">
            <Icon :icon="actionBusy === item.id ? 'ri:loader-4-line' : 'ri:delete-bin-6-line'" :class="{ 'animate-spin': actionBusy === item.id }" />
          </button>
        </div>
        <div v-if="replyingId === item.id" class="comment-reply-popover" @click.stop>
          <div class="flex items-center justify-between gap-2 mb-1.5">
            <span class="text-xs font-medium text-foreground">回复 {{ item.nick }}</span>
            <span class="text-[10px] text-text-quaternary tabular-nums">{{ replyText.length }}/500</span>
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
            <button type="button" class="comment-popover-btn is-muted" @click="cancelReply">取消</button>
            <button type="button" class="comment-popover-btn is-primary" :disabled="!replyText.trim() || replyBusy === item.id" @click="sendReply(item)">
              <Icon :icon="replyBusy === item.id ? 'ri:loader-4-line' : 'ri:send-plane-line'" :class="{ 'animate-spin': replyBusy === item.id }" />
              发送
            </button>
          </div>
        </div>
        <div v-if="deleteConfirmId === item.id" class="comment-delete-popover" role="alertdialog" @click.stop>
          <span>删除这条评论？</span>
          <div class="flex gap-1.5">
            <button type="button" class="comment-popover-btn is-muted" @click="deleteConfirmId = null">取消</button>
            <button type="button" class="comment-popover-btn is-danger" :disabled="actionBusy === item.id" @click="moderate(item, 'delete')">确认删除</button>
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
.recent-placeholder {
  min-height: 120px;
}

.comment-content-bubble {
  padding: 7px 9px;
  border: 1px solid color-mix(in srgb, #fff 6%, transparent);
  border-radius: 0 10px 10px 10px;
  background: color-mix(in srgb, #fff 3%, transparent);
}

.comment-inline-actions {
  display: flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 150ms var(--ease-standard);
}

.apple-comment-item:hover .comment-inline-actions,
.apple-comment-item:focus-within .comment-inline-actions {
  opacity: 1;
}

.comment-inline-actions button {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  color: var(--color-muted-foreground);
  background: transparent;
  cursor: pointer;
}

.comment-inline-actions button:hover:not(:disabled) {
  color: var(--color-primary);
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}

.comment-inline-actions button.action-approve:hover:not(:disabled) {
  color: var(--color-state-success);
  border-color: var(--color-state-success);
  background: color-mix(in srgb, var(--color-state-success) 10%, transparent);
}

.comment-inline-actions button.action-reply:hover:not(:disabled) {
  color: var(--color-primary);
}

.comment-inline-actions button.action-spam:hover:not(:disabled) {
  color: var(--color-state-warning);
  border-color: var(--color-state-warning);
  background: color-mix(in srgb, var(--color-state-warning) 10%, transparent);
}

.comment-inline-actions button.action-delete:hover:not(:disabled) {
  color: var(--color-state-error);
  border-color: var(--color-state-error);
  background: color-mix(in srgb, var(--color-state-error) 10%, transparent);
}

.comment-inline-actions button:disabled {
  cursor: progress;
  opacity: 0.5;
}

.comment-reply-popover,
.comment-delete-popover {
  margin: 8px 0 2px 44px;
  padding: 10px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 22%, var(--color-border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-popover) 92%, transparent);
  box-shadow: 0 12px 28px -18px rgba(0, 0, 0, 0.8);
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
  border-radius: 9px;
  outline: none;
  background: color-mix(in srgb, var(--color-background) 70%, transparent);
  color: var(--color-foreground);
  font: inherit;
  font-size: 12px;
}

.comment-reply-input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 15%, transparent);
}

.comment-popover-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 26px;
  padding: 0 8px;
  border: 1px solid var(--color-border);
  border-radius: 7px;
  background: transparent;
  color: var(--color-foreground);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.comment-popover-btn:disabled {
  cursor: progress;
  opacity: 0.5;
}

.comment-popover-btn.is-muted { color: var(--color-muted-foreground); }
.comment-popover-btn.is-primary { border-color: color-mix(in srgb, var(--color-primary) 48%, transparent); color: var(--color-primary); }
.comment-popover-btn.is-danger { border-color: color-mix(in srgb, var(--color-state-error) 48%, transparent); color: var(--color-state-error); }

.comment-inline-actions svg {
  width: 14px;
  height: 14px;
}
</style>
