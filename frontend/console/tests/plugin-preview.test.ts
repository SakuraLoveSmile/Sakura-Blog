// 行为面：组件库标签预览（Spec 63）。
// 机械校验入口：make test → console-test → npm run test → node --test tests/*.test.ts
// Node 22.18+ 原生剥离类型运行 .ts，零新增依赖；断言纯 TS 模块（能力清单/资源映射/
// iframe 文档构建/渲染门闸），不经 Vue 组件。
import { test, mock } from "node:test";
import assert from "node:assert/strict";
import {
  plugins,
  PLUGIN_PREVIEW,
  previewInfoFor,
  type PluginPreviewInfo,
} from "../src/modules/plugins/plugins-data.ts";
import {
  SERVICE_SCRIPTS,
  MARKED_CDN,
  MAIN_JS,
  previewScriptsFor,
  FIXTURE_LABELS,
} from "../src/modules/plugins/preview/resources.ts";
import {
  buildPreviewDoc,
  MIN_PREVIEW_HEIGHT,
  MAX_PREVIEW_HEIGHT,
  DEFAULT_PREVIEW_HEIGHT,
  resolvePreviewHeight,
  clampPreviewHeight,
} from "../src/modules/plugins/preview/preview-document.ts";
import { RenderGate } from "../src/modules/plugins/preview/render-gate.ts";

const SURFACE = "插件预览";

const LEGAL_CAPABILITIES = new Set(["static", "interactive", "service", "external"]);

function pluginByName(name: string): PluginPreviewInfo {
  const p = plugins.find((x) => x.name === name);
  assert.ok(p, `plugin ${name} should exist`);
  return previewInfoFor(p!);
}

// ---------- 能力清单 ----------

test(`${SURFACE} · 每个插件都有合法预览能力描述（static/interactive/service/external）`, () => {
  assert.ok(plugins.length >= 50, `插件数应以当前清单为准（实际 ${plugins.length}），不再用旧 29 个假设`);
  for (const p of plugins) {
    const info = previewInfoFor(p);
    assert.ok(
      LEGAL_CAPABILITIES.has(info.capability),
      `${p.name} capability 非法: ${info.capability}`
    );
    for (const s of info.services || []) {
      assert.ok(s in SERVICE_SCRIPTS, `${p.name} 引用了未登记的 service: ${s}`);
    }
    if (info.fixture) {
      assert.ok(info.fixture in FIXTURE_LABELS, `${p.name} 引用了未登记的 fixture: ${info.fixture}`);
    }
    if (info.height !== undefined) {
      assert.ok(info.height >= 80 && info.height <= 900, `${p.name} 预览高度越界: ${info.height}`);
    }
  }
  // PLUGIN_PREVIEW 中登记的条目也必须合法
  for (const [name, info] of Object.entries(PLUGIN_PREVIEW)) {
    assert.ok(LEGAL_CAPABILITIES.has(info.capability), `PLUGIN_PREVIEW[${name}] capability 非法`);
  }
});

test(`${SURFACE} · 静态插件不会加载无关 service（资源清单为空）`, () => {
  for (const p of plugins) {
    const info = previewInfoFor(p);
    if (info.capability !== "static") continue;
    const scripts = previewScriptsFor(info);
    assert.deepEqual(scripts, [], `${p.name}（static）不应加载任何脚本: ${scripts.join(",")}`);
  }
});

// ---------- 资源清单 ----------

test(`${SURFACE} · tabs 资源清单 = main.js（复用前台交互逻辑）`, () => {
  assert.deepEqual(previewScriptsFor(pluginByName("tabs")), [MAIN_JS]);
});

test(`${SURFACE} · swiper 资源清单 = services/swiper.js（本地 vendor 由其自行加载）`, () => {
  assert.deepEqual(previewScriptsFor(pluginByName("swiper")), ["/assets/js/services/swiper.js"]);
});

test(`${SURFACE} · rating / vote 资源清单正确`, () => {
  assert.deepEqual(previewScriptsFor(pluginByName("rating")), ["/assets/js/services/rating.js"]);
  assert.deepEqual(previewScriptsFor(pluginByName("vote")), ["/assets/js/services/vote.js"]);
});

test(`${SURFACE} · md（external）marked 依赖先行于 mdrender`, () => {
  const scripts = previewScriptsFor(pluginByName("md"));
  assert.deepEqual(scripts, [MARKED_CDN, "/assets/js/services/mdrender.js"]);
});

test(`${SURFACE} · mermaid（external）加载 math-mermaid-loader.js`, () => {
  assert.deepEqual(previewScriptsFor(pluginByName("mermaid")), ["/assets/js/math-mermaid-loader.js"]);
});

