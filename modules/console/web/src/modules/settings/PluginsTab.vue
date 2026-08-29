<script setup lang="ts">
import { VButton, VSwitch } from "@/components/ui";

// 前端体验增强插件（Spec 36 段入库版）：开关 + 参数表单。
// 数据结构对齐 site.yml plugins 段；子配置为指针，未配置=关闭（JSON 缺省空对象等价）。
const props = defineProps<{
  data: any;
  saving: string;
}>();

const emit = defineEmits<{ (e: "save"): void }>();

const inputCls = "apple-input";
</script>

<template>
  <div class="space-y-6">
    <!-- 封面文字自适应 -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <div class="flex items-center justify-between gap-4">
        <div class="min-w-0">
          <div class="text-sm font-medium text-text-secondary">封面文字自适应颜色</div>
          <p class="mt-1 text-xs text-text-quaternary">根据封面平均色调整 Hero、横幅和置顶内容的文字对比度</p>
        </div>
        <VSwitch v-model="data.adaptive_text.enable" />
      </div>
    </div>

    <!-- Card Hover（Stellar 1.44） -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <div class="flex items-center justify-between gap-4">
        <div class="min-w-0">
          <div class="text-sm font-medium text-text-secondary">卡片 Hover 光斑与倾斜</div>
          <p class="mt-1 text-xs text-text-quaternary">为文章、文库、link/grid 卡片启用 Spotlight 与 3D Tilt（桌面精细指针）</p>
        </div>
        <VSwitch v-model="data.card_hover.enable" />
      </div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">光斑颜色</label>
          <input v-model="data.card_hover.spotlight_color" :class="inputCls" placeholder="rgba(255, 255, 255, 0.25)" />
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">最大倾角（0–8°）</label>
          <input v-model.number="data.card_hover.max_tilt" type="number" min="0" max="8" step="0.5" :class="inputCls" />
        </div>
      </div>
    </div>

    <!-- 预加载 -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <label class="flex items-center gap-2 text-sm font-medium text-text-secondary">
        <input type="checkbox" v-model="data.preload.enable" class="rounded" />
        页面资源预加载（Flying Pages）
      </label>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-text-quaternary mb-1 block"
            >服务（预留字段，当前仅支持内置 flying_pages，留空即可）</label
          >
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

    <!-- Swiper -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
        ><input type="checkbox" v-model="data.swiper.enable" class="rounded" />轮播图（Swiper）</label
      >
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">CSS 资源 URL（留空使用本地）</label
          ><input v-model="data.swiper.css" :class="inputCls" />
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">JS 资源 URL（留空使用本地）</label
          ><input v-model="data.swiper.js" :class="inputCls" />
        </div>
      </div>
    </div>

    <!-- 数学与图表 -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <div class="grid grid-cols-2 gap-4">
        <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
          ><input type="checkbox" v-model="data.katex.enable" class="rounded" />KaTeX 数学公式</label
        >
        <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
          ><input type="checkbox" v-model="data.mathjax.enable" class="rounded" />MathJax 数学公式</label
        >
        <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
          ><input type="checkbox" v-model="data.mermaid.enable" class="rounded" />Mermaid 图表</label
        >
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">Mermaid 主题</label
          ><select v-model="data.mermaid.theme" :class="inputCls">
            <option value="default">default</option>
            <option value="dark">dark</option>
            <option value="forest">forest</option>
            <option value="neutral">neutral</option>
          </select>
        </div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">KaTeX head 注入</label
          ><textarea v-model="data.katex.inject" rows="3" :class="inputCls" placeholder="可填 link 标签"></textarea>
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">Mermaid JS URL（留空使用本地）</label
          ><input v-model="data.mermaid.js" :class="inputCls" />
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">MathJax JS URL</label
          ><input v-model="data.mathjax.js" :class="inputCls" placeholder="https://cdnjs.cloudflare.com/..." />
        </div>
        <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
          ><input type="checkbox" v-model="data.mathjax.v3" class="rounded" />MathJax v3</label
        >
        <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
          ><input type="checkbox" v-model="data.mermaid.style_optimization" class="rounded" />使用主题 Mermaid
          样式</label
        >
      </div>
    </div>

    <!-- 代码复制 -->
    <div class="border border-border rounded-xl p-4 space-y-3">
      <label class="flex items-center gap-2 text-sm font-medium text-text-secondary"
        ><input type="checkbox" v-model="data.copycode.enable" class="rounded" />代码块复制按钮</label
      >
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">默认文案</label
          ><input v-model="data.copycode.default_text" :class="inputCls" />
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">成功文案</label
          ><input v-model="data.copycode.success_text" :class="inputCls" />
        </div>
        <div>
          <label class="text-xs text-text-tertiary mb-1 block">提示文案</label
          ><input v-model="data.copycode.toast" :class="inputCls" />
        </div>
      </div>
    </div>
  </div>

  <div class="mt-4">
    <VButton type="primary" size="sm" :loading="saving === 'plugins'" @click="emit('save')">保存插件设置</VButton>
  </div>
</template>
