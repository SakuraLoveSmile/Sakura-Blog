document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;

  const els = root.getElementsByClassName('ds-giscus');
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
          const limit = el.getAttribute('limit');
          const cells = [];
          data.forEach((item, i) => {
            if (limit && i >= limit) {
              return;
            }
            const comment = item.body.length > 50 ? item.body.substring(0, 50) + '...' : item.body;
            var cell = '<div class="timenode" index="' + i + '">';
            cell += '<div class="header">';
            cell += '<div class="user-info">';
            cell += '<img src="' + url(item.author.avatarUrl || default_avatar) + '" data-fallback-src="' + url(default_avatar) + '">';
            cell += '<span>' + text(item.author.login) + '</span>';
            cell += '</div>';
            cell += '<span>' + new Date(item.createdAt).toLocaleString() + '</span>';
            cell += '</div>';
            cell += '<a class="body" href="' + url(item.url, 'href') + '" target="_blank" rel="external nofollow noopener noreferrer">';
            cell += text(comment);
            cell += '</a>';
            cell += '</div>';
            cells.push(cell);
          });
          if (cells.length) utils.dom(el).append(cells.join(''));
        });
      }

};
