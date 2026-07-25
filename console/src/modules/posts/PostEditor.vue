<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { postApi, taxonomyApi, previewApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import CollapsibleSection from "@/components/CollapsibleSection.vue";
import PluginParamForm from "@/modules/plugins/PluginParamForm.vue";
import RevisionPanel from "./RevisionPanel.vue";
import {
  plugins,
  PLUGIN_CATEGORIES,
  type PluginMeta,
  type PluginCategory,
} from "@/modules/plugins/plugins-data";

const route = useRoute();
const router = useRouter();

const isEdit = computed(() => !!route.params.slug);
const slug = computed(() => route.params.slug as string);

const form = ref({
  slug: "",
  title: "",
  excerpt: "",
  cover: "",
  category: "",
  topic: "",
  tags: "",
  content_md: "",
  status: "draft",
  pinned: false,
  password: "",
  publish_at: "",
});

const loading = ref(false);
const saving = ref(false);
const error = ref("");
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
const coverPickerOpen = ref(false);
const revisionOpen = ref(false);

// Slug 自动生成：新建文章时标题变化自动推导 slug，用户手动编辑后停止覆盖（Spec Task 6）
const slugManuallyEdited = ref(false);

/** 从标题推导 URL 友好的 slug（与后端 generateSlug 对齐：保留 Unicode 字母/数字） */
function autoSlug(title: string): string {
  let s = (title || "").trim().toLowerCase();
  s = s.replace(/\s+/g, "-");
  s = s.replace(/[^\p{L}\p{N}-]/gu, "");
  s = s.replace(/-{2,}/g, "-");
  s = s.replace(/^-+|-+$/g, "");
  return s;
}

// 标题变化时自动填充 slug（仅新建模式 + 用户未手动编辑）
watch(() => form.value.title, (newTitle) => {
  if (!isEdit.value && !slugManuallyEdited.value) {
    form.value.slug = autoSlug(newTitle);
  }
});

/** 从 picsum.photos 获取随机封面图（Spec Task 4，source.unsplash.com 已停用的替代方案） */
function fetchRandomCover() {
  // picsum.photos 免费无需 API Key，时间戳参数确保每次获取不同图片
  form.value.cover = `https://picsum.photos/1200/675?random=${Date.now()}`;
}

// 当前正文字数（非空白 rune），供版本面板计算字数变化。
const currentWordCount = computed(() => {
  const s = form.value.content_md || "";
  let n = 0;
  for (const ch of s) {
    if (!/\s/.test(ch)) n++;
  }
  return n;
});

// ---------- 分类/标签可选项（Spec 29 P0-4） ----------
const categories = ref<string[]>([]);
const tags = ref<string[]>([]);

async function loadTaxonomy() {
  try {
    const [cRes, tRes] = await Promise.all([
      taxonomyApi.categories(),
      taxonomyApi.tags(),
    ]);
    categories.value = (cRes.data.items || []).map((i: any) => i.name);
    tags.value = (tRes.data.items || []).map((i: any) => i.name);
  } catch {
    /* 静默：可选项缺失不阻塞编辑 */
  }
}

function pickCategory(name: string) {
  form.value.category = name;
}

// 点标签 chip：按逗号拆分去重后追加
function pickTag(name: string) {
  const list = form.value.tags
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (!list.includes(name)) list.push(name);
  form.value.tags = list.join(",");
}

// ---------- 标签插件面板（Spec 29 P1-8） ----------
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
  const dark = document.documentElement.dataset.theme === "dark";
  return `<!DOCTYPE html>
<html data-theme="${dark ? "dark" : "light"}">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>html,body{margin:0;padding:0;}body{padding:16px;background:${dark ? "#1c1c1e" : "#fff"};}.md-text{max-width:100%;}</style>
</head>
<body><article class="md-text content">${pluginPreviewHtml.value}</article></body>
</html>`;
});

function insertPlugin() {
  if (!pluginCode.value) return;
  editorRef.value?.insertBlock("\n" + pluginCode.value + "\n");
}

