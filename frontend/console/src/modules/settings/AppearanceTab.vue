<script setup lang="ts">
const props = defineProps<{
 data: Record<string, any>;
 saving: string;
}>();
const emit = defineEmits<{
 save: [section: string];
}>();

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";

/** 从 px 字符串提取数值，如"16px" → 16 */
function pxNum(v: string | undefined): number {
 if (!v) return 16;
 return parseInt(String(v).replace("px",""), 10) || 16;
}
/** 数值转 px 字符串 */
function numPx(n: number): string {
 return n +"px";
}
/** header_prefix 文本编辑：合法 JSON 才写入，非法输入静默忽略（保留原值） */
function onHeaderPrefixInput(e: Event) {
 try {
 props.data.style.header_prefix = JSON.parse((e.target as HTMLTextAreaElement).value);
 } catch {
 // 非法 JSON：忽略本次输入，保留原值
 }
}
</script>

<template>
 <div class="space-y-4">
 <!-- 1. 主题色 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">主题色</h3>
 <div class="grid grid-cols-3 gap-4">
 <div>
 <label class="block text-xs text-text-tertiary mb-1">主题色 (theme)</label>
 <div class="flex gap-2 items-center">
 <input type="color" v-model="data.style.color.theme" class="w-10 h-10 rounded border border-border cursor-pointer" />
 <input v-model="data.style.color.theme" :class="inputCls" class="flex-1" placeholder="#2E8DFF" />
 </div>
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">强调色 (accent)</label>
 <div class="flex gap-2 items-center">
 <input type="color" v-model="data.style.color.accent" class="w-10 h-10 rounded border border-border cursor-pointer" />
 <input v-model="data.style.color.accent" :class="inputCls" class="flex-1" placeholder="#ff9800" />
 </div>
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">链接色 (link)</label>
 <div class="flex gap-2 items-center">
 <input type="color" v-model="data.style.color.link" class="w-10 h-10 rounded border border-border cursor-pointer" />
 <input v-model="data.style.color.link" :class="inputCls" class="flex-1" placeholder="#2196f3" />
 </div>
 </div>
 </div>
 </div>

 <!-- 2. 字体字号 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">字体字号</h3>
 <div class="grid grid-cols-2 gap-4">
 <div>
 <label class="block text-xs text-text-tertiary mb-1">根字号 (root)</label>
 <div class="flex items-center gap-3">
 <input type="range" min="12" max="20" step="1" :value="pxNum(data.style.font_size?.root)" @input="data.style.font_size.root = numPx(Number(($event.target as HTMLInputElement).value))" class="flex-1 accent-secondary" />
 <span class="text-sm text-text-tertiary w-12 text-right">{{ data.style.font_size.root || '16px' }}</span>
 </div>
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">正文字号 (body)</label>
 <div class="flex items-center gap-3">
 <input type="range" min="12" max="20" step="1" :value="pxNum(data.style.font_size?.body)" @input="data.style.font_size.body = numPx(Number(($event.target as HTMLInputElement).value))" class="flex-1 accent-secondary" />
 <span class="text-sm text-text-tertiary w-12 text-right">{{ data.style.font_size.body || '15px' }}</span>
 </div>
 </div>
 </div>
 </div>

 <!-- 3. 字体族 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">字体族</h3>
 <div class="grid grid-cols-3 gap-4">
 <div>
 <label class="block text-xs text-text-tertiary mb-1">正文字体 (body)</label>
 <input v-model="data.style.font_family.body" :class="inputCls" placeholder="留空使用默认字体" />
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">行内代码字体 (code)</label>
 <input v-model="data.style.font_family.code" :class="inputCls" placeholder="留空使用默认字体" />
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">代码块字体 (codeblock)</label>
 <input v-model="data.style.font_family.codeblock" :class="inputCls" placeholder="留空使用默认字体" />
 </div>
 </div>
 </div>

 <!-- 4. 左侧栏背景 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">左侧栏背景</h3>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">背景色</label>
 <div class="flex gap-2 items-center">
 <input type="color" v-model="data.style.leftbar.background_color_dark" class="w-10 h-10 rounded border border-border cursor-pointer" />
 <input v-model="data.style.leftbar.background_color_dark" :class="inputCls" class="flex-1" />
 </div>
 </div>
 <div class="mt-3">
 <label class="block text-xs text-text-tertiary mb-1">背景图片 URL</label>
 <div class="flex gap-2 items-start">
 <input v-model="data.style.leftbar.background_image" :class="inputCls" class="flex-1" placeholder="输入图片 URL" />
 <img v-if="data.style.leftbar.background_image" :src="data.style.leftbar.background_image" class="w-10 h-10 rounded object-cover border border-border" alt="" />
 </div>
 </div>
 <div class="grid grid-cols-2 gap-4 mt-3">
 <div>
 <label class="block text-xs text-text-tertiary mb-1">模糊半径 (blur_px)</label>
 <div class="flex items-center gap-3">
 <input type="range" min="0" max="20" step="1" v-model.number="data.style.leftbar.blur_px" class="flex-1 accent-secondary" />
 <span class="text-sm text-text-tertiary w-12 text-right">{{ data.style.leftbar.blur_px ?? 0 }}px</span>
 </div>
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">背景不透明度</label>
 <input v-model="data.style.leftbar.background_opacity" :class="inputCls" placeholder="1.0" />
 </div>
 </div>
 </div>

 <!-- 5. 全站背景 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">全站背景</h3>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">背景图片 URL</label>
 <div class="flex gap-2 items-start">
 <input v-model="data.style.site.background_image" :class="inputCls" class="flex-1" placeholder="输入图片 URL" />
 <img v-if="data.style.site.background_image" :src="data.style.site.background_image" class="w-10 h-10 rounded object-cover border border-border" alt="" />
 </div>
 </div>
 <div class="grid grid-cols-3 gap-4 mt-3">
 <div>
 <label class="block text-xs text-text-tertiary mb-1">模糊半径 (blur_px)</label>
 <input v-model="data.style.site.blur_px" :class="inputCls" placeholder="0" />
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">模糊饱和度 (blur_sat)</label>
 <input v-model="data.style.site.blur_sat" :class="inputCls" placeholder="1" />
 </div>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">模糊背景 (blur_bg)</label>
 <input v-model="data.style.site.blur_bg" :class="inputCls" placeholder="0" />
 </div>
 </div>
 </div>

 <!-- 6. 代码块 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">代码块</h3>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">高亮主题 URL (scrollbar)</label>
 <input v-model="data.style.codeblock.scrollbar" :class="inputCls" placeholder="代码块高亮主题 CSS URL" />
 </div>
 </div>

 <!-- 7. 杂项 -->
 <div class="mb-6">
 <h3 class="text-sm font-semibold text-text-secondary mb-3">杂项</h3>
 <div>
 <label class="block text-xs text-text-tertiary mb-1">标题前缀 (header_prefix) — JSON 格式</label>
 <textarea
 :value="JSON.stringify(data.style.header_prefix || {}, null, 2)"
 @input="onHeaderPrefixInput($event)"
 rows="3"
 :class="inputCls + ' font-mono text-xs'"
 placeholder='{"h2":"#","h3":"="}'
 ></textarea>
 </div>
 </div>

 <!-- 保存按钮 -->
 <div>
 <button @click="emit('save', 'style')" :disabled="saving === 'style'" :class="btnCls">
 {{ saving === 'style' ? '保存中...' : '保存外观设置' }}
 </button>
 </div>
 </div>
</template>
