<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { previewApi } from "@/api";
import {
  plugins,
  PLUGIN_CATEGORIES,
  type PluginMeta,
  type PluginCategory,
} from "./plugins-data";

// Console 插件库（Spec 23）：29 个标签插件的分类速查 + 实时渲染预览 + 一键复制。
// 预览复用后端 POST /api/v1/preview/markdown 渲染管线，
// 通过 iframe srcdoc 注入前台 main.css + tag-plugins.css，保证样式与前台一致。

const search = ref("");
const activeCategory = ref<PluginCategory | "全部">("全部");
const selected = ref<PluginMeta>(plugins[0]);
const example = ref(plugins[0].example);

// 预览主题：默认跟随 Console 当前主题，可独立切换亮/暗。
const previewTheme = ref<"light" | "dark">(
  document.documentElement.dataset.theme === "dark" ? "dark" : "light"
);

const filtered = computed(() => {
  const kw = search.value.trim().toLowerCase();
  return plugins.filter((p) => {
    if (activeCategory.value !== "全部" && p.category !== activeCategory.value)
      return false;
    if (!kw) return true;
    return (
      p.name.toLowerCase().includes(kw) ||
      p.title.toLowerCase().includes(kw) ||
      p.desc.toLowerCase().includes(kw)
    );
  });
});

const categoryCount = computed(() => {
  const map: Record<string, number> = { 全部: plugins.length };
  for (const c of PLUGIN_CATEGORIES) map[c] = 0;
  for (const p of plugins) map[p.category]++;
  return map;
});

function selectPlugin(p: PluginMeta) {
  selected.value = p;
  example.value = p.example;
  schedulePreview(true);
}

// ---------- 实时预览（防抖 500ms，走后端 goldmark + shortcode 管线） ----------

const previewHtml = ref("");
const rendering = ref(false);
const previewError = ref("");
let previewTimer: ReturnType<typeof setTimeout> | undefined;

async function refreshPreview() {
  rendering.value = true;
  previewError.value = "";
  try {
    const { data } = await previewApi.render(example.value);
    previewHtml.value = data.html;
  } catch {
    previewError.value = "预览渲染失败";
  } finally {
    rendering.value = false;
  }
}

function schedulePreview(immediate = false) {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, immediate ? 0 : 500);
}

watch(example, () => schedulePreview());
// 首次进入立即渲染一次
refreshPreview();

onBeforeUnmount(() => clearTimeout(previewTimer));

// iframe srcdoc：注入前台样式 + 渲染 HTML，data-theme 控制亮/暗。
const srcdoc = computed(() => {
  const t = previewTheme.value;
  return `<!DOCTYPE html>
<html data-theme="${t}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>
  html,body{margin:0;padding:0;}
  body{padding:24px;background:${t === "dark" ? "#1c1c1e" : "#fff"};}
  .md-text{max-width:760px;margin:0 auto;}
</style>
</head>
<body><article class="md-text content">${previewHtml.value}</article></body>
</html>`;
});

// ---------- 复制 ----------

const copied = ref<"syntax" | "example" | "">("");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

async function copyText(kind: "syntax" | "example", text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
  }
  copied.value = kind;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => (copied.value = ""), 1500);
}

