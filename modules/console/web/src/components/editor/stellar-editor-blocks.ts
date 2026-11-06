import type { Node as ProseMirrorNode } from "@tiptap/pm/model";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet, type EditorView, type NodeView } from "@tiptap/pm/view";

export interface StellarCodeToken { from: number; to: number; className: string }
export interface StellarCodeBlock { text: string; language: string; tokens: StellarCodeToken[] }
export const stellarPageBlocksKey = new PluginKey<StellarCodeBlock[]>("stellar-page-blocks");

// 只接入 Chroma 的语法类别。正文中伪造的 class 不能带入布局、隐藏或交互类。
const tokenClasses = new Set(("err k kc kd kn kp kr kt n na nb bp nc no nd ni ne nf fm nl nn nx py nt nv vc vg vi vm l ld s sa sb sc dl sd s2 se sh si sx sr s1 ss m mb mf mh mi il mo o ow p c ch cm c1 cs cp cpf g gd ge gr gh gi go gp gs gu gt gl w").split(" "));
const safeLanguage = (value: unknown): string => typeof value === "string" && /^[a-z0-9_+-]{1,32}$/i.test(value) ? value.toLowerCase() : "";

/** 只读取前台的文本和有限 token 类，不把任何返回 HTML 接入同源编辑 DOM。 */
export function extractStellarCodeBlocks(document: Document): StellarCodeBlock[] {
  return [...document.querySelectorAll<HTMLElement>("article.md-text .highlight.chroma .code pre > code")].map((code) => {
    const container = code.closest(".highlight.chroma");
    const language = [...(container?.classList || [])].find((name) => name !== "chroma" && name !== "highlight") || "";
    const tokens: StellarCodeToken[] = [];
    let offset = 0;
    const visit = (node: Node) => {
      if (node.nodeType === 3) {
        offset += node.textContent?.length || 0;
        return;
      }
      if (node.nodeType !== 1) return;
      const element = node as Element;
      // script/style 等即使伪装在 highlight 中也不参与文字匹配。
      if (element.tagName !== "SPAN" && element !== code) return;
      const from = offset;
      for (const child of [...node.childNodes]) visit(child);
      const classes = [...element.classList].filter((name) => tokenClasses.has(name));
      if (classes.length && offset > from) tokens.push({ from, to: offset, className: classes.join(" ") });
    };
    visit(code);
    // 使用同一遍历的纯文本；不接受被跳过的主动内容形成的偏移。
    const text = [...code.childNodes].map(function plain(node): string {
      if (node.nodeType === 3) return node.textContent || "";
      if (node.nodeType !== 1 || (node as Element).tagName !== "SPAN") return "";
      return [...node.childNodes].map(plain).join("");
    }).join("");
    return { text, language: safeLanguage(language), tokens };
  });
}

function codeBlockView(node: ProseMirrorNode, view: EditorView): NodeView {
  const document = view.dom.ownerDocument;
  const dom = document.createElement("div");
  const table = document.createElement("table");
  table.className = "lntable";
  const body = document.createElement("tbody");
  const row = document.createElement("tr");
  const gutter = document.createElement("td");
  gutter.className = "lntd gutter";
  gutter.contentEditable = "false";
  gutter.setAttribute("aria-hidden", "true");
  const numbers = document.createElement("pre");
  numbers.className = "chroma";
  gutter.appendChild(numbers);
  const cell = document.createElement("td");
  cell.className = "lntd code";
  const pre = document.createElement("pre");
  pre.className = "chroma";
  const contentDOM = document.createElement("code");
  pre.appendChild(contentDOM);
  cell.appendChild(pre);
  row.append(gutter, cell);
  body.appendChild(row);
  table.appendChild(body);
  dom.appendChild(table);
  const update = (next: ProseMirrorNode) => {
    if (next.type !== node.type) return false;
    node = next;
    const language = safeLanguage(node.attrs.language);
    dom.className = `chroma highlight${language ? ` ${language}` : ""}`;
    contentDOM.setAttribute("data-language", language);
    numbers.replaceChildren();
    for (let index = 1; index <= node.textContent.split("\n").length; index++) {
      const number = document.createElement("span");
      number.className = "lnt";
      number.textContent = `${index}\n`;
      numbers.appendChild(number);
    }
    return true;
  };
  update(node);
  return {
    dom, contentDOM, update,
    ignoreMutation: (mutation) => mutation.type !== "selection" && !contentDOM.contains(mutation.target),
  };
}

function tableView(node: ProseMirrorNode, view: EditorView): NodeView {
  const document = view.dom.ownerDocument;
  const dom = document.createElement("div");
  dom.className = "md-table-scroll";
  const table = document.createElement("table");
  const contentDOM = document.createElement("tbody");
  table.appendChild(contentDOM);
  dom.appendChild(table);
  return {
    dom, contentDOM,
    update: (next) => next.type === node.type,
    ignoreMutation: (mutation) => mutation.type !== "selection" && !contentDOM.contains(mutation.target),
  };
}

/** 视觉包装仅存在于 NodeView/装饰，文档 schema 和 Markdown 序列化保持原样。 */
export function createStellarPageBlocksPlugin(): Plugin<StellarCodeBlock[]> {
  return new Plugin<StellarCodeBlock[]>({
    key: stellarPageBlocksKey,
    state: {
      init: () => [],
      apply: (transaction, previous) => {
        const next = transaction.getMeta(stellarPageBlocksKey);
        return Array.isArray(next) ? next : previous;
      },
    },
    props: {
      nodeViews: { codeBlock: codeBlockView, table: tableView },
      decorations(state) {
        const blocks = stellarPageBlocksKey.getState(state) || [];
        const decorations: Decoration[] = [];
        let nextBlock = 0;
        state.doc.descendants((node, position) => {
          if (node.type.name !== "codeBlock") return;
          const text = node.textContent;
          const language = safeLanguage(node.attrs.language);
          // shortcode 也可能渲染代码；顺序向前匹配正文与语言，拒绝迟到响应的旧 token。
          const match = blocks.findIndex((block, index) => index >= nextBlock && block.language === language
            && (block.text === text || block.text === `${text}\n`));
          if (match < 0) return false;
          nextBlock = match + 1;
          for (const token of blocks[match].tokens) {
            const to = Math.min(text.length, token.to);
            if (token.from >= 0 && token.from < to && Number.isInteger(token.from) && Number.isInteger(to)
              && token.className.split(" ").every((name) => tokenClasses.has(name))) {
              decorations.push(Decoration.inline(position + 1 + token.from, position + 1 + to, { class: token.className }));
            }
          }
          return false;
        });
        return DecorationSet.create(state.doc, decorations);
      },
    },
  });
}