test(`${SURFACE} · GitHub 卡片为 service 类且无客户端脚本（服务端 Fixture 直出）`, () => {
  for (const name of ["ghrepo", "ghuser", "ghcard"]) {
    const info = pluginByName(name);
    assert.equal(info.capability, "service", `${name} 应为 service 类`);
    assert.equal(info.fixture, "gh", `${name} fixture 应为 gh`);
    assert.deepEqual(previewScriptsFor(info), [], `${name} 不需要客户端脚本`);
  }
});

// ---------- iframe 文档构建 ----------

test(`${SURFACE} · 预览文档只有暗色主题（Spec 64 暗色单模式）`, () => {
  const doc = buildPreviewDoc({ bodyHtml: "<p>x</p>" });
  assert.ok(doc.includes('data-theme="dark"'), "文档必须固定 data-theme=dark");
  assert.ok(!doc.includes('data-theme="light"'), "不得出现亮色主题属性");
});

test(`${SURFACE} · 文档注入前台样式、shim 与主题背景`, () => {
  const doc = buildPreviewDoc({ bodyHtml: "<p>x</p>" });
  for (const want of [
    "/assets/css/main.css",
    "/assets/css/tag-plugins.css",
    "/assets/css/highlight.css",
    'class="md-text content"',
    "const utils = {",
  ]) {
    assert.ok(doc.includes(want), `文档缺少 ${want}`);
  }
});

test(`${SURFACE} · 预览文档无 CDN 脚本地址（Spec 64 依赖本地化）`, () => {
  const doc = buildPreviewDoc({
    bodyHtml: "<p>x</p>",
    scripts: previewScriptsFor(pluginByName("md")),
  });
  // 注入脚本（script src）与 shim deps 必须全部为本地路径
  const scriptSrcs = [...doc.matchAll(/<script src="([^"]+)"/g)].map((m) => m[1]);
  for (const src of scriptSrcs) {
    assert.ok(src.startsWith("/assets/"), `脚本必须为本地路径: ${src}`);
  }
  assert.ok(doc.includes("/assets/vendor/marked/marked.umd.min.js"), "marked 应为本地 vendor");
  assert.ok(!doc.includes("jsdelivr") && !doc.includes("gcore"), "文档不得出现 CDN 域名");
});

test(`${SURFACE} · Fixture 路由与失败降级提示正确`, () => {
  // 降级提示检测：mermaid 无 svg、mdrender 空、数据服务空 grid-box
  const doc = buildPreviewDoc({
    bodyHtml: '<div class="mermaid">graph TB</div><div class="data-service ds-mdrender" src="/api/v1/preview/md"></div>',
  });
  for (const want of [
    "Mermaid 资源加载失败",
    "外部 Markdown 加载失败",
    "本地 Fixture 不可用",
    "component-preview-resize", // 高度回传
  ]) {
    assert.ok(doc.includes(want), `文档缺少降级提示 ${want}`);
  }
  // 数据标签端点改写由后端完成；前端断言 fixture 端点名与 FIXTURE_LABELS 文案一一对应
  const fixtureNames = new Set<string>();
  for (const p of plugins) {
    const f = previewInfoFor(p).fixture;
    if (f) fixtureNames.add(f);
  }
  for (const f of fixtureNames) {
    assert.ok(f in FIXTURE_LABELS, `fixture ${f} 缺少界面说明文案`);
  }
  assert.deepEqual(
    [...fixtureNames].sort(),
    ["friends", "gh", "md", "rating", "sites", "vote"],
    "Fixture 端点清单应与后端 /api/v1/preview/* 对齐"
  );
});

test(`${SURFACE} · 正文含 </script 时被安全转义（防 iframe 文档截断）`, () => {
  const doc = buildPreviewDoc({ bodyHtml: '<script>alert(1)</script>' });
  assert.ok(!doc.includes("</script><script>alert"), "正文 script 闭合标签必须被转义");
});

test(`${SURFACE} · 高度回传以内容高度（body.scrollHeight）优先，避免视口反馈环`, () => {
  // main.css 的 html{height:100%} 使 documentElement.scrollHeight 等于 iframe 视口
  // （父级已设置的高度），逐次回传会正反馈膨胀到 900 上限；body.scrollHeight 只反映内容。
  const doc = buildPreviewDoc({ theme: "dark", bodyHtml: "<p>x</p>" });
  const start = doc.indexOf("function report");
  assert.ok(start !== -1, "应包含高度回传脚本");
  const block = doc.slice(start, start + 400);
  const bodyIdx = block.indexOf("document.body.scrollHeight");
  const elemIdx = block.indexOf("documentElement.scrollHeight");
  assert.ok(bodyIdx !== -1, "回传脚本必须使用 document.body.scrollHeight");
  assert.ok(elemIdx === -1 || bodyIdx < elemIdx, "body.scrollHeight 必须优先于 documentElement.scrollHeight");
});

