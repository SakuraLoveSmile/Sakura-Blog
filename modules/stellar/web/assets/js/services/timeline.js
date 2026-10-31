document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;
  const color = utils.safeColor;

  const reactions = {
      '+1': '👍',
      '-1': '👎', 
      'laugh': '😀', 
      'hooray': '🎉', 
      'confused': '😕', 
      'heart': '❤️', 
      'rocket': '🚀', 
      'eyes': '👀'
    }
    const timelines = root.getElementsByClassName('ds-timeline');
    for (var i = 0; i < timelines.length; i++) {
      const el = timelines[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      // layout
      utils.request(el, api, async resp => {
        const data = await resp.json();
        const cells = [];
        const query = new URL(api, el.baseURI).search;
        const arr = data.content || data;
        var users = [];
        const filter = el.getAttribute('user');
        if (filter && filter.length > 0) {
          users = filter.split(",");
        }
        var hide = [];
        const hideStr = el.getAttribute('hide');
        if (hideStr && hideStr.length > 0) {
          hide = hideStr.split(",");
        }
        arr.forEach((item, i) => {
          if (item.user && item.user.login && users.length > 0) {
            if (!users.includes(item.user.login)) {
              return;
            }
          }
          var cell = '<div class="timenode" index="' + i + '">';
          cell += '<div class="header">';
          if (!users.length && item.user && !hide.includes('user')) {
            cell += '<a class="user-info" href="' + url(item.user.html_url, 'href') + '" target="_blank" rel="external nofollow noopener noreferrer">';
            cell += '<img src="' + url(item.user.avatar_url) + '">';
            cell += '<span>' + text(item.user.login) + '</span>';
            cell += '</a>';
          }
          let date = new Date(item.created_at);
          cell += '<span>' + date.toLocaleString() + '</span>';
          cell += '</div>';
          cell += '<div class="body">';
          if (!hide.includes('title')) {
            cell += '<p class="title">';
            cell += '<a href="' + url(item.html_url, 'href') + '" target="_blank" rel="external nofollow noopener noreferrer">';
            cell += text(item.title || item.name || item.tag_name);
            cell += '</a>';
            cell += '</p>';
          }
          
          cell += marked.parse(item.body || '');
          if (!hide.includes('footer')) {
            cell += '<div class="footer">';
            cell += '<div class="flex left">';
            if (item.labels) {
              item.labels.forEach((label, i) => {
                if (!query || !query.includes(encodeURI(label.name))) {
                  cell += '<div class="item label ' + text(label.name) + '" style="background:#' + color(label.color) + '18;border-color:#' + color(label.color) + '36">';
                  cell += '<span>' + text(label.name) + '</span>';
                  cell += '</div>';
                }
              });
            } else if (item.zipball_url) {
              cell += '<a class="item download" href="' + url(item.zipball_url, 'href') + '" target="_blank" rel="external nofollow noopener noreferrer">';
              cell += '<span>📦 ' + text(item.tag_name) + '.zip</span>';
              cell += '</a>';
            }
            cell += '</div>';
            cell += '<div class="flex right">';
            if (item.reactions?.total_count > 0) {
              for (let key of Object.keys(reactions)) {
                let num = item.reactions[key];
                if (num > 0) {
                  cell += '<div class="item reaction ' + key + '">';
                  cell += '<span>' + reactions[key] + ' ' + text(item.reactions[key]) + '</span>';
                  cell += '</div>';
                }
              }
            }
            if (item.comments != null) {
              cell += '<a class="item comments last" href="' + url(item.html_url, 'href') + '#issuecomment-new" target="_blank" rel="external nofollow noopener noreferrer">';
              cell += '<span>' + ctx.icons['default:to-comment'] + ' ' + text(item.comments || 0) + '</span>';
              cell += '</a>';
            }
            
            cell += '</div>';
            cell += '</div>';
          }
          
          cell += '</div>';
          cell += '</div>';
          cells.push(cell);
        });
        if (cells.length) utils.dom(el).append(cells.join(''));
        window.wrapLazyloadImages(el);
      });
    }

};
