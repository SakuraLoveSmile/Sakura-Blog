import { Node } from "@tiptap/core";
import { VueNodeViewRenderer } from "@tiptap/vue-3";
import { runtimePlugins } from "@/modules/plugins/usePluginCatalog";
import StellarTagView from "./StellarTagView.vue";
import { decodeRaw, encodeRaw, tagNameOf, wrapShortcodes } from "./stellar-tag-syntax";
export { decodeRaw, encodeRaw, tagNameOf, wrapShortcodes } from "./stellar-tag-syntax";

// Stellar 标签插件的富文本可视化节点（Spec：Stellar 标签富文本可视化）。
// 原理：markdown-it core rule 把独占整行/整块的 {% ... %} 预替换为
// <div data-stellar-tag="BASE64(raw)"></div> 占位（html:true 下按 html_block 透传），
// TipTap parseHTML 将其解析为 atom 节点；NodeView 用 iframe 渲染前台真实效果；
// 序列化时原样写回 raw，round-trip 无损。

// 标签名 → 中文标题映射（来自插件库元数据），供气泡类型标签显示
/** 类型标签中文名（如 note → 「提示框 Note」）；未命中回退标签名 */
export function typeLabelOf(raw: string): string {
  const name = tagNameOf(raw);
  return runtimePlugins.value.find((plugin) => plugin.name === name)?.title || name;
}

export const StellarTag = Node.create({
  name: "stellarTag",
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
        tag: "div[data-stellar-tag]",
        getAttrs: (el) => ({
          raw: decodeRaw((el as HTMLElement).getAttribute("data-stellar-tag") || ""),
        }),
      },
    ];
  },

  renderHTML({ node }) {
    return ["div", { "data-stellar-tag": encodeRaw(node.attrs.raw) }];
  },

  addNodeView() {
    return VueNodeViewRenderer(StellarTagView);
  },

  // tiptap-markdown 集成：serialize 原样回写 raw；parse.setup 注册 core rule
  // 覆盖初始内容 / setContent / insertContent / 粘贴全部解析路径
  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(node.attrs.raw);
          state.closeBlock(node);
        },
        parse: {
          setup(md: any) {
            md.core.ruler.before("normalize", "stellar_tag", (state: any) => {
              state.src = wrapShortcodes(state.src);
            });
          },
        },
      },
    };
  },
});
