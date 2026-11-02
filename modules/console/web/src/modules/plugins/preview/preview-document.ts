// 组件库预览 iframe 文档构建（Spec 63 + Spec 64）：标签插件库与侧边栏组件库共用
// 同一套「CSS 注入 + 前台运行时 shim + 按能力脚本注入 + 降级检测 + 高度回传」逻辑，
// 避免两套预览行为继续分叉。纯 TS，可脱离 Vue 单测。
//
// buildPreviewDoc 仍是组件库的暗色单模式文档；服务端整页预览（preparePreviewDocument）
// 跟随文档自身主题，只补不透明表面，不强行改成 Console 配色。
// iframe 内注入的 ctx/def/deps/utils 为 base.html 前台运行时的等价最小版：
// - ctx：main.js / services 依赖的全局对象（data_cache 关闭，保证预览每次拿最新 Fixture）；
// - def：friends.js/sites.js 读取的占位素材；
// - deps：marked 等懒加载依赖的本地 vendor 地址（预览运行时不得加载 CDN）；
// - utils：css/js/dom/request 等加载辅助（对齐 base.html utils 的语义）。

import { PREVIEW_STYLES } from "./resources.ts";

export interface MockFixtureConfig {
  endpoint: string;
  format: "json" | "markdown";
  state: "normal" | "empty" | "error";
  customData?: string;
  emptyData?: string;
}

export interface PreviewDocOptions {
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
  /** 自定义 Mock Fixtures 数据与状态覆盖表 */
  mockFixtures?: Record<string, MockFixtureConfig>;
}

/**
 * preview 专属存储回退：sandbox="allow-scripts" 的 srcdoc iframe 是 opaque origin，
 * 访问 localStorage/sessionStorage 会直接抛 SecurityError（rating/vote 的 hasRated 在初始化时同步读取）。
 * 这里只在预览文档内把不可用的 Storage 换成内存实现，公开站脚本与行为保持不变。
 */
const PREVIEW_STORAGE_SHIM = `<script data-preview-storage="1">
(function () {
  function memoryStorage() {
    var data = {};
    return {
      get length() { return Object.keys(data).length; },
      key: function (index) { var keys = Object.keys(data); return index >= 0 && index < keys.length ? keys[index] : null; },
      getItem: function (key) { key = String(key); return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : null; },
      setItem: function (key, value) { data[String(key)] = String(value); },
      removeItem: function (key) { delete data[String(key)]; },
      clear: function () { data = {}; }
    };
  }
  function install(name) {
    try {
      var existing = window[name];
      existing.setItem('__preview_probe__', '1');
      existing.removeItem('__preview_probe__');
      return;
    } catch (error) {
      // opaque origin：继续安装内存回退
    }
    var fallback = memoryStorage();
    try {
      Object.defineProperty(window, name, { configurable: true, get: function () { return fallback; } });
    } catch (error) {
      // 极老浏览器不允许重定义：保持原行为并让脚本自行降级
    }
  }
  install('localStorage');
  install('sessionStorage');
})();
</script>`;

/**
 * 把内联脚本放在 charset 之后、其余脚本之前：
 * - 不把 <meta charset> 推出前 1024 字节；
 * - 保证守卫/存储回退早于站点脚本执行。
 */
function insertAfterCharset(document: string, fragment: string): string {
  const charset = document.match(/<meta[^>]+charset=[^>]*>/i);
  if (charset && charset.index !== undefined) {
    const at = charset.index + charset[0].length;
    return document.slice(0, at) + fragment + document.slice(at);
  }
  const head = document.match(/<head[^>]*>/i);
  if (head && head.index !== undefined) {
    const at = head.index + head[0].length;
    return document.slice(0, at) + fragment + document.slice(at);
  }
  const body = document.match(/<body[^>]*>/i);
  if (body && body.index !== undefined) {
    const at = body.index + body[0].length;
    return document.slice(0, at) + fragment + document.slice(at);
  }
  return fragment + document;
}

// ---------- 预览表面：深色 Console 中的不透明可读背景（BLG-20261031-03 V1） ----------

/** 无主题变量或变量透明时的表面回退色；使用中性色，不复用 Console 的深色面板色。 */
export const PREVIEW_SURFACE_LIGHT = "#ffffff";
export const PREVIEW_SURFACE_DARK = "#1c1c1e";

