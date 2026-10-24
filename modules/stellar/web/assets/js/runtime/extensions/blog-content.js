// Blog 保留的分享、打赏与旧Wiki辅助交互；不修改官方main或strict主题配置。
export async function mount(root, context) {
  await context.assets.script('/assets/js/blog/lifecycle.js');
  context.signal.throwIfAborted();
  const scope = window.blogRuntime.createScope(root, context);
  const config = context.extension.config;
  const footer = root.querySelector('#post-footer, #share');
  const url = footer?.getAttribute('data-url') || location.href;
  const title = footer?.getAttribute('data-title') || document.title;
  const weibo = root.querySelector('#share .share-item.weibo');
  const email = root.querySelector('#share .share-item.email');
  if (weibo) weibo.href = 'https://service.weibo.com/share/share.php?url=' + encodeURIComponent(url) + '&title=' + encodeURIComponent(title);
  if (email) email.href = 'mailto:?subject=' + encodeURIComponent(title) + '&body=' + encodeURIComponent(url);
  function copy(text) {
    if (!navigator.clipboard?.writeText) { window.prompt(config.promptCopy || '复制链接', text); return; }
    navigator.clipboard.writeText(text).then(() => {
      if (scope.active()) window.stellar?.toast?.(config.copied || '已复制', 2500);
    }).catch(() => { if (scope.active()) window.prompt(config.promptCopy || '复制链接', text); });
  }
  scope.listen(root, 'click', event => {
    const share = event.target.closest?.('#share .share-item');
    if (share) {
      const platform = share.getAttribute('data-platform');
      if (platform === 'link') { event.preventDefault(); copy(url); }
      else if (platform === 'wechat') { event.preventDefault(); root.querySelector('#share-qrcode')?.classList.toggle('display'); }
      return;
    }
    const reward = event.target.closest?.('#reward-toggle');
    if (reward) {
      event.preventDefault();
      const codes = root.querySelector('#reward-codes');
      if (!codes) return;
      const expanded = !codes.classList.contains('is-open');
      codes.classList.toggle('is-open', expanded); codes.inert = !expanded;
      reward.setAttribute('aria-expanded', String(expanded));
      return;
    }
    const empty = event.target.closest?.('a[data-wiki-empty]');
    if (empty) { event.preventDefault(); window.stellar?.toast?.(empty.getAttribute('data-wiki-empty'), 2500); return; }
    const button = event.target.closest?.('.wiki-copy[data-copy-text]');
    if (button) { event.preventDefault(); copy(button.getAttribute('data-copy-text') || ''); }
  });
  return scope.cleanup;
}
