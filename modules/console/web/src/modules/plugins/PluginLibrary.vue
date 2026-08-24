<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader } from "@/components/ui";
import {
  PLUGIN_CATEGORIES,
  previewInfoFor,
  type PluginMeta,
  type PluginCategory,
  type PluginPreviewInfo,
} from "./plugin-types";
import { CAPABILITY_LABELS, FIXTURE_LABELS } from "./preview/resources";
import { usePluginPreview } from "./preview/usePluginPreview";
import PluginParamForm from "./PluginParamForm.vue";
import AssetGallery from "./AssetGallery.vue";
import WidgetLibraryView from "./WidgetLibraryView.vue";
import { loadPluginCatalog, runtimePlugins } from "./usePluginCatalog";
import { WIDGET_META } from "../settings/widget-meta";

// Console 插件库（Spec 23 + Spec 63 + Spec 64）：标签插件的分类速查 + 实时渲染预览 + 一键复制；
// 「素材速查」视图扩展：图标画廊（icons.yml 全量 key）与表情画廊（可用表情名清单）。
// 预览复用后端 POST /api/v1/preview/markdown（component-library Fixture 模式）渲染管线，
// 共享预览运行时（preview/）构建 iframe：注入前台样式与按能力脚本、固定暗色主题、
// 防抖 + 请求序列号、推荐高度 + 内容高度回传、降级提示。本组件只负责选择插件、编辑示例与传入预览能力。

const viewMode = ref<"plugins" | "assets" | "widgets">("plugins");
const search = ref("");
const activeCategory = ref<PluginCategory | "全部">("全部");
const loadingPlugin: PluginMeta = {
  name: "__loading__",
  title: "插件目录",
  category: "布局容器",
  type: "inline",
  desc: "正在加载后端 shortcode catalog。",
  signature: "",
  params: [],
  example: "",
};
const selected = ref<PluginMeta>(loadingPlugin);
const example = ref("");

watch(runtimePlugins, (next) => {
  const refreshed = next.find((plugin) => plugin.name === selected.value.name);
  if (refreshed) {
    selected.value = refreshed;
  } else if (next[0] && selected.value.name === loadingPlugin.name) {
    selected.value = next[0];
    example.value = next[0].example;
  }
});

const filtered = computed(() => {
  const kw = search.value.trim().toLowerCase();
  return runtimePlugins.value.filter((p) => {
    if (activeCategory.value !== "全部" && p.category !== activeCategory.value) return false;
    if (!kw) return true;
    return p.name.toLowerCase().includes(kw) || p.title.toLowerCase().includes(kw) || p.desc.toLowerCase().includes(kw);
  });
});

const categoryCount = computed(() => {
  const map: Record<string, number> = { 全部: runtimePlugins.value.length };
  for (const c of PLUGIN_CATEGORIES) map[c] = 0;
  for (const p of runtimePlugins.value) map[p.category]++;
  return map;
});

function selectPlugin(p: PluginMeta) {
  selected.value = p;
  example.value = p.example;
  preview.resetHeight(); // 选中标签先应用推荐高度，再按 iframe 实际内容高度回传调整
  preview.scheduleRender(example.value, true);
}

// 结构化参数表单产出 shortcode（Spec 29）：同步驱动示例/预览/复制
function onFormCode(code: string) {
  example.value = code;
}

// ---------- 实时预览（Spec 63：共享预览运行时） ----------
// 防抖 500ms + 请求序列号（旧请求不覆盖新预览）；请求携带 preview_profile=component-library
// 让后端进入本地 Fixture 模式（GitHub/友链确定性数据、rating/vote 隔离 ID 不写库）。

const previewFrame = ref<HTMLIFrameElement | null>(null);
const preview = usePluginPreview(() => previewInfoFor(selected.value));

const selectedPreview = computed<PluginPreviewInfo>(() => previewInfoFor(selected.value));
const capabilityLabel = computed(() => CAPABILITY_LABELS[selectedPreview.value.capability]);
const fixtureLabel = computed(() =>
  selectedPreview.value.fixture ? FIXTURE_LABELS[selectedPreview.value.fixture] : "",
);

watch(example, () => {
  if (example.value) preview.scheduleRender(example.value);
});

