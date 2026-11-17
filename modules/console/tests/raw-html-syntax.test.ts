import { test } from "node:test";
import assert from "node:assert/strict";
import MarkdownIt from "markdown-it";
import {
  RAW_HTML_ATTR,
  RAW_HTML_INLINE_ATTR,
  isForeignHtml,
  rawHtmlPlaceholder,
  registerRawHtmlInlineRule,
  registerRawHtmlRule,
  scanHtmlTags,
} from "../web/src/components/editor/raw-html-syntax.ts";
import { decodeRaw } from "../web/src/components/editor/stellar-tag-syntax.ts";

function render(src: string): string {
  const md = new MarkdownIt({ html: true });
  registerRawHtmlRule(md);
  return md.render(src);
}

function renderBoth(src: string): string {
  const md = new MarkdownIt({ html: true });
  registerRawHtmlRule(md);
  registerRawHtmlInlineRule(md);
  return md.render(src);
}

function originals(html: string): string[] {
  const pattern = new RegExp(`<div ${RAW_HTML_ATTR}="([^"]*)"></div>`, "g");
  return Array.from(html.matchAll(pattern), (match) => decodeRaw(match[1]));
}

function inlineOriginals(html: string): string[] {
  const pattern = new RegExp(`<span ${RAW_HTML_INLINE_ATTR}="([^"]*)"></span>`, "g");
  return Array.from(html.matchAll(pattern), (match) => decodeRaw(match[1]));
}

test("占位编解码保住 Unicode 与引号", () => {
  const source = '<div class="卡片" data-note=\'他说 "你好" 👋\'>\n  <span>内容</span>\n</div>';
  assert.deepEqual(originals(rawHtmlPlaceholder(source)), [source]);
  assert.equal(originals('<div class="x">不是占位</div>').length, 0);
});

// 外部 HTML 必须逐字节保真：缩进、空行、大小写、属性顺序与引号风格都不能被规范化。
for (const source of [
  '<div class="card" style="color:red">\n  <span data-x="1">你好 👋</span>\n</div>',
  '<DIV  Class="A"   data-b=\'2\' >\n\t<iframe src="https://example.com"></iframe>\n</DIV>',
  '<section id="a">\n\n  <p style="margin:0">中间有空行</p>\n\n</section>',
  '<script>\nconst s = "<div class=\'x\'>";\n</script>',
  '<style>\n.a > .b { content: "<span>"; }\n</style>',
  '<!DOCTYPE html>\n<html lang="zh">\n<head><title>t</title></head>\n<body><p>x</p></body>\n</html>',
  '<img src="a.png" width="120" height="80">',
  '<input type="text" name="q">',
  '<h2 id="custom">自定义锚点</h2>',
]) {
  test(`外部 HTML 逐字保真：${source.slice(0, 42).replace(/\n/g, "\\n")}`, () => {
    const out = render(`${source}\n`);
    assert.deepEqual(originals(out), [source]);
  });
}

// 干净语义 HTML 沿用既有原生可编辑路径，不能被原子化退化成不透明卡片。
for (const source of [
  "<table>\n<tr><td>a</td></tr>\n</table>",
  "<table>\n<thead><tr><th>头</th></tr></thead>\n<tbody><tr><td>体</td></tr></tbody>\n</table>",
  "<p>普通段落</p>",
  "<blockquote>\n<p>引用</p>\n</blockquote>",
  "<ul>\n<li>一</li>\n<li>二</li>\n</ul>",
  '<pre><code class="language-go">fmt.Println("hi")</code></pre>',
  '<a href="https://example.com" title="t">链接</a>',
  '<img src="a.png" alt="图" title="题">',
  '<ol start="3">\n<li>三</li>\n</ol>',
  '<input type="checkbox" checked disabled>',
  "<hr>",
  "<p><strong>粗</strong> 与 <em>斜</em> 与 <del>删</del></p>",
]) {
  test(`原生语义 HTML 保持可编辑：${source.slice(0, 42).replace(/\n/g, "\\n")}`, () => {
    const out = render(`${source}\n`);
    assert.equal(originals(out).length, 0, "干净语义 HTML 不应被原子化");
  });
}

// 代码区与注释里的 HTML 示例不是内容，不能被吞成原子节点。
for (const source of [
  '```html\n<div class="x">hi</div>\n```',
  '~~~html\n<div class="x">hi</div>\n~~~',
  '````markdown\n```\n<div class="x">hi</div>\n```\n````',
  '> ```html\n> <div class="x">hi</div>\n> ```',
  '- ```html\n  <div class="x">hi</div>\n  ```',
  '    <div class="x">hi</div>',
  '\t<div class="x">hi</div>',
  '文本 `<div class="x">hi</div>` 结束',
  '<!-- <div class="x">示例</div> -->',
]) {
  test(`保护区不被接管：${source.slice(0, 42).replace(/\n/g, "\\n")}`, () => {
    assert.equal(originals(render(source)).length, 0);
  });
}