// ---------- 渲染门闸（快速连续编辑防旧覆盖） ----------

test(`${SURFACE} · 快速连续编辑只执行最后一次渲染（防抖）`, () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const gate = new RenderGate(500);
    const runs: string[] = [];
    gate.schedule(async () => runs.push("a"));
    gate.schedule(async () => runs.push("b"));
    gate.schedule(async () => runs.push("c"));
    mock.timers.tick(500);
    assert.deepEqual(runs, ["c"], "连续调度只应执行最后一次");
  } finally {
    mock.timers.reset();
  }
});

test(`${SURFACE} · 旧请求结果不会覆盖新预览（请求序列号）`, async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const gate = new RenderGate(500);
    const results: string[] = [];
    // 第一次渲染立即执行但响应慢（模拟旧请求）
    const seq1 = gate.schedule(async () => {
      await new Promise((resolve) => setTimeout(resolve, 1000));
      if (gate.isCurrent(seq1)) results.push("old-result");
    }, true);
    mock.timers.tick(0); // seq1 的 run 开始，进入 1000ms 等待
    assert.equal(gate.latestSeq, seq1);
    // 用户再次编辑触发第二次渲染（新请求先返回）
    const seq2 = gate.schedule(async () => {
      results.push("new-result");
    }, true);
    mock.timers.tick(0);
    assert.equal(gate.latestSeq, seq2);
    // 旧请求此时才返回：序列号已过期，结果被丢弃
    mock.timers.tick(1000);
    assert.deepEqual(results, ["new-result"], "旧请求结果必须被序列号丢弃");
  } finally {
    mock.timers.reset();
  }
});

test(`${SURFACE} · 门闸 dispose 后不再触发待执行渲染`, () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const gate = new RenderGate(500);
    let ran = false;
    gate.schedule(async () => {
      ran = true;
    });
    gate.dispose();
    mock.timers.tick(5000);
    assert.equal(ran, false, "dispose 后待执行渲染应被取消");
  } finally {
    mock.timers.reset();
  }
});

// ---------- 预览高度（Spec 64：推荐高度 + 内容高度回传） ----------

test(`${SURFACE} · 推荐高度解析：登记高度直接采用，缺省用默认值，钳制到上下限`, () => {
  assert.equal(resolvePreviewHeight({ height: 180 }), 180);
  assert.equal(resolvePreviewHeight({ height: 420 }), 420);
  assert.equal(resolvePreviewHeight({}), DEFAULT_PREVIEW_HEIGHT);
  assert.equal(resolvePreviewHeight({ height: 20 }), MIN_PREVIEW_HEIGHT, "低于下限应钳制到下限");
  assert.equal(resolvePreviewHeight({ height: 5000 }), MAX_PREVIEW_HEIGHT, "超过上限应钳制到上限");
  // 实际清单抽查：小行内组件推荐高度远小于旧固定 420 画布
  assert.ok(resolvePreviewHeight(pluginByName("emoji")) < 200, "emoji 不应被 420px 画布包围");
  assert.ok(resolvePreviewHeight(pluginByName("chat")) >= 300, "chat 高内容应有较大推荐高度");
});

test(`${SURFACE} · 内容高度回传钳制：不低于推荐高度、不超上限`, () => {
  const min = resolvePreviewHeight({ height: 180 });
  assert.equal(clampPreviewHeight(100, min), min, "回传低于推荐高度时保持推荐高度");
  assert.equal(clampPreviewHeight(400, min), 424, "回传高于推荐高度时按内容 +24px");
  assert.equal(clampPreviewHeight(5000, min), MAX_PREVIEW_HEIGHT, "超上限钳制到 900");
});

// ---------- 默认示例资源本地化（Spec 64：不依赖公网） ----------

test(`${SURFACE} · 所有默认示例的资源地址均为本地（无 picsum/w3schools/GitHub 头像/emoji CDN）`, () => {
  const forbidden = [
    "picsum.photos",
    "w3schools.com",
    "avatars.githubusercontent.com",
    "raw.githubusercontent.com",
    "cdn.jsdelivr.net",
  ];
  for (const p of plugins) {
    const texts = [p.example, ...(p.form || []).map((f) => f.default || "")];
    for (const t of texts) {
      for (const host of forbidden) {
        assert.ok(!t.includes(host), `${p.name} 默认示例仍含公网资源 ${host}: ${t.slice(0, 60)}`);
      }
    }
  }
});