function onPreviewMessage(event: MessageEvent) {
  preview.onMessage(event, previewFrame.value);
}
onMounted(() => {
  window.addEventListener("message", onPreviewMessage);
  void loadPluginCatalog().then((next) => {
    if (next[0] && selected.value.name === loadingPlugin.name) {
      selected.value = next[0];
      example.value = next[0].example;
      preview.resetHeight();
      preview.scheduleRender(example.value, true);
    }
  });
});
onBeforeUnmount(() => window.removeEventListener("message", onPreviewMessage));

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
    <VPageHeader title="插件库">
      <template #icon>
        <Icon icon="ri:puzzle-line" class="mr-2 self-center text-xl" />
      </template>
      <template #subtitle>
        {{
          viewMode === "plugins"
            ? `${runtimePlugins.length} 个标签插件 · 语法速查 + 实时渲染预览（本地 Fixture 数据，不依赖公网）`
            : viewMode === "assets"
              ? "图标与表情素材速查 · 点击复制 shortcode，图标 key 可用于分类/标签图标配置"
              : `${WIDGET_META.length} 种侧边栏组件 · 编辑参数保存为方案，可在设置 → 组件编排中直接选用`
        }}
      </template>
    </VPageHeader>

    <!-- 视图切换：插件 / 素材速查 / 侧边栏组件 -->
    <div class="px-5 pt-4 pb-4 md:px-6 md:pb-5">
      <div class="apple-segmented" style="max-width: 420px">
        <button type="button" :class="{ active: viewMode === 'plugins' }" @click="viewMode = 'plugins'">
          标签插件
        </button>
        <button type="button" :class="{ active: viewMode === 'assets' }" @click="viewMode = 'assets'">素材速查</button>
        <button type="button" :class="{ active: viewMode === 'widgets' }" @click="viewMode = 'widgets'">
          侧边栏组件
        </button>
      </div>
    </div>

    <!-- 素材速查视图 -->
    <div v-if="viewMode === 'assets'" class="flex-1 overflow-y-auto p-5 md:p-6">
      <AssetGallery />
    </div>

    <!-- 侧边栏组件视图（Spec 58：方案保存与套用） -->
    <div v-else-if="viewMode === 'widgets'" class="flex-1 overflow-hidden">
      <WidgetLibraryView />
    </div>

    <div v-else-if="viewMode === 'plugins'" class="flex-1 flex flex-col overflow-hidden md:flex-row">
      <!-- 左侧：分类 + 搜索 + 列表 -->
      <aside class="w-full shrink-0 border-b border-border flex flex-col bg-card md:w-64 md:border-r md:border-b-0">
        <div class="p-3 space-y-2 border-b border-border">
          <div class="relative">
            <Icon icon="ri:search-line" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-quaternary" />
            <input
              v-model="search"
              placeholder="搜索插件..."
              class="w-full pl-8 pr-3 py-1.5 text-sm rounded-lg border border-border focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <div class="flex flex-wrap gap-1">
            <button
              @click="activeCategory = '全部'"
              :class="[
                'px-2 py-0.5 text-xs rounded-full transition-colors',
                activeCategory === '全部'
                  ? 'bg-secondary text-foreground'
                  : 'bg-secondary text-text-tertiary hover:bg-accent ',
              ]"
            >
              全部 {{ categoryCount["全部"] }}
            </button>
            <button
              v-for="c in PLUGIN_CATEGORIES"
              :key="c"
              @click="activeCategory = c"
              :class="[
                'px-2 py-0.5 text-xs rounded-full transition-colors',
                activeCategory === c
                  ? 'bg-secondary text-foreground'
                  : 'bg-secondary text-text-tertiary hover:bg-accent ',
              ]"
            >
              {{ c }} {{ categoryCount[c] }}
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto py-1 max-h-48 md:max-h-none">
          <button
            v-for="p in filtered"
            :key="p.name"
            @click="selectPlugin(p)"
            :class="[
              'w-full text-left px-4 py-2 flex items-center gap-2 transition-colors',
              selected.name === p.name ? 'bg-primary/10' : 'hover:bg-accent',
            ]"
          >
            <span
              :class="[
                'text-[10px] px-1 py-px rounded font-mono shrink-0',
                p.type === 'block' ? 'bg-state-warning/15 text-state-warning' : 'bg-primary/15 text-primary',
              ]"
              >{{ p.type === "block" ? "块" : "行" }}</span
            >
            <span
              :class="[
                'text-sm truncate',
                selected.name === p.name ? 'font-medium text-primary' : 'text-text-secondary ',
              ]"
              >{{ p.title }}</span
            >
            <span class="ml-auto text-[11px] text-text-quaternary font-mono shrink-0">{{ p.name }}</span>
          </button>
          <p v-if="filtered.length === 0" class="px-4 py-6 text-center text-sm text-text-quaternary">无匹配插件</p>
        </div>
      </aside>

      <!-- 右侧：详情 + 预览 -->
      <section class="min-h-0 flex-1 overflow-y-auto p-6 space-y-5">
        <!-- 标题区 -->
        <div>
          <div class="flex items-center gap-2 flex-wrap">
            <h3 class="text-xl font-semibold text-foreground">{{ selected.title }}</h3>
            <code class="text-xs px-1.5 py-0.5 rounded bg-secondary text-text-tertiary font-mono"
              >{% {{ selected.name }} %}</code
            >
            <span class="text-[11px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary">{{
              selected.category
            }}</span>
            <span
              :class="[
                'text-[11px] px-1.5 py-0.5 rounded-full',
                selected.type === 'block' ? 'bg-state-warning/15 text-state-warning' : 'bg-primary/15 text-primary',
              ]"
              >{{ selected.type === "block" ? "块级" : "行内" }}</span
            >
          </div>
          <p class="text-sm text-text-tertiary mt-1">{{ selected.desc }}</p>
        </div>

        <!-- 语法 -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">语法</span>
            <button
              @click="copyText('syntax', selected.signature)"
              class="flex items-center gap-1 text-xs text-primary"
            >
              <Icon :icon="copied === 'syntax' ? 'ri:check-line' : 'ri:file-copy-line'" />
              {{ copied === "syntax" ? "已复制" : "复制语法" }}
            </button>
          </div>
          <pre class="px-4 py-3 text-sm font-mono text-text-secondary overflow-x-auto">{{ selected.signature }}</pre>
        </div>

        <!-- 参数表 -->
        <div v-if="selected.params.length" class="rounded-xl border border-border overflow-hidden">
          <div class="px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">参数</span>
          </div>
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-text-quaternary border-b border-border">
                <th class="px-4 py-2 font-medium w-32">参数</th>
                <th class="px-4 py-2 font-medium">说明</th>
                <th class="px-4 py-2 font-medium w-28">默认值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="param in selected.params" :key="param.name" class="border-b border-border last:border-0 /50">
                <td class="px-4 py-2">
                  <code class="text-xs font-mono text-text-secondary">{{ param.name }}</code>
                </td>
                <td class="px-4 py-2 text-text-tertiary">{{ param.desc }}</td>
                <td class="px-4 py-2 text-text-quaternary font-mono text-xs">{{ param.default }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 结构化参数表单（Spec 29，有 form schema 的插件） -->
        <div v-if="selected.form" class="rounded-xl border border-border overflow-hidden">
          <div class="px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">参数设置（自动拼装 shortcode）</span>
          </div>
          <div class="p-4">
            <PluginParamForm :plugin="selected" :model-value="example" @update:model-value="onFormCode" />
          </div>
        </div>

        <!-- 示例编辑 + 复制 -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">示例（可编辑，实时预览）</span>
            <button @click="copyText('example', example)" class="flex items-center gap-1 text-xs text-primary">
              <Icon :icon="copied === 'example' ? 'ri:check-line' : 'ri:file-copy-line'" />
              {{ copied === "example" ? "已复制" : "复制示例" }}
            </button>
          </div>
          <textarea
            v-model="example"
            spellcheck="false"
            class="w-full px-4 py-3 text-sm font-mono leading-relaxed bg-transparent focus:outline-none resize-y min-h-[120px] text-text-secondary"
          ></textarea>
        </div>

        <!-- 预览（Spec 63 + Spec 64：共享运行时 + 固定暗色 + Fixture 标识 + 降级态） -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary flex items-center gap-1.5">
              渲染预览
              <span v-if="preview.rendering.value" class="text-[11px] text-text-quaternary">渲染中...</span>
            </span>
            <div class="flex items-center gap-1.5">
              <span
                v-if="fixtureLabel"
                class="text-[10px] px-1.5 py-0.5 rounded-full bg-state-success/15 text-state-success inline-flex items-center gap-1"
                :title="fixtureLabel"
              >
                <Icon icon="ri:database-2-line" />数据为预览 Fixture
              </span>
              <button
                type="button"
                title="重新渲染"
                class="flex items-center gap-1 rounded-full border border-border px-2 py-1 text-xs text-text-secondary transition-colors hover:bg-accent"
                @click="preview.scheduleRender(example, true)"
              >
                <Icon icon="ri:refresh-line" />
                重新渲染
              </button>
            </div>
          </div>
          <!-- 依赖状态（能力 + 所需前台资源） -->
          <div class="flex items-center gap-2 px-4 py-1.5 border-b border-border bg-card/40 flex-wrap">
            <span class="text-[11px] text-text-quaternary">
              依赖：<span class="text-text-tertiary">{{ capabilityLabel }}</span>
              <template v-if="selectedPreview.services?.length">
                <span class="mx-1 text-text-quaternary">·</span>
                <code class="font-mono text-[10px] text-text-tertiary">{{ selectedPreview.services.join(" / ") }}</code>
              </template>
            </span>
            <span v-if="fixtureLabel" class="text-[10px] text-text-quaternary hidden md:inline">{{
              fixtureLabel
            }}</span>
          </div>
          <div class="relative bg-secondary">
            <p v-if="preview.previewError.value" class="px-4 py-6 text-center text-sm text-state-error">
              {{ preview.previewError.value }}
            </p>
            <p v-else-if="!preview.previewHtml.value" class="px-4 py-6 text-center text-sm text-text-quaternary">
              预览为空：该插件示例未产生可见输出，请检查参数是否齐全。
            </p>
            <iframe
              v-else
              ref="previewFrame"
              :srcdoc="preview.srcdoc.value"
              class="w-full border-0"
              :style="{ height: preview.frameHeight.value + 'px' }"
              sandbox="allow-same-origin allow-scripts"
            ></iframe>
          </div>
          <p
            v-if="selectedPreview.fallback && !preview.previewError.value"
            class="px-4 py-2 text-[11px] text-text-quaternary border-t border-border"
          >
            {{ selectedPreview.fallback }}
          </p>
        </div>
      </section>
    </div>
  </div>
</template>
