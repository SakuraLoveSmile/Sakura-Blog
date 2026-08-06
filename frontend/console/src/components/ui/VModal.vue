<script setup lang="ts">
// Halo VModal 复刻（Apple Dark Mode 视觉调整）
withDefaults(
  defineProps<{
    visible?: boolean;
    title?: string;
    width?: number;
  }>(),
  { visible: false, title: undefined, width: 500 }
);

const emit = defineEmits<{
  (event: "update:visible", visible: boolean): void;
  (event: "close"): void;
}>();

function close() {
  emit("update:visible", false);
  emit("close");
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div v-if="visible" class="fixed inset-0 z-[999] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 backdrop-blur-sm"
          style="background: rgba(0,0,0,0.6);"
          @click="close"
        ></div>
        <Transition name="modal-zoom" appear>
          <div
            :style="{ maxWidth: `${width}px` }"
            class="relative w-full overflow-hidden rounded-xl bg-card shadow-xl"
            style="border: 1px solid var(--color-border);"
          >
            <div
              v-if="title"
              class="flex items-center justify-between px-5 py-4"
              style="border-bottom: 1px solid var(--color-border);"
            >
              <h3 class="text-base font-semibold text-foreground">{{ title }}</h3>
              <button
                class="rounded-md p-1.5 transition-colors hover:bg-secondary"
                style="color: var(--color-text-tertiary);"
                @click="close"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path d="M6 18L18 6M6 6l12 12" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
            </div>
            <div class="max-h-[70vh] overflow-y-auto p-5">
              <slot />
            </div>
            <div
              v-if="$slots.footer"
              class="flex justify-end gap-2 px-5 py-4"
              style="border-top: 1px solid var(--color-border);"
            >
              <slot name="footer" />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
.modal-zoom-enter-active {
  transition: all 0.2s ease-out;
}
.modal-zoom-enter-from {
  opacity: 0;
  transform: scale(0.95);
}
</style>
