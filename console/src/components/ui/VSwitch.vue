<script setup lang="ts">
// Halo VSwitch 复刻（packages/components/src/components/switch）
const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    disabled?: boolean;
  }>(),
  { modelValue: false, disabled: false }
);

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void;
  (event: "change", value: boolean): void;
}>();

function toggle() {
  if (props.disabled) return;
  const next = !props.modelValue;
  emit("update:modelValue", next);
  emit("change", next);
}
</script>

<template>
  <button
    :class="[
      'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
      modelValue ? 'bg-primary' : 'bg-gray-200 dark:bg-gray-600',
      disabled ? 'cursor-not-allowed opacity-50' : '',
    ]"
    role="switch"
    :aria-checked="modelValue"
    type="button"
    @click="toggle"
  >
    <span
      :class="[
        'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
        modelValue ? 'translate-x-5' : 'translate-x-0',
      ]"
    ></span>
  </button>
</template>
