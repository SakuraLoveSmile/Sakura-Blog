// 组件库预览 iframe 文档构建（Spec 63）：标签插件库与侧边栏组件库共用同一套
// 「CSS 注入 + 前台运行时 shim + 按能力脚本注入 + 降级检测 + 高度回传」逻辑，
// 避免两套预览行为继续分叉。纯 TS，可脱离 Vue 单测。
//
// iframe 内注入的 ctx/def/deps/utils 为 base.html 前台运行时的等价最小版：
// - ctx：main.js / services 依赖的全局对象（data_cache 关闭，保证预览每次拿最新 Fixture）；
// - def：friends.js/sites.js 读取的占位素材；
// - deps：marked 等懒加载依赖的 CDN 地址；
// - utils：css/js/dom/request 等加载辅助（对齐 base.html utils 的语义）。

import { PREVIEW_STYLES } from "./resources.ts";

export type PreviewTheme = "light" | "dark";

export interface PreviewDocOptions {
  theme: PreviewTheme;
  /** 渲染后的正文/片段 HTML（标签插件为 goldmark 产物；侧栏组件为提取的 aside 片段） */
  bodyHtml: string;
  /** 需要加载的前台脚本（完整 URL 或 /assets 相对路径；依赖须按序排列） */
  scripts?: string[];
  /** 注入 head 的额外内容（如侧栏组件预览的布局修正样式） */
  extraHead?: string;
  /** body 额外 class（侧栏组件预览保留页面 body class 用） */
  bodyClass?: string;
  /** 是否包裹 <article class="md-text content">（标签插件预览）；侧栏组件预览传 false */
  wrapContent?: boolean;
  /** 是否注入降级提示检测脚本（mermaid/md/数据服务空态），默认 true */
  degradeHints?: boolean;
  /** 是否注入 iframe 高度回传脚本（postMessage → 父窗口），默认 true */
  autoHeight?: boolean;
  /** 额外内联脚本（原样注入，位于内置 shim 之后；侧栏组件预览复用页面 services scan 用） */
  inlineScripts?: string[];
}