onBeforeUnmount(() => clearTimeout(copiedTimer));
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- 页头 -->
    <header class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 shrink-0">
      <h2 class="text-lg font-semibold text-gray-800 dark:text-gray-100 flex items-center gap-2">
        <Icon icon="ri:puzzle-line" class="text-xl text-indigo-500" />
        插件库
      </h2>
      <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
        {{ plugins.length }} 个标签插件 · 语法速查 + 实时渲染预览，与前台样式完全一致
      </p>
    </header>

    <div class="flex-1 flex overflow-hidden">
      <!-- 左侧：分类 + 搜索 + 列表 -->
      <aside class="w-64 shrink-0 border-r border-gray-200 dark:border-gray-700 flex flex-col bg-white dark:bg-gray-800">
        <div class="p-3 space-y-2 border-b border-gray-100 dark:border-gray-700">
          <div class="relative">
            <Icon icon="ri:search-line" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="search"
              placeholder="搜索插件..."
              class="w-full pl-8 pr-3 py-1.5 text-sm rounded-lg border border-gray-200 focus:outline-none focus:ring-1 focus:ring-indigo-400 dark:bg-gray-700 dark:border-gray-600 dark:text-gray-100"
            />
          </div>
          <div class="flex flex-wrap gap-1">
            <button
              @click="activeCategory = '全部'"
              :class="[
                'px-2 py-0.5 text-xs rounded-full transition-colors',
                activeCategory === '全部'
                  ? 'bg-indigo-600 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300',
              ]"
            >全部 {{ categoryCount['全部'] }}</button>
            <button
              v-for="c in PLUGIN_CATEGORIES"
              :key="c"
              @click="activeCategory = c"
              :class="[
                'px-2 py-0.5 text-xs rounded-full transition-colors',
                activeCategory === c
                  ? 'bg-indigo-600 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300',
              ]"
            >{{ c }} {{ categoryCount[c] }}</button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto py-1">
          <button
            v-for="p in filtered"
            :key="p.name"
            @click="selectPlugin(p)"
            :class="[
              'w-full text-left px-4 py-2 flex items-center gap-2 transition-colors',
              selected.name === p.name
                ? 'bg-indigo-50 dark:bg-indigo-500/15'
                : 'hover:bg-gray-50 dark:hover:bg-gray-700/50',
            ]"
          >
            <span
              :class="[
                'text-[10px] px-1 py-px rounded font-mono shrink-0',
                p.type === 'block'
                  ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400'
                  : 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400',
              ]"
            >{{ p.type === "block" ? "块" : "行" }}</span>
            <span
              :class="[
                'text-sm truncate',
                selected.name === p.name
                  ? 'text-indigo-700 font-medium dark:text-indigo-400'
                  : 'text-gray-700 dark:text-gray-300',
              ]"
            >{{ p.title }}</span>
            <span class="ml-auto text-[11px] text-gray-400 font-mono shrink-0">{{ p.name }}</span>
          </button>
          <p v-if="filtered.length === 0" class="px-4 py-6 text-center text-sm text-gray-400">
            无匹配插件
          </p>
        </div>
      </aside>

      <!-- 右侧：详情 + 预览 -->
      <section class="flex-1 overflow-y-auto p-6 space-y-5">
        <!-- 标题区 -->
        <div>
          <div class="flex items-center gap-2 flex-wrap">
            <h3 class="text-xl font-semibold text-gray-800 dark:text-gray-100">{{ selected.title }}</h3>
            <code class="text-xs px-1.5 py-0.5 rounded bg-gray-100 text-gray-600 font-mono dark:bg-gray-700 dark:text-gray-300">{% {{ selected.name }} %}</code>
            <span class="text-[11px] px-1.5 py-0.5 rounded-full bg-indigo-50 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400">{{ selected.category }}</span>
            <span
              :class="[
                'text-[11px] px-1.5 py-0.5 rounded-full',
                selected.type === 'block'
                  ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400'
                  : 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400',
              ]"
            >{{ selected.type === "block" ? "块级" : "行内" }}</span>
          </div>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{{ selected.desc }}</p>
        </div>

        <!-- 语法 -->
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-200 dark:bg-gray-700/40 dark:border-gray-700">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">语法</span>
            <button
              @click="copyText('syntax', selected.signature)"
              class="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
            >
              <Icon :icon="copied === 'syntax' ? 'ri:check-line' : 'ri:file-copy-line'" />
              {{ copied === "syntax" ? "已复制" : "复制语法" }}
            </button>
          </div>
          <pre class="px-4 py-3 text-sm font-mono text-gray-700 overflow-x-auto dark:text-gray-200">{{ selected.signature }}</pre>
        </div>

        <!-- 参数表 -->
        <div v-if="selected.params.length" class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="px-4 py-2 bg-gray-50 border-b border-gray-200 dark:bg-gray-700/40 dark:border-gray-700">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">参数</span>
          </div>
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-400 border-b border-gray-100 dark:border-gray-700">
                <th class="px-4 py-2 font-medium w-32">参数</th>
                <th class="px-4 py-2 font-medium">说明</th>
                <th class="px-4 py-2 font-medium w-28">默认值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="param in selected.params" :key="param.name" class="border-b border-gray-50 last:border-0 dark:border-gray-700/50">
                <td class="px-4 py-2"><code class="text-xs font-mono text-indigo-600 dark:text-indigo-400">{{ param.name }}</code></td>
                <td class="px-4 py-2 text-gray-600 dark:text-gray-300">{{ param.desc }}</td>
                <td class="px-4 py-2 text-gray-400 font-mono text-xs">{{ param.default }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 示例编辑 + 复制 -->
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-200 dark:bg-gray-700/40 dark:border-gray-700">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">示例（可编辑，实时预览）</span>
            <button
              @click="copyText('example', example)"
              class="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-700 dark:text-indigo-400"
            >
              <Icon :icon="copied === 'example' ? 'ri:check-line' : 'ri:file-copy-line'" />
              {{ copied === "example" ? "已复制" : "复制示例" }}
            </button>
          </div>
          <textarea
            v-model="example"
            spellcheck="false"
            class="w-full px-4 py-3 text-sm font-mono leading-relaxed bg-transparent focus:outline-none resize-y min-h-[120px] text-gray-700 dark:text-gray-200"
          ></textarea>
        </div>

        <!-- 预览 -->
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-200 dark:bg-gray-700/40 dark:border-gray-700">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400 flex items-center gap-1.5">
              渲染预览
              <span v-if="rendering" class="text-[11px] text-gray-400">渲染中...</span>
            </span>
            <div class="flex items-center gap-1 p-0.5 bg-gray-200/60 rounded-lg dark:bg-gray-600/40">
              <button
                @click="previewTheme = 'light'"
                :title="'亮色预览'"
                :class="[
                  'p-1 rounded-md text-sm transition-colors',
                  previewTheme === 'light' ? 'bg-white text-amber-500 shadow-sm dark:bg-gray-800' : 'text-gray-500 dark:text-gray-400',
                ]"
              ><Icon icon="ri:sun-line" /></button>
              <button
                @click="previewTheme = 'dark'"
                :title="'暗色预览'"
                :class="[
                  'p-1 rounded-md text-sm transition-colors',
                  previewTheme === 'dark' ? 'bg-white text-indigo-500 shadow-sm dark:bg-gray-800' : 'text-gray-500 dark:text-gray-400',
                ]"
              ><Icon icon="ri:moon-line" /></button>
            </div>
          </div>
          <div class="relative bg-gray-100 dark:bg-gray-900">
            <p v-if="previewError" class="px-4 py-6 text-center text-sm text-red-500">{{ previewError }}</p>
            <iframe
              v-else
              :srcdoc="srcdoc"
              class="w-full border-0 bg-white"
              style="height: 420px"
              sandbox="allow-same-origin allow-scripts"
            ></iframe>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
