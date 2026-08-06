<script setup lang="ts">
import { ref, onMounted, computed, watch } from"vue";
import { useRoute, useRouter } from"vue-router";
import { postApi, taxonomyApi, previewApi } from"@/api";
import MarkdownEditor from"@/components/MarkdownEditor.vue";
import RichTextEditor from"@/components/RichTextEditor.vue";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import EditorToolbar from"./EditorToolbar.vue";
import EditorSidebar from"./EditorSidebar.vue";
import AiGenerateModal from"./AiGenerateModal.vue";
import RevisionPanel from"./RevisionPanel.vue";
import { Icon } from"@iconify/vue";
import { Toast, Dialog } from"@/components/ui";

// 文章编辑器：Halo 式三栏布局——顶部全宽工具栏 + 左侧主编辑区 + 右侧大纲/详情面板。
// 双编辑器模式：默认富文本所见即得（TipTap），可切 Markdown 分屏兑底；
// 两者绑定同一份 form.content_md，切换即天然同步。
const route = useRoute();
const router = useRouter();

const isEdit = computed(() => !!route.params.slug);
const slug = computed(() => route.params.slug as string);

const form = ref({
 slug:"",
 title:"",
 excerpt:"",
 cover:"",
 category:"",
 topic:"",
 tags:"",
 content_md:"",
 status:"draft",
 pinned: false,
 password:"",
 publish_at:"",
 meta_description:"",
 meta_keywords:"",
 author:"",
});

const loading = ref(false);
const saving = ref(false);
const publishing = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
const richRef = ref<InstanceType<typeof RichTextEditor> | null>(null);
const coverPickerOpen = ref(false);
const revisionOpen = ref(false);
const aiOpen = ref(false);

// 编辑器种类：富文本（默认）/ Markdown 分屏，选择持久化
const KIND_KEY ="post-editor:kind";
type EditorKind ="rich" |"markdown";
function loadKind(): EditorKind {
 return localStorage.getItem(KIND_KEY) ==="markdown" ?"markdown" :"rich";
}
const editorKind = ref<EditorKind>(loadKind());
function onEditorKindSelect(e: Event) {
 const v = (e.target as HTMLSelectElement).value as EditorKind;
 editorKind.value = v;
 localStorage.setItem(KIND_KEY, v);
}

// 草稿头状态：originalStatus 为服务器线上状态，hasDraft 表示存在已保存未发布的草稿
const originalStatus = ref("draft");
const hasDraft = ref(false);
// AI 文章摘要（Spec 53）：独立于主表单保存链路，EditorSidebar 面板与 AiGenerateModal 共用此状态
const aiSummary = ref("");
const isPublishedPost = computed(() => isEdit.value && originalStatus.value ==="published");

// 编辑器模式（页头下拉与工具栏三态按钮双向同步）
type EditorMode ="edit" |"split" |"preview";
const editorMode = ref<EditorMode>("split");
const editorModeItems: { value: EditorMode; label: string }[] = [
 { value:"edit", label:"仅编辑" },
 { value:"split", label:"即时渲染" },
 { value:"preview", label:"仅预览" },
];
function onEditorModeSelect(e: Event) {
 const v = (e.target as HTMLSelectElement).value as EditorMode;
 setEditorMode(v);
}
function setEditorMode(m: EditorMode) {
 editorMode.value = m;
 editorRef.value?.setMode(m);
}

// 工具栏格式按钮激活态（仅富文本模式有；Markdown 模式不高亮）
const toolbarActive = computed<Record<string, boolean>>(() =>
 editorKind.value ==="rich" ? (richRef.value?.activeStates ?? {}) : {}
);

// 发布前预览：当前表单 POST 拿临时 token → 新标签页打开前台草稿渲染
const previewing = ref(false);
async function handlePreview() {
 if (previewing.value) return;
 previewing.value = true;
 try {
 const { data } = await previewApi.createPostPreview({ ...form.value });
 window.open(`/preview/draft/${data.token}`,"_blank");
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"预览生成失败");
 } finally {
 previewing.value = false;
 }
}

// 全屏模式：覆盖侧边栏，编辑器占满视口
const fullscreen = ref(false);
function toggleFullscreen() {
 fullscreen.value = !fullscreen.value;
}

// 右侧面板开关（批注：设置参数合并成大组件、可从右侧收起），状态持久化
const SIDEBAR_KEY ="post-editor:sidebar-open";
const sidebarOpen = ref(localStorage.getItem(SIDEBAR_KEY) !=="0");
function toggleSidebar() {
 sidebarOpen.value = !sidebarOpen.value;
 localStorage.setItem(SIDEBAR_KEY, sidebarOpen.value ?"1" :"0");
}

