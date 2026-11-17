<script setup lang="ts">
/**
 * 组件实例配置弹窗（参照 Halo `WidgetConfigFormModal` 的“每实例独立 config”思路）。
 *
 * 只为 `widgetRegistry` 里声明了 `configFields` 的组件渲染；字段类型与
 * `WidgetConfigField` 一一对应，不引入 FormKit。保存时只提交已声明字段，
 * 未知键不落盘。
 */
import { computed, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import {
  configFieldOptions,
  type WidgetConfigField,
  type WidgetDefinition,
} from "./widgets/registry";

const props = defineProps<{
  visible: boolean;
  definition: WidgetDefinition;
  config: Record<string, unknown>;
}>();

const emit = defineEmits<{
  "update:visible": [value: boolean];
  save: [config: Record<string, unknown>];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit("update:visible", value),
});

const fields = computed<WidgetConfigField[]>(() => props.definition.configFields ?? []);

/** 编辑草稿：每次打开按当前生效配置重建，取消不写回。 */
const draft = ref<Record<string, unknown>>({});

function seedDraft() {
  draft.value = JSON.parse(JSON.stringify(props.config ?? {})) as Record<string, unknown>;
}

watch(
  () => [props.visible, props.definition.id, props.config],
  () => {
    if (props.visible) seedDraft();
  },
  { immediate: true, deep: true },
);

function fieldValue(field: WidgetConfigField): string {
  const value = draft.value[field.key];
  if (value === undefined || value === null) return "";
  if (Array.isArray(value)) return value.join(",");
  return String(value);
}

function setScalar(field: WidgetConfigField, value: string) {
  if (field.type === "boolean") {
    draft.value = { ...draft.value, [field.key]: value === "true" };
    return;
  }
  if (field.type === "number") {
    const n = Number(value);
    if (!Number.isFinite(n)) return;
    const min = field.min ?? Number.NEGATIVE_INFINITY;
    const max = field.max ?? Number.POSITIVE_INFINITY;
    draft.value = { ...draft.value, [field.key]: Math.min(Math.max(n, min), max) };
    return;
  }
  draft.value = { ...draft.value, [field.key]: value };
}

/** 多选字段（当前只有叠放容器的子组件列表）：按点击顺序追加/移除。 */
function toggleMulti(field: WidgetConfigField, value: string) {
  const current = Array.isArray(draft.value[field.key])
    ? [...(draft.value[field.key] as unknown[])].filter((item): item is string => typeof item === "string")
    : [];
  const index = current.indexOf(value);
  if (index >= 0) current.splice(index, 1);
  else current.push(value);
  draft.value = { ...draft.value, [field.key]: current };
}

function isMulti(field: WidgetConfigField): boolean {
  return field.key === "stackWidgets";
}

