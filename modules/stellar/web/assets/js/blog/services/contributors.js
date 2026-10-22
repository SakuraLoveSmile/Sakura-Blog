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
  function parseGithubFileContributors(data) {
      // 去重贡献者（按 login）
      const contributorsMap = new Map();

      for (const commit of data) {
        const author = commit.author;
        if (author) {
          const login = author.login;
          if (!contributorsMap.has(login)) {
            contributorsMap.set(login, {
              login: login,
              avatar_url: author.avatar_url,
              html_url: author.html_url,
              count: 1
            });
          } else {
            contributorsMap.get(login).count++;
          }
        }
      }
      // 转为数组并按提交次数排序（降序）
      const sortedContributors = Array.from(contributorsMap.values())
        .sort((a, b) => b.count - a.count);
      return sortedContributors;
    }

    const els = document.getElementsByClassName('ds-contributors');
    for (var i = 0; i < els.length; i++) {
      const el = els[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      const default_avatar = def.avatar;
      // layout
      utils.request(el, api, async resp => {
        const data = await resp.json();
        const list = parseGithubFileContributors(data);
        for (let item of list) {
          var cell = `<div class="grid-cell user-card">`;
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${item.html_url || item.url}">`;;
          cell += `<img src="${item.avatar_url || item.avatar || item.icon || default_avatar}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_avatar}\';"/>`;
          cell += `<div class="name image-meta">`;
          cell += `<span class="image-caption">${item.title || item.login}</span>`;
          cell += `</div>`;
          cell += `</a>`;
          cell += `</div>`;
          utils.dom(el).find('.grid-box').append(cell);
        }
        window.wrapLazyloadImages(el);
      });
    }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
