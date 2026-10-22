(function () {
  const loaded = new WeakSet();
document.currentScript.stellarMount = function (root, context) {
  const scope = window.blogRuntime.createScope(root, context);
  const document = scope.document;
  const fetch = scope.request;
  const utils = Object.create(window.utils);
  utils.request = (element, url, callback, failure, options = {}) => {
    if (element && loaded.has(element)) return Promise.resolve();
    return scope.clientRequest(url, { ...options, onNetworkStart: () => window.utils.onLoading?.(element) })
    .then(async response => { if (scope.active()) { window.utils.onLoadSuccess?.(element); await callback(response); if (element && scope.active()) loaded.add(element); } })
    .catch(error => { if (scope.active()) { window.utils.onLoadFailure?.(element); failure?.(error); context.reportError(error); } });
  };
  const match = selector => {
    const result = [...root.querySelectorAll(selector)];
    if (root.matches?.(selector)) result.unshift(root);
    return result;
  };
  const scopedDocument = new Proxy(document, { get(target, key) {
    if (key === 'querySelectorAll') return match;
    if (key === 'getElementsByClassName') return name => match('.' + name);
    return target[key];
  } });
  (function (document) {
// siteinfo 图标兜底：sites.js 渲染卡片后，对未配 icon/avatar 的卡片用 siteinfo API 补图标。
// 监听 stellar:sites-ready（sites.js 渲染完成后派发），对带 data-siteinfo-api 的卡片请求补图。
(function () {
  function setSiteCardIcon(nodes) {
    nodes = 'forEach' in (nodes || {}) ? nodes : document.querySelectorAll('.site-card .card-link[data-siteinfo-api]');
    nodes.forEach(function (el) {
      if (el.nodeType !== 1) return;
      var api = el.dataset.siteinfoApi;
      if (api == null) return;
      utils.request(null, api, function (response) {
        return response.json().then(function (data) {
          if (data.icon && data.icon.length > 0) {
            var icon = el.querySelector('.siteinfo-icon');
            if (icon) {
              icon.src = data.icon;
              icon.setAttribute('data-src', data.icon);
            }
          }
        }).catch(function (error) {
          console.error(error);
        });
      }, undefined, { service: 'siteinfo' });
    });
  }

  function refreshSiteCardIcons() {
    setSiteCardIcon();
  }

  // sites.js and siteinfo.js are loaded independently by the service scanner.
  // Handle both load orders so the custom API is not missed when sites.js
  // finishes before this listener is registered.
  scope.listen(window, 'stellar:sites-ready', refreshSiteCardIcons);
  if (document.readyState !== 'loading') {
    refreshSiteCardIcons();
  } else {
    scope.listen(document, 'DOMContentLoaded', refreshSiteCardIcons);
  }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
