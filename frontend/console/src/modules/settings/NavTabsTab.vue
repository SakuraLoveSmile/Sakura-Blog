<script setup lang="ts">
import { Icon } from"@iconify/vue";
import IconPicker from"@/components/IconPicker.vue";

// 导航栏标签页（site.yml nav_tabs 段入库版）：
// enable_custom=true 时导航栏完全由 items 定义；false 时固定 tab + items 追加。
const props = defineProps<{
 data: any;
 saving: string;
}>();

const emit = defineEmits<{ (e:"save"): void }>();

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-foreground text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
const addBtnCls =
"inline-flex items-center gap-1 text-sm text-primary transition-colors hover:text-primary/80";
const rowBtnCls ="text-text-quaternary hover:text-text-tertiary hover:text-foreground disabled:opacity-30";

function addItem() {
 // Spec 62：前端专用稳定 key（增删列表 v-for 不复用错位）
 props.data.items.push({ title:"", url:"", external: false, icon:"", active_rules:"", _key: crypto.randomUUID() });
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
 <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
 <input type="checkbox" v-model="data.enable_custom" class="rounded" />
 完全由下方列表驱动导航栏（关闭时保留「近期发布/分类/标签/归档」固定 tab，列表项追加在后）
 </label>

 <div class="space-y-2">
 <p class="text-xs text-text-quaternary">导航标签（active_rules 为路径前缀/正则，留空按 URL 精确匹配高亮）</p>
 <div
 v-for="(it, i) in data.items"
 :key="it._key || i"
 class="border border-border rounded-xl p-4 space-y-2"
 >
 <div class="grid grid-cols-12 gap-2 items-center">
 <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-3" />
 <input v-model="it.url" placeholder="/url 或 https://..." :class="inputCls" class="col-span-4" />
 <div class="col-span-3">
 <IconPicker v-model="it.icon" compact placeholder="图标 key（可选）" hint="该图标当前前台暂未渲染（数据预留）" />
 </div>
 <div class="col-span-2 flex items-center gap-1.5 justify-end">
 <button @click="moveItem(Number(i), -1)" :disabled="Number(i) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
 <button @click="moveItem(Number(i), 1)" :disabled="Number(i) === data.items.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
 <button @click="removeAt(Number(i))" class="text-state-error hover:text-state-error"><Icon icon="ri:delete-bin-line" /></button>
 </div>
 </div>
 <div class="grid grid-cols-12 gap-2 items-center">
 <input v-model="it.active_rules" placeholder="active_rules（可选，如 /categories/技术/）" :class="inputCls" class="col-span-10" />
 <label class="col-span-2 flex items-center gap-1.5 text-sm text-text-tertiary">
 <input type="checkbox" v-model="it.external" class="rounded" /> 外链
 </label>
 </div>
 </div>
 <button @click="addItem" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加标签</button>
 </div>

 <div><button @click="emit('save')" :disabled="saving === 'nav_tabs'" :class="btnCls">保存导航标签</button></div>
 </div>
</template>
