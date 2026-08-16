// copycode.js: 代码块复制按钮，配置由 ctx.copycode 提供。
(function () {
  "use strict";

  function init() {
    var cfg = (typeof ctx !== "undefined" && ctx.copycode) || {};
    if (cfg.enable === false) return;
    document.querySelectorAll(".highlight .code").forEach(function (codeElement) {
      if (codeElement.querySelector(".copy-btn")) return;
      var button = document.createElement("button");
      button.type = "button";
      button.className = "copy-btn";
      button.textContent = cfg.default_text || "Copy";
      button.addEventListener("click", async function () {
        var code = codeElement.querySelector("pre");
        var text = code ? code.innerText : "";
        try {
          if (!navigator.clipboard) throw new Error("clipboard unavailable");
          await navigator.clipboard.writeText(text);
          button.textContent = cfg.success_text || "Copied";
          button.classList.add("success");
          if (typeof hud !== "undefined" && hud.toast) hud.toast(cfg.toast || "Copied", 2500);
        } catch (_) {
          button.textContent = "复制失败";
          button.classList.add("warning");
        }
        setTimeout(function () {
          button.textContent = cfg.default_text || "Copy";
          button.classList.remove("success", "warning");
        }, 3000);
      });
      codeElement.appendChild(button);
    });
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
