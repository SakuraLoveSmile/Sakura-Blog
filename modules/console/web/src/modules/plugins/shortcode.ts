// 标签插件 shortcode 拼装纯逻辑（Spec 29，自 SchemaForm 适配器提取）。
// 独立成模块以便 node:test 行为测试（tests/shortcode.test.ts）直接导入，不经 Vue 组件。
// 拼装规则：positional 按字段顺序拼在标签名后；named 拼 key:value（值非空才拼）；
// flag 为真时拼 key:true（renderer 并非所有 flag 都接受裸 token）；
// content 作正文，块级插件包在 {% name %}…{% endname %} 之间。
import type { PluginFormField, PluginMeta, PluginVariant } from "./plugin-types";

export interface ShortcodeBuildResult {
  raw: string;
  valid: boolean;
  error?: string;
}

function valueString(value: unknown): string {
  if (typeof value === "boolean") return value ? "true" : "false";
  return value == null ? "" : String(value);
}

/**
 * Serialize one non-content shortcode argument without inventing an escape
 * syntax. The parser understands matching single/double quotes, so use the
 * other quote when a value contains only one quote kind. Values containing
 * both quote kinds or a newline cannot be represented losslessly.
 */
export function serializeShortcodeArgument(value: unknown): { value: string; error?: string } {
  const trimmed = valueString(value).trim();
  if (!trimmed) return { value: "" };
  if (trimmed.includes("\n") || trimmed.includes("\r")) {
    return { value: "", error: "参数含换行，无法安全写入 shortcode；请切换源码编辑。" };
  }
  const hasSingle = trimmed.includes("'");
  const hasDouble = trimmed.includes('"');
  if (hasSingle && hasDouble) {
    return { value: "", error: "参数同时含单双引号，无法安全写入 shortcode；请切换源码编辑。" };
  }
  if (!/\s/.test(trimmed) && !hasSingle && !hasDouble) return { value: trimmed };
  const quote = hasSingle ? '"' : hasDouble ? "'" : '"';
  return { value: `${quote}${trimmed}${quote}` };
}

function validateEditValues(plugin: PluginMeta, values: Record<string, string | boolean>): string | undefined {
  const edit = plugin.edit;
  if (!edit?.enabled) return undefined;
  const text = (key: string) => valueString(values[key]).trim();
  for (const key of edit.required || []) {
    if (!text(key)) return `请填写必填参数“${key}”。`;
  }
  for (const group of edit.required_any || []) {
    if (!group.some((key) => text(key))) return `请至少填写一个参数：${group.join("、")}。`;
  }
  return undefined;
}