/**
 * 按文档主题（data-theme）或系统偏好选择回退表面；auto/缺失时跟随 prefers-color-scheme。
 * 返回值直接写死为字面量，保证 .toString() 注入 iframe 后不依赖模块常量。
 */
export function previewSurfaceFallback(theme: string | null | undefined, prefersDark: boolean): string {
  const normalized = String(theme || "").trim().toLowerCase();
  const dark = normalized === "dark" || (normalized !== "light" && prefersDark);
  return dark ? "#1c1c1e" : "#ffffff";
}

/**
 * 从已解析的 CSS 颜色候选中取第一个可见表面色；半透明色与不透明回退底色合成，
 * 保证最终背景真正不透明。候选为空/全透明时返回回退色。
 * 解析逻辑内嵌在函数内（函数声明，不使用外层变量），.toString() 注入 iframe 后可独立运行。
 */
export function previewSurfaceColor(
  candidates: readonly (string | null | undefined)[],
  fallback: string,
): string {
  function parseColor(value: string | null | undefined): [number, number, number, number] | null {
    const text = String(value || "").trim();
    const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(text);
    if (hex) {
      let digits = hex[1];
      if (digits.length === 3) digits = digits[0] + digits[0] + digits[1] + digits[1] + digits[2] + digits[2];
      return [parseInt(digits.slice(0, 2), 16), parseInt(digits.slice(2, 4), 16), parseInt(digits.slice(4, 6), 16), 1];
    }
    const match = /^rgba?\(\s*([^)]+)\)$/i.exec(text);
    if (!match) return null;
    const tokens = match[1]
      .replace(/\//g, " ")
      .split(/[\s,]+/)
      .filter(Boolean);
    if (tokens.length < 3 || tokens.length > 4) return null;
    const values = [Number(tokens[0]), Number(tokens[1]), Number(tokens[2]), tokens.length === 4 ? Number(tokens[3]) : 1];
    for (const item of values) {
      if (!Number.isFinite(item)) return null;
    }
    return [
      Math.min(255, Math.max(0, values[0])),
      Math.min(255, Math.max(0, values[1])),
      Math.min(255, Math.max(0, values[2])),
      Math.min(1, Math.max(0, values[3])),
    ];
  }
  const base = parseColor(fallback) ?? [255, 255, 255, 1];
  for (const candidate of candidates) {
    const color = parseColor(candidate);
    if (!color || color[3] <= 0) continue;
    const alpha = color[3];
    const red = Math.round(color[0] * alpha + base[0] * (1 - alpha));
    const green = Math.round(color[1] * alpha + base[1] * (1 - alpha));
    const blue = Math.round(color[2] * alpha + base[2] * (1 - alpha));
    return `rgb(${red}, ${green}, ${blue})`;
  }
  return `rgb(${Math.round(base[0])}, ${Math.round(base[1])}, ${Math.round(base[2])})`;
}

/** 首屏 CSS 兜底：即使运行时脚本尚未执行，html/body 也不会透出 Console 深色。 */
function previewSurfaceStyle(): string {
  return `<style data-preview-surface="1">
html, body { background-color: var(--background, ${PREVIEW_SURFACE_LIGHT}) !important; }
html[data-theme="dark"], html[data-theme="dark"] body { background-color: var(--background, ${PREVIEW_SURFACE_DARK}) !important; }
@media (prefers-color-scheme: dark) {
  html:not([data-theme="light"]), html:not([data-theme="light"]) body { background-color: var(--background, ${PREVIEW_SURFACE_DARK}) !important; }
}
</style>`;
}

