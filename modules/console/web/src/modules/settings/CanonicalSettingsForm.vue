<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";
import StructuredSetting from "./StructuredSetting.vue";
import StructuredSettingChoices from "./StructuredSettingChoices.vue";
import SettingColorEditor from "./SettingColorEditor.vue";
import { structuredSettingChoicesAvailable } from "./structured-setting-guidance";
import { settingFieldEffect } from "./settings-field-effects";
import { themeSettingSchema, themeSettingDefault, type SettingSchema } from "./stellar-theme-schema";
import AttachmentField from "@/components/AttachmentField.vue";
import { readJSONField, readJSONFieldRaw } from "@/components/editor/stellar-form-json";
import {
  canonicalSettingsGroups,
  effectiveSearchProvider,
  searchProviderChanges,
  type CanonicalSettingsField,
  type CanonicalSettingsTab,
} from "./settings-canonical-fields";

const props = withDefaults(
  defineProps<{
    tab: CanonicalSettingsTab;
    document: string;
    saving?: string;
    disabled?: boolean;
    saveLabel?: string;
    saveHelp?: string;
    invalidHelp?: string;
  }>(),
  {
    saveLabel: "保存全部主题设置",
    saveHelp: "同时保存所有分类中尚未保存的主题修改；基本与反馈单独保存",
    invalidHelp: "请先修正上方输入问题，再保存主题设置。",
  },
);
const emit = defineEmits<{
  change: [path: string, value: unknown, rawValue?: string];
  save: [];
  pending: [value: boolean];
}>();
const groups = computed(() => canonicalSettingsGroups[props.tab]);
const drafts = reactive<Record<string, string>>({});
const errors = reactive<Record<string, string>>({});
const newKeys = reactive<Record<string, string>>({});
const newValues = reactive<Record<string, string>>({});
const hasErrors = computed(() => Object.values(errors).some(Boolean));
const structuredRevision = reactive<Record<string, number>>({});
const structuredPending = reactive<Record<string, boolean>>({});
const hasMapDraft = computed(() => Object.values(newKeys).some(Boolean) || Object.values(newValues).some(Boolean));
const hasPending = computed(
  () => hasErrors.value || hasMapDraft.value || Object.values(structuredPending).some(Boolean),
);
watch(hasPending, (value) => emit("pending", value), { immediate: true });
watch(
  () => props.document,
  () => {
    // 已成功同步的输入跟随共享草稿；无效 JSON 留在原字段，等待用户修正。
    for (const path of Object.keys(drafts)) if (!errors[path]) delete drafts[path];
  },
);
const values = computed(() => {
  const result: Record<string, unknown> = {};
  try {
    for (const group of groups.value)
      for (const item of group.fields) result[item.path] = readJSONField(props.document, item.path);
    result["blog_extensions.search.provider"] = readJSONField(props.document, "blog_extensions.search.provider");
  } catch {
    // 完整 JSON 的语法诊断由父层处理；这里不修改共享草稿。
  }
  return result;
});
const current = (item: CanonicalSettingsField): any =>
  values.value[item.path] === undefined ? item.default : values.value[item.path];
