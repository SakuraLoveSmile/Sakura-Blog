<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { previewApi, settingsApi, postApi, aiApi } from "@/api";
import PluginParamForm from "@/modules/plugins/PluginParamForm.vue";
import { VButton, Toast } from "@/components/ui";
import {
  plugins,
  PLUGIN_CATEGORIES,
  type PluginMeta,
  type PluginCategory,
} from "@/modules/plugins/plugins-data";

// 编辑器右侧辅助面板（对照 Halo）：大纲 / 详情 两个 Tab。
// 大纲：解析 Markdown 标题可点击跳转；详情：合并原右栏全部设置字段与标签插件参数面板。
interface PostForm {
  slug: string;
  title: string;
  excerpt: string;
  cover: string;
  category: string;
  topic: string;
  tags: string;
  content_md: string;
  status: string;
  pinned: boolean;
  password: string;
  publish_at: string;
  meta_description: string;
  meta_keywords: string;
  author: string;
}

const props = defineProps<{
  form: PostForm;
  categories: string[];
  tags: string[];
  aiSummary: string; // Spec 53：AI 摘要由 PostEditor 持有（与 AiGenerateModal 回填共用一份状态）
}>();

const emit = defineEmits<{
  (e: "jump", line: number, index: number): void;
  (e: "insert", code: string): void;
  (e: "openCoverPicker"): void;
  (e: "slugEdited"): void;
  (e: "update:aiSummary", value: string): void;
}>();

const activeTab = ref<"outline" | "detail">("outline");

// ---------- 作者可选项（Spec 38 Wave 5）：authors 为空 map 时不渲染控件 ----------
const authorOptions = ref<Record<string, { name: string }>>({});

async function loadAuthors() {
  try {
    const { data } = await settingsApi.all();
    if (data.authors && typeof data.authors === "object") {
      authorOptions.value = data.authors;
    }
  } catch {
    /* 静默：作者列表缺失不阻塞编辑 */
  }
}

onMounted(loadAuthors);

// ---------- 大纲：解析 ATX 标题（跳过代码围栏），点击跳转到对应行 ----------

interface OutlineItem {
  level: number;
  text: string;
  line: number;
}