/** 前台运行时等价最小 shim（对齐 base.html 的 ctx/def/deps/utils，data_cache 关闭）。 */
const PREVIEW_SHIM = `<script>
const ctx = { date_suffix: { just: '刚刚', min: '分钟前', hour: '小时前', day: '天前' }, root: '/', tag_plugins: { chat: {} }, search: { service: 'local_search', path: '/search.json' }, data_cache: { enable: false, default_ttl: 0 }, copycode: {}, fancybox: {} };
const def = { avatar: '', cover: '', loading: '' };
const deps = { marked: 'https://gcore.jsdelivr.net/npm/marked@13.0/lib/marked.umd.min.js', lazyload: '/assets/js/vendor/lazyload.min.js' };
const utils = {
  _loadedStyles: new Set(), _loadedScripts: new Set(), _loadedElements: new WeakSet(), _inflight: new Map(),
  css: function (href) { if (utils._loadedStyles.has(href)) return null; var ss = document.createElement('link'); ss.rel = 'stylesheet'; ss.href = href; utils._loadedStyles.add(href); document.head.appendChild(ss); return ss; },
  js: function (src, opt) { return new Promise(function (resolve, reject) { if (src.charAt(0) === '/') src = ctx.root + src.substring(1); if (utils._loadedScripts.has(src)) { resolve(); return; } var s = document.createElement('script'); s.src = src; if (opt) { for (var k in opt) { s[k] = opt[k]; } } else { s.async = true; } s.onerror = reject; s.onload = function () { utils._loadedScripts.add(src); resolve(); }; document.head.appendChild(s); }); },
  jq: function (fn) { fn(); },
  qs: function (sel, c) { return (c || document).querySelector(sel); },
  qsa: function (sel, c) { return Array.prototype.slice.call((c || document).querySelectorAll(sel)); },
  dom: function (selector, c) {
    var els = [];
    if (typeof selector === 'string') els = utils.qsa(selector, c);
    else if (selector && selector.nodeType === 1) els = [selector];
    else if (selector && typeof selector.length === 'number') els = Array.prototype.slice.call(selector);
    var api = {
      length: els.length,
      each: function (fn) { els.forEach(function (el, i) { fn.call(el, i, el); }); return api; },
      find: function (sel) { var r = []; els.forEach(function (el) { Array.prototype.push.apply(r, el.querySelectorAll(sel)); }); return utils.dom(r); },
      append: function (ch) { els.forEach(function (el) { if (typeof ch === 'string') el.insertAdjacentHTML('beforeend', ch); else if (ch && ch.nodeType === 1) el.appendChild(ch); }); return api; },
      remove: function () { els.forEach(function (el) { el.remove(); }); return api; },
      addClass: function (cname) { els.forEach(function (el) { el.classList.add(cname); }); return api; },
      removeClass: function (cname) { els.forEach(function (el) { el.classList.remove(cname); }); return api; },
      attr: function (n, v) { if (v === undefined) return els[0] ? els[0].getAttribute(n) : undefined; els.forEach(function (el) { el.setAttribute(n, v); }); return api; },
      text: function (v) { if (v === undefined) return els[0] ? els[0].textContent : undefined; els.forEach(function (el) { el.textContent = v; }); return api; },
      html: function (v) { if (v === undefined) return els[0] ? els[0].innerHTML : undefined; els.forEach(function (el) { el.innerHTML = v; }); return api; },
      val: function (v) { if (v === undefined) return els[0] ? els[0].value : undefined; els.forEach(function (el) { el.value = v; }); return api; },
      on: function (e, cb) { els.forEach(function (el) { el.addEventListener(e, cb); }); return api; },
      empty: function () { els.forEach(function (el) { el.replaceChildren(); }); return api; }
    };
    els.forEach(function (el, i) { api[i] = el; });
    return api;
  },
  onLoading: function (el) { if (el && el.querySelector('.loading-wrap') === null) { el.insertAdjacentHTML('beforeend', '<div class="loading-wrap"><svg xmlns="http://www.w3.org/2000/svg" width="2em" height="2em" viewBox="0 0 24 24"><g fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"><path stroke-dasharray="60" stroke-dashoffset="60" stroke-opacity=".3" d="M12 3C16.9706 3 21 7.02944 21 12C21 16.9706 16.9706 21 12 21C7.02944 21 3 16.9706 3 12C3 7.02944 7.02944 3 12 3Z"><animate fill="freeze" attributeName="stroke-dashoffset" dur="1.3s" values="60;0"/></path><path stroke-dasharray="15" stroke-dashoffset="15" d="M12 3C16.9706 3 21 7.02944 21 12"><animate fill="freeze" attributeName="stroke-dashoffset" dur="0.3s" values="15;0"/><animateTransform attributeName="transform" dur="1.5s" repeatCount="indefinite" type="rotate" values="0 12 12;360 12 12"/></path></g></svg></div>'); } },
  onLoadSuccess: function (el) { if (el) { var w = el.querySelector('.loading-wrap'); if (w) w.remove(); } },
  onLoadFailure: function (el) { if (el) { var w = el.querySelector('.loading-wrap'); if (w) w.classList.add('error'); } },
  request: function (el, url, callback, onFailure) {
    if (el && utils._loadedElements.has(el)) return;
    if (utils._inflight.has(url)) return utils._inflight.get(url);
    var p = new Promise(function (resolve, reject) {
      utils.onLoading(el);
      fetch(url).then(function (resp) { if (!resp.ok) throw new Error('bad response'); return resp; })
        .then(function (data) { if (el) utils._loadedElements.add(el); utils.onLoadSuccess(el); callback(data); resolve(data); })
        .catch(function (err) { utils.onLoadFailure(el); if (onFailure) onFailure(err); reject(err); });
    });
    utils._inflight.set(url, p);
    p.finally(function () { utils._inflight.delete(url); });
    return p;
  },
  requestWithoutLoading: function (url, options) { return fetch(url, options); },
  requestAnimationFrame: function (fn) { window.requestAnimationFrame(fn); },
  dark: {}
};
window.wrapLazyloadImages = function () {};
</script>`;