// Slug 自动生成：新建文章时标题变化自动推导 slug，用户手动编辑后停止覆盖（Spec Task 6）
const slugManuallyEdited = ref(false);

/** 从标题推导 URL 友好的 slug（与后端 generateSlug 对齐：保留 Unicode 字母/数字） */
function autoSlug(title: string): string {
 let s = (title ||"").trim().toLowerCase();
 s = s.replace(/\s+/g,"-");
 s = s.replace(/[^\p{L}\p{N}-]/gu,"");
 s = s.replace(/-{2,}/g,"-");
 s = s.replace(/^-+|-+$/g,"");
 return s;
}

// 标题变化时自动填充 slug（仅新建模式 + 用户未手动编辑）
watch(() => form.value.title, (newTitle) => {
 if (!isEdit.value && !slugManuallyEdited.value) {
 form.value.slug = autoSlug(newTitle);
 }
});

// 当前正文字数（非空白 rune），供版本面板计算字数变化。
const currentWordCount = computed(() => {
 const s = form.value.content_md ||"";
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

// ---------- 工具栏 → 编辑器动作路由（按当前编辑器种类分发） ----------

// Markdown 模式的 format(id) 映射表（与 EditorToolbar formatTools id 一致）
const tableTemplate ="\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";
const mdFormatMap: Record<string, () => void> = {
 bold: () => editorRef.value?.wrapSelection("**","**","加粗文本"),
 italic: () => editorRef.value?.wrapSelection("*","*","斜体文本"),
 strike: () => editorRef.value?.wrapSelection("~~","~~","删除线"),
 heading: () => editorRef.value?.prefixLines("##", false),
 link: () => editorRef.value?.wrapSelection("[","](url)","链接文字"),
 quote: () => editorRef.value?.prefixLines(">", false),
 code: () => editorRef.value?.wrapSelection("`","`","code"),
 codeblock: () => editorRef.value?.wrapSelection("\n```\n","\n```\n","code"),
 ul: () => editorRef.value?.prefixLines("-", false),
 ol: () => editorRef.value?.prefixLines("", true),
 table: () => editorRef.value?.insertBlock(tableTemplate),
};

function onFormat(id: string) {
 if (editorKind.value ==="rich") {
 richRef.value?.applyFormat(id);
 } else {
 mdFormatMap[id]?.();
 }
}

function onInsert(text: string) {
 if (editorKind.value ==="rich") {
 richRef.value?.insertMarkdown(text);
 } else {
 editorRef.value?.insertBlock(text);
 }
}

function onUpload() {
 (editorKind.value ==="rich" ? richRef.value : editorRef.value)?.openImageUpload();
}

function onPick() {
 (editorKind.value ==="rich" ? richRef.value : editorRef.value)?.openImagePicker();
}

/** 清除当前编辑器的本地草稿（同一 storageKey，两实例任一清除即可） */
function clearActiveDraft() {
 (editorKind.value ==="rich" ? richRef.value : editorRef.value)?.clearDraft();
}

// 大纲点击跳转：富文本按标题序号定位节点，Markdown 按行号滚动
function onJump(line: number, index: number) {
 if (editorKind.value ==="rich") {
 richRef.value?.scrollToHeading(index);
 } else {
 editorRef.value?.scrollToLine(line);
 }
}

// AI 属性生成结果回填（tags 为逗号串；summary 走独立端点直接落库）
async function applyAi(values: Record<string, string>) {
 if (values.title !== undefined) form.value.title = values.title;
 if (values.slug !== undefined) {
 form.value.slug = values.slug;
 slugManuallyEdited.value = true;
 }
 if (values.category !== undefined) form.value.category = values.category;
 if (values.tags !== undefined) form.value.tags = values.tags;
 if (values.excerpt !== undefined) form.value.excerpt = values.excerpt;
 if (values.summary !== undefined) {
 aiSummary.value = values.summary;
 if (slug.value) {
 try {
 await postApi.updateAISummary(slug.value, values.summary);
 } catch {
 Toast.warning("AI 摘要已回填但落库失败，请在侧栏重试");
 }
 }
 }
 Toast.success("已应用 AI 生成结果，记得保存");
}

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
 publish_at: src.publish_at ||"",
 meta_description: src.meta_description ||"",
 meta_keywords: src.meta_keywords ||"",
 author: src.author ||"",
 };
 aiSummary.value = data.ai_summary ||""; // Spec 53：AI 摘要随文章详情载入
}

onMounted(async () => {
 loadTaxonomy();
 editorMode.value = (editorRef.value?.getMode?.() as EditorMode) ||"split";
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
 clearActiveDraft();
 } catch (e: any) {
 Toast.error("回滚后重载失败");
 }
}

