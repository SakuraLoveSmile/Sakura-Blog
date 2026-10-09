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
    class="sticky top-0 z-20 flex min-h-[58px] flex-wrap items-center justify-between gap-3 px-4 py-3.5 backdrop-blur-2xl transition-all sm:px-6 sm:py-4 md:px-8"
    :class="{
      'page-header-scroll-aware': hideOnScroll,
      'page-header-scroll-hidden': hideOnScroll && hidden,
      'page-header-stack-below-lg': actionsLayout === 'stack-below-lg',
    }"
    :aria-hidden="hideOnScroll && hidden ? 'true' : undefined"
    :inert="hideOnScroll && hidden"
    style="
      background: color-mix(in srgb, #000 82%, transparent);
      border-bottom: 1px solid var(--color-border);
      box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.35);
    "
  >
    <div
      class="min-w-0 self-center"
      :class="actionsLayout === 'stack-below-lg' ? 'w-full lg:w-auto lg:flex-1' : 'flex-1'"
    >
      <div class="flex items-center gap-2.5">
        <slot name="icon" />
        <h1 class="truncate text-xl font-semibold tracking-tight text-foreground sm:text-[22px] md:text-[24px]">
          <slot name="title">{{ title }}</slot>
        </h1>
        <slot name="badge" />
      </div>
      <p v-if="subtitle || $slots.subtitle" class="mt-0.5 truncate text-xs text-text-tertiary sm:text-[13px]">
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
