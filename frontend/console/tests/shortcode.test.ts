// 行为面：标签插件 shortcode 拼装（src/modules/plugins/shortcode.ts）。
// 机械校验入口：make test → console-test → npm run test → node --test tests/*.test.ts
// Node 22.18+ 原生剥离类型运行 .ts，零新增依赖；断言拼装规则本身，不经 Vue 组件。
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildShortcode } from "../src/modules/plugins/shortcode.ts";
import type { PluginMeta } from "../src/modules/plugins/plugins-data.ts";

const SURFACE = "shortcode 拼装";

// note 插件缩影：named 配色 + positional 标题 + content 正文（对照 plugins-data.ts 的 form schema）
const notePlugin: PluginMeta = {
  name: "note",
  title: "提示框 Note",
  category: "提示标注",
  type: "inline",
  desc: "",
  signature: "",
  params: [],
  example: "",
  form: [
    { key: "color", label: "配色", type: "select", role: "named", options: ["blue", "green"], default: "blue" },
    { key: "title", label: "标题", type: "text", role: "positional" },
    { key: "content", label: "内容", type: "textarea", role: "content" },
  ],
};

// 块级插件缩影：positional + flag + content
const foldingPlugin: PluginMeta = {
  name: "folding",
  title: "折叠框",
  category: "布局容器",
  type: "block",
  desc: "",
  signature: "",
  params: [],
  example: "",
  form: [
    { key: "title", label: "标题", type: "text", role: "positional" },
    { key: "open", label: "默认展开", type: "flag", role: "flag" },
    { key: "content", label: "内容", type: "textarea", role: "content" },
  ],
};

test(`${SURFACE} · inline：named + positional + content 按规则拼装`, () => {
  assert.equal(
    buildShortcode(notePlugin, { color: "green", title: "提示", content: "正文内容" }),
    "{% note 提示 color:green 正文内容 %}"
  );
});

test(`${SURFACE} · inline：named 值为空则整个 key:value 省略，content 为空不留悬挂空格`, () => {
  assert.equal(buildShortcode(notePlugin, { color: "", title: "", content: "" }), "{% note %}");
  assert.equal(buildShortcode(notePlugin, { color: " ", title: "标题", content: "" }), "{% note 标题 %}");
});

test(`${SURFACE} · block：content 包在 {% name %}…{% endname %} 之间并前后换行`, () => {
  assert.equal(
    buildShortcode(foldingPlugin, { title: "详情", open: "", content: "折叠正文" }),
    "{% folding 详情 %}\n折叠正文\n{% endfolding %}"
  );
});

test(`${SURFACE} · block：flag 为 true 仅拼 key，非 true 不拼`, () => {
  assert.equal(
    buildShortcode(foldingPlugin, { title: "详情", open: "true", content: "正文" }),
    "{% folding 详情 open %}\n正文\n{% endfolding %}"
  );
  assert.equal(
    buildShortcode(foldingPlugin, { title: "详情", open: "false", content: "正文" }),
    "{% folding 详情 %}\n正文\n{% endfolding %}"
  );
});

test(`${SURFACE} · block：content 为空时正文位置保留单个空格占位`, () => {
  assert.equal(
    buildShortcode(foldingPlugin, { title: "", open: "", content: "  " }),
    "{% folding %} {% endfolding %}"
  );
});

test(`${SURFACE} · 无 form schema 的插件产出裸标签`, () => {
  const bare: PluginMeta = { ...notePlugin, name: "hr", form: undefined };
  assert.equal(buildShortcode(bare, {}), "{% hr %}");
});
