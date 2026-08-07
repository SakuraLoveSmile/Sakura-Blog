<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";

// Halo FloatingDropdown 筛选下拉复刻（触发器 + w-72 popper 列表）
export interface FilterItem {
  label: string;
  value?: string;
}

const props = defineProps<{
  label: string;
  items: FilterItem[];
  modelValue?: string;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: string | undefined): void;
  (event: "change", value: string | undefined): void;
}>();

const open = ref(false);
const rootEl = ref<HTMLElement>();

function select(item: FilterItem) {
  emit("update:modelValue", item.value);
  emit("change", item.value);
  open.value = false;
}

function onClickOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false;
  }
}

onMounted(() => document.addEventListener("click", onClickOutside));
onBeforeUnmount(() => document.removeEventListener("click", onClickOutside));

defineExpose({ open });
const isSelected = (item: FilterItem) => props.modelValue === item.value;
</script>

<template>
  <div ref="rootEl" class="relative">
    <div
      class="flex cursor-pointer select-none items-center text-sm text-text-secondary transition-colors hover:text-foreground"
      @click="open = !open"
    >
      <span class="mr-0.5">{{ label }}</span>
      <span><Icon icon="ri:arrow-down-s-line" /></span>
    </div>
    <Transition name="dropdown-fade">
      <div
        v-if="open"
        class="absolute right-0 top-full z-30 mt-2 w-72 rounded-lg border border-border bg-card p-4 shadow-lg"
      >
        <ul class="space-y-1">
          <li
            v-for="(item, index) in items"
            :key="index"
            :class="{ 'bg-secondary': isSelected(item) }"
            class="flex cursor-pointer items-center rounded px-3 py-2 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground"
            @click="select(item)"
          >
            <span class="truncate">{{ item.label }}</span>
          </li>
        </ul>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all var(--duration-fast) var(--ease-standard);
}
.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
