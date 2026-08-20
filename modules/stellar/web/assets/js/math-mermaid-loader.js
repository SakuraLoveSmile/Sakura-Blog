// Spec 61 阶段 1 + Spec 64 依赖本地化：KaTeX（数学公式）/ Mermaid（图表）按需加载器。
// 页面 .md-text 中存在数学定界符或 .mermaid 元素时才动态注入资源，非公式页零请求。
// Spec 64：mermaid@11.16.1 固定为本地 vendor（/assets/vendor/mermaid/），本地加载失败时
// 才回退 CDN（与 Spec 53 swiper 依赖本地化同原则）；KaTeX 未 vendor，维持 CDN。
// 导航为整页刷新（Spec 59 起无 PJAX），DOMContentLoaded 扫描一次即可；加载失败静默降级（保持原文）。
(function () {
  "use strict";

  var KATEX_CSS = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.css";
  var KATEX_JS = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.js";
  var KATEX_AUTORENDER = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/contrib/auto-render.min.js";
  var MERMAID_JS = "/assets/vendor/mermaid/mermaid.min.js";
  var MERMAID_CDN = "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js";
  var pluginConfig = window.__stellarPlugins || {};
  var MATH_DELIMITERS = [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "$", right: "$", display: false },
    { left: "\\(", right: "\\)", display: false }
  ];

  // 是否存在数学定界符：显式标记（$$ \[ \] \( \)）+ 行内 $...$ 对。
  // 启发式偏宽松：误判只浪费一次按需加载，漏判则公式不渲染。
  function hasMath(el) {
    var text = el.textContent || "";
    if (text.indexOf("$$") !== -1) return true;
    if (text.indexOf("\\[") !== -1 || text.indexOf("\\]") !== -1) return true;
    if (text.indexOf("\\(") !== -1 || text.indexOf("\\)") !== -1) return true;
    return /\$[^$\n]+\$/.test(text);
  }

  function loadCss(href) {
    return new Promise(function (resolve) {
      var link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = href;
      link.setAttribute("crossorigin", "anonymous");
      link.onload = resolve;
      // A failed stylesheet must not prevent Mermaid from rendering with its
      // official theme; resolve on error and let the renderer continue.
      link.onerror = resolve;
      document.head.appendChild(link);
    });
  }

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = src;
      s.setAttribute("crossorigin", "anonymous");
      s.onload = resolve;
      s.onerror = function () {
        s.remove();
        reject(new Error("load failed: " + src));
      };
      document.head.appendChild(s);
    });
  }

  // 本地 vendor 优先，加载失败回退 CDN（mermaid 专用；KaTeX 未 vendor 直接用 CDN）。
  function loadScriptLocalFirst(localSrc, cdnSrc) {
    return loadScript(localSrc).catch(function () {
      return loadScript(cdnSrc);
    });
  }

  function loadMath() {
    loadCss(KATEX_CSS);
    loadScript(KATEX_JS)
      .then(function () {
        return loadScript(KATEX_AUTORENDER);
      })
      .then(function () {
        document.querySelectorAll(".md-text").forEach(function (el) {
          renderMathInElement(el, { delimiters: MATH_DELIMITERS, throwOnError: false });
        });
      })
      .catch(function () { /* CDN 不可达：静默降级，保持原文 */ });
  }

  function loadMermaid() {
    var mermaidConfig = pluginConfig.mermaid || {};
    var configured = mermaidConfig.js;
    var styleOptimization = mermaidConfig.style_optimization === true;
    // 1.43.0 precedence: explicit URL, local vendor, then CDN fallback.
    var promise = configured
      ? loadScript(configured).catch(function () { return loadScriptLocalFirst(MERMAID_JS, MERMAID_CDN); })
      : loadScriptLocalFirst(MERMAID_JS, MERMAID_CDN);
    promise
      .then(function () {
        // Official Mermaid theme is deliberately left untouched when custom
        // Stellar CSS is disabled. The configured theme remains compatible;
        // neutral is the upstream default for this mode.
        var cssReady = styleOptimization ? loadCss("/assets/css/mermaid.css") : Promise.resolve();
        return cssReady.then(function () {
          var theme = styleOptimization ? (mermaidConfig.theme || "dark") : (mermaidConfig.theme || "neutral");
          mermaid.initialize({ startOnLoad: false, theme: theme });
          return mermaid.run({ querySelector: ".md-text .mermaid, .mermaid" });
        });
      })
      .catch(function () { /* 本地 vendor 与 CDN 均不可达：静默降级，图表显示源码 */ });
  }

  function init() {
    var needMath = false;
    document.querySelectorAll(".md-text").forEach(function (el) {
      if (hasMath(el)) needMath = true;
    });
    var needMermaid = !!document.querySelector(".md-text .mermaid, .mermaid");
    var katexEnabled = !pluginConfig.katex || pluginConfig.katex.enable !== false;
    var mermaidEnabled = !pluginConfig.mermaid || pluginConfig.mermaid.enable !== false;
    if (needMath && katexEnabled) loadMath();
    if (needMermaid && mermaidEnabled) loadMermaid();

    var mathjax = pluginConfig.mathjax;
    if (mathjax && mathjax.enable && mathjax.js) {
      if (mathjax.v3) {
        window.MathJax = {
          tex: {
            inlineMath: [["$", "$"], ["\\(", "\\)"]],
            processEscapes: true,
            skipTags: ["script", "noscript", "style", "textarea", "pre", "code"]
          }
        };
      }
      loadScript(mathjax.js).catch(function () { /* MathJax 不可达时保留原文 */ });
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
