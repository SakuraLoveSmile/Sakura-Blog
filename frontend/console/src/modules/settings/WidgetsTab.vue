<script setup lang="ts">
import { ref, computed } from"vue";
import { Icon } from"@iconify/vue";
import {
  BUILTIN_WIDGET_NAMES,
  builtinLayoutOf,
  fieldsForLayout,
  NO_EXTRA_PARAM_LAYOUTS,
} from"./widget-meta";

const props = defineProps<{
 data: Record<string, any>;
 saving: string;
}>();
const emit = defineEmits<{
 save: [section: string];
}>();

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-foreground text-sm font-medium rounded-full transition-colors hover:bg-accent disabled:opacity-50";
const rowBtnCls =
"text-text-quaternary hover:text-text-tertiary hover:text-foreground disabled:opacity-30";
const addBtnCls =
"inline-flex items-center gap-1 text-sm text-primary transition-colors hover:text-primary/80";

// ---------- 页面类型 ----------
// 分组对齐后端 PageTypeOf 枚举；文库/笔记本/友链/瞬间等类型此前 UI 未暴露，
// 导致这些页面的左右栏编排无入口（渲染时回退 index 或内置缺省）。
const pageGroups: { label: string; options: { key: string; label: string }[] }[] = [
 {
  label: "内容",
  options: [
   { key:"index", label:"首页" },
   { key:"post", label:"文章页" },
   { key:"page", label:"独立页" },
   { key:"archive", label:"归档页" },
   { key:"categories", label:"分类页" },
   { key:"tags", label:"标签页" },
  ],
 },
 {
  label: "文库",
  options: [
   { key:"wiki", label:"文库列表页" },
   { key:"wiki-doc", label:"文库文档页" },
  ],
 },
 {
  label: "笔记本",
  options: [
   { key:"notebooks", label:"笔记本列表页" },
   { key:"notebook", label:"笔记本页" },
   { key:"note", label:"笔记页" },
  ],
 },
 {
  label: "互动",
  options: [
   { key:"friends", label:"友链页" },
   { key:"moments", label:"瞬间页" },
   { key:"topic", label:"专栏页" },
  ],
 },
 {
  label: "作者",
  options: [{ key:"author", label:"作者页" }],
 },
];
const pageOptions = computed(() => pageGroups.flatMap((g) => g.options));
const selectedPage = ref("index");

// ---------- 数据访问 ----------
// 内置组件元数据由 widget-meta.ts 统一提供（与后端 DefaultWidgetLibrary 保持一致）。
// 修复：添加组件下拉此前只读用户自定义 library（未自定义时为空）导致"选不了"，
// 现以内置组件兜底，保证任何时候都有可选项；非内置名条目为用户保存的"方案"（Spec 58）。
function ensureWidgets() {
 const d = props.data;
 d.widgets = d.widgets || {};
 d.widgets.library = d.widgets.library || {};
 d.widgets.pages = d.widgets.pages || {};
 return d.widgets;
}

const library = computed(() => {
 const w = ensureWidgets();
 return w.library as Record<string, any>;
});

const pages = computed(() => {
 const w = ensureWidgets();
 return w.pages as Record<string, { leftbar: string[]; rightbar: string[] }>;
});

/** 当前页面类型是否已有自定义配置 */
const hasCustomConfig = computed(() => !!pages.value[selectedPage.value]);

function currentBarWidgets(side:"leftbar" |"rightbar"): string[] {
 if (!pages.value[selectedPage.value]) return [];
 return pages.value[selectedPage.value][side] || [];
}

/** 创建空序列（"自定义"按钮） */
function customise() {
 pages.value[selectedPage.value] = { leftbar: [], rightbar: [] };
}

/** 恢复默认 */
function resetDefaults() {
 delete pages.value[selectedPage.value];
}

// ---------- 拖拽排序（复用 MenubarEditor 模式） ----------
const dragIndex = ref<number | null>(null);
const dragSide = ref<"leftbar" |"rightbar" | null>(null);

