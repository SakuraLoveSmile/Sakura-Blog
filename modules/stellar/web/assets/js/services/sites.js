document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const text = utils.escapeHTML;
  const url = utils.safeURL;
  const color = utils.safeColor;

  const els = root.getElementsByClassName('ds-sites');
    for (var i = 0; i < els.length; i++) {
      const el = els[i];
      const api = el.dataset.api;
      if (api == null) {
        continue;
      }
      const default_avatar = def.avatar;
      const default_cover = def.cover;
      // layout
      utils.request(el, api, async resp => {
        const data = await resp.json();
        const cells = [];
        for (let item of (data.content || data)) {
          const siteinfoApi = !item.appicon && !item.icon && !item.avatar && ctx.services.siteinfo?.api
            ? ctx.services.siteinfo.api.replace('{href}', encodeURIComponent(item.url))
            : '';
          var cell = `<div class="grid-cell site-card">`;
          cell += `<a class="card-link"${siteinfoApi ? ` data-siteinfo-api="${url(siteinfoApi)}"` : ''} target="_blank" rel="external nofollow noopener noreferrer" href="${url(item.url, 'href')}">`;
          cell += `<img src="${url(item.cover || item.snapshot || item.screenshot, 'src')}" data-fallback-src="${url(default_cover)}"/>`;
          cell += `<div class="info">`;
          cell += `<img class="siteinfo-icon" src="${url(item.appicon || item.icon || item.avatar || default_avatar, 'src')}" data-fallback-src="${url(default_avatar)}"/>`;
          cell += `<span class="title">${text(item.title)}</span>`;
          cell += `<span class="desc">${text(item.description || item.url)}</span>`;
          cell += `</div>`;
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
          cell += `</a>`;
          cell += `</div>`;
          cells.push(cell);
        }
        if (cells.length) utils.dom(el).find('.grid-box').append(cells.join(''));
        window.wrapLazyloadImages(el);
        window.dispatchEvent(new CustomEvent('stellar:sites-ready', { detail: { target: el } }));
      });
    }

};
