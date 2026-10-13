// 仅在 sandbox allow-scripts 的 Gist 子文档中加载；官方document.write脚本先执行。
(function () {
  'use strict';
  if (window.parent === window) return;
  let frame = null, previous = 0, destroyed = false;
  function measure() {
    frame = null;
    if (destroyed) return;
    const token = new URLSearchParams(window.location.hash.slice(1)).get('stellar-gist-token');
    const height = Math.ceil(document.body.getBoundingClientRect().height);
    if (token && height > 0 && Number.isSafeInteger(height) && height !== previous) {
      previous = height;
      window.parent.postMessage({ type: 'stellar:gist:resize', height, token }, '*');
    }
  }
  function schedule() { if (!destroyed && frame === null) frame = window.requestAnimationFrame(measure); }
  function onHashChange() { previous = 0; schedule(); }
  const observer = new ResizeObserver(schedule);
  observer.observe(document.body);
  function cleanup() {
    if (destroyed) return;
    destroyed = true;
    observer.disconnect();
    if (frame !== null) window.cancelAnimationFrame(frame);
    frame = null;
    window.removeEventListener('load', schedule);
    window.removeEventListener('hashchange', onHashChange);
    window.removeEventListener('pagehide', cleanup);
  }
  window.addEventListener('load', schedule);
  window.addEventListener('hashchange', onHashChange);
  window.addEventListener('pagehide', cleanup);
  schedule();
})();