function onDragStart(side:"leftbar" |"rightbar", i: number) {
 dragIndex.value = i;
 dragSide.value = side;
}
function onDragOver(e: DragEvent, side:"leftbar" |"rightbar", i: number) {
 e.preventDefault();
 if (dragIndex.value === null || dragSide.value !== side || dragIndex.value === i) return;
 const arr = currentBarWidgets(side);
 const [moved] = arr.splice(dragIndex.value, 1);
 arr.splice(i, 0, moved);
 dragIndex.value = i;
}
function onDragEnd() {
 dragIndex.value = null;
 dragSide.value = null;
}

function moveItem(arr: string[], i: number, dir: -1 | 1) {
 const j = i + dir;
 if (j < 0 || j >= arr.length) return;
 [arr[i], arr[j]] = [arr[j], arr[i]];
}

// ---------- 添加 / 移除 ----------
const addSelectLeft = ref("");
const addSelectRight = ref("");

/** 当前栏中尚未添加的 widget 列表：内置组件 + 自定义 library（含方案），排除已用 */
function availableWidgets(side:"leftbar" |"rightbar"): string[] {
 const used = new Set(currentBarWidgets(side));
 const names = new Set<string>([...BUILTIN_WIDGET_NAMES, ...Object.keys(library.value)]);
 return [...names].filter((k) => !used.has(k)).sort();
}

/** 非内置名的 library 条目 = 组件库保存的方案（Spec 58） */
function isScheme(name: string): boolean {
 return !BUILTIN_WIDGET_NAMES.has(name);
}

function addWidget(side:"leftbar" |"rightbar") {
 const sel = side ==="leftbar" ? addSelectLeft.value : addSelectRight.value;
 if (!sel) return;
 if (!pages.value[selectedPage.value]) customise();
 const bar = currentBarWidgets(side);
 bar.push(sel);
 // 确保 library 有定义（内置组件用其默认 layout，避免 welcome 等被写成同名 layout）
 if (!library.value[sel]) {
 library.value[sel] = { layout: builtinLayoutOf(sel) || sel };
 }
 if (side ==="leftbar") addSelectLeft.value ="";
 else addSelectRight.value ="";
}

function removeWidget(side:"leftbar" |"rightbar", i: number) {
 currentBarWidgets(side).splice(i, 1);
}

// ---------- 展开参数编辑 ----------
const expandedWidget = ref<string | null>(null);

function toggleExpand(name: string) {
 expandedWidget.value = expandedWidget.value === name ? null : name;
}

/** 参数字段 schema（widget-meta 按 layout 提供，自定义组件与内置组件共享同 layout 时同样命中） */
function widgetLayout(name: string): string {
 return library.value[name]?.layout || builtinLayoutOf(name) ||"";
}

function widgetFields(name: string) {
 return fieldsForLayout(widgetLayout(name)) || null;
}

function isNoExtraParams(name: string) {
 return NO_EXTRA_PARAM_LAYOUTS.has(widgetLayout(name));
}

// JSON 兜底 textarea
function jsonText(name: string): string {
 return JSON.stringify(library.value[name], null, 2);
}
function setJsonText(name: string, val: string) {
 try {
 library.value[name] = JSON.parse(val);
 } catch {
 // 输入中暂不更新
 }
}
</script>

