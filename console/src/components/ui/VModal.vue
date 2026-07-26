<script setup lang="ts">
// Halo VModal 复刻（packages/components/src/components/modal/Modal.vue 简化版）
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
        <div class="absolute inset-0 bg-gray-500/75 transition-opacity" @click="close"></div>
        <Transition name="modal-zoom" appear>
          <div
            :style="{ maxWidth: `${width}px` }"
            class="relative w-full overflow-hidden rounded-base bg-white shadow-xl dark:bg-gray-800"
          >
            <div v-if="title" class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-gray-700">
              <h3 class="text-base font-bold text-gray-900 dark:text-gray-100">{{ title }}</h3>
              <button
                class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700"
                @click="close"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path d="M6 18L18 6M6 6l12 12" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </button>
            </div>
            <div class="max-h-[70vh] overflow-y-auto p-4">
              <slot />
            </div>
            <div v-if="$slots.footer" class="border-t border-gray-100 px-4 py-3 dark:border-gray-700">
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
