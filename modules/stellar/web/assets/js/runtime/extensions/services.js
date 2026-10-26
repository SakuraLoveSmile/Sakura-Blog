/* global setMdLinkIcon */
// 只有响应解码和区域渲染都完成后才保留成功状态；失败/取消仍可在新区域重试。
const renderedServices = new WeakMap();

function serviceText(value) {
  return String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
}

function serviceURL(value, base, attribute = 'src') {
  const text = String(value ?? '').trim();
  if (!text) return '';
  if (attribute === 'href' && text.startsWith('#')) return text;
  try {
    const url = new URL(text, base);
    const protocols = attribute === 'href' ? ['http:', 'https:', 'mailto:', 'tel:'] : ['http:', 'https:'];
    return protocols.includes(url.protocol) ? text : '';
  } catch { return ''; }
}

function serviceStyle(value) {
  const color = /^(?:#[\da-f]{3,8}|(?:rgb|hsl)a?\([\d.,%\s+-]+\)|white|black|transparent|currentcolor)$/i;
  return String(value).split(';').flatMap(declaration => {
    const separator = declaration.indexOf(':');
    if (separator < 0) return [];
    const name = declaration.slice(0, separator).trim().toLowerCase();
    const val = declaration.slice(separator + 1).trim();
    if (['color', 'background', 'background-color', 'border-color'].includes(name) && color.test(val)) return [name + ':' + val];
    if (name === 'text-align' && /^(left|right|center|justify)$/.test(val)) return [name + ':' + val];
    if (['padding-bottom', 'border-bottom'].includes(name) && /^(0|none)$/.test(val)) return [name + ':' + val];
    return [];
  }).join(';');
}

export async function mount(root, context) {
  const assets = context.assets;
  const config = context.extension.config;
  const services = context.legacy.ctx.services;
  const deps = { marked: config.marked };
  const loads = [];
  const onSitesReady = event => {
    const element = event.detail?.target;
    if (element && root.contains(element) && !context.signal.aborted) {
      void assets.script(services.siteinfo.js).then(() => {
        if (!context.signal.aborted) window.setSiteCardIcon?.(element.querySelectorAll('.card-link[data-siteinfo-api]'), context.signal);
      }).catch(error => { if (!context.signal.aborted) context.reportError(error); });
    }
  };
  window.addEventListener('stellar:sites-ready', onSitesReady, { signal: context.signal });

  const enhanceLinks = links => {
    if (context.signal.aborted) return;
    const mdlinks = [];
    for (const link of links) {
      if (!root.contains(link)) continue;
      const href = link.getAttribute('href')?.trim();
      if (!href || href.startsWith('#') || link.getAttribute('class') || link.getAttribute('role') ||
          link.matches('[cardlink], [data-md-link], [data-siteinfo-api]') ||
          link.closest('pre, code, .highlight, .footnotes') ||
          link.querySelector('img, svg') || !link.textContent.trim()) continue;
      let url;
      try { url = new URL(href, link.baseURI); } catch { continue; }
      if (!['http:', 'https:'].includes(url.protocol)) continue;
      link.setAttribute('data-md-link', '');
      const icon = document.createElement('span');
      icon.className = 'md-link-icon ui-icon';
      icon.setAttribute('aria-hidden', 'true');
      icon.innerHTML = context.legacy.ctx.icons['default:link'];
      link.prepend(icon);
      if (services.siteinfo.api) {
        link.setAttribute('data-siteinfo-api', services.siteinfo.api.replace('{href}', encodeURIComponent(url.href)));
        mdlinks.push(link);
      }
    }
    if (mdlinks.length) {
      void assets.script(services.siteinfo.js).then(() => {
        if (!context.signal.aborted) setMdLinkIcon(mdlinks, context.signal);
      }).catch(error => { if (!context.signal.aborted) context.reportError(error); });
    }
  };
  const onMarkdownRendered = event => enhanceLinks(event.detail?.links || []);
  document.addEventListener('stellar:mdrender', onMarkdownRendered, { signal: context.signal });

  // Inline comment providers render asynchronously and replace content on edits/pagination.
  const commentRoots = root.querySelectorAll('#artalk_container, #twikoo_container, #waline_container');
  const commentLinkSelector = '.atk-content a[href], .tk-content a[href], .wl-content a[href]';
  let commentObserver = null;
  if (commentRoots.length) {
    commentObserver = new MutationObserver(records => {
      const links = new Set();
      for (const record of records) {
        for (const node of record.addedNodes) {
          if (node.nodeType !== 1) continue;
          if (node.matches(commentLinkSelector)) links.add(node);
          node.querySelectorAll(commentLinkSelector).forEach(link => links.add(link));
        }
      }
      enhanceLinks(links);
    });
    commentRoots.forEach(element => commentObserver.observe(element, { childList: true, subtree: true }));
    context.signal.addEventListener('abort', () => commentObserver.disconnect(), { once: true });
  }

  const voiceCleanups = [];
  const baseUtils = window.utils;
  const serviceUtils = Object.create(baseUtils);
  const base = (root.ownerDocument || document).baseURI;
  serviceUtils.escapeHTML = serviceText;
  serviceUtils.resolveURL = (value, attribute) => serviceURL(value, base, attribute);
  serviceUtils.safeURL = (value, attribute) => serviceText(serviceUtils.resolveURL(value, attribute));
  serviceUtils.safeColor = value => /^[\da-f]{3,8}$/i.test(String(value)) ? String(value) : '';
  // 动态卡片的默认图使用区域监听，避免保留不受信任的内联事件属性。
  root.addEventListener?.('error', event => {
    const image = event.target;
    if (context.signal.aborted || image?.tagName !== 'IMG') return;
    const fallback = serviceURL(image.getAttribute('data-fallback-src'), base);
    if (!fallback) return;
    image.removeAttribute('data-fallback-src');
    image.removeAttribute('data-src');
    image.src = fallback;
  }, { capture: true, signal: context.signal });
  let purify;
  let sanitizerReady;
  const ensureSanitizer = () => sanitizerReady ||= assets.script('/assets/vendor/dompurify/purify.min.js').then(() => {
    context.signal.throwIfAborted();
    if (typeof window.DOMPurify !== 'function') throw new TypeError('Dynamic content sanitizer is unavailable');
    purify = window.DOMPurify(window);
    if (!purify.isSupported || typeof purify.sanitize !== 'function') throw new TypeError('Dynamic content sanitizer is unsupported');
    purify.addHook('uponSanitizeAttribute', (_node, data) => {
      const name = data.attrName.toLowerCase();
      if (name === 'style') {
        data.attrValue = serviceStyle(data.attrValue);
        data.keepAttr = !!data.attrValue;
      } else if (['href', 'src', 'poster', 'xlink:href', 'data-siteinfo-api', 'data-fallback-src'].includes(name)) {
        const value = serviceURL(data.attrValue, base, name === 'xlink:href' ? 'href' : name);
        data.attrValue = value;
        data.keepAttr = !!value;
      }
    });
    purify.addHook('afterSanitizeAttributes', node => {
      if (node.tagName === 'A' && node.getAttribute('target') === '_blank') {
        const rel = new Set((node.getAttribute('rel') || '').split(/\s+/).filter(Boolean));
        rel.add('noopener'); rel.add('noreferrer');
        node.setAttribute('rel', [...rel].join(' '));
      }
    });
  });
  serviceUtils.sanitizeHTML = value => {
    if (!purify) throw new TypeError('Dynamic content sanitizer is not ready');
    return purify.sanitize(String(value ?? ''), {
      USE_PROFILES: { html: true, svg: true, svgFilters: true },
      FORBID_TAGS: ['style', 'form', 'input', 'textarea', 'button', 'select', 'option', 'iframe', 'object', 'embed'],
      FORBID_ATTR: ['srcdoc', 'srcset'],
      ALLOW_DATA_ATTR: false,
      ADD_ATTR: ['target', 'index', 'data-siteinfo-api', 'data-fallback-src'],
      SANITIZE_NAMED_PROPS: true,
    });
  };
  // 只收窄动态服务的插入边界，不改变公共 utils 或其他扩展的 DOM 语义。
  const wrapDOM = dom => new Proxy(dom, { get(target, key) {
    const value = target[key];
    if (key === 'append') return content => {
      context.signal.throwIfAborted();
      value.call(target, typeof content === 'string' ? serviceUtils.sanitizeHTML(content) : content);
      return wrapDOM(target);
    };
    if (key === 'attr') return (name, content) => {
      if (content !== undefined) {
        context.signal.throwIfAborted();
        if (/^(href|src|poster)$/i.test(name)) content = serviceURL(content, base, name.toLowerCase());
      }
      const result = value.call(target, name, content);
      return result === target ? wrapDOM(target) : result;
    };
    return typeof value === 'function' ? (...args) => {
      const result = value.apply(target, args);
      return result && typeof result.append === 'function' ? wrapDOM(result) : result;
    } : value;
  } });
  serviceUtils.dom = (...args) => wrapDOM(baseUtils.dom(...args));
  serviceUtils.request = (element, url, callback, failure, options = {}) => {
    const previous = element && renderedServices.get(element);
    if (previous?.url === url && (previous.complete || previous.signal === context.signal)) return previous.promise;
    const state = { url, signal: context.signal, complete: false, promise: null };
    if (element) {
      renderedServices.set(element, state);
      baseUtils.onLoading?.(element);
    }
    const requestOptions = { ...options, signal: context.signal, service: options.service || String(element?.className || '').match(/\bds-([\w-]+)\b/)?.[1] };
    // 全局 adapter 的成功标记早于 callback；本区域自行在完整 callback 后记成功。
    state.promise = baseUtils.request(null, url, async response => {
      context.signal.throwIfAborted();
      const guarded = new Proxy(response, { get(target, key) {
        if (key === 'json' || key === 'text') return async () => {
          const data = await target[key]();
          context.signal.throwIfAborted();
          return data;
        };
        const value = Reflect.get(target, key, target);
        return typeof value === 'function' ? value.bind(target) : value;
      } });
      await callback(guarded);
      context.signal.throwIfAborted();
      state.complete = true;
      baseUtils.onLoadSuccess?.(element);
    }, undefined, requestOptions).catch(error => {
      if (element && renderedServices.get(element) === state) renderedServices.delete(element);
      if (!context.signal.aborted) {
        baseUtils.onLoadFailure?.(element);
        failure?.(error);
        context.reportError(error);
      }
    });
    return state.promise;
  };
  serviceUtils.requestWithoutLoading = (url, options = {}) => baseUtils.requestWithoutLoading(url, { ...options, signal: context.signal });
  function loadService(js) {
    return ensureSanitizer().then(() => assets.script(js)).then(script => {
      context.signal.throwIfAborted();
      if (typeof script.stellarMount !== 'function') throw new TypeError(`data-service asset has no regional mount: ${js}`);
      const siteRoot = (context.legacy.ctx.root || '/').replace(/\/?$/, '/');
      return script.stellarMount(root, { ...context, serviceUtils, remoteMarkdownAPI: siteRoot + 'api/stellar/remote-md' });
    });
  }

  // 用于存储需要清理的资源
  let intervals = [];
  let timeouts = [];

  for (let id of Object.keys(services)) {
    const js = services[id].js;
    if (id == 'siteinfo') {
      const cardlinks = root.querySelectorAll('a.link-card[cardlink]');
      const mdlinks = root.querySelectorAll('a[data-md-link][data-siteinfo-api]');
      const siteCards = root.querySelectorAll('.ds-sites, .site-card .card-link[data-siteinfo-api]');
      if (cardlinks?.length > 0 || siteCards?.length > 0 || mdlinks.length > 0) {
        loads.push(assets.script(js).then(function () {
          context.signal?.throwIfAborted();
          setMdLinkIcon(mdlinks, context.signal);
          window.setSiteCardIcon?.(root.querySelectorAll('.site-card .card-link[data-siteinfo-api]'), context.signal);
          if (cardlinks?.length > 0) {
            setCardLink(cardlinks, context.signal);
          }
        }));
      }
    } else if (id == 'ghinfo') {
      const els = root.querySelectorAll('.ds-ghinfo');
      if (els.length > 0) {
        loads.push(loadService(js));
      }
    } else if (id == 'voice') {
      const voiceAudios = root.querySelectorAll('.voice>audio');
      if (voiceAudios?.length > 0) {
        loads.push(assets.script(js).then(function () {
          context.signal?.throwIfAborted();
          const voiceCleanup = createVoiceDom(voiceAudios);
          if (typeof voiceCleanup === 'function') voiceCleanups.push(voiceCleanup);
        }));
      }
    } else if (id == 'video') {
      const videos = root.querySelectorAll('.video>video');
      if (videos?.length > 0) {
        loads.push(assets.script(js).then(function () {
          context.signal?.throwIfAborted();
          voiceCleanups.push(videoEvents(videos));
        }));
      }
    } else if (id == 'download-file') {
      const files = root.querySelectorAll('.chat-file');
      if (files?.length > 0) {
        loads.push(assets.script(js).then(function () {
          context.signal?.throwIfAborted();
          voiceCleanups.push(downloadFileEvent(files));
        }));
      }
    } else {
      const els = root.getElementsByClassName(`ds-${id}`);
      if (els?.length > 0) {
        if (id == 'timeline' || id == 'memos' || id == 'marked') {
          loads.push(assets.script(deps.marked).then(function () {
          context.signal?.throwIfAborted();
            return loadService(js);
          }));
        } else {
          loads.push(loadService(js));
        }
      }
    }
  }

  commentRoots.forEach(element => enhanceLinks(element.querySelectorAll(commentLinkSelector)));

  // chat iphone time
  let phoneTimes = root.querySelectorAll('.chat .status-bar .time');
  let firstAdjustInterval = null;
  let mainInterval = null;

  if (phoneTimes.length > 0) {
    NowTime();
    const date = new Date();
    const sec = date.getSeconds();
    firstAdjustInterval = setInterval(firstAdjustTime, 1000 * (60 - sec));
    intervals.push(firstAdjustInterval);

    function firstAdjustTime() {
      NowTime();
      if (firstAdjustInterval) {
        clearInterval(firstAdjustInterval);
        firstAdjustInterval = null;
      }
      mainInterval = setInterval(NowTime, 1000 * 60);
      intervals.push(mainInterval);
    }

    function NowTime() {
      for (let i = 0; i < phoneTimes.length; ++i) {
        const timeSpan = phoneTimes[i];
        const date = new Date();
        const hour = date.getHours();
        const min = date.getMinutes();
        timeSpan.innerHTML = check(hour) + ":" + check(min);
      }
    };

    function check(val) {
      if (val < 10) {
        return ("0" + val);
      }
      return (val);
    }
  }

  // chat quote - 存储事件监听器以便清理
  const quoteClickHandlers = new Map();
  const chat_quote_obverser = new IntersectionObserver((entries, observer) => {
    entries.filter((entry) => { return entry.isIntersecting }).sort((a, b) => a.intersectionRect.y !== b.intersectionRect.y ? a.intersectionRect.y - b.intersectionRect.y : a.intersectionRect.x - b.intersectionRect.x).forEach((entry, index) => {
        observer.unobserve(entry.target);
        const blinkStart = setTimeout(() => {
          entry.target.classList.add('quote-blink');
          const blinkEnd = setTimeout(() => {
            entry.target.classList.remove('quote-blink');
          }, 1000);
          timeouts.push(blinkEnd);
        }, Math.max(100, 16) * (index + 1));
        timeouts.push(blinkStart);
      });
  });

  var chatQuotes = root.querySelectorAll(".chat .talk .quote");
  chatQuotes.forEach((quote) => {
    const handler = function () {
      var candidate = (root.ownerDocument || root).getElementById("quote-" + quote.getAttribute("quotedCellTag"));
      var chatCellDom = candidate && (root.documentElement || root).contains(candidate) ? candidate : null;
      if (chatCellDom) {
        var chatDiv = chatCellDom.parentElement;
        var mid = chatDiv.clientHeight / 2;
        var offsetTop = chatCellDom.offsetTop;
        if (offsetTop > mid - chatCellDom.clientHeight / 2) {
          chatDiv.scrollTo({
            top: chatCellDom.offsetTop - mid + chatCellDom.clientHeight / 2,
            behavior: "smooth"
          });
        } else {
          chatDiv.scrollTo({
            top: 0,
            behavior: "smooth"
          });
        }
        chat_quote_obverser.observe(chatCellDom);
      }
    };
    quote.addEventListener('click', handler);
    quoteClickHandlers.set(quote, handler); // 保存处理器引用
  });

  // 返回清理函数，用于清理定时器和观察器
  const cleanup = () => {
    commentObserver?.disconnect();
    // 清理所有定时器（包括可能未完成的 firstAdjustInterval）
    intervals.forEach(timer => {
      if (timer) clearInterval(timer);
    });
    intervals = [];
    timeouts.forEach(timer => {
      if (timer) clearTimeout(timer);
    });
    timeouts = [];
    if (firstAdjustInterval) {
      clearInterval(firstAdjustInterval);
      firstAdjustInterval = null;
    }
    if (mainInterval) {
      clearInterval(mainInterval);
      mainInterval = null;
    }

    // 断开观察器
    if (chat_quote_obverser) {
      chat_quote_obverser.disconnect();
    }

    // 移除所有 click 事件监听器
    quoteClickHandlers.forEach((handler, quote) => {
      quote.removeEventListener('click', handler);
    });
    quoteClickHandlers.clear();
    for (let index = voiceCleanups.length - 1; index >= 0; index -= 1) {
      voiceCleanups[index]();
    }
    voiceCleanups.length = 0;
  };
  context.onCleanup?.(cleanup);
  try {
    await Promise.allSettled(loads.map(promise => promise.catch(error => { if (!context.signal.aborted) context.reportError(error); })));
  } catch (error) {
    cleanup();
    throw error;
  }
  return cleanup;
}
