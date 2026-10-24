// opaque-origin 子文档的尺寸只归当前挂载代所有，离页取消加载并销毁其脚本。
const sources = new WeakMap();
export function mount(root, context) {
  if (context.signal.aborted) return () => {};
  const frames = new Map();
  const documentRef = root.ownerDocument || root;
  const view = documentRef.defaultView || window;
  // 初次消息可能早于 region 挂载；先监听，再通过 load 请求当前代的尺寸。
  view.addEventListener('message', resize);
  context.signal.addEventListener('abort', cleanup, { once: true });
  context.onCleanup?.(cleanup);
  for (const frame of root.querySelectorAll('iframe[data-stellar-gist]')) {
    if (!sources.has(frame)) sources.set(frame, frame.getAttribute('src'));
    const source = sources.get(frame);
    if (!source) continue;
    const bytes = new Uint8Array(16);
    view.crypto.getRandomValues(bytes);
    const token = [...bytes].map(byte => byte.toString(16).padStart(2, '0')).join('');
    const url = new URL(source, documentRef.baseURI);
    url.hash = new URLSearchParams({ 'stellar-gist-token': token }).toString();
    const measure = () => {
      if (!context.signal.aborted && frame.isConnected) {
        frame.contentWindow.postMessage({ type: 'stellar:gist:measure', token }, '*');
      }
    };
    frames.set(frame, { token, measure });
    frame.addEventListener('load', measure);
    frame.setAttribute('src', url.href);
  }
  function resize(event) {
    if (context.signal.aborted || event.origin !== 'null' || event.data?.type !== 'stellar:gist:resize') return;
    const height = event.data.height;
    if (!Number.isSafeInteger(height) || height < 1 || height > 10000000) return;
    for (const [frame, { token }] of frames) {
      if (event.source === frame.contentWindow && event.data.token === token && frame.isConnected) {
        frame.style.height = `${height}px`;
        return;
      }
    }
  }
  function cleanup() {
    view.removeEventListener('message', resize);
    context.signal.removeEventListener('abort', cleanup);
    for (const [frame, { measure }] of frames) {
      frame.removeEventListener('load', measure);
      frame.setAttribute('src', 'about:blank');
    }
    frames.clear();
  }
  if (context.signal.aborted) { cleanup(); return cleanup; }
  return cleanup;
}
