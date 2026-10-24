// 本地服务在独立的 DOM 子树启用；不得同时让 stock services 接管同一树。
export async function mount(root, context) {
  const services = context.extension.config.services || {};
  await context.assets.script('/assets/js/blog/lifecycle.js');
  const cleanups = [];
  context.onCleanup(() => cleanups.splice(0).reverse().forEach(cleanup => cleanup()));
  for (const service of Object.values(services)) {
    if (!service || !service.module || !service.selector) throw new TypeError('Blog service requires module and selector');
    const elements = [...root.querySelectorAll(service.selector)];
    if (root.matches?.(service.selector)) elements.unshift(root);
    if (!elements.length) continue;
    if (service.marked) await context.assets.script(service.marked);
    const script = await context.assets.script(service.module);
    context.signal.throwIfAborted();
    if (typeof script.stellarMount !== 'function') throw new TypeError('Blog service has no regional mount');
    for (const element of elements) {
      const cleanup = script.stellarMount(element, { ...context, options: service.options || {} });
      if (typeof cleanup === 'function') cleanups.push(cleanup);
    }
  }
}
