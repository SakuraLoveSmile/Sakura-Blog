<script setup lang="ts">
import { computed, ref, watch, useId, onBeforeUnmount } from "vue";
import { VButton, VSwitch } from "@/components/ui";
import {
  openSettingSchema,
  schemaDefault,
  settingLabel,
  settingValueLabel,
  type SettingSchema,
} from "./stellar-theme-schema";

const props = withDefaults(
  defineProps<{
    schema: SettingSchema;
    modelValue?: any;
    defaultValue?: any;
    label: string;
    path: string;
    depth?: number;
    showHeading?: boolean;
  }>(),
  { depth: 0, showHeading: true },
);
const emit = defineEmits<{ "update:modelValue": [value: any]; pending: [value: boolean] }>();
const typeLabels: Record<string, string> = {
  null: "空值（继承 / 关闭）",
  object: "自定义配置",
  array: "自定义列表",
  boolean: "开关",
  string: "自定义文字",
  number: "数字",
};
const revision = ref(0);
const keyRevisions = ref<Record<string, number>>(Object.create(null));
const error = ref("");
const newKey = ref("");
const childPending = ref(new Map<string, boolean>());
const effectiveDefault = computed(() =>
  props.defaultValue === undefined ? schemaDefault(props.schema) : props.defaultValue,
);
const shown = computed(() => (props.modelValue === undefined ? effectiveDefault.value : props.modelValue));
const kind = computed(() =>
  shown.value === null ? "null" : Array.isArray(shown.value) ? "array" : typeof shown.value,
);
const types = computed(() => props.schema.type.filter((type) => type !== "undefined"));
const object = computed(() => (shown.value && kind.value === "object" ? shown.value : {}));
const keys = computed(() => [
  ...new Set([
    ...(props.schema.propertyOrder ?? Object.keys(props.schema.properties ?? {})),
    ...Object.keys(object.value),
  ]),
]);
const canAddKey = computed(
  () =>
    Boolean(props.schema.additionalProperties) ||
    (!props.schema.sealed && (!props.schema.properties || props.schema.normalizer === "parameter_bag")),
);
const id = useId();
const pending = computed(() => Boolean(error.value || newKey.value || [...childPending.value.values()].some(Boolean)));
watch(pending, (value) => emit("pending", value), { immediate: true, flush: "sync" });
// 项被显式移除或恢复默认时，及时注销其未完成状态。
onBeforeUnmount(() => emit("pending", false));
function change(value: any) {
  error.value = "";
  emit("update:modelValue", value);
}
function reset() {
  revision.value++;
  newKey.value = "";
  childPending.value = new Map();
  change(undefined);
}
function changeType(event: Event) {
  const type = (event.target as HTMLSelectElement).value;
  revision.value++;
  childPending.value = new Map();
  newKey.value = "";
  change(
    type === "default"
      ? undefined
      : type === "null"
        ? null
        : type === "object"
          ? {}
          : type === "array"
            ? []
            : type === "boolean"
              ? false
              : type === "number"
                ? 0
                : "",
  );
}
function childSchema(key: string): SettingSchema {
  return props.schema.properties && Object.hasOwn(props.schema.properties, key)
    ? props.schema.properties[key]
    : (props.schema.additionalProperties ?? openSettingSchema);
}
function updateKey(key: string, value: any) {
  // 对象展开保留来源 token；不将仅供显示的默认对象整体写回。
  const next = { ...(props.modelValue && typeof props.modelValue === "object" ? props.modelValue : {}) };
  if (value === undefined) delete next[key];
  else Object.defineProperty(next, key, { value, enumerable: true, writable: true, configurable: true });
  change(next);
}
function isGroup(key: string): boolean {
  const value = object.value[key] ?? effectiveDefault.value?.[key] ?? schemaDefault(childSchema(key));
  return value !== null && typeof value === "object";
}
function resetKey(key: string) {
  keyRevisions.value[key] = (keyRevisions.value[key] ?? 0) + 1;
  childPending.value.delete(key);
  updateKey(key, undefined);
}
function addKey() {
  const key = newKey.value.trim();
  if (!key) {
    error.value = "请填写名称。";
    return;
  }
  if (keys.value.includes(key)) {
    error.value = "名称已存在。";
    return;
  }
  updateKey(key, schemaDefault(childSchema(key)));
  newKey.value = "";
}
function updateIndex(index: number, value: any) {
  const next = [...shown.value];
  next[index] = value === undefined ? schemaDefault(props.schema.items ?? openSettingSchema) : value;
  change(next);
}
function move(index: number, offset: number) {
  const next = [...shown.value];
  [next[index], next[index + offset]] = [next[index + offset], next[index]];
  change(next);
}
function remove(index: number) {
  change(shown.value.filter((_: any, i: number) => i !== index));
}
function updateNumber(event: Event) {
  const input = event.target as HTMLInputElement;
  if (!input.value && !input.validity.badInput) {
    change(undefined);
    return;
  }
  const value = Number(input.value);
  if (
    input.validity.badInput ||
    !Number.isFinite(value) ||
    Math.abs(value) > Number.MAX_SAFE_INTEGER ||
    (props.schema.minimum !== undefined && value < props.schema.minimum) ||
    (props.schema.maximum !== undefined && value > props.schema.maximum) ||
    (props.schema.exclusiveMinimum !== undefined && value <= props.schema.exclusiveMinimum)
  ) {
    error.value = "请输入范围内的有效数字。";
    return;
  }
  change(value);
}
</script>

