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
import { buildPreviewDoc } from "../src/modules/plugins/preview/preview-document.ts";
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

test(`${SURFACE} · light/dark 主题属性正确`, () => {
  const light = buildPreviewDoc({ theme: "light", bodyHtml: "<p>x</p>" });
  assert.ok(light.includes('data-theme="light"'), "light 文档应带 data-theme=light");
  const dark = buildPreviewDoc({ theme: "dark", bodyHtml: "<p>x</p>" });
  assert.ok(dark.includes('data-theme="dark"'), "dark 文档应带 data-theme=dark");
});

test(`${SURFACE} · 文档注入前台样式、shim 与主题背景`, () => {
  const doc = buildPreviewDoc({ theme: "dark", bodyHtml: "<p>x</p>" });
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

test(`${SURFACE} · Fixture 路由与失败降级提示正确`, () => {
  // 降级提示检测：mermaid 无 svg、mdrender 空、数据服务空 grid-box
  const doc = buildPreviewDoc({
    theme: "dark",
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
  const doc = buildPreviewDoc({ theme: "dark", bodyHtml: '<script>alert(1)</script>' });
  assert.ok(!doc.includes("</script><script>alert"), "正文 script 闭合标签必须被转义");
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