function buildShortcodeResult(
  plugin: PluginMeta,
  values: Record<string, string | boolean>,
  variant?: PluginVariant,
): ShortcodeBuildResult {
  const validationError = validateEditValues(plugin, values);
  if (validationError) return { raw: "", valid: false, error: validationError };

  const form = plugin.form || [];
  const emojiURL = plugin.name === "emoji" && valueString(values.url).trim() !== "";
  const positional: string[] = [];
  const named: string[] = [];
  let content = "";
  for (const f of form) {
    if (emojiURL && f.key !== "url" && f.key !== "name" && f.key !== "height") continue;
    const raw = valueString(values[f.key]).trim();
    if (f.role === "content") {
      content = valueString(values[f.key]);
      continue;
    }
    if (f.role === "flag") {
      if (values[f.key] === true || raw === "true") named.push(`${f.key}:true`);
      continue;
    }
    if (!raw) continue;
    const serialized = serializeShortcodeArgument(raw);
    if (serialized.error) return { raw: "", valid: false, error: `${f.label || f.key}：${serialized.error}` };
    const token = f.role === "named" ? `${f.key}:${serialized.value}` : serialized.value;
    (f.role === "named" ? named : positional).push(token);
  }

  // Emoji URL mode has a distinct grammar: url is named and source/name are
  // not positional. Keeping this branch explicit prevents editing height or
  // alt from dropping the external image URL.
  if (plugin.name === "emoji") {
    const url = valueString(values.url).trim();
    if (url) {
      const urlArg = serializeShortcodeArgument(url);
      if (urlArg.error) return { raw: "", valid: false, error: `图片 URL：${urlArg.error}` };
      const emojiNamed: string[] = [`url:${urlArg.value}`];
      for (const key of ["name", "height"]) {
        const value = valueString(values[key]).trim();
        if (!value) continue;
        const serialized = serializeShortcodeArgument(value);
        if (serialized.error) return { raw: "", valid: false, error: `${key}：${serialized.error}` };
        emojiNamed.push(`${key}:${serialized.value}`);
      }
      return { raw: `{% emoji ${emojiNamed.join(" ")} %}`, valid: true };
    }
  }

  const head = [plugin.name, ...positional, ...named].join(" ");
  // hybrid 默认按块级拼装（插件库插入时取 canonical 块级形式），只有 variant=inline 时才单行。
  const isBlock = plugin.type === "block" || (plugin.type === "hybrid" && variant !== "inline");
  if (isBlock) {
    const body = content.trim() ? `\n${content.trim()}\n` : " ";
    return { raw: `{% ${head} %}${body}{% end${plugin.name} %}`, valid: true };
  }
  // 单行 hybrid（navbar）正文含换行会破坏参数切分（splitArgs 只按空格/制表符切）
  const inline = plugin.type === "hybrid" ? content.trim().replace(/\s*\n\s*/g, " ") : content.trim();
  // inline 单标签插件（如 note）：正文作为最后一个位置参数拼入标签内
  return { raw: `{% ${inline ? `${head} ${inline}` : head} %}`, valid: true };
}

export function buildShortcode(
  plugin: PluginMeta,
  values: Record<string, string | boolean>,
  variant?: PluginVariant,
): string {
  return buildShortcodeResult(plugin, values, variant).raw;
}

export { buildShortcodeResult };

export type ShortcodeFormValues = Record<string, string | boolean>;

/**
 * 初始化属性表单的值。
 * 插入模式（initialValues 为 undefined）用 catalog default；
 * 编辑模式（initialValues 来自 /plugins/shortcode/parse）只取 raw 里真实解析出的值，
 * 缺失字段一律为空值——否则保存时会把作者从没写过的 default 静默写进正文。
 */
export function initFormValues(
  form: PluginFormField[],
  initialValues?: Record<string, unknown>,
): ShortcodeFormValues {
  const editing = initialValues !== undefined;
  const next: ShortcodeFormValues = {};
  for (const field of form) {
    const initial = initialValues?.[field.key];
    if (field.type === "flag") {
      next[field.key] = editing ? initial === true || initial === "true" : field.default === "true";
      continue;
    }
    if (editing) {
      next[field.key] = initial === undefined || initial === null ? "" : String(initial);
      continue;
    }
    next[field.key] = field.default ?? "";
  }
  return next;
}

// frame 单行旧语法的参数布局与块级（device/width/content）不同。
// 全项目仅此一处 hybrid form override，不为 hybrid 设计通用 VariantFormRegistry。
const FRAME_INLINE_FORM: PluginFormField[] = [
  { key: "device", label: "设备", type: "text", role: "positional", placeholder: "iphone / ipad / macbook" },
  { key: "img", label: "图片 URL", type: "text", role: "named" },
  { key: "video", label: "视频 URL", type: "text", role: "named" },
  { key: "focus", label: "焦点", type: "text", role: "named" },
  { key: "alt", label: "替代文本", type: "text", role: "positional" },
];

/** 按 variant 返回实际用于属性表单的插件定义（目前只影响单行 frame）。 */
export function formForVariant(plugin: PluginMeta, variant?: PluginVariant): PluginMeta {
  if (plugin.name === "frame" && variant === "inline") {
    return { ...plugin, form: FRAME_INLINE_FORM.map((field) => ({ ...field })) };
  }
  return plugin;
}
