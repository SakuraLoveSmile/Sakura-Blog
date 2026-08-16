import CodeBlock from "@tiptap/extension-code-block";
import { mergeAttributes } from "@tiptap/core";

// 代码块扩展：在渲染出的 <code> 上补充 data-language 属性（值为原始语言名，如 "go"）。
// 纯 CSS 无法从 language-go 这类 class 中截出语言名，而悬停行首标记（```go）与
// 右上角语言标签都需要它：CSS attr(data-language) 直接取值，无语言时为空串。
// schema/解析/序列化与 StarterKit 内置 CodeBlock 完全一致，仅多输出一个属性。
export const CodeBlockWithLang = CodeBlock.extend({
  renderHTML({ node, HTMLAttributes }) {
    const attrs = mergeAttributes(HTMLAttributes, {
      class: node.attrs.language ? `${this.options.languageClassPrefix}${node.attrs.language}` : null,
      "data-language": node.attrs.language || "",
    });
    return ["pre", ["code", attrs, 0]];
  },
});
