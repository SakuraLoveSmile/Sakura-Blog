<script setup lang="ts">
import { ref, computed } from "vue";
import { Icon } from "@iconify/vue";

interface MenuItem {
  id: string;
  title: string;
  url: string;
  icon: string;
  theme: string;
}

const props = defineProps<{
  items: MenuItem[];
  columns: number;
}>();

const emit = defineEmits<{
  (e: "update:columns", v: number): void;
  (e: "save"): void;
}>();

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50";
const addBtnCls =
  "inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300";

// 拖拽状态
const dragIndex = ref<number | null>(null);

function onDragStart(i: number) {
  dragIndex.value = i;
}
function onDragOver(e: DragEvent, i: number) {
  e.preventDefault();
  if (dragIndex.value === null || dragIndex.value === i) return;
  const arr = props.items;
  const [moved] = arr.splice(dragIndex.value, 1);
  arr.splice(i, 0, moved);
  dragIndex.value = i;
}
function onDragEnd() {
  dragIndex.value = null;
}

function moveUp(i: number) {
  if (i <= 0) return;
  const arr = props.items;
  [arr[i - 1], arr[i]] = [arr[i], arr[i - 1]];
}
function moveDown(i: number) {
  const arr = props.items;
  if (i >= arr.length - 1) return;
  [arr[i], arr[i + 1]] = [arr[i + 1], arr[i]];
}

function addItem() {
  props.items.push({ id: "", title: "", url: "", icon: "", theme: "" });
}
function removeItem(i: number) {
  props.items.splice(i, 1);
}

// 校验
const errors = computed(() => {
  const errs: Record<number, string[]> = {};
  const ids = new Set<string>();
  props.items.forEach((it, i) => {
    const e: string[] = [];
    if (!it.id.trim()) e.push("id 必填");
    else if (ids.has(it.id)) e.push("id 重复");
    else ids.add(it.id);
    if (!it.title.trim()) e.push("标题必填");
    if (!it.url.trim()) e.push("URL 必填");
    else if (!it.url.startsWith("/") && !it.url.startsWith("http"))
      e.push("URL 须以 / 或 http 开头");
    if (it.theme && !/^#[0-9a-fA-F]{6}$/.test(it.theme))
      e.push("色值须为 #RRGGBB");
    if (e.length) errs[i] = e;
  });
  return errs;
});
const hasErrors = computed(() => Object.keys(errors.value).length > 0);

// 判断 icon key 是否为 Iconify 格式（含冒号）
function isIconify(key: string) {
  return key.includes(":");
}
</script>

<template>
  <div class="space-y-4">
    <div class="w-40">
      <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">菜单列数</label>
      <input
        :value="columns"
        @input="emit('update:columns', Number(($event.target as HTMLInputElement).value))"
        type="number"
        min="1"
        :class="inputCls"
      />
    </div>

    <div class="space-y-3">
      <div
        v-for="(it, i) in items"
        :key="i"
        draggable="true"
        @dragstart="onDragStart(i)"
        @dragover="onDragOver($event, i)"
        @dragend="onDragEnd"
        :class="[
          'grid grid-cols-12 gap-2 items-start p-3 rounded-lg border transition-colors cursor-grab active:cursor-grabbing',
          errors[i]
            ? 'border-red-300 bg-red-50/50 dark:border-red-500/40 dark:bg-red-500/5'
            : 'border-gray-200 dark:border-gray-700',
          dragIndex === i ? 'opacity-50' : '',
        ]"
      >
        <!-- 拖拽手柄 + 排序 -->
        <div class="col-span-1 flex flex-col items-center gap-0.5 pt-2">
          <Icon icon="ri:drag-move-2-fill" class="text-gray-300 dark:text-gray-600 text-lg" />
          <div class="flex gap-0.5">
            <button @click="moveUp(i)" :disabled="i === 0" class="text-gray-400 hover:text-gray-600 disabled:opacity-20 dark:text-gray-500">
              <Icon icon="ri:arrow-up-s-line" />
            </button>
            <button @click="moveDown(i)" :disabled="i === items.length - 1" class="text-gray-400 hover:text-gray-600 disabled:opacity-20 dark:text-gray-500">
              <Icon icon="ri:arrow-down-s-line" />
            </button>
          </div>
        </div>

        <!-- id -->
        <div class="col-span-2">
          <label class="block text-[10px] text-gray-400 mb-0.5">ID</label>
          <input v-model="it.id" placeholder="post" :class="inputCls" />
        </div>

        <!-- title -->
        <div class="col-span-2">
          <label class="block text-[10px] text-gray-400 mb-0.5">标题</label>
          <input v-model="it.title" placeholder="博客" :class="inputCls" />
        </div>

        <!-- url -->
        <div class="col-span-2">
          <label class="block text-[10px] text-gray-400 mb-0.5">URL</label>
          <input v-model="it.url" placeholder="/" :class="inputCls" />
        </div>

        <!-- icon + preview -->
        <div class="col-span-3">
          <label class="block text-[10px] text-gray-400 mb-0.5">图标 (Iconify key)</label>
          <div class="flex items-center gap-1.5">
            <Icon v-if="isIconify(it.icon)" :icon="it.icon" class="text-lg shrink-0 text-gray-600 dark:text-gray-300" />
            <span v-else class="w-5 h-5 flex items-center justify-center text-[10px] text-gray-300 border border-dashed border-gray-300 rounded dark:border-gray-600">?</span>
            <input v-model="it.icon" placeholder="solar:documents-bold-duotone" :class="inputCls" />
          </div>
        </div>

        <!-- theme color -->
        <div class="col-span-1">
          <label class="block text-[10px] text-gray-400 mb-0.5">配色</label>
          <div class="flex items-center gap-1">
            <input
              type="color"
              :value="it.theme || '#6366f1'"
              @input="it.theme = ($event.target as HTMLInputElement).value"
              class="w-7 h-7 rounded border border-gray-200 dark:border-gray-600 cursor-pointer p-0.5"
            />
          </div>
          <input v-model="it.theme" placeholder="#hex" class="mt-1 w-full px-1.5 py-1 text-[10px] border border-gray-200 rounded focus:outline-none dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100" />
        </div>

        <!-- delete -->
        <div class="col-span-1 flex justify-center pt-5">
          <button @click="removeItem(i)" class="text-red-400 hover:text-red-600 dark:text-red-500 dark:hover:text-red-400">
            <Icon icon="ri:delete-bin-line" class="text-lg" />
          </button>
        </div>

        <!-- errors -->
        <div v-if="errors[i]" class="col-span-12 -mt-1">
          <p class="text-[11px] text-red-500 dark:text-red-400">{{ errors[i].join("；") }}</p>
        </div>
      </div>
    </div>

    <button @click="addItem" :class="addBtnCls">
      <Icon icon="ri:add-line" /> 添加菜单项
    </button>

    <div class="flex items-center gap-3">
      <button @click="emit('save')" :disabled="hasErrors" :class="btnCls">保存菜单</button>
      <span v-if="hasErrors" class="text-xs text-red-500 dark:text-red-400">请修正上方标红项后保存</span>
    </div>
  </div>
</template>
