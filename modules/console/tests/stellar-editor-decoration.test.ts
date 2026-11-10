import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { Schema } from "../web/node_modules/@tiptap/pm/model/dist/index.js";
import { EditorState } from "../web/node_modules/@tiptap/pm/state/dist/index.js";
import {
  createStellarPageInfoPlugin,
  exchangeSectionEnd,
  siteInfoHintText,
  stellarPageInfoKey,
} from "../web/src/components/editor/stellar-editor-decoration.ts";

const schema = new Schema({
  nodes: {
    doc: { content: "block+" },
    paragraph: { group: "block", content: "text*" },
    heading: { group: "block", content: "text*", attrs: { level: { default: 2 } } },
    stellarTag: { group: "block", atom: true, attrs: { raw: {} } },
    text: { group: "inline" },
  },
});
const heading = (text: string, level = 2) => schema.node("heading", { level }, schema.text(text));
const paragraph = (text: string) => schema.node("paragraph", null, schema.text(text));

test("本站信息放在交换分区末尾，三级标题继续属于交换分区", () => {
  const before = [heading("如何交换友链"), paragraph("交换说明"), heading("附加说明", 3), paragraph("资料")];
  const position = before.reduce((size, node) => size + node.nodeSize, 0);
  for (const level of [1, 2]) {
    const doc = schema.node("doc", null, [...before, heading("其他内容", level), paragraph("后续")]);
    assert.equal(exchangeSectionEnd(doc), position);
  }
});

test("没有后续分区时放在正文末尾，没有交换标题时不添加资料", () => {
  const doc = schema.node("doc", null, [heading("如何交换友链"), paragraph("正文")]);
  assert.equal(exchangeSectionEnd(doc), doc.content.size);
  assert.equal(exchangeSectionEnd(schema.node("doc", null, [heading("友情链接"), paragraph("正文")])), null);
  assert.equal(exchangeSectionEnd(schema.node("doc", null, [heading("交换友链", 3), paragraph("正文")])), null);
});

test("本站信息仅为只读装饰，不改正文或未知 shortcode，也不生成文档变更事务", () => {
  const raw = '{% unknown-card arbitrary="保留" %}原文{% endunknown-card %}';
  const doc = schema.node("doc", null, [heading("如何交换友链"), schema.node("stellarTag", { raw }), heading("失联友链")]);
  const original = doc.toJSON();
  const plugin = createStellarPageInfoPlugin();
  let state = EditorState.create({ schema, doc, plugins: [plugin] });
  const html = '<section class="friends-site-info"><h2>本站信息</h2></section>';
  const transaction = state.tr.setMeta(stellarPageInfoKey, html);
  assert.equal(transaction.docChanged, false);
  state = state.apply(transaction);
  assert.deepEqual(state.doc.toJSON(), original);
  assert.equal(stellarPageInfoKey.getState(state), html);
  const decorations = plugin.props.decorations?.call(plugin, state)?.find() || [];
  assert.equal(decorations.length, 1);
  assert.equal(decorations[0].from, doc.child(0).nodeSize + doc.child(1).nodeSize);
  assert.equal(decorations[0].spec.ignoreSelection, true);
  assert.equal(decorations[0].spec.stopEvent(), true);

  state = state.apply(state.tr.setMeta(stellarPageInfoKey, ""));
  assert.equal(plugin.props.decorations?.call(plugin, state)?.find().length, 0);
  assert.deepEqual(state.doc.toJSON(), original);
});

test("只读提示由编辑器宿主添加，位于本站资料块上方且不来自页面片段", () => {
  assert.match(siteInfoHintText, /^只读/);
  assert.match(siteInfoHintText, /设置/);
  assert.match(siteInfoHintText, /站点信息/);
  // 提示 DOM 由宿主创建，因此这里的结构约定只能静态核对；真实渲染由浏览器验收覆盖。
  const sourceFile = new URL("../web/src/components/editor/stellar-editor-decoration.ts", import.meta.url);
  const source = readFileSync(sourceFile, "utf8");
  assert.match(source, /wrapper\.className = "stellar-site-info-widget"/);
  assert.match(source, /hint\.textContent = siteInfoHintText/);
  assert.match(source, /wrapper\.append\(hint, ownerDocument\.importNode\(source, true\)\)/);
  assert.match(source, /wrapper\.contentEditable = "false"/);
});
