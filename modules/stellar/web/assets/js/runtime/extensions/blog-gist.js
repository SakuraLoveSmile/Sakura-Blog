// 官方 Gist 在 opaque-origin iframe 中运行；父页面只接收对应 frame 的尺寸。
export function mount(root, context) {
  const frames = new Set(root.querySelectorAll('iframe[data-stellar-gist]'));
  const view = (root.ownerDocument || root).defaultView || window;
  function resize(event) {
    if (event.origin !== 'null' || event.data?.type !== 'stellar:gist:resize') return;
    const height = event.data.height;
    if (!Number.isSafeInteger(height) || height < 1 || height > 10000000) return;
    for (const frame of frames) {
      if (event.source === frame.contentWindow && frame.isConnected) {
        frame.style.height = `${height}px`;
        return;
      }
    }
  }
  function cleanup() {
    view.removeEventListener('message', resize);
    context.signal?.removeEventListener('abort', cleanup);
    frames.clear();
  }
  if (context.signal?.aborted) return cleanup;
  view.addEventListener('message', resize);
  context.signal?.addEventListener('abort', cleanup, { once: true });
  context.onCleanup?.(cleanup);
  return cleanup;
}
