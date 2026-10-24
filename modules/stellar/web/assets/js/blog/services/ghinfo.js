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
  const els = document.getElementsByClassName('ds-ghinfo');
    for (var i = 0; i < els.length; i++) {
      const el = els[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      // layout
      utils.request(null, api, async resp => {
        const data = await resp.json();
        function fill(data) {
          for (let key of Object.keys(data)) {
            utils.dom(el).find("[type=text]#" + key).text(data[key]);
            utils.dom(el).find("[type=link]#" + key).attr("href", data[key]);
            utils.dom(el).find("[type=img]#" + key).attr("src", data[key]);
          }
        }
        const idx = el.getAttribute('index');
        if (idx != undefined) {
          const arr = data.content || data;
          if (arr && arr.length > idx) {
            let obj = arr[idx];
            obj['latest-tag-name'] = obj['name'];
            fill(arr[idx]);
          }
        } else {
          fill(data);
        }
        if (el.classList && el.classList.contains('wiki-stars')) {
          el.classList.add('loaded');
        }
      });
    }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
