<script setup lang="ts">
import { computed, ref, watch } from "vue";
import SchemaForm from "@/components/forms/SchemaForm.vue";
import type { SchemaField, SchemaValues } from "@/components/forms/schema";
import type { PluginMeta, PluginVariant } from "./plugin-types";
import { buildShortcode, initFormValues } from "./shortcode";

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
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const values = ref<SchemaValues>({});

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
}

function onSchemaUpdate(next: SchemaValues) {
  values.value = next;
  const shortcodeValues: Record<string, string> = {};
  for (const field of props.plugin.form || []) {
    const value = next[field.key];
    shortcodeValues[field.key] =
      field.type === "flag" ? (value === true ? "true" : "") : value == null ? "" : String(value);
  }
  emit("update:modelValue", buildShortcode(props.plugin, shortcodeValues, props.variant));
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
