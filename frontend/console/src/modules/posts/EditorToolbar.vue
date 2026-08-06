<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from"vue";
import { Icon } from"@iconify/vue";
import { previewApi } from"@/api";
import {
 plugins,
 PLUGIN_CATEGORIES,
 type PluginMeta,
} from"@/modules/plugins/plugins-data";

// 编辑器顶部工具栏（对照 Halo 编辑器）：
// 左侧「+」插入下拉（Stellar 标签，悬停出预览气泡）+ 格式化按钮组 + 图片；
// 右侧 AI 属性生成、编辑/分屏/预览三态切换（仅 Markdown 模式）、右侧面板开关。
// 格式化按钮发语义化 format(id) 事件，由宿主路由到富文本/Markdown 编辑器。
type ViewMode ="edit" |"split" |"preview";
type EditorKind ="rich" |"markdown";

const props = defineProps<{
 kind: EditorKind;
 mode: ViewMode;
 sidebarOpen: boolean;
 isEdit: boolean;
 fullscreen: boolean;
 activeStates?: Record<string, boolean>;
}>();

const emit = defineEmits<{
 (e:"format", id: string): void;
 (e:"insert", text: string): void;
 (e:"upload"): void;
 (e:"pick"): void;
 (e:"update:mode", mode: ViewMode): void;
 (e:"toggleSidebar"): void;
 (e:"openAi"): void;
 (e:"preview"): void;
 (e:"toggleFullscreen"): void;
 (e:"openRevision"): void;
}>();

// 格式化按钮（id 与 RichTextEditor.applyFormat / PostEditor markdown 映射表一致）
const formatTools = [
 { id:"bold", icon:"ri:bold", title:"加粗 (Cmd/Ctrl+B)" },
 { id:"italic", icon:"ri:italic", title:"斜体 (Cmd/Ctrl+I)" },
 { id:"strike", icon:"ri:strikethrough", title:"删除线" },
 { id:"heading", icon:"ri:heading", title:"标题" },
 { id:"link", icon:"ri:link", title:"链接 (Cmd/Ctrl+K)" },
 { id:"quote", icon:"ri:double-quotes-l", title:"引用" },
 { id:"code", icon:"ri:code-line", title:"行内代码" },
 { id:"codeblock", icon:"ri:code-box-line", title:"代码块" },
 { id:"ul", icon:"ri:list-unordered", title:"无序列表" },
 { id:"ol", icon:"ri:list-ordered", title:"有序列表" },
 { id:"table", icon:"ri:table-2", title:"插入表格" },
];

const modeButtons: { value: ViewMode; icon: string; title: string }[] = [
 { value:"edit", icon:"ri:edit-2-line", title:"仅编辑" },
 { value:"split", icon:"ri:layout-column-line", title:"即时渲染（分屏预览）" },
 { value:"preview", icon:"ri:eye-line", title:"仅预览" },
];

// ---------- 「+」插入下拉：Stellar 标签插件，悬停出预览气泡 ----------

const insertOpen = ref(false);
const dropdownRef = ref<HTMLElement | null>(null);

// 按分类分组（保持 PLUGIN_CATEGORIES 顺序）
const groupedPlugins = computed(() =>
 PLUGIN_CATEGORIES.map((cat) => ({
 cat,
 items: plugins.filter((p) => p.category === cat),
 })).filter((g) => g.items.length > 0)
);

