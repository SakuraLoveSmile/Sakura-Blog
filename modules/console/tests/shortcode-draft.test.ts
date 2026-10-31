import { test, mock } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import type { ShortcodeEditResult } from "@blog/api-client";
import type { PluginMeta } from "../web/src/modules/plugins/plugin-types.ts";
import { useShortcodeDraft } from "../web/src/modules/plugins/useShortcodeDraft.ts";

// 测试目录位于 workspace package 之外，按真实 Console 包解析同一份 Vue runtime。
const { effectScope } = createRequire(new URL("../web/package.json", import.meta.url))("vue");

const button: PluginMeta = {
  name: "button",
  title: "按钮",
  category: "提示标注",
  type: "inline",
  desc: "",
  signature: "",
  params: [],
  example: "{% button 示例 url:/docs %}",
  form: [
    { key: "text", label: "文本", type: "text", role: "positional" },
    { key: "url", label: "URL", type: "text", role: "named" },
    { key: "color", label: "配色", type: "text", role: "named", default: "blue" },
  ],
  edit: { enabled: true, required: ["text", "url"] },
};

const parsedButton = (text: string, url = "/docs"): ShortcodeEditResult => ({
  name: "button",
  editable: true,
  values: { text, url },
  present: ["text", "url"],
  variant: "inline",
});

function start(parse: (raw: string) => Promise<ShortcodeEditResult>) {
  const scope = effectScope();
  const draft = scope.run(() => useShortcodeDraft(() => button, parse))!;
  return { draft, stop: () => scope.stop() };
}

async function tick(ms = 0) {
  mock.timers.tick(ms);
  await Promise.resolve();
  await Promise.resolve();
}

test("标签源码同步 · 合法示例回填真实参数，不注入目录默认值或改写原文", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  const { draft, stop } = start(async () => parsedButton("示例"));
  try {
    draft.updateSource(button.example, true);
    assert.equal(draft.canApply.value, false);
    await tick();
    assert.equal(draft.raw.value, button.example);
    assert.deepEqual(draft.initialValues.value, { text: "示例", url: "/docs" });
    assert.deepEqual(draft.present.value, ["text", "url"]);
    assert.equal(draft.variant.value, "inline");
    assert.equal(draft.structured.value, true);
    assert.equal(draft.canApply.value, true);
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 手改新源码后旧解析不得覆盖当前表单", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  const pending: Array<(data: ShortcodeEditResult) => void> = [];
  const { draft, stop } = start(() => new Promise((resolve) => pending.push(resolve)));
  try {
    draft.updateSource("{% button old url:/old %}", true);
    await tick();
    draft.updateSource("{% button new url:/new %}", true);
    await tick();
    pending[1]!(parsedButton("new", "/new"));
    await tick();
    pending[0]!(parsedButton("old", "/old"));
    await tick();
    assert.deepEqual(draft.initialValues.value, { text: "new", url: "/new" });
    assert.equal(draft.raw.value, "{% button new url:/new %}");
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 新原文解析期间禁止旧表单生成结果覆盖源码", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  const { draft, stop } = start(async () => parsedButton("new"));
  try {
    draft.startInsertion();
    draft.updateValidation({ valid: true });
    draft.updateSource("{% button new url:/docs %}");
    assert.equal(draft.formPlugin.value, null, "解析期间销毁旧表单，禁止在途序列化写入新源码");
    draft.updateForm("{% button stale url:/old %}");
    draft.updateValidation({ valid: false, error: "old error" });
    assert.equal(draft.raw.value, "{% button new url:/docs %}");
    assert.equal(draft.canApply.value, false);
    await tick(250);
    assert.equal(draft.canApply.value, true);
    assert.equal(draft.formError.value, "");
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 复杂或不同标签保留源码模式，不用旧参数重新生成", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  const source = "{% button 文本 url:/docs custom:opaque %}";
  const { draft, stop } = start(async () => ({ name: "button", editable: false, values: {} }));
  try {
    draft.updateSource(source, true);
    await tick();
    assert.equal(draft.structured.value, false);
    assert.match(draft.sourceReason.value, /保留源码编辑/);
    assert.equal(draft.raw.value, source);
    assert.equal(draft.canApply.value, true);
    draft.updateForm("{% button overwritten %}");
    assert.equal(draft.raw.value, source);
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 缺必填字段时不能复制或插入上一次有效源码", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  const { draft, stop } = start(async () => parsedButton("示例"));
  try {
    draft.updateSource(button.example, true);
    await tick();
    draft.updateValidation({ valid: false, error: "请填写必填参数“url”。" });
    draft.updateForm("{% button stale url:/old %}");
    assert.equal(draft.canApply.value, false);
    assert.equal(draft.raw.value, button.example);
    assert.match(draft.formError.value, /url/);
    draft.updateValidation({ valid: true });
    draft.updateForm("{% button changed url:/new %}");
    assert.equal(draft.raw.value, "{% button changed url:/new %}");
    assert.equal(draft.canApply.value, true);
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 新插入使用空表单，迟到示例解析不能回填上一个选择", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  let resolve!: (result: ShortcodeEditResult) => void;
  const { draft, stop } = start(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  try {
    draft.updateSource(button.example, true);
    await tick();
    draft.startInsertion();
    resolve(parsedButton("old"));
    await tick();
    assert.equal(draft.raw.value, "");
    assert.equal(draft.initialValues.value, undefined);
    assert.equal(draft.present.value, undefined);
    assert.equal(draft.canApply.value, false);
    assert.match(draft.formError.value, /text/);
  } finally {
    stop();
    mock.timers.reset();
  }
});

test("标签源码同步 · 清空和卸载使在途请求失效", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  let resolve!: (result: ShortcodeEditResult) => void;
  const { draft, stop } = start(
    () =>
      new Promise((done) => {
        resolve = done;
      }),
  );
  try {
    draft.updateSource(button.example, true);
    await tick();
    draft.updateSource("");
    stop();
    resolve(parsedButton("late"));
    await tick();
    assert.equal(draft.raw.value, "");
    assert.equal(draft.initialValues.value, undefined);
    assert.equal(draft.structured.value, false);
    assert.equal(draft.parsing.value, false);
  } finally {
    stop();
    mock.timers.reset();
  }
});
