// 行为面：标签插件 shortcode 拼装（src/modules/plugins/shortcode.ts）。
// 机械校验入口：make test → console-test → pnpm run test → node --test ../tests/*.test.ts
// Node 22.18+ 原生剥离类型运行 .ts，零新增依赖；断言拼装规则本身，不经 Vue 组件。
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildShortcode, formForVariant } from "../web/src/modules/plugins/shortcode.ts";
import type { PluginMeta } from "../web/src/modules/plugins/plugin-types.ts";

const SURFACE = "shortcode 拼装";

// note 插件缩影：named 配色 + positional 标题 + content 正文（对照 catalog 的 form schema）
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

// flag 统一输出 key:true：renderer 并非所有 flag 都接受裸 token
// （folding open / image download / about back / video autoplay / ghcard show_owner / checkbox checked）
test(`${SURFACE} · block：flag 为 true 拼 key:true，非 true 不拼`, () => {
  assert.equal(
    buildShortcode(foldingPlugin, { title: "详情", open: "true", content: "正文" }),
    "{% folding 详情 open:true %}\n正文\n{% endfolding %}"
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

// mbti 插件缩影（1.38 新标签）：双 positional（type/scores）+ 双 named（url/image）
const mbtiPlugin: PluginMeta = {
  name: "mbti",
  title: "人格卡片 MBTI",
  category: "文艺装饰",
  type: "inline",
  desc: "",
  signature: "",
  params: [],
  example: "",
  form: [
    { key: "type", label: "人格类型", type: "text", role: "positional" },
    { key: "scores", label: "五维百分比", type: "text", role: "positional" },
    { key: "url", label: "档案链接", type: "text", role: "named" },
    { key: "image", label: "横幅图片", type: "text", role: "named" },
  ],
};

test(`${SURFACE} · inline：mbti 双 positional 按字段顺序、双 named 按声明顺序拼装，空 named 省略`, () => {
  assert.equal(
    buildShortcode(mbtiPlugin, { type: "INTJ-T", scores: "93-72-78-83-51", url: "https://16p.com/x", image: "" }),
    "{% mbti INTJ-T 93-72-78-83-51 url:https://16p.com/x %}"
  );
  assert.equal(
    buildShortcode(mbtiPlugin, { type: "intj", scores: "50-50-50-50-50", url: "", image: "" }),
    "{% mbti intj 50-50-50-50-50 %}"
  );
});

// ---- Spec 62 边界补充 ----

test('shortcode 拼装 · 特殊字符：值含空格/引号/花括号不被破坏', () => {
  const special = buildShortcode(notePlugin, { color: "blue", title: '带 "引号" 与 花括号 {x}', content: "正文" });
  assert.ok(special.includes('带 "引号" 与 花括号 {x}'), `值原样保留: ${special}`);
  assert.ok(special.includes("{% note"), "标签开头完整");
});

test('shortcode 拼装 · flag 仅精确 true 触发（UI checkbox 固定传 "true"）', () => {
  const on = buildShortcode(foldingPlugin, { open: "true", title: "t" });
  assert.ok(on.includes("open:true"), `true 触发 flag: ${on}`);
  const off = buildShortcode(foldingPlugin, { open: "false", title: "t" });
  assert.ok(!off.includes("open"), "false 不触发");
  const upper = buildShortcode(foldingPlugin, { open: "TRUE", title: "t" });
  assert.ok(!upper.includes("open"), "大写 TRUE 不触发（严格匹配）");
});

test('shortcode 拼装 · select 未传值则不输出参数（default 由 UI 层 PluginParamForm 填充）', () => {
  const out = buildShortcode(notePlugin, { title: "x", content: "c" });
  assert.ok(!out.includes("color:"), `未选 select 不输出空参数: ${out}`);
  // 显式传入时正常输出
  const filled = buildShortcode(notePlugin, { color: "green", title: "x", content: "c" });
  assert.ok(filled.includes("color:green"));
});

test('shortcode 拼装 · 空 values 对象不 panic，产出最少标签', () => {
  const out = buildShortcode(foldingPlugin, {});
  assert.ok(out.includes("{% folding"), `空值仍产出标签: ${out}`);
  assert.ok(!out.includes(":false") && !out.includes('=""'), "无空参数残留");
});

// ---- 全 catalog 可视化编辑补全：hybrid / 零参数 / 含空格位置参数 ----

// hybrid 缩影：navbar 单行与块级共用 active + content，只有包装方式不同
const navbarPlugin: PluginMeta = {
  name: "navbar",
  title: "文章内导航 Navbar",
  category: "布局容器",
  type: "hybrid",
  desc: "",
  signature: "",
  params: [],
  example: "",
  form: [
    { key: "active", label: "高亮项", type: "text", role: "named" },
    { key: "content", label: "导航链接", type: "textarea", role: "content" },
  ],
};

// hybrid 缩影：frame 块级（device/width/content）
const framePlugin: PluginMeta = {
  name: "frame",
  title: "设备框架 Frame",
  category: "布局容器",
  type: "hybrid",
  desc: "",
  signature: "",
  params: [],
  example: "",
  form: [
    { key: "device", label: "设备", type: "select", role: "named", options: ["iphone", "ipad", "macbook"] },
    { key: "width", label: "最大宽度", type: "text", role: "named" },
    { key: "content", label: "媒体内容", type: "textarea", role: "content" },
  ],
};

// 零参数缩影：links
const linksPlugin: PluginMeta = {
  name: "links",
  title: "友链分组 Links",
  category: "媒体播放",
  type: "inline",
  desc: "",
  signature: "",
  params: [],
  example: "{% links %}",
  form: [],
};

test(`${SURFACE} · hybrid navbar：variant 缺省/块级 → 块级包装`, () => {
  const values = { active: "首页", content: "[首页](/)\n[关于](/about)" };
  assert.equal(
    buildShortcode(navbarPlugin, values),
    "{% navbar active:首页 %}\n[首页](/)\n[关于](/about)\n{% endnavbar %}"
  );
  assert.equal(
    buildShortcode(navbarPlugin, values, "block"),
    "{% navbar active:首页 %}\n[首页](/)\n[关于](/about)\n{% endnavbar %}"
  );
});

test(`${SURFACE} · hybrid navbar：variant=inline → 单行且正文换行折叠为空格`, () => {
  assert.equal(
    buildShortcode(navbarPlugin, { active: "/", content: "[首页](/)\n[关于](/about)" }, "inline"),
    "{% navbar active:/ [首页](/) [关于](/about) %}"
  );
});

test(`${SURFACE} · hybrid frame：块级用 device/width/content`, () => {
  assert.equal(
    buildShortcode(framePlugin, { device: "iphone", width: "320px", content: "![截图](/a.jpg)" }, "block"),
    "{% frame device:iphone width:320px %}\n![截图](/a.jpg)\n{% endframe %}"
  );
});

test(`${SURFACE} · hybrid frame：单行用局部 override 的 device/img/video/focus/alt`, () => {
  const inline = formForVariant(framePlugin, "inline");
  assert.deepEqual(
    inline.form?.map((f) => f.key),
    ["device", "img", "video", "focus", "alt"]
  );
  // 原插件定义不被污染
  assert.deepEqual(
    framePlugin.form?.map((f) => f.key),
    ["device", "width", "content"]
  );
  assert.equal(
    buildShortcode(inline, { device: "iphone", img: "/a.jpg", video: "", focus: "top", alt: "手机截图" }, "inline"),
    "{% frame iphone 手机截图 img:/a.jpg focus:top %}"
  );
});

test(`${SURFACE} · formForVariant 只影响 frame 单行，其余插件原样返回`, () => {
  assert.equal(formForVariant(navbarPlugin, "inline"), navbarPlugin);
  assert.equal(formForVariant(framePlugin, "block"), framePlugin);
  assert.equal(formForVariant(framePlugin, undefined), framePlugin);
});

test(`${SURFACE} · 零参数 links 产出裸标签`, () => {
  assert.equal(buildShortcode(linksPlugin, {}), "{% links %}");
});

test(`${SURFACE} · 位置参数含空格时按字段顺序整体保留（image src/alt）`, () => {
  const imagePlugin: PluginMeta = {
    ...linksPlugin,
    name: "image",
    form: [
      { key: "src", label: "图片地址", type: "text", role: "positional" },
      { key: "alt", label: "替代文本", type: "text", role: "positional" },
      { key: "width", label: "宽度", type: "text", role: "named" },
      { key: "download", label: "下载", type: "flag", role: "flag" },
    ],
  };
  assert.equal(
    buildShortcode(imagePlugin, { src: "/a.jpg", alt: "这是一张 很长的 说明", width: "80%", download: "true" }),
    "{% image /a.jpg 这是一张 很长的 说明 width:80% download:true %}"
  );
});
