document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;
  const color = utils.safeColor;

  const els = root.getElementsByClassName('ds-friends_and_posts');
    for (var i = 0; i < els.length; i++) {
      const el = els[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      const default_avatar = def.avatar;
      // 宿主友链页开关由本请求的 SSR 快照提供；省略时沿用官方展示。
      const showPosts = el.dataset.showPosts !== 'false';
      // 本站 API 已由服务端缓存聚合结果，避免浏览器旧缓存把后台更新延迟一天。
      let localFriends = false;
      try {
        const apiURL = new URL(api, document.baseURI);
        const siteURL = new URL(context.legacy?.ctx?.root || '/', document.baseURI);
        const siteRoot = siteURL.pathname.replace(/\/?$/, '/');
        localFriends = apiURL.origin === siteURL.origin &&
          [siteRoot + 'api/friends', siteRoot + 'api/friends-aggregated'].includes(apiURL.pathname);
      } catch (error) {
        // 无效地址交给原有的逐组件请求错误处理，不能阻断后续组件挂载。
        void error;
      }
      // layout
      utils.request(el, api, async resp => {
        const data = await resp.json();
        const cells = [];
        const unavailableCells = [];
        const requestedLimit = Number.parseInt(el.dataset.limit || '', 10);
        const items = data.content || data;
        const visibleItems = Number.isInteger(requestedLimit) && requestedLimit > 0
          ? items.slice(0, requestedLimit)
          : items;
        for (let item of visibleItems) {
          const unavailable = item.unavailable === true;
          var cell = `<div class="grid-cell user-post-card${unavailable ? ' unavailable-card' : ''}">`;
          cell += `<div class="avatar-box">`;
          cell += unavailable
            ? `<div class="card-link" aria-disabled="true">`
            : `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${url(item.html_url || item.url, 'href')}">`;
          cell += `<img src="${url(item.avatar_url || item.avatar || item.appicon || item.icon || default_avatar, 'src')}" data-fallback-src="${url(default_avatar)}"/>`;
          cell += `<span class="title">${text(item.title || item.login)}</span>`;
          cell += unavailable ? `</div>` : `</a>`;
          cell += `<div class="labels">`;
          for (let label of (item.labels || [])) {
            if (label.lightness > 75) {
              cell += `<div class="label" style="background:#${color(label.color)};color:hsla(${Number(label.hue) || 0}, ${Number(label.saturation) || 0}%, 20%, 1);">${text(label.name)}</div>`;
            } else if (label.saturation > 90 && label.lightness > 40) {
              cell += `<div class="label" style="background:#${color(label.color)};color:hsla(${Number(label.hue) || 0}, 50%, 20%, 1);">${text(label.name)}</div>`;
            } else {
              cell += `<div class="label" style="background:#${color(label.color)};color:white">${text(label.name)}</div>`;
            }
          }
          cell += `</div>`;
          cell += `</div>`;
          cell += `<div class="post-panel">`;
          if (item.description) {
            cell += `<div class="desc">${text(item.description || item.issue_number || '')}</div>`;
          } else {
            cell += `<div class="desc">#${text(item.issue_number)}</div>`;
          }
          cell += `<div class="posts">`;
          if (showPosts && !unavailable) {
          if (item.posts?.length > 0) {
            for (let post of item.posts) {
              cell += `<a class="post-link" target="_blank" rel="external nofollow noopener noreferrer" href="${url(post.link, 'href')}">`;
              cell += `<span class="title">${text(post.title)}</span>`;
              cell += `<span class="date">${text(post.published)}</span>`;
              cell += `</a>`;
            }
          } else if (item.feed?.length > 0) {
            cell += `<span class="no-post">动态暂不可用</span>`;
          }
          }
          cell += `</div>`;
          cell += `</div>`;
          cell += `</div>`;
          (unavailable ? unavailableCells : cells).push(cell);
        }
        if (cells.length) utils.dom(el).find('.grid-box').append(cells.join(''));
        if (unavailableCells.length) {
          utils.dom(el).append(`<div class="unavailable-friends"><div class="unavailable-title">失联友链</div><div class="grid-box">${unavailableCells.join('')}</div></div>`);
        }
        window.wrapLazyloadImages(el);
      }, undefined, localFriends ? { cache: 'no-store' } : {});
    }

};
