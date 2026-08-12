// Spec 61 阶段 1：KaTeX（数学公式）/ Mermaid（图表）按需加载器。
// 页面 .md-text 中存在数学定界符或 .mermaid 元素时才动态注入 CDN 资源，非公式页零请求。
// 导航为整页刷新（Spec 59 起无 PJAX），DOMContentLoaded 扫描一次即可；加载失败静默降级（保持原文）。
(function () {
  "use strict";

  var KATEX_CSS = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.css";
  var KATEX_JS = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.js";
  var KATEX_AUTORENDER = "https://cdn.jsdelivr.net/npm/katex@0.16/dist/contrib/auto-render.min.js";
  var MERMAID_JS = "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js";
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
    var link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.setAttribute("crossorigin", "anonymous");
    document.head.appendChild(link);
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
    loadScript(MERMAID_JS)
      .then(function () {
        mermaid.initialize({ startOnLoad: false, theme: "dark" });
        mermaid.run({ querySelector: ".md-text .mermaid, .mermaid" });
      })
      .catch(function () { /* CDN 不可达：静默降级，图表显示源码 */ });
  }

  function init() {
    var needMath = false;
    document.querySelectorAll(".md-text").forEach(function (el) {
      if (hasMath(el)) needMath = true;
    });
    var needMermaid = !!document.querySelector(".md-text .mermaid, .mermaid");
    if (needMath) loadMath();
    if (needMermaid) loadMermaid();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