/** 运行时读取文档主题变量并落成不透明 rgb；变量透明/缺失时按主题回退。 */
function previewSurfaceScript(): string {
  return `<script data-preview-surface-script="1">
(function () {
  // 源码经 esbuild keepNames 时会插入 __name(fn, "name")；iframe 里没有该辅助，提供空实现。
  var __name = function (fn) { return fn; };
  var fallbackFor = ${previewSurfaceFallback.toString()};
  var colorFor = ${previewSurfaceColor.toString()};
  function resolveColor(value) {
    if (!value) return '';
    try {
      var probe = document.createElement('span');
      probe.style.display = 'none';
      probe.style.color = 'rgb(1, 2, 3)';
      probe.style.color = value;
      (document.body || document.documentElement).appendChild(probe);
      var resolved = window.getComputedStyle(probe).color;
      probe.remove();
      if (resolved === 'rgb(1, 2, 3)' && String(value).replace(/\s+/g, '') !== 'rgb(1,2,3)') return '';
      return resolved;
    } catch (error) {
      return '';
    }
  }
  function candidates() {
    var result = [];
    [document.documentElement, document.body].forEach(function (el) {
      if (!el) return;
      var style = window.getComputedStyle(el);
      result.push(resolveColor(style.getPropertyValue('--background')));
      result.push(resolveColor(style.getPropertyValue('--bg')));
      result.push(style.backgroundColor);
    });
    return result;
  }
  function apply() {
    if (!document.documentElement) return;
    var theme = document.documentElement.getAttribute('data-theme');
    var media = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)');
    var fallback = fallbackFor(theme, Boolean(media && media.matches));
    var color = colorFor(candidates(), fallback);
    [document.documentElement, document.body].forEach(function (el) {
      if (!el || !el.style) return;
      try { el.style.setProperty('background-color', color, 'important'); } catch (error) {}
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', apply); else apply();
  window.addEventListener('load', apply);
  if (window.matchMedia) {
    var media = window.matchMedia('(prefers-color-scheme: dark)');
    if (media && media.addEventListener) media.addEventListener('change', apply);
  }
  if (window.MutationObserver) {
    try {
      new MutationObserver(apply).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    } catch (error) {}
  }
  setTimeout(apply, 50);
  setTimeout(apply, 300);
  // 样式表可能稍后完成（预览文档由服务端整页返回）；在前 2 秒内周期性复核，
  // 变量可用后把半透明/透明主题色合成为真正不透明背景。
  var attempts = 0;
  var refine = setInterval(function () {
    apply();
    attempts += 1;
    if (attempts >= 20) clearInterval(refine);
  }, 100);
})();
</script>`;
}

function insertBeforeClosingTag(document: string, tag: string, fragment: string): string {
  const closing = document.match(new RegExp(`</${tag}\\s*>`, "i"));
  if (closing && closing.index !== undefined) {
    return document.slice(0, closing.index) + fragment + document.slice(closing.index);
  }
  return tag === "head" ? insertAfterCharset(document, fragment) : document + fragment;
}

/**
 * 注入预览表面（不透明背景）。放在 </head> 前以确保晚于站点样式表，
 * 运行时脚本放在 </body> 前读取已计算的主题变量。幂等，纯函数。
 */
export function injectPreviewSurface(document: string): string {
  if (!document || document.includes('data-preview-surface="1"')) return document;
  const withStyle = insertBeforeClosingTag(document, "head", previewSurfaceStyle());
  return insertBeforeClosingTag(withStyle, "body", previewSurfaceScript());
}

/**
 * 服务端整页预览文档不是由 buildPreviewDoc 组装的，需在 <head> 最前面注入同一份存储回退，
 * 确保 rating/vote 等运行时脚本在 opaque origin 下读取 localStorage 不抛错。
 * 仅在 component-library 预览使用；真实站上下文保持旧行为（不注入）。
 * 幂等：已注入时原样返回。纯函数，可单测。
 */
export function injectPreviewStorageFallback(document: string): string {
  if (!document || document.includes('data-preview-storage="1"')) return document;
  return insertAfterCharset(document, PREVIEW_STORAGE_SHIM);
}

/**
 * 预览写守卫允许的精确 fixture 路径（query 允许，pathname 必须完全相等）。
 * 不再使用 /api/v1/preview/* 前缀，避免未来新增的鉴权/写端点被误放行。
 */
export const PREVIEW_WRITE_PATHS: readonly string[] = [
  "/api/v1/preview/rating",
  "/api/v1/preview/rating/info",
  "/api/v1/preview/rating/update",
  "/api/v1/preview/vote/info",
  "/api/v1/preview/vote/update",
  "/api/v1/preview/friends",
  "/api/v1/preview/sites",
  "/api/v1/preview/md",
];

