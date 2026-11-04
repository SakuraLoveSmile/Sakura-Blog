<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, useSlots } from "vue";
import { Icon } from "@iconify/vue";
import { VModal, VButton } from "@/components/ui";
import { type PluginMeta, previewInfoFor } from "@/modules/plugins/plugin-types";
import { PLUGIN_CATEGORIES } from "@/modules/plugins/plugin-types";
import { loadPluginCatalog, runtimePlugins } from "@/modules/plugins/usePluginCatalog";
import { usePluginPreview } from "@/modules/plugins/preview/usePluginPreview";
import { sharedPreviewQueue } from "@/modules/plugins/preview/render-gate";

// 编辑器顶部工具栏（对照 Halo 编辑器）：
// 左侧「+」插入下拉（Stellar 标签，悬停出预览气泡）+ 格式化按钮组 + 图片；
// 右侧 AI 属性生成、编辑/分屏/预览三态切换（仅 Markdown 模式）、右侧面板开关。
// 格式化按钮发语义化 format(id) 事件，由宿主路由到富文本/Markdown 编辑器。
type ViewMode = "edit" | "split" | "preview";
type EditorKind = "rich" | "markdown";

const props = defineProps<{
  kind: EditorKind;
  mode: ViewMode;
  sidebarOpen: boolean;
  isEdit: boolean;
  fullscreen: boolean;
  activeStates?: Record<string, boolean>;
}>();

const slots = useSlots();

const emit = defineEmits<{
  (e: "format", id: string): void;
  (e: "insert", text: string): void;
  (e: "import", file: File, legacy: boolean): void;
  (e: "upload"): void;
  (e: "pick"): void;
  (e: "update:mode", mode: ViewMode): void;
  (e: "toggleSidebar"): void;
  (e: "openAi"): void;
  (e: "preview"): void;
  (e: "toggleFullscreen"): void;
  (e: "openRevision"): void;
}>();

// 格式化按钮（id 与 RichTextEditor.applyFormat / PostEditor markdown 映射表一致）
const formatTools = [
  { id: "bold", icon: "ri:bold", title: "加粗 (Cmd/Ctrl+B)" },
  { id: "italic", icon: "ri:italic", title: "斜体 (Cmd/Ctrl+I)" },
  { id: "strike", icon: "ri:strikethrough", title: "删除线" },
  { id: "heading", icon: "ri:heading", title: "标题" },
  { id: "link", icon: "ri:link", title: "链接 (Cmd/Ctrl+K)" },
  { id: "quote", icon: "ri:double-quotes-l", title: "引用" },
  { id: "code", icon: "ri:code-line", title: "行内代码" },
  { id: "codeblock", icon: "ri:code-box-line", title: "代码块" },
  { id: "ul", icon: "ri:list-unordered", title: "无序列表" },
  { id: "ol", icon: "ri:list-ordered", title: "有序列表" },
  { id: "table", icon: "ri:table-2", title: "插入表格" },
];

const modeButtons: { value: ViewMode; icon: string; title: string }[] = [
  { value: "edit", icon: "ri:edit-2-line", title: "仅编辑" },
  { value: "split", icon: "ri:layout-column-line", title: "即时渲染（分屏预览）" },
  { value: "preview", icon: "ri:eye-line", title: "仅预览" },
];

// 格式化按钮按设计稿分组（组间左边框分隔）
const formatGroupText = ["bold", "italic", "strike", "heading"];
const formatGroupInsert = ["link", "quote", "code", "codeblock"];
const formatGroupList = ["ul", "ol", "table"];
function toolsById(ids: string[]) {
  return formatTools.filter((t) => ids.includes(t.id));
}

// ---------- 「+」插入下拉：Stellar 标签插件，悬停出预览气泡 ----------

const insertOpen = ref(false);
const dropdownRef = ref<HTMLElement | null>(null);
const importInput = ref<HTMLInputElement | null>(null);
const importFormatOpen = ref(false);
const legacyImport = ref(false);

function chooseImportFile(legacy: boolean) {
  legacyImport.value = legacy;
  importFormatOpen.value = false;
  importInput.value?.click();
}

function onImportFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  // Reset immediately so selecting the same file twice still emits an event.
  input.value = "";
  if (file) emit("import", file, legacyImport.value);
}

// 按分类分组（保持 PLUGIN_CATEGORIES 顺序）
const groupedPlugins = computed(() =>
  PLUGIN_CATEGORIES.map((cat) => ({
    cat,
    items: runtimePlugins.value.filter((p) => p.category === cat),
  })).filter((g) => g.items.length > 0),
);

// 分类图标（菜单项左侧，对照 Halo 插入菜单的 icon+文字样式）
const categoryIcons: Record<string, string> = {
  提示标注: "ri:alert-line",
  布局容器: "ri:layout-grid-line",
  媒体播放: "ri:play-circle-line",
  数据图表: "ri:bar-chart-2-line",
  GitHub: "ri:github-line",
};

function toggleInsert() {
  insertOpen.value = !insertOpen.value;
  if (!insertOpen.value) hideBubble();
}

function onDocClick(e: MouseEvent) {
  if (insertOpen.value && dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    insertOpen.value = false;
    hideBubble();
  }
}
onMounted(() => {
  loadPluginCatalog();
  document.addEventListener("click", onDocClick);
  window.addEventListener("message", onPreviewMessage);
});
onBeforeUnmount(() => {
  document.removeEventListener("click", onDocClick);
  window.removeEventListener("message", onPreviewMessage);
  clearTimeout(hoverTimer);
});

function insertPlugin(p: PluginMeta) {
  emit("insert", "\n" + p.example + "\n");
  insertOpen.value = false;
  hideBubble();
}

// ---------- 悬停/聚焦预览气泡 ----------
// 与插入面板共用整文档预览管线（component-library fixture、mode:content）：
// 加载运行时脚本后 friends/timeline/tabs 不再是 CSS 空壳；请求静默、按示例缓存、序列号防串台。

const bubblePlugin = ref<PluginMeta | null>(null);
const bubbleTop = ref(0);
const bubbleFrame = ref<HTMLIFrameElement | null>(null);
const bubblePreview = usePluginPreview(
  () => (bubblePlugin.value ? previewInfoFor(bubblePlugin.value) : { capability: "static", height: 180 }),
  undefined,
  { delayMs: 0, queue: sharedPreviewQueue, frame: { minHeight: 180, maxHeight: 360, margin: 0 } },
);
const bubbleHeight = computed(() => bubblePreview.frameHeight.value);
let hoverTimer: ReturnType<typeof setTimeout> | undefined;
// 记录当前气泡展示的是哪个插件：A→B 未返回时先清旧文档，避免标题 B 配内容 A。
let shownPluginName: string | null = null;

function showBubble(p: PluginMeta, item: HTMLElement) {
  const switching = shownPluginName !== null && shownPluginName !== p.name;
  bubblePlugin.value = p;
  // 气泡顶部与菜单项对齐（相对下拉容器），限制不超出下拉高度
  const menu = dropdownRef.value?.querySelector(".insert-menu") as HTMLElement | null;
  bubbleTop.value = Math.max(0, item.offsetTop - (menu?.scrollTop || 0));
  if (switching) bubblePreview.clearPreview();
  shownPluginName = p.name;
  bubblePreview.resetHeight();
  bubblePreview.scheduleRender(p.example, { immediate: true });
}

// hover 与键盘 focus 共用同一 300ms 防抖：Tab 连续遍历只渲染最后一项，mousedown focus 后 click 插入也不会多打请求。
function scheduleBubble(p: PluginMeta, item: HTMLElement) {
  clearTimeout(hoverTimer);
  hoverTimer = setTimeout(() => showBubble(p, item), 300);
}

function onItemEnter(p: PluginMeta, e: Event) {
  scheduleBubble(p, e.currentTarget as HTMLElement);
}

function onItemFocus(p: PluginMeta, e: FocusEvent) {
  scheduleBubble(p, e.currentTarget as HTMLElement);
}

function onItemLeave() {
  clearTimeout(hoverTimer);
}

function hideBubble() {
  clearTimeout(hoverTimer);
  bubblePlugin.value = null;
  shownPluginName = null;
  bubblePreview.clearPreview();
}

