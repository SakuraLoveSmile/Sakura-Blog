const { searchCacheKey } = await import(`../request-cache.js${new URL(import.meta.url).search}`);
const roots = new WeakMap();
async function initialize(documentRef, context) {
  await context.assets.script('/assets/js/blog/search/local-search.js');
  await context.assets.script('/assets/js/search/shortcut.js');
  context.signal.throwIfAborted();
  const view = documentRef.defaultView || window;
  const search = view.blogLocalSearch;
  const providerCleanup = search.mount(documentRef, { key: searchCacheKey });
  const shortcutCleanup = view.stellarSearchShortcut.mount(documentRef);
  // stock settings继续拥有缓存UI；先清空Blog内存，随后其handler清理相同v6命名空间。
  const clearMemory = event => {
    const action = event.target.closest?.('[data-cache-action]')?.dataset.cacheAction;
    if (action === 'search' || action === 'all') search.clearMemory();
  };
  documentRef.addEventListener('click', clearMemory, { capture: true });
  let cleaned = false;
  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    documentRef.removeEventListener('click', clearMemory, { capture: true });
    shortcutCleanup(); providerCleanup();
    roots.delete(documentRef);
  };
  context.onCleanup(cleanup);
  return cleanup;
}
export function mount(documentRef, context) {
  if (roots.has(documentRef)) return roots.get(documentRef);
  const pending = initialize(documentRef, context);
  roots.set(documentRef, pending);
  pending.catch(() => { if (roots.get(documentRef) === pending) roots.delete(documentRef); });
  return pending;
}
