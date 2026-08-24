<script setup lang="ts">
import { toRef } from "vue";
import { provideWidgetSize } from "./useWidgetSize";
import type { WidgetSizePreset } from "./widgets/layout";

const props = defineProps<{
  preset: WidgetSizePreset;
  gridW: number;
  gridH: number;
}>();

provideWidgetSize(toRef(props, "preset"), toRef(props, "gridW"), toRef(props, "gridH"));
</script>

<template>
  <slot />
</template>