/** 保存：已发布文章 → 存草稿头（线上不变，列表出小绿点）；草稿/新建 → 直接保存为草稿 */
async function handleSave() {
 saving.value = true;
 try {
 if (isPublishedPost.value) {
 await postApi.saveDraft(slug.value, { ...form.value, status:"published" });
 hasDraft.value = true;
 clearActiveDraft();
 Toast.success("内容已保存，但还未发布");
 } else if (isEdit.value) {
 await postApi.update(slug.value, { ...form.value, status:"draft" });
 clearActiveDraft();
 Toast.success("保存成功");
 } else {
 const { data } = await postApi.create({ ...form.value, status:"draft" });
 clearActiveDraft();
 Toast.success("保存成功");
 // 转入编辑模式，后续保存走更新
 router.replace(`/posts/edit/${data.slug}`);
 originalStatus.value ="draft";
 }
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"保存失败");
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
 await postApi.create({ ...form.value, status:"published" });
 }
 clearActiveDraft();
 Toast.success("发布成功");
 router.push("/posts");
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"发布失败");
 } finally {
 publishing.value = false;
 }
}

/** 放弃草稿头，回到线上内容 */
function handleDiscardDraft() {
 Dialog.warning({
 title:"确定要放弃未发布的草稿吗？",
 description:"将丢弃已保存的草稿内容，恢复为线上发布版本",
 confirmType:"danger",
 onConfirm: async () => {
 try {
 await postApi.discardDraft(slug.value);
 hasDraft.value = false;
 await loadPost();
 clearActiveDraft();
 Toast.success("已放弃草稿");
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"操作失败");
 }
 },
 });
}
</script>

