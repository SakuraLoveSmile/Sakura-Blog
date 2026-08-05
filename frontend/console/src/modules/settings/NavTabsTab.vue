<script setup lang="ts">
import { Icon } from "@iconify/vue";

// 导航栏标签页（site.yml nav_tabs 段入库版）：
// enable_custom=true 时导航栏完全由 items 定义；false 时固定 tab + items 追加。
const props = defineProps<{
  data: any;
  saving: string;
}>();

const emit = defineEmits<{ (e: "save"): void }>();

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
const addBtnCls =
  "inline-flex items-center gap-1 text-sm text-secondary hover:text-primary transition-colors dark:text-primary dark:hover:text-primary/80";
const rowBtnCls = "text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 disabled:opacity-30";

function addItem() {
  props.data.items.push({ title: "", url: "", external: false, icon: "", active_rules: "" });
}
function removeAt(i: number) {
  props.data.items.splice(i, 1);
}
function moveItem(i: number, dir: -1 | 1) {
  const j = i + dir;
  const arr = props.data.items;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}
</script>

<template>
  <div class="space-y-4">
    <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
      <input type="checkbox" v-model="data.enable_custom" class="rounded" />
      完全由下方列表驱动导航栏（关闭时保留「近期发布/分类/标签/归档」固定 tab，列表项追加在后）
    </label>

    <div class="space-y-2">
      <p class="text-xs text-gray-400 dark:text-gray-500">导航标签（active_rules 为路径前缀/正则，留空按 URL 精确匹配高亮）</p>
      <div
        v-for="(it, i) in data.items"
        :key="i"
        class="border border-gray-200 rounded-xl p-4 space-y-2 dark:border-gray-700"
      >
        <div class="grid grid-cols-12 gap-2 items-center">
          <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-3" />
          <input v-model="it.url" placeholder="/url 或 https://..." :class="inputCls" class="col-span-4" />
          <input v-model="it.icon" placeholder="图标 key（可选）" :class="inputCls" class="col-span-3" />
          <div class="col-span-2 flex items-center gap-1.5 justify-end">
            <button @click="moveItem(Number(i), -1)" :disabled="Number(i) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
            <button @click="moveItem(Number(i), 1)" :disabled="Number(i) === data.items.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
            <button @click="removeAt(Number(i))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
          </div>
        </div>
        <div class="grid grid-cols-12 gap-2 items-center">
          <input v-model="it.active_rules" placeholder="active_rules（可选，如 /categories/技术/）" :class="inputCls" class="col-span-10" />
          <label class="col-span-2 flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400">
            <input type="checkbox" v-model="it.external" class="rounded" /> 外链
          </label>
        </div>
      </div>
      <button @click="addItem" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加标签</button>
    </div>

    <div><button @click="emit('save')" :disabled="saving === 'nav_tabs'" :class="btnCls">保存导航标签</button></div>
  </div>
</template>
