// ai-summary.js | Spec 53 AI 文章摘要：读者自助生成面板交互。
// 面板结构由 partial/content.html 渲染（.ai-summary 容器 + data-user-api 开关）。
// 生成走 POST /api/v1/ai/summary/guest 透传读者自己的 OpenAI 兼容 API：
// Key 仅随本次请求发送，不落服务端；「记住」勾选仅存接口/模型到 localStorage（不含 Key）。
// document 级事件委托绑定，PJAX 换页后 DOM 替换不受影响。
(function () {
  var STORE_KEY = 'Stellar.ai';
  var CONTENT_LIMIT = 40000; // 与后端 aiGuestContentLimit 对齐，前端先截断省流量

  function loadSaved() {
    try {
      var v = JSON.parse(localStorage.getItem(STORE_KEY) || '{}');
      return { endpoint: v.endpoint || '', model: v.model || '' };
    } catch (e) {
      return { endpoint: '', model: '' };
    }
  }

  function savePrefs(endpoint, model) {
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify({ endpoint: endpoint, model: model }));
    } catch (e) { /* 隐私模式等场景静默 */ }
  }

  // 取文章正文纯文本（送模型的素材），截断到 CONTENT_LIMIT
  function articleContent() {
    var el = document.querySelector('article.md-text.content');
    if (!el) return '';
    var text = (el.innerText || '').trim();
    var runes = Array.from(text);
    if (runes.length > CONTENT_LIMIT) {
      return runes.slice(0, CONTENT_LIMIT).join('');
    }
    return text;
  }

  function showError(root, msg) {
    var btn = root.querySelector('.ai-summary-generate');
    if (btn) {
      btn.disabled = false;
      btn.textContent = btn.dataset.label || btn.textContent;
    }
    if (window.hud && hud.toast) {
      hud.toast(msg, 4000);
    }
  }

  function renderSummary(root, text) {
    var body = root.querySelector('.body');
    if (body) {
      body.replaceChildren(); // 清空旧占位，不用 innerHTML 避免解析标记
      var p = document.createElement('p');
      p.textContent = text;
      body.appendChild(p);
    }
    var panel = root.querySelector('.ai-summary-panel');
    if (panel) {
      panel.remove(); // 已出结果，收起自助入口
    }
    if (window.hud && hud.toast) {
      hud.toast(root.dataset.doneMsg || '摘要已生成', 2500);
    }
  }

  function generate(root) {
    var endpoint = (root.querySelector('.ai-summary-endpoint').value || '').trim();
    var model = (root.querySelector('.ai-summary-model').value || '').trim();
    var key = (root.querySelector('.ai-summary-key').value || '').trim();
    var remember = root.querySelector('.ai-summary-remember-check');
    if (!endpoint || !model) {
      showError(root, root.dataset.requiredMsg || '请填写接口地址与模型');
      return;
    }
    if (remember && remember.checked) {
      savePrefs(endpoint, model); // 只存接口与模型，Key 永不落浏览器
    }
    var btn = root.querySelector('.ai-summary-generate');
    if (btn) {
      btn.dataset.label = btn.textContent;
      btn.disabled = true;
      btn.textContent = root.dataset.generatingMsg || '生成中…';
    }
    fetch('/api/v1/ai/summary/guest', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: endpoint, api_key: key, model: model, content: articleContent() })
    }).then(function (resp) {
      return resp.json().then(function (data) { return { ok: resp.ok, data: data }; });
    }).then(function (r) {
      if (r.ok && r.data.summary) {
        renderSummary(root, r.data.summary);
        return;
      }
      showError(root, (r.data && r.data.error) || '生成失败');
    }).catch(function () {
      showError(root, '网络错误，请检查连接后重试');
    });
  }

  // document 级委托：展开按钮 + 生成按钮（PJAX 友好，只绑一次）
  document.addEventListener('click', function (e) {
    var open = e.target.closest && e.target.closest('.ai-summary-open');
    if (open) {
      var form = open.parentElement.querySelector('.ai-summary-form');
      if (form) {
        form.hidden = !form.hidden;
        if (!form.hidden && !form.dataset.prefilled) {
          form.dataset.prefilled = '1';
          var saved = loadSaved();
          var root = open.closest('#ai-summary');
          var ep = root.querySelector('.ai-summary-endpoint');
          var md = root.querySelector('.ai-summary-model');
          if (ep && saved.endpoint) ep.value = saved.endpoint;
          if (md && saved.model) md.value = saved.model;
          if (saved.endpoint || saved.model) {
            var cb = root.querySelector('.ai-summary-remember-check');
            if (cb) cb.checked = true;
          }
        }
      }
      return;
    }
    var gen = e.target.closest && e.target.closest('.ai-summary-generate');
    if (gen) {
      var root = gen.closest('#ai-summary');
      if (root) generate(root);
    }
  });
})();
