// 新 Hero 的唯一版本消费者。官方 ghinfo 不接管 data-blog-service 子树。
(function () {
  const loaded = new WeakSet();
  document.currentScript.stellarMount = function (root, context) {
    const scope = window.blogRuntime.createScope(root, context);
    const elements = [...root.querySelectorAll('[data-blog-service="wiki-release"]')];
    if (root.matches?.('[data-blog-service="wiki-release"]')) elements.unshift(root);
    for (const el of elements) {
      if (loaded.has(el)) continue;
      const api = el.dataset.api;
      if (!api) continue;
      scope.clientRequest(api).then(async response => {
        if (!response.ok) throw new Error('Wiki release request failed: ' + response.status);
        const data = await response.json();
        if (!scope.active()) return;
        const values = data.content || data;
        const index = Number(el.getAttribute('index') || 0);
        const item = Array.isArray(values) ? values[index] : values;
        const tag = item?.tag_name || item?.name;
        const value = el.querySelector('.wiki-cover-release-value');
        if (!tag || !value) { el.remove(); return; }
        const target = item.html_url || (el.dataset.repo && 'https://github.com/' + el.dataset.repo.split('/').map(encodeURIComponent).join('/') + '/tree/' + encodeURIComponent(tag));
        let url;
        try { url = new URL(target); } catch { el.remove(); return; }
        if (!['https:', 'http:'].includes(url.protocol)) { el.remove(); return; }
        value.textContent = el.dataset.projectName + ' ' + tag;
        el.href = url.href;
        el.classList.remove('is-loading');
        el.classList.add('loaded');
        loaded.add(el);
      }).catch(error => {
        if (!scope.active()) return;
        el.remove();
        context.reportError(error);
      });
    }
    return scope.cleanup;
  };
})();
