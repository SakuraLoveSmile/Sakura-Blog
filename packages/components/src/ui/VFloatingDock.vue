<script setup lang="ts">
// VFloatingDock（Apple Dark Mode 底部悬浮胶囊批量操作栏）
withDefaults(
  defineProps<{
    visible?: boolean;
    count?: number;
    title?: string;
  }>(),
  {
    visible: false,
    count: 0,
    title: "",
  },
);

defineEmits<{
  (event: "clear"): void;
}>();
</script>

<template>
  <Teleport to="body">
    <Transition name="dock-slide">
      <div
        v-if="visible && count > 0"
        class="v-floating-dock fixed bottom-[calc(4.5rem+env(safe-area-inset-bottom,0px))] left-1/2 z-50 flex -translate-x-1/2 max-w-[calc(100%-2rem)] flex-wrap justify-center items-center gap-3 rounded-lg border border-border px-4 py-2.5 shadow-lg  md:bottom-6"
        style="
          background: var(--color-card);
          box-shadow: var(--shadow-lg);
        "
      >
        <!-- 已选计数药丸 -->
        <div class="flex items-center gap-2 pr-1 border-r border-border">
          <span
            class="flex h-6 min-w-6 items-center justify-center rounded-full bg-primary px-2 text-xs font-bold text-primary-foreground"
          >
            {{ count }}
          </span>
          <span class="text-xs font-medium text-foreground whitespace-nowrap">
            {{ title || "已选择" }}
          </span>
        </div>

        <!-- 动作操作区 -->
        <div class="flex flex-wrap items-center justify-center gap-2">
          <slot />
        </div>

        <!-- 取消选择按钮 -->
        <button
          type="button"
          class="ml-1 flex min-h-9 min-w-9 items-center justify-center rounded-lg p-1 text-text-quaternary transition-colors hover:bg-secondary hover:text-foreground"
          title="取消全选"
          aria-label="取消全选"
          @click="$emit('clear')"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dock-slide-enter-active,
.dock-slide-leave-active {
  transition:
    transform var(--duration-normal) var(--ease-standard),
    opacity var(--duration-normal) var(--ease-standard);
}
.dock-slide-enter-from,
.dock-slide-leave-to {
  opacity: 0;
  transform: translate(-50%, 20px) scale(0.96);
}
</style>
