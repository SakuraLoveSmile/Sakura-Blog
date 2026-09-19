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
  window.addEventListener('stellar:sites-ready', refreshSiteCardIcons);
  if (document.readyState !== 'loading') {
    refreshSiteCardIcons();
  } else {
    document.addEventListener('DOMContentLoaded', refreshSiteCardIcons);
  }
})();
