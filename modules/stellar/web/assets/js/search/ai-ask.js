// AI knowledge-base Q&A for the Stellar search surface.
// The interaction follows ai-summary.js: delegated events, rune-safe text output,
// a short typewriter effect, and no persistence of the user's API key.
(function () {
  "use strict";

  var activeTimer = null;

  function writeAnswer(target, text) {
    if (activeTimer) clearInterval(activeTimer);
    target.replaceChildren();
    var body = document.createElement("p");
    target.appendChild(body);
    var cursor = document.createElement("span");
    cursor.className = "ai-cursor";
    body.appendChild(cursor);
    var chars = Array.from(text || "");
    var index = 0;
    activeTimer = setInterval(function () {
      if (index < chars.length) {
        var step = index + 2 <= chars.length ? 2 : 1;
        cursor.before(document.createTextNode(chars.slice(index, index + step).join("")));
        index += step;
        return;
      }
      clearInterval(activeTimer);
      activeTimer = null;
      cursor.remove();
    }, 24);
  }

  function addCitations(target, citations) {
    if (!Array.isArray(citations) || citations.length === 0) return;
    var heading = document.createElement("div");
    heading.className = "search-ai-ask-citations-title";
    heading.textContent = "文章来源";
    target.appendChild(heading);
    var list = document.createElement("div");
    list.className = "search-ai-ask-citations";
    citations.forEach(function (source) {
      var link = document.createElement("a");
      link.href = source.url || "#";
      link.textContent = "[" + (source.id || "S") + "] " + (source.title || source.slug || "原文");
      link.setAttribute("rel", "noreferrer");
      list.appendChild(link);
    });
    target.appendChild(list);
  }

  function showError(root, message) {
    var result = root.querySelector("[data-rag-ai-result]");
    if (!result) return;
    result.hidden = false;
    result.classList.add("is-error");
    result.textContent = message || "AI 问答失败，请稍后重试";
  }

  function ask(root, event) {
    event.preventDefault();
    var form = root.querySelector("[data-rag-ai-form]");
    var input = root.querySelector("[data-rag-ai-input]");
    var question = input && input.value.trim();
    if (!question) {
      showError(root, "请输入问题");
      return;
    }
    var button = root.querySelector(".search-ai-ask-submit");
    if (button) {
      button.disabled = true;
      button.textContent = "回答中…";
    }
    var endpoint = root.querySelector("[data-rag-ai-endpoint]");
    var model = root.querySelector("[data-rag-ai-model]");
    var key = root.querySelector("[data-rag-ai-key]");
    var payload = { question: question };
    if (endpoint && endpoint.value.trim()) payload.endpoint = endpoint.value.trim();
    if (model && model.value.trim()) payload.model = model.value.trim();
    if (key && key.value.trim()) payload.api_key = key.value.trim();
    fetch("/api/v1/rag/ask/public", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    }).then(function (response) {
      return response.json().then(function (data) { return { ok: response.ok, data: data }; });
    }).then(function (result) {
      if (!result.ok) {
        showError(root, result.data && (result.data.error || result.data.detail));
        return;
      }
      var output = root.querySelector("[data-rag-ai-result]");
      if (!output) return;
      output.hidden = false;
      output.classList.remove("is-error");
      writeAnswer(output, result.data.answer || "知识库没有返回答案。");
      addCitations(output, result.data.citations);
      if (key) key.value = "";
    }).catch(function () {
      showError(root, "网络错误，请检查连接后重试");
    }).finally(function () {
      if (button) {
        button.disabled = false;
        button.textContent = "发送";
      }
    });
  }

  document.addEventListener("click", function (event) {
    var open = event.target.closest && event.target.closest("[data-rag-ai-open]");
    if (open) {
      var root = open.closest("#rag-ai-ask");
      var form = root && root.querySelector("[data-rag-ai-form]");
      if (form) {
        form.hidden = !form.hidden;
        if (!form.hidden) {
          var input = form.querySelector("[data-rag-ai-input]");
          if (input) input.focus();
        }
      }
      return;
    }
    var byok = event.target.closest && event.target.closest("[data-rag-ai-byok]");
    if (byok) {
      var byokRoot = byok.closest("#rag-ai-ask");
      var byokForm = byokRoot && byokRoot.querySelector("[data-rag-ai-byok-form]");
      if (byokForm) byokForm.hidden = !byokForm.hidden;
    }
  });

  document.addEventListener("submit", function (event) {
    var form = event.target.closest && event.target.closest("[data-rag-ai-form]");
    if (form) ask(form.closest("#rag-ai-ask"), event);
  });
})();
