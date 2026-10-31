<script setup lang="ts">
import { computed } from "vue";
import { changeSettingColorAlpha, parseSettingColor, replaceSettingColor } from "./setting-color";

const props = withDefaults(defineProps<{ id: string; modelValue: string; label?: string; hexOnly?: boolean }>(), {
  hexOnly: false,
});
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const parsed = computed(() => {
  if (!props.hexOnly) return parseSettingColor(props.modelValue);
  const value = props.modelValue.trim();
  return /^#?(?:[\da-f]{3}|[\da-f]{6})$/i.test(value)
    ? parseSettingColor(value.startsWith("#") ? value : `#${value}`)
    : undefined;
});
const name = computed(() => props.label || "颜色");
const opacity = computed(() => Math.round((parsed.value?.alpha ?? 1) * 100));
const contextDependent = computed(() =>
  /\b(?:var|env|light-dark)\s*\(|\bcurrentcolor\b|^(?:inherit|initial|unset|revert(?:-layer)?)$/i.test(
    props.modelValue.trim(),
  ),
);
const preview = computed(() =>
  contextDependent.value
    ? undefined
    : props.hexOnly
      ? parsed.value?.hex
      : typeof CSS !== "undefined" && CSS.supports("color", props.modelValue)
        ? props.modelValue
        : undefined,
);
const palette = [
  { name: "浅蓝", value: "#66abff" },
  { name: "蓝色", value: "#007aff" },
  { name: "紫色", value: "#af52de" },
  { name: "靛蓝", value: "#5856d6" },
  { name: "红色", value: "#ff3b30" },
  { name: "橙色", value: "#ff9500" },
  { name: "绿色", value: "#34c759" },
  { name: "石墨", value: "#48484a" },
];
const inputValue = (event: Event) => (event.target as HTMLInputElement).value;
function choose(value: string) {
  emit(
    "update:modelValue",
    props.hexOnly ? parseSettingColor(value)?.hex || value : replaceSettingColor(props.modelValue, value),
  );
}
</script>

<template>
  <div class="setting-color-editor" data-testid="setting-color-editor">
    <div class="color-entry-row">
      <span
        class="color-current"
        :class="{ 'color-unavailable': !preview }"
        :title="preview ? '当前颜色示意' : '当前值无法显示颜色'"
        aria-hidden="true"
      >
        <span v-if="preview" :style="{ backgroundColor: preview }"></span>
        <span v-else>—</span>
      </span>
      <input
        :id="id"
        type="text"
        class="apple-input color-text-input"
        :value="modelValue"
        :aria-label="label"
        :aria-describedby="`${id}-color-help`"
        autocomplete="off"
        spellcheck="false"
        @input="emit('update:modelValue', inputValue($event))"
      />
      <label class="color-picker-label" :for="`${id}-picker`">
        <input
          :id="`${id}-picker`"
          type="color"
          :value="parsed?.hex || '#000000'"
          :aria-label="`${name}取色器`"
          @input="choose(inputValue($event))"
        />
        <span>取色</span>
      </label>
    </div>
    <div class="color-palette" role="group" :aria-label="`${name}常用色板`">
      <button
        v-for="color in palette"
        :key="color.value"
        type="button"
        class="color-palette-option"
        :class="{ selected: parsed?.hex === color.value }"
        :aria-label="`${color.name} ${color.value}`"
        :aria-pressed="parsed?.hex === color.value"
        :title="`${color.name} ${color.value}`"
        @click="choose(color.value)"
      >
        <span :style="{ backgroundColor: color.value }"></span>
      </button>
    </div>
    <div v-if="!hexOnly" class="color-opacity-row">
      <label :for="`${id}-opacity`">不透明度</label>
      <input
        :id="`${id}-opacity`"
        type="range"
        min="0"
        max="100"
        step="1"
        :value="opacity"
        :disabled="!parsed"
        :aria-label="`${name}不透明度`"
        :aria-valuetext="`${opacity}%`"
        @input="emit('update:modelValue', changeSettingColorAlpha(modelValue, Number(inputValue($event))))"
      />
      <output :for="`${id}-opacity`">{{ parsed ? `${opacity}%` : "—" }}</output>
    </div>
    <p :id="`${id}-color-help`" class="color-help">
      {{
        contextDependent
          ? "此颜色依赖前台环境，请在草稿预览中查看；也可保留原文或重新取色。"
          : hexOnly
            ? "支持 HEX 颜色；也可以取色或选择色板。"
            : parsed
              ? "支持 HEX、RGB、HSL；选色保留不透明度。0% 完全透明，100% 完全不透明。"
              : "保留当前原文。可手动输入 HEX、RGB、HSL，或取色、选择色板开始新颜色。"
      }}
    </p>
  </div>
</template>

<style scoped>
.setting-color-editor {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.color-entry-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.color-current {
  display: grid;
  place-items: center;
  flex: 0 0 32px;
  width: 32px;
  height: 32px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-background);
  overflow: hidden;
}
.color-current > span {
  width: 100%;
  height: 100%;
}
.color-current.color-unavailable > span {
  display: grid;
  place-items: center;
  color: var(--color-text-tertiary);
}
.color-text-input {
  flex: 1;
  width: 0;
  min-width: 0;
}
.color-picker-label {
  position: relative;
  display: grid;
  place-items: center;
  flex: 0 0 52px;
  height: 44px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  background: var(--color-background);
  font-size: 13px;
  cursor: pointer;
}
.color-picker-label input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  padding: 0;
  opacity: 0;
  cursor: pointer;
}
.color-picker-label:has(input:focus-visible),
.color-palette-option:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.color-palette {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
.color-palette-option {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  padding: 6px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: transparent;
  cursor: pointer;
}
.color-palette-option:hover {
  background: var(--color-background);
}
.color-palette-option.selected {
  border-color: var(--color-primary);
}
.color-palette-option > span {
  width: 28px;
  height: 28px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-full);
}
.color-opacity-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--color-text-secondary);
  font-size: 12px;
}
.color-opacity-row label {
  flex-shrink: 0;
}
.color-opacity-row input {
  flex: 1;
  min-width: 0;
  height: 28px;
  margin: 0;
  accent-color: var(--color-primary);
}
.color-opacity-row output {
  min-width: 36px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.color-help {
  margin: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}
</style>
