// 行为面：组件库标签预览（Spec 63）。
// 机械校验入口：make test → console-test → pnpm run test → node --test ../tests/*.test.ts
// Node 22.18+ 原生剥离类型运行 .ts，零新增依赖；断言纯 TS 模块（能力清单/资源映射/
// iframe 文档构建/渲染门闸），不经 Vue 组件。
import { test, mock } from "node:test";
import assert from "node:assert/strict";
import { previewInfoFor, type PluginPreviewInfo } from "../web/src/modules/plugins/plugin-types.ts";
import {
  SERVICE_SCRIPTS,
  MARKED_CDN,
  MAIN_JS,
  previewScriptsFor,
  FIXTURE_LABELS,
} from "../web/src/modules/plugins/preview/resources.ts";
import {
  buildPreviewDoc,
  injectPreviewStorageFallback,
  injectPreviewSurface,
  injectPreviewWriteGuard,
  preparePreviewDocument,
  withPreviewContentWidth,
  previewFailureMessage,
  previewInvalidNoticeVisible,
  previewSurfaceColor,
  previewSurfaceFallback,
  previewWriteAllowed,
  PREVIEW_WRITE_PATHS,
  MIN_PREVIEW_HEIGHT,
  MAX_PREVIEW_HEIGHT,
  DEFAULT_PREVIEW_HEIGHT,
  resolvePreviewHeight,
  clampPreviewHeight,
} from "../web/src/modules/plugins/preview/preview-document.ts";
import { runInNewContext } from "node:vm";
import { RenderGate } from "../web/src/modules/plugins/preview/render-gate.ts";

const SURFACE = "插件预览";

const LEGAL_CAPABILITIES = new Set(["static", "interactive", "service", "external"]);

// 运行时由 /api/v1/plugins/catalog 提供；测试保留最小 contract fixture，验证资源映射纯逻辑。
const PREVIEW_FIXTURES: Record<string, PluginPreviewInfo> = {
  folding: { capability: "interactive", mainJs: true, height: 180 },
  tabs: { capability: "interactive", mainJs: true, height: 220 },
  gallery: { capability: "interactive", height: 320 },
  folders: { capability: "interactive", mainJs: true, height: 260 },
  navbar: { capability: "interactive", mainJs: true, height: 160 },
  grid: { capability: "static", height: 240 },
  swiper: { capability: "interactive", services: ["swiper"], height: 320 },
  albums: { capability: "service", services: ["friends"], fixture: "friends", height: 300 },
  posters: { capability: "service", services: ["friends"], fixture: "friends", height: 300 },
  rating: { capability: "service", services: ["rating"], fixture: "rating", height: 180 },
  vote: { capability: "service", services: ["vote"], fixture: "vote", height: 180 },
  friends: { capability: "service", services: ["friends"], fixture: "friends", height: 320 },
  sites: { capability: "service", services: ["sites"], fixture: "sites", height: 320 },
  ghcard: { capability: "service", fixture: "gh", height: 200 },
  ghrepo: { capability: "service", fixture: "gh", height: 200 },
  ghuser: { capability: "service", fixture: "gh", height: 200 },
  chat: { capability: "static", height: 360 },
  mermaid: { capability: "external", services: ["mermaid"], height: 320 },
  md: { capability: "external", services: ["mdrender"], fixture: "md", height: 320 },
  timeline: { capability: "static", height: 260 },
  toc: { capability: "static", height: 160 },
  emoji: { capability: "static", height: 120 },
  image: { capability: "static", height: 300 },
  video: { capability: "static", height: 320 },
  audio: { capability: "static", height: 120 },
};

function pluginByName(name: string): PluginPreviewInfo {
  const preview = PREVIEW_FIXTURES[name];
  assert.ok(preview, `plugin ${name} should have preview metadata`);
  return previewInfoFor({ preview });
}

// ---------- 能力清单 ----------

