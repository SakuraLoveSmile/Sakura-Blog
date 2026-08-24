<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { colorInputValue, withSchemaDefaults, type SchemaField, type SchemaValues } from "./schema";

const props = withDefaults(
  defineProps<{
    fields: SchemaField[];
    modelValue?: SchemaValues;
  }>(),
  { modelValue: () => ({}) },
);

const emit = defineEmits<{
  "update:modelValue": [value: SchemaValues];
}>();

const values = ref<SchemaValues>({});
const { t, te } = useI18n();

function syncValues() {
  values.value = withSchemaDefaults(props.fields, props.modelValue);
}

watch([() => props.fields, () => props.modelValue], syncValues, { immediate: true, deep: true });

function updateValue(field: SchemaField, value: unknown) {
  const next = { ...values.value, [field.key]: value };
  values.value = next;
  emit("update:modelValue", next);
}

function inputValue(field: SchemaField, event: Event): unknown {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
  if (field.type === "number") return target.value === "" ? "" : Number(target.value);
  return target.value;
}

function checkedValue(event: Event): boolean {
  return (event.target as HTMLInputElement).checked;
}

function isChecked(value: unknown): boolean {
  return value === true || value === "true";
}

function displayValue(value: unknown): string | number {
  return typeof value === "number" || typeof value === "string" ? value : "";
}

const inputCls = "apple-input";

function fieldLabel(field: SchemaField): string {
  return field.label_key && te(field.label_key) ? t(field.label_key) : field.label || field.key;
}

function fieldHint(field: SchemaField): string | undefined {
  return field.hint_key && te(field.hint_key) ? t(field.hint_key) : field.hint;
}
</script>

<template>
  <div class="space-y-2.5">
    <div v-for="field in fields" :key="field.key">
      <label
        v-if="field.type !== 'switch'"
        class="mb-1 block text-xs text-text-tertiary"
        :for="`schema-field-${field.key}`"
      >
        {{ fieldLabel(field) }}
      </label>

      <label
        v-if="field.type === 'switch'"
        class="flex items-center gap-2 text-sm text-text-tertiary"
        :for="`schema-field-${field.key}`"
      >
        <input
          :id="`schema-field-${field.key}`"
          type="checkbox"
          :checked="isChecked(values[field.key])"
          class="rounded border-border-strong"
          @change="updateValue(field, checkedValue($event))"
        />
        {{ fieldLabel(field) }}
      </label>

      <select
        v-else-if="field.type === 'select'"
        :id="`schema-field-${field.key}`"
        :value="displayValue(values[field.key])"
        :class="inputCls"
        @change="updateValue(field, inputValue(field, $event))"
      >
        <option v-for="option in field.options || []" :key="option" :value="option">
          {{ option || "（默认）" }}
        </option>
      </select>

      <input
        v-else-if="field.type === 'number'"
        :id="`schema-field-${field.key}`"
        type="number"
        :value="displayValue(values[field.key])"
        :placeholder="field.placeholder"
        :class="inputCls"
        @input="updateValue(field, inputValue(field, $event))"
      />

      <div v-else-if="field.type === 'color'" class="flex items-center gap-2">
        <input
          :id="`schema-field-${field.key}`"
          type="color"
          :value="colorInputValue(values[field.key])"
          class="h-9 w-12 cursor-pointer rounded-lg border border-border-strong bg-transparent p-1"
          @input="updateValue(field, inputValue(field, $event))"
        />
        <input
          :value="displayValue(values[field.key])"
          :placeholder="field.placeholder"
          :class="inputCls + ' flex-1'"
          @input="updateValue(field, inputValue(field, $event))"
        />
      </div>

      <textarea
        v-else-if="field.type === 'textarea'"
        :id="`schema-field-${field.key}`"
        :value="displayValue(values[field.key])"
        :rows="field.rows || 4"
        spellcheck="false"
        :placeholder="field.placeholder"
        :class="inputCls + ' resize-y font-mono text-xs leading-relaxed'"
        @input="updateValue(field, inputValue(field, $event))"
      ></textarea>

      <input
        v-else-if="field.type === 'password'"
        :id="`schema-field-${field.key}`"
        type="password"
        :value="displayValue(values[field.key])"
        :placeholder="field.placeholder"
        :class="inputCls"
        autocomplete="new-password"
        @input="updateValue(field, inputValue(field, $event))"
      />

      <input
        v-else
        :id="`schema-field-${field.key}`"
        type="text"
        :value="displayValue(values[field.key])"
        :placeholder="field.placeholder"
        :class="inputCls"
        @input="updateValue(field, inputValue(field, $event))"
      />

      <p v-if="fieldHint(field)" class="mt-1 text-[11px] text-text-quaternary">{{ fieldHint(field) }}</p>
    </div>
  </div>
</template>
