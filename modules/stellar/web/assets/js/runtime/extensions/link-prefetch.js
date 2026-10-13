export async function mount(root, context) {
  await context.assets.script(context.extension.config.asset);
  context.signal?.throwIfAborted();
  const cleanup = window.flyingPages({ delay: 0, ignoreKeywords: [], maxRPS: 5, hoverDelay: 25 });
  context.onCleanup?.(cleanup);
  return cleanup;
}