test(`${SURFACE} · 每个插件都有合法预览能力描述（static/interactive/service/external）`, () => {
  assert.ok(Object.keys(PREVIEW_FIXTURES).length >= 10, "预览能力清单不应退化为空");
  for (const [name, info] of Object.entries(PREVIEW_FIXTURES)) {
    assert.ok(
      LEGAL_CAPABILITIES.has(info.capability),
      `${name} capability 非法: ${info.capability}`
    );
    for (const s of info.services || []) {
      assert.ok(s in SERVICE_SCRIPTS, `${name} 引用了未登记的 service: ${s}`);
    }
    if (info.fixture) {
      assert.ok(info.fixture in FIXTURE_LABELS, `${name} 引用了未登记的 fixture: ${info.fixture}`);
    }
    if (info.height !== undefined) {
      assert.ok(info.height >= 80 && info.height <= 900, `${name} 预览高度越界: ${info.height}`);
    }
  }
});

test(`${SURFACE} · 静态插件不会加载无关 service（资源清单为空）`, () => {
  for (const [name, info] of Object.entries(PREVIEW_FIXTURES)) {
    if (info.capability !== "static") continue;
    const scripts = previewScriptsFor(info);
    assert.deepEqual(scripts, [], `${name}（static）不应加载任何脚本: ${scripts.join(",")}`);
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
    "/assets/css/tagcloud.css",
    "/assets/css/highlight.css",
    'class="md-text content"',
    "const utils = {",
  ]) {
    assert.ok(doc.includes(want), `文档缺少 ${want}`);
  }
});

test(`${SURFACE} · 预览 shim 会 eager 加载图片与背景图并处理异步节点`, () => {
  const doc = buildPreviewDoc({
    bodyHtml: '<img class="lazy" data-src="/assets/img/preview/image-demo.jpg"><div class="lazy img" data-bg="/assets/img/preview/banner-bg.jpg"></div>',
  });
  for (const want of [
    "window.wrapLazyloadImages = wrapLazyloadImages",
    "data-preview-lazy-bound",
    "data-src",
    "data-bg",
    "classList.add('loaded')",
    "MutationObserver",
  ]) {
    assert.ok(doc.includes(want), `预览 shim 缺少 ${want}`);
  }
});