<template>
  <div class="structured-setting" :data-structured-path="path">
    <div v-if="showHeading" class="structured-heading">
      <label :for="id">{{ label }}</label>
      <button type="button" class="settings-reset" :aria-label="`${label}恢复默认`" @click="reset">恢复默认</button>
    </div>
    <select
      v-if="schema.values"
      :id="id"
      class="apple-select"
      :value="schema.values.findIndex((value) => value === shown)"
      @change="change(schema.values![Number(($event.target as HTMLSelectElement).value)])"
    >
      <option v-if="!schema.values.includes(shown)" value="-1">当前值：{{ settingValueLabel(shown) }}</option>
      <option v-for="(value, index) in schema.values" :key="index" :value="index">
        {{ settingValueLabel(value) }}
      </option>
    </select>
    <template v-else>
      <select
        v-if="types.length > 1"
        :id="`${id}-type`"
        class="apple-select structured-type"
        :aria-label="`${label}取值方式`"
        :value="modelValue === undefined ? 'default' : kind"
        @change="changeType"
      >
        <option value="default">继承默认</option>
        <option v-for="type in types" :key="type" :value="type">{{ typeLabels[type] }}</option>
      </select>
      <div v-if="kind === 'object'" class="structured-children">
        <component
          :is="isGroup(key) ? 'details' : 'div'"
          v-for="key in keys"
          :key="`${revision}-${key}-${keyRevisions[key] ?? 0}`"
          class="structured-item"
          :open="depth === 0 && path === 'theme.appearance' && key === 'colors'"
        >
          <summary v-if="isGroup(key)">
            {{ settingLabel(key) }}
            <button
              type="button"
              class="settings-reset"
              :aria-label="`${settingLabel(key)}恢复默认`"
              @click.stop.prevent="resetKey(key)"
            >
              恢复默认
            </button>
          </summary>
          <StructuredSetting
            :schema="childSchema(key)"
            :show-heading="!isGroup(key)"
            :model-value="modelValue?.[key]"
            :default-value="effectiveDefault?.[key]"
            :label="settingLabel(key)"
            :path="`${path}.${key}`"
            :depth="depth + 1"
            @update:model-value="updateKey(key, $event)"
            @pending="childPending.set(key, $event)"
          />
          <VButton
            v-if="!schema.properties || !Object.hasOwn(schema.properties, key)"
            size="sm"
            :disabled="pending"
            :aria-label="`移除${settingLabel(key)}`"
            @click="updateKey(key, undefined)"
            >移除</VButton
          >
        </component>
        <div v-if="canAddKey" class="structured-add">
          <input v-model="newKey" class="apple-input" :aria-label="`新增${label}名称`" placeholder="新增项目名称" />
          <VButton size="sm" @click="addKey">添加项目</VButton>
        </div>
      </div>
      <div v-else-if="kind === 'array'" class="structured-children">
        <div v-for="(value, index) in shown" :key="`${revision}-${index}`" class="structured-item">
          <StructuredSetting
            :schema="schema.items ?? openSettingSchema"
            :model-value="value"
            :label="`${label}第 ${Number(index) + 1} 项`"
            :path="`${path}.${index}`"
            :depth="depth + 1"
            @update:model-value="updateIndex(Number(index), $event)"
            @pending="childPending.set(String(index), $event)"
          />
          <div class="structured-actions">
            <VButton
              size="sm"
              :disabled="index === 0 || pending"
              :aria-label="`${label}第 ${Number(index) + 1} 项上移`"
              @click="move(Number(index), -1)"
              >上移</VButton
            >
            <VButton
              size="sm"
              :disabled="index === shown.length - 1 || pending"
              :aria-label="`${label}第 ${Number(index) + 1} 项下移`"
              @click="move(Number(index), 1)"
              >下移</VButton
            >
            <VButton
              size="sm"
              :disabled="pending"
              :aria-label="`移除${label}第 ${Number(index) + 1} 项`"
              @click="remove(Number(index))"
              >移除</VButton
            >
          </div>
        </div>
        <p v-if="!shown.length" class="settings-help">暂无项目。</p>
        <VButton
          size="sm"
          :aria-label="`添加${label}`"
          @click="change([...shown, schemaDefault(schema.items ?? openSettingSchema)])"
          >添加项目</VButton
        >
      </div>
      <VSwitch
        v-else-if="kind === 'boolean'"
        :id="id"
        :model-value="shown"
        :aria-label="label"
        @update:model-value="change($event)"
      />
      <input
        v-else-if="kind === 'number'"
        :id="id"
        type="number"
        step="any"
        :min="schema.minimum"
        :max="schema.maximum"
        :value="shown"
        class="apple-input"
        @input="updateNumber"
      />
      <p v-else-if="kind === 'null'" class="settings-help">当前为空值；保留继承或关闭状态。</p>
      <textarea
        v-else-if="String(shown ?? '').includes('\n') || /\.(content|html)$/.test(path)"
        :id="id"
        :value="shown ?? ''"
        class="apple-textarea"
        :rows="String(shown ?? '').includes('\n') ? 4 : 2"
        @input="change(($event.target as HTMLTextAreaElement).value)"
      ></textarea>
      <input
        v-else
        :id="id"
        :value="shown ?? ''"
        class="apple-input"
        @input="change(($event.target as HTMLInputElement).value)"
      />
    </template>
    <p v-if="error" class="settings-error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.structured-setting {
  min-width: 0;
}
.structured-heading,
.structured-actions,
.structured-add {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.structured-heading {
  justify-content: space-between;
  margin-bottom: 8px;
}
.structured-heading label {
  margin: 0;
  color: var(--color-text-secondary);
}
.structured-children {
  display: grid;
  gap: 12px;
  margin-top: 12px;
}
.structured-item {
  min-width: 0;
  padding: 8px 0;
}
details.structured-item {
  border-bottom: 1px solid var(--color-border);
  padding: 14px 0;
}
.settings-reset {
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--color-primary);
  font-size: 12px;
  cursor: pointer;
}
summary .settings-reset {
  float: right;
  margin-left: 12px;
}
.structured-setting > :is(.apple-input, .apple-select, .apple-textarea) {
  width: 100%;
}
.structured-item summary {
  cursor: pointer;
  color: var(--color-text-secondary);
  font-size: 13px;
  overflow-wrap: anywhere;
}
.structured-item[open] > summary {
  margin-bottom: 12px;
}
.structured-actions {
  margin-top: 10px;
}
.structured-add .apple-input {
  flex: 1;
  min-width: 120px;
}
.structured-type {
  margin-bottom: 8px;
}
.structured-setting :is(input, select, textarea) {
  max-width: 100%;
  box-sizing: border-box;
}
@media (max-width: 767px) {
  .structured-item {
    padding: 8px 0 8px 8px;
  }
}
</style>
