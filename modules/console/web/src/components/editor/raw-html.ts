import { Node } from "@tiptap/core";
import { VueNodeViewRenderer } from "@tiptap/vue-3";
import RawHtmlView from "./RawHtmlView.vue";
import { withOwnerDocumentDragEvents } from "./stellar-tag";
import { decodeRaw, encodeRaw } from "./stellar-tag-syntax";
import { RAW_HTML_ATTR, registerRawHtmlRule } from "./raw-html-syntax";
import { rawHtmlPastePlugin } from "./raw-html-paste";

// 外部 HTML 的富文本保真节点（与 StellarTag 同构）。
// markdown-it core rule 把 TipTap schema 无法表示的 html_block 换成
// <div data-raw-html="BASE64(raw)"></div> 占位，解析为 atom 节点；
// NodeView 用 sandbox iframe 渲染真实效果，序列化原样回写 raw，round-trip 无损。
// 作者即可信：不做净化，script/iframe/style 原样保留，与后端 goldmark WithUnsafe 一致。
export const RawHtml = Node.create({
  name: "rawHtml",
  group: "block",
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      raw: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: `div[${RAW_HTML_ATTR}]`,
        getAttrs: (el) => ({
          raw: decodeRaw((el as HTMLElement).getAttribute(RAW_HTML_ATTR) || ""),
        }),
      },
    ];
  },

  renderHTML({ node }) {
    return ["div", { [RAW_HTML_ATTR]: encodeRaw(node.attrs.raw) }];
  },

  addNodeView() {
    const render = VueNodeViewRenderer(RawHtmlView);
    return (props) => withOwnerDocumentDragEvents(render(props));
  },

  addProseMirrorPlugins() {
    return [rawHtmlPastePlugin()];
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(node.attrs.raw);
          state.closeBlock(node);
        },
        parse: {
          setup(md: any) {
            registerRawHtmlRule(md);
          },
        },
      },
    };
  },
});