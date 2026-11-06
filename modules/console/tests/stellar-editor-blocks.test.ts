import { test } from "node:test";
import assert from "node:assert/strict";
import { Schema } from "../web/node_modules/@tiptap/pm/model/dist/index.js";
import { EditorState } from "../web/node_modules/@tiptap/pm/state/dist/index.js";
import {
  createStellarPageBlocksPlugin,
  extractStellarCodeBlocks,
  stellarPageBlocksKey,
} from "../web/src/components/editor/stellar-editor-blocks.ts";

const schema = new Schema({ nodes: {
  doc: { content: "block+" },
  paragraph: { group: "block", content: "text*" },
  codeBlock: { group: "block", content: "text*", code: true, attrs: { language: { default: "" } } },
  table: { group: "block", content: "tableRow+" },
  tableRow: { content: "tableCell+" },
  tableCell: { content: "paragraph+" },
  text: { group: "inline" },
} });
const code = (text: string, language = "go") => schema.node("codeBlock", { language }, text ? schema.text(text) : undefined);

class TextNode {
  nodeType = 3;
  constructor(public textContent: string) {}
}
class ElementNode {
  nodeType = 1;
  className = "";
  childNodes: (ElementNode | TextNode)[] = [];
  parent: ElementNode | null = null;
  attributes = new Map<string, string>();
  contentEditable = "";
  constructor(public tagName: string) {}
  get classList() { return this.className.split(/\s+/).filter(Boolean); }
  get textContent(): string { return this.childNodes.map((child) => child.textContent).join(""); }
  set textContent(text: string) { this.childNodes = [new TextNode(text)]; }
  appendChild(child: ElementNode | TextNode) { this.childNodes.push(child); if (child instanceof ElementNode) child.parent = this; return child; }
  append(...children: (ElementNode | TextNode)[]) { children.forEach((child) => this.appendChild(child)); }
  replaceChildren() { this.childNodes = []; }
  setAttribute(name: string, value: string) { this.attributes.set(name, value); }
  contains(target: ElementNode | TextNode): boolean { return this === target || this.childNodes.some((child) => child === target || child instanceof ElementNode && child.contains(target)); }
  closest() { return this.parent; }
}
const element = (tag: string, className = "", text?: string) => {
  const node = new ElementNode(tag.toUpperCase()); node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};
const document = { createElement: (tag: string) => element(tag) };

test("服务端代码只提取文字和 Chroma token，拒绝主动内容及任意布局类", () => {
  const container = element("div", "chroma highlight go");
  const source = element("code");
  source.append(element("span", "kn hidden", "package"), new TextNode(" "), element("script", "", "alert(1)"), element("span", "nx fixed", "main"), new TextNode("\n"));
  container.appendChild(source);
  const result = extractStellarCodeBlocks({ querySelectorAll: () => [source] } as unknown as Document);
  assert.deepEqual(result, [{ text: "package main\n", language: "go", tokens: [
    { from: 0, to: 7, className: "kn" }, { from: 8, to: 12, className: "nx" },
  ] }]);
});

test("高亮只匹配相同正文与语言，终止换行不进入文档，也不着色迟到的旧正文", () => {
  const plugin = createStellarPageBlocksPlugin();
  const doc = schema.node("doc", null, [code("package main"), code("print(1)", "python")]);
  const before = doc.toJSON();
  let state = EditorState.create({ schema, doc, plugins: [plugin] });
  const transaction = state.tr.setMeta(stellarPageBlocksKey, [
    // shortcode 中生成的代码不可错套到后面的普通正文。
    { text: "other\n", language: "go", tokens: [{ from: 0, to: 5, className: "k" }] },
    { text: "package main\n", language: "go", tokens: [{ from: 0, to: 7, className: "kn" }, { from: 12, to: 13, className: "w" }] },
    { text: "print(1)\n", language: "go", tokens: [{ from: 0, to: 5, className: "nf" }] },
  ]);
  assert.equal(transaction.docChanged, false);
  state = state.apply(transaction);
  const decorations = plugin.props.decorations?.call(plugin, state)?.find() || [];
  assert.equal(decorations.length, 1);
  assert.deepEqual([decorations[0].from, decorations[0].to], [1, 8]);
  assert.equal((decorations[0].type as any).attrs.class, "kn");
  assert.deepEqual(state.doc.toJSON(), before);
  state = state.apply(state.tr.insertText("x", 1));
  assert.equal(plugin.props.decorations?.call(plugin, state)?.find().length, 0);
});

test("行号与表格包装只属于 NodeView，保留同一可编辑 DOM 与原 schema 数据", () => {
  const plugin = createStellarPageBlocksPlugin();
  const node = code("a\nb", 'go" onclick="bad');
  const before = node.toJSON();
  const view = { dom: { ownerDocument: document } } as any;
  const codeView = plugin.props.nodeViews!.codeBlock(node, view, () => 0, [], null as any);
  const dom = codeView.dom as unknown as ElementNode;
  const table = dom.childNodes[0] as ElementNode;
  const row = (table.childNodes[0] as ElementNode).childNodes[0] as ElementNode;
  const gutter = row.childNodes[0] as ElementNode;
  const content = codeView.contentDOM;
  assert.equal(dom.className, "chroma highlight");
  assert.equal(gutter.textContent, "1\n2\n");
  assert.equal(gutter.contentEditable, "false");
  assert.equal(gutter.attributes.get("aria-hidden"), "true");
  assert.equal((content as unknown as ElementNode).tagName, "CODE");
  assert.equal(codeView.update?.(code("a\nb\nc"), [], null as any), true);
  assert.equal(codeView.contentDOM, content);
  assert.equal(gutter.textContent, "1\n2\n3\n");
  assert.equal(codeView.ignoreMutation?.({ type: "childList", target: gutter } as any), true);
  assert.equal(codeView.ignoreMutation?.({ type: "characterData", target: content } as any), false);
  assert.deepEqual(node.toJSON(), before);

  const tableNode = schema.node("table", null, schema.node("tableRow", null, schema.node("tableCell", null, schema.node("paragraph"))));
  const tableBefore = tableNode.toJSON();
  const tableView = plugin.props.nodeViews!.table(tableNode, view, () => 0, [], null as any);
  assert.equal((tableView.dom as unknown as ElementNode).className, "md-table-scroll");
  assert.equal((tableView.contentDOM as unknown as ElementNode).tagName, "TBODY");
  assert.deepEqual(tableNode.toJSON(), tableBefore);
});
