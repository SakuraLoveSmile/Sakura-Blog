<script setup lang="ts">
import { computed, useId } from "vue";
import { settingValueLabel } from "./stellar-theme-schema";

const props = defineProps<{ path: string; options: unknown[]; modelValue: unknown }>();
const emit = defineEmits<{ "update:modelValue": [value: unknown] }>();
const name = useId();
const descriptions: Record<string, string> = {
  card: "卡片分区清楚，内容层次鲜明",
  glass: "半透明层次，搭配背景更明显",
  minimal: "减少装饰，让阅读内容更突出",
  flat: "弱化卡片边界，页面更平整",
  auto: "跟随访客设备的明暗偏好",
  light: "以浅色背景呈现内容",
  dark: "以深色背景呈现内容",
  hero: "大幅封面突出图片内容",
  classic: "缩略图与文字并排，信息更紧凑",
  carousel: "置顶文章以轮播方式展示",
  tech: "适合代码、教程与技术内容",
  story: "适合连续阅读的叙事内容",
  local: "在浏览器搜索静态索引",
  go: "请求站点服务进行搜索",
  algolia: "使用已配置的 Algolia 索引",
  none: "不提供访客搜索服务",
};
const labels: Record<string, string> = {
  hero: "大图卡片",
  classic: "经典卡片",
  carousel: "轮播",
  tech: "技术文章",
  story: "叙事文章",
  local: "本地静态索引",
  go: "Go 服务搜索",
  algolia: "Algolia",
  none: "关闭搜索",
};
const fieldLabels: Record<string, string> = {
  "theme.appearance.preset": "外观预设",
  "theme.appearance.color_scheme": "配色模式",
  "theme.article.listing.card_layout": "文章卡片",
  "theme.article.listing.pinned_layout": "置顶布局",
  "theme.article.style": "文章风格",
  "theme.search.provider": "搜索服务",
};
const choiceLabel = (value: unknown) =>
  props.path === "theme.article.listing.pinned_layout" && value === "flat"
    ? "平铺"
    : Object.hasOwn(labels, String(value))
      ? labels[String(value)]
      : settingValueLabel(value);
const choiceDescription = (value: unknown) =>
  props.path === "theme.article.listing.pinned_layout" && value === "flat"
    ? "置顶文章平铺显示，无需切换轮播"
    : descriptions[String(value)];
const scheme = computed(() => props.path === "theme.appearance.color_scheme");
const preset = computed(() => props.path === "theme.appearance.preset");
const known = computed(() => props.options.includes(props.modelValue));
</script>

<template>
  <div
    class="setting-visual-choices"
    :class="{ 'preset-choices': preset, 'two-column-choices': !preset && options.length !== 3 }"
    role="radiogroup"
    :aria-label="fieldLabels[path] || path"
  >
    <label
      v-for="(value, index) in options"
      :key="index"
      class="visual-choice"
      :class="{ selected: value === modelValue }"
    >
      <input
        type="radio"
        :name="name"
        :value="value as string"
        :checked="value === modelValue"
        :aria-label="choiceLabel(value)"
        @change="emit('update:modelValue', value)"
      />
      <span v-if="preset" class="preset-example" :class="`example-${value}`" aria-hidden="true">
        <span class="example-sidebar"><i></i><i></i><i></i></span>
        <span class="example-main"
          ><span class="example-article"><i></i><i></i><i></i></span><span class="example-article"><i></i><i></i></span
        ></span>
      </span>
      <span v-else-if="scheme" class="scheme-example" :class="`scheme-${value}`" aria-hidden="true"
        ><i></i><i></i><i></i
      ></span>
      <span class="choice-name"
        >{{ choiceLabel(value)
        }}<span v-if="value === modelValue" class="choice-check" aria-hidden="true">✓</span></span
      >
      <span class="choice-description">{{ choiceDescription(value) }}</span>
    </label>
    <label v-if="!known" class="visual-choice selected unknown-choice">
      <input
        type="radio"
        :name="name"
        :value="modelValue as string"
        checked
        :aria-label="`当前值：${settingValueLabel(modelValue)}`"
      />
      <span class="choice-name">当前值：{{ settingValueLabel(modelValue) }}</span>
      <span class="choice-description">保留当前配置；选择其他选项后才会更改。</span>
    </label>
  </div>
  <p v-if="preset || scheme" class="visual-choice-caption">
    {{ preset ? "风格示意" : "配色示意" }} · 实际效果请查看草稿预览
  </p>
