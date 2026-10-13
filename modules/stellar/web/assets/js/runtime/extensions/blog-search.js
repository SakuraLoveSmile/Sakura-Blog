const { searchCacheKey } = await import(`../request-cache.js${new URL(import.meta.url).search}`);
export async function mount(documentRef, context) {
  await context.assets.script('/assets/js/blog/search/local-search.js');
  await context.assets.script('/assets/js/search/shortcut.js');
  context.signal.throwIfAborted();
  const view = documentRef.defaultView || window;
  const search = view.blogLocalSearch;
  search.mount({ key: searchCacheKey });
  const shortcutCleanup = view.stellarSearchShortcut.mount(documentRef);
  // stock settings继续拥有缓存UI；先清空Blog内存，随后其handler清理相同v6命名空间。
  const clearMemory = event => {
    const action = event.target.closest?.('[data-cache-action]')?.dataset.cacheAction;
    if (action === 'search' || action === 'all') search.clearMemory();
  };
  documentRef.addEventListener('click', clearMemory, { capture: true });
  return () => {
    documentRef.removeEventListener('click', clearMemory, { capture: true });
    shortcutCleanup(); search.unmount();
  };
}
