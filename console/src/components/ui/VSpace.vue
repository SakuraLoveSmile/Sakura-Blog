<script setup lang="ts">
import { computed } from "vue";

// Halo VSpace 复刻（packages/components/src/components/space/Space.vue）
// 统一的间距布局组件，用于按钮组、表单项等场景。
const props = withDefaults(
  defineProps<{
    /** 主轴方向 */
    direction?: "row" | "column";
    /** 间距大小 */
    spacing?: "xs" | "sm" | "md" | "lg";
    /** 对齐方式 */
    align?: "start" | "center" | "end" | "stretch";
    /** 是否换行（仅 row 方向生效） */
    wrap?: boolean;
  }>(),
  {
    direction: "row",
    spacing: "md",
    align: "center",
    wrap: false,
  }
);

const spacingMap = {
  xs: "0.25rem",
  sm: "0.5rem",
  md: "1rem",
  lg: "1.5rem",
};

const styles = computed(() => ({
  display: "flex",
  flexDirection: props.direction,
  alignItems: props.align === "stretch" ? "stretch" : props.align,
  flexWrap: props.wrap ? ("wrap" as const) : ("nowrap" as const),
  gap: spacingMap[props.spacing],
}));
</script>

<template>
  <div :style="styles">
    <slot />
  </div>
</template>