/**
 * 写入守卫：sandbox srcdoc 是 opaque origin，浏览器 CORS 只会拦住“响应”，
 * 真实 POST 仍可能已经到达服务端（例如 /api/vote/update）。这里在预览文档内、
 * 站点脚本之前包装 fetch / XHR / sendBeacon：
 * - GET/HEAD/OPTIONS 放行；
 * - 非幂等方法只允许精确 fixture 路径集合中的相对 URL（query 允许）；
 * - 反斜杠在 WHATWG URL 中与 `/` 等价（`\\evil.example/...`、`/\evil.example/...`
 *   会被解析成外域协议相对 URL），写请求一律拒绝；
 * - //host 协议相对 URL、带 scheme 的绝对 URL、解析失败一律拒绝；
 * - 请求时读取实时 document.baseURI 解析，同时要求该 base 与守卫安装时的 base
 *   快照同源，防止站点动态插入 <base> 把 Fixture 相对路径导向外域；
 * - 解析出的实际 origin 必须与当前 base 的 origin 一致（不拿同一字符串重解析做恒真比较）；
 * - 被拒绝时在 window.__stellarPreviewWriteBlocked 留下诊断信息。
 * 纯 URL 判断抽成 previewWriteAllowed，便于单测并与内联脚本共用同一份实现。
 */
export function previewWriteAllowed(
  method: string,
  raw: string,
  baseURI: string,
  trustedBaseURI: string = baseURI,
  allowedPaths: readonly string[] = PREVIEW_WRITE_PATHS,
): boolean {
  const upper = String(method || "GET").toUpperCase();
  if (upper === "GET" || upper === "HEAD" || upper === "OPTIONS") return true;
  const value = String(raw || "").trim();
  if (!value) return false;
  // 反斜杠会被 URL 解析器当作 `/`，可把相对路径变成外域协议相对 URL，写请求直接拒绝。
  if (value.indexOf("\\") !== -1) return false;
  // 协议相对（//host/...）与带 scheme 的绝对 URL 一律视为跨文档请求，不允许写。
  if (value.indexOf("//") === 0 || /^[a-z][a-z0-9+.-]*:/i.test(value)) return false;
  // 不抽嵌套函数：保证 .toString() 注入 iframe 后不依赖打包器的 __name 辅助。
  let actualBase = String(baseURI || "");
  if (!actualBase || actualBase.indexOf("about:") === 0) actualBase = "http://preview.invalid/";
  let expectedBase = String(trustedBaseURI || "");
  if (!expectedBase || expectedBase.indexOf("about:") === 0) expectedBase = "http://preview.invalid/";
  let actual: URL;
  let actualBaseUrl: URL;
  let trustedBaseUrl: URL;
  try {
    actual = new URL(value, actualBase);
    actualBaseUrl = new URL(actualBase);
    trustedBaseUrl = new URL(expectedBase);
  } catch {
    return false;
  }
  if (!allowedPaths.includes(actual.pathname)) return false;
  // 当前 base 已被改到别的源（动态 <base>）时拒绝。
  if (actualBaseUrl.origin !== trustedBaseUrl.origin) return false;
  // 解析结果必须落在当前 base 的源内；相对路径不得被解析器带到外域。
  return actual.origin === actualBaseUrl.origin;
}

const PREVIEW_WRITE_GUARD = `<script data-preview-write-guard="1">
(function () {
  var allowed = ${previewWriteAllowed.toString()};
  var allowedPaths = ${JSON.stringify(PREVIEW_WRITE_PATHS)};
  var trustedBase = '';
  try { trustedBase = document.baseURI || ''; } catch (error) {}
  function currentBase() {
    try { return document.baseURI || ''; } catch (error) { return ''; }
  }
  function requestInfo(input, init) {
    var method = init && init.method ? init.method : (input && input.method ? input.method : 'GET');
    var url = typeof input === 'string' ? input : (input && input.url ? input.url : String(input));
    return { method: String(method).toUpperCase(), url: String(url) };
  }
  function blocked(method, url) {
    var info = { method: String(method), url: String(url), at: Date.now() };
    try { window.__stellarPreviewWriteBlocked = info; } catch (error) {}
    var failure = new Error('preview write blocked: ' + info.method + ' ' + info.url);
    failure.name = 'PreviewWriteBlockedError';
    return failure;
  }
  var rawFetch = window.fetch;
  if (typeof rawFetch === 'function') {
    window.fetch = function (input, init) {
      var info = requestInfo(input, init);
      if (!allowed(info.method, info.url, currentBase(), trustedBase, allowedPaths)) return Promise.reject(blocked(info.method, info.url));
      return rawFetch.apply(this, arguments);
    };
  }
  if (window.XMLHttpRequest && window.XMLHttpRequest.prototype) {
    var xhrProto = window.XMLHttpRequest.prototype;
    var rawOpen = xhrProto.open;
    var rawSend = xhrProto.send;
    xhrProto.open = function (method, url) {
      this.__previewRequest = { method: String(method || 'GET').toUpperCase(), url: String(url) };
      return rawOpen.apply(this, arguments);
    };
    xhrProto.send = function () {
      var info = this.__previewRequest;
      if (info && !allowed(info.method, info.url, currentBase(), trustedBase, allowedPaths)) throw blocked(info.method, info.url);
      return rawSend.apply(this, arguments);
    };
  }
  if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
    var rawBeacon = navigator.sendBeacon;
    navigator.sendBeacon = function (url) {
      if (!allowed('POST', String(url), currentBase(), trustedBase, allowedPaths)) {
        blocked('POST', String(url));
        return false;
      }
      return rawBeacon.apply(navigator, arguments);
    };
  }
})();
</script>`;

