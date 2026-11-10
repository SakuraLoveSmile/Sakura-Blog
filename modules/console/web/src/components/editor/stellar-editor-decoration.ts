import type { Node as ProseMirrorNode } from "@tiptap/pm/model";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";

export const stellarPageInfoKey = new PluginKey<string>("stellar-page-site-info");

/** 对齐友链宿主：交换分区末尾、下一处同级/更高标题之前。 */
export function exchangeSectionEnd(doc: ProseMirrorNode): number | null {
  let found = false;
  let result: number | null = null;
  doc.forEach((node, offset) => {
    if (result !== null) return;
    if (found && node.type.name === "heading" && Number(node.attrs.level) <= 2) {
      result = offset;
      return;
    }
    if (!found) {
      const matches = (candidate: ProseMirrorNode) =>
        candidate.type.name === "heading" && Number(candidate.attrs.level) === 2 && candidate.textContent.includes("交换友链");
      found = matches(node);
      if (!found) node.descendants((child) => { if (matches(child)) found = true; });
    }
  });
  return result ?? (found ? doc.content.size : null);
}

/** 编辑器专用提示：这一块由站点设置生成，正文无法直接改写。 */
export const siteInfoHintText = "只读 · 由「设置 → 站点信息」自动生成";

/** 仅接入后端生成的信息块，拒绝将任何主动内容带入同源编辑画布。 */
export function siteInfoWidget(html: string, ownerDocument: Document): HTMLElement {
  const parsed = ownerDocument.implementation.createHTMLDocument("");
  parsed.body.innerHTML = html;
  const source = parsed.querySelector<HTMLElement>(".friends-site-info");
  const wrapper = ownerDocument.createElement("div");
  wrapper.className = "stellar-site-info-widget";
  if (!source || source.tagName !== "SECTION") return wrapper;
  const allowed = new Set(["SECTION", "H2", "P", "DIV", "PRE", "CODE", "STRONG"]);
  for (const element of [source, ...source.querySelectorAll<HTMLElement>("*")]) {
    if (!allowed.has(element.tagName)) {
      element.remove();
      continue;
    }
    for (const attribute of [...element.attributes]) {
      if (!["class", "id", "role", "aria-label", "aria-labelledby"].includes(attribute.name)) element.removeAttribute(attribute.name);
    }
  }
  // 提示行只属于编辑器界面，由宿主创建，不来自服务端返回的页面片段。
  const hint = ownerDocument.createElement("div");
  hint.className = "stellar-site-info-hint";
  hint.setAttribute("role", "note");
  hint.textContent = siteInfoHintText;
  wrapper.contentEditable = "false";
  wrapper.setAttribute("aria-readonly", "true");
  wrapper.append(hint, ownerDocument.importNode(source, true));
  return wrapper;
}

export function createStellarPageInfoPlugin(): Plugin<string> {
  return new Plugin<string>({
    key: stellarPageInfoKey,
    state: {
      init: () => "",
      apply: (transaction, previous) => {
        const next = transaction.getMeta(stellarPageInfoKey);
        return typeof next === "string" ? next : previous;
      },
    },
    props: {
      decorations(state) {
        const html = stellarPageInfoKey.getState(state);
        const position = exchangeSectionEnd(state.doc);
        if (!html || position === null) return DecorationSet.empty;
        return DecorationSet.create(state.doc, [Decoration.widget(position, (view) => siteInfoWidget(html, view.dom.ownerDocument), {
          key: html, side: -1, ignoreSelection: true, stopEvent: () => true,
        })]);
      },
    },
  });
}