const id = (item: CanonicalSettingsField) => `canonical-${item.path.replaceAll(".", "-")}`;
const inputValue = (event: Event) => (event.target as HTMLInputElement).value;
function change(item: CanonicalSettingsField, value: unknown, raw?: string) {
  delete errors[item.path];
  emit("change", item.path, value, raw);
}
function reset(item: CanonicalSettingsField) {
  structuredRevision[item.path] = (structuredRevision[item.path] ?? 0) + 1;
  delete structuredPending[item.path];
  delete errors[item.path];
  delete drafts[item.path];
  delete newKeys[item.path];
  delete newValues[item.path];
  change(item, undefined);
  if (item.kind === "provider") emit("change", "blog_extensions.search.provider", undefined);
}
function provider(item: CanonicalSettingsField): string {
  return effectiveSearchProvider(values.value[item.path], values.value["blog_extensions.search.provider"]);
}
function dependencyHint(item: CanonicalSettingsField): string | undefined {
  if (
    item.path.startsWith("theme.search.local.") &&
    effectiveSearchProvider(values.value["theme.search.provider"], values.value["blog_extensions.search.provider"]) !==
      "local"
  )
    return "当前搜索服务不使用此项；切换为本地静态索引后生效，仍可提前配置。";
  if (
    item.path.startsWith("blog_extensions.content.") &&
    item.path !== "blog_extensions.content.enabled" &&
    !values.value["blog_extensions.content.enabled"]
  )
    return "文章扩展尚未启用；此项会保留，开启文章扩展后才生效。";
  return undefined;
}
function updateProvider(event: Event) {
  changeProvider(inputValue(event));
}
function changeProvider(provider: string) {
  for (const [path, value] of searchProviderChanges(provider)) emit("change", path, value);
}
function selected(item: CanonicalSettingsField): string {
  return item.kind === "provider" ? provider(item) : String(current(item) ?? "");
}
function unknownOption(item: CanonicalSettingsField): boolean {
  return !item.options?.some(([value]) => value === selected(item));
}
function updateNumber(item: CanonicalSettingsField, event: Event) {
  const input = event.target as HTMLInputElement;
  if (input.validity.badInput) {
    errors[item.path] = "请填写有效数字。";
    return;
  }
  const value = input.value === "" ? undefined : Number(input.value);
  if (
    value !== undefined &&
    (!Number.isFinite(value) ||
      (item.step === 1 && !Number.isInteger(value)) ||
      (item.min !== undefined && value < item.min) ||
      (item.max !== undefined && value > item.max) ||
      (item.path.endsWith(".cover_ratio") && value <= 0))
  ) {
    errors[item.path] = item.path.endsWith(".cover_ratio")
      ? "宽高比必须大于 0。"
      : `请填写${item.step === 1 ? "整数" : "数字"}${item.min !== undefined ? `，最小为 ${item.min}` : ""}${item.max !== undefined ? `，最大为 ${item.max}` : ""}。`;
    return;
  }
  change(item, value);
}
function choices(item: CanonicalSettingsField): string[] {
  return Array.isArray(current(item)) ? current(item) : [];
}
function updateChoice(item: CanonicalSettingsField, value: string, event: Event) {
  const next = [...choices(item)];
  if ((event.target as HTMLInputElement).checked) next.push(value);
  else next.splice(next.indexOf(value), 1);
  change(item, next);
}
function rawText(item: CanonicalSettingsField): string {
  if (Object.hasOwn(drafts, item.path)) return drafts[item.path];
  try {
    return readJSONFieldRaw(props.document, item.path) ?? "{}";
  } catch {
    return "{}";
  }
}
function updateJSON(item: CanonicalSettingsField, event: Event) {
  const text = inputValue(event);
  drafts[item.path] = text;
  try {
    const value = JSON.parse(text);
    if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error();
    change(item, value, text);
  } catch {
    errors[item.path] = "请输入有效的 JSON 对象。此输入尚未同步，请修正后再保存；也可恢复默认。";
  }
}
function mapValue(item: CanonicalSettingsField): Record<string, any> {
  const value = current(item);
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}
function mapEdit(item: CanonicalSettingsField, oldKey: string, key: string, value: unknown) {
  const source = mapValue(item);
  if (!key.trim()) {
    errors[item.path] = "名称不能为空。";
    return;
  }
  if (oldKey !== key && Object.hasOwn(source, key)) {
    errors[item.path] = "名称已存在，请使用其他名称。";
    return;
  }
  // 保留 readJSONField 附带的来源 token，只改所选映射项；点号不会被当作路径分隔符。
  const next = { ...source };
  if (oldKey !== key) delete next[oldKey];
  Object.defineProperty(next, key, { value, enumerable: true, writable: true, configurable: true });
  change(item, next);
}
function mapRemove(item: CanonicalSettingsField, key: string) {
  const next = { ...mapValue(item) };
  delete next[key];
  change(item, next);
}
function mapAdd(item: CanonicalSettingsField) {
  const key = (newKeys[item.path] ?? "").trim();
  if (!key) {
    errors[item.path] = "请先填写名称。";
    return;
  }
  if (Object.hasOwn(mapValue(item), key)) {
    errors[item.path] = "名称已存在，请使用其他名称。";
    return;
  }
  mapEdit(item, "", key, newValues[item.path] ?? "");
  newKeys[item.path] = "";
  newValues[item.path] = "";
}
function licenseMode(item: CanonicalSettingsField): string {
  return values.value[item.path] === undefined ? "default" : values.value[item.path] === false ? "off" : "custom";
}
function updateLicense(item: CanonicalSettingsField, event: Event) {
  const mode = inputValue(event);
  change(
    item,
    mode === "default" ? undefined : mode === "off" ? false : typeof current(item) === "string" ? current(item) : "",
  );
}
function updateLabel(item: CanonicalSettingsField, key: string, value: string) {
  change(item, { ...mapValue(item), [key]: value });
}
function structuredSchema(item: CanonicalSettingsField): SettingSchema {
  if (item.path.startsWith("theme.")) return themeSettingSchema(item.path);
  if (item.path.endsWith(".show_posts"))
    return { type: ["boolean", "null"], default: { kind: "literal", value: true } };
  if (item.path.endsWith(".widgets"))
    return {
      type: ["object"],
      additionalProperties: {
        type: ["object"],
        additionalProperties: { type: ["string", "number", "boolean", "null", "object", "array"] },
      },
    };
  if (item.path.endsWith(".hero"))
    return {
      type: ["object", "null"],
      properties: { title: { type: ["string"] }, description: { type: ["string"] }, background: { type: ["string"] } },
      additionalProperties: { type: ["string", "number", "boolean", "null", "object", "array"] },
    };
  return {
    type: ["object"],
    additionalProperties: {
      type: ["object"],
      properties: {
        name: { type: ["string"] },
        avatar: { type: ["string"] },
        description: { type: ["string"] },
        url: { type: ["string"] },
        hidden: { type: ["boolean"] },
      },
    },
  };
}
function submit() {
  if (!props.disabled && !props.saving && !hasPending.value) emit("save");
}
</script>

