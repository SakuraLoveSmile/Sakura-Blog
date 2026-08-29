// 标签插件 shortcode 拼装纯逻辑（Spec 29，自 SchemaForm 适配器提取）。
// 独立成模块以便 node:test 行为测试（tests/shortcode.test.ts）直接导入，不经 Vue 组件。
// 拼装规则：positional 按字段顺序拼在标签名后；named 拼 key:value（值非空才拼）；
// flag 为真时拼 key:true（renderer 并非所有 flag 都接受裸 token）；
// content 作正文，块级插件包在 {% name %}…{% endname %} 之间。
import type { PluginFormField, PluginMeta, PluginVariant } from "./plugin-types";

export function buildShortcode(
  plugin: PluginMeta,
  values: Record<string, string>,
  variant?: PluginVariant,
): string {
  const form = plugin.form || [];
  const positional: string[] = [];
  const named: string[] = [];
  let content = "";
  for (const f of form) {
    const raw = (values[f.key] ?? "").trim();
    if (f.role === "content") {
      content = values[f.key] ?? "";
    } else if (f.role === "flag") {
      if (raw === "true") named.push(`${f.key}:true`);
    } else if (f.role === "named") {
      if (raw) named.push(`${f.key}:${raw}`);
    } else {
      if (raw) positional.push(raw);
    }
  }
  const head = [plugin.name, ...positional, ...named].join(" ");
  // hybrid 默认按块级拼装（插件库插入时取 canonical 块级形式），只有 variant=inline 时才单行。
  const isBlock = plugin.type === "block" || (plugin.type === "hybrid" && variant !== "inline");
  if (isBlock) {
    const body = content.trim() ? `\n${content.trim()}\n` : " ";
    return `{% ${head} %}${body}{% end${plugin.name} %}`;
  }
  // 单行 hybrid（navbar）正文含换行会破坏参数切分（splitArgs 只按空格/制表符切）
  const inline = plugin.type === "hybrid" ? content.trim().replace(/\s*\n\s*/g, " ") : content.trim();
  // inline 单标签插件（如 note）：正文作为最后一个位置参数拼入标签内
  return `{% ${inline ? `${head} ${inline}` : head} %}`;
}

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