test("Stellar 标签占位不被二次接管", () => {
  const source = '<div data-stellar-tag="YWJj"></div>';
  const out = render(`${source}\n`);
  assert.equal(originals(out).length, 0);
  assert.ok(out.includes("data-stellar-tag"));
});

test("HTML 与 Stellar 标签混合块整体回退源码", () => {
  const source = '<div data-stellar-tag="YWJj"></div>\n<div class="x">a</div>';
  const out = render(`${source}\n`);
  assert.equal(originals(out).length, 0);
  assert.ok(out.includes('<div class="x">a</div>'));
});

test("重复解析幂等，占位不会累积或嵌套", () => {
  const source = '<section id="a">\n  <p style="x">hi</p>\n</section>';
  const once = render(`${source}\n`);
  assert.equal(render(once), once);
  assert.equal(render(render(once)), once);
  assert.deepEqual(originals(once), [source]);
});

test("同一文档内多个外部块各自保真且互不串台", () => {
  const first = '<div class="a">甲</div>';
  const second = '<div class="b">乙</div>';
  const out = render(`${first}\n\n中间段落\n\n${second}\n`);
  assert.deepEqual(originals(out), [first, second]);
  assert.ok(out.includes("<p>中间段落</p>"));
});

test("被空行切断的同一个元素并回一个节点，内部空行原样保留", () => {
  const source = '<section id="a">\n\n  <p style="margin:0">中间有空行</p>\n\n</section>';
  const out = render(`${source}\n`);
  assert.deepEqual(originals(out), [source]);
});

test("已配平的外部块不牵连邻近的干净语义块", () => {
  const foreign = '<div class="x">外部</div>';
  const out = render(`${foreign}\n\n<p>普通段落</p>\n`);
  assert.deepEqual(originals(out), [foreign]);
  assert.ok(out.includes("<p>普通段落</p>"), "干净段落必须保持原生可编辑");
});

test("doctype 与紧随其后的 html 元素并为一个节点", () => {
  const source = '<!DOCTYPE html>\n<html lang="zh">\n<body><p>x</p></body>\n</html>';
  assert.deepEqual(originals(render(`${source}\n`)), [source]);
});

// CommonMark 边界：非 type-6 标签且行内还有其他内容时属行内 HTML，不是 html_block。
// 块规则不得越界接管，行内规则负责保真。
for (const source of [
  '<video controls width="640"><source src="a.mp4" type="video/mp4"></video>',
  '<a href="https://example.com" class="btn">带 class 的链接</a>',
  '<code class="not-language">x</code>',
]) {
  test(`行内 HTML 由行内规则保真：${source.slice(0, 42)}`, () => {
    assert.equal(originals(render(`${source}\n`)).length, 0, "块规则不应接管行内 HTML");
    assert.deepEqual(inlineOriginals(renderBoth(`${source}\n`)), [source]);
  });
}

for (const source of [
  '前文 <span style="color:red">红字</span> 后文',
  '公式 H<sub>2</sub>O 与 x<sup>2</sup> 结束',
  '这里 <mark>高亮</mark> 一下',
  '段落 <iframe src="https://example.com" width="320"></iframe> 尾巴',
  '一个 <span class="a"><span class="b">嵌套</span></span> 结构',
]) {
  test(`段落内行内 HTML 保真：${source.slice(0, 42)}`, () => {
    const out = renderBoth(source);
    assert.equal(originals(out).length, 0);
    if (/<sub>|<sup>|<mark>/.test(source)) {
      // mark/sub/sup 由真正的 Mark 承担，渲染后仍是原标签，不该退化成 chip。
      assert.equal(inlineOriginals(out).length, 0);
      assert.ok(out.includes("<sub>") || out.includes("<sup>") || out.includes("<mark>"));
      return;
    }
    assert.ok(inlineOriginals(out).length > 0, "foreign 行内 HTML 必须保真");
  });
}

test("行内段含 markdown 格式标记时不接管，避免重建丢失 ** 原文", () => {
  const source = '看 <span style="x">**粗体**</span> 这里';
  const out = renderBoth(source);
  assert.equal(inlineOriginals(out).length, 0);
  assert.ok(out.includes("<strong>粗体</strong>"));
});

