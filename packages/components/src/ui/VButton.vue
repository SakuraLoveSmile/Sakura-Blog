<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { RouteLocationRaw } from "vue-router";

// Halo VButton 复刻（packages/components/src/components/button/Button.vue）
const props = withDefaults(
  defineProps<{
    type?: "default" | "primary" | "secondary" | "danger";
    size?: "lg" | "md" | "sm" | "xs";
    circle?: boolean;
    block?: boolean;
    disabled?: boolean;
    loading?: boolean;
    route?: RouteLocationRaw;
  }>(),
  {
    type: "default",
    size: "md",
    circle: false,
    block: false,
    disabled: false,
    loading: false,
    route: undefined,
  }
);

const router = useRouter();
const emit = defineEmits<{ (event: "click"): void }>();

const classes = computed(() => [
  `btn-${props.size}`,
  `btn-${props.type}`,
  { "btn-circle": props.circle },
  { "btn-block": props.block },
  { "btn-loading": props.loading },
]);

function handleClick() {
  if (props.disabled || props.loading) return;
  if (props.route) {
    router.push(props.route);
    return;
  }
  emit("click");
}
</script>

<template>
  <button :class="classes" :disabled="disabled" class="btn" type="button" @click="handleClick">
    <span v-if="$slots.icon || loading" class="btn-icon">
      <svg v-if="loading" class="animate-spin" fill="none" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path
          class="opacity-75"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          fill="currentColor"
        ></path>
      </svg>
      <slot v-else name="icon" />
    </span>
    <span class="btn-content">
      <slot />
    </span>
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  flex-shrink: 0;
  cursor: pointer;
  user-select: none;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  height: 40px;
  padding: 0 16px;
  border: 1px solid transparent;
  outline: 0;
  appearance: none;
  vertical-align: middle;
  border-radius: var(--radius-full);
  transition: all var(--duration-fast) var(--ease-standard);
  background: transparent;
  white-space: nowrap;
}
.btn:hover {
  opacity: 1;
}
.btn:active:not(:disabled) {
  transform: scale(0.97);
}
.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* 默认按钮：描边卡片风格 */
.btn-default {
  border-color: var(--color-border);
  color: var(--color-foreground);
  background: var(--color-card);
}
.btn-default:hover {
  background: var(--color-secondary);
  border-color: var(--color-border-strong);
}
.btn-default .btn-icon {
  color: currentColor;
}

/* 主按钮：蓝底 + 辉光 */
.btn-primary {
  color: var(--color-primary-foreground);
  background: var(--color-primary);
}
.btn-primary:hover:not(:disabled) {
  filter: brightness(0.96);
}
.btn-primary:active:not(:disabled) {
  background: var(--color-primary-active);
  transform: scale(0.97);
}

/* 次要按钮：深灰底 */
.btn-secondary {
  color: var(--color-foreground);
  background: var(--color-secondary);
}
.btn-secondary:hover:not(:disabled) {
  background: var(--color-accent);
}

/* 危险按钮：红色 */
.btn-danger {
  color: var(--color-foreground);
  background: var(--color-state-error);
}
.btn-danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-state-error) 85%, #fff);
  box-shadow: var(--shadow-md);
  transform: translateY(-1px);
}

.btn-block {
  width: 100%;
}

.btn-icon {
  height: 18px;
  width: 18px;
  margin-right: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
/* 图标默认继承按钮文字色（primary 为 primary-foreground，与库规范一致） */
.btn-secondary .btn-icon,
.btn-danger .btn-icon {
  color: var(--color-foreground);
}

.btn-loading {
  cursor: not-allowed;
}
.btn-loading:hover {
  transform: none;
}

.btn-lg {
  height: 44px;
  padding: 0 20px;
  font-size: 16px;
}
.btn-sm {
  height: 28px;
  padding: 0 12px;
  font-size: 13px;
}
.btn-sm .btn-icon {
  height: 14px;
  width: 14px;
  margin-right: 6px;
}
.btn-xs {
  height: 24px;
  padding: 0 8px;
  font-size: 12px;
  border-radius: var(--radius-full);
}
.btn-xs .btn-icon {
  height: 12px;
  width: 12px;
  margin-right: 4px;
}

.btn-circle {
  width: 36px;
  padding: 0;
}
.btn-lg.btn-circle {
  width: 44px;
}
.btn-sm.btn-circle {
  width: 28px;
}
.btn-xs.btn-circle {
  width: 24px;
}
</style>
