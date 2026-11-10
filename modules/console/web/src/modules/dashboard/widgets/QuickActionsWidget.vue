<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";
import { dashboardApi } from "@/api";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const router = useRouter();
interface QuickAction {
  key: string;
  label: string;
  desc?: string;
  icon: string;
  path?: string;
  isExternal?: boolean;
}

const baseActions: QuickAction[] = [
  {
    key: "write_post",
    label: "写文章",
    desc: "发布新内容",
    icon: "ri:quill-pen-line",
    path: "/posts/editor",
  },
  {
    key: "review_comments",
    label: "评论",
    desc: "互动审核",
    icon: "ri:chat-3-line",
    path: "/comments",
  },
  {
    key: "manage_categories",
    label: "分类",
    desc: "组织结构",
    icon: "ri:folder-3-line",
    path: "/categories",
  },
  {
    key: "settings",
    label: "设置",
    desc: "系统首选项",
    icon: "ri:settings-3-line",
    path: "/settings",
  },
];

const extendedActions: QuickAction[] = [
  ...baseActions,
  {
    key: "manage_posts",
    label: "文章列表",
    desc: "管理已有文章",
    icon: "ri:file-list-3-line",
    path: "/posts",
  },
  {
    key: "upload_attachment",
    label: "媒体库",
    desc: "文件与图片",
    icon: "ri:attachment-2",
    path: "/attachments",
  },
  {
    key: "system_update",
    label: "系统更新",
    desc: "版本与升级",
    icon: "ri:rocket-2-line",
    path: "/system-update",
  },
  {
    key: "open_frontend",
    label: "查看博客",
    desc: "打开前台页面",
    icon: "ri:external-link-line",
    path: "/",
    isExternal: true,
  },
];

const actionCatalog = [...extendedActions];
const remoteActions = ref<QuickAction[] | null>(null);

async function loadActions() {
  try {
    const response = await dashboardApi.quickActions();
    const keys = Array.isArray(response.data?.items) ? response.data.items : [];
    const resolved: QuickAction[] = keys.flatMap((key: unknown) => {
      const action = actionCatalog.find((candidate: QuickAction) => candidate.key === key);
      return action ? [action] : [];
    });
    remoteActions.value = resolved.length ? resolved : null;
  } catch {
    remoteActions.value = null;
  }
}

const displayedActions = computed(() => {
  const source = remoteActions.value ?? (sizePreset.value === "large" ? extendedActions : baseActions);
  if (sizePreset.value === "large") return source;
  return source.slice(0, 4);
});

function go(act: QuickAction) {
  if (!act.path) return;
  if (act.isExternal) {
    window.open(act.path, "_blank", "noopener,noreferrer");
  } else {
    router.push(act.path);
  }
}

onMounted(() => {
  loadActions();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadActions);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadActions));
</script>

<template>
  <div class="apple-card quick-actions" :class="`quick-card-${sizePreset}`" :data-bp="breakpoint">
    <div class="quick-actions-heading">
      <h3 class="apple-section-title">快捷操作</h3>
      <span class="text-[12px] text-muted-foreground">{{ displayedActions.length }} 个入口</span>
    </div>

    <div class="quick-action-list">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        {{ act.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 卡片改成纵向 flex，让入口网格吃掉剩余高度：同一档位下不留半张空白。 */
.quick-actions {
  display: flex;
  flex-direction: column;
}

.quick-actions-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.quick-action-list {
  display: grid;
  flex: 1 1 auto;
  min-height: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: minmax(0, auto);
  column-gap: 24px;
  align-content: start;
}

.quick-action-item {
  display: flex;
  align-items: center;
  min-height: 40px;
  padding: 8px 0;
  border: 0;
  border-top: 1px solid var(--color-border);
  border-radius: 0;
  background: transparent;
  color: var(--color-text-secondary);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard);
}

.quick-action-item:hover,
.quick-action-item:focus-visible {
  color: var(--color-foreground);
  text-decoration: underline;
  text-underline-offset: 3px;
}

/* 小档盒子 116px：四个入口排两行，行高收紧；大档整行后加高行距。 */
.quick-card-small .quick-actions-heading {
  margin-bottom: 4px;
}
.quick-card-small .quick-action-item {
  min-height: 26px;
  padding: 4px 0;
  font-size: 12px;
}
/* 中/大档把行高按剩余空间均分，避免固定行高留下大片空白。 */
.quick-card-medium .quick-action-list,
.quick-card-large .quick-action-list {
  grid-auto-rows: minmax(0, 1fr);
  align-content: stretch;
}
.quick-card-medium .quick-action-item,
.quick-card-large .quick-action-item {
  min-height: 0;
  padding-block: 8px;
}
.quick-card-large .quick-action-list {
  column-gap: 40px;
}

@media (max-width: 767px) {
  .quick-action-item {
    min-height: 44px;
  }
}
</style>
