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
  canonicalFieldByPath,
  canonicalSettingsGroups,
  effectiveSearchProvider,
  searchProviderChanges,
  type CanonicalSettingsField,
  type CanonicalSettingsGroup,
  type CanonicalSettingsTab,
} from "./settings-canonical-fields";
import { canonicalFieldPaths, settingsPageGroups, settingsSectionAnchor, type SettingsPageKey } from "./settings-pages";
import { hasOverride, parseDocument } from "./settings-page-state";
import "./settings-form.css";

const props = withDefaults(
  defineProps<{
    /** 系统设置里的页面；AI 配置等非设置页场景改用 tab。 */
    page?: SettingsPageKey;
    tab?: CanonicalSettingsTab;
    document: string;
    /** 已保存的原文。传入后，相对它有改动的行会出现“未保存”橙点。 */
    saved?: string;
    saving?: string;
    disabled?: boolean;
    saveLabel?: string;
    saveHelp?: string;
    invalidHelp?: string;
  }>(),
  {
    page: undefined,
    tab: undefined,
    saved: undefined,
    saveLabel: "保存全部主题设置",
    saveHelp: "同时保存所有页面中尚未保存的主题修改；站点信息与访客反馈单独保存",
    invalidHelp: "请先修正上方输入问题，再保存主题设置。",
  },
);
const emit = defineEmits<{
  change: [path: string, value: unknown, rawValue?: string];
  save: [];
  pending: [value: boolean];
}>();
const formKey = computed(() => props.page ?? props.tab ?? "ai");
const groups = computed<CanonicalSettingsGroup[]>(() =>
  props.page ? settingsPageGroups(props.page) : canonicalSettingsGroups[props.tab ?? "ai"],
);
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
const blocked = computed(() =>
  hasErrors.value ? props.invalidHelp : hasMapDraft.value ? "请先添加或清空尚未加入的映射项。" : "",
);
watch(hasPending, (value) => emit("pending", value), { immediate: true });
watch(
  () => props.document,
  () => {
    // 已成功同步的输入跟随共享草稿；无效 JSON 留在原字段，等待用户修正。
    for (const path of Object.keys(drafts)) if (!errors[path]) delete drafts[path];
  },
);
// 依赖项可能属于别的页面（例如“列表与卡片”里的图标依赖“文章页”的文章扩展），所以这些路径总是读取。
const EXTRA_PATHS = ["blog_extensions.search.provider", "blog_extensions.content.enabled"];
const values = computed(() => {
  const result: Record<string, unknown> = {};
  try {
    for (const group of groups.value)
      for (const item of group.fields) result[item.path] = readJSONField(props.document, item.path);
    for (const path of EXTRA_PATHS) if (!(path in result)) result[path] = readJSONField(props.document, path);
  } catch {
    // 完整 JSON 的语法诊断由父层处理；这里不修改共享草稿。
  }
  return result;
});
const parsed = computed(() => parseDocument(props.document));
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
    return props.page && props.page !== "post"
      ? "文章扩展尚未启用（在「文章页 · 文章扩展」中开启）；此项会保留，开启后才生效。"
      : "文章扩展尚未启用；此项会保留，开启文章扩展后才生效。";
  return undefined;
}
// 同一分区里依赖项的说明只在第一项可见，其余只留给读屏，避免整页重复同一句话；变淡的样式仍逐行标示状态。
function firstDependent(group: CanonicalSettingsGroup): string | undefined {
  return group.fields.find((item) => dependencyHint(item))?.path;
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
// 选项少而短的选择直接平铺为分段选择；当前值不在选项里时回退为下拉，保证能看到并修正它。
function segmented(item: CanonicalSettingsField): boolean {
  const options = item.options ?? [];
  return (
    item.kind === "select" &&
    !structuredSettingChoicesAvailable(item.path) &&
    options.length > 1 &&
    options.length <= 4 &&
    options.every(([, label]) => label.length <= 8) &&
    !unknownOption(item)
  );
}
const BLOCK_KINDS = new Set(["structured", "map", "json", "textarea", "list", "ai-label", "license", "image"]);
// 名称与控件上下排布：大型编辑器、视觉选项卡、多选项。其余是左名称、右控件的一行。
function isBlock(item: CanonicalSettingsField): boolean {
  if (BLOCK_KINDS.has(item.kind)) return true;
  if (item.options && structuredSettingChoicesAvailable(item.path)) return true;
  return item.kind === "choices" && (item.options?.length ?? 0) > 4;
}
// 开关、数字这类很短的控件手机上也与名称同行；分段选择、下拉与输入框上下堆叠（分段选择占满整行），
// 否则较长的选项文字会把名称挤成窄窄的两三行。
const compactKinds = new Set(["switch", "number"]);
function isCompact(item: CanonicalSettingsField): boolean {
  return compactKinds.has(item.kind);
}
// 只有一个结构化字段的分区不再重复一行同名标题，字段说明和操作放进分区标题栏。
function solo(group: CanonicalSettingsGroup): CanonicalSettingsField | undefined {
  return group.fields.length === 1 && group.fields[0].kind === "structured" ? group.fields[0] : undefined;
}
const sectionId = (group: CanonicalSettingsGroup) =>
  props.page && group.id ? settingsSectionAnchor(props.page, group.id) : undefined;
function changed(item: CanonicalSettingsField): boolean {
  if (errors[item.path] || structuredPending[item.path] || newKeys[item.path] || newValues[item.path]) return true;
  if (props.saved === undefined) return false;
  try {
    return canonicalFieldPaths(item).some(
      (path) => readJSONFieldRaw(props.document, path) !== readJSONFieldRaw(props.saved!, path),
    );
  } catch {
    return false;
  }
}
// “恢复默认”就是移除草稿里的显式覆盖，所以只在存在覆盖（或有未同步输入）时提供。
function resettable(item: CanonicalSettingsField): boolean {
  if (errors[item.path] || structuredPending[item.path] || newKeys[item.path] || newValues[item.path]) return true;
  return hasOverride(parsed.value, item);
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
// 缩进示意：显式取值直接决定；“自动”时叙事风格（story）缩进、技术风格不缩进。非缩进字段返回 undefined。
function indented(item: CanonicalSettingsField): boolean | undefined {
  if (!item.path.endsWith(".paragraph_indent")) return undefined;
  const value = selected(item);
  if (value === "always" || value === "never") return value === "always";
  const style = values.value["theme.article.style"] ?? canonicalFieldByPath.get("theme.article.style")?.default;
  return style === "story";
}
function submit() {
  if (!props.disabled && !props.saving && !hasPending.value) emit("save");
}
</script>

<template>
  <form
    class="canonical-settings-form"
    :class="`settings-${formKey}-form`"
    :data-settings-tab="formKey"
    @submit.prevent="submit"
  >
    <fieldset :disabled="disabled" class="settings-form-fields">
      <section
        v-for="group in groups"
        :id="sectionId(group)"
        :key="group.title"
        class="settings-sec"
        :class="{ 'is-solo': solo(group) }"
        :data-setting-path="solo(group)?.path"
      >
        <div class="settings-sec-head">
          <div class="settings-sec-title">
            <h2>
              {{ group.title }}
              <span
                v-if="solo(group) && changed(solo(group)!)"
                class="settings-dot"
                role="img"
                aria-label="有未保存修改"
              ></span>
            </h2>
            <p>{{ group.description }}</p>
          </div>
          <div v-if="solo(group) && resettable(solo(group)!)" class="settings-sec-actions">
            <button
              type="button"
              class="settings-reset settings-row-reset"
              :aria-label="`${solo(group)!.label}恢复默认`"
              @click="reset(solo(group)!)"
            >
              恢复默认
            </button>
          </div>
        </div>
        <template v-if="solo(group)">
          <div class="settings-solo">
            <StructuredSetting
              :key="structuredRevision[solo(group)!.path] ?? 0"
              :schema="structuredSchema(solo(group)!)"
              :show-heading="false"
              :model-value="values[solo(group)!.path]"
              :default-value="
                solo(group)!.path.startsWith('theme.') ? themeSettingDefault(solo(group)!.path) : undefined
              "
              :label="solo(group)!.label"
              :path="solo(group)!.path"
              @update:model-value="change(solo(group)!, $event)"
              @pending="structuredPending[solo(group)!.path] = $event"
            />
            <p
              v-if="errors[solo(group)!.path]"
              :id="`${id(solo(group)!)}-error`"
              class="settings-error"
              role="alert"
            >
              {{ errors[solo(group)!.path] }}
            </p>
          </div>
        </template>
        <div v-else class="settings-rows">
          <div
            v-for="item in group.fields"
            :key="item.path"
            class="settings-row"
            :class="{
              'is-block': isBlock(item),
              'is-compact': isCompact(item),
              'is-dim': dependencyHint(item),
              'is-switch': item.kind === 'switch',
            }"
            :data-setting-path="item.path"
          >
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label :for="id(item)">{{ item.label }}</label>
                <span v-if="changed(item)" class="settings-dot" role="img" aria-label="有未保存修改"></span>
                <button
                  v-if="isBlock(item) && resettable(item)"
                  type="button"
                  class="settings-reset settings-row-reset"
                  :aria-label="`${item.label}恢复默认`"
                  @click="reset(item)"
                >
                  恢复默认
                </button>
              </div>
              <p
                v-if="dependencyHint(item)"
                class="settings-dependency"
                :class="{ 'is-quiet': item.path !== firstDependent(group) }"
              >
                <Icon icon="ri:information-line" aria-hidden="true" />{{ dependencyHint(item) }}
              </p>
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
            </div>
            <div class="settings-row-control">
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
              <div
                v-else-if="segmented(item)"
                :id="id(item)"
                class="settings-seg"
                role="radiogroup"
                :aria-label="item.label"
              >
                <label v-for="[value, label] in item.options" :key="value" :class="{ 'is-on': selected(item) === value }">
                  <input
                    type="radio"
                    :name="id(item)"
                    :value="value"
                    :checked="selected(item) === value"
                    @change="change(item, value)"
                  />
                  <span>{{ label }}</span>
                </label>
              </div>
              <select
                v-else-if="item.kind === 'select' || item.kind === 'provider'"
                :id="id(item)"
                :value="selected(item)"
                class="apple-select settings-control-select"
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
                class="apple-input settings-control-number"
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
                  class="apple-select settings-control-select"
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
                class="apple-input settings-control-text"
                :autocomplete="item.kind === 'password' ? 'new-password' : undefined"
                @input="change(item, inputValue($event))"
              />
              <button
                v-if="!isBlock(item) && resettable(item)"
                type="button"
                class="settings-reset settings-row-reset"
                :aria-label="`${item.label}恢复默认`"
                @click="reset(item)"
              >
                恢复默认
              </button>
            </div>
            <div
              v-if="indented(item) !== undefined"
              class="settings-indent-sample"
              role="img"
              aria-label="段落缩进效果示意"
            >
              <p :class="{ 'is-indented': indented(item) }">第一段正文示例：段落首行是否缩进，取决于上面的选择。</p>
              <p :class="{ 'is-indented': indented(item) }">第二段正文示例：技术类文章通常不缩进，叙事类文章通常缩进。</p>
              <span class="settings-indent-caption">当前效果：{{ indented(item) ? "首行缩进两字" : "首行不缩进" }}</span>
            </div>
            <p v-if="errors[item.path]" :id="`${id(item)}-error`" class="settings-error settings-row-error" role="alert">
              {{ errors[item.path] }}
            </p>
          </div>
        </div>
      </section>
    </fieldset>
    <slot name="after" />
    <slot
      name="actions"
      :blocked="blocked"
      :has-errors="hasErrors"
      :has-map-draft="hasMapDraft"
      :has-pending="hasPending"
    >
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
    </slot>
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
.settings-solo {
  min-width: 0;
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
@media (max-width: 767px) {
  /* 手机点击区：名称很短的选项（如“X”）标签不足 44px 宽；左右各补 8px 内边距，再用负外边距抵回，文字与勾选框位置不变。 */
  .settings-choices label {
    padding-inline: 8px;
    margin-inline: -8px;
  }
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
.settings-canonical-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  /* 横向留白由 .settings-container（桌面 32 / 手机 16）统一提供，操作条只补纵向安全区。 */
  padding: 16px 0 calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: var(--color-background);
}
@media (max-width: 640px) {
  .settings-map-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
  }
  .settings-map-row > input:first-child {
    grid-column: 1 / -1;
  }
}
</style>
