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
    return String(value ?? '').replace(/[&<>"']/g, function (character) {
      return {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
      }[character];
    });
  }

  const els = document.getElementsByClassName('ds-friends_and_posts');
    for (var i = 0; i < els.length; i++) {
      const el = els[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      const default_avatar = def.avatar;
      // RSS 订阅组件开关：data-show-posts="false" 时只渲染友链卡片、跳过每链文章列表（pages.friends.show_posts）。
      const showPosts = el.dataset.showPosts !== 'false';
      // layout
      utils.request(el, api, async resp => {
        const data = await resp.json();
        const unavailable = [];
        for (let item of (data.content || data)) {
          if (item.unavailable === true) {
            unavailable.push(item);
            continue;
          }
          var cell = `<div class="grid-cell user-post-card">`;
          cell += `<div class="avatar-box">`;
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${escapeHTML(item.html_url || item.url || '#')}">`;;
          cell += `<img src="${escapeHTML(item.avatar_url || item.avatar || item.icon || default_avatar)}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${escapeHTML(default_avatar)}\';"/>`;
          cell += `<span class="title">${escapeHTML(item.title || item.login || '')}</span>`;
          cell += `</a>`;
          cell += `<div class="labels">`;
          for (let label of (item.labels || [])) {
            if (label.lightness > 75) {
              cell += `<div class="label" style="background:#${escapeHTML(label.color)};color:hsla(${label.hue}, ${label.saturation}%, 20%, 1);">${escapeHTML(label.name)}</div>`;
            } else if (label.saturation > 90 && label.lightness > 40) {
              cell += `<div class="label" style="background:#${escapeHTML(label.color)};color:hsla(${label.hue}, 50%, 20%, 1);">${escapeHTML(label.name)}</div>`;
            } else {
              cell += `<div class="label" style="background:#${escapeHTML(label.color)};color:white">${escapeHTML(label.name)}</div>`;
            }
          }
          cell += `</div>`;
          cell += `</div>`;
          // Stellar 的 post-panel 负责描述、文章列表和日期的纵向排版。
          // 缺少它时 .post-link 会退化为普通 inline 内容，RSS 标题和日期会粘连。
          cell += `<div class="post-panel">`;
          if (item.description) {
            cell += `<div class="desc">${escapeHTML(item.description)}</div>`;
          } else {
            cell += `<div class="desc">${escapeHTML(item.issue_number ? '#' + item.issue_number : '')}</div>`;
          }
          cell += `<div class="posts">`;
          if (showPosts) {
          if (item.posts?.length > 0) {
            for (let post of item.posts) {
              cell += `<a class="post-link" target="_blank" rel="external nofollow noopener noreferrer" href="${escapeHTML(post.link || '#')}">`;
              cell += `<span class="title">${escapeHTML(post.title || '')}</span>`;
              cell += `<span class="date">${escapeHTML(post.published || '')}</span>`;
              cell += `</a>`;
            }
          } else {
            cell += `<span class="no-post">${item.feed?.length > 0 ? 'RSS 解析失败' : '未设置 RSS 链接'}</span>`;
          }
          }
          cell += `</div>`;
          cell += `</div>`;
          cell += `</div>`;
          utils.dom(el).find('.grid-box').append(cell);
        }
        if (unavailable.length > 0) {
          let section = `<div class="unavailable-friends"><div class="unavailable-title">失联友链</div><div class="grid-box">`;
          for (let item of unavailable) {
            section += `<div class="grid-cell user-post-card unavailable-card">`;
            section += `<div class="avatar-box">`;
            section += `<div class="card-link unavailable-card" aria-disabled="true">`;
            section += `<img src="${escapeHTML(item.avatar_url || item.avatar || item.icon || default_avatar)}"/>`;
            section += `<span class="title">${escapeHTML(item.title || item.login || '')}</span>`;
            section += `</div>`;
            section += `<div class="labels">`;
            for (let label of (item.labels || [])) {
              if (label.lightness > 75) {
                section += `<div class="label" style="background:#${escapeHTML(label.color)};color:hsla(${label.hue}, ${label.saturation}%, 20%, 1);">${escapeHTML(label.name)}</div>`;
              } else if (label.saturation > 90 && label.lightness > 40) {
                section += `<div class="label" style="background:#${escapeHTML(label.color)};color:hsla(${label.hue}, 50%, 20%, 1);">${escapeHTML(label.name)}</div>`;
              } else {
                section += `<div class="label" style="background:#${escapeHTML(label.color)};color:white">${escapeHTML(label.name)}</div>`;
              }
            }
            section += `</div></div>`;
            section += `<div class="post-panel"><div class="desc">${escapeHTML(item.description || (item.issue_number ? '#' + item.issue_number : ''))}</div><div class="posts"></div></div>`;
            section += `</div>`;
          }
          section += `</div></div>`;
          utils.dom(el).append(section);
        }
        window.wrapLazyloadImages(el);
      }, undefined, { service: 'friends_and_posts' });
    }
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
