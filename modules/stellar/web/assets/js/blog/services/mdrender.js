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
  'use strict';

  function slugifyHeading(text) {
    return String(text)
      .replace(/<[^>]*>/g, '')
      // eslint-disable-next-line no-control-regex
      .replace(/[\u0000-\u001f]/g, '')
      .replace(/[\s~`!@#$%^&*()\-_+=[\]{}|\\;:"'<>,.?/]+/g, '-')
      .replace(/-{2,}/g, '-')
      .replace(/^-+|-+$/g, '')
      .toLowerCase();
  }

  function normalizeHeadings(container, usedIds) {
    Array.from(container.querySelectorAll('h1,h2,h3,h4,h5,h6')).forEach(function (heading) {
      if (heading.tagName === 'H1') {
        heading.remove();
      }
    });
    Array.from(container.querySelectorAll('h1,h2,h3,h4,h5,h6')).forEach(function (heading) {
      const base = slugifyHeading(heading.textContent) || 'section';
      let id = base;
      let suffix = 1;
      while (usedIds.has(id)) {
        id = base + '-' + suffix++;
      }
      usedIds.add(id);
      heading.id = id;
      const anchor = document.createElement('a');
      anchor.className = 'headerlink';
      anchor.href = '#' + id;
      anchor.title = heading.textContent.trim();
      heading.insertBefore(anchor, heading.firstChild);
    });
  }

  function rewriteRelativeUrls(container, base) {
    Array.from(container.querySelectorAll('img[src],a[href]')).forEach(function (element) {
      const attribute = element.tagName === 'IMG' ? 'src' : 'href';
      const value = element.getAttribute(attribute);
      if (!value || /^(#|https?:|mailto:|tel:|data:|javascript:)/i.test(value) || value.indexOf('//') === 0) {
        return;
      }
      try {
        element.setAttribute(attribute, new URL(value, base).href);
      } catch (error) {
        // Keep malformed relative URLs unchanged.
      }
    });
  }

  function render(el) {
    const src = el.getAttribute('src');
    if (!src) return;
    const url = src + (src.indexOf('?') === -1 ? '?' : '&') + 't=' + Date.now();
    utils.request(el, url, async function (resp) {
      const data = await resp.text();
      const tmp = document.createElement('div');
      tmp.innerHTML = marked.parse(data);

      const base = el.getAttribute('data-base');
      if (base) rewriteRelativeUrls(tmp, base);
      if (el.getAttribute('data-heading') === 'true') {
        const usedIds = new Set();
        const scope = el.closest('article, .md-text') || document;
        Array.from(scope.querySelectorAll('[id]')).forEach(function (node) {
          usedIds.add(node.id);
        });
        normalizeHeadings(tmp, usedIds);
      }

      let target = el;
      if (el.getAttribute('data-replace') === 'true') {
        const nodes = Array.from(tmp.childNodes);
        if (nodes.length === 0) {
          el.remove();
          target = null;
        } else {
          el.replaceWith.apply(el, nodes);
          target = nodes[0];
        }
      } else {
        el.innerHTML = tmp.innerHTML;
      }
      document.dispatchEvent(new CustomEvent('stellar:mdrender', { detail: { target: target } }));
    });
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { slugifyHeading: slugifyHeading };
    return;
  }

  Array.from(document.getElementsByClassName('ds-mdrender')).forEach(render);
})();

  })(scopedDocument);
  return scope.cleanup;
};

})();
