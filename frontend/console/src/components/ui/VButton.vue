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
  font-size: 0.875rem;
  text-decoration: none;
  height: 2.25rem;
  padding: 0 1rem;
  border: none;
  outline: 0;
  appearance: none;
  vertical-align: middle;
  border-radius: var(--radius-base);
  transition: all 0.15s ease;
  background: transparent;
}
.btn:hover {
  opacity: 0.9;
}
.btn:active {
  opacity: 1;
}

/* 默认按钮 hover 改背景加深而非透明度，避免白底上发灰看不清（令牌随亮/暗切换） */
.btn-default:hover {
  opacity: 1;
  background: var(--color-muted);
  border-color: var(--color-muted-foreground);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-default {
  border: 1px solid var(--color-border);
  color: var(--color-foreground);
  background: var(--color-card);
}
/* 图标跟随文字颜色，与文案对比度一致，避免近黑色在浅色按钮上突兀 */
.btn-default .btn-icon {
  color: currentColor;
}

.btn-primary {
  color: var(--color-primary-foreground) !important;
  background: var(--color-primary) !important;
}
.btn-secondary {
  color: #fff !important;
  background: var(--color-secondary) !important;
}
.btn-danger {
  color: #fff !important;
  background: var(--color-state-error) !important;
}

.btn-block {
  width: 100%;
}

.btn-icon {
  height: 1.25rem;
  width: 1.25rem;
  margin-right: 0.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.btn-primary .btn-icon,
.btn-secondary .btn-icon,
.btn-danger .btn-icon {
  color: #fff;
}

.btn-loading {
  cursor: not-allowed;
}
.btn-loading:hover {
  opacity: 1;
}

.btn-lg {
  height: 2.75rem;
  padding: 0 1.25rem;
  font-size: 1.125rem;
}
.btn-sm {
  height: 1.75rem;
  padding: 0 0.75rem;
  font-size: 0.75rem;
}
.btn-sm .btn-icon {
  height: 0.75rem;
  width: 0.75rem;
  margin-right: 0.5rem;
}
.btn-xs {
  height: 1.5rem;
  padding: 0 0.5rem;
  font-size: 0.75rem;
}
.btn-xs .btn-icon {
  height: 0.75rem;
  width: 0.75rem;
  margin-right: 0.5rem;
}

.btn-circle {
  width: 2.25rem;
  padding: 0;
  border-radius: 9999px;
}
.btn-lg.btn-circle {
  width: 2.75rem;
}
.btn-sm.btn-circle {
  width: 1.75rem;
}
.btn-xs.btn-circle {
  width: 1.5rem;
}
</style>
