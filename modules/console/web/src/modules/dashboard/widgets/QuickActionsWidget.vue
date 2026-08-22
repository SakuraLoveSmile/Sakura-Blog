<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { themeColors } from "@/lib/theme-colors";

const { breakpoint } = useBreakpoint();
const router = useRouter();
const colors = themeColors({
  primary: "#2E8DFF",
  success: "#30D158",
  warning: "#FF9F0A",
  chart5: "#BF5AF2",
});

interface QuickAction {
  label: string;
  icon: string;
  color: string;
  bgColor: string;
  path?: string;
  action?: () => void;
}

const actions: QuickAction[] = [
  {
    label: "写文章",
    icon: "ri:quill-pen-line",
    color: colors.primary,
    bgColor: "color-mix(in srgb, var(--color-primary) 12%, transparent)",
    path: "/posts/editor",
  },
  {
    label: "评论",
    icon: "ri:chat-3-line",
    color: colors.success,
    bgColor: "color-mix(in srgb, var(--color-state-success) 12%, transparent)",
    path: "/comments",
  },
  {
    label: "分类",
    icon: "ri:folder-3-line",
    color: colors.warning,
    bgColor: "color-mix(in srgb, var(--color-state-warning) 12%, transparent)",
    path: "/categories",
  },
  {
    label: "设置",
    icon: "ri:settings-3-line",
    color: colors.chart5,
    bgColor: "color-mix(in srgb, var(--color-chart-5) 12%, transparent)",
    path: "/settings",
  },
];

function go(path?: string) {
  if (path) router.push(path);
}

const hoveredIdx = ref<number | null>(null);
</script>

<template>
  <div class="apple-card" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-4">
      <h3 class="apple-section-title">快捷操作</h3>
    </div>

    <div class="quick-grid-compact">
      <button
        v-for="(act, i) in actions"
        :key="act.label"
        class="quick-item-compact"
        @click="go(act.path)"
        @mouseenter="hoveredIdx = i"
        @mouseleave="hoveredIdx = null"
      >
        <div
          class="quick-icon-compact"
          :style="{
            backgroundColor: hoveredIdx === i ? act.bgColor.replace('0.12', '0.22') : act.bgColor,
            color: act.color,
          }"
        >
          <Icon :icon="act.icon" class="w-5 h-5" />
        </div>
        <span class="quick-label-compact">{{ act.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.quick-grid-compact {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.quick-item-compact {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 12px 8px;
  border-radius: 14px;
  background: color-mix(in srgb, #fff 3%, transparent);
  border: 1px solid color-mix(in srgb, #fff 4%, transparent);
  color: inherit;
  font: inherit;
  cursor: pointer;
  transition: all 0.2s var(--ease-standard);
}

.quick-item-compact:hover {
  background: color-mix(in srgb, #fff 6%, transparent);
  border-color: color-mix(in srgb, #fff 8%, transparent);
  transform: translateY(-2px);
}

.quick-icon-compact {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s var(--ease-standard);
}

.quick-label-compact {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-foreground);
  letter-spacing: -0.1px;
}

/* M */
.apple-card:is([data-bp="md"], [data-bp="sm"]) .quick-icon-compact {
  width: 34px;
  height: 34px;
  border-radius: 10px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .quick-item-compact { padding: 10px 6px; gap: 5px; border-radius: 12px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .quick-label-compact { font-size: 11px; }

/* S */
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .quick-grid-compact {
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .quick-icon-compact {
  width: 40px;
  height: 40px;
  border-radius: 12px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .quick-item-compact { padding: 12px 4px; gap: 6px; border-radius: 14px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .quick-label-compact { font-size: 11px; }
</style>
