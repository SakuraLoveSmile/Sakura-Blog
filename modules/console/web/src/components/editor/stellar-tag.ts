import { Node, type NodeView as TipTapNodeView } from "@tiptap/core";
import { NodeSelection } from "@tiptap/pm/state";
import type { NodeView } from "@tiptap/pm/view";
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

/** 仅接管上游拖拽状态的 document 归属，其余节点事件规则仍交给 TipTap。 */
export function withOwnerDocumentDragEvents(nodeView: NodeView): NodeView {
  if (!nodeView.stopEvent) return nodeView;
  const view = nodeView as TipTapNodeView<unknown>;
  const stopEvent = view.stopEvent.bind(view);
  const destroy = nodeView.destroy?.bind(nodeView);
  let dragDocument: Document | null = null;
  const finishDrag = () => {
    view.isDragging = false;
    for (const type of ["dragend", "drop", "mouseup"]) dragDocument?.removeEventListener(type, finishDrag);
    dragDocument = null;
  };
  nodeView.stopEvent = (event) => {
    const target = event.target as HTMLElement;
    const dom = view.dom;
    if (event.type === "mousedown" && !view.isDragging && view.editor.isEditable && view.node.type.spec.draggable
      && NodeSelection.isSelectable(view.node) && dom?.contains(target) && !view.contentDOM?.contains(target)
      && !["INPUT", "BUTTON", "SELECT", "TEXTAREA"].includes(target.tagName) && !target.isContentEditable) {
      const handle = target.closest("[data-drag-handle]");
      if (handle && (dom === handle || dom.contains(handle))) {
        // Stellar atom 可选中：预置状态不会改变上游 mousedown 的 false 返回值。
        view.isDragging = true;
        dragDocument = dom.ownerDocument;
        for (const type of ["dragend", "drop", "mouseup"]) dragDocument.addEventListener(type, finishDrag);
      }
    }
    return stopEvent(event);
  };
  nodeView.destroy = () => { finishDrag(); destroy?.(); };
  return nodeView;
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
    const render = VueNodeViewRenderer(StellarTagView);
    return (props) => withOwnerDocumentDragEvents(render(props));
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
