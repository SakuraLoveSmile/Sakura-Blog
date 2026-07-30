<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { postApi, taxonomyApi, previewApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import CollapsibleSection from "@/components/CollapsibleSection.vue";
import PluginParamForm from "@/modules/plugins/PluginParamForm.vue";
import RevisionPanel from "./RevisionPanel.vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, Toast, Dialog } from "@/components/ui";
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
  meta_description: "",
  meta_keywords: "",
});

const loading = ref(false);
const saving = ref(false);
const publishing = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
const coverPickerOpen = ref(false);
const revisionOpen = ref(false);

// 草稿头状态：originalStatus 为服务器线上状态，hasDraft 表示存在已保存未发布的草稿
const originalStatus = ref("draft");
const hasDraft = ref(false);
const isPublishedPost = computed(() => isEdit.value && originalStatus.value === "published");

// 编辑器模式（页头下拉与工具栏三态按钮双向同步）
type EditorMode = "edit" | "split" | "preview";
const editorMode = ref<EditorMode>("split");
const editorModeItems: { value: EditorMode; label: string }[] = [
  { value: "edit", label: "仅编辑" },
  { value: "split", label: "即时渲染" },
  { value: "preview", label: "仅预览" },
];
function onEditorModeSelect(e: Event) {
  const v = (e.target as HTMLSelectElement).value as EditorMode;
  editorMode.value = v;
  editorRef.value?.setMode(v);
}

// 全屏模式：覆盖侧边栏，编辑器占满视口
const fullscreen = ref(false);
function toggleFullscreen() {
  fullscreen.value = !fullscreen.value;
}

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

