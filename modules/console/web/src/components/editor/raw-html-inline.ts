import { Mark, Node } from "@tiptap/core";
import { VueNodeViewRenderer } from "@tiptap/vue-3";
import RawHtmlInlineView from "./RawHtmlInlineView.vue";
import { decodeRaw, encodeRaw } from "./stellar-tag-syntax";
import { RAW_HTML_INLINE_ATTR, registerRawHtmlInlineRule } from "./raw-html-syntax";

// 行内外部 HTML：schema 无法表示的 html_inline 段保真为 inline atom chip，
// 序列化原样回写 raw（不 closeBlock，避免把段落拆成块）。
export const RawHtmlInline = Node.create({
  name: "rawHtmlInline",
  group: "inline",
  inline: true,
  atom: true,

  addAttributes() {
    return {
      raw: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: `span[${RAW_HTML_INLINE_ATTR}]`,
        getAttrs: (el) => ({
          raw: decodeRaw((el as HTMLElement).getAttribute(RAW_HTML_INLINE_ATTR) || ""),
        }),
      },
    ];
  },

  renderHTML({ node }) {
    return ["span", { [RAW_HTML_INLINE_ATTR]: encodeRaw(node.attrs.raw) }];
  },

  addNodeView() {
    return VueNodeViewRenderer(RawHtmlInlineView);
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(node.attrs.raw);
        },
        parse: {
          setup(md: any) {
            registerRawHtmlInlineRule(md);
          },
        },
      },
    };
  },
});

// StarterKit 只带 bold/italic/strike/code；mark/sub/sup 是常见行内 HTML，
// 补成真正的 Mark 才能既渲染出效果又原样回写，而不是退化成 chip。
function htmlMark(name: string, tag: string) {
  return Mark.create({
    name,
    parseHTML() {
      return [{ tag }];
    },
    renderHTML() {
      return [tag, 0];
    },
    addStorage() {
      return {
        markdown: {
          serialize: { open: `<${tag}>`, close: `</${tag}>` },
        },
      };
    },
  });
}

export const MarkHighlight = htmlMark("markHighlight", "mark");
export const Subscript = htmlMark("subscript", "sub");
export const Superscript = htmlMark("superscript", "sup");