</template>

<style scoped>
.setting-visual-choices {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  min-width: 0;
}
.two-column-choices {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.preset-choices {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.visual-choice {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  margin: 0;
  padding: 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-background);
  cursor: pointer;
}
.visual-choice:hover {
  border-color: var(--color-text-tertiary);
}
.visual-choice.selected {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 6%, var(--color-background));
}
.visual-choice:has(input:focus-visible) {
  outline: 2px solid var(--color-primary);
  outline-offset: 3px;
}
.visual-choice input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
.choice-name {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.choice-check {
  color: var(--color-primary);
}
.choice-description {
  color: var(--color-text-tertiary);
  font-size: 11px;
  font-weight: 400;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.visual-choice-caption {
  margin: 8px 0 0;
  color: var(--color-text-tertiary);
  font-size: 11px;
  line-height: 1.5;
}
.preset-example {
  display: grid;
  grid-template-columns: 24% 1fr;
  gap: 8px;
  height: 68px;
  margin-bottom: 4px;
  padding: 8px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-card);
}
.example-sidebar {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding-top: 6px;
}
.example-sidebar i,
.example-article i {
  display: block;
  height: 3px;
  border-radius: 2px;
  background: var(--color-text-quaternary);
}
.example-sidebar i:first-child {
  height: 10px;
  width: 10px;
  margin-bottom: 2px;
  border-radius: 50%;
  background: var(--color-primary);
}
.example-main {
  display: grid;
  gap: 6px;
}
.example-article {
  display: grid;
  align-content: center;
  gap: 4px;
  padding: 6px;
  border: 1px solid var(--color-border);
  border-radius: 5px;
  background: var(--color-background);
}
.example-article i:first-child {
  width: 65%;
  background: var(--color-text-secondary);
}
.example-article i:last-child {
  width: 80%;
}
.example-glass {
  background: color-mix(in srgb, var(--color-primary) 18%, var(--color-card));
}
.example-glass .example-article {
  background: color-mix(in srgb, var(--color-foreground) 10%, transparent);
  border-color: color-mix(in srgb, var(--color-foreground) 15%, transparent);
}
.example-minimal {
  background: var(--color-background);
}
.example-minimal .example-article {
  background: transparent;
  border-color: transparent;
  padding-inline: 0;
}
.example-flat .example-article {
  border-color: transparent;
  border-radius: 0;
}
.scheme-example {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  width: 34px;
  height: 30px;
  margin-bottom: 2px;
  padding: 6px;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-foreground);
}
.scheme-example i {
  display: block;
  height: 2px;
  background: var(--color-background);
}
.scheme-example i:last-child {
  width: 60%;
}
.scheme-dark {
  background: var(--color-background);
}
.scheme-dark i {
  background: var(--color-foreground);
}
.scheme-auto {
  background: var(--color-background);
  box-shadow: inset 17px 0 var(--color-foreground);
}
.scheme-auto i {
  background: var(--color-foreground);
  box-shadow: inset 11px 0 var(--color-background);
}
@media (max-width: 900px) {
  .two-column-choices {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .preset-choices {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 480px) {
  .setting-visual-choices {
    gap: 8px;
  }
  .visual-choice {
    padding: 10px;
  }
}
@media (forced-colors: active) {
  .visual-choice input {
    position: static;
    width: auto;
    height: auto;
    opacity: 1;
  }
  .visual-choice.selected {
    outline: 2px solid Highlight;
  }
}
</style>
