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
  var els = document.getElementsByClassName('ds-wiki-release');
  for (var i = 0; i < els.length; i++) {
    (function (el) {
      var api = el.dataset.api;
      if (!api) return;
      utils.request(null, api, function (resp) {
        resp.json().then(function (data) {
          if (!data || !data.tag_name) return;
          var tag = el.querySelector('[type=text]#tag_name');
          var link = el.querySelector('[type=link]#html_url');
          if (tag) tag.textContent = data.tag_name;
          if (link && data.html_url) link.href = data.html_url;
          el.classList.remove('is-loading');
          el.classList.add('loaded');
          el.hidden = false;
        }).catch(function () {});
      });
    })(els[i]);
  }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
