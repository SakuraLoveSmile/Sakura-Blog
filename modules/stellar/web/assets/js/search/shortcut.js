// 快捷键由 Stellar 2.0 搜索扩展统一挂载，移动端同样可打开 dialog。
(function() {
  function isSearchShortcut(event) {
    if (event.defaultPrevented || event.isComposing) return false;
    if (event.altKey || event.shiftKey) return false;
    if (!event.metaKey && !event.ctrlKey) return false;
    return typeof event.key === 'string' && event.key.toLowerCase() === 'k';
  }
  function isEditableTarget(target) {
    if (!target || typeof target.closest !== 'function') return false;
    return target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])') !== null;
  }
  function mount(openSearch) {
    function handleShortcut(event) {
      if (!isSearchShortcut(event)) return;
      var input = document.getElementById('search-input');
      if (!input || (event.target !== input && isEditableTarget(event.target))) return;
      if (typeof openSearch === 'function') {
        if (openSearch(document.activeElement) === false) return;
      } else {
        // 兼容旧页面；2.0 页面只能由搜索扩展打开原生 dialog。
        if (document.getElementById('site-search-dialog')) return;
        input.focus();
      }
      event.preventDefault();
    }
    document.addEventListener('keydown', handleShortcut);
    return function() { document.removeEventListener('keydown', handleShortcut); };
  }
  window.blogSearchShortcut = { mount: mount };
  function init() {
    if (!document.getElementById('stellar-runtime-config')) mount();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init, { once: true });
  else init();
})();