onBeforeUnmount(() => clearTimeout(pluginTimer));

// ---------- 加载 / 保存 ----------
async function loadPost() {
  const { data } = await postApi.get(slug.value);
  form.value = {
    slug: data.slug,
    title: data.title,
    excerpt: data.excerpt,
    cover: data.cover,
    category: data.category,
    topic: data.topic,
    tags: data.tags,
    content_md: data.content_md,
    status: data.status,
    pinned: data.pinned,
    password: data.password,
    publish_at: data.publish_at || "",
  };
}

onMounted(async () => {
  loadTaxonomy();
  if (!isEdit.value) return;
  loading.value = true;
  try {
    await loadPost();
  } catch (e: any) {
    error.value = "加载失败";
  } finally {
    loading.value = false;
  }
});

// 回滚成功后重新拉取文章内容到编辑器。
async function handleRestored() {
  try {
    await loadPost();
    editorRef.value?.clearDraft();
  } catch (e: any) {
    error.value = "回滚后重载失败";
  }
}

async function handleSave() {
  saving.value = true;
  error.value = "";
  try {
    if (isEdit.value) {
      await postApi.update(slug.value, form.value);
    } else {
      await postApi.create(form.value);
    }
    editorRef.value?.clearDraft();
    router.push("/posts");
  } catch (e: any) {
    error.value = e.response?.data?.error || "保存失败";
  } finally {
    saving.value = false;
  }
}

const fieldCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const labelCls = "block text-xs text-gray-500 mb-1 dark:text-gray-400";
const chipCls =
  "px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600 hover:bg-indigo-100 hover:text-indigo-700 transition-colors dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-indigo-500/20 dark:hover:text-indigo-300";
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">
        {{ isEdit ? "编辑文章" : "新建文章" }}
      </h2>
      <div class="flex gap-2">
        <button
          v-if="isEdit"
          @click="revisionOpen = true"
          class="px-4 py-2 text-sm text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
        >版本历史</button>
        <router-link to="/posts" class="px-4 py-2 text-sm text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200">取消</router-link>
        <button
          @click="handleSave"
          :disabled="saving"
          class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
        >
          {{ saving ? "保存中..." : "保存" }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 text-sm text-red-600 bg-red-50 px-3 py-2 rounded-lg dark:text-red-400 dark:bg-red-500/10">{{ error }}</p>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Editor main -->
      <div class="lg:col-span-2 space-y-4">
        <input
          v-model="form.title"
          placeholder="文章标题"
          class="w-full px-4 py-3 text-lg font-medium border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
        />
        <MarkdownEditor
          ref="editorRef"
          v-model="form.content_md"
          :storage-key="'post:' + (slug || 'new')"
          @save="handleSave"
        />
      </div>

      <!-- Settings sidebar -->
      <div class="space-y-4">
        <CollapsibleSection title="发布设置" icon="ri:settings-3-line">
          <div>
            <label :class="labelCls">别名 (Slug)</label>
            <input v-model="form.slug" :class="fieldCls" @input="slugManuallyEdited = true" />
            <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">用于文章 URL，留空保存时从标题自动生成</p>
          </div>
          <div>
            <label :class="labelCls">状态</label>
            <select v-model="form.status" :class="fieldCls">
              <option value="draft">草稿</option>
              <option value="published">发布</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input type="checkbox" v-model="form.pinned" id="pinned" class="rounded" />
            <label for="pinned" class="text-sm text-gray-600 dark:text-gray-400">置顶</label>
          </div>
          <div>
            <label :class="labelCls">定时发布</label>
            <input v-model="form.publish_at" type="datetime-local" :class="fieldCls" />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="分类与标签" icon="ri:price-tag-3-line">
          <div>
            <label :class="labelCls">分类</label>
            <input v-model="form.category" :class="fieldCls" />
            <div v-if="categories.length" class="flex flex-wrap gap-1 mt-1.5">
              <button
                v-for="c in categories"
                :key="c"
                type="button"
                @click="pickCategory(c)"
                :class="[chipCls, form.category === c && '!bg-indigo-100 !text-indigo-700 dark:!bg-indigo-500/25 dark:!text-indigo-300']"
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
            <div v-if="tags.length" class="flex flex-wrap gap-1 mt-1.5">
              <button
                v-for="t in tags"
                :key="t"
                type="button"
                @click="pickTag(t)"
                :class="chipCls"
              >{{ t }}</button>
            </div>
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="其他" icon="ri:more-line">
          <div>
            <label :class="labelCls">摘要</label>
            <textarea v-model="form.excerpt" rows="3" :class="fieldCls + ' resize-y'"></textarea>
          </div>
          <div>
            <label :class="labelCls">封面 URL</label>
            <div class="flex gap-1.5">
              <input v-model="form.cover" :class="fieldCls + ' flex-1 min-w-0'" />
              <button type="button" @click="coverPickerOpen = true" title="从图床选择"
                class="shrink-0 px-2.5 py-2 text-xs rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700">选择</button>
              <button type="button" @click="fetchRandomCover" title="随机封面图（picsum.photos）"
                class="shrink-0 px-2.5 py-2 text-xs rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700">随机</button>
            </div>
          </div>
          <div>
            <label :class="labelCls">阅读密码</label>
            <input v-model="form.password" type="password" :class="fieldCls" />
          </div>
        </CollapsibleSection>

        <!-- 标签插件面板（Spec 29） -->
        <CollapsibleSection title="标签插件" icon="ri:puzzle-line" :default-open="false">
          <div class="flex flex-wrap gap-1 mb-2">
            <button
              type="button"
              @click="pluginCat = '全部'"
              :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === '全部' ? 'bg-indigo-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
            >全部</button>
            <button
              v-for="c in PLUGIN_CATEGORIES"
              :key="c"
              type="button"
              @click="pluginCat = c"
              :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === c ? 'bg-indigo-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
            >{{ c }}</button>
          </div>
          <select
            :value="selectedPlugin?.name"
            @change="selectPlugin(plugins.find((p) => p.name === ($event.target as HTMLSelectElement).value)!)"
            :class="fieldCls"
          >
            <option :value="undefined" disabled>选择插件…</option>
            <option v-for="p in pluginList" :key="p.name" :value="p.name">{{ p.title }}</option>
          </select>

          <div v-if="selectedPlugin" class="mt-3 space-y-3">
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
                @input="schedulePluginPreview()"
                rows="4"
                spellcheck="false"
                :class="fieldCls + ' resize-y font-mono text-xs leading-relaxed'"
              ></textarea>
            </div>

            <div>
              <label :class="labelCls">预览<span v-if="pluginRendering" class="ml-1 text-gray-400">渲染中…</span></label>
              <div class="rounded-lg border border-gray-200 overflow-hidden dark:border-gray-700">
                <iframe
                  v-if="pluginPreviewHtml"
                  :srcdoc="pluginSrcdoc"
                  class="w-full border-0 bg-white"
                  style="height: 200px"
                  sandbox="allow-same-origin allow-scripts"
                ></iframe>
                <p v-else class="px-3 py-4 text-center text-xs text-gray-400">填写参数后预览</p>
              </div>
            </div>

            <button
              type="button"
              @click="insertPlugin"
              :disabled="!pluginCode"
              class="w-full px-3 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
            >插入到正文</button>
          </div>
          <p v-else class="text-xs text-gray-400 dark:text-gray-500">选择插件后可编辑参数并插入正文。</p>
        </CollapsibleSection>
      </div>
    </div>

    <AttachmentPickerModal :open="coverPickerOpen" @select="(url: string) => { form.cover = url; coverPickerOpen = false; }" @close="coverPickerOpen = false" />

    <RevisionPanel
      :open="revisionOpen"
      :slug="slug"
      :current-content="form.content_md"
      :current-word-count="currentWordCount"
      @close="revisionOpen = false"
      @restored="handleRestored"
    />
  </div>
</template>
