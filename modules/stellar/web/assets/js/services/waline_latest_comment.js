document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;

  const els = root.getElementsByClassName('ds-waline');
      for (var i = 0; i < els.length; i++) {
        const el = els[i];
        const limit = parseInt(el.getAttribute('limit')) || 10;
        const apiBase = el.dataset.api;
        if (apiBase == null) {
          continue;
        }
        const api = apiBase + '/comment?type=recent&count=' + limit;
        const default_avatar = def.avatar;
        utils.request(el, api, async resp => {
          const payload = await resp.json();
          // 兼容旧版数组与新版 { data: [...] } 返回结构（#630）
          const data = Array.isArray(payload) ? payload : (payload && Array.isArray(payload.data) ? payload.data : []);
          const cells = [];
          data.forEach((item, i) => {
            var cell = '<div class="timenode" index="' + i + '">';
            cell += '<div class="header">';
            cell += '<div class="user-info">';
            cell += '<img src="' + url(item.avatar || default_avatar) + '" data-fallback-src="' + url(default_avatar) + '">';
            cell += '<span>' + text(item.nick) + '</span>';
            cell += '</div>';
            cell += '<span>' + new Date(item.time).toLocaleString() + '</span>';
            cell += '</div>';
            cell += '<a class="body" href="' + url(item.url, 'href') + '#' + text(item.objectId) + '" target="_blank" rel="external nofollow noopener noreferrer">';
            cell += item.comment.replace(/<a\b[^>]*>(.*?)<\/a>/g, '$1');
            cell += '</a>';
            cell += '</div>';
            cells.push(cell);
          });
          if (cells.length) utils.dom(el).append(cells.join(''));
        });
      }

};