test("行内占位重复解析幂等", () => {
  const source = '前文 <span style="color:red">红字</span> 后文';
  const once = renderBoth(source);
  assert.equal(renderBoth(once), once);
});

test("块规则与行内规则共存互不干扰", () => {
  const block = '<div class="card">卡片</div>';
  const source = `${block}\n\n段落 <span style="x">行内</span> 结束\n`;
  const out = renderBoth(source);
  assert.deepEqual(originals(out), [block]);
  assert.deepEqual(inlineOriginals(out), ['<span style="x">行内</span>']);
});

test("重复注册规则保持幂等，被空行切断的块不会重复原子化", () => {
  const source = '<section id="a">\n\n  <p style="margin:0">中间有空行</p>\n\n</section>';
  // tiptap-markdown 会对同一实例多次调用 parse.setup；重复注册曾导致内容重复。
  for (const times of [2, 3]) {
    const md = new MarkdownIt({ html: true });
    for (let i = 0; i < times; i++) {
      registerRawHtmlRule(md);
      registerRawHtmlInlineRule(md);
    }
    assert.deepEqual(originals(md.render(`${source}\n`)), [source], `注册 ${times} 次后仍应只有一个节点`);
  }
});

test("mutation-check：改一个属性值，保真原文精确随之改变", () => {
  const before = '<div class="card" data-v="1">x</div>';
  const after = '<div class="card" data-v="2">x</div>';
  assert.deepEqual(originals(render(`${before}\n`)), [before]);
  assert.deepEqual(originals(render(`${after}\n`)), [after]);
  assert.notDeepEqual(originals(render(`${before}\n`)), originals(render(`${after}\n`)));
});

test("TaskItem 自身生成的 checkbox 标记不被原子化（任务列表往返回归）", () => {
  const generated = '<input class="task-list-item-checkbox" disabled="" type="checkbox">';
  assert.equal(isForeignHtml(generated), false);
  assert.equal(inlineOriginals(renderBoth(`- [ ] ${generated} pending`)).length, 0);
  // 其他 class 仍必须保真，不能因为放行 TaskItem 就放宽整个 input。
  assert.equal(isForeignHtml('<input class="custom" type="checkbox">'), true);
});

test("isForeignHtml 判定矩阵", () => {
  assert.equal(isForeignHtml(""), false);
  assert.equal(isForeignHtml("纯文本"), false);
  assert.equal(isForeignHtml("<!-- 注释 -->"), false);
  assert.equal(isForeignHtml("<p>x</p>"), false);
  assert.equal(isForeignHtml("<div>x</div>"), true);
  assert.equal(isForeignHtml("<span>x</span>"), true);
  assert.equal(isForeignHtml('<p style="x">y</p>'), true);
  assert.equal(isForeignHtml('<p onclick="x()">y</p>'), true);
  assert.equal(isForeignHtml('<p data-id="1">y</p>'), true);
  assert.equal(isForeignHtml('<p aria-label="x">y</p>'), true);
  assert.equal(isForeignHtml("<iframe src=\"x\"></iframe>"), true);
  assert.equal(isForeignHtml("<script>x</script>"), true);
  assert.equal(isForeignHtml('<code class="language-js">x</code>'), false);
  assert.equal(isForeignHtml('<code class="language-js other">x</code>'), true);
  assert.equal(isForeignHtml('<input type="checkbox">'), false);
  assert.equal(isForeignHtml('<input type="radio">'), true);
});

test("扫描器跳过原始文本元素内容，但保留其闭合标签", () => {
  const tags = scanHtmlTags('<script>var a = "<div class=\'x\'>";</script>');
  assert.deepEqual(tags.map((tag) => `${tag.name}:${tag.closing ? "close" : "open"}`), ["script:open", "script:close"]);
  const styled = scanHtmlTags('<style>.a { content: "<span>"; }</style>');
  assert.deepEqual(styled.map((tag) => tag.name), ["style", "style"]);
});

test("扫描器读出属性名与值，含无引号与布尔属性", () => {
  const [tag] = scanHtmlTags('<video controls width=640 data-x="a>b"></video>');
  assert.equal(tag.name, "video");
  assert.deepEqual(tag.attrs, [
    { name: "controls", value: "" },
    { name: "width", value: "640" },
    { name: "data-x", value: "a>b" },
  ]);
});

test("自闭合与 void 元素不被误判为未闭合", () => {
  const tags = scanHtmlTags('<br/><img src="a.png"><hr />');
  assert.deepEqual(tags.map((tag) => tag.name), ["br", "img", "hr"]);
  assert.equal(tags[0].selfClosing, true);
  assert.equal(tags[2].selfClosing, true);
});