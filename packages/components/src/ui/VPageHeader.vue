<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";

// VPageHeader（Apple Dark Mode 增强型吸顶标题栏）
const props = withDefaults(
  defineProps<{
    title?: string;
    subtitle?: string;
    hideOnScroll?: boolean;
    actionsLayout?: "default" | "stack-below-lg";
  }>(),
  {
    hideOnScroll: false,
    actionsLayout: "default",
  },
);

const headerEl = ref<HTMLElement | null>(null);
const hidden = ref(false);
let scrollContainer: HTMLElement | null = null;
let listenToWindow = false;
let lastScrollTop = 0;

function findScrollContainer(element: HTMLElement | null): HTMLElement | null {
  let parent = element?.parentElement ?? null;
  while (parent) {
    const overflowY = window.getComputedStyle(parent).overflowY;
    if (overflowY === "auto" || overflowY === "scroll" || overflowY === "overlay") return parent;
    parent = parent.parentElement;
  }
  return document.scrollingElement as HTMLElement | null;
}

function currentScrollTop(): number {
  return scrollContainer ? scrollContainer.scrollTop : window.scrollY;
}

function handleScroll() {
  const current = currentScrollTop();
  const delta = current - lastScrollTop;
  if (current <= 12) {
    hidden.value = false;
  } else if (delta > 2) {
    hidden.value = true;
  } else if (delta < -2) {
    hidden.value = false;
  }
  lastScrollTop = current;
}

onMounted(() => {
  if (!props.hideOnScroll) return;
  scrollContainer = findScrollContainer(headerEl.value);
  lastScrollTop = currentScrollTop();
  if (scrollContainer) scrollContainer.addEventListener("scroll", handleScroll, { passive: true });
  else {
    listenToWindow = true;
    window.addEventListener("scroll", handleScroll, { passive: true });
  }
});

onUnmounted(() => {
  if (scrollContainer) scrollContainer.removeEventListener("scroll", handleScroll);
  if (listenToWindow) window.removeEventListener("scroll", handleScroll);
});
</script>

<template>
  <header
    ref="headerEl"
    class="page-header sticky top-0 z-20 flex min-h-0 flex-wrap items-center justify-between gap-3 px-4 pb-2 pt-6 transition-transform duration-200 sm:px-6 sm:pt-7 md:px-8"
    :class="{
      'page-header-scroll-aware': hideOnScroll,
      'page-header-scroll-hidden': hideOnScroll && hidden,
      'page-header-stack-below-lg': actionsLayout === 'stack-below-lg',
    }"
    :aria-hidden="hideOnScroll && hidden ? 'true' : undefined"
    :inert="hideOnScroll && hidden"
    style="background: var(--color-background)"
  >
    <div
      class="page-header-title min-w-0 self-center"
      :class="actionsLayout === 'stack-below-lg' ? 'w-full lg:w-auto lg:flex-1' : 'flex-1'"
    >
      <div class="flex items-center gap-2.5">
        <h1 class="truncate text-[20px] font-semibold tracking-[-0.02em] text-foreground sm:text-[24px]">
          <slot name="title">{{ title }}</slot>
        </h1>
        <slot name="badge" />
      </div>
      <p v-if="subtitle || $slots.subtitle" class="mt-1 truncate text-[12px] text-text-tertiary">
        <slot name="subtitle">{{ subtitle }}</slot>
      </p>
    </div>
    <div
      v-if="$slots.actions"
      class="page-header-actions flex min-w-0 max-w-full flex-wrap items-center gap-2 self-center sm:gap-2.5"
      :class="actionsLayout === 'stack-below-lg' ? 'w-full lg:w-auto lg:flex-shrink-0' : ''"
    >
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
/* 标题栏只用留白与字号建立层级，不再使用下边框与标题图标。 */

/* 手机标题独占一行，避免搜索与操作把标题压成零宽。 */
@media (max-width: 767px) {
  .page-header-title {
    flex: none;
    width: 100%;
  }
  .page-header-actions {
    width: 100%;
  }
}

.page-header-scroll-aware {
  will-change: transform, opacity;
  transition:
    transform var(--duration-normal) var(--ease-standard),
    opacity var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard);
}

.page-header-scroll-hidden {
  pointer-events: none;
  transform: translateY(calc(-100% - 1px));
  opacity: 0;
}
</style>
