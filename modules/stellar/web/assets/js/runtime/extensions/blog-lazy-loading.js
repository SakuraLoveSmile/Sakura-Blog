// 保留官方懒加载实现；额外处理 Blog 的 data-fallback 和资源加载失败。
const official = await import(`./lazy-loading.js${new URL(import.meta.url).search}`);
export async function mount(root, context) {
  const view = root.defaultView || window;
  const attempts = new WeakMap();
  let failed = false;
  let fallbackWrap = null;
  let officialCleanup = null;
  const alive = () => !context.signal.aborted;
  const images = container => {
    const result = [...container.querySelectorAll('img.lazy[data-src]')];
    if (container.matches?.('img.lazy[data-src]')) result.unshift(container);
    return result;
  };
  function removeSpinner(image) {
    (image.closest?.('.lazy-box') || image.parentElement)?.querySelector('.lazy-icon')?.remove();
  }
  function restore(container) {
    if (!alive()) return;
    images(container).forEach(image => {
      // 保留已经加载中的src，只有缺失时补回原图地址。
      if (!image.getAttribute('src')) image.setAttribute('src', image.getAttribute('data-src'));
    });
  }
  function onError(event) {
    const image = event.target;
    if (!alive() || image?.tagName !== 'IMG' || !root.documentElement.contains(image)) return;
    const fallback = image.getAttribute('data-fallback');
    const src = image.getAttribute('src');
    if (fallback && fallback !== src && attempts.get(image) !== fallback) {
      attempts.set(image, fallback);
      image.classList.remove('error');
      // 防止LazyLoad的下一次扫描把失败原图重新覆盖回去。
      image.setAttribute('data-src', fallback);
      image.setAttribute('src', fallback);
      return;
    }
    image.classList.add('error');
    removeSpinner(image);
  }
  function onLoad(event) {
    const image = event.target;
    if (!alive() || image?.tagName !== 'IMG' || !root.documentElement.contains(image)) return;
    image.classList.remove('error'); image.classList.add('loaded'); removeSpinner(image);
  }
  root.addEventListener('error', onError, { capture: true });
  root.addEventListener('load', onLoad, { capture: true });
  const observer = new view.MutationObserver(records => {
    if (!failed || !alive()) return;
    records.forEach(record => [...record.addedNodes].forEach(node => { if (node.nodeType === 1) restore(node); }));
  });
  observer.observe(root.documentElement, { childList: true, subtree: true });
  const cleanup = () => {
    observer.disconnect();
    root.removeEventListener('error', onError, { capture: true });
    root.removeEventListener('load', onLoad, { capture: true });
    officialCleanup?.();
    if (fallbackWrap && view.wrapLazyloadImages === fallbackWrap) delete view.wrapLazyloadImages;
  };
  context.onCleanup(cleanup);
  try {
    officialCleanup = await official.mount(root, context);
    context.signal.throwIfAborted();
  } catch (error) {
    if (context.signal.aborted) { cleanup(); throw error; }
    failed = true;
    fallbackWrap = container => {
      const target = typeof container === 'string' ? root.querySelector(container) : container;
      if (target) restore(target);
    };
    view.wrapLazyloadImages = fallbackWrap;
    restore(root);
    context.reportError(error);
  }
  return cleanup;
}
