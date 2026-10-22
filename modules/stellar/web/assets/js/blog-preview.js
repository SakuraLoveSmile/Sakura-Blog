// 隔离预览只通过带代次 token 的消息交换尺寸和滚动；不读取父站 DOM。
(() => {
  'use strict';
  const token = document.documentElement.dataset.stellarPreviewToken;
  if (!token || window.parent === window) return;
  let frame = 0;
  let pendingScroll = false;
  const send = (type, value) => window.parent.postMessage({type, token, ...value}, '*');
  const measure = () => {
    frame = 0;
    const content = document.querySelector('[data-stellar-preview-content]') || document.querySelector('article.md-text') || document.body;
    send('component-preview-resize', {height: Math.ceil(content.getBoundingClientRect().height)});
  };
  const schedule = () => { if (!frame) frame = requestAnimationFrame(measure); };
  const observer = new ResizeObserver(schedule);
  observer.observe(document.body);
  document.addEventListener('load', schedule, true);
  const onScroll = () => {
    if (pendingScroll) return;
    pendingScroll = true;
    requestAnimationFrame(() => {
      pendingScroll = false;
      const target = document.scrollingElement;
      if (!target) return;
      const range = target.scrollHeight - target.clientHeight;
      send('stellar-preview-scroll', {ratio: range > 0 ? target.scrollTop / range : 0});
    });
  };
  const onMessage = (event) => {
    const data = event.data;
    if (event.source !== window.parent || !data || data.token !== token || data.type !== 'stellar-preview-scroll-to') return;
    if (typeof data.ratio !== 'number' || !Number.isFinite(data.ratio)) return;
    const target = document.scrollingElement;
    if (target) target.scrollTop = Math.max(0, Math.min(1, data.ratio)) * Math.max(0, target.scrollHeight - target.clientHeight);
  };
  window.addEventListener('message', onMessage);
  window.addEventListener('scroll', onScroll, {passive: true});
  window.addEventListener('pagehide', () => {
    observer.disconnect();
    cancelAnimationFrame(frame);
    window.removeEventListener('message', onMessage);
    window.removeEventListener('scroll', onScroll);
    document.removeEventListener('load', schedule, true);
  }, {once: true});
  schedule();
})();
