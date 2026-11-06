// 写作助手核心逻辑：请求构建与截断、标题大纲、围栏感知的块边界、重新定位、标签校验、快捷键。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ASSIST_ACTIONS,
  ASSIST_LIMITS,
  assistBlockedReason,
  blockEnd,
  buildAssistRequest,
  countChars,
  diffApplicable,
  excerpt,
  extractOutline,
  firstChars,
  isAssistantHotkey,
  lastChars,
  normalizeResult,
  preferredView,
  relocate,
  relocateAfter,
  shortcodeTags,
  shortcodesChanged,
} from "../web/src/components/editor/assistant/assistant-core.ts";

// ---------- 字符计数与截取 ----------

test("chars: counts and slices by code point without splitting surrogate pairs", () => {
  assert.equal(countChars("a😀b"), 3);
  assert.equal(firstChars("a😀b😀c", 3), "a😀b");
  assert.equal(lastChars("a😀b😀c", 3), "b😀c");
  assert.equal(firstChars("abc", 10), "abc");
  assert.equal(lastChars("abc", 10), "abc");
  assert.equal(firstChars("abc", 0), "");
  assert.equal(lastChars("abc", 0), "");
  // 码元数超过 n 但码点数不够：整段返回。
  assert.equal(lastChars("😀😀", 3), "😀😀");
  assert.equal(firstChars("😀😀", 3), "😀😀");
  // 中文按字计。
  assert.equal(lastChars("写博客是整理思路", 4), "整理思路");
  assert.equal(firstChars("写博客是整理思路", 3), "写博客");
});

test("excerpt: collapses whitespace and truncates with an ellipsis", () => {
  assert.equal(excerpt("  第一行\n\n 第二行 "), "第一行 第二行");
  assert.equal(excerpt("一二三四五六七八九十", 5), "一二三四五…");
  assert.equal(excerpt("短文本", 5), "短文本");
});

// ---------- 大纲 ----------

test("outline: collects headings, skips fenced code, setext and deep indentation", () => {
  const md = [
    "# 一级标题",
    "正文",
    "## 二级标题 ##",
    "```md",
    "# 代码里的标题",
    "```",
    "    # 缩进四格不是标题",
    "~~~",
    "## 波浪围栏内",
    "~~~",
    "###### 六级",
    "#没有空格不是标题",
    "### 末尾标题",
  ].join("\n");
  assert.deepEqual(extractOutline(md), ["一级标题", "二级标题", "六级", "末尾标题"]);
  assert.deepEqual(extractOutline(""), []);
  // 围栏必须用同种标记、长度不短于开头才算关闭。
  assert.deepEqual(extractOutline("````\n```\n# 仍在围栏里\n````\n# 出来了"), ["出来了"]);
});

// ---------- 请求构建 ----------

test("request: selection actions carry the selection, context, title and outline", () => {
  const request = buildAssistRequest({
    action: "polish",
    context: { text: "要润色的一段", before: "前文。", after: "后文。" },
    title: " 我的文章 ",
    outline: ["引言", "", "  正文  "],
  });
  assert.deepEqual(request, {
    action: "polish",
    selection: "要润色的一段",
    before: "前文。",
    after: "后文。",
    title: "我的文章",
    outline: ["引言", "正文"],
  });
});

test("request: continue never sends the selection; empty fields are omitted", () => {
  const request = buildAssistRequest({
    action: "continue",
    context: { text: "误带的选区", before: "接着写", after: "" },
    title: "",
    outline: [],
  });
  assert.deepEqual(request, { action: "continue", before: "接着写" });
});

test("request: custom carries the instruction, translate carries the language", () => {
  const custom = buildAssistRequest({
    action: "custom",
    context: { text: "", before: "前文", after: "后文" },
    title: "T",
    outline: [],
    instruction: "  加一个例子  ",
    targetLang: "en",
  });
  assert.equal(custom.instruction, "加一个例子");
  assert.equal(custom.selection, undefined);
  assert.equal(custom.target_lang, undefined);

  const translate = buildAssistRequest({
    action: "translate",
    context: { text: "你好", before: "", after: "" },
    title: "",
    outline: [],
    instruction: "不会被带上",
    targetLang: "ja",
  });
  assert.equal(translate.target_lang, "ja");
  assert.equal(translate.instruction, undefined);
});