<template>
 <div class="space-y-6">
 <!-- 页面类型选择器 -->
 <div class="w-64">
 <label class="block text-xs text-text-tertiary mb-1">页面类型</label>
 <select v-model="selectedPage" :class="inputCls">
 <optgroup v-for="g in pageGroups" :key="g.label" :label="g.label">
 <option v-for="o in g.options" :key="o.key" :value="o.key">{{ o.label }}</option>
 </optgroup>
 </select>
 </div>

 <!-- 未自定义提示 -->
 <div v-if="!hasCustomConfig" class="flex items-center gap-3 border border-dashed border-border-strong rounded-xl p-4">
 <div class="text-sm text-text-tertiary">
 当前「{{ pageOptions.find(o => o.key === selectedPage)?.label }}」使用默认布局
 </div>
 <button @click="customise" :class="btnCls" class="py-1.5! px-3! text-xs!">自定义</button>
 </div>

 <!-- 左右栏编排 -->
 <template v-if="hasCustomConfig">
 <div class="grid grid-cols-2 gap-6">
 <!-- 左栏 -->
 <div class="space-y-3">
 <h4 class="text-sm font-medium text-text-secondary">左栏 (leftbar)</h4>
 <div
 v-for="(name, i) in currentBarWidgets('leftbar')"
 :key="'l-' + i"
 draggable="true"
 @dragstart="onDragStart('leftbar', i)"
 @dragover="onDragOver($event, 'leftbar', i)"
 @dragend="onDragEnd"
 :class="[
 'flex items-center gap-2 p-2.5 rounded-lg border border-border cursor-grab active:cursor-grabbing transition-colors ',
 dragIndex === i && dragSide === 'leftbar' ? 'opacity-50' : '',
 expandedWidget === name ? 'ring-1 ring-primary/40' : '',
 ]"
 >
 <Icon icon="ri:drag-move-2-fill" class="text-text-quaternary text-base shrink-0" />
 <div class="flex-1 min-w-0 cursor-pointer" @click="toggleExpand(name)">
 <span class="text-sm font-medium text-text-secondary">{{ name }}</span>
 <span class="ml-1.5 text-xs text-text-quaternary">{{ widgetLayout(name) }}</span>
 <span v-if="isScheme(name)" class="ml-1.5 inline-flex items-center gap-0.5 rounded-full bg-primary/10 px-1.5 py-px text-[10px] text-primary align-middle"><Icon icon="ri:bookmark-line" />方案</span>
 </div>
 <div class="flex items-center gap-1 shrink-0">
 <button @click="moveItem(currentBarWidgets('leftbar'), i, -1)" :disabled="i === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
 <button @click="moveItem(currentBarWidgets('leftbar'), i, 1)" :disabled="i === currentBarWidgets('leftbar').length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
 <button @click="removeWidget('leftbar', i)" class="text-state-error hover:text-state-error text-state-error hover:text-state-error"><Icon icon="ri:delete-bin-line" /></button>
 </div>
 </div>
 <!-- 添加左栏组件 -->
 <div class="flex items-center gap-2">
 <select v-model="addSelectLeft" :class="inputCls" class="flex-1">
 <option value="" disabled>选择组件...</option>
 <option v-for="w in availableWidgets('leftbar')" :key="w" :value="w">{{ isScheme(w) ? "方案 · " : "" }}{{ w }} ({{ widgetLayout(w) || w }})</option>
 </select>
 <button @click="addWidget('leftbar')" :disabled="!addSelectLeft" :class="addBtnCls"><Icon icon="ri:add-line" /></button>
 </div>
 </div>

 <!-- 右栏 -->
 <div class="space-y-3">
 <h4 class="text-sm font-medium text-text-secondary">右栏 (rightbar)</h4>
 <div
 v-for="(name, i) in currentBarWidgets('rightbar')"
 :key="'r-' + i"
 draggable="true"
 @dragstart="onDragStart('rightbar', i)"
 @dragover="onDragOver($event, 'rightbar', i)"
 @dragend="onDragEnd"
 :class="[
 'flex items-center gap-2 p-2.5 rounded-lg border border-border cursor-grab active:cursor-grabbing transition-colors ',
 dragIndex === i && dragSide === 'rightbar' ? 'opacity-50' : '',
 expandedWidget === name ? 'ring-1 ring-primary/40' : '',
 ]"
 >
 <Icon icon="ri:drag-move-2-fill" class="text-text-quaternary text-base shrink-0" />
 <div class="flex-1 min-w-0 cursor-pointer" @click="toggleExpand(name)">
 <span class="text-sm font-medium text-text-secondary">{{ name }}</span>
 <span class="ml-1.5 text-xs text-text-quaternary">{{ widgetLayout(name) }}</span>
 <span v-if="isScheme(name)" class="ml-1.5 inline-flex items-center gap-0.5 rounded-full bg-primary/10 px-1.5 py-px text-[10px] text-primary align-middle"><Icon icon="ri:bookmark-line" />方案</span>
 </div>
 <div class="flex items-center gap-1 shrink-0">
 <button @click="moveItem(currentBarWidgets('rightbar'), i, -1)" :disabled="i === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
 <button @click="moveItem(currentBarWidgets('rightbar'), i, 1)" :disabled="i === currentBarWidgets('rightbar').length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
 <button @click="removeWidget('rightbar', i)" class="text-state-error hover:text-state-error text-state-error hover:text-state-error"><Icon icon="ri:delete-bin-line" /></button>
 </div>
 </div>
 <!-- 添加右栏组件 -->
 <div class="flex items-center gap-2">
 <select v-model="addSelectRight" :class="inputCls" class="flex-1">
 <option value="" disabled>选择组件...</option>
 <option v-for="w in availableWidgets('rightbar')" :key="w" :value="w">{{ isScheme(w) ? "方案 · " : "" }}{{ w }} ({{ widgetLayout(w) || w }})</option>
 </select>
 <button @click="addWidget('rightbar')" :disabled="!addSelectRight" :class="addBtnCls"><Icon icon="ri:add-line" /></button>
 </div>
 </div>
 </div>

 <!-- 恢复默认 -->
 <div class="flex items-center gap-3">
 <button @click="resetDefaults" class="px-3 py-1.5 text-xs rounded-full border border-border-strong text-text-tertiary hover:bg-accent">
 恢复默认
 </button>
 <span class="text-xs text-text-quaternary">删除自定义序列，回退到缺省布局</span>
 </div>
 </template>

 <!-- 组件参数编辑区（展开） -->
 <div v-if="expandedWidget && library[expandedWidget]" class="border border-border rounded-xl p-4 space-y-3">
 <h4 class="text-sm font-medium text-text-secondary">
 组件参数：<span class="text-primary">{{ expandedWidget }}</span>
 <span class="ml-1 text-xs text-text-quaternary">layout: {{ widgetLayout(expandedWidget) }}</span>
 </h4>

 <!-- 已知参数字段 -->
 <template v-if="widgetFields(expandedWidget)">
 <div v-for="f in widgetFields(expandedWidget)!" :key="f.key" class="max-w-xs">
 <label class="block text-xs text-text-tertiary mb-1">{{ f.label || f.key }}</label>
 <input
 v-if="f.type === 'checkbox'"
 type="checkbox"
 :checked="!!library[expandedWidget][f.key]"
 @change="library[expandedWidget][f.key] = ($event.target as HTMLInputElement).checked"
 class="rounded border-border-strong"
 />
 <input
 v-else-if="f.type === 'number'"
 type="number"
 :value="library[expandedWidget][f.key]"
 @input="library[expandedWidget][f.key] = Number(($event.target as HTMLInputElement).value)"
 :class="inputCls"
 />
 <select
 v-else-if="f.type === 'select'"
 :value="library[expandedWidget][f.key]"
 @change="library[expandedWidget][f.key] = ($event.target as HTMLSelectElement).value"
 :class="inputCls"
 >
 <option v-for="o in f.options" :key="o" :value="o">{{ o }}</option>
 </select>
 <textarea
 v-else-if="f.type === 'textarea'"
 rows="4"
 :value="library[expandedWidget][f.key]"
 @input="library[expandedWidget][f.key] = ($event.target as HTMLTextAreaElement).value"
 :class="inputCls + ' font-mono text-xs'"
 ></textarea>
 <input
 v-else
 type="text"
 :value="library[expandedWidget][f.key]"
 @input="library[expandedWidget][f.key] = ($event.target as HTMLInputElement).value"
 :class="inputCls"
 />
 </div>
 </template>

 <!-- 无额外参数的组件 -->
 <p v-else-if="isNoExtraParams(expandedWidget)" class="text-xs text-text-quaternary">该组件无额外可配置参数。</p>

 <!-- 未知 layout：JSON 兜底 -->
 <div v-else>
 <label class="block text-xs text-text-tertiary mb-1">参数 (JSON)</label>
 <textarea
 :value="jsonText(expandedWidget)"
 @input="setJsonText(expandedWidget, ($event.target as HTMLTextAreaElement).value)"
 rows="6"
 :class="inputCls + ' font-mono text-xs'"
 ></textarea>
 </div>
 </div>

 <!-- 保存按钮 -->
 <div>
 <button @click="emit('save', 'widgets')" :disabled="saving === 'widgets'" :class="btnCls">保存组件设置</button>
 </div>
 </div>
</template>
