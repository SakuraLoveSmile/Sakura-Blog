import { Plugin } from "@tiptap/pm/state";
import { isForeignHtml, rawHtmlPlaceholder } from "./raw-html-syntax";

// 外部网页粘贴走 text/html，ProseMirror 默认用 schema 解析 DOM，未知标签与 style/class 会被削掉。
// 这里在解析前逐顶层块判定：超出 schema 表示能力的块换成 atom 占位（与 core rule 共用同一判定），
// 干净语义块保留原生可编辑。整文档粘贴整体保真为一个节点。

/** 完整 HTML 文档（含 doctype 或 html 根）：head 里的 style/meta 也要保住，整体作为一个片段。 */
export function isFullDocument(html: string): boolean {
  return /<!doctype\s+html/i.test(html) || /<html[\s>]/i.test(html);
}

function escapeText(text: string): string {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function rawHtmlPastePlugin(): Plugin {
  return new Plugin({
    props: {
      transformPastedHTML(html: string): string {
        if (!html.trim()) return html;
        if (isFullDocument(html)) return rawHtmlPlaceholder(html);
        const doc = new DOMParser().parseFromString(html, "text/html");
        let out = "";
        for (const node of Array.from(doc.body.childNodes)) {
          if (node.nodeType === 3) {
            out += escapeText(node.textContent || "");
            continue;
          }
          if (node.nodeType === 8) {
            out += `<!--${node.nodeValue || ""}-->`;
            continue;
          }
          if (node.nodeType !== 1) continue;
          const element = node as HTMLElement;
          const serialized = element.outerHTML;
          out += isForeignHtml(serialized) ? rawHtmlPlaceholder(serialized) : serialized;
        }
        return out || html;
      },
    },
  });
}