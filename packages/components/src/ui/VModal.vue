<script setup lang="ts">
import { ref, watch, nextTick, onBeforeUnmount } from "vue";

// Halo VModal 复刻（Apple Dark Mode 视觉调整与 WAI-ARIA Dialog 无障碍对齐）
const props = withDefaults(
  defineProps<{
    visible?: boolean;
    title?: string;
    width?: number;
    ariaLabel?: string;
  }>(),
  { visible: false, title: undefined, width: 500, ariaLabel: undefined },
);

const emit = defineEmits<{
  (event: "update:visible", visible: boolean): void;
  (event: "close"): void;
}>();

const modalId = `modal-${Math.random().toString(36).slice(2, 9)}`;
const modalRef = ref<HTMLElement | null>(null);
let previouslyFocusedElement: HTMLElement | null = null;

function close() {
  emit("update:visible", false);
  emit("close");
}

function handleKeydown(e: KeyboardEvent) {
  if (!props.visible) return;
  if (e.key === "Escape") {
    e.stopPropagation();
    close();
    return;
  }
  if (e.key === "Tab") {
    const container = modalRef.value;
    if (!container) return;
    const focusables = Array.from(
      container.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ),
    ).filter((el) => el.offsetParent !== null || el === document.activeElement);

    if (focusables.length === 0) {
      e.preventDefault();
      container.focus();
      return;
    }

    const first = focusables[0];
    const last = focusables[focusables.length - 1];

    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      previouslyFocusedElement = document.activeElement as HTMLElement | null;
      window.addEventListener("keydown", handleKeydown, true);
      nextTick(() => {
        const container = modalRef.value;
        if (!container) return;
        const autoFocusEl = container.querySelector<HTMLElement>("[autofocus]");
        if (autoFocusEl) {
          autoFocusEl.focus();
          return;
        }
        const firstFocusable = container.querySelector<HTMLElement>(
          'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
        );
        if (firstFocusable) {
          firstFocusable.focus();
        } else {
          container.focus();
        }
      });
    } else {
      window.removeEventListener("keydown", handleKeydown, true);
      previouslyFocusedElement?.focus?.();
      previouslyFocusedElement = null;
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown, true);
});
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div v-if="visible" class="fixed inset-0 z-[999] flex items-center justify-center p-4">
        <div
          class="absolute inset-0 backdrop-blur-sm"
          style="background: color-mix(in srgb, #000 60%, transparent)"
          @click="close"
        ></div>
        <Transition name="modal-zoom" appear>
          <div
            ref="modalRef"
            role="dialog"
            aria-modal="true"
            :aria-labelledby="title ? `${modalId}-title` : undefined"
            :aria-label="!title ? (ariaLabel || '对话框') : undefined"
            tabindex="-1"
            :style="{ maxWidth: `${width}px` }"
            class="relative w-full overflow-hidden rounded-xl bg-card shadow-xl outline-none"
            style="border: 1px solid var(--color-border)"
          >
            <div
              v-if="title"
              class="flex items-center justify-between px-5 py-4"
              style="border-bottom: 1px solid var(--color-border)"
            >
              <h3 :id="`${modalId}-title`" class="text-base font-semibold text-foreground">{{ title }}</h3>
              <button
                type="button"
                class="rounded-md p-1.5 transition-colors hover:bg-secondary"
                style="color: var(--color-text-tertiary)"
                aria-label="关闭对话框"
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
              style="border-top: 1px solid var(--color-border)"
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
  transition: opacity var(--duration-normal) var(--ease-standard);
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
.modal-zoom-enter-active,
.modal-zoom-leave-active {
  /* Spec 62：`var(--ease-standard)-out` 把 -out 拼在变量值后是非法值（回退默认 ease），
     且缺 leave-active 导致关闭无动画；修正为规范 token 写法并补齐离开态。 */
  transition: all var(--duration-normal) var(--ease-standard);
}
.modal-zoom-enter-from,
.modal-zoom-leave-to {
  opacity: 0;
  transform: scale(0.95);
}
</style>
