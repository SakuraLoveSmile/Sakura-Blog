// Blog shell：采用 Stellar 2.0 的 region/drawer 和 root leftbar-state 契约。
export function mount(documentRef) {
  const shell = documentRef.querySelector('.site-shell[data-regions]');
  if (!shell) return;
  const view = documentRef.defaultView;
  const root = documentRef.documentElement;
  const media = {
    leftbar: view.matchMedia('(max-width: 768px)'),
    rightbar: view.matchMedia('(max-width: 1180px)'),
  };
  const region = name => documentRef.getElementById(name + '-region');
  const cleanups = [];
  let trigger = null;
  function listen(target, type, handler) {
    target.addEventListener(type, handler);
    cleanups.push(() => target.removeEventListener(type, handler));
  }
  function sync() {
    const open = shell.dataset.drawer;
    for (const name of ['leftbar', 'rightbar']) {
      const panel = region(name);
      const expanded = open === name && media[name].matches;
      if (panel) {
        panel.inert = media[name].matches && !expanded;
        if (panel.inert) panel.setAttribute('aria-hidden', 'true');
        else panel.removeAttribute('aria-hidden');
      }
      documentRef.querySelectorAll(`[data-shell-action="toggle-${name}-drawer"]`).forEach(button => {
        button.setAttribute('aria-expanded', String(expanded));
      });
    }
    documentRef.querySelectorAll('[data-shell-action="toggle-leftbar"]').forEach(button => {
      const expanded = media.leftbar.matches ? open === 'leftbar' : root.dataset.leftbarState !== 'collapsed';
      button.setAttribute('aria-expanded', String(expanded));
      button.setAttribute('aria-label', expanded ? '收起菜单' : '展开菜单');
    });
    // 桌面收起左栏后，上游将右栏显示为窄轨；让键盘可展开其内容。
    const right = region('rightbar');
    if (right) {
      if (!media.rightbar.matches && root.dataset.leftbarState === 'collapsed') right.setAttribute('tabindex', '0');
      else right.removeAttribute('tabindex');
    }
  }
  function dismiss(options = {}) {
    const wasOpen = !!shell.dataset.drawer;
    delete shell.dataset.drawer;
    sync();
    if (wasOpen && options.restoreFocus !== false && trigger?.isConnected) trigger.focus();
    trigger = null;
  }
  function toggleDrawer(name, button) {
    const panel = region(name);
    if (!panel || !media[name]?.matches) return;
    if (shell.dataset.drawer === name) { dismiss(); return; }
    trigger = button || documentRef.activeElement;
    shell.dataset.drawer = name;
    sync();
    const first = [...panel.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
      .find(element => !element.closest('[hidden], [inert]') && element.getClientRects().length > 0);
    if (first) first.focus();
    else { panel.setAttribute('tabindex', '-1'); panel.focus(); }
  }
  function toggleLeftbar(button) {
    if (media.leftbar.matches) { toggleDrawer('leftbar', button); return; }
    root.dataset.leftbarState = root.dataset.leftbarState === 'collapsed' ? 'expanded' : 'collapsed';
    try { view.localStorage.setItem('stellar:v2:leftbar-state', root.dataset.leftbarState); } catch (error) { void error; }
    sync();
  }
  try {
    const saved = view.localStorage.getItem('stellar:v2:leftbar-state');
    if (saved === 'collapsed' || saved === 'expanded') root.dataset.leftbarState = saved;
  } catch (error) { void error; }
  const actions = {
    'toggle-leftbar': toggleLeftbar,
    'toggle-leftbar-drawer': button => toggleDrawer('leftbar', button),
    'toggle-rightbar-drawer': button => toggleDrawer('rightbar', button),
    'dismiss-drawer': () => dismiss(),
  };
  listen(documentRef, 'click', event => {
    const button = event.target.closest?.('[data-shell-action]');
    const action = actions[button?.dataset.shellAction];
    if (!action || event.defaultPrevented) return;
    event.preventDefault();
    action(button);
  });
  listen(documentRef, 'keydown', event => {
    if (event.key === 'Escape' && shell.dataset.drawer && !documentRef.getElementById('site-search-dialog')?.open) {
      event.preventDefault();
      dismiss();
    }
  });
  for (const query of Object.values(media)) listen(query, 'change', () => dismiss({ restoreFocus: false }));
  const api = Object.freeze({ toggleLeftbar, toggleDrawer, dismiss });
  view.blogShell = api;
  sync();
  return () => {
    cleanups.reverse().forEach(cleanup => cleanup());
    dismiss({ restoreFocus: false });
    if (view.blogShell === api) delete view.blogShell;
  };
}