test("request: context is cut to the server limits so a huge document stays under the body cap", () => {
  const huge = "字".repeat(1_000_000);
  const request = buildAssistRequest({
    action: "polish",
    context: { text: "选区", before: huge, after: huge },
    title: "题".repeat(500),
    outline: Array.from({ length: 200 }, (_, i) => `标题${i}`.padEnd(300, "x")),
  });
  assert.equal(countChars(request.before ?? ""), ASSIST_LIMITS.before);
  assert.equal(countChars(request.after ?? ""), ASSIST_LIMITS.after);
  assert.equal(countChars(request.title ?? ""), ASSIST_LIMITS.title);
  assert.equal(request.outline?.length, ASSIST_LIMITS.outlineItems);
  assert.ok(request.outline?.every((item) => countChars(item) <= ASSIST_LIMITS.outlineItemChars));
  const bytes = new TextEncoder().encode(JSON.stringify(request)).length;
  assert.ok(bytes < 256 * 1024, `body is ${bytes} bytes`);
  // 前文保留靠近选区的末尾，后文保留靠近选区的开头。
  const tailed = buildAssistRequest({
    action: "polish",
    context: { text: "x", before: "旧".repeat(2000) + "近", after: "近" + "远".repeat(2000) },
    title: "",
    outline: [],
  });
  assert.ok(tailed.before?.endsWith("近"));
  assert.ok(tailed.after?.startsWith("近"));
});

test("blocked reason: explains why an action cannot run", () => {
  const base = { action: "polish" as const, selectionChars: 10, editable: true, instruction: "" };
  assert.equal(assistBlockedReason(base), "");
  assert.match(assistBlockedReason({ ...base, editable: false }), /编辑模式/);
  assert.match(assistBlockedReason({ ...base, selectionChars: 0 }), /选中/);
  assert.match(assistBlockedReason({ ...base, selectionChars: ASSIST_LIMITS.selection + 1 }), /8000/);
  // 续写不要选区，也不受选区长度限制。
  assert.equal(assistBlockedReason({ ...base, action: "continue", selectionChars: 0 }), "");
  assert.equal(
    assistBlockedReason({ ...base, action: "continue", selectionChars: ASSIST_LIMITS.selection + 1 }),
    "",
  );
  // 自定义指令必填且有长度上限，但可以没有选区。
  assert.match(assistBlockedReason({ ...base, action: "custom", selectionChars: 0 }), /输入/);
  assert.equal(assistBlockedReason({ ...base, action: "custom", selectionChars: 0, instruction: "加个例子" }), "");
  assert.match(
    assistBlockedReason({ ...base, action: "custom", instruction: "长".repeat(ASSIST_LIMITS.instruction + 1) }),
    /500/,
  );
  assert.deepEqual(
    ASSIST_ACTIONS.map((a) => a.id),
    ["polish", "rewrite", "expand", "shorten", "continue", "translate"],
  );
});

// ---------- 结果 ----------

test("result: normalizes newlines and trims blank lines but keeps inner structure", () => {
  assert.equal(normalizeResult("\r\n\r\n第一段\r\n\r\n第二段  \n\n"), "第一段\n\n第二段");
  assert.equal(normalizeResult("  缩进保留"), "  缩进保留");
  assert.equal(normalizeResult("\n\n"), "");
});

test("view: diff only applies to replace-style actions that have an original", () => {
  assert.equal(diffApplicable("polish", "replace"), true);
  assert.equal(diffApplicable("custom", "replace"), true);
  assert.equal(diffApplicable("translate", "replace"), false);
  assert.equal(diffApplicable("continue", "after-block"), false);
  assert.equal(diffApplicable("custom", "at-caret"), false);
  assert.equal(preferredView(0.9), "diff");
  assert.equal(preferredView(0.2), "result");
  assert.equal(preferredView(null), "result");
});

// ---------- 标签 ----------

test("shortcodes: detects added, removed and rewritten tags regardless of whitespace", () => {
  const original = "前文\n{% note info %}\n内容\n{% endnote %}\n后文";
  assert.deepEqual(shortcodeTags(original), ["{% note info %}", "{% endnote %}"]);
  assert.equal(shortcodesChanged(original, "改写\n{%  note   info %}\n新\n{% endnote %}"), false);
  assert.equal(shortcodesChanged(original, "把标签丢了"), true);
  assert.equal(shortcodesChanged(original, "{% note warning %}\n{% endnote %}"), true);
  assert.equal(shortcodesChanged("没有标签", "也没有标签"), false);
  assert.equal(shortcodesChanged("没有标签", "{% note %}凭空多出来{% endnote %}"), true);
});

// ---------- 块边界 ----------

