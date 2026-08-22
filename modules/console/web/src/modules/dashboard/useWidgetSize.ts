import { computed, inject, provide, type InjectionKey, type Ref } from "vue";
import type { WidgetSizePreset } from "./widgets/layout";

export interface WidgetSizeContext {
  preset: Ref<WidgetSizePreset>;
  gridW?: Ref<number>;
  gridH?: Ref<number>;
}

export const WIDGET_SIZE_KEY: InjectionKey<WidgetSizeContext> = Symbol("dashboard-widget-size");

export function provideWidgetSize(
  preset: Ref<WidgetSizePreset>,
  gridW?: Ref<number>,
  gridH?: Ref<number>,
) {
  const context: WidgetSizeContext = { preset, gridW, gridH };
  provide(WIDGET_SIZE_KEY, context);
  return context;
}

export function useWidgetSize(fallbackPreset: WidgetSizePreset = "medium") {
  const context = inject(WIDGET_SIZE_KEY, null);
  const preset = computed<WidgetSizePreset>(() => context?.preset.value ?? fallbackPreset);
  const gridW = computed<number | undefined>(() => context?.gridW?.value);
  const gridH = computed<number | undefined>(() => context?.gridH?.value);

  const isSmall = computed(() => preset.value === "small");
  const isMedium = computed(() => preset.value === "medium");
  const isLarge = computed(() => preset.value === "large");

  return {
    preset,
    gridW,
    gridH,
    isSmall,
    isMedium,
    isLarge,
  };
}
