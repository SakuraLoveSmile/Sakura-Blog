<script setup lang="ts">
import { ref, watch } from "vue";
import type { PluginMeta } from "./plugins-data";

// 标签插件结构化参数表单（Spec 29）。
// 入参一个具备 form schema 的插件，按字段 role 拼装 shortcode 并经 v-model 产出。
// 拼装规则：positional 按字段顺序拼在标签名后；named 拼 key:value（值非空才拼）；
// flag 为真时仅拼 key；content 作正文，块级插件包在 {% name %}…{% endname %} 之间。
const props = defineProps<{
  plugin: PluginMeta;
  modelValue: string;
}>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();

const values = ref<Record<string, string>>({});

function initValues() {
  const v: Record<string, string> = {};
  for (const f of props.plugin.form || []) v[f.key] = f.default ?? "";
  values.value = v;
}

function build(): string {
  const form = props.plugin.form || [];
  const positional: string[] = [];
  const named: string[] = [];
  let content = "";
  for (const f of form) {
    const raw = (values.value[f.key] ?? "").trim();
    if (f.role === "content") {
      content = values.value[f.key] ?? "";
    } else if (f.role === "flag") {
      if (raw === "true") named.push(f.key);
    } else if (f.role === "named") {
      if (raw) named.push(`${f.key}:${raw}`);
    } else {
      if (raw) positional.push(raw);
    }
  }
  const head = [props.plugin.name, ...positional, ...named].join(" ");
  if (props.plugin.type === "block") {
    const body = content.trim() ? `\n${content.trim()}\n` : " ";
    return `{% ${head} %}${body}{% end${props.plugin.name} %}`;
  }
  // inline 单标签插件（如 note）：正文作为最后一个位置参数拼入标签内
  const inline = content.trim() ? `${head} ${content.trim()}` : head;
  return `{% ${inline} %}`;
}

function refresh() {
  emit("update:modelValue", build());
}

watch(
  () => props.plugin.name,
  () => {
    initValues();
    refresh();
  },
  { immediate: true }
);

const inputCls =
  "w-full px-2.5 py-1.5 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-400 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
</script>

<template>
  <div class="space-y-2.5">
    <div v-for="f in plugin.form || []" :key="f.key">
      <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">{{ f.label }}</label>
      <select
        v-if="f.type === 'select'"
        v-model="values[f.key]"
        @change="refresh"
        :class="inputCls"
      >
        <option v-for="opt in f.options" :key="opt" :value="opt">{{ opt || "（默认）" }}</option>
      </select>
      <label
        v-else-if="f.type === 'flag'"
        class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400"
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
