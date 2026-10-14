const SELECTOR = '.wiki-cover-background-video';
const owners = new WeakMap();

// 封面由独立region持有；同集合partial导航保留媒体，离页取消立即暂停。
export function mount(root, context = {}) {
  const videos = [...root.querySelectorAll(SELECTOR)];
  if (root.matches?.(SELECTOR)) videos.unshift(root);
  const owner = {};
  let disposed = false;
  for (const video of videos) owners.set(video, owner);

  function cleanup() {
    if (disposed) return;
    disposed = true;
    context.signal?.removeEventListener('abort', cleanup);
    for (const video of videos) {
      if (owners.get(video) !== owner) continue;
      video.pause();
      owners.delete(video);
    }
  }
  function failed(error) {
    if (!disposed && error?.name !== 'AbortError' && error?.name !== 'NotAllowedError') {
      context.reportError?.(error);
    }
  }
  context.signal?.addEventListener('abort', cleanup, { once: true });
  context.onCleanup?.(cleanup);
  if (context.signal?.aborted) cleanup();
  for (const video of videos) {
    if (disposed || !video.autoplay) continue;
    try {
      Promise.resolve(video.play()).then(() => {
        // 已取消的play可能迟到成功；新挂载代正在播放时仍归新代所有。
        if (disposed && !owners.has(video)) video.pause();
      }, failed);
    } catch (error) {
      failed(error);
    }
  }
  return cleanup;
}
