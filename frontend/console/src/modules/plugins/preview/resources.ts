// 组件库预览资源映射（Spec 63 + Spec 64）：按插件预览能力把前台真实资源按需接入 iframe。
// 纯 TS，可脱离 Vue 单测。资源路径与 base.html / head.html 中的前台加载保持一致：
// - main.js：tabs/folding/navbar 等交互初始化（stellar.initPage 内部有缺失守卫）；
// - services/*.js：数据服务（rating/vote/friends/sites/mdrender/swiper）；
// - math-mermaid-loader.js：Mermaid/KaTeX 按需加载（本地 vendor 优先，失败降级为源码）；
// - marked：mdrender/timeline 的正文渲染依赖（本地 vendor，预览运行时不得加载 CDN）。

import type { PluginCapability, PluginPreviewInfo } from "../plugins-data.ts";

/** service 资源名 → 前台脚本路径（services 字段取值域）。 */
export const SERVICE_SCRIPTS: Record<string, string> = {
  rating: "/assets/js/services/rating.js",
  vote: "/assets/js/services/vote.js",
  friends: "/assets/js/services/friends.js",
  sites: "/assets/js/services/sites.js",
  mdrender: "/assets/js/services/mdrender.js",
  swiper: "/assets/js/services/swiper.js",
  // mermaid 特例：math-mermaid-loader.js（KaTeX/Mermaid 按需加载器）
  mermaid: "/assets/js/math-mermaid-loader.js",
};

/**
 * marked 依赖（mdrender/timeline 渲染正文用）。
 * Spec 64：固定为本地 vendor（marked@13.0.3），预览运行时不再加载 CDN。
 */
export const MARKED_CDN = "/assets/vendor/marked/marked.umd.min.js";

/** 前台交互初始化脚本（tabs/folding/navbar 等）。 */
export const MAIN_JS = "/assets/js/main.js";

/** 预览 iframe 固定注入的样式（与前台 head.html 顺序一致）。 */
export const PREVIEW_STYLES = [
  "/assets/fonts/lxgw/lxgwwenkaiscreen.css",
  "/assets/css/main.css",
  "/assets/css/tag-plugins.css",
  "/assets/css/highlight.css",
];

/** 需要 marked 依赖的 service 资源名。 */
const MARKED_DEP_SERVICES = new Set(["mdrender"]);

/**
 * 按预览能力返回需要注入 iframe 的脚本（按序：依赖在前，服务在后，main.js 最后）。
 * - static：无任何脚本（只注入 CSS + HTML）；
 * - interactive：main.js（tabs/folding/navbar 复用前台交互逻辑）；
 * - service：对应 services/*.js（marked 依赖先行）；
 * - external：math-mermaid-loader.js 或 mdrender.js + marked。
 */
export function previewScriptsFor(info: PluginPreviewInfo): string[] {
  const out: string[] = [];
  // main.js：tabs/folding/navbar 等复用前台交互逻辑（仅需要它的插件开启，swiper 自行初始化）
  if (info.mainJs) {
    out.push(MAIN_JS);
  }
  for (const name of info.services || []) {
    if (MARKED_DEP_SERVICES.has(name) && !out.includes(MARKED_CDN)) out.push(MARKED_CDN);
  }
  for (const name of info.services || []) {
    const url = SERVICE_SCRIPTS[name];
    if (url) out.push(url);
  }
  return out;
}

/** 能力标签文案（组件库界面依赖状态展示）。 */
export const CAPABILITY_LABELS: Record<PluginCapability, string> = {
  static: "静态渲染",
  interactive: "交互初始化",
  service: "数据服务 + Fixture",
  external: "外部资源按需加载",
};

/** Fixture 端点名 → 说明（界面「数据为预览 Fixture」标识用）。 */
export const FIXTURE_LABELS: Record<string, string> = {
  rating: "评分数据为预览 Fixture，不写入真实统计",
  vote: "投票数据为预览 Fixture，不写入真实统计",
  friends: "友链数据为本地 Fixture",
  sites: "站点数据为本地 Fixture",
  md: "外部 Markdown 为本地 Fixture",
  gh: "GitHub 数据为服务端确定性 Fixture",
};
