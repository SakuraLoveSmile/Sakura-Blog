(function () {
  const els = document.getElementsByClassName('ds-sites');
  // siteinfo API：无 icon/avatar 的卡片用 siteinfo API 补图标（配置了 api 时生效）
  const siteinfoApi = (typeof ctx !== 'undefined' && ctx.services && ctx.services.siteinfo && ctx.services.siteinfo.api) || '';
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
        for (let item of data.content) {
          var cell = `<div class="grid-cell site-card">`;
          var siteinfoAttr = (siteinfoApi && !item.icon && !item.avatar) ? ` data-siteinfo-api="${siteinfoApi.replace(/{href}/, encodeURIComponent(item.url))}"` : '';
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${item.url}"${siteinfoAttr}>`;
          cell += `<img src="${item.cover || item.snapshot || item.screenshot}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_cover}\';"/>`;
          cell += `<div class="info">`;
          cell += `<img class="siteinfo-icon" src="${item.icon || item.avatar || default_avatar}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_avatar}\';"/>`;
          cell += `<span class="title">${item.title}</span>`;
          cell += `<span class="desc">${item.description || item.url}</span>`;
          cell += `</div>`;
          cell += `<div class="labels">`;
          for (let label of item.labels) {
            if (label.lightness > 75) {
              cell += `<div class="label" style="background:#${label.color};color:hsla(${label.hue}, ${label.saturation}%, 20%, 1);">${label.name}</div>`;
            } else if (label.saturation > 90 && label.lightness > 40) {
              cell += `<div class="label" style="background:#${label.color};color:hsla(${label.hue}, 50%, 20%, 1);">${label.name}</div>`;
            } else {
              cell += `<div class="label" style="background:#${label.color};color:white">${label.name}</div>`;
            }
          }
          cell += `</div>`;
          cell += `</a>`;
          cell += `</div>`;
          utils.dom(el).find('.grid-box').append(cell);
        }
        window.wrapLazyloadImages(el);
        // 通知 siteinfo.js 补图标
        window.dispatchEvent(new Event('stellar:sites-ready'));
      });
    }
})();