/** 降级提示检测：异步内容（Mermaid/外部 Markdown/数据服务）加载失败时给出可读提示，不破坏布局。 */
const DEGRADE_SCRIPT = `<script>
(function () {
  var HINTS = {
    mermaid: 'Mermaid 资源加载失败，已保留图表源码。',
    mdrender: '外部 Markdown 加载失败（本地 Fixture 不可用）。',
    friends: '友链数据加载失败（本地 Fixture 不可用）。',
    sites: '站点数据加载失败（本地 Fixture 不可用）。',
    timeline: '时间轴数据加载失败（本地 Fixture 不可用）。'
  };
  function hint(text) {
    var el = document.createElement('div');
    el.className = 'preview-hint';
    el.textContent = text;
    return el;
  }
  function check() {
    document.querySelectorAll('.mermaid').forEach(function (el) {
      if (!el.querySelector('svg') && !el.querySelector('.preview-hint')) el.appendChild(hint(HINTS.mermaid));
    });
    document.querySelectorAll('.ds-mdrender').forEach(function (el) {
      if (!el.textContent.trim() && !el.querySelector('.preview-hint')) el.appendChild(hint(HINTS.mdrender));
    });
    document.querySelectorAll('.ds-friends, .ds-friends_and_posts, .ds-sites, .ds-timeline').forEach(function (el) {
      var box = el.querySelector('.grid-box');
      if (box && !box.children.length && !el.querySelector('.preview-hint')) {
        var cls = el.classList;
        var key = cls.contains('ds-friends') ? 'friends' : (cls.contains('ds-sites') ? 'sites' : 'timeline');
        el.appendChild(hint(HINTS[key] || '数据加载失败（本地 Fixture 不可用）。'));
      }
    });
  }
  function start() { setTimeout(check, 5000); }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start); else start();
})();
</script>`;

/** iframe 高度回传：ResizeObserver + load 兜底，异步内容加载完成后不被裁剪。 */
const RESIZE_SCRIPT = `<script>
(function () {
  function report() {
    var h = document.documentElement.scrollHeight || document.body.scrollHeight || 0;
    window.parent.postMessage({ type: 'component-preview-resize', height: h }, '*');
  }
  window.addEventListener('load', report);
  if (window.ResizeObserver) {
    try { new ResizeObserver(report).observe(document.body); } catch (e) {}
  }
  setTimeout(report, 100);
  setTimeout(report, 800);
  setTimeout(report, 3000);
})();
</script>`;

function safeHtml(html: string): string {
  return html.replace(/<\/script/gi, "<\\/script");
}

/**
 * 构建预览 iframe 文档（srcdoc）。标签插件库与侧边栏组件库共用：
 * 样式注入（main.css/tag-plugins.css/highlight.css + 字体）、主题属性、
 * 前台运行时 shim、按能力脚本、降级提示、高度回传。
 */
export function buildPreviewDoc(opts: PreviewDocOptions): string {
  const {
    theme,
    bodyHtml,
    scripts = [],
    extraHead = "",
    bodyClass = "",
    wrapContent = true,
    degradeHints = true,
    autoHeight = true,
    inlineScripts = [],
  } = opts;

  const styleLinks = PREVIEW_STYLES.map(
    (href) => `<link rel="stylesheet" href="${href}">`
  ).join("\n");

  const scriptTags = scripts.map((src) => `<script src="${src}" defer></script>`).join("\n");
  const inlineTags = inlineScripts.join("\n");
  const body = wrapContent
    ? `<article class="md-text content">${safeHtml(bodyHtml)}</article>`
    : safeHtml(bodyHtml);

  return `<!doctype html>
<html lang="zh-CN" data-theme="${theme}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
${styleLinks}
<style>
html,body{margin:0;padding:0}
body{padding:24px;background:var(--background,#1c1c1e);font-family:"LXGW WenKai Screen",system-ui,"Microsoft Yahei","Helvetica Neue",Helvetica,Arial,sans-serif}
.md-text{max-width:760px;margin:0 auto}
.preview-hint{margin:10px 0;padding:10px 12px;border:1px dashed var(--border,#666);border-radius:8px;color:var(--text-p3,#999);font-size:13px}
</style>
${extraHead}
</head>
<body${bodyClass ? ` class="${bodyClass}"` : ""}>
${body}
${PREVIEW_SHIM}
${scriptTags}
${inlineTags}
${degradeHints ? DEGRADE_SCRIPT : ""}
${autoHeight ? RESIZE_SCRIPT : ""}
</body>
</html>`;
}