<template>
  <form
    class="canonical-settings-form"
    :class="`settings-${tab}-form`"
    :data-settings-tab="tab"
    @submit.prevent="submit"
  >
    <fieldset :disabled="disabled" class="settings-form-fields">
      <section v-for="group in groups" :key="group.title" class="settings-section">
        <div v-if="tab !== 'appearance'" class="settings-section-heading">
          <h2>{{ group.title }}</h2>
          <p>{{ group.description }}</p>
        </div>
        <div class="settings-section-body">
          <div
            v-for="item in group.fields"
            :key="item.path"
            class="settings-field"
            :class="{ 'settings-field-switch': item.kind === 'switch' }"
            :data-setting-path="item.path"
          >
            <div class="settings-field-heading">
              <label :for="id(item)">{{ item.label }}</label>
              <button type="button" class="settings-reset" :aria-label="`${item.label}恢复默认`" @click="reset(item)">
                恢复默认
              </button>
            </div>
            <p v-if="dependencyHint(item)" class="settings-dependency">
              <Icon icon="ri:information-line" aria-hidden="true" />{{ dependencyHint(item) }}
            </p>
            <StructuredSetting
              v-if="item.kind === 'structured'"
              :key="structuredRevision[item.path] ?? 0"
              :schema="structuredSchema(item)"
              :show-heading="false"
              :model-value="values[item.path]"
              :default-value="item.path.startsWith('theme.') ? themeSettingDefault(item.path) : undefined"
              :label="item.label"
              :path="item.path"
              @update:model-value="change(item, $event)"
              @pending="structuredPending[item.path] = $event"
            />
            <VSwitch
              v-else-if="item.kind === 'switch'"
              :id="id(item)"
              :model-value="Boolean(current(item))"
              :aria-label="item.label"
              :disabled="disabled"
              @update:model-value="change(item, $event)"
            />
            <StructuredSettingChoices
              v-else-if="item.options && structuredSettingChoicesAvailable(item.path)"
              :path="item.path"
              :model-value="selected(item)"
              :options="item.options.map(([value]) => value)"
              @update:model-value="item.kind === 'provider' ? changeProvider(String($event)) : change(item, $event)"
            />
            <select
              v-else-if="item.kind === 'select' || item.kind === 'provider'"
              :id="id(item)"
              :value="selected(item)"
              class="apple-select"
              @change="item.kind === 'provider' ? updateProvider($event) : change(item, inputValue($event))"
            >
              <option v-if="unknownOption(item)" :value="selected(item)">当前值：{{ selected(item) }}</option>
              <option v-for="[value, label] in item.options" :key="value" :value="value">{{ label }}</option>
            </select>
            <input
              v-else-if="item.kind === 'number'"
              :id="id(item)"
              :value="current(item)"
              type="number"
              :min="item.min"
              :max="item.max"
              :step="item.step"
              class="apple-input"
              @input="updateNumber(item, $event)"
            />
            <div
              v-else-if="item.kind === 'choices'"
              :id="id(item)"
              class="settings-choices"
              role="group"
              :aria-label="item.label"
            >
              <label v-for="[value, label] in item.options" :key="value"
                ><input
                  type="checkbox"
                  :checked="choices(item).includes(value)"
                  @change="updateChoice(item, value, $event)"
                />{{ label }}</label
              >
              <span
                v-for="value in choices(item).filter((value) => !item.options?.some(([key]) => key === value))"
                :key="value"
                class="settings-help"
                >保留其他平台：{{ value }}</span
              >
            </div>
            <textarea
              v-else-if="item.kind === 'list'"
              :id="id(item)"
              :value="Array.isArray(current(item)) ? current(item).join('\n') : ''"
              class="apple-textarea"
              rows="3"
              @input="
                change(
                  item,
                  inputValue($event)
                    .split(/\r?\n/)
                    .map((line) => line.trim())
                    .filter(Boolean),
                )
              "
            ></textarea>
            <textarea
              v-else-if="item.kind === 'json'"
              :id="id(item)"
              :value="rawText(item)"
              class="apple-textarea code-style"
              rows="8"
              spellcheck="false"
              :aria-invalid="Boolean(errors[item.path])"
              :aria-describedby="errors[item.path] ? `${id(item)}-error` : undefined"
              @input="updateJSON(item, $event)"
            ></textarea>
            <textarea
              v-else-if="item.kind === 'textarea'"
              :id="id(item)"
              :value="current(item) ?? ''"
              class="apple-textarea code-style"
              rows="5"
              @input="change(item, inputValue($event))"
            ></textarea>
            <div v-else-if="item.kind === 'license'" class="settings-license">
              <select
                :id="id(item)"
                :value="licenseMode(item)"
                class="apple-select"
                @change="updateLicense(item, $event)"
              >
                <option value="default">使用主题默认许可</option>
                <option value="off">关闭许可声明</option>
                <option value="custom">自定义 Markdown</option>
              </select>
              <textarea
                v-if="licenseMode(item) === 'custom'"
                :value="current(item)"
                :aria-label="`${item.label} Markdown`"
                :placeholder="item.placeholder"
                class="apple-textarea"
                rows="4"
                @input="change(item, inputValue($event))"
              ></textarea>
              <p v-if="licenseMode(item) === 'default'" class="settings-help">{{ item.placeholder }}</p>
            </div>
            <div
              v-else-if="item.kind === 'map'"
              :id="id(item)"
              class="settings-map"
              role="group"
              :aria-label="item.label"
            >
              <div
                v-for="(value, key) in mapValue(item)"
                :key="key"
                class="settings-map-row"
                :class="{ 'settings-color-map-row': item.path === 'theme.article.category_colors' }"
              >
                <input
                  :value="key"
                  class="apple-input"
                  :aria-label="`${item.label}名称 ${key}`"
                  @change="mapEdit(item, String(key), inputValue($event), value)"
                />
                <SettingColorEditor
                  v-if="item.path === 'theme.article.category_colors'"
                  :id="`${id(item)}-color-${encodeURIComponent(String(key))}`"
                  :model-value="String(value ?? '')"
                  :label="`${item.label}值 ${key}`"
                  hex-only
                  @update:model-value="mapEdit(item, String(key), String(key), $event)"
                />
                <input
                  v-else
                  :value="value"
                  class="apple-input"
                  :aria-label="`${item.label}值 ${key}`"
                  @input="mapEdit(item, String(key), String(key), inputValue($event))"
                />
                <VButton
                  type="secondary"
                  size="sm"
                  :aria-label="`移除${item.label} ${key}`"
                  @click="mapRemove(item, String(key))"
                  >移除</VButton
                >
              </div>
              <div
                class="settings-map-row"
                :class="{ 'settings-color-map-row': item.path === 'theme.article.category_colors' }"
              >
                <input
                  v-model="newKeys[item.path]"
                  class="apple-input"
                  placeholder="名称"
                  :aria-label="`新增${item.label}名称`"
                />
                <SettingColorEditor
                  v-if="item.path === 'theme.article.category_colors'"
                  :id="`${id(item)}-new-color`"
                  :model-value="newValues[item.path] ?? ''"
                  :label="`新增${item.label}值`"
                  hex-only
                  @update:model-value="newValues[item.path] = $event"
                />
                <input
                  v-else
                  v-model="newValues[item.path]"
                  class="apple-input"
                  placeholder="值"
                  :aria-label="`新增${item.label}值`"
                />
                <VButton type="secondary" size="sm" @click="mapAdd(item)">添加</VButton>
              </div>
              <p v-if="newKeys[item.path] || newValues[item.path]" class="settings-help">
                请点击“添加”加入此项后再保存，或清空新增输入。
              </p>
            </div>
            <div
              v-else-if="item.kind === 'ai-label'"
              :id="id(item)"
              class="settings-label-options"
              role="group"
              :aria-label="item.label"
            >
              <div class="settings-switch">
                <span>显示此档位标记</span
                ><VSwitch
                  :model-value="current(item) !== null"
                  :aria-label="`显示${item.label}标记`"
                  :disabled="disabled"
                  @update:model-value="change(item, $event ? item.default : null)"
                />
              </div>
              <template v-if="current(item) !== null">
                <div class="settings-label-color">
                  <label :for="`${id(item)}-color`">颜色</label>
                  <SettingColorEditor
                    :id="`${id(item)}-color`"
                    :model-value="String(mapValue(item).color ?? '')"
                    :label="`${item.label}颜色`"
                    hex-only
                    @update:model-value="updateLabel(item, 'color', $event)"
                  />
                </div>
                <label
                  >图标<input
                    :value="mapValue(item).icon ?? ''"
                    class="apple-input"
                    :aria-label="`${item.label}图标`"
                    @input="updateLabel(item, 'icon', inputValue($event))"
                /></label>
              </template>
            </div>
            <div v-else-if="item.kind === 'image'" class="settings-image">
              <input
                :id="id(item)"
                :value="current(item) ?? ''"
                class="apple-input"
                placeholder="图片地址"
                @input="change(item, inputValue($event))"
              />
              <AttachmentField
                :model-value="current(item) ?? ''"
                show-thumb
                @update:model-value="change(item, $event)"
              />
            </div>
            <input
              v-else
              :id="id(item)"
              :value="current(item) ?? ''"
              :type="item.kind === 'password' ? 'password' : 'text'"
              class="apple-input"
              :autocomplete="item.kind === 'password' ? 'new-password' : undefined"
              @input="change(item, inputValue($event))"
            />
            <p
              v-if="settingFieldEffect(item.path, item.kind === 'provider' ? provider(item) : current(item))"
              class="settings-field-effect"
            >
              <Icon icon="ri:information-line" aria-hidden="true" />
              <span>{{
                settingFieldEffect(item.path, item.kind === "provider" ? provider(item) : current(item))
              }}</span>
            </p>
            <p v-if="item.help" class="settings-help">{{ item.help }}</p>
            <p v-if="errors[item.path]" :id="`${id(item)}-error`" class="settings-error" role="alert">
              {{ errors[item.path] }}
            </p>
          </div>
        </div>
      </section>
    </fieldset>
    <div class="settings-canonical-actions">
      <VButton
        type="primary"
        size="md"
        native-type="submit"
        :disabled="disabled || Boolean(saving) || hasPending"
        :loading="saving === 'stellar2'"
        ><template #icon><Icon icon="ri:save-line" /></template>{{ saveLabel }}</VButton
      >
      <p class="settings-help">{{ saveHelp }}</p>
      <p v-if="hasErrors" class="settings-error" role="alert">{{ invalidHelp }}</p>
      <p v-else-if="hasMapDraft" class="settings-help">请先添加或清空尚未加入的映射项。</p>
    </div>
  </form>
