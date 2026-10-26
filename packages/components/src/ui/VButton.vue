<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { RouteLocationRaw } from "vue-router";

// Halo VButton 复刻（Apple Dark Mode 现代化精简控件）
const props = withDefaults(
  defineProps<{
    type?: "default" | "primary" | "secondary" | "danger" | "ghost";
    nativeType?: "button" | "submit" | "reset";
    size?: "lg" | "md" | "sm" | "xs";
    circle?: boolean;
    pill?: boolean;
    block?: boolean;
    disabled?: boolean;
    loading?: boolean;
    route?: RouteLocationRaw;
  }>(),
  {
    type: "default",
    nativeType: "button",
    size: "md",
    circle: false,
    pill: false,
    block: false,
    disabled: false,
    loading: false,
    route: undefined,
  },
);

const router = useRouter();
const emit = defineEmits<{ (event: "click", e?: any): void }>();

const classes = computed(() => [
  `btn-${props.size}`,
  `btn-${props.type}`,
  { "btn-circle": props.circle },
  { "btn-pill": props.pill },
  { "btn-block": props.block },
  { "btn-loading": props.loading },
]);

function handleClick(e: MouseEvent) {
  if (props.disabled || props.loading) return;
  if (props.route) {
    router.push(props.route);
    return;
  }
  emit("click", e);
}
</script>

<template>
  <button :class="classes" :disabled="disabled || loading" class="btn" :type="nativeType" @click="handleClick">
    <span v-if="$slots.icon || loading" class="btn-icon">
      <svg v-if="loading" class="animate-spin btn-spinner" fill="none" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path
          class="opacity-75"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          fill="currentColor"
        ></path>
      </svg>
      <slot v-else name="icon" />
    </span>
    <span v-if="$slots.default" class="btn-content">
      <slot />
    </span>
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  flex-shrink: 0;
  flex-wrap: nowrap;
  align-items: center;
  justify-content: center;
  gap: 6px;
  cursor: pointer;
  user-select: none;
  text-align: center;
  text-decoration: none;
  font-weight: 550;
  letter-spacing: 0;
  height: 36px;
  padding: 0 16px;
  font-size: 13.5px;
  line-height: 1;
  border: 1px solid transparent;
  outline: 0;
  appearance: none;
  vertical-align: middle;
  border-radius: var(--radius-sm, 8px);
  transition: background-color var(--duration-fast, 150ms) var(--ease-standard), border-color var(--duration-fast, 150ms) var(--ease-standard), color var(--duration-fast, 150ms) var(--ease-standard), opacity var(--duration-fast, 150ms) var(--ease-standard), transform var(--duration-fast, 150ms) var(--ease-standard);
  background: transparent;
  white-space: nowrap;
  box-sizing: border-box;
}

.btn:hover:not(:disabled) {
  opacity: 1;
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.btn:focus-visible {
  outline: 2px solid var(--color-ring, var(--color-primary, #2e8dff));
  outline-offset: 2px;
}

/* 默认按钮：Apple 深色卡片微描边风格 */
.btn-default {
  border-color: color-mix(in srgb, #fff 12%, transparent);
  color: var(--color-foreground, #f5f5f7);
  background: color-mix(in srgb, var(--color-muted-foreground, #8e8e93) 8%, var(--color-card, #1c1c1e));
  box-shadow: none;
}
.btn-default:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-muted-foreground, #8e8e93) 16%, var(--color-card, #1c1c1e));
  border-color: color-mix(in srgb, #fff 20%, transparent);
}
.btn-default:active:not(:disabled) {
  background: color-mix(in srgb, var(--color-muted-foreground, #8e8e93) 22%, var(--color-card, #1c1c1e));
}

/* 主按钮：品牌蓝底 + 黑色高对比前景色 + 无外发光 */
.btn-primary {
  color: var(--color-primary-foreground, #000000);
  background: var(--color-primary, #2e8dff);
  border-color: transparent;
  box-shadow: none;
}
.btn-primary:hover:not(:disabled) {
  background: var(--color-primary-hover, #66abff);
  box-shadow: none;
}
.btn-primary:active:not(:disabled) {
  background: var(--color-primary-active, #007aff);
}

/* 次要按钮：深色半透明胶囊 */
.btn-secondary {
  color: var(--color-foreground, #f5f5f7);
  background: color-mix(in srgb, #fff 8%, transparent);
  border-color: color-mix(in srgb, #fff 8%, transparent);
}
.btn-secondary:hover:not(:disabled) {
  background: color-mix(in srgb, #fff 14%, transparent);
  border-color: color-mix(in srgb, #fff 14%, transparent);
}
.btn-secondary:active:not(:disabled) {
  background: color-mix(in srgb, #fff 20%, transparent);
}

/* 幽灵按钮：透明底 */
.btn-ghost {
  color: var(--color-muted-foreground, #8e8e93);
  background: transparent;
  border-color: transparent;
}
.btn-ghost:hover:not(:disabled) {
  color: var(--color-foreground, #f5f5f7);
  background: color-mix(in srgb, #fff 8%, transparent);
}

/* 危险按钮：Apple 红色 + 黑色高对比前景色（与主按钮「蓝底黑字」策略一致）+ 无外发光 */
.btn-danger {
  color: var(--color-primary-foreground, #000000);
  background: var(--color-state-error, #ff453a);
  border-color: transparent;
  box-shadow: none;
}
.btn-danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-state-error, #ff453a) 88%, #fff);
  box-shadow: none;
}
.btn-danger:active:not(:disabled) {
  background: color-mix(in srgb, var(--color-state-error, #ff453a) 85%, #000);
}

.btn-block {
  width: 100%;
}

.btn-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  line-height: 1;
}

.btn-spinner {
  width: 14px;
  height: 14px;
}

.btn-content {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  white-space: nowrap;
  line-height: 1;
}

.btn-loading {
  cursor: not-allowed;
}

/* 尺寸阶梯：xs 24 / sm 28 / md 36 / lg 40px */
.btn-lg {
  height: 40px;
  padding: 0 18px;
  font-size: 14.5px;
}
.btn-lg .btn-spinner {
  width: 16px;
  height: 16px;
}

.btn-md {
  height: 36px;
  padding: 0 16px;
  font-size: 13.5px;
}
.btn-md .btn-spinner {
  width: 14px;
  height: 14px;
}

.btn-sm {
  height: 28px;
  padding: 0 11px;
  font-size: 12px;
  gap: 4px;
}
.btn-sm .btn-spinner {
  width: 12px;
  height: 12px;
}

.btn-xs {
  height: 24px;
  padding: 0 8px;
  font-size: 11.5px;
  gap: 4px;
}
.btn-xs .btn-spinner {
  width: 11px;
  height: 11px;
}

/* 圆角规则放在尺寸规则之后，避免 sm/lg 覆盖胶囊形状 */
.btn-pill {
  border-radius: 9999px;
}

/* 圆形按钮：保持正圆 */
.btn-circle {
  width: 36px;
  height: 36px;
  padding: 0;
  border-radius: 9999px;
}
.btn-lg.btn-circle {
  width: 40px;
  height: 40px;
}
.btn-sm.btn-circle {
  width: 28px;
  height: 28px;
}
.btn-xs.btn-circle {
  width: 24px;
  height: 24px;
}

/* 移动端触控尺寸保证：手机宽度 <768px 时点击区域至少 44px */
@media (max-width: 767px) {
  .btn {
    min-height: 44px;
  }
  .btn.btn-circle {
    min-width: 44px;
    min-height: 44px;
  }
}
</style>
