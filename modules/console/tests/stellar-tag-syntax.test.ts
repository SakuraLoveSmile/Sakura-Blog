import { test } from "node:test";
import assert from "node:assert/strict";
import { decodeRaw, encodeRaw, wrapShortcodes } from "../web/src/components/editor/stellar-tag-syntax.ts";

function originals(rendered: string): string[] {
  return Array.from(rendered.matchAll(/<div data-stellar-tag="([^"]+)"><\/div>/g), (match) => decodeRaw(match[1]));
}

test("Unicode 标签原文可以编码并解码", () => {
  const source = '{% note "你好 👋" "显式 false 与空值" color: %}';
  assert.equal(decodeRaw(encodeRaw(source)), source);
});

for (const source of [
  "```markdown\n{% note Title Content %}\n```",
  "~~~markdown\n{% folding Example %}\nContent\n{% endfolding %}\n~~~",
  "````markdown\n```\n{% note Title Content %}\n```\n````",
  "> ```markdown\n> {% note Title Content %}\n> ```",
  "- ```markdown\n  {% note Title Content %}\n  ```",
  "    {% note Title Content %}",
  "\t{% note Title Content %}",
  "`{% note Title Content %}`",
  "``multiline\n{% note Title Content %}\ncode``",
  "```markdown\n{% note Title Content %}",
]) {
  test(`代码示例保持源码：${JSON.stringify(source)}`, () => {
    assert.equal(wrapShortcodes(source), source);
  });
}

for (const source of [
  "{% folding Outer %}\n{% dropdown Menu %}\n- [Home](/)\n{% enddropdown %}\n{% endfolding %}",
  "{% folding Outer %}\n{% folding Inner %}\nNested\n{% endfolding %}\n{% endfolding %}",
  "{% tabs %}\n<!-- tab Outer -->\n{% tabs %}\n<!-- tab Inner -->\nInner body\n{% endtabs %}\n{% endtabs %}",
  "{% folding Example %}\n```markdown\n{% endfolding %}\n```\nStill inside\n{% endfolding %}",
]) {
  test(`完整外层节点保留原始嵌套标签：${source.split("\n")[0]}`, () => {
    const output = wrapShortcodes(source);
    assert.deepEqual(originals(output), [source]);
    assert.doesNotMatch(originals(output)[0], /data-stellar-tag/);
  });
}

for (const source of [
  "{% note First Content %} {% note Second Content %}",
  "<!--\n{% note Title Content %}\n-->",
  "<pre>\n{% note Title Content %}\n</pre>",
]) {
  test(`不可独立可视化的结构保留源码：${JSON.stringify(source)}`, () => {
    assert.equal(wrapShortcodes(source), source);
  });
}

test("独立行内形式不会吞掉后面的块级标签", () => {
  const inline = "{% frame image.png %}";
  const block = "{% folding Later %}\nContent\n{% endfolding %}";
  assert.deepEqual(originals(wrapShortcodes(`${inline}\n\n${block}`)), [inline, block]);
});

test("关闭同名嵌套后，后续独立标签仍各自保留", () => {
  const first = "{% box %}\n{% box %}nested{% endbox %}\n{% endbox %}";
  const second = "{% image image.png %}";
  assert.deepEqual(originals(wrapShortcodes(`${first}\n\n${second}`)), [first, second]);
});

test("属性引号内的关闭符不会截断标签", () => {
  const source = '{% note "Title %} remains" "Content" %}';
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});

test("转义反引号不会把后面的独立标签当成代码", () => {
  const tag = "{% note Title Content %}";
  assert.deepEqual(originals(wrapShortcodes(`\\\`text\n${tag}\nend\``)), [tag]);
});

test("未闭合的行内代码不跨过空段落吞掉后面的标签", () => {
  const tag = "{% note Title Content %}";
  assert.deepEqual(originals(wrapShortcodes(`\`text\n${" ".repeat(200)}\n${tag}\nend\``)), [tag]);
});

