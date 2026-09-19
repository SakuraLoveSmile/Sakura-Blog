/**
 * checkbox-interactive.js - {% checkbox %} 交互模式（tag_plugins.checkbox.interactive=true 时加载）
 * 上游 Stellar interactive 仅为 CSS pointer-events 开关（点击可切换勾选态，刷新即还原）；
 * 本实现在此基础上增加 localStorage 持久化（自有扩展）：按页面路径 + 序号记忆勾选状态。
 * 零依赖事件委托：正文内 .tag-plugin.checkbox / .tag-plugin.radio 的 input 点击即切换，
 * 无匹配元素时零副作用；状态变更写 localStorage（key: cb:<pathname>:<序号>）。
 */
(function () {
  'use strict';

  var STORE_PREFIX = 'cb:';

  function keyFor(el) {
    var boxes = document.querySelectorAll('.md-text .tag-plugin.checkbox input, .md-text .tag-plugin.radio input');
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i] === el) return STORE_PREFIX + location.pathname + ':' + i;
    }
    return null;
  }

  function restore() {
    var boxes = document.querySelectorAll('.md-text .tag-plugin.checkbox input, .md-text .tag-plugin.radio input');
    for (var i = 0; i < boxes.length; i++) {
      try {
        var saved = localStorage.getItem(STORE_PREFIX + location.pathname + ':' + i);
        if (saved === null) continue;
        boxes[i].checked = saved === '1';
      } catch (e) { /* localStorage 不可用时保持静态渲染结果 */ }
    }
  }

  document.addEventListener('click', function (event) {
    var input = event.target;
    if (!input || input.tagName !== 'INPUT') return;
    var wrap = input.closest('.tag-plugin.checkbox, .tag-plugin.radio');
    if (!wrap) return;
    var key = keyFor(input);
    if (!key) return;
    try { localStorage.setItem(key, input.checked ? '1' : '0'); } catch (e) {}
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', restore);
  } else {
    restore();
  }
})();
