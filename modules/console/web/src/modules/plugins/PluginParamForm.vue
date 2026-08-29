<script setup lang="ts">
import { computed, ref, watch } from "vue";
import SchemaForm from "@/components/forms/SchemaForm.vue";
import type { SchemaField, SchemaValues } from "@/components/forms/schema";
import type { PluginMeta, PluginVariant } from "./plugin-types";
import { buildShortcodeResult, initFormValues } from "./shortcode";

// 标签插件的领域适配器：通用字段渲染交给 SchemaForm，shortcode 语义仍由本模块负责。
// 插入模式（initialValues 未传）用 catalog default 初始化；
// 编辑模式（initialValues 来自 /plugins/shortcode/parse）只用 raw 真实解析出的值，
// 缺失字段取空值，绝不回填 default，避免保存时静默写入作者没写过的参数。
const props = defineProps<{
  plugin: PluginMeta;
  modelValue: string;
  initialValues?: Record<string, unknown>;
  variant?: PluginVariant;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  validation: [state: { valid: boolean; error?: string }];
}>();

const values = ref<SchemaValues>({});
const validation = ref<{ valid: boolean; error?: string }>({ valid: true });

const fields = computed<SchemaField[]>(() =>
  (props.plugin.form || []).map((field) => ({
    key: field.key,
    label: field.label,
    label_key: field.label_key,
    type: field.type === "flag" ? "switch" : field.type,
    default: field.type === "flag" ? field.default === "true" : field.default,
    options: field.options,
    placeholder: field.placeholder,
    hint: field.hint,
    hint_key: field.hint_key,
  })),
);

function initValues() {
  values.value = initFormValues(props.plugin.form || [], props.initialValues);
  updateValidation(values.value, false);
}

function shortcodeValues(next: SchemaValues): Record<string, string | boolean> {
  const shortcodeValues: Record<string, string> = {};
  for (const field of props.plugin.form || []) {
    const value = next[field.key];
    shortcodeValues[field.key] =
      field.type === "flag" ? (value === true ? "true" : "") : value == null ? "" : String(value);
  }
  return shortcodeValues;
}

function updateValidation(next: SchemaValues, emitUpdate: boolean) {
  const result = buildShortcodeResult(props.plugin, shortcodeValues(next), props.variant);
  validation.value = result.valid ? { valid: true } : { valid: false, error: result.error };
  emit("validation", validation.value);
  if (emitUpdate && result.valid) emit("update:modelValue", result.raw);
}

function onSchemaUpdate(next: SchemaValues) {
  values.value = next;
  updateValidation(next, true);
}

watch(
  [() => props.plugin.name, () => props.initialValues, () => props.variant],
  () => {
    initValues();
  },
  { immediate: true, deep: true },
);
</script>

<template>
  <SchemaForm :fields="fields" :model-value="values" @update:model-value="onSchemaUpdate" />
</template>
