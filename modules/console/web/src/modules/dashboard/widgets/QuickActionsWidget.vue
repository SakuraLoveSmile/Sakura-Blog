<script setup lang="ts">
import { Icon } from "@iconify/vue";
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
  <div class="apple-card h-full flex flex-col justify-between" :class="`quick-card-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3">
      <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">快捷操作</h3>
      <span class="text-xs text-muted-foreground">{{ displayedActions.length }} 个快捷入口</span>
    </div>

    <!-- Small 尺寸：极紧凑 4 小宫格 -->
    <div v-if="sizePreset === 'small'" class="grid grid-cols-2 gap-2 flex-1">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item flex items-center gap-2 p-2 rounded-xl text-left"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        <div
          class="w-7 h-7 rounded-lg flex items-center justify-center flex-shrink-0"
        >
          <Icon :icon="act.icon" class="w-3.5 h-3.5" />
        </div>
        <span class="text-xs font-medium text-foreground truncate">{{ act.label }}</span>
      </button>
    </div>

    <!-- Medium 尺寸：经典 2x2 网格 -->
    <div v-else-if="sizePreset === 'medium'" class="quick-grid-compact flex-1">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item flex flex-col items-center gap-1.5 p-3 rounded-lg"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        <div
          class="quick-icon-compact"
        >
          <Icon :icon="act.icon" class="w-5 h-5" />
        </div>
        <span class="quick-label-compact">{{ act.label }}</span>
      </button>
    </div>

    <!-- Large 尺寸：8 宫格全功能扩展 -->
    <div v-else class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 flex-1">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item flex flex-col items-center justify-center p-3 rounded-lg text-center"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        <div
          class="w-10 h-10 rounded-xl flex items-center justify-center mb-2 "
        >
          <Icon :icon="act.icon" class="w-5 h-5" />
        </div>
        <span class="text-xs font-semibold text-foreground">{{ act.label }}</span>
        <span class="text-xs text-muted-foreground mt-0.5">{{ act.desc }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 三种尺寸只改变排版，入口共用中性表面与图标。 */
.quick-action-item {
  min-height: 44px;
  background: transparent;
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  font: inherit;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.quick-action-item:hover {
  background: var(--color-accent);
  border-color: var(--color-border-strong);
}

.quick-action-item.quick-action-featured {
  color: var(--color-foreground);
}

.quick-grid-compact {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.quick-icon-compact {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.quick-label-compact {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-foreground);
  letter-spacing: -0.1px;
}
</style>
