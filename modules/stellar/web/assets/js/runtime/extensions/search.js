const { searchCacheKey, clearSearchStorage } = await import(`../request-cache.js${new URL(import.meta.url).search}`);

export async function mount(root, context) {
  const config = context.extension.config;
  const assets = context.assets;
  if (config.provider === 'local') {
    await assets.script(config.assets.provider);
  }
  await assets.script(config.assets.shortcut);
  context.signal?.throwIfAborted();
  const providerCleanup = window.stellarLocalSearch?.mount?.(root, { key: searchCacheKey, clear: clearSearchStorage });
  const shortcutCleanup = window.stellarSearchShortcut?.mount?.(root);
  return () => {
    shortcutCleanup?.();
    providerCleanup?.();
  };
}