<template>
  <div :class="{ 'fixed inset-0 z-[9999] overflow-y-auto bg-background': fullscreen }">
    <!-- 紧凑 sticky 毛玻璃页头 -->
    <header
      class="sticky top-0 z-50 flex items-center gap-4 px-7 py-3"
      style="background: rgba(0,0,0,0.75); backdrop-filter: saturate(180%) blur(20px); -webkit-backdrop-filter: saturate(180%) blur(20px); border-bottom: 1px solid var(--color-border);"
    >
      <button
        type="button"
        title="返回"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
        @click="router.push('/posts')"
      >
        <Icon icon="ri:arrow-left-s-line" class="text-2xl" />
      </button>
      <h1 class="text-[17px] font-semibold tracking-tight text-foreground">
        {{ isEdit ? '编辑文章' : '新建文章' }}
      </h1>
      <div class="flex-1"></div>
      <div class="flex items-center gap-2">
        <!-- 编辑器选择（富文本/Markdown） -->
        <div
          class="relative flex items-center gap-1.5 rounded-lg border border-border bg-card px-2.5 py-1.5 text-sm text-text-secondary transition-colors hover:border-[var(--color-border-strong)]"
        >
          <Icon icon="ri:edit-box-line" class="text-base text-text-quaternary" />
          <select
            :value="editorKind"
            @change="onEditorKindSelect"
            class="cursor-pointer select-none appearance-none whitespace-nowrap border-0 bg-transparent pr-4 outline-none"
            title="切换编辑器"
          >
            <option value="rich">富文本编辑器</option>
            <option value="markdown">Markdown 编辑器</option>
          </select>
          <Icon
            icon="ri:arrow-down-s-line"
            class="pointer-events-none absolute right-1.5 text-base text-text-quaternary"
          />
        </div>
        <!-- Markdown 模式下的视图三态切换（与工具栏同步） -->
        <div
          v-if="editorKind === 'markdown'"
          class="relative flex items-center gap-1.5 rounded-lg border border-border bg-card px-2.5 py-1.5 text-sm text-text-secondary"
        >
          <select
            :value="editorMode"
            @change="onEditorModeSelect"
            class="cursor-pointer select-none appearance-none whitespace-nowrap border-0 bg-transparent pr-4 outline-none"
            title="切换视图模式"
          >
            <option v-for="m in editorModeItems" :key="m.value" :value="m.value">{{ m.label }}</option>
          </select>
          <Icon
            icon="ri:arrow-down-s-line"
            class="pointer-events-none absolute right-1.5 text-base text-text-quaternary"
          />
        </div>
        <!-- 预览（ghost） -->
        <button
          type="button"
          class="inline-flex h-[34px] items-center gap-1.5 rounded-[9px] px-3.5 text-[13px] font-medium text-text-secondary transition-colors hover:bg-white/5 hover:text-foreground"
          :disabled="previewing"
          @click="handlePreview"
        >
          <Icon icon="ri:eye-line" class="text-[15px]" />
          预览
        </button>
        <!-- 保存草稿（secondary） -->
        <button
          type="button"
          class="inline-flex h-[34px] items-center gap-1.5 rounded-[9px] border border-border bg-secondary px-3.5 text-[13px] font-medium text-foreground transition-colors hover:bg-accent disabled:opacity-50"
          :disabled="saving"
          @click="handleSave"
        >
          <Icon icon="ri:save-line" class="text-[15px]" />
          {{ saving ? '保存中…' : '保存草稿' }}
        </button>
        <!-- 发布（primary） -->
        <button
          type="button"
          class="inline-flex h-[34px] items-center gap-1.5 rounded-[9px] px-4 text-[13px] font-semibold text-white transition-all"
          style="background: var(--color-primary); box-shadow: 0 0 24px rgba(10,132,255,0.2);"
          :disabled="publishing"
          @click="handlePublish"
        >
          <Icon icon="ri:send-plane-fill" class="text-[15px]" />
          {{ publishing ? '发布中…' : '发布' }}
        </button>
      </div>
    </header>

    <!-- 草稿头条幅：令牌化绿色提示条 -->
    <div
      v-if="hasDraft && isPublishedPost"
      class="mx-7 mt-4 flex items-center justify-between gap-3 rounded-lg px-4 py-2 text-sm"
      style="background: rgba(48,209,88,0.12); color: #30D158; border: 1px solid rgba(48,209,88,0.2);"
    >
      <span class="flex items-center gap-2">
        <span class="inline-block h-1.5 w-1.5 rounded-full" style="background: #30D158;"></span>
        当前有内容已保存，但还未发布。正在编辑的是草稿版本，线上内容不受影响。
      </span>
      <button
        class="shrink-0 text-xs underline opacity-80 transition-opacity hover:opacity-100"
        @click="handleDiscardDraft"
      >
        放弃草稿
      </button>
    </div>

    <!-- 编辑区：左右两栏（左自适应 + 右 320px） -->
    <div class="flex gap-6 px-7 py-6">
      <!-- 左侧自适应编辑区 -->
      <div class="flex min-w-0 flex-1 flex-col">
        <!-- 标题区 -->
        <div
          class="rounded-t-[14px] border border-b-0 border-border bg-card px-9 pb-5 pt-7"
        >
          <input
            v-model="form.title"
            placeholder="文章标题"
            class="w-full border-0 bg-transparent text-[28px] font-bold leading-tight tracking-tight text-foreground outline-none placeholder:text-text-quaternary placeholder:font-semibold"
            style="font-size: 28px;"
          />
        </div>

        <!-- 工具栏（独立卡片条） -->
        <div class="border border-b-0 border-border bg-card">
          <EditorToolbar
            :kind="editorKind"
            :mode="editorMode"
            :sidebar-open="sidebarOpen"
            :is-edit="isEdit"
            :fullscreen="fullscreen"
            :active-states="toolbarActive"
            @format="onFormat"
            @insert="onInsert"
            @upload="onUpload"
            @pick="onPick"
            @update:mode="setEditorMode"
            @toggle-sidebar="toggleSidebar"
            @open-ai="aiOpen = true"
            @preview="handlePreview"
            @toggle-fullscreen="toggleFullscreen"
            @open-revision="revisionOpen = true"
          />
        </div>

        <!-- 编辑器内核（不改） -->
        <div class="rounded-b-[14px] border border-border bg-card">
          <RichTextEditor
            v-if="editorKind === 'rich'"
            ref="richRef"
            v-model="form.content_md"
            :storage-key="'post:' + (slug || 'new')"
            class="!border-0"
            @save="handleSave"
          />
          <MarkdownEditor
            v-else
            ref="editorRef"
            v-model="form.content_md"
            :toolbar="false"
            :storage-key="'post:' + (slug || 'new')"
            class="!border-0"
            @save="handleSave"
            @mode-change="(m: any) => (editorMode = m)"
          />
        </div>
      </div>

      <!-- 右侧 320px 固定设置面板 -->
      <div
        v-show="sidebarOpen"
        class="sticky top-[72px] w-80 shrink-0"
        style="max-height: calc(100vh - 88px); overflow-y: auto;"
      >
        <EditorSidebar
          :form="form"
          :categories="categories"
          :tags="tags"
          :ai-summary="aiSummary"
          @jump="onJump"
          @insert="onInsert"
          @open-cover-picker="coverPickerOpen = true"
          @slug-edited="slugManuallyEdited = true"
          @update:ai-summary="aiSummary = $event"
        />
      </div>
    </div>

    <AttachmentPickerModal
      :open="coverPickerOpen"
      @select="(url: string) => { form.cover = url; coverPickerOpen = false; }"
      @close="coverPickerOpen = false"
    />

    <AiGenerateModal
      :open="aiOpen"
      :form="form"
      :categories="categories"
      :tags="tags"
      @close="aiOpen = false"
      @apply="applyAi"
    />

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