/** 随机封面图：picsum seed 模式，保存后 URL 固定图片不再变化（source.unsplash.com 已停用） */
function fetchRandomCover() {
  // picsum.photos 免费无需 API Key；seed 模式同一 URL 永远返回同一张图
  const seed = Math.random().toString(36).slice(2, 10);
  form.value.cover = `https://picsum.photos/seed/${seed}/1200/675`;
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

// ---------- 加载 / 保存 / 发布 ----------
async function loadPost() {
  const { data } = await postApi.get(slug.value);
  originalStatus.value = data.status;
  hasDraft.value = !!data.has_draft;
  // 有草稿头时优先载入草稿内容（线上内容不受影响）
  const src = data.has_draft && data.draft ? { ...data, ...data.draft } : data;
  form.value = {
    slug: src.slug || data.slug,
    title: src.title,
    excerpt: src.excerpt,
    cover: src.cover,
    category: src.category,
    topic: src.topic,
    tags: src.tags,
    content_md: src.content_md,
    status: data.status,
    pinned: src.pinned,
    password: src.password,
    publish_at: src.publish_at || "",
    meta_description: src.meta_description || "",
    meta_keywords: src.meta_keywords || "",
  };
}

onMounted(async () => {
  loadTaxonomy();
  editorMode.value = (editorRef.value?.getMode?.() as EditorMode) || "split";
  if (!isEdit.value) return;
  loading.value = true;
  try {
    await loadPost();
  } catch (e: any) {
    Toast.error("加载失败");
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
    Toast.error("回滚后重载失败");
  }
}

/** 保存：已发布文章 → 存草稿头（线上不变，列表出小绿点）；草稿/新建 → 直接保存为草稿 */
async function handleSave() {
  saving.value = true;
  try {
    if (isPublishedPost.value) {
      await postApi.saveDraft(slug.value, { ...form.value, status: "published" });
      hasDraft.value = true;
      editorRef.value?.clearDraft();
      Toast.success("内容已保存，但还未发布");
    } else if (isEdit.value) {
      await postApi.update(slug.value, { ...form.value, status: "draft" });
      editorRef.value?.clearDraft();
      Toast.success("保存成功");
    } else {
      const { data } = await postApi.create({ ...form.value, status: "draft" });
      editorRef.value?.clearDraft();
      Toast.success("保存成功");
      // 转入编辑模式，后续保存走更新
      router.replace(`/posts/edit/${data.slug}`);
      originalStatus.value = "draft";
    }
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

/** 发布：应用草稿头并上线（新建/草稿文章直接以发布状态保存） */
async function handlePublish() {
  publishing.value = true;
  try {
    if (isEdit.value) {
      await postApi.publish(slug.value, form.value);
    } else {
      await postApi.create({ ...form.value, status: "published" });
    }
    editorRef.value?.clearDraft();
    Toast.success("发布成功");
    router.push("/posts");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "发布失败");
  } finally {
    publishing.value = false;
  }
}

/** 放弃草稿头，回到线上内容 */
function handleDiscardDraft() {
  Dialog.warning({
    title: "确定要放弃未发布的草稿吗？",
    description: "将丢弃已保存的草稿内容，恢复为线上发布版本",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.discardDraft(slug.value);
        hasDraft.value = false;
        await loadPost();
        editorRef.value?.clearDraft();
        Toast.success("已放弃草稿");
      } catch (e: any) {
        Toast.error(e.response?.data?.error || "操作失败");
      }
    },
  });
}

const fieldCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const labelCls = "block text-xs text-gray-500 mb-1 dark:text-gray-400";
const chipCls =
  "px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600 hover:bg-primary/15 hover:text-secondary transition-colors dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-primary/20 dark:hover:text-primary";
</script>

<template>
  <div :class="{ 'fixed inset-0 z-[9999] overflow-y-auto bg-gray-50 dark:bg-gray-900': fullscreen }">
    <VPageHeader :title="isEdit ? '编辑文章' : '新建文章'">
      <template #icon>
        <Icon icon="ri:book-read-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <div class="flex items-center gap-2">
          <!-- 编辑器模式切换（与工具栏三态按钮同步） -->
          <div class="group flex cursor-pointer items-center gap-1.5 rounded p-1 hover:bg-gray-100 dark:hover:bg-gray-700">
            <Icon icon="ri:edit-box-line" class="text-base text-gray-600 dark:text-gray-400" />
            <select
              :value="editorMode"
              @change="onEditorModeSelect"
              class="cursor-pointer select-none whitespace-nowrap border-0 bg-transparent text-sm text-gray-600 outline-none group-hover:text-gray-900 dark:text-gray-400 dark:group-hover:text-gray-100"
              title="切换编辑器模式"
            >
              <option v-for="m in editorModeItems" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
          </div>
          <VButton size="sm" :title="fullscreen ? '退出全屏' : '全屏编辑'" @click="toggleFullscreen">
            <template #icon>
              <Icon :icon="fullscreen ? 'ri:fullscreen-exit-line' : 'ri:fullscreen-line'" class="h-full w-full" />
            </template>
          </VButton>
          <VButton v-if="isEdit" size="sm" @click="revisionOpen = true">
            <template #icon>
              <Icon icon="ri:history-line" class="h-full w-full" />
            </template>
            版本历史
          </VButton>
          <VButton @click="router.push('/posts')">取消</VButton>
          <VButton :loading="saving" @click="handleSave">
            <template #icon>
              <Icon icon="ri:save-line" class="h-full w-full" />
            </template>
            保存
          </VButton>
          <VButton type="secondary" :loading="publishing" @click="handlePublish">
            <template #icon>
              <Icon icon="ri:send-plane-fill" class="h-full w-full" />
            </template>
            发布
          </VButton>
        </div>
      </template>
    </VPageHeader>

    <!-- 草稿头条幅：已发布文章存在已保存未发布的内容 -->
    <div
      v-if="hasDraft && isPublishedPost"
      class="mx-0 mt-4 flex items-center justify-between gap-3 border border-green-200 bg-green-50 px-4 py-2 text-sm text-green-800 md:mx-4 dark:border-green-500/20 dark:bg-green-500/10 dark:text-green-400"
      style="border-radius: 4px"
    >
      <span class="flex items-center gap-2">
        <span class="inline-block h-1.5 w-1.5 rounded-full bg-green-500"></span>
        当前有内容已保存，但还未发布。正在编辑的是草稿版本，线上内容不受影响。
      </span>
      <button
        class="shrink-0 text-xs text-green-700 underline hover:text-green-900 dark:text-green-400 dark:hover:text-green-300"
        @click="handleDiscardDraft"
      >
        放弃草稿
      </button>
    </div>

    <div class="m-0 md:m-4">
    <div class="grid grid-cols-1 gap-6" :class="fullscreen ? 'lg:grid-cols-1' : 'lg:grid-cols-3'">
      <!-- Editor main -->
      <div :class="['space-y-4', fullscreen ? '' : 'lg:col-span-2']">
        <input
          v-model="form.title"
          placeholder="文章标题"
          class="w-full px-4 py-3 text-lg font-medium border border-gray-200 rounded-xl focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
        />
        <MarkdownEditor
          ref="editorRef"
          v-model="form.content_md"
          :storage-key="'post:' + (slug || 'new')"
          @save="handleSave"
          @mode-change="(m: any) => (editorMode = m)"
        />
      </div>

      <!-- Settings sidebar（全屏时隐藏） -->
      <div v-show="!fullscreen" class="space-y-4">
        <CollapsibleSection title="发布设置" icon="ri:settings-3-line">
          <div>
            <label :class="labelCls">别名 (Slug)</label>
            <input v-model="form.slug" :class="fieldCls" @input="slugManuallyEdited = true" />
            <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">用于文章 URL，留空保存时从标题自动生成</p>
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
                :class="[chipCls, form.category === c && '!bg-primary/20 !text-secondary dark:!bg-primary/25 dark:!text-primary']"
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

        <!-- 文章级 SEO（Spec 42）：留空回退站点默认/摘要 -->
        <CollapsibleSection title="SEO" icon="ri:search-eye-line" :default-open="false">
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
        </CollapsibleSection>

        <!-- 标签插件面板（Spec 29） -->
        <CollapsibleSection title="标签插件" icon="ri:puzzle-line" :default-open="false">
          <div class="flex flex-wrap gap-1 mb-2">
            <button
              type="button"
              @click="pluginCat = '全部'"
              :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === '全部' ? 'bg-secondary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
            >全部</button>
            <button
              v-for="c in PLUGIN_CATEGORIES"
              :key="c"
              type="button"
              @click="pluginCat = c"
              :class="['px-2 py-0.5 text-xs rounded-full transition-colors', pluginCat === c ? 'bg-secondary text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300']"
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

            <VButton
              type="secondary"
              block
              :disabled="!pluginCode"
              @click="insertPlugin"
            >插入到正文</VButton>
          </div>
          <p v-else class="text-xs text-gray-400 dark:text-gray-500">选择插件后可编辑参数并插入正文。</p>
        </CollapsibleSection>
      </div>
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
