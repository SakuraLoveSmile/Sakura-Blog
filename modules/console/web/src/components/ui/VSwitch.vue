<script setup lang="ts">
// Halo VSwitch 复刻（iOS 风格开关）
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
      'relative inline-flex h-[31px] w-[51px] flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-150 ease-standard focus:outline-none',
      modelValue ? 'bg-state-success' : 'bg-accent',
      disabled ? 'cursor-not-allowed opacity-50' : '',
    ]"
    role="switch"
    :aria-checked="modelValue"
    type="button"
    @click="toggle"
  >
    <span
      :class="[
        'pointer-events-none inline-block h-[27px] w-[27px] transform rounded-full bg-white shadow ring-0 transition duration-150 ease-standard',
        modelValue ? 'translate-x-5' : 'translate-x-0',
      ]"
    ></span>
  </button>
</template>
