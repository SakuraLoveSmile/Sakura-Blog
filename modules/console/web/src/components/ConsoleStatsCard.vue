<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";

const props = withDefaults(
  defineProps<{
    /** 统计标签文字 */
    label?: string;
    /** 统计数值 */
    value?: string | number;
    /** 次级辅助说明（如 "篇博文"、"个页面"） */
    sub?: string | number;
    /** Remix Icon 名称（如 "ri:article-line"） */
    icon?: string;
    /**
     * 主题身份语义色。支持预设标识（primary, success, warning, danger, purple, cyan）或任意合法 CSS 颜色。
     * 缺省为 "primary"。
     */
    color?: string;
    /** 数值文字自定义 class（如 text-state-success 等） */
    valueClass?: string;
    /** 是否支持点击交互 */
    clickable?: boolean;
  }>(),
  {
    label: "",
    value: 0,
    sub: "",
    icon: "",
    color: "primary",
    valueClass: "",
    clickable: false,
  },
);

defineEmits<{
  (event: "click", e: MouseEvent): void;
}>();

const colorPresetMap: Record<string, { text: string; bg: string }> = {
  primary: {
    text: "var(--color-primary, #2e8dff)",
    bg: "color-mix(in srgb, var(--color-primary, #2e8dff) 12%, transparent)",
  },
  success: {
    text: "var(--color-state-success, #30d158)",
    bg: "color-mix(in srgb, var(--color-state-success, #30d158) 12%, transparent)",
  },
  warning: {
    text: "var(--color-state-warning, #ff9f0a)",
    bg: "color-mix(in srgb, var(--color-state-warning) 12%, transparent)",
  },
  danger: {
    text: "var(--color-state-error, #ff453a)",
    bg: "color-mix(in srgb, var(--color-state-error, #ff453a) 12%, transparent)",
  },
  purple: {
    text: "#af52de",
    bg: "color-mix(in srgb, #af52de 12%, transparent)",
  },
  cyan: {
    text: "#30b0c7",
    bg: "color-mix(in srgb, #30b0c7 12%, transparent)",
  },
};

const iconStyle = computed(() => {
  const preset = colorPresetMap[props.color];
  if (preset) {
    return {
      color: preset.text,
      backgroundColor: preset.bg,
    };
  }
  return {
    color: props.color,
    backgroundColor: `color-mix(in srgb, ${props.color} 12%, transparent)`,
  };
});
</script>

<template>
  <div
    class="stats-card"
    :class="{ 'cursor-pointer hover:border-border-strong': clickable }"
    @click="$emit('click', $event)"
  >
    <div v-if="icon || $slots.icon" class="stats-icon" :style="iconStyle">
      <slot name="icon">
        <Icon v-if="icon" :icon="icon" />
      </slot>
    </div>
    <div class="min-w-0 flex-1">
      <div class="stats-label truncate">
        <slot name="label">{{ label }}</slot>
      </div>
      <div class="stats-value truncate" :class="valueClass">
        <slot name="value">{{ value }}</slot>
        <span v-if="sub || $slots.sub" class="stats-sub">
          <slot name="sub">{{ sub }}</slot>
        </span>
      </div>
    </div>
  </div>
</template>
