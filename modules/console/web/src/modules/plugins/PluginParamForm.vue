<script setup lang="ts">
import { computed, ref, watch } from "vue";
import SchemaForm from "@/components/forms/SchemaForm.vue";
import type { SchemaField, SchemaValues } from "@/components/forms/schema";
import type { PluginMeta } from "./plugins-data";
import { buildShortcode } from "./shortcode";

// 标签插件的领域适配器：通用字段渲染交给 SchemaForm，shortcode 语义仍由本模块负责。
// 选中插件时只初始化默认值，不用拼装结果覆盖插件库内的精选示例；用户编辑字段时才产出。
const props = defineProps<{
  plugin: PluginMeta;
  modelValue: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const values = ref<SchemaValues>({});

const fields = computed<SchemaField[]>(() =>
  (props.plugin.form || []).map((field) => ({
    key: field.key,
    label: field.label,
    type: field.type === "flag" ? "switch" : field.type,
    default: field.type === "flag" ? field.default === "true" : field.default,
    options: field.options,
    placeholder: field.placeholder,
  }))
);

function initValues() {
  const next: SchemaValues = {};
  for (const field of props.plugin.form || []) {
    next[field.key] = field.type === "flag" ? field.default === "true" : field.default ?? "";
  }
  values.value = next;
}

function onSchemaUpdate(next: SchemaValues) {
  values.value = next;
  const shortcodeValues: Record<string, string> = {};
  for (const field of props.plugin.form || []) {
    const value = next[field.key];
    shortcodeValues[field.key] =
      field.type === "flag" ? (value === true ? "true" : "") : value == null ? "" : String(value);
  }
  emit("update:modelValue", buildShortcode(props.plugin, shortcodeValues));
}

watch(
  () => props.plugin.name,
  () => {
    initValues();
  },
  { immediate: true }
);
</script>

<template>
  <SchemaForm :fields="fields" :model-value="values" @update:model-value="onSchemaUpdate" />
</template>