</template>

<style scoped>
.canonical-settings-form {
  background: transparent;
}
.settings-form-fields {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}
.settings-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}
.settings-section-heading {
  max-width: 860px;
  margin-bottom: 18px;
}
.settings-section-heading h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--color-foreground);
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
}
.settings-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 16px;
  border-radius: 2px;
  background: var(--color-primary);
  flex-shrink: 0;
}
.settings-section-heading p,
.settings-help {
  margin: 6px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.settings-section-body {
  max-width: 860px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
}
.settings-field {
  min-width: 0;
  max-width: 860px;
  margin: 0 0 20px;
  padding: 0 0 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}
.settings-field:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: 0;
}
.settings-field-switch {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  column-gap: 16px;
}
.settings-field-switch .settings-field-heading {
  margin-bottom: 0;
}
.settings-field-switch > :deep([role="switch"]) {
  grid-column: 2;
  grid-row: 1;
}
.settings-field-switch > p {
  grid-column: 1 / -1;
}
.settings-field-effect {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 8px 0 0;
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}
.settings-dependency {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 0 0 10px;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}
.settings-dependency svg {
  flex-shrink: 0;
  margin-top: 2px;
}
.settings-field-switch > .settings-dependency {
  grid-row: 2;
}
.settings-field-effect svg {
  flex-shrink: 0;
  margin-top: 3px;
  color: var(--color-primary);
}
.settings-field-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.settings-field-heading > label {
  margin: 0;
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
}
.settings-reset {
  flex-shrink: 0;
  min-height: 36px;
  padding: 6px 0 6px 8px;
  border: 0;
  background: transparent;
  color: var(--color-primary);
  font-size: 12px;
  cursor: pointer;
}
.apple-input,
.apple-select,
.apple-textarea {
  width: 100%;
  min-width: 0;
  max-width: 100%;
}
.code-style {
  font-family: var(--font-mono);
}
.settings-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
}
.settings-choices label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  color: var(--color-foreground);
  font-size: 14px;
}
.settings-choices input {
  accent-color: var(--color-primary);
}
.settings-license,
.settings-map,
.settings-label-options,
.settings-image {
  display: grid;
  gap: 12px;
}
.settings-map-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr) auto;
  align-items: center;
  gap: 12px;
}
.settings-color-map-row {
  align-items: start;
}
.settings-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: var(--color-foreground);
  font-size: 14px;
}
.settings-label-options > label,
.settings-label-color {
  display: grid;
  grid-template-columns: 50px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  color: var(--color-text-tertiary);
  font-size: 12px;
}
.settings-error {
  margin: 8px 0 0;
  color: var(--color-state-error);
  font-size: 12px;
  line-height: 1.5;
}
.settings-canonical-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: var(--color-card);
}
@media (max-width: 767px) {
  .settings-section {
    padding-left: 16px;
    padding-right: 16px;
  }
  .settings-canonical-actions {
    padding-left: 16px;
    padding-right: 16px;
  }
}
@media (max-width: 640px) {
  .settings-section {
    padding-top: 24px;
    padding-bottom: 24px;
  }
  .settings-map-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
  }
  .settings-map-row > input:first-child {
    grid-column: 1 / -1;
  }
}
</style>