test("blockEnd: ends at the blank line after the paragraph holding the caret", () => {
  const md = "第一段\n\n第二段第一行\n第二段第二行\n\n第三段";
  assert.equal(blockEnd(md, 0), 3);
  assert.equal(blockEnd(md, 2), 3);
  assert.equal(blockEnd(md, 3), 3);
  // 第二段跨两行：落在任意一行都到该段末尾。
  const secondStart = md.indexOf("第二段第一行");
  const secondEnd = md.indexOf("\n\n第三段");
  assert.equal(blockEnd(md, secondStart + 2), secondEnd);
  assert.equal(blockEnd(md, secondEnd), secondEnd);
  assert.equal(blockEnd(md, md.length), md.length);
});

test("blockEnd: a caret on a blank line continues the block above", () => {
  const md = "上面一段\n\n\n下面一段";
  assert.equal(blockEnd(md, md.indexOf("\n\n\n") + 1), "上面一段".length);
  assert.equal(blockEnd(md, md.indexOf("\n\n\n") + 2), "上面一段".length);
});

test("blockEnd: fenced code blocks keep their blank lines inside the block", () => {
  const md = "说明\n\n```js\nconst a = 1;\n\nconst b = 2;\n```\n\n结尾";
  const fenceEnd = md.indexOf("```\n\n结尾") + 3;
  // 光标在围栏中间的空行上。
  assert.equal(blockEnd(md, md.indexOf("\n\nconst b")), fenceEnd);
  // 光标在围栏里的代码行上。
  assert.equal(blockEnd(md, md.indexOf("const a") + 3), fenceEnd);
  // 围栏紧挨着段落时，整体算一块。
  const tight = "前面一行\n```\ncode\n```\n后面一行\n\n下一段";
  assert.equal(blockEnd(tight, 2), tight.indexOf("\n\n下一段"));
  // 没有闭合的围栏一直延伸到文末。
  const open = "说明\n\n```\n没有结尾\n\n还在代码里";
  assert.equal(blockEnd(open, open.indexOf("没有结尾")), open.length);
});

test("blockEnd: empty documents and trailing newlines", () => {
  assert.equal(blockEnd("", 0), 0);
  assert.equal(blockEnd("\n\n\n", 1), 0);
  assert.equal(blockEnd("只有一段\n", 5), 4);
  assert.equal(blockEnd("只有一段\n", 100), 4);
  assert.equal(blockEnd("甲\n\n乙\n", 4), 4);
});

// ---------- 重新定位 ----------

test("relocate: keeps the position when untouched, follows a unique move, refuses ambiguity", () => {
  assert.equal(relocate("前面ABC后面", "ABC", 2), 2);
  assert.equal(relocate("新增前面ABC后面", "ABC", 2), 4);
  assert.equal(relocate("ABC和ABC", "ABC", 1), -1);
  assert.equal(relocate("没有了", "ABC", 0), -1);
  assert.equal(relocate("任何", "", 0), -1);
  // 位置没动、内容有重复时仍认原位置。
  assert.equal(relocate("ABC和ABC", "ABC", 4), 4);
});

test("relocateAfter: finds the end of a unique tail, or the start of an empty one", () => {
  assert.equal(relocateAfter("一二三四五", "三四", 4), 4);
  assert.equal(relocateAfter("新增一二三四五", "三四", 4), 6);
  assert.equal(relocateAfter("三四和三四", "三四", 99), -1);
  assert.equal(relocateAfter("丢了", "三四", 4), -1);
  assert.equal(relocateAfter("任何内容", "", 0), 0);
  assert.equal(relocateAfter("任何内容", "", 3), -1);
});

// ---------- 快捷键 ----------

test("hotkey: Ctrl/⌘+J with the same modifier rule as the console search shortcut", () => {
  const press = (patch: Partial<Parameters<typeof isAssistantHotkey>[0]>) => ({
    key: "j",
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    ...patch,
  });
  assert.equal(isAssistantHotkey(press({ ctrlKey: true }), false), true);
  assert.equal(isAssistantHotkey(press({ metaKey: true }), true), true);
  assert.equal(isAssistantHotkey(press({ key: "J", ctrlKey: true }), false), true);
  // 苹果系统上 Ctrl+J 不触发（它是终端里的换行）。
  assert.equal(isAssistantHotkey(press({ ctrlKey: true }), true), false);
  assert.equal(isAssistantHotkey(press({}), false), false);
  assert.equal(isAssistantHotkey(press({ ctrlKey: true, shiftKey: true }), false), false);
  assert.equal(isAssistantHotkey(press({ ctrlKey: true, altKey: true }), false), false);
  assert.equal(isAssistantHotkey(press({ ctrlKey: true, key: "k" }), false), false);
  // 输入法组字中、长按重复都不触发。
  assert.equal(isAssistantHotkey(press({ ctrlKey: true, isComposing: true }), false), false);
  assert.equal(isAssistantHotkey(press({ ctrlKey: true, repeat: true }), false), false);
});