// 分类图标（菜单项左侧，对照 Halo 插入菜单的 icon+文字样式）
const categoryIcons: Record<string, string> = {
 提示标注:"ri:alert-line",
 布局容器:"ri:layout-grid-line",
 媒体播放:"ri:play-circle-line",
 数据图表:"ri:bar-chart-2-line",
 GitHub:"ri:github-line",
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
onMounted(() => document.addEventListener("click", onDocClick));
onBeforeUnmount(() => {
 document.removeEventListener("click", onDocClick);
 clearTimeout(hoverTimer);
});

function insertPlugin(p: PluginMeta) {
 emit("insert","\n" + p.example +"\n");
 insertOpen.value = false;
 hideBubble();
}

// ---------- 悬停预览气泡（300ms 防抖；渲染结果按插件名缓存） ----------

const bubblePlugin = ref<PluginMeta | null>(null);
const bubbleTop = ref(0);
const bubbleHtml = ref("");
const bubbleLoading = ref(false);
const previewCache = new Map<string, string>();
let hoverTimer: ReturnType<typeof setTimeout> | undefined;

function onItemEnter(p: PluginMeta, e: MouseEvent) {
 clearTimeout(hoverTimer);
 const item = e.currentTarget as HTMLElement;
 hoverTimer = setTimeout(async () => {
 bubblePlugin.value = p;
 // 气泡顶部与菜单项对齐（相对下拉容器），限制不超出下拉高度
 const menu = dropdownRef.value?.querySelector(".insert-menu") as HTMLElement | null;
 bubbleTop.value = Math.max(0, item.offsetTop - (menu?.scrollTop || 0));
 const cached = previewCache.get(p.name);
 if (cached !== undefined) {
 bubbleHtml.value = cached;
 return;
 }
 bubbleLoading.value = true;
 bubbleHtml.value ="";
 try {
 const { data } = await previewApi.render(p.example);
 previewCache.set(p.name, data.html);
 // 请求返回时鼠标可能已移到其他项，只有仍悬停在该项才展示
 if (bubblePlugin.value?.name === p.name) bubbleHtml.value = data.html;
 } catch {
 if (bubblePlugin.value?.name === p.name) bubbleHtml.value = '<p style="color:#ef4444">预览渲染失败</p>';
 } finally {
 bubbleLoading.value = false;
 }
 }, 300);
}

function onItemLeave() {
 clearTimeout(hoverTimer);
}

function hideBubble() {
 clearTimeout(hoverTimer);
 bubblePlugin.value = null;
 bubbleHtml.value ="";
}

// 预览气泡 iframe：深色-only，固定 dark 主题
const bubbleSrcdoc = computed(() => {
 return `<!DOCTYPE html>
<html data-theme="dark">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>html,body{margin:0;padding:0;}body{padding:12px;background:#1c1c1e;}.md-text{max-width:100%;}</style>
</head>
<body><article class="md-text content">${bubbleHtml.value}</article></body>
</html>`;
});

const iconBtnCls =
"p-1.5 rounded-md text-text-tertiary hover:text-foreground hover:bg-accent transition-colors";
</script>

<template>
 <div
 class="flex flex-wrap items-center gap-0.5 border border-border rounded-xl bg-card px-2 py-1.5"
 >
 <!-- 「+」插入下拉（Stellar 标签插件；与工具栏同风格的方形图标按钮） -->
 <div ref="dropdownRef" class="relative">
 <button
 type="button"
 title="插入 Stellar 标签"
 :class="[
 'p-1.5 rounded-md transition-colors',
 insertOpen
 ? 'bg-accent text-foreground'
 : 'text-text-tertiary hover:text-foreground hover:bg-accent',
 ]"
 @click="toggleInsert"
 >
 <Icon icon="ri:add-line" class="text-lg" />
 </button>

 <div
 v-if="insertOpen"
 class="absolute left-0 top-full z-30 mt-1 flex items-start"
 >
 <!-- 菜单（对照 Halo 插入菜单：icon + 文字列表） -->
 <div
 class="insert-menu max-h-96 w-52 overflow-y-auto rounded-lg border border-border bg-card py-1 shadow-lg"
 >
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
 >
 <span class="flex-1 truncate">{{ p.title }}</span>
 <code class="text-[10px] text-text-quaternary">{{ p.name }}</code>
 </button>
 </template>
 </div>

 <!-- 悬停预览气泡（右侧弹出，iframe 渲染前台真实效果） -->
 <div
 v-if="bubblePlugin"
 class="ml-2 w-80 overflow-hidden rounded-lg border border-border bg-card shadow-xl"
 :style="{ marginTop: bubbleTop + 'px' }"
 >
 <div class="flex items-center justify-between border-b border-border px-3 py-1.5">
 <span class="text-xs font-medium text-text-secondary">{{ bubblePlugin.title }} 预览</span>
 <span v-if="bubbleLoading" class="text-[10px] text-text-quaternary">渲染中…</span>
 </div>
 <iframe
 v-if="bubbleHtml"
 :srcdoc="bubbleSrcdoc"
 class="block w-full border-0 bg-card"
 style="min-height: 180px; max-height: 360px; resize: vertical; overflow: auto;"
 sandbox="allow-same-origin allow-scripts"
 title="标签预览"
 ></iframe>
 <p v-else class="px-3 py-6 text-center text-xs text-text-quaternary">渲染中…</p>
 </div>
 </div>
 </div>

 <span class="mx-1 h-5 w-px bg-border"></span>

 <!-- 格式化按钮组（语义化 format 事件；命中态高亮） -->
 <button
 v-for="tool in formatTools"
 :key="tool.id"
 type="button"
 :title="tool.title"
 :class="[
 'p-1.5 rounded-md transition-colors',
 props.activeStates?.[tool.id]
 ? 'bg-accent text-primary'
 : 'text-text-tertiary hover:text-foreground hover:bg-accent',
 ]"
 @click="emit('format', tool.id)"
 >
 <Icon :icon="tool.icon" class="text-lg" />
 </button>

 <span class="mx-1 h-5 w-px bg-border"></span>

 <button type="button" title="上传图片" :class="iconBtnCls" @click="emit('upload')">
 <Icon icon="ri:image-add-line" class="text-lg" />
 </button>
 <button type="button" title="从图床选择" :class="iconBtnCls" @click="emit('pick')">
 <Icon icon="ri:gallery-line" class="text-lg" />
 </button>

 <span class="flex-1"></span>

 <!-- AI 属性生成 -->
 <button
 type="button"
 title="AI 属性生成"
 class="p-1.5 rounded-md text-system-indigo hover:bg-accent transition-colors"
 @click="emit('openAi')"
 >
 <Icon icon="ri:sparkling-2-line" class="text-lg" />
 </button>

 <!-- 预览（新标签页前台渲染） -->
 <button type="button" title="发布前预览" :class="iconBtnCls" @click="emit('preview')">
 <Icon icon="ri:eye-line" class="text-lg" />
 </button>

 <!-- 版本历史（仅编辑态，仅图标） -->
 <button v-if="props.isEdit" type="button" title="版本历史" :class="iconBtnCls" @click="emit('openRevision')">
 <Icon icon="ri:history-line" class="text-lg" />
 </button>

 <!-- 全屏编辑 -->
 <button
 type="button"
 :title="props.fullscreen ? '退出全屏' : '全屏编辑'"
 :class="[
 'p-1.5 rounded-md transition-colors',
 props.fullscreen
 ? 'bg-accent text-foreground'
 : 'text-text-tertiary hover:text-foreground hover:bg-accent',
 ]"
 @click="emit('toggleFullscreen')"
 >
 <Icon :icon="props.fullscreen ? 'ri:fullscreen-exit-line' : 'ri:fullscreen-line'" class="text-lg" />
 </button>

 <!-- 编辑 / 分屏 / 预览 三态切换（仅 Markdown 模式；富文本本身即所见即得） -->
 <div v-if="props.kind === 'markdown'" class="mx-1 flex items-center gap-0.5 rounded-lg bg-secondary p-0.5">
 <button
 v-for="btn in modeButtons"
 :key="btn.value"
 type="button"
 :title="btn.title"
 :class="[
 'p-1.5 rounded-md text-base transition-colors',
 props.mode === btn.value
 ? 'bg-card text-primary shadow-sm'
 : 'text-text-tertiary hover:text-foreground',
 ]"
 @click="emit('update:mode', btn.value)"
 >
 <Icon :icon="btn.icon" />
 </button>
 </div>

 <!-- 右侧面板开关（对照 Halo 工具栏最右图标） -->
 <button
 type="button"
 :title="props.sidebarOpen ? '收起右侧面板' : '展开右侧面板'"
 :class="[
 'p-1.5 rounded-md transition-colors',
 props.sidebarOpen
 ? 'bg-accent text-foreground'
 : 'text-text-tertiary hover:text-foreground hover:bg-accent',
 ]"
 @click="emit('toggleSidebar')"
 >
 <Icon icon="ri:layout-right-line" class="text-lg" />
 </button>
 </div>
</template>
