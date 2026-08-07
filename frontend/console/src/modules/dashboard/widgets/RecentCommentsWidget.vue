<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";

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

function getInitials(name: string): string {
  return name ? name.slice(0, 1).toUpperCase() : "?";
}

function avatarColor(name: string): string {
  const colors = ["#0A84FF", "#BF5AF2", "#30D158", "#FF9F0A", "#FF453A", "#5AC8FA"];
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  return colors[Math.abs(hash) % colors.length];
}

/** 跳转到前台文章中该评论所在位置（新标签打开，带锚点） */
function openComment(item: CommentItem) {
  const base = item.page_key || "/";
  const url = `${base}#comment-${item.id}`;
  window.open(url, "_blank", "noopener,noreferrer");
}

onMounted(() => {
  fetch("/api/v1/dashboard/recent-comments", { credentials: "include" })
    .then((r) => {
      if (!r.ok) throw new Error("加载评论失败");
      return r.json();
    })
    .then((resp: CommentsResp) => (items.value = resp.items))
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});
</script>

<template>
  <div class="apple-card h-full" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-4">
      <h3 class="apple-section-title">最近评论</h3>
      <button @click="router.push('/comments')" class="apple-link">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="recent-placeholder flex items-center justify-center text-[#8E8E93]">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-red-400">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-[#8E8E93] text-sm">
      暂无评论
    </div>
    <div v-else class="apple-comment-list">
      <button
        v-for="item in items"
        :key="item.id"
        class="apple-comment-item"
        type="button"
        @click="openComment(item)"
      >
        <div v-if="item.avatar" class="apple-avatar">
          <img :src="item.avatar" :alt="item.nick" class="w-full h-full rounded-full object-cover" loading="lazy" />
        </div>
        <div v-else class="apple-avatar-fallback" :style="{ backgroundColor: avatarColor(item.nick) }">
          {{ getInitials(item.nick) }}
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-[#EBEBF5] text-sm font-medium">{{ item.nick }}</span>
            <span
              v-if="item.status === 'pending'"
              class="apple-pending-badge"
            >
              待审
            </span>
            <span class="text-[#636366] text-xs ml-auto">{{ item.date }}</span>
          </div>
          <p class="text-[#8E8E93] text-sm mt-1 line-clamp-2">{{ item.content }}</p>
          <div v-if="item.page_key" class="flex items-center gap-1 mt-1.5 text-xs text-[#636366]">
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
  min-height: 200px;
}
.apple-card[data-bp="m"] .recent-placeholder,
.apple-card[data-bp="s"] .recent-placeholder { min-height: 180px; }
.apple-card[data-bp="s"] .recent-placeholder { min-height: 140px; }
</style>
