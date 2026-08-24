<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { commentApi, dashboardApi } from "@/api";
import { Toast } from "@/components/ui";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";

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

function getInitials(name: string): string {
  return name ? name.slice(0, 1).toUpperCase() : "?";
}

function avatarClass(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  return `avatar-gradient-${(Math.abs(hash) % 8) + 1}`;
}

/** 跳转到前台文章中该评论所在位置（新标签打开，带锚点） */
function openComment(item: CommentItem) {
  const base = item.page_key || "/";
  const url = `${base}#comment-${item.id}`;
  window.open(url, "_blank", "noopener,noreferrer");
}

async function moderate(item: CommentItem, action: "approved" | "spam" | "delete") {
  if (actionBusy.value !== null) return;
  if (action === "delete" && !window.confirm("确定删除这条评论吗？删除后不可恢复。")) return;
  actionBusy.value = item.id;
  try {
    if (action === "delete") {
      await commentApi.delete(item.id);
      items.value = items.value.filter((current) => current.id !== item.id);
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

async function reply(item: CommentItem) {
  const content = window.prompt("回复评论", "");
  if (!content?.trim() || actionBusy.value !== null) return;
  actionBusy.value = item.id;
  try {
    await commentApi.reply({ parent_id: item.id, content: content.trim() });
    Toast.success("回复已发送");
  } catch (err: any) {
    Toast.error(err?.response?.data?.error || "回复失败");
  } finally {
    actionBusy.value = null;
  }
}

onMounted(() => {
  dashboardApi
    .recentComments()
    .then((r: { data: CommentsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});

const displayedItems = computed(() => {
  if (sizePreset.value === "small") return items.value.slice(0, 2);
  if (sizePreset.value === "medium") return items.value.slice(0, 4);
  return items.value.slice(0, 8);
});
</script>

<template>
  <div
    class="apple-card h-full flex flex-col justify-between"
    :class="`recent-comments-${sizePreset}`"
    :data-bp="breakpoint"
  >
    <div class="flex items-center justify-between mb-3">
      <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近评论</h3>
      <button @click="router.push('/comments')" class="apple-link text-xs">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="recent-placeholder flex items-center justify-center text-muted-foreground flex-1">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">
      {{ error }}
    </div>
    <div
      v-else-if="items.length === 0"
      class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1"
    >
      暂无评论
    </div>

    <!-- Small 尺寸：超紧凑单行/双行评论 -->
    <div v-else-if="sizePreset === 'small'" class="space-y-2 flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.id"
        class="w-full flex items-start gap-2 p-1.5 rounded-lg hover:bg-white/5 text-left transition-colors"
        type="button"
        @click="openComment(item)"
      >
        <div v-if="item.avatar" class="w-6 h-6 rounded-full overflow-hidden flex-shrink-0">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full object-cover" loading="lazy" />
        </div>
        <div
          v-else
          :class="[
            'w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold text-foreground flex-shrink-0',
            avatarClass(item.nick),
          ]"
        >
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
      <div v-for="item in displayedItems" :key="item.id" class="apple-comment-item group">
        <button class="flex min-w-0 flex-1 gap-3 text-left" type="button" @click="openComment(item)">
          <div v-if="item.avatar" class="apple-avatar">
            <img :src="item.avatar" :alt="item.nick" class="w-full h-full rounded-full object-cover" loading="lazy" />
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
            <p class="text-muted-foreground text-sm mt-1 line-clamp-2">{{ item.content }}</p>
            <div v-if="item.page_key" class="flex items-center gap-1 mt-1.5 text-xs text-text-quaternary">
              <Icon icon="ri:article-line" class="w-3 h-3" />
              <span class="truncate">{{ item.page_key }}</span>
            </div>
          </div>
        </button>
        <div class="comment-inline-actions" aria-label="评论操作">
          <button
            type="button"
            title="通过"
            :disabled="actionBusy === item.id"
            @click.stop="moderate(item, 'approved')"
          >
            <Icon icon="ri:check-line" />
          </button>
          <button type="button" title="回复" :disabled="actionBusy === item.id" @click.stop="reply(item)">
            <Icon icon="ri:reply-line" />
          </button>
          <button
            type="button"
            title="标记垃圾"
            :disabled="actionBusy === item.id"
            @click.stop="moderate(item, 'spam')"
          >
            <Icon icon="ri:spam-2-line" />
          </button>
          <button type="button" title="删除" :disabled="actionBusy === item.id" @click.stop="moderate(item, 'delete')">
            <Icon icon="ri:delete-bin-6-line" />
          </button>
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
        <div v-if="item.avatar" class="apple-avatar">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full rounded-full object-cover" loading="lazy" />
        </div>
        <div v-else :class="['apple-avatar-fallback', avatarClass(item.nick)]">
          {{ getInitials(item.nick) }}
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-foreground text-sm font-medium">{{ item.nick }}</span>
            <span v-if="item.status === 'pending'" class="apple-pending-badge"> 待审 </span>
            <span class="text-text-quaternary text-xs ml-auto">{{ item.date }}</span>
          </div>
          <p class="text-muted-foreground text-sm mt-1 line-clamp-2">{{ item.content }}</p>
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

.comment-inline-actions button:disabled {
  cursor: progress;
  opacity: 0.5;
}

.comment-inline-actions svg {
  width: 14px;
  height: 14px;
}
</style>
