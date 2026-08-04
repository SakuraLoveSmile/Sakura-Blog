<script setup lang="ts">
import { computed } from "vue";

// 前端体验增强插件（Spec 36 段入库版）：开关 + 参数表单。
// 数据结构对齐 site.yml plugins 段；子配置为指针，未配置=关闭（JSON 缺省空对象等价）。
const props = defineProps<{
  data: any;
  saving: string;
}>();

const emit = defineEmits<{ (e: "save"): void }>();

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";

// pjax selectors 逗号分隔文本代理（保存时转回数组）
const pjaxSelectorsText = computed({
  get: () => (props.data.pjax.selectors || []).join(", "),
  set: (v: string) => {
    props.data.pjax.selectors = v
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  },
});
</script>

<template>
  <div class="space-y-6">
    <!-- 预加载 -->
    <div class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input type="checkbox" v-model="data.preload.enable" class="rounded" />
        页面资源预加载（Flying Pages）
      </label>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">服务（空用内置 flying_pages）</label>
          <input v-model="data.preload.service" :class="inputCls" placeholder="flying_pages" />
        </div>
      </div>
    </div>

    <!-- 滚动渐现 -->
    <div class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input type="checkbox" v-model="data.scrollreveal.enable" class="rounded" />
        滚动渐现动画（ScrollReveal）
      </label>
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">距离</label>
          <input v-model="data.scrollreveal.distance" :class="inputCls" placeholder="60px" />
        </div>
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">时长（ms）</label>
          <input v-model.number="data.scrollreveal.duration" type="number" :class="inputCls" placeholder="600" />
        </div>
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">间隔（ms）</label>
          <input v-model.number="data.scrollreveal.interval" type="number" :class="inputCls" placeholder="100" />
        </div>
      </div>
    </div>

    <!-- 中文排版 -->
    <div class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input type="checkbox" v-model="data.heti.enable" class="rounded" />
        中文排版优化（赫蹏 Heti）
      </label>
    </div>

    <!-- 图片灯箱 -->
    <div class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input type="checkbox" v-model="data.fancybox.enable" class="rounded" />
        图片灯箱（Fancybox）
      </label>
      <div>
        <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">选择器（空用默认 div.lazy-box&gt;img.lazy）</label>
        <input v-model="data.fancybox.selector" :class="inputCls" placeholder="div.lazy-box>img.lazy" />
      </div>
    </div>

    <!-- PJAX -->
    <div class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        <input type="checkbox" v-model="data.pjax.enable" class="rounded" />
        无刷新导航（PJAX）
      </label>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">交换选择器（逗号分隔）</label>
          <input v-model="pjaxSelectorsText" :class="inputCls" placeholder="title, #l_cover, .l_body" />
        </div>
        <div>
          <label class="text-xs text-gray-500 mb-1 block dark:text-gray-400">超时（ms）</label>
          <input v-model.number="data.pjax.timeout" type="number" :class="inputCls" placeholder="10000" />
        </div>
      </div>
      <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
        <input type="checkbox" v-model="data.pjax.cache_bust" class="rounded" />
        缓存破坏（cache_bust）
      </label>
    </div>

    <button @click="emit('save')" :disabled="saving === 'plugins'" :class="btnCls">
      保存插件设置
    </button>
  </div>
</template>