export function injectPreviewWriteGuard(document: string): string {
  if (!document || document.includes('data-preview-write-guard="1"')) return document;
  return insertAfterCharset(document, PREVIEW_WRITE_GUARD);
}

/**
 * 预览文档统一预处理：写守卫始终注入；内存 Storage 仅 component-library 预览注入。
 * 真实站上下文（NodeView/Markdown）不注入 Storage，rating/vote 保持不可交互的旧行为。
 */
export function preparePreviewDocument(document: string, profile?: string | null): string {
  // 五处预览都经过这里：先补不透明、跟随文档主题的表面，再注入写守卫与可选存储回退。
  const surfaced = injectPreviewSurface(document);
  const guarded = injectPreviewWriteGuard(surfaced);
  return profile === "component-library" ? injectPreviewStorageFallback(guarded) : guarded;
}

/** 前台运行时等价最小 shim（对齐 base.html 的 ctx/def/deps/utils，data_cache 关闭）。 */
const PREVIEW_SHIM = `<script>
const ctx = { date_suffix: { just: '刚刚', min: '分钟前', hour: '小时前', day: '天前' }, root: '/', tag_plugins: { chat: {} }, search: { service: 'local_search', path: '/search.json' }, data_cache: { enable: false, default_ttl: 0 }, copycode: {}, fancybox: {} };
const def = { avatar: "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3EA%3C/text%3E%3C/svg%3E", cover: '', loading: '' };
const deps = { marked: '/assets/vendor/marked/marked.umd.min.js', lazyload: '/assets/js/vendor/lazyload.min.js' };
(function () {
  var fixtures = window.__MOCK_FIXTURES__ || {};
  if (Object.keys(fixtures).length > 0) {
    var rawFetch = window.fetch;
    window.fetch = function (input, init) {
      var url = typeof input === 'string' ? input : (input && input.url ? input.url : '');
      var path = '';
      try {
        path = new URL(url, window.location.href).pathname;
      } catch (e) {
        // about:srcdoc 不能作为相对 URL 的 base；仍需去掉 query/hash 后做 pathname 精确匹配。
        path = String(url).replace(/^[a-z][a-z0-9+.-]*:\\/\\/[^/]+/i, '').split(/[?#]/)[0];
      }
      for (var key in fixtures) {
        var f = fixtures[key];
        if (f && f.endpoint && path === f.endpoint) {
          if (f.state === 'error') {
            return Promise.resolve(new Response(JSON.stringify({ status: 500, error: 'simulated fixture error' }), {
              status: 500,
              statusText: 'Internal Server Error',
              headers: { 'Content-Type': 'application/json' }
            }));
          }
          if (f.state === 'empty') {
            var emptyText = f.emptyData != null ? f.emptyData : '';
            return Promise.resolve(new Response(emptyText, {
              status: 200,
              headers: { 'Content-Type': f.format === 'markdown' ? 'text/markdown; charset=utf-8' : 'application/json; charset=utf-8' }
            }));
          }
          if (f.customData != null && f.customData !== '') {
            return Promise.resolve(new Response(f.customData, {
              status: 200,
              headers: { 'Content-Type': f.format === 'markdown' ? 'text/markdown; charset=utf-8' : 'application/json; charset=utf-8' }
            }));
          }
        }
      }
      return rawFetch.apply(this, arguments);
    };
  }
})();
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
(function () {
  function markImage(img) {
    if (img.getAttribute('data-preview-lazy-bound') === '1') return;
    img.setAttribute('data-preview-lazy-bound', '1');
    var source = img.getAttribute('data-src');
    if (source && img.getAttribute('src') !== source) img.setAttribute('src', source);
    img.classList.add('loading');
    var done = function () { img.classList.remove('loading'); img.classList.add('loaded'); };
    var failed = function () { img.classList.remove('loading'); img.classList.add('error'); };
    img.addEventListener('load', done, { once: true });
    img.addEventListener('error', failed, { once: true });
    if (img.complete) {
      if (img.naturalWidth > 0) done();
      else if (img.getAttribute('src')) failed();
    }
  }
  function markBackground(el) {
    var source = el.getAttribute('data-bg');
    if (!source) return;
    el.style.backgroundImage = 'url("' + source.replace(/"/g, '\\"') + '")';
    el.classList.add('loaded');
  }
  function eagerLoad(container) {
    if (typeof container === 'string') container = document.querySelector(container);
    if (!container) container = document;
    if (container.nodeType === 1 && container.classList.contains('lazy')) {
      if (container.tagName === 'IMG') markImage(container);
      else markBackground(container);
    }
    container.querySelectorAll('.lazy').forEach(function (el) {
      if (el.tagName === 'IMG') markImage(el);
      else markBackground(el);
    });
  }
  // 前台 wrapLazyloadImages 会把裸 img 包进 .lazy-box（layout/_partial/scripts/lazyload.ejs），
  // 卡片样式全部挂在 .lazy-box > img 上（如 .sites-wrap .site-card）。预览若只做 eagerLoad
  // 而不包裹，站点卡片的封面裁切/圆角/阴影与 28px 头像全部落空，与前台渲染不一致。
  function wrapBareImages(container) {
    if (typeof container === 'string') container = document.querySelector(container);
    if (!container || container.nodeType !== 1) return;
    container.querySelectorAll('img').forEach(function (img) {
      if (img.classList.contains('lazy')) return; // 幂等：已包裹的不重复处理
      var src = img.getAttribute('src');
      if (!src) return;
      var wrapper = document.createElement('div');
      wrapper.className = 'lazy-box';
      var next = img.cloneNode();
      next.removeAttribute('src');
      next.setAttribute('data-src', src);
      next.classList.add('lazy');
      var icon = document.createElement('div');
      icon.className = 'lazy-icon';
      wrapper.appendChild(next);
      wrapper.appendChild(icon);
      img.replaceWith(wrapper);
    });
  }
  // 先包裹再 eagerLoad：包裹产生的 img.lazy 由 eagerLoad 把 data-src 解析回 src，
  // 后续 MutationObserver 也会覆盖异步插入的节点。
  function wrapLazyloadImages(container) {
    wrapBareImages(container);
    eagerLoad(container);
  }
  window.wrapLazyloadImages = wrapLazyloadImages;
  eagerLoad(document);
  if (window.MutationObserver && document.body) {
    new MutationObserver(function (mutations) {
      mutations.forEach(function (mutation) {
        mutation.addedNodes.forEach(function (node) {
          if (node.nodeType === 1) eagerLoad(node);
        });
      });
    }).observe(document.body, { childList: true, subtree: true });
  }
})();
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
      var empty = box ? !box.children.length : !el.textContent.trim();
      if (empty && !el.querySelector('.preview-hint')) {
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

/** iframe 高度回传：ResizeObserver + load 兜底，异步内容加载完成后不被裁剪。
 *  必须用 body.scrollHeight（内容高度）优先：preview iframe 的视口高度 = 父级设置的
 *  iframe 高度，而 main.css 的 html{height:100%} 使 documentElement.scrollHeight 随
 *  视口反馈增长（420→444→468…→900 上限的正反馈环），body.scrollHeight 只反映内容。 */
const RESIZE_SCRIPT = `<script>
(function () {
  function report() {
    var h = document.body.scrollHeight || document.documentElement.scrollHeight || 0;
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

// ---------- 预览高度（Spec 64：推荐高度 + 内容高度回传） ----------

/** 预览 iframe 高度上下限：小插件不再被固定 420px 画布包围，高内容最大 900px。 */
export const MIN_PREVIEW_HEIGHT = 60;
export const MAX_PREVIEW_HEIGHT = 900;
/** 未登记推荐高度的插件（静态兜底）初始高度。 */
export const DEFAULT_PREVIEW_HEIGHT = 240;

/**
 * 解析插件推荐初始高度：取 PluginPreviewInfo.height，缺省用默认值，并钳制到上下限。
 * 纯函数，可脱离 Vue 单测（tests/plugin-preview.test.ts）。
 */
export function resolvePreviewHeight(info: { height?: number }): number {
  const h = info.height ?? DEFAULT_PREVIEW_HEIGHT;
  return Math.min(MAX_PREVIEW_HEIGHT, Math.max(MIN_PREVIEW_HEIGHT, h));
}

/** 按内容高度回传值计算 iframe 高度：钳制在 [minHeight, MAX_PREVIEW_HEIGHT] 内。 */
export function clampPreviewHeight(reported: number, minHeight: number): number {
  return Math.min(MAX_PREVIEW_HEIGHT, Math.max(minHeight, Math.ceil(reported) + 24));
}

/** 旧文档存在但当前参数无效时，画布叠加“参数无效，预览未更新”，不隐藏旧预览。 */
export function previewInvalidNoticeVisible(canApply: boolean, parsing: boolean, hasDocument: boolean): boolean {
  return !canApply && !parsing && hasDocument;
}

/** Markdown 分屏失败文案：没有历史文档时不能声称“仍显示上一次结果”。 */
export function previewFailureMessage(hasDocument: boolean): string {
  return hasDocument ? "预览渲染失败，仍显示上一次结果" : "预览渲染失败";
}

/**
 * 校验 component-preview-resize 消息并返回有效高度。
 * token 必须是当前文档的 token；其他窗口、旧文档或非法尺寸一律返回 null。
 */
export function previewHeightFromMessage(data: unknown, token: string): number | null {
  if (!data || typeof data !== "object") return null;
  const message = data as { type?: unknown; token?: unknown; height?: unknown };
  if (message.type !== "component-preview-resize" || message.token !== token) return null;
  const height = message.height;
  return typeof height === "number" && Number.isFinite(height) && height > 0 ? height : null;
}

/**
 * 构建预览 iframe 文档（srcdoc）。标签插件库与侧边栏组件库共用：
 * 样式注入（main.css/tag-plugins.css/highlight.css + 字体）、固定暗色主题、
 * 前台运行时 shim、按能力脚本、降级提示、高度回传。
 */
export function buildPreviewDoc(opts: PreviewDocOptions): string {
  const {
    bodyHtml,
    scripts = [],
    extraHead = "",
    bodyClass = "",
    wrapContent = true,
    degradeHints = true,
    autoHeight = true,
    inlineScripts = [],
  } = opts;

  const styleLinks = PREVIEW_STYLES.map((href) => `<link rel="stylesheet" href="${href}">`).join("\n");

  const scriptTags = scripts.map((src) => `<script src="${src}" defer></script>`).join("\n");
  const inlineTags = inlineScripts.join("\n");
  const body = wrapContent ? `<article class="md-text content">${safeHtml(bodyHtml)}</article>` : safeHtml(bodyHtml);

  const fixtureScript =
    opts.mockFixtures && Object.keys(opts.mockFixtures).length > 0
      ? `<script>window.__MOCK_FIXTURES__ = ${JSON.stringify(opts.mockFixtures).replace(/<\/script/gi, "<\\/script")};</script>`
      : "";

  return `<!doctype html>
<html lang="zh-CN" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
${styleLinks}
<style>
html,body{margin:0;padding:0}
body{padding:24px;background:var(--background,#1c1c1e);font-family:"LXGW WenKai Screen",system-ui,"Microsoft Yahei","Helvetica Neue",Helvetica,Arial,sans-serif}
.md-text{max-width:760px;margin:0 auto}
.dropdown[open] > .dropdown-menu{visibility:visible;opacity:1}
.preview-hint{margin:10px 0;padding:10px 12px;border:1px dashed var(--border,#6e6e73);border-radius:8px;color:var(--text-p3,#8e8e93);font-size:13px}
</style>
${extraHead}
</head>
<body${bodyClass ? ` class="${bodyClass}"` : ""}>
${body}
${fixtureScript}
${PREVIEW_STORAGE_SHIM}
${PREVIEW_SHIM}
${scriptTags}
${inlineTags}
${degradeHints ? DEGRADE_SCRIPT : ""}
${autoHeight ? RESIZE_SCRIPT : ""}
</body>
</html>`;
}