function onPreviewMessage(event: MessageEvent) {
  bubblePreview.onMessage(event, bubbleFrame.value);
}

// 工具栏按钮统一样式（Apple 风：32px、rounded-lg、hover 灰底）。
// 激活态用 scoped 的 .btn-active（浅蓝 tint 底 + 蓝色图标）——不叠工具类，
// 避免 text-primary-foreground 等颜色工具类在级联中被 btnChrome 覆盖失效。
const btnBase = "flex h-8 w-8 items-center justify-center rounded-lg transition-colors";
const btnChrome = "text-text-secondary hover:bg-secondary hover:text-foreground";
const btnActive = "btn-active";
</script>

<template>
  <div class="editor-toolbar relative z-10 flex min-w-0 items-center gap-0 px-4 py-2">
    <!-- 分组 1：插入 Stellar 标签 -->
    <div class="editor-toolbar-insert relative z-20 flex shrink-0 items-center gap-0.5 px-1.5">
      <div ref="dropdownRef" class="relative">
        <button
          type="button"
          title="插入 Stellar 标签"
          :class="[btnBase, insertOpen ? btnActive : btnChrome]"
          @click="toggleInsert"
        >
          <Icon icon="ri:add-line" class="text-lg" />
        </button>

        <div
          v-if="insertOpen"
          class="absolute left-0 top-full z-30 insert-popover mt-1 flex flex-col gap-2 sm:flex-row sm:items-start sm:gap-0"
        >
          <div class="insert-menu max-h-96 w-52 overflow-y-auto rounded-lg border border-border bg-card py-1 shadow-lg">
            <template v-for="group in groupedPlugins" :key="group.cat">
              <div class="flex items-center gap-1.5 px-3 pb-1 pt-2 text-[11px] text-text-quaternary">
                <Icon :icon="categoryIcons[group.cat] || 'ri:puzzle-line'" />
                {{ group.cat }}
              </div>
              <button
                v-for="p in group.items"
                :key="p.name"
                type="button"
                class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm text-text-secondary transition-colors hover:bg-accent"
                @click="insertPlugin(p)"
                @mouseenter="onItemEnter(p, $event)"
                @mouseleave="onItemLeave"
                @focus="onItemFocus(p, $event)"
                @blur="onItemLeave"
              >
                <span class="flex-1 truncate">{{ p.title }}</span>
                <code class="text-[10px] text-text-quaternary">{{ p.name }}</code>
              </button>
            </template>
          </div>

          <div
            v-if="bubblePlugin"
            class="insert-bubble w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-lg border border-border bg-card shadow-xl sm:ml-2"
            :style="{ marginTop: bubbleTop + 'px' }"
          >
            <div class="flex items-center justify-between gap-2 border-b border-border px-3 py-1.5">
              <span class="text-xs font-medium text-text-secondary">{{ bubblePlugin.title }} 预览</span>
              <span v-if="bubblePreview.status.value === 'loading'" class="text-[10px] text-text-quaternary"
                >渲染中…</span
              >
              <span v-else-if="bubblePreview.status.value === 'updating'" class="text-[10px] text-text-quaternary"
                >更新中…</span
              >
              <button
                v-else-if="bubblePreview.status.value === 'error'"
                type="button"
                class="text-[10px] text-primary"
                @click="bubblePreview.retry()"
              >
                重试
              </button>
            </div>
            <iframe
              v-if="bubblePreview.srcdoc.value"
              ref="bubbleFrame"
              :srcdoc="bubblePreview.srcdoc.value"
              class="block w-full border-0 bg-card transition-[height] duration-150"
              :style="{ height: bubbleHeight + 'px', maxHeight: '360px' }"
              sandbox="allow-scripts"
              title="标签预览"
            ></iframe>
            <p
              v-else-if="bubblePreview.status.value === 'error'"
              class="px-3 py-6 text-center text-xs text-state-error"
            >
              {{ bubblePreview.previewError.value }}
              <button type="button" class="ml-1 text-primary" @click="bubblePreview.retry()">重试</button>
            </p>
            <p v-else class="px-3 py-6 text-center text-xs text-text-quaternary">渲染中…</p>
            <p
              v-if="bubblePreview.srcdoc.value && bubblePreview.status.value === 'error'"
              class="border-t border-state-error/30 px-3 py-1.5 text-[11px] text-state-error"
            >
              预览更新失败，仍显示上一次结果
            </p>
          </div>
        </div>
      </div>
      <input
        ref="importInput"
        type="file"
        accept=".md,.markdown,text/markdown,text/plain"
        class="hidden"
        @change="onImportFileChange"
      />
      <button type="button" title="导入 Markdown 文件" :class="[btnBase, btnChrome]" @click="importFormatOpen = true">
        <Icon icon="ri:file-upload-line" class="text-lg" />
      </button>
    </div>

    <!-- 其余工具保留横向滚动，但不再裁剪插入菜单的垂直溢出内容。 -->
    <div class="editor-toolbar-scroll flex min-w-0 flex-1 items-center gap-0 overflow-x-auto">
      <!-- 分组 2：文本格式（粗体/斜体/删除线/标题） -->
      <div class="flex items-center gap-0.5 border-l border-border px-1.5">
        <button
          v-for="tool in toolsById(formatGroupText)"
          :key="tool.id"
          type="button"
          :title="tool.title"
          :class="[btnBase, props.activeStates?.[tool.id] ? btnActive : btnChrome]"
          @click="emit('format', tool.id)"
        >
          <Icon :icon="tool.icon" class="text-lg" />
        </button>
      </div>

      <!-- 分组 3：链接/图片/代码/引用 -->
      <div class="flex items-center gap-0.5 border-l border-border px-1.5">
        <button
          v-for="tool in toolsById(formatGroupInsert)"
          :key="tool.id"
          type="button"
          :title="tool.title"
          :class="[btnBase, props.activeStates?.[tool.id] ? btnActive : btnChrome]"
          @click="emit('format', tool.id)"
        >
          <Icon :icon="tool.icon" class="text-lg" />
        </button>
        <button type="button" title="上传图片" :class="[btnBase, btnChrome]" @click="emit('upload')">
          <Icon icon="ri:image-add-line" class="text-lg" />
        </button>
        <button type="button" title="从图床选择" :class="[btnBase, btnChrome]" @click="emit('pick')">
          <Icon icon="ri:gallery-line" class="text-lg" />
        </button>
      </div>

      <!-- 分组 4：列表/表格 -->
      <div class="flex items-center gap-0.5 border-l border-border px-1.5">
        <button
          v-for="tool in toolsById(formatGroupList)"
          :key="tool.id"
          type="button"
          :title="tool.title"
          :class="[btnBase, props.activeStates?.[tool.id] ? btnActive : btnChrome]"
          @click="emit('format', tool.id)"
        >
          <Icon :icon="tool.icon" class="text-lg" />
        </button>
      </div>

      <span class="flex-1"></span>

      <!-- 分组 5：AI / 预览 / 版本 / 全屏 -->
      <div class="flex items-center gap-0.5 px-1.5">
        <button
          type="button"
          title="AI 属性生成"
          class="flex h-8 w-8 items-center justify-center rounded-lg text-system-indigo transition-colors hover:bg-secondary"
          @click="emit('openAi')"
        >
          <Icon icon="ri:sparkling-2-line" class="text-lg" />
        </button>
        <button type="button" title="发布前预览" :class="[btnBase, btnChrome]" @click="emit('preview')">
          <Icon icon="ri:eye-line" class="text-lg" />
        </button>
        <button
          v-if="props.isEdit"
          type="button"
          title="版本历史"
          :class="[btnBase, btnChrome]"
          @click="emit('openRevision')"
        >
          <Icon icon="ri:history-line" class="text-lg" />
        </button>
        <button
          type="button"
          :title="props.fullscreen ? '退出全屏' : '全屏编辑'"
          :class="[btnBase, props.fullscreen ? btnActive : btnChrome]"
          @click="emit('toggleFullscreen')"
        >
          <Icon :icon="props.fullscreen ? 'ri:fullscreen-exit-line' : 'ri:fullscreen-line'" class="text-lg" />
        </button>
      </div>

      <!-- 编辑 / 分屏 / 预览 三态切换（仅 Markdown 模式） -->
      <div v-if="props.kind === 'markdown'" class="ml-1 flex items-center gap-0.5 rounded-lg bg-secondary p-0.5">
        <button
          v-for="btn in modeButtons"
          :key="btn.value"
          type="button"
          :title="btn.title"
          :class="[
            'flex h-7 items-center justify-center rounded-md px-2.5 text-base transition-colors',
            props.mode === btn.value ? 'bg-card text-primary shadow-sm' : 'text-text-tertiary hover:text-foreground',
          ]"
          @click="emit('update:mode', btn.value)"
        >
          <Icon :icon="btn.icon" />
        </button>
      </div>

      <!-- 右侧面板开关（宿主提供 #end 时由宿主接管） -->
      <div v-if="!slots.end" class="flex items-center gap-0.5 border-l border-border px-1.5">
        <button
          type="button"
          :title="props.sidebarOpen ? '收起右侧面板' : '展开右侧面板'"
          :class="[btnBase, props.sidebarOpen ? btnActive : btnChrome]"
          @click="emit('toggleSidebar')"
        >
          <Icon icon="ri:layout-right-line" class="text-lg" />
        </button>
      </div>
    </div>

    <!-- 宿主专属右侧控件：不参与中间横向滚动，也不受 overflow-x-auto 裁切。 -->
    <div v-if="slots.end" class="editor-toolbar-end relative z-20 flex shrink-0 items-center gap-0.5 pl-1">
      <slot name="end" />
    </div>
  </div>
  <VModal v-model:visible="importFormatOpen" title="选择 Markdown 格式" :width="480">
    <div class="space-y-4 text-sm text-text-secondary">
      <p>选择文件后会先显示预览，确认回填后仍需保存。</p>
      <div class="space-y-2">
        <VButton class="w-full" @click="chooseImportFile(false)">Stellar 2 / 普通 Markdown</VButton>
        <p class="text-xs text-text-tertiary">适用于 2.0 导出的文件，或没有旧配置字段的 Markdown。</p>
      </div>
      <div class="space-y-2">
        <VButton class="w-full" type="primary" @click="chooseImportFile(true)">从 1.x 转换</VButton>
        <p class="text-xs text-text-tertiary">转换旧版状态、分类、置顶和标签；无法转换的字段会说明原因。</p>
      </div>
    </div>
  </VModal>