test("可视化节点保留独立行原有空白和 CRLF", () => {
  const source = "  {% folding 标题 %}\r\n正文\r\n  {% endfolding %}  ";
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});

test("同名 hybrid 的单行与块级形式分别保留", () => {
  const single = "{% frame device:iphone img:/first.png %}";
  const block = "{% frame device:iphone %}\n![截图](/second.png)\n{% endframe %}";
  assert.deepEqual(originals(wrapShortcodes(`${single}\n\n${block}`)), [single, block]);
});

test("未闭合外层块不能暴露已闭合内层为可视化节点", () => {
  const source = "{% folding Outer %}\n{% box %}\nNested\n{% endbox %}";
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});

test("交错结束符即使随后补齐也保留原文节点", () => {
  const source = "{% folding Outer %}\n{% box %}\n{% endfolding %}\n{% endbox %}\n{% endfolding %}";
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});

test("混合 Markdown 的占位往返保留全文和显式空值 false", () => {
  const source = [
    "# 标题 👋",
    "",
    '```markdown\n{% note title:"示例" %}\n```',
    "",
    '{% folding title:"你好 %} 世界" open:false color:"" %}\n{% box %}\n正文\n{% endbox %}\n{% endfolding %}',
    "",
    "{% frame device:iphone img:/image.png %}",
    "",
    "普通正文 `inline {% note %}`。",
  ].join("\r\n");
  const wrapped = wrapShortcodes(source);
  assert.equal(originals(wrapped).length, 2);
  assert.equal(
    wrapped.replace(/<div data-stellar-tag="([^"]+)"><\/div>/g, (_match, value: string) => decodeRaw(value)),
    source,
  );
});

test("hybrid 闭合只归属最近同名开符，不干扰前面外层块中的单行", () => {
  const box = "{% box Outer %}\n{% frame device:iphone img:/inline.png %}\n{% endbox %}";
  const frame = "{% frame device:iphone %}\n![Screenshot](/block.png)\n{% endframe %}";
  assert.deepEqual(originals(wrapShortcodes(`${box}\n\n${frame}`)), [box, frame]);
});

test("hybrid 确实跨越外层结束符时仍保留交错原文节点", () => {
  const source = "{% box Outer %}\n{% frame device:iphone %}\n{% endbox %}\n{% endframe %}";
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});

// 仅返回源文本会让 TipTap 删除 HTML marker；原文节点才是源码回退的持久化边界。
for (const source of [
  "{% custom %}\n<!-- tab Keep -->\n{% note Title Body %}\n{% endcustom %}",
  "{% custom %}\n<!-- tab Keep -->\n{% note Title Body %}",
  "{% folding Outer %}\n<!-- tab Keep -->\n{% note Title Body %}",
  "{% folding Outer %}\n{% box %}\n<!-- tab Keep -->\n{% endfolding %}\n{% endbox %}",
  "{% folding Outer %}\n{% dropdown Inner %}\n{% endfolding %}\n{% enddropdown %}",
  "Before {% folding Inline %}text<!-- tab Keep -->{% endfolding %} after",
  '{% note title:"Unclosed %}\n<!-- tab Keep -->\n{% note Title Body %}',
  "{% custom missing-end\n<!-- tab Keep -->",
]) {
  test(`源码回退保留完整原文节点：${JSON.stringify(source)}`, () => {
    const output = wrapShortcodes(source);
    assert.deepEqual(originals(output), [source]);
    assert.doesNotMatch(output, /<!-- tab Keep -->/);
  });
}

test("同一行多个块级区间合并为一份完整原文，不重复截取", () => {
  const source = "Before {% box %}First<!-- tab Keep -->{% endbox %} {% folding %}Second{% endfolding %} after";
  assert.deepEqual(originals(wrapShortcodes(source)), [source]);
});
