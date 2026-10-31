// 远端元信息只作为文本和 HTTP(S) 图片地址使用；区域取消后允许再次挂载。
const siteInfoLoads = new WeakMap();
function siteInfoURL(value, base) {
  if (typeof value !== 'string' || !value.trim()) return '';
  try {
    const url = new URL(value, base);
    return ['http:', 'https:'].includes(url.protocol) ? url.href : '';
  } catch { return ''; }
}
function requestSiteInfo(element, kind, api, signal, render) {
  if (signal?.aborted) return Promise.resolve();
  let loads = siteInfoLoads.get(element);
  if (!loads) { loads = new Map(); siteInfoLoads.set(element, loads); }
  const previous = loads.get(kind);
  if (previous?.api === api && (previous.complete || previous.signal === signal && !signal?.aborted)) return previous.promise;
  const state = { api, signal, complete: false, promise: null };
  loads.set(kind, state);
  state.promise = utils.request(null, api, async response => {
    const data = await response.json();
    if (signal?.aborted) return;
    await render(data);
    if (!signal?.aborted) state.complete = true;
  }, undefined, { service: 'siteinfo', signal }).catch(() => {});
  state.promise.finally(() => { if (!state.complete && loads.get(kind) === state) loads.delete(kind); });
  return state.promise;
}

// 本插件由CardLink定制而成，原项目源码: https://github.com/Lete114/CardLink
function setCardLink(nodes, signal) {
  // If the `nodes` do not contain a `forEach` method, then the default `a[cardlink]` is used
  nodes = 'forEach' in (nodes || {}) ? nodes : document.querySelectorAll('a[cardlink]')
  nodes.forEach((el) => {
    // If it is not a tag element then it is not processed
    if (el.nodeType !== 1) return;
    const api = el.dataset.api;
    if (api == null) return;
    // 走统一请求入口，数据缓存对 siteinfo 同样生效
    requestSiteInfo(el, 'card', api, signal, async function(data) {
        var autofill = [];
        const autofillStr = el.getAttribute('autofill');
        if (autofillStr) {
          autofill = autofillStr.split(',');
        }
        if (data.title && data.title.length > 0 && autofill.includes('title')) {
          el.querySelector('.title').textContent = data.title;
          el.title = data.title;
        }
        const preferredIcon = el.classList.contains('rich') ? data.favicon : data.appicon;
        const icon = siteInfoURL(typeof preferredIcon === 'string' && preferredIcon ? preferredIcon : data.icon, el.baseURI);
        if (typeof icon === 'string' && icon && autofill.includes('icon')) {
          el.querySelector('.img').style.backgroundImage = 'url(' + JSON.stringify(icon) + ')';
          el.querySelector('.img').setAttribute('data-bg', icon);
        }
        let desc = el.querySelector('.desc');
        if (desc && data.desc && data.desc.length > 0 && autofill.includes('desc')) {
          desc.textContent = data.desc;
        }
        el.removeAttribute('cardlink');
    });
  })
}

function setMdLinkIcon(nodes, signal) {
  nodes.forEach((el) => {
    requestSiteInfo(el, 'markdown-icon', el.dataset.siteinfoApi, signal, async function(data) {
        const icon = typeof data.favicon === 'string' && data.favicon ? data.favicon : data.icon;
        if (typeof icon !== 'string' || !icon) return;
        let url;
        try { url = new URL(icon, el.href); } catch { return; }
        if (!['http:', 'https:'].includes(url.protocol)) return;
        const image = new Image();
        image.alt = '';
        await new Promise((resolve, reject) => {
          const done = error => {
            image.onload = image.onerror = null;
            signal?.removeEventListener('abort', cancel);
            error ? reject(error) : resolve();
          };
          const cancel = () => done();
          image.onload = function() {
            if (!signal?.aborted && el.isConnected) el.querySelector('.md-link-icon')?.replaceChildren(image);
            done();
          };
          image.onerror = () => done(new Error('Site icon failed to load'));
          signal?.addEventListener('abort', cancel, { once: true });
          image.src = url.href;
        });
    });
  });
}

function setSiteCardIcon(nodes, signal) {
  nodes = 'forEach' in (nodes || {}) ? nodes : document.querySelectorAll('.site-card .card-link[data-siteinfo-api]')
  nodes.forEach((el) => {
    if (el.nodeType !== 1) return;
    const api = el.dataset.siteinfoApi;
    if (api == null) return;
    requestSiteInfo(el, 'site-icon', api, signal, async function(data) {
        const iconUrl = siteInfoURL(typeof data.appicon === 'string' && data.appicon ? data.appicon : data.icon, el.baseURI);
        if (typeof iconUrl === 'string' && iconUrl) {
          const icon = el.querySelector('.siteinfo-icon');
          if (icon) {
            icon.src = iconUrl;
            icon.setAttribute('data-src', iconUrl);
          }
        }
    });
  })
}
