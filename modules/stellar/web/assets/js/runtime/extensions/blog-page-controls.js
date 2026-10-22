// 官方页面控件完整挂载；Blog 评论有独立目标，不覆盖官方 #comments。
const official = await import(`./page-controls.js${new URL(import.meta.url).search}`);
export async function mount(root, context) {
  const documentRef = root.ownerDocument || root;
  const view = documentRef.defaultView || window;
  const selector = '[data-blog-action="scroll-comments"], [data-comments-target="blog-comments"]';
  const onClick = event => {
    const trigger = event.target.closest?.(selector);
    if (!trigger || !root.contains(trigger) || event.defaultPrevented || context.signal.aborted) return;
    const target = documentRef.getElementById('blog-comments');
    if (!target) return;
    event.preventDefault();
    // 捕获期停止遗留inline onclick，避免又跳向官方第三方评论。
    event.stopPropagation();
    view.scrollTo({ top: Math.max(0, target.getBoundingClientRect().top + view.scrollY - 32), behavior: 'smooth' });
    view.stellar?.syncPageShell?.();
  };
  root.addEventListener('click', onClick, { capture: true });
  const cleanup = () => root.removeEventListener('click', onClick, { capture: true });
  context.onCleanup(cleanup);
  await official.mount(root, context);
  context.signal.throwIfAborted();
  return cleanup;
}
