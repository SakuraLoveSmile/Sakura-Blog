// 标题/标签/分类索引沿用官方local配置、v6缓存和快捷键；与stock/Go provider排他。
const { searchCacheKey, clearSearchStorage } = await import(`../request-cache.js${new URL(import.meta.url).search}`);
const roots = new WeakMap();
async function initialize(root, context) {
  const config = context.extension.config;
  if (config.provider !== 'local') throw new TypeError('title search requires local provider');
  await context.assets.script(config.assets.provider);
  await context.assets.script(config.assets.shortcut);
  context.signal.throwIfAborted();
  const provider = window.stellarTitleSearch;
  if (!provider) throw new Error('title search provider was not loaded');
  const providerCleanup = provider.mount(root, { key: searchCacheKey, clear: clearSearchStorage });
  const shortcutCleanup = window.stellarSearchShortcut.mount(root);
  const clearMemory = event => {
    const action = event.target.closest?.('[data-cache-action]')?.dataset.cacheAction;
    if (action === 'search' || action === 'all') provider.clearMemory();
  };
  root.addEventListener('click', clearMemory, { capture: true });
  let cleaned = false;
  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    root.removeEventListener('click', clearMemory, { capture: true });
    shortcutCleanup(); providerCleanup();
    roots.delete(root);
  };
  context.onCleanup(cleanup);
  return cleanup;
}

export function mount(root, context) {
  if (roots.has(root)) return roots.get(root);
  const pending = initialize(root, context);
  roots.set(root, pending);
  pending.catch(() => { if (roots.get(root) === pending) roots.delete(root); });
  return pending;
}
