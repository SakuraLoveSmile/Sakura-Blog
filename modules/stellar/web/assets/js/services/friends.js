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

  const els = document.getElementsByClassName('ds-friends');
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
        const unavailable = [];
        for (let item of (data.content || data)) {
          if (item.unavailable === true) {
            unavailable.push(item);
            continue;
          }
          var cell = `<div class="grid-cell user-card">`;
          cell += `<a class="card-link" target="_blank" rel="external nofollow noopener noreferrer" href="${item.html_url || item.url}">`;;
          cell += `<img src="${item.avatar_url || item.avatar || item.icon || default_avatar}" onerror="javascript:this.removeAttribute(\'data-src\');this.src=\'${default_avatar}\';"/>`;
          cell += `<div class="name image-meta">`;
          cell += `<span class="image-caption">${item.title || item.login}</span>`;
          cell += `</div>`;
          if (item.labels && item.labels.length > 0) {
            let label = item.labels[0];
            cell += `<div class="label" style="background:#${label.color};">${label.name}</div>`;
          }
          cell += `</a>`;
          cell += `</div>`;
          utils.dom(el).find('.grid-box').append(cell);
        }
        if (unavailable.length > 0) {
          let section = `<div class="unavailable-friends"><div class="unavailable-title">失联友链</div><div class="grid-box">`;
          for (let item of unavailable) {
            section += `<div class="grid-cell user-card unavailable-card">`;
            section += `<div class="card-link unavailable-card" aria-disabled="true">`;
            section += `<img src="${escapeHTML(item.avatar_url || item.avatar || item.icon || default_avatar)}"/>`;
            section += `<div class="name image-meta"><span class="image-caption">${escapeHTML(item.title || item.login || '')}</span></div>`;
            if (item.labels && item.labels.length > 0) {
              let label = item.labels[0];
              section += `<div class="label" style="background:#${escapeHTML(label.color)};">${escapeHTML(label.name)}</div>`;
            }
            section += `</div></div>`;
          }
          section += `</div></div>`;
          utils.dom(el).append(section);
        }
        window.wrapLazyloadImages(el);
      });
    }
})();