</template>

<style scoped>
/* ===== 选中态（加粗/侧边栏/全屏/插入下拉等）=====
   浅蓝 tint 底 + 蓝色图标（macOS 工具栏惯例；scoped 特异性高于工具类，避免级联覆盖） */
.btn-active {
  background: color-mix(in srgb, var(--color-primary) 18%, transparent);
  color: var(--color-primary);
}
.btn-active:hover {
  background: color-mix(in srgb, var(--color-primary) 26%, transparent);
  color: var(--color-primary-hover);
}

/* ===== 横向滚动条隐藏 =====
   工具栏在窄宽度下保留横向滚动，但滚动条本身会贴着按钮下沿显示（macOS 悬浮条在滚动时可见），
   观感与工具栏不一致，因此只隐藏滚动条，不改变溢出滚动行为。 */
.editor-toolbar-scroll {
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.editor-toolbar-scroll::-webkit-scrollbar {
  display: none;
}

/* ===== 移动端适配 ===== */
@media (max-width: 640px) {
  /* 工具栏更紧凑 */
  :deep(.toolbar-wrap) {
    gap: 0;
    padding: 4px 8px;
  }
  /* 390px 等窄视口：菜单与气泡改为上下排列，气泡收窄到视口内，不横向溢出 */
  .insert-popover {
    flex-direction: column;
    gap: 8px;
  }
  .insert-bubble {
    width: calc(100vw - 2rem);
    max-width: 20rem;
    margin-left: 0;
    margin-top: 0 !important;
  }
  /* 分组边框在移动端稍微减淡 */
  .border-l {
    border-left-width: 1px;
  }
}
</style>
