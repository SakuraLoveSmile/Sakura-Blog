// 写作助手逐词差异：还原性、小改动的可读性、过大输入退回「只看结果」。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildInlineDiff,
  tokenize,
  tokenizeFallback,
  DIFF_MAX_EDIT,
  type InlineDiff,
} from "../web/src/components/editor/assistant/inline-diff.ts";

/** 任何差异都必须能还原两边的原文，这与分词方式无关。 */
function assertReconstructs(diff: InlineDiff, before: string, after: string) {
  const left = diff.segments
    .filter((s) => s.type !== "insert")
    .map((s) => s.text)
    .join("");
  const right = diff.segments
    .filter((s) => s.type !== "delete")
    .map((s) => s.text)
    .join("");
  assert.equal(left, before, "equal+delete must rebuild the original");
  assert.equal(right, after, "equal+insert must rebuild the result");
}

test("tokenize: joining tokens always restores the text", () => {
  const samples = [
    "",
    "写博客是整理思路的一种方式。",
    "Hello, world! 你好 😀 foo_bar-baz 123",
    "第一行\n\n第二行\t缩进  双空格",
    "{% note info %}\n标签\n{% endnote %}",
  ];
  for (const text of samples) {
    assert.equal(tokenize(text).join(""), text);
    assert.equal(tokenizeFallback(text).join(""), text);
  }
  assert.deepEqual(tokenizeFallback("ab 你好"), ["ab", " ", "你", "好"]);
});

test("inline diff: identical text is one equal segment", () => {
  const diff = buildInlineDiff("同样的内容", "同样的内容");
  assert.ok(diff);
  assert.deepEqual(diff.segments, [{ type: "equal", text: "同样的内容" }]);
  assert.equal(diff.similarity, 1);
  assert.equal(diff.added + diff.removed, 0);
  assert.deepEqual(buildInlineDiff("", "")?.segments, []);
});

test("inline diff: a small word swap stays small and readable", () => {
  const diff = buildInlineDiff("the quick brown fox", "the slow brown fox");
  assert.ok(diff);
  assert.deepEqual(diff.segments, [
    { type: "equal", text: "the " },
    { type: "delete", text: "quick" },
    { type: "insert", text: "slow" },
    { type: "equal", text: " brown fox" },
  ]);
  assert.equal(diff.added, 4);
  assert.equal(diff.removed, 5);
  assert.ok(diff.similarity > 0.7);
});

test("inline diff: insertion only and deletion only", () => {
  const added = buildInlineDiff("alpha gamma", "alpha beta gamma");
  assert.ok(added);
  assertReconstructs(added, "alpha gamma", "alpha beta gamma");
  assert.equal(added.removed, 0);
  assert.ok(added.added > 0);

  const removed = buildInlineDiff("alpha beta gamma", "alpha gamma");
  assert.ok(removed);
  assertReconstructs(removed, "alpha beta gamma", "alpha gamma");
  assert.equal(removed.added, 0);

  const fromEmpty = buildInlineDiff("", "全新的内容");
  assert.ok(fromEmpty);
  assert.deepEqual(fromEmpty.segments, [{ type: "insert", text: "全新的内容" }]);
  assert.equal(fromEmpty.similarity, 0);
});

test("inline diff: trivial unchanged gaps between edits are folded into the change", () => {
  const diff = buildInlineDiff("a b", "x y");
  assert.ok(diff);
  assert.deepEqual(diff.segments, [
    { type: "delete", text: "a b" },
    { type: "insert", text: "x y" },
  ]);
});

test("inline diff: Chinese, mixed script and emoji text reconstruct exactly", () => {
  const pairs: Array<[string, string]> = [
    ["写博客是整理思路的一种方式。我一直觉得很有用。", "写博客是梳理思路的一种方法。我一直觉得非常有用。"],
    ["Use Markdown 写作，配合静态站点😀。", "用 Markdown 写作，再配一个静态站点🙂！"],
    ["第一段\n\n第二段\n\n第三段", "第一段\n\n新增段\n\n第二段\n\n第三段"],
    ["{% note %}保持{% endnote %}", "{% note %}保持不变{% endnote %}"],
    ["", ""],
    ["abc", ""],
  ];
  for (const [before, after] of pairs) {
    const diff = buildInlineDiff(before, after);
    assert.ok(diff, `diff for ${JSON.stringify(before)}`);
    assertReconstructs(diff, before, after);
    assert.ok(diff.similarity >= 0 && diff.similarity <= 1);
  }
});

test("inline diff: pseudo-random edits always reconstruct", () => {
  let seed = 20261106;
  const next = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 2 ** 32;
  const words = ["写作", "博客", "整理", "思路", "alpha", "beta", "，", "。", " ", "\n", "😀", "{% tag %}"];
  const sentence = (n: number) => Array.from({ length: n }, () => words[Math.floor(next() * words.length)]).join("");
  for (let round = 0; round < 40; round++) {
    const before = sentence(30 + Math.floor(next() * 30));
    const parts = before.split("");
    for (let edit = 0; edit < 5; edit++) {
      const at = Math.floor(next() * parts.length);
      if (next() < 0.5) parts.splice(at, 1);
      else parts.splice(at, 0, words[Math.floor(next() * words.length)]);
    }
    const after = parts.join("");
    const diff = buildInlineDiff(before, after);
    assert.ok(diff, `round ${round}`);
    assertReconstructs(diff, before, after);
  }
});

test("inline diff: very different or oversized input falls back to null", () => {
  // 两边完全不同且很长：编辑距离超过上限，不再逐词对比。
  const before = Array.from({ length: DIFF_MAX_EDIT + 200 }, (_, i) => `a${i}`).join(" ");
  const after = Array.from({ length: DIFF_MAX_EDIT + 200 }, (_, i) => `b${i}`).join(" ");
  assert.equal(buildInlineDiff(before, after), null);
  // 词数之和超过上限。
  assert.equal(buildInlineDiff("a ".repeat(15000), "b ".repeat(15000)), null);
});

test("inline diff: an 8000-character polish is fast", () => {
  const base = "写博客是整理思路的一种方式，只有把想法写下来，才算真正想明白了。".repeat(250); // 约 8000 字
  assert.ok(base.length >= 7500);
  const edited = base.replace(/整理/g, "梳理").replace(/真正/g, "真的");
  const started = performance.now();
  const diff = buildInlineDiff(base, edited);
  const elapsed = performance.now() - started;
  assert.ok(diff);
  assertReconstructs(diff, base, edited);
  assert.ok(elapsed < 1500, `took ${elapsed.toFixed(0)}ms`);
});
