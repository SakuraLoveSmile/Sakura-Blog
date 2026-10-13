// 保留 Blog 的搜索索引和 Wiki data-filter，只适配 2.0 原生 dialog 生命周期。
export function mount(documentRef) {
  const dialog = documentRef.getElementById('site-search-dialog');
  const input = documentRef.getElementById('search-input');
  if (!dialog || !input) return;
  const view = documentRef.defaultView;
  if (!view.blogLocalSearch || !view.blogSearchShortcut) throw new Error('Blog search scripts were not loaded');
  const root = documentRef.documentElement;
  const cleanups = [];
  let trigger = null;
  function listen(target, type, handler, options) {
    target.addEventListener(type, handler, options);
    cleanups.push(() => target.removeEventListener(type, handler, options));
  }
  function restoreFocus() {
    root.removeAttribute('data-search-open');
    documentRef.querySelectorAll('[data-search-action="open"]').forEach(button => button.setAttribute('aria-expanded', 'false'));
    if (trigger?.isConnected && !trigger.closest('[inert]')) trigger.focus();
    trigger = null;
  }
  function close() {
    if (dialog.open) dialog.close();
    restoreFocus();
  }
  function open(button) {
    if (!dialog.open) {
      trigger = button || documentRef.activeElement;
      const gutter = Math.max(0, view.innerWidth - root.clientWidth);
      root.style.setProperty('--stellar-scrollbar-gutter', gutter + 'px');
      // 原生 showModal 保证焦点限制与背景 inert；不使用 open 属性模拟 dialog。
      dialog.showModal();
      root.setAttribute('data-search-open', '');
      documentRef.querySelectorAll('[data-search-action="open"]').forEach(button => button.setAttribute('aria-expanded', 'true'));
    }
    input.focus();
    return true;
  }
  view.blogLocalSearch.mount();
  cleanups.push(() => view.blogLocalSearch?.unmount());
  cleanups.push(view.blogSearchShortcut.mount(open));
  listen(documentRef, 'click', event => {
    const button = event.target.closest?.('[data-search-action]');
    if (!button || event.defaultPrevented) return;
    if (button.dataset.searchAction === 'open') { event.preventDefault(); open(button); }
    else if (button.dataset.searchAction === 'close') { event.preventDefault(); close(); }
  });
  // type=search 的原生 Escape 会先清空文字；捕获按键保证一次即可退出浮层。
  listen(dialog, 'keydown', event => {
    if (event.key !== 'Escape' || event.isComposing || !dialog.open) return;
    event.preventDefault();
    event.stopPropagation();
    close();
  }, { capture: true });
  listen(dialog, 'cancel', event => { event.preventDefault(); close(); });
  listen(dialog, 'close', restoreFocus);
  listen(dialog, 'click', event => {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) close();
  });
  return () => {
    cleanups.reverse().forEach(cleanup => cleanup());
    trigger = null;
    close();
  };
}
