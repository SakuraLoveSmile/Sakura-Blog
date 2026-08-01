<script setup lang="ts">
const props = defineProps<{
  data: Record<string, any>;
  saving: string;
}>();
const emit = defineEmits<{
  save: [section: string];
}>();

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";

/** 从 px 字符串提取数值，如 "16px" → 16 */
function pxNum(v: string | undefined): number {
  if (!v) return 16;
  return parseInt(String(v).replace("px", ""), 10) || 16;
}
/** 数值转 px 字符串 */
function numPx(n: number): string {
  return n + "px";
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
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">主题色</h3>
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">主题色 (theme)</label>
          <div class="flex gap-2 items-center">
            <input type="color" v-model="data.style.color.theme" class="w-10 h-10 rounded border border-gray-200 cursor-pointer dark:border-gray-600" />
            <input v-model="data.style.color.theme" :class="inputCls" class="flex-1" placeholder="#4ccba0" />
          </div>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">强调色 (accent)</label>
          <div class="flex gap-2 items-center">
            <input type="color" v-model="data.style.color.accent" class="w-10 h-10 rounded border border-gray-200 cursor-pointer dark:border-gray-600" />
            <input v-model="data.style.color.accent" :class="inputCls" class="flex-1" placeholder="#ff9800" />
          </div>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">链接色 (link)</label>
          <div class="flex gap-2 items-center">
            <input type="color" v-model="data.style.color.link" class="w-10 h-10 rounded border border-gray-200 cursor-pointer dark:border-gray-600" />
            <input v-model="data.style.color.link" :class="inputCls" class="flex-1" placeholder="#2196f3" />
          </div>
        </div>
      </div>
    </div>

    <!-- 2. 深浅模式 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">深浅模式</h3>
      <div class="flex gap-6">
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input type="radio" v-model="data.style.prefers_theme" value="" /> 自动
        </label>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input type="radio" v-model="data.style.prefers_theme" value="light" /> 浅色
        </label>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input type="radio" v-model="data.style.prefers_theme" value="dark" /> 深色
        </label>
      </div>
    </div>

    <!-- 3. 字体字号 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">字体字号</h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">根字号 (root)</label>
          <div class="flex items-center gap-3">
            <input type="range" min="12" max="20" step="1" :value="pxNum(data.style.font_size?.root)" @input="data.style.font_size.root = numPx(Number(($event.target as HTMLInputElement).value))" class="flex-1 accent-secondary" />
            <span class="text-sm text-gray-600 w-12 text-right dark:text-gray-400">{{ data.style.font_size.root || '16px' }}</span>
          </div>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">正文字号 (body)</label>
          <div class="flex items-center gap-3">
            <input type="range" min="12" max="20" step="1" :value="pxNum(data.style.font_size?.body)" @input="data.style.font_size.body = numPx(Number(($event.target as HTMLInputElement).value))" class="flex-1 accent-secondary" />
            <span class="text-sm text-gray-600 w-12 text-right dark:text-gray-400">{{ data.style.font_size.body || '15px' }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 4. 字体族 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">字体族</h3>
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">正文字体 (body)</label>
          <input v-model="data.style.font_family.body" :class="inputCls" placeholder="留空使用默认字体" />
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">行内代码字体 (code)</label>
          <input v-model="data.style.font_family.code" :class="inputCls" placeholder="留空使用默认字体" />
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">代码块字体 (codeblock)</label>
          <input v-model="data.style.font_family.codeblock" :class="inputCls" placeholder="留空使用默认字体" />
        </div>
      </div>
    </div>

    <!-- 5. 左侧栏背景 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">左侧栏背景</h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">浅色模式背景色</label>
          <div class="flex gap-2 items-center">
            <input type="color" v-model="data.style.leftbar.background_color_light" class="w-10 h-10 rounded border border-gray-200 cursor-pointer dark:border-gray-600" />
            <input v-model="data.style.leftbar.background_color_light" :class="inputCls" class="flex-1" />
          </div>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">深色模式背景色</label>
          <div class="flex gap-2 items-center">
            <input type="color" v-model="data.style.leftbar.background_color_dark" class="w-10 h-10 rounded border border-gray-200 cursor-pointer dark:border-gray-600" />
            <input v-model="data.style.leftbar.background_color_dark" :class="inputCls" class="flex-1" />
          </div>
        </div>
      </div>
      <div class="mt-3">
        <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">背景图片 URL</label>
        <div class="flex gap-2 items-start">
          <input v-model="data.style.leftbar.background_image" :class="inputCls" class="flex-1" placeholder="输入图片 URL" />
          <img v-if="data.style.leftbar.background_image" :src="data.style.leftbar.background_image" class="w-10 h-10 rounded object-cover border border-gray-200 dark:border-gray-600" alt="" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4 mt-3">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">模糊半径 (blur_px)</label>
          <div class="flex items-center gap-3">
            <input type="range" min="0" max="20" step="1" v-model.number="data.style.leftbar.blur_px" class="flex-1 accent-secondary" />
            <span class="text-sm text-gray-600 w-12 text-right dark:text-gray-400">{{ data.style.leftbar.blur_px ?? 0 }}px</span>
          </div>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">背景不透明度</label>
          <input v-model="data.style.leftbar.background_opacity" :class="inputCls" placeholder="1.0" />
        </div>
      </div>
    </div>

    <!-- 6. 全站背景 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">全站背景</h3>
      <div>
        <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">背景图片 URL</label>
        <div class="flex gap-2 items-start">
          <input v-model="data.style.site.background_image" :class="inputCls" class="flex-1" placeholder="输入图片 URL" />
          <img v-if="data.style.site.background_image" :src="data.style.site.background_image" class="w-10 h-10 rounded object-cover border border-gray-200 dark:border-gray-600" alt="" />
        </div>
      </div>
      <div class="grid grid-cols-3 gap-4 mt-3">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">模糊半径 (blur_px)</label>
          <input v-model="data.style.site.blur_px" :class="inputCls" placeholder="0" />
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">模糊饱和度 (blur_sat)</label>
          <input v-model="data.style.site.blur_sat" :class="inputCls" placeholder="1" />
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">模糊背景 (blur_bg)</label>
          <input v-model="data.style.site.blur_bg" :class="inputCls" placeholder="0" />
        </div>
      </div>
    </div>

    <!-- 7. 代码块 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">代码块</h3>
      <div>
        <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">高亮主题 URL (scrollbar)</label>
        <input v-model="data.style.codeblock.scrollbar" :class="inputCls" placeholder="代码块高亮主题 CSS URL" />
      </div>
    </div>

    <!-- 8. 杂项 -->
    <div class="mb-6">
      <h3 class="text-sm font-semibold text-gray-700 mb-3 dark:text-gray-200">杂项</h3>
      <div>
        <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">标题前缀 (header_prefix) — JSON 格式</label>
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
