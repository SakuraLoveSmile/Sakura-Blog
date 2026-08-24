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
        class="fixed bottom-6 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-full border border-border px-4 py-2.5 shadow-2xl backdrop-blur-2xl transition-all"
        style="
          background: color-mix(in srgb, var(--color-card) 85%, transparent);
          box-shadow:
            0 16px 36px -8px rgba(0, 0, 0, 0.65),
            0 0 0 1px rgba(255, 255, 255, 0.08);
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
        <div class="flex items-center gap-2">
          <slot />
        </div>

        <!-- 取消选择按钮 -->
        <button
          type="button"
          class="ml-1 rounded-full p-1 text-text-quaternary transition-colors hover:bg-secondary hover:text-foreground"
          title="取消全选"
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
