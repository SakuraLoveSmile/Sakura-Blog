<script setup lang="ts">
import { ref, watch } from"vue";
import type { PluginMeta } from"./plugins-data";
import { buildShortcode } from"./shortcode";

// 标签插件结构化参数表单（Spec 29）。
// 入参一个具备 form schema 的插件，按字段 role 拼装 shortcode 并经 v-model 产出。
// 拼装纯逻辑在 ./shortcode.ts（便于 node:test 行为测试），本组件只管表单状态与产出。
const props = defineProps<{
 plugin: PluginMeta;
 modelValue: string;
}>();
const emit = defineEmits<{ (e:"update:modelValue", v: string): void }>();

const values = ref<Record<string, string>>({});

function initValues() {
 const v: Record<string, string> = {};
 for (const f of props.plugin.form || []) v[f.key] = f.default ??"";
 values.value = v;
}

function refresh() {
 emit("update:modelValue", buildShortcode(props.plugin, values.value));
}

watch(
 () => props.plugin.name,
 () => {
 initValues();
 refresh();
 },
 { immediate: true }
);

const inputCls = "apple-input";
</script>

<template>
 <div class="space-y-2.5">
 <div v-for="f in plugin.form || []" :key="f.key">
 <label class="block text-xs text-text-tertiary mb-1">{{ f.label }}</label>
 <select
 v-if="f.type === 'select'"
 v-model="values[f.key]"
 @change="refresh"
 :class="inputCls"
 >
 <option v-for="opt in f.options" :key="opt" :value="opt">{{ opt ||"（默认）" }}</option>
 </select>
 <label
 v-else-if="f.type === 'flag'"
 class="flex items-center gap-2 text-sm text-text-tertiary"
 >
 <input
 type="checkbox"
 :checked="values[f.key] === 'true'"
 @change="values[f.key] = ($event.target as HTMLInputElement).checked ? 'true' : ''; refresh()"
 class="rounded"
 />
 启用
 </label>
 <textarea
 v-else-if="f.type === 'textarea'"
 v-model="values[f.key]"
 @input="refresh"
 rows="4"
 spellcheck="false"
 :placeholder="f.placeholder"
 :class="inputCls + ' resize-y font-mono text-xs leading-relaxed'"
 ></textarea>
 <input
 v-else
 v-model="values[f.key]"
 @input="refresh"
 type="text"
 :placeholder="f.placeholder"
 :class="inputCls"
 />
 </div>
 </div>
</template>
