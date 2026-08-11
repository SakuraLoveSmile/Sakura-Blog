<script setup lang="ts">
// 前端体验增强插件（Spec 36 段入库版）：开关 + 参数表单。
// 数据结构对齐 site.yml plugins 段；子配置为指针，未配置=关闭（JSON 缺省空对象等价）。
const props = defineProps<{
 data: any;
 saving: string;
}>();

const emit = defineEmits<{ (e:"save"): void }>();

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
</script>

<template>
 <div class="space-y-6">
 <!-- 预加载 -->
 <div class="border border-border rounded-xl p-4 space-y-3">
 <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
 <input type="checkbox" v-model="data.preload.enable" class="rounded" />
 页面资源预加载（Flying Pages）
 </label>
 <div class="grid grid-cols-2 gap-4">
 <div>
 <label class="text-xs text-text-quaternary mb-1 block">服务（预留字段，当前仅支持内置 flying_pages，留空即可）</label>
 <input v-model="data.preload.service" :class="inputCls" placeholder="留空即可" />
 </div>
 </div>
 </div>

 <!-- 滚动渐现 -->
 <div class="border border-border rounded-xl p-4 space-y-3">
 <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
 <input type="checkbox" v-model="data.scrollreveal.enable" class="rounded" />
 滚动渐现动画（ScrollReveal）
 </label>
 <div class="grid grid-cols-3 gap-4">
 <div>
 <label class="text-xs text-text-tertiary mb-1 block">距离</label>
 <input v-model="data.scrollreveal.distance" :class="inputCls" placeholder="60px" />
 </div>
 <div>
 <label class="text-xs text-text-tertiary mb-1 block">时长（ms）</label>
 <input v-model.number="data.scrollreveal.duration" type="number" :class="inputCls" placeholder="600" />
 </div>
 <div>
 <label class="text-xs text-text-tertiary mb-1 block">间隔（ms）</label>
 <input v-model.number="data.scrollreveal.interval" type="number" :class="inputCls" placeholder="100" />
 </div>
 </div>
 </div>

 <!-- 中文排版 -->
 <div class="border border-border rounded-xl p-4 space-y-3">
 <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
 <input type="checkbox" v-model="data.heti.enable" class="rounded" />
 中文排版优化（赫蹏 Heti）
 </label>
 </div>

 <!-- 图片灯箱 -->
 <div class="border border-border rounded-xl p-4 space-y-3">
 <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
 <input type="checkbox" v-model="data.fancybox.enable" class="rounded" />
 图片灯箱（Fancybox）
 </label>
 <div>
 <label class="text-xs text-text-tertiary mb-1 block">选择器（空用默认 div.lazy-box&gt;img.lazy）</label>
 <input v-model="data.fancybox.selector" :class="inputCls" placeholder="div.lazy-box>img.lazy" />
 </div>
 </div>

 <button @click="emit('save')" :disabled="saving === 'plugins'" :class="btnCls">
 保存插件设置
 </button>
 </div>
</template>
