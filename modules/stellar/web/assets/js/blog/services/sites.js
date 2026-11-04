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
(function () {
  function escapeHTML(value) {
    return String(value ?? "").replace(/[&<>"']/g, function (character) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      }[character];
    });
  }

  const els = document.getElementsByClassName("ds-sites");
  for (var i = 0; i < els.length; i++) {
    const el = els[i];
    const api = el.dataset.api;
    if (api == null) {
      continue;
    }
    const default_avatar = def.avatar;
    const default_cover = def.cover;
    // layout
    utils.request(
      el,
      api,
      async (resp) => {
        const data = await resp.json();
        for (let item of data.content) {
          var cell = `<div class="grid-cell site-card">`;
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${item.url}">`;
          cell += `<img src="${item.cover || item.snapshot || item.screenshot}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_cover}\';"/>`;
          cell += `<div class="info">`;
          cell += `<img class="siteinfo-icon" src="${item.icon || item.avatar || default_avatar}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_avatar}\';"/>`;
          cell += `<span class="title">${item.title}</span>`;
          cell += `<span class="desc">${item.description || item.url}</span>`;
          cell += `</div>`;
          cell += `<div class="labels">`;
          for (let label of item.labels) {
            if (label.lightness > 75) {
              cell += `<div class="label" style="background:#${label.color};color:hsla(${label.hue}, ${label.saturation}%, 20%, 1);">${label.name}</div>`;
            } else if (label.saturation > 90 && label.lightness > 40) {
              cell += `<div class="label" style="background:#${label.color};color:hsla(${label.hue}, 50%, 20%, 1);">${label.name}</div>`;
            } else {
              cell += `<div class="label" style="background:#${label.color};color:white">${label.name}</div>`;
            }
          }
          cell += `</div>`;
          cell += `</a>`;
          cell += `</div>`;
          utils.dom(el).find(".grid-box").append(cell);
        }
        window.wrapLazyloadImages(el);
      },
      undefined,
      { service: 'sites' },
    );
  }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