// 站点卡片（sites）等卡片的样式全部挂在 .lazy-box > img 上（main.css），
// 前台由 wrapLazyloadImages 补这层包裹；预览 shim 必须同语义，否则预览与前台渲染不一致。
test(`${SURFACE} · wrapLazyloadImages 会为裸 img 补 .lazy-box 包裹`, () => {
  const doc = buildPreviewDoc({ bodyHtml: "" });
  for (const want of [
    "function wrapBareImages(container)",
    "wrapper.className = 'lazy-box'",
    "next.setAttribute('data-src', src)",
    "next.classList.add('lazy')",
    "icon.className = 'lazy-icon'",
    "img.replaceWith(wrapper)",
    // 幂等：已是 .lazy 的不再重复包裹
    "if (img.classList.contains('lazy')) return",
    // 包裹后仍要把 data-src 解析回 src
    "wrapBareImages(container);\n    eagerLoad(container);",
  ]) {
    assert.ok(doc.includes(want), `预览 shim 缺少包裹逻辑：${want}`);
  }
  // 整文档不做自动包裹，避免改写与卡片无关的正文图片结构
  assert.ok(doc.includes("eagerLoad(document);"), "预览 shim 应保留初始 eagerLoad(document)");
  assert.ok(!/wrapBareImages\(document\)/.test(doc), "预览 shim 不应对整文档自动包裹");
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
  for (const name of Object.keys(PREVIEW_FIXTURES)) {
    const f = PREVIEW_FIXTURES[name].fixture;
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

test(`${SURFACE} · 服务端整页预览文档注入存储回退且幂等`, () => {
  const serverDoc = '<!doctype html><html><head><meta charset="utf-8"><script src="/assets/js/main.js"></script></head><body></body></html>';
  const injected = injectPreviewStorageFallback(serverDoc);
  assert.ok(injected.includes('data-preview-storage="1"'), "应在 head 注入存储回退");
  assert.ok(injected.indexOf('data-preview-storage="1"') > injected.indexOf('<meta charset="utf-8">'), "不得把 charset 推出前 1024 字节");
  assert.ok(injected.indexOf('data-preview-storage="1"') < injected.indexOf("/assets/js/main.js"), "必须早于运行时脚本");
  assert.equal(injectPreviewStorageFallback(injected), injected, "重复注入应保持幂等");
  // 原生 buildPreviewDoc 文档同样带存储回退（opaque origin 下 rating/vote 不抛 SecurityError）
  assert.ok(buildPreviewDoc({ bodyHtml: "" }).includes('data-preview-storage="1"'));
});

test(`${SURFACE} · 组件 iframe 运行时包含侧栏交互脚本`, () => {
  const doc = buildPreviewDoc({
    bodyHtml: '<widget class="widget-wrapper tagcloud"></widget>',
    scripts: ["/assets/js/main.js", "/assets/js/tagcloud.js"],
  });
  assert.ok(doc.includes('<script src="/assets/js/main.js" defer></script>'));
  assert.ok(doc.includes('<script src="/assets/js/tagcloud.js" defer></script>'));
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

test(`${SURFACE} · 清空和卸载立即使已发出请求失效`, async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    for (const action of ["invalidate", "dispose"] as const) {
      const gate = new RenderGate(500);
      let resolve!: () => void;
      let result = "";
      const seq = gate.schedule(async () => {
        await new Promise<void>(done => { resolve = done; });
        if (gate.isCurrent(seq)) result = "过期预览";
      }, true);
      mock.timers.tick(0);
      gate[action]();
      resolve();
      await Promise.resolve();
      assert.equal(result, "", `${action} 后迟到响应必须丢弃`);
      assert.equal(gate.isCurrent(seq), false);
    }
  } finally { mock.timers.reset(); }
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

// ---------- 预览表面：不透明背景与主题回退（V1） ----------

test(`${SURFACE} · 预览表面注入不透明背景、跟随文档主题且幂等`, () => {
  const serverDoc =
    '<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/assets/css/main.css"></head><body><article class="md-text content"><p>x</p></article></body></html>';
  const prepared = preparePreviewDocument(serverDoc, null);
  assert.ok(prepared.includes('data-preview-surface="1"'), "必须注入表面样式");
  assert.ok(prepared.includes('data-preview-surface-script="1"'), "必须注入运行时表面解析");
  assert.ok(prepared.indexOf('data-preview-surface="1"') < prepared.indexOf("</head>"), "样式应晚于站点样式表");
  assert.ok(prepared.indexOf('data-preview-surface-script="1"') < prepared.indexOf("</body>"), "脚本应在正文之后");
  assert.ok(prepared.includes("--background") && prepared.includes("--bg"), "优先读取文档主题变量");
  assert.ok(prepared.includes("prefers-color-scheme: dark"), "无 data-theme 时跟随系统偏好");
  assert.ok(prepared.includes('html[data-theme="dark"]'), "深色文档使用深底");
  assert.ok(prepared.includes("#1c1c1e") && prepared.includes("#ffffff"), "提供中性明暗回退色");
  assert.equal(injectPreviewSurface(prepared), prepared, "重复注入必须幂等");
  assert.equal((prepared.match(/data-preview-surface="1"/g) || []).length, 1);
});

test(`${SURFACE} · 嵌入正文不增加表面，写守卫和 profile 存储规则仍有效`, () => {
  const serverDoc = '<!doctype html><html><head><meta charset="utf-8"><style>html,body{background:transparent}</style></head><body></body></html>';
  for (const profile of [null, "component-library"]) {
    const embedded = preparePreviewDocument(serverDoc, profile, { surface: "embedded" });
    assert.ok(!embedded.includes('data-preview-surface="1"'));
    assert.ok(!embedded.includes('data-preview-surface-script="1"'));
    assert.ok(embedded.includes("html,body{background:transparent}"));
    assert.ok(embedded.includes('data-preview-write-guard="1"'));
    assert.equal(embedded.includes('data-preview-storage="1"'), profile === "component-library");
    assert.equal(preparePreviewDocument(embedded, profile, { surface: "embedded" }), embedded);
  }
});

test(`${SURFACE} · 正文宽度只约束内容，更新幂等且拒绝非法尺寸`, () => {
  const serverDoc = '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><style>@media(max-width:767px){.component{display:block}}</style></head><body><main data-stellar-preview-content><article class="md-text content"><div class="component">x</div></article></main><script>window.example = 1;</script></body></html>';
  const prepared = preparePreviewDocument(serverDoc, null, { surface: "embedded" });
  for (const invalid of [0, -1, NaN, Infinity, -Infinity, "600; color:red" as any]) {
    assert.equal(withPreviewContentWidth(prepared, invalid), prepared, `非法宽度 ${invalid} 不改文档`);
  }
  const narrow = withPreviewContentWidth(prepared, 615.6);
  assert.equal(withPreviewContentWidth(narrow, 615.6), narrow);
  const wide = withPreviewContentWidth(narrow, 860);
  assert.equal((wide.match(/data-preview-content-width="1"/g) || []).length, 1);
  assert.ok(wide.includes("width:860px"));
  assert.ok(!wide.includes("width:616px"));
  const injected = wide.match(/<style data-preview-content-width="1">([\s\S]*?)<\/style>/)?.[1] ?? "";
  assert.ok(injected.startsWith("[data-stellar-preview-content],[data-stellar-preview-content] article{"));
  assert.ok(!/\b(html|body|iframe)\b|@media/.test(injected), "正文宽度不得改媒体查询视口");
  assert.equal(wide.replace(/<style data-preview-content-width="1">[\s\S]*?<\/style>/, ""), prepared, "组件样式、脚本、写守卫及视口元数据必须保持原样");
  assert.ok(withPreviewContentWidth(prepared, Number.MAX_VALUE).includes("width:16384px"));
  assert.ok(withPreviewContentWidth(prepared, 0.1).includes("width:1px"));
});

function surfaceScriptOf(document: string): string {
  const match = document.match(/<script data-preview-surface-script="1">([\s\S]*?)<\/script>/);
  assert.ok(match, "缺少预览表面脚本");
  return match[1];
}

function runSurfaceScript(options: { theme?: string | null; prefersDark?: boolean; background?: string }) {
  const styleSink = () => {
    const style: Record<string, string> & { setProperty?: (name: string, value: string) => void } = {};
    style.setProperty = (name: string, value: string) => {
      style[name] = value;
    };
    return style;
  };
  const probe = { style: {} as Record<string, string>, remove() {} };
  const html = {
    style: styleSink(),
    getAttribute: (name: string) => (name === "data-theme" ? options.theme ?? null : null),
    appendChild() {},
  };
  const body = { style: styleSink(), appendChild() {} };
  const background = options.background ?? "";
  const sandbox: any = {
    document: {
      documentElement: html,
      body,
      readyState: "complete",
      createElement: () => probe,
      addEventListener() {},
    },
    window: {
      getComputedStyle(el: any) {
        if (el === probe) return { color: probe.style.color };
        return {
          getPropertyValue: (name: string) => (name === "--background" ? background : ""),
          backgroundColor: "rgba(0, 0, 0, 0)",
        };
      },
      addEventListener() {},
      matchMedia: () => ({ matches: Boolean(options.prefersDark), addEventListener() {} }),
      MutationObserver: class {
        observe() {}
      },
    },
    setTimeout() {},
    setInterval() {},
    clearInterval() {},
    console,
  };
  runInNewContext(surfaceScriptOf(
    preparePreviewDocument('<!doctype html><html><head><meta charset="utf-8"></head><body></body></html>', null),
  ), sandbox);
  return { html: html.style["background-color"], body: body.style["background-color"] };
}

test(`${SURFACE} · 表面回退按主题/系统偏好，透明变量合成到不透明底色`, () => {
  assert.equal(previewSurfaceFallback("dark", false), "#1c1c1e");
  assert.equal(previewSurfaceFallback("light", true), "#ffffff");
  assert.equal(previewSurfaceFallback(undefined, true), "#1c1c1e");
  assert.equal(previewSurfaceFallback("auto", false), "#ffffff");
  assert.equal(previewSurfaceColor([], "#ffffff"), "rgb(255, 255, 255)");
  assert.equal(previewSurfaceColor(["rgba(0, 0, 0, 0)"], "#ffffff"), "rgb(255, 255, 255)", "透明变量必须回退");
  assert.equal(previewSurfaceColor(["rgba(0, 0, 0, 0.5)"], "#ffffff"), "rgb(128, 128, 128)", "半透明色需合成");
  assert.equal(previewSurfaceColor(["rgb(18, 18, 20)"], "#ffffff"), "rgb(18, 18, 20)", "不透明主题色直接复用");
  assert.equal(previewSurfaceColor(["rgba(0, 0, 0, 0)", "rgb(24, 24, 24)"], "#ffffff"), "rgb(24, 24, 24)");

  // 实际执行注入脚本：主题变量、透明回退、data-theme 与 prefers-color-scheme 都要落到 html/body。
  const themed = runSurfaceScript({ theme: null, prefersDark: false, background: "rgb(250, 250, 250)" });
  assert.equal(themed.html, "rgb(250, 250, 250)");
  assert.equal(themed.body, "rgb(250, 250, 250)");
  const transparentDark = runSurfaceScript({ theme: "dark", prefersDark: false, background: "rgba(0, 0, 0, 0)" });
  assert.equal(transparentDark.html, "rgb(28, 28, 30)");
  const systemDark = runSurfaceScript({ theme: null, prefersDark: true, background: "" });
  assert.equal(systemDark.html, "rgb(28, 28, 30)");
  const forcedLight = runSurfaceScript({ theme: "light", prefersDark: true, background: "" });
  assert.equal(forcedLight.html, "rgb(255, 255, 255)");
});

// ---------- 写入守卫 / 存储回退 profile 收窄 ----------

function guardScriptOf(document: string): string {
  const match = document.match(/<script data-preview-write-guard="1">([\s\S]*?)<\/script>/);
  assert.ok(match, "缺少写入守卫脚本");
  return match[1];
}

test(`${SURFACE} · 写入守卫 URL 判定：GET 放行、精确 Fixture 写放行、真实写/绝对 URL 拒绝`, () => {
  assert.equal(previewWriteAllowed("GET", "/api/vote/info", "about:srcdoc"), true);
  assert.equal(previewWriteAllowed("HEAD", "/api/vote/update", "about:srcdoc"), true);
  assert.equal(previewWriteAllowed("OPTIONS", "/api/vote/update", "about:srcdoc"), true);
  assert.equal(previewWriteAllowed("GET", "https://evil.example/api/vote/info", "about:srcdoc"), true, "GET 放行");
  for (const path of PREVIEW_WRITE_PATHS) {
    assert.equal(previewWriteAllowed("POST", path, "about:srcdoc"), true, `${path} 应放行`);
    assert.equal(previewWriteAllowed("POST", `${path}?id=demo`, "about:srcdoc"), true, `${path} 允许 query`);
  }
  assert.equal(previewWriteAllowed("post", "/api/v1/preview/vote/update", "about:srcdoc"), true);
  assert.equal(previewWriteAllowed("POST", "/api/vote/update", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "/api/rating/update", "about:srcdoc"), false);
  // 只允许精确路径：前缀相近、尾斜杠、额外段都不放行
  assert.equal(previewWriteAllowed("POST", "/api/v1/preview/rating-extra", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "/api/v1/preview/rating/info/extra", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "/api/v1/preview/friends/", "about:srcdoc"), false);
  // 协议相对 URL 视为绝对，即使是同源也不放行
  assert.equal(previewWriteAllowed("POST", "//localhost:8096/api/v1/preview/rating", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "//evil.example/api/v1/preview/friends", "about:srcdoc"), false);
  // 反斜杠在 URL 解析中等价于 `/`：`\\evil.example/...`、`/\evil.example/...` 曾会被解析成外域并放行
  assert.equal(previewWriteAllowed("POST", "\\\\evil.example/api/v1/preview/rating", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "/\\evil.example/api/v1/preview/rating", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "\\\\localhost:8096/api/v1/preview/rating", "about:srcdoc"), false);
  // GET/HEAD/OPTIONS 不受影响
  assert.equal(previewWriteAllowed("GET", "\\\\evil.example/api/v1/preview/rating", "about:srcdoc"), true);
  // 绝对外域即使路径像 Fixture 也不能放行
  assert.equal(previewWriteAllowed("POST", "https://evil.example/api/v1/preview/friends", "about:srcdoc"), false);
  // 解析结果的 origin 必须与当前 base 的 origin 一致
  assert.equal(
    previewWriteAllowed("POST", "/api/v1/preview/rating", "http://localhost:8096/console", "http://localhost:8096/console"),
    true,
  );
  // 动态 <base> 把相对 Fixture 导向外域时必须拒绝
  assert.equal(
    previewWriteAllowed("POST", "/api/v1/preview/rating", "https://evil.example/console", "http://localhost:8096/console"),
    false,
  );
  assert.equal(
    previewWriteAllowed("POST", "/api/v1/preview/rating", "http://localhost:8097/console", "http://localhost:8096/console"),
    false,
    "base 端口变化也要拒绝",
  );
  // 非法 URL 不能因解析异常而放行
  assert.equal(previewWriteAllowed("POST", "http://[bad", "about:srcdoc"), false);
  assert.equal(previewWriteAllowed("POST", "/api/v1/preview/rating", "http://[bad", "http://[bad"), false);
});

test(`${SURFACE} · 写入守卫脚本拦截真实 fetch/XHR/beacon，放行 Fixture 写与 GET`, async () => {
  const document = injectPreviewWriteGuard(
    '<!doctype html><html><head><meta charset="utf-8"><script src="/assets/js/main.js"></script></head><body></body></html>',
  );
  const calls: string[] = [];
  class FakeXHR {
    method = "";
    url = "";
    open(method: string, url: string) {
      this.method = method;
      this.url = url;
    }
    send() {
      calls.push(`xhr:${this.method}:${this.url}`);
    }
  }
  const sandbox: any = {
    window: {
      fetch(input: any, init?: any) {
        calls.push(`fetch:${init?.method ?? "GET"}:${String(input)}`);
        return Promise.resolve({ ok: true });
      },
      XMLHttpRequest: FakeXHR,
    },
    navigator: {
      sendBeacon(url: string) {
        calls.push(`beacon:${url}`);
        return true;
      },
    },
    document: { baseURI: "about:srcdoc" },
    URL,
    Promise,
    Error,
    Date,
    console,
  };
  runInNewContext(guardScriptOf(document), sandbox);

  // GET 放行
  await sandbox.window.fetch("/api/vote/info");
  // Fixture POST 放行
  await sandbox.window.fetch("/api/v1/preview/rating/update", { method: "POST" });
  // 真实 POST 拒绝：promise rejected 且原 fetch 未被调用
  await assert.rejects(sandbox.window.fetch("/api/vote/update", { method: "POST" }), /preview write blocked/);
  assert.equal(sandbox.window.__stellarPreviewWriteBlocked.method, "POST");
  assert.equal(sandbox.window.__stellarPreviewWriteBlocked.url, "/api/vote/update");
  assert.deepEqual(calls, ["fetch:GET:/api/vote/info", "fetch:POST:/api/v1/preview/rating/update"]);

  // XHR：Fixture 放行，真实写同步抛错
  const allowedXHR = new sandbox.window.XMLHttpRequest();
  allowedXHR.open("POST", "/api/v1/preview/vote/update");
  allowedXHR.send();
  const blockedXHR = new sandbox.window.XMLHttpRequest();
  blockedXHR.open("POST", "/api/vote/update");
  assert.throws(() => blockedXHR.send(), /preview write blocked/);
  assert.ok(calls.includes("xhr:POST:/api/v1/preview/vote/update"));

  // sendBeacon：真实写返回 false，Fixture 放行
  assert.equal(sandbox.navigator.sendBeacon("/api/vote/update"), false);
  assert.equal(sandbox.navigator.sendBeacon("/api/v1/preview/vote/update"), true);
  assert.ok(calls.includes("beacon:/api/v1/preview/vote/update"));

  // 协议相对 / 反斜杠变体 / 动态 <base>：写请求必须拒绝
  await assert.rejects(
    sandbox.window.fetch("//localhost:8096/api/v1/preview/rating/update", { method: "POST" }),
    /preview write blocked/,
  );
  await assert.rejects(
    sandbox.window.fetch("\\\\evil.example/api/v1/preview/rating", { method: "POST" }),
    /preview write blocked/,
    "反斜杠绝对 URL 不得放行",
  );
  await assert.rejects(
    sandbox.window.fetch("/\\evil.example/api/v1/preview/rating", { method: "POST" }),
    /preview write blocked/,
    "斜杠+反斜杠绝对 URL 不得放行",
  );
  assert.equal(sandbox.navigator.sendBeacon("\\\\evil.example/api/v1/preview/rating"), false);
  sandbox.document.baseURI = "https://evil.example/console";
  await assert.rejects(
    sandbox.window.fetch("/api/v1/preview/rating/update", { method: "POST" }),
    /preview write blocked/,
  );
  assert.equal(sandbox.navigator.sendBeacon("/api/v1/preview/vote/update"), false);
});

test(`${SURFACE} · 写入守卫幂等、位于 charset 之后且早于站点脚本`, () => {
  const serverDoc = '<!doctype html><html><head><meta charset="utf-8"><script src="/assets/js/main.js"></script></head><body></body></html>';
  const guarded = injectPreviewWriteGuard(serverDoc);
  assert.equal(injectPreviewWriteGuard(guarded), guarded, "重复注入必须幂等");
  assert.equal((guarded.match(/data-preview-write-guard="1"/g) || []).length, 1);
  assert.ok(guarded.indexOf('data-preview-write-guard="1"') > guarded.indexOf('<meta charset="utf-8">'));
  assert.ok(guarded.indexOf('data-preview-write-guard="1"') < guarded.indexOf("/assets/js/main.js"));
});

test(`${SURFACE} · 存储回退只在 component-library 预览注入，真实上下文保持旧行为`, () => {
  const serverDoc = '<!doctype html><html><head><meta charset="utf-8"></head><body></body></html>';
  const library = preparePreviewDocument(serverDoc, "component-library");
  assert.ok(library.includes('data-preview-write-guard="1"'), "component-library 必须有写守卫");
  assert.ok(library.includes('data-preview-storage="1"'), "component-library 应注入内存 Storage");

  for (const profile of [null, undefined, "", "real"]) {
    const real = preparePreviewDocument(serverDoc, profile as any);
    assert.ok(real.includes('data-preview-write-guard="1"'), `${String(profile)} 仍必须有写守卫`);
    assert.ok(!real.includes('data-preview-storage="1"'), `${String(profile)} 不应注入内存 Storage`);
  }
  // 旧 API 仍可单独使用
  assert.ok(injectPreviewStorageFallback(serverDoc).includes('data-preview-storage="1"'));
});

test(`${SURFACE} · 参数无效提示与 Markdown 失败文案`, () => {
  assert.equal(previewInvalidNoticeVisible(false, false, true), true);
  assert.equal(previewInvalidNoticeVisible(false, true, true), false, "校验中不提示参数无效");
  assert.equal(previewInvalidNoticeVisible(true, false, true), false);
  assert.equal(previewInvalidNoticeVisible(false, false, false), false, "无旧文档时走源码错误态");
  assert.equal(previewFailureMessage(true), "预览渲染失败，仍显示上一次结果");
  assert.equal(previewFailureMessage(false), "预览渲染失败");
});
