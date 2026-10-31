document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;
  const color = utils.safeColor;

  const els = root.getElementsByClassName('ds-friends');
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
        const cells = [];
        for (let item of (data.content || data)) {
          var cell = `<div class="grid-cell user-card">`;
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${url(item.html_url || item.url, 'href')}">`;;
          cell += `<img src="${url(item.avatar_url || item.avatar || item.appicon || item.icon || default_avatar, 'src')}" data-fallback-src="${url(default_avatar)}"/>`;
          cell += `<div class="name image-meta">`;
          cell += `<span class="image-caption">${text(item.title || item.login)}</span>`;
          cell += `</div>`;
          if (item.labels && item.labels.length > 0) {
            let label = item.labels[0];
            cell += `<div class="label" style="background:#${color(label.color)};">${text(label.name)}</div>`;
          }
          cell += `</a>`;
          cell += `</div>`;
          cells.push(cell);
        }
        if (cells.length) utils.dom(el).find('.grid-box').append(cells.join(''));
        window.wrapLazyloadImages(el);
      });
    }

};
