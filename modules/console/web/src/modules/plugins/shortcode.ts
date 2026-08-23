// 标签插件 shortcode 拼装纯逻辑（Spec 29，自 SchemaForm 适配器提取）。
// 独立成模块以便 node:test 行为测试（tests/shortcode.test.ts）直接导入，不经 Vue 组件。
// 拼装规则：positional 按字段顺序拼在标签名后；named 拼 key:value（值非空才拼）；
// flag 为真时仅拼 key；content 作正文，块级插件包在 {% name %}…{% endname %} 之间。
import type { PluginMeta } from "./plugins-data";

export function buildShortcode(plugin: PluginMeta, values: Record<string, string>): string {
  const form = plugin.form || [];
  const positional: string[] = [];
  const named: string[] = [];
  let content = "";
  for (const f of form) {
    const raw = (values[f.key] ?? "").trim();
    if (f.role === "content") {
      content = values[f.key] ?? "";
    } else if (f.role === "flag") {
      if (raw === "true") named.push(f.key);
    } else if (f.role === "named") {
      if (raw) named.push(`${f.key}:${raw}`);
    } else {
      if (raw) positional.push(raw);
    }
  }
  const head = [plugin.name, ...positional, ...named].join(" ");
  if (plugin.type === "block") {
    const body = content.trim() ? `\n${content.trim()}\n` : " ";
    return `{% ${head} %}${body}{% end${plugin.name} %}`;
  }
  // inline 单标签插件（如 note）：正文作为最后一个位置参数拼入标签内
  const inline = content.trim() ? `${head} ${content.trim()}` : head;
  return `{% ${inline} %}`;
}
