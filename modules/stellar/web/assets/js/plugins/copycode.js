function createCopyButtons(codeElements, signal) {
  if (signal?.aborted) return () => {};
  let active = true;
  const buttons = [];
  const messages = ctx.copycode;
  const current = () => active && !signal?.aborted;
  function cleanup() {
    if (!active) return;
    active = false;
    signal?.removeEventListener('abort', cleanup);
    buttons.forEach(({ button, copy }) => {
      button.removeEventListener('click', copy);
      button.remove();
    });
    buttons.length = 0;
  }
  signal?.addEventListener('abort', cleanup, { once: true });
  codeElements.forEach((codeElement) => {
    if (codeElement.querySelector('.copy-btn')) return;
    // 创建复制按钮
    const codeCopyBtn = document.createElement('button');
    const copyIcon = ctx.icons['copy:copy'] || '';
    codeCopyBtn.className = 'copy-btn';
    codeCopyBtn.type = 'button';
    codeCopyBtn.innerHTML = copyIcon;
    codeCopyBtn.setAttribute('aria-label', messages.label);
    codeCopyBtn.setAttribute('title', messages.label);
    codeElement.appendChild(codeCopyBtn);

    // 添加点击事件监听
    const copy = async () => {
      if (!current()) return;
      const codeToCopy = codeElement.querySelector('pre')?.innerText || '';
      if (navigator.clipboard) {
        try {
          await navigator.clipboard.writeText(codeToCopy);
          if (current()) hud.toast(messages.copied, messages.toast_ms);
        } catch (error) {
          if (current()) hud.toast(messages.denied, messages.toast_ms);
        }
      } else {
        hud.toast(messages.unsupported, messages.toast_ms);
      }
    };
    codeCopyBtn.addEventListener('click', copy);
    buttons.push({ button: codeCopyBtn, copy });
  });
  return cleanup;
}

if (typeof window.codeElements !== 'undefined') {
  createCopyButtons(window.codeElements);
}
