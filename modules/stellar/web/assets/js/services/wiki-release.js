(function () {
  var els = document.getElementsByClassName('ds-wiki-release');
  for (var i = 0; i < els.length; i++) {
    (function (el) {
      var api = el.dataset.api;
      if (!api) return;
      utils.request(null, api, function (resp) {
        resp.json().then(function (data) {
          if (!data || !data.tag_name) return;
          var tag = el.querySelector('[type=text]#tag_name');
          var link = el.querySelector('[type=link]#html_url');
          if (tag) tag.textContent = data.tag_name;
          if (link && data.html_url) link.href = data.html_url;
          el.classList.remove('is-loading');
          el.classList.add('loaded');
          el.hidden = false;
        }).catch(function () {});
      });
    })(els[i]);
  }
})();
