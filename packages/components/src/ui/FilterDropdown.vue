<script setup lang="ts">
import { ref, nextTick, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";

// Halo FloatingDropdown 筛选下拉复刻（触发器 + w-72 popper 列表与 WAI-ARIA Listbox 支持）
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
const triggerBtn = ref<HTMLButtonElement>();
const listboxRef = ref<HTMLUListElement>();
const focusedIndex = ref(-1);
const dropdownId = `filter-dropdown-${Math.random().toString(36).slice(2, 9)}`;
const listboxId = `${dropdownId}-listbox`;

function select(item: FilterItem) {
  emit("update:modelValue", item.value);
  emit("change", item.value);
  open.value = false;
  triggerBtn.value?.focus();
}

function onClickOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false;
  }
}

function toggleDropdown() {
  open.value = !open.value;
  if (open.value) {
    const idx = props.items.findIndex((item) => item.value === props.modelValue);
    focusedIndex.value = idx >= 0 ? idx : 0;
    nextTick(() => {
      focusOption(focusedIndex.value);
    });
  }
}

function focusOption(index: number) {
  if (!listboxRef.value) return;
  const options = listboxRef.value.querySelectorAll<HTMLElement>("[role='option']");
  if (options[index]) {
    options[index].focus();
  }
}

function onTriggerKeydown(e: KeyboardEvent) {
  if (e.key === "ArrowDown" || e.key === "ArrowUp" || e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    if (!open.value) {
      toggleDropdown();
    }
  }
}

function onListboxKeydown(e: KeyboardEvent) {
  if (!open.value) return;
  if (e.key === "Escape") {
    e.stopPropagation();
    open.value = false;
    triggerBtn.value?.focus();
    return;
  }
  if (e.key === "ArrowDown") {
    e.preventDefault();
    if (props.items.length === 0) return;
    focusedIndex.value = (focusedIndex.value + 1) % props.items.length;
    focusOption(focusedIndex.value);
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    if (props.items.length === 0) return;
    focusedIndex.value = (focusedIndex.value - 1 + props.items.length) % props.items.length;
    focusOption(focusedIndex.value);
    return;
  }
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    if (focusedIndex.value >= 0 && focusedIndex.value < props.items.length) {
      select(props.items[focusedIndex.value]);
    }
    return;
  }
  if (e.key === "Tab") {
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
    <button
      ref="triggerBtn"
      type="button"
      class="apple-filter-btn flex cursor-pointer select-none items-center text-sm text-text-secondary transition-colors hover:text-foreground bg-transparent border-0 p-0 outline-none"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="listboxId"
      :aria-label="label"
      @click="toggleDropdown"
      @keydown="onTriggerKeydown"
    >
      <span class="mr-0.5">{{ label }}</span>
      <span><Icon icon="ri:arrow-down-s-line" /></span>
    </button>
    <Transition name="dropdown-fade">
      <div
        v-if="open"
        class="absolute right-0 top-full z-30 mt-2 w-72 rounded-lg border border-border bg-card p-4 shadow-lg outline-none"
      >
        <ul
          :id="listboxId"
          ref="listboxRef"
          role="listbox"
          :aria-label="label"
          class="space-y-1 outline-none"
          tabindex="-1"
          @keydown="onListboxKeydown"
        >
          <li
            v-for="(item, index) in items"
            :id="`${dropdownId}-opt-${index}`"
            :key="index"
            role="option"
            tabindex="0"
            :aria-selected="isSelected(item)"
            :class="{ 'bg-secondary': isSelected(item), 'ring-1 ring-primary': focusedIndex === index }"
            class="flex cursor-pointer items-center rounded px-3 py-2 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground outline-none"
            @click="select(item)"
            @focus="focusedIndex = index"
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
  transition: background-color var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}
.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