const outline = computed<OutlineItem[]>(() => {
  const items: OutlineItem[] = [];
  let inFence = false;
  const lines = (props.form.content_md || "").split("\n");
  lines.forEach((raw, i) => {
    const line = raw.trim();
    if (/^(```|~~~)/.test(line)) {
      inFence = !inFence;
      return;
    }
    if (inFence) return;
    const m = /^(#{1,6})\s+(.+?)\s*#*\s*$/.exec(line);
    if (m) items.push({ level: m[1].length, text: m[2], line: i });
  });
  return items;
});

// 顶层标题级别（缩进以此为基准，如全文最高是 h2 则 h2 不缩进）
const minLevel = computed(() =>
  outline.value.length ? Math.min(...outline.value.map((i) => i.level)) : 1
);

// ---------- 详情：分类/标签 chips ----------

function pickCategory(name: string) {
  props.form.category = name;
}

// 点标签 chip：按逗号拆分去重后追加
function pickTag(name: string) {
  const list = props.form.tags
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (!list.includes(name)) list.push(name);
  props.form.tags = list.join(",");
}

/** 随机封面图：picsum seed 模式，保存后 URL 固定图片不再变化 */
function fetchRandomCover() {
  const seed = Math.random().toString(36).slice(2, 10);
  props.form.cover = `https://picsum.photos/seed/${seed}/1200/675`;
}

// ---------- 标签插件参数面板（自 PostEditor 迁入，Spec 29） ----------

const pluginCat = ref<PluginCategory | "全部">("全部");
const pluginList = computed(() =>
  pluginCat.value === "全部"
    ? plugins
    : plugins.filter((p) => p.category === pluginCat.value)
);
const selectedPlugin = ref<PluginMeta | null>(null);
const pluginCode = ref("");
const pluginPreviewHtml = ref("");
const pluginRendering = ref(false);
let pluginTimer: ReturnType<typeof setTimeout> | undefined;

function selectPlugin(p: PluginMeta) {
  selectedPlugin.value = p;
  // 有 form schema：由 PluginParamForm 初始化后立即产出 shortcode；否则回退示例文本
  pluginCode.value = p.form ? "" : p.example;
  schedulePluginPreview(true);
}

function onPluginCode(code: string) {
  pluginCode.value = code;
  schedulePluginPreview();
}

async function refreshPluginPreview() {
  if (!pluginCode.value) {
    pluginPreviewHtml.value = "";
    return;
  }
  pluginRendering.value = true;
  try {
    const { data } = await previewApi.render(pluginCode.value);
    pluginPreviewHtml.value = data.html;
  } catch {
    pluginPreviewHtml.value = '<p style="color:#ef4444">预览渲染失败</p>';
  } finally {
    pluginRendering.value = false;
  }
}

function schedulePluginPreview(immediate = false) {
  clearTimeout(pluginTimer);
  pluginTimer = setTimeout(refreshPluginPreview, immediate ? 0 : 400);
}

const pluginSrcdoc = computed(() => {
  return `<!DOCTYPE html>
<html data-theme="dark">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>html,body{margin:0;padding:0;}body{padding:16px;background:#1c1c1e;}.md-text{max-width:100%;}</style>
</head>
<body><article class="md-text content">${pluginPreviewHtml.value}</article><scr` +
"ipt>\n" +
"document.querySelectorAll('.tabs .nav-tabs .tab').forEach(function(el) {\n" +
"  el.addEventListener('click', function(e) {\n" +
"    e.preventDefault();\n" +
"    if (el.classList.contains('active')) return;\n" +
"    Array.prototype.forEach.call(el.parentNode.children, function(t) {\n" +
"      t.classList.toggle('active', t === el);\n" +
"    });\n" +
"    var a = el.querySelector('a');\n" +
"    if (!a) return;\n" +
"    var target = document.getElementById(a.getAttribute('href').replace('#', ''));\n" +
"    if (!target) return;\n" +
"    Array.prototype.forEach.call(target.parentNode.children, function(t) {\n" +
"      t.classList.toggle('active', t === target);\n" +
"    });\n" +
"  });\n" +
"});\n" +
"<\/script></body>\n" +
"</html>";
});

function insertPlugin() {
  if (!pluginCode.value) return;
  emit("insert", "\n" + pluginCode.value + "\n");
}

onBeforeUnmount(() => clearTimeout(pluginTimer));

// ---------- AI 摘要（Spec 53）：textarea 手动编辑 + AI 生成，保存走独立端点 ----------
const aiSaving = ref(false);
const aiGenerating = ref(false);

async function saveAiSummary() {
  if (!props.form.slug) {
    Toast.warning("请先保存文章（生成 slug）后再保存摘要");
    return;
  }
  aiSaving.value = true;
  try {
    await postApi.updateAISummary(props.form.slug, props.aiSummary.trim());
    Toast.success("AI 摘要已保存");
  } catch {
    Toast.error("AI 摘要保存失败");
  } finally {
    aiSaving.value = false;
  }
}

// 清空：直接传空串落库（不依赖 props 回写时序）
async function clearAiSummary() {
  if (!props.form.slug) {
    emit("update:aiSummary", "");
    return;
  }
  aiSaving.value = true;
  try {
    await postApi.updateAISummary(props.form.slug, "");
    emit("update:aiSummary", "");
    Toast.success("AI 摘要已清空");
  } catch {
    Toast.error("AI 摘要清空失败");
  } finally {
    aiSaving.value = false;
  }
}

async function generateAiSummary() {
  if (!props.form.title && !props.form.content_md) {
    Toast.warning("标题与正文均为空，无法生成");
    return;
  }
  aiGenerating.value = true;
  try {
    const { data } = await aiApi.generate({
      fields: ["summary"],
      title: props.form.title,
      content: props.form.content_md,
      categories: [],
      tags: [],
    });
    if (data.summary) {
      emit("update:aiSummary", data.summary);
      // 已有 slug 时直接落库，免去再点一次保存
      if (props.form.slug) {
        await postApi.updateAISummary(props.form.slug, data.summary);
        Toast.success("AI 摘要已生成并保存");
      } else {
        Toast.success("AI 摘要已生成，保存文章后可落库");
      }
    } else {
      Toast.error("生成结果为空，请重试");
    }
  } catch (err: unknown) {
    const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
    Toast.error(msg || "AI 生成失败，请检查 设置 → AI 配置");
  } finally {
    aiGenerating.value = false;
  }
}

const fieldCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const labelCls = "block text-xs text-gray-500 mb-1 dark:text-gray-400";
const chipCls =
  "px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600 hover:bg-primary/15 hover:text-secondary transition-colors dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-primary/20 dark:hover:text-primary";
</script>

<template>
  <div class="flex flex-col rounded-xl border border-gray-200 bg-white lg:max-h-[calc(100vh-2rem)] dark:border-gray-600 dark:bg-gray-800">
    <!-- Tab 切换（对照 Halo 分段控件） -->
    <div class="p-2">
      <div class="grid grid-cols-2 gap-0.5 rounded-lg bg-gray-100 p-0.5 dark:bg-gray-700/50">
        <button
          v-for="tab in [
            { key: 'outline', label: '大纲' },
            { key: 'detail', label: '详情' },
          ]"
          :key="tab.key"
          type="button"
          :class="[
            'rounded-md py-1.5 text-sm transition-colors',
            activeTab === tab.key
              ? 'bg-white font-medium text-gray-900 shadow-sm dark:bg-gray-800 dark:text-gray-100'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
          ]"
          @click="activeTab = tab.key as 'outline' | 'detail'"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <!-- 大纲 -->
    <div v-if="activeTab === 'outline'" class="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
      <p v-if="outline.length === 0" class="py-10 text-center text-sm text-gray-400 dark:text-gray-500">暂无大纲</p>
      <ul v-else class="space-y-0.5">
        <li v-for="(item, i) in outline" :key="i">
          <button
            type="button"
            class="block w-full truncate rounded px-2 py-1.5 text-left text-sm text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-100"
            :style="{ paddingLeft: 8 + (item.level - minLevel) * 14 + 'px' }"
            :title="item.text"
            @click="emit('jump', item.line, i)"
          >
            {{ item.text }}
          </button>
        </li>
      </ul>
    </div>

    <!-- 详情（原右栏设置合并） -->
    <div v-else class="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 pb-4">
      <!-- 发布设置 -->
      <section class="space-y-3">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
          <Icon icon="ri:settings-3-line" />发布设置
        </h4>
        <div>
          <label :class="labelCls">别名 (Slug)</label>
          <input v-model="form.slug" :class="fieldCls" @input="emit('slugEdited')" />
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">用于文章 URL，留空保存时从标题自动生成</p>
        </div>
        <!-- 作者选择（Spec 38 Wave 5）：authors 为空时不渲染 -->
        <div v-if="Object.keys(authorOptions).length">
          <label :class="labelCls">作者</label>
          <select v-model="form.author" :class="fieldCls">
            <option value="">默认（站长）</option>
            <option v-for="(info, id) in authorOptions" :key="id" :value="id">{{ info.name }}</option>
          </select>
        </div>
        <div class="flex items-center gap-2">
          <input id="pinned" v-model="form.pinned" type="checkbox" class="rounded" />
          <label for="pinned" class="text-sm text-gray-600 dark:text-gray-400">置顶</label>
        </div>
        <div>
          <label :class="labelCls">定时发布</label>
          <input v-model="form.publish_at" type="datetime-local" :class="fieldCls" />
        </div>
      </section>

      <!-- 分类与标签 -->
      <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-gray-700">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
          <Icon icon="ri:price-tag-3-line" />分类与标签
        </h4>
        <div>
          <label :class="labelCls">分类</label>
          <input v-model="form.category" :class="fieldCls" />
          <div v-if="categories.length" class="mt-1.5 flex flex-wrap gap-1">
            <button
              v-for="c in categories"
              :key="c"
              type="button"
              :class="[chipCls, form.category === c && '!bg-primary/20 !text-secondary dark:!bg-primary/25 dark:!text-primary']"
              @click="pickCategory(c)"
            >{{ c }}</button>
          </div>
        </div>
        <div>
          <label :class="labelCls">专栏</label>
          <input v-model="form.topic" :class="fieldCls" />
        </div>
        <div>
          <label :class="labelCls">标签（逗号分隔）</label>
          <input v-model="form.tags" :class="fieldCls" />
          <div v-if="tags.length" class="mt-1.5 flex flex-wrap gap-1">
            <button v-for="t in tags" :key="t" type="button" :class="chipCls" @click="pickTag(t)">{{ t }}</button>
          </div>
        </div>
      </section>

      <!-- 其他 -->
      <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-gray-700">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
          <Icon icon="ri:more-line" />其他
        </h4>
        <div>
          <label :class="labelCls">摘要</label>
          <textarea v-model="form.excerpt" rows="3" :class="fieldCls + ' resize-y'"></textarea>
        </div>
        <!-- AI 文章摘要（Spec 53）：前台文章顶部导读；手动编辑或 AI 生成，独立端点保存 -->
        <div>
          <label :class="labelCls">AI 摘要（前台导读）</label>
          <textarea
            :value="aiSummary"
            rows="4"
            :class="fieldCls + ' resize-y'"
            placeholder="后台预生成的导读摘要，前台文章顶部展示；可手动编辑或用 AI 生成"
            @input="emit('update:aiSummary', ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
          <div class="mt-1.5 flex items-center gap-1.5">
            <button
              type="button"
              class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-600 hover:bg-gray-100 disabled:opacity-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
              :disabled="aiGenerating"
              @click="generateAiSummary"
            >{{ aiGenerating ? "生成中…" : "AI 生成" }}</button>
            <button
              type="button"
              class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-600 hover:bg-gray-100 disabled:opacity-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
              :disabled="aiSaving"
              @click="saveAiSummary"
            >{{ aiSaving ? "保存中…" : "保存摘要" }}</button>
            <button
              v-if="aiSummary"
              type="button"
              class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-400 hover:bg-gray-100 dark:border-gray-600 dark:hover:bg-gray-700"
              :disabled="aiSaving"
              @click="clearAiSummary"
            >清空</button>
          </div>
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">AI 未配置时生成不可用（设置 → AI）；读者也可在前台用自己的 API 临时生成</p>
        </div>
        <div>
          <label :class="labelCls">封面 URL</label>
          <div class="flex gap-1.5">
            <input v-model="form.cover" :class="fieldCls + ' flex-1 min-w-0'" />
            <button
              type="button"
              title="从图床选择"
              class="shrink-0 rounded-lg border border-gray-200 px-2.5 py-2 text-xs text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
              @click="emit('openCoverPicker')"
            >选择</button>
            <button
              type="button"
              title="随机封面图（picsum.photos）"
              class="shrink-0 rounded-lg border border-gray-200 px-2.5 py-2 text-xs text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
              @click="fetchRandomCover"
            >随机</button>
          </div>
        </div>
        <div>
          <label :class="labelCls">阅读密码</label>
          <input v-model="form.password" type="password" :class="fieldCls" />
        </div>
        <!-- 文章级 SEO（Spec 42）：留空回退站点默认/摘要 -->
        <div>
          <label :class="labelCls">Meta Description</label>
          <textarea v-model="form.meta_description" rows="3" :class="fieldCls + ' resize-y'"></textarea>
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">留空时依次回退：站点默认 description → 文章摘要</p>
        </div>
        <div>
          <label :class="labelCls">Meta Keywords（逗号分隔）</label>
          <input v-model="form.meta_keywords" :class="fieldCls" />
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">留空时使用站点默认 keywords</p>
        </div>
      </section>

      <!-- 标签插件参数面板（精细调参用；快捷插入走工具栏「+」） -->
      <section class="space-y-3 border-t border-gray-100 pt-4 dark:border-gray-700">
        <h4 class="flex items-center gap-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
          <Icon icon="ri:puzzle-line" />标签插件
        </h4>
        <div class="flex flex-wrap gap-1">
          <button
            type="button"
            :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === '全部' ? 'bg-secondary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
            @click="pluginCat = '全部'"
          >全部</button>
          <button
            v-for="c in PLUGIN_CATEGORIES"
            :key="c"
            type="button"
            :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === c ? 'bg-secondary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
            @click="pluginCat = c"
          >{{ c }}</button>
        </div>
        <select
          :value="selectedPlugin?.name"
          :class="fieldCls"
          @change="selectPlugin(plugins.find((p) => p.name === ($event.target as HTMLSelectElement).value)!)"
        >
          <option :value="undefined" disabled>选择插件…</option>
          <option v-for="p in pluginList" :key="p.name" :value="p.name">{{ p.title }}</option>
        </select>

        <div v-if="selectedPlugin" class="space-y-3">
          <PluginParamForm
            v-if="selectedPlugin.form"
            :plugin="selectedPlugin"
            :model-value="pluginCode"
            @update:model-value="onPluginCode"
          />
          <div v-else>
            <label :class="labelCls">示例（可编辑）</label>
            <textarea
              v-model="pluginCode"
              rows="4"
              spellcheck="false"
              :class="fieldCls + ' resize-y font-mono text-xs leading-relaxed'"
              @input="schedulePluginPreview()"
            ></textarea>
          </div>

          <div>
            <label :class="labelCls">预览<span v-if="pluginRendering" class="ml-1 text-gray-400">渲染中…</span></label>
            <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
              <iframe
                v-if="pluginPreviewHtml"
                :srcdoc="pluginSrcdoc"
                class="w-full border-0"
                style="min-height: 200px; max-height: 400px; resize: vertical; overflow: auto;"
                sandbox="allow-same-origin allow-scripts"
              ></iframe>
              <p v-else class="px-3 py-4 text-center text-xs text-gray-400">填写参数后预览</p>
            </div>
          </div>

          <VButton type="secondary" block :disabled="!pluginCode" @click="insertPlugin">插入到正文</VButton>
        </div>
        <p v-else class="text-xs text-gray-400 dark:text-gray-500">选择插件后可编辑参数并插入正文。</p>
      </section>
    </div>
  </div>
</template>