function selectedValues(field: WidgetConfigField): string[] {
  const value = draft.value[field.key];
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function isBooleanOn(field: WidgetConfigField): boolean {
  return draft.value[field.key] === true;
}

function save() {
  const next: Record<string, unknown> = {};
  for (const field of fields.value) {
    const value = draft.value[field.key];
    if (value === undefined) continue;
    next[field.key] = value;
  }
  emit("save", next);
  emit("update:visible", false);
}

function close() {
  emit("update:visible", false);
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="组件设置" :width="560">
    <div class="config-modal-header">
      <div class="config-modal-icon">
        <Icon :icon="definition.icon" class="h-5 w-5" />
      </div>
      <div class="config-modal-meta">
        <div class="config-modal-name">{{ definition.label }}</div>
        <div class="config-modal-hint">配置只作用于这一个组件实例</div>
      </div>
    </div>

    <div v-if="!fields.length" class="config-modal-empty">
      <Icon icon="ri:information-line" class="h-4 w-4" />
      这个组件还没有可配置项。
    </div>

    <div v-else class="config-field-list">
      <div v-for="field in fields" :key="field.key" class="config-field">
        <div class="config-field-label">
          <span>{{ field.label }}</span>
        </div>
        <p v-if="field.description" class="config-field-description">{{ field.description }}</p>

        <!-- 多选：叠放容器的子组件列表 -->
        <div v-if="isMulti(field)" class="config-multi">
          <button
            v-for="option in configFieldOptions(field)"
            :key="option.value"
            type="button"
            class="config-multi-item"
            :class="{ 'is-selected': selectedValues(field).includes(option.value) }"
            :aria-pressed="selectedValues(field).includes(option.value)"
            @click="toggleMulti(field, option.value)"
          >
            <Icon
              :icon="selectedValues(field).includes(option.value) ? 'ri:checkbox-circle-line' : 'ri:circle-line'"
              class="h-4 w-4"
            />
            {{ option.label }}
          </button>
        </div>

        <!-- 布尔：开关 -->
        <button
          v-else-if="field.type === 'boolean'"
          type="button"
          class="config-switch"
          role="switch"
          :aria-checked="isBooleanOn(field)"
          @click="setScalar(field, isBooleanOn(field) ? 'false' : 'true')"
        >
          <span class="config-switch-track" :class="{ 'is-on': isBooleanOn(field) }">
            <span class="config-switch-thumb" />
          </span>
          <span class="config-switch-text">{{ isBooleanOn(field) ? "已开启" : "已关闭" }}</span>
        </button>

        <!-- 下拉：select -->
        <select
          v-else-if="field.type === 'select'"
          class="config-select"
          :value="fieldValue(field)"
          @change="setScalar(field, ($event.target as HTMLSelectElement).value)"
        >
          <option v-for="option in configFieldOptions(field)" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>

        <!-- 数字 -->
        <input
          v-else-if="field.type === 'number'"
          class="config-input"
          type="number"
          :min="field.min"
          :max="field.max"
          :value="fieldValue(field)"
          @input="setScalar(field, ($event.target as HTMLInputElement).value)"
        />

        <!-- 文本 -->
        <input
          v-else
          class="config-input"
          type="text"
          :value="fieldValue(field)"
          :placeholder="field.placeholder"
          @input="setScalar(field, ($event.target as HTMLInputElement).value)"
        />
      </div>
    </div>

    <template #footer>
      <button class="config-cancel" @click="close">取消</button>
      <button class="config-save" :disabled="!fields.length" @click="save">保存</button>
    </template>
  </VModal>
</template>

<style scoped>
.config-modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  padding: 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.config-modal-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  color: var(--color-text-tertiary);
  flex-shrink: 0;
}

.config-modal-meta {
  min-width: 0;
}

.config-modal-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-foreground);
}

.config-modal-hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.config-modal-empty {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  font-size: 12px;
  color: var(--color-text-secondary);
}

.config-field-list {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.config-field-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
}

.config-field-description {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.config-select,
.config-input {
  margin-top: 8px;
  width: 100%;
  height: 36px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: var(--color-card);
  color: var(--color-foreground);
  font-size: 13px;
}

.config-select:focus-visible,
.config-input:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 1px;
}

.config-multi {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 8px;
}

.config-multi-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-tertiary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: color 150ms, border-color 150ms, background-color 150ms;
}

.config-multi-item:hover {
  color: var(--color-text-secondary);
  border-color: var(--color-border-strong);
}

.config-multi-item.is-selected {
  color: var(--color-foreground);
  border-color: var(--color-border-strong);
  background: var(--color-accent);
}

.config-multi-item:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.config-switch {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  background: transparent;
  border: 0;
  padding: 0;
  cursor: pointer;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.config-switch:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
  border-radius: var(--radius-sm);
}

.config-switch-track {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 40px;
  height: 22px;
  padding: 2px;
  border-radius: var(--radius-full);
  background: var(--color-accent);
  border: 1px solid var(--color-border);
  transition: background-color 150ms;
}

.config-switch-track.is-on {
  background: var(--color-state-success);
  border-color: var(--color-state-success);
}

.config-switch-thumb {
  width: 16px;
  height: 16px;
  border-radius: var(--radius-full);
  background: var(--color-foreground);
  transition: transform 150ms;
}

.config-switch-track.is-on .config-switch-thumb {
  transform: translateX(18px);
}

.config-cancel {
  margin-right: 8px;
  padding: 8px 16px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  color: var(--color-text-secondary);
  background: transparent;
  border: 1px solid var(--color-border);
  cursor: pointer;
}

.config-save {
  padding: 8px 16px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-primary-foreground);
  background: var(--color-primary);
  border: 0;
  cursor: pointer;
}

.config-save:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
