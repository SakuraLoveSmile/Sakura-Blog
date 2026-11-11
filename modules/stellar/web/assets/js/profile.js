(function () {
  var SUPPORTED_PROVIDERS = ['artalk', 'waline', 'twikoo', 'native'];
  // 原生评论（站点未接入第三方评论后端）：已登录读者用站点账号（/api/auth），
  // 未登录时与上游一致，落到浏览器本地的匿名资料，保证不登录也能保存。
  var NATIVE_PROVIDER = 'native';
  var SESSION_ENDPOINT = '/api/auth/me';
  var PROFILE_ENDPOINT = '/api/auth/profile';
  var LOGOUT_ENDPOINT = '/api/auth/logout';
  // 与 comments.js 的匿名身份共用同一份键，评论区与设置页读写同一份资料。
  var LOCAL_NAME_KEY = 'cmt_nick';
  var LOCAL_EMAIL_KEY = 'cmt_email';
  var LOCAL_URL_KEY = 'cmt_link';
  var LOCAL_AVATAR_KEY = 'cmt_avatar';
  var SETTINGS_ICON = 'default:settings';
  var PROFILE_ICON = 'default:profile';
  var GRAVATAR_BASE_URL = window.stellarClientData.ctx.services.gravatar.baseUrl.replace(/\/+$/, '') + '/';
  var localStore = null;
  var sessionStore = null;
  var gravatarHashes = new Map();
  var widgetRenderRevisions = new WeakMap();
  try { localStore = localStorage; } catch (error) {}
  try { sessionStore = sessionStorage; } catch (error) {}

  function parseStorage(store, key) {
    try {
      var raw = store?.getItem?.(key);
      if (!raw) return null;
      var value = JSON.parse(raw);
      return value && typeof value === 'object' && !Array.isArray(value) ? value : null;
    } catch (error) {
      return null;
    }
  }

  function hasStorageValue(store, key) {
    try { return store?.getItem?.(key) != null; } catch (error) { return false; }
  }

  function writeStorage(store, key, value) {
    try {
      store?.setItem?.(key, JSON.stringify(value));
      return !!store?.setItem;
    } catch (error) { return false; }
  }

  function removeStorage(store, key) {
    try {
      store?.removeItem?.(key);
      return !!store?.removeItem;
    } catch (error) { return false; }
  }

  // 匿名资料的 cmt_* 键与评论区共用，必须是纯文本（评论区按字符串直读直写）。
  function readText(store, key) {
    try {
      var value = store?.getItem?.(key);
      return typeof value === 'string' ? value : '';
    } catch (error) { return ''; }
  }

  function writeText(store, key, value) {
    try {
      if (!store?.setItem) return false;
      store.setItem(key, value || '');
      return true;
    } catch (error) { return false; }
  }

  function cleanText(value, maxLength) {
    return typeof value === 'string' ? value.trim().slice(0, maxLength || 80) : '';
  }

  function cleanEmail(value) {
    var email = cleanText(value, 254);
    if (!email) return '';
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) ? email : null;
  }

  function cleanHttpUrl(value) {
    if (typeof value !== 'string' || value.trim().length === 0) return '';
    var candidate = value.trim();
    if (candidate.startsWith('//')) candidate = 'https:' + candidate;
    try {
      var parsed = new URL(candidate);
      return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.href : null;
    } catch (error) { return null; }
  }

  // 保存时与后端一致：缺协议的输入按 https 补全（read 侧仍只认完整地址）。
  function normalizeHttpUrl(value) {
    if (typeof value !== 'string' || value.trim().length === 0) return '';
    var candidate = value.trim();
    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(candidate) && !candidate.startsWith('//')) candidate = 'https://' + candidate;
    return cleanHttpUrl(candidate);
  }

  // 保存时与后端一致：缺协议的输入按 https 补全（读取侧仍只认完整地址）。
  function normalizeHttpUrl(value) {
    if (typeof value !== 'string' || value.trim().length === 0) return '';
    var candidate = value.trim();
    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(candidate) && !candidate.startsWith('//')) candidate = 'https://' + candidate;
    return cleanHttpUrl(candidate);
  }

  function normalizeGravatarEmail(value) {
    var email = cleanEmail(value);
    return email ? email.toLowerCase() : '';
  }

  function hashToHex(buffer) {
    return Array.from(new Uint8Array(buffer), function (byte) {
      return byte.toString(16).padStart(2, '0');
    }).join('');
  }

  function getGravatarHash(email) {
    var normalized = normalizeGravatarEmail(email);
    if (!normalized) return Promise.resolve('');
    if (gravatarHashes.has(normalized)) return gravatarHashes.get(normalized);
    var cryptoApi = window.crypto;
    if (!cryptoApi?.subtle?.digest || typeof TextEncoder !== 'function') return Promise.resolve('');
    var promise;
    try {
      promise = Promise.resolve(cryptoApi.subtle.digest('SHA-256', new TextEncoder().encode(normalized)))
        .then(hashToHex)
        .catch(function () { return ''; });
    } catch (error) {
      promise = Promise.resolve('');
    }
    gravatarHashes.set(normalized, promise);
    return promise;
  }

  function getGravatarUrl(email, size) {
    var requestedSize = Math.round(Number(size));
    var safeSize = Number.isFinite(requestedSize) ? Math.min(2048, Math.max(1, requestedSize)) : 80;
    return getGravatarHash(email).then(function (hash) {
      return hash ? GRAVATAR_BASE_URL + hash + '?s=' + safeSize + '&r=g&d=404' : '';
    });
  }

  function emptyIdentity(provider) {
    var id = String(provider || '').toLowerCase();
    return { provider: id, supported: SUPPORTED_PROVIDERS.includes(id), name: '', email: '', url: '', avatar: '' };
  }

  function readIdentity(provider) {
    var id = String(provider || '').toLowerCase();
    if (id === NATIVE_PROVIDER) return readNativeIdentity();
    var identity = emptyIdentity(id);
    var value = null;
    if (id === 'artalk') {
      value = parseStorage(localStore, 'ArtalkUser') || {};
      identity.name = cleanText(value.name || value.nick);
      identity.email = cleanText(value.email || value.mail, 254);
      identity.url = cleanHttpUrl(value.link || value.url) || '';
      identity.avatar = cleanHttpUrl(value.avatar) || '';
      return identity;
    }
    if (id === 'waline') {
      value = parseStorage(sessionStore, 'WALINE_USER') || parseStorage(localStore, 'WALINE_USER') || {};
      var meta = parseStorage(sessionStore, 'WALINE_USER_META') || parseStorage(localStore, 'WALINE_USER_META') || {};
      identity.name = cleanText(value.display_name || value.nick || meta.nick);
      identity.email = cleanText(value.email || value.mail || meta.mail, 254);
      identity.url = cleanHttpUrl(value.url || value.link || meta.link) || '';
      identity.avatar = cleanHttpUrl(value.avatar || meta.avatar) || '';
      return identity;
    }
    if (id === 'twikoo') {
      value = parseStorage(localStore, 'twikoo') || {};
      identity.name = cleanText(value.nick);
      identity.email = cleanText(value.mail, 254);
      identity.url = cleanHttpUrl(value.link) || '';
      identity.avatar = cleanHttpUrl(value.avatar) || '';
      return identity;
    }
    return identity;
  }

  function requestJSON(path, options) {
    var fetchApi = window.fetch;
    if (typeof fetchApi !== 'function') return Promise.resolve(null);
    var init = Object.assign({ credentials: 'same-origin' }, options || {});
    return Promise.resolve(fetchApi(path, init)).then(function (response) {
      return Promise.resolve(response.json()).then(function (body) {
        return { status: response.status, ok: !!response.ok, body: body };
      }, function () {
        return { status: response.status, ok: !!response.ok, body: null };
      });
    }, function () { return null; });
  }

  // 沙箱 iframe（Console 预览）是不透明源，向站点接口发同源请求必然被 CORS 拦下并报错，
  // 这里直接跳过网络，只用本地资料。
  function isOpaqueOrigin() {
    try {
      var origin = window.location && window.location.origin;
      return !origin || origin === 'null';
    } catch (error) { return true; }
  }

  // 会话读取在整页生命周期内只发一次：挂件会在 input/click/导航等事件后重渲染，
  // 每次都请求 /api/auth/me 既浪费也可能触发限流。
  var sessionPromise = null;
  function readSession(force) {
    if (!force && sessionPromise) return sessionPromise;
    sessionPromise = isOpaqueOrigin()
      ? Promise.resolve(null)
      : requestJSON(SESSION_ENDPOINT, { method: 'GET' });
    return sessionPromise;
  }

  function invalidateSession() {
    sessionPromise = null;
  }

  // 未登录读者的资料与上游一致存在浏览器本地，键名与评论区匿名身份共用。
  function readLocalIdentity() {
    var identity = emptyIdentity(NATIVE_PROVIDER);
    identity.supported = true;
    identity.local = true;
    identity.name = cleanText(readText(localStore, LOCAL_NAME_KEY), 80);
    identity.email = cleanText(readText(localStore, LOCAL_EMAIL_KEY), 254);
    identity.url = cleanHttpUrl(readText(localStore, LOCAL_URL_KEY)) || '';
    identity.avatar = cleanHttpUrl(readText(localStore, LOCAL_AVATAR_KEY)) || '';
    return identity;
  }

  function hasLocalIdentity(identity) {
    return !!(identity && (identity.name || identity.email || identity.url));
  }

  function writeLocalIdentity(fields) {
    var rawName = typeof fields?.name === 'string' ? fields.name.trim() : '';
    var name = cleanText(rawName);
    var email = cleanEmail(fields?.email);
    var url = normalizeHttpUrl(fields?.url);
    if (!name || rawName.length > 80) return { ok: false, validation: 'name', partial: false };
    if (email === null) return { ok: false, validation: 'email', partial: false };
    if (url === null) return { ok: false, validation: 'url', partial: false };
    var results = [
      writeText(localStore, LOCAL_NAME_KEY, name),
      writeText(localStore, LOCAL_EMAIL_KEY, email),
      writeText(localStore, LOCAL_URL_KEY, url)
    ];
    var succeeded = results.filter(Boolean).length;
    var outcome = { ok: succeeded === results.length && succeeded > 0, partial: succeeded > 0 && succeeded < results.length, local: true };
    if (succeeded > 0) dispatchChange(NATIVE_PROVIDER);
    return outcome;
  }

  function readNativeIdentity() {
    return readSession().then(function (result) {
      var body = result && result.body;
      if (result && result.ok && body && body.verified) {
        var identity = emptyIdentity(NATIVE_PROVIDER);
        identity.supported = true;
        identity.account = true;
        identity.name = cleanText(body.nickname, 80);
        identity.email = cleanText(body.email, 254);
        identity.url = cleanHttpUrl(body.website) || '';
        identity.avatar = cleanHttpUrl(body.avatar) || '';
        return identity;
      }
      // 未登录：照上游用本地资料，设置页因此对访客也可用。
      return readLocalIdentity();
    });
  }

  function writeNativeIdentity(fields) {
    var payload = {
      name: typeof fields?.name === 'string' ? fields.name.trim() : '',
      email: typeof fields?.email === 'string' ? fields.email.trim() : '',
      url: typeof fields?.url === 'string' ? fields.url.trim() : ''
    };
    return readSession().then(function (session) {
      var body = session && session.body;
      if (!session || !session.ok || !body || !body.verified) return writeLocalIdentity(fields);
      return requestJSON(PROFILE_ENDPOINT, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }).then(function (result) {
        if (!result) return { ok: false, partial: false };
        var responseBody = result.body || {};
        if (result.ok) {
          dispatchChange(NATIVE_PROVIDER);
          return { ok: true, partial: false, account: true };
        }
        var outcome = { ok: false, partial: false, account: true };
        if (typeof responseBody.error === 'string' && responseBody.error) outcome.message = responseBody.error;
        if (typeof responseBody.field === 'string' && responseBody.field) outcome.field = responseBody.field;
        if (result.status === 401) outcome.unauthenticated = true;
        return outcome;
      });
    });
  }

  function logoutNative() {
    return readSession().then(function (session) {
      var body = session && session.body;
      if (session && session.ok && body && body.verified) {
        return requestJSON(LOGOUT_ENDPOINT, { method: 'POST' }).then(function (result) {
          invalidateSession();
          if (!result || !result.ok) return { ok: false, partial: false };
          // 退出后整页重载：评论区的登录态与身份栏同源，需按新状态重新渲染。
          try { window.location?.reload?.(); } catch (error) {}
          return { ok: true, partial: false };
        });
      }
      // 未登录时「退出登录」等价于清掉本地匿名资料，和上游一致。
      var results = [
        removeStorage(localStore, LOCAL_NAME_KEY),
        removeStorage(localStore, LOCAL_EMAIL_KEY),
        removeStorage(localStore, LOCAL_URL_KEY),
        removeStorage(localStore, LOCAL_AVATAR_KEY)
      ];
      var succeeded = results.filter(Boolean).length;
      var outcome = { ok: succeeded === results.length, partial: succeeded > 0 && succeeded < results.length, local: true };
      if (succeeded > 0) dispatchChange(NATIVE_PROVIDER);
      return outcome;
    });
  }

  function dispatchChange(provider) {
    // 只用于通知本页刷新；环境没有 CustomEvent（如 node 测试）时静默跳过。
    var CustomEventCtor = window.CustomEvent || (typeof CustomEvent === 'function' ? CustomEvent : null);
    if (!CustomEventCtor) return;
    try {
      window.dispatchEvent?.(new CustomEventCtor('stellar:profile-change', { detail: { provider: provider } }));
    } catch (error) {}
  }

  function updateExistingCopies(key, updater) {
    return [localStore, sessionStore]
      .filter(function (store) { return hasStorageValue(store, key); })
      .map(function (store) { return writeStorage(store, key, updater(parseStorage(store, key) || {})); });
  }

  function writeIdentity(provider, fields) {
    var id = String(provider || '').toLowerCase();
    if (id === NATIVE_PROVIDER) return writeNativeIdentity(fields);
    if (!SUPPORTED_PROVIDERS.includes(id)) return { ok: false, unsupported: true, partial: false };
    var rawName = typeof fields?.name === 'string' ? fields.name.trim() : '';
    var name = cleanText(rawName);
    var email = cleanEmail(fields?.email);
    var url = cleanHttpUrl(fields?.url);
    if (!name || rawName.length > 80) return { ok: false, validation: 'name', partial: false };
    if (email === null) return { ok: false, validation: 'email', partial: false };
    if (url === null) return { ok: false, validation: 'url', partial: false };

    var results = [];
    if (id === 'artalk') {
      var artalk = parseStorage(localStore, 'ArtalkUser') || {};
      results.push(writeStorage(localStore, 'ArtalkUser', Object.assign({}, artalk, { name: name, email: email, link: url })));
    } else if (id === 'twikoo') {
      var twikoo = parseStorage(localStore, 'twikoo') || {};
      results.push(writeStorage(localStore, 'twikoo', Object.assign({}, twikoo, { nick: name, mail: email, link: url })));
    } else {
      results = results.concat(
        updateExistingCopies('WALINE_USER', function (current) { return Object.assign({}, current, { display_name: name, email: email, url: url }); }),
        updateExistingCopies('WALINE_USER_META', function (current) { return Object.assign({}, current, { nick: name, mail: email, link: url }); })
      );
      if (results.length === 0) results.push(writeStorage(localStore, 'WALINE_USER_META', { nick: name, mail: email, link: url }));
    }
    var succeeded = results.filter(Boolean).length;
    var outcome = { ok: succeeded === results.length && succeeded > 0, partial: succeeded > 0 && succeeded < results.length };
    if (succeeded > 0) dispatchChange(id);
    return outcome;
  }

  function logout(provider) {
    var id = String(provider || '').toLowerCase();
    if (id === NATIVE_PROVIDER) return logoutNative();
    if (!SUPPORTED_PROVIDERS.includes(id)) return { ok: false, unsupported: true, partial: false };
    var targets = id === 'artalk'
      ? [[localStore, 'ArtalkUser']]
      : id === 'twikoo'
        ? [[localStore, 'twikoo']]
        : [[localStore, 'WALINE_USER'], [sessionStore, 'WALINE_USER'], [localStore, 'WALINE_USER_META'], [sessionStore, 'WALINE_USER_META']];
    var results = targets.map(function (target) { return removeStorage(target[0], target[1]); });
    var succeeded = results.filter(Boolean).length;
    var outcome = { ok: succeeded === results.length, partial: succeeded > 0 && succeeded < results.length };
    if (succeeded > 0) dispatchChange(id);
    return outcome;
  }

  function renderIconMedia(avatar, key) {
    var media = document.createElement('span');
    media.className = 'ui-icon';
    media.innerHTML = window.stellarIcons?.[key] || '';
    avatar.replaceChildren(media);
  }

  function renderImageMedia(avatar, url, onError) {
    var media = document.createElement('img');
    media.className = 'settings-widget__image';
    media.alt = '';
    media.onerror = onError || function () { renderIconMedia(avatar, PROFILE_ICON); };
    media.src = url;
    avatar.replaceChildren(media);
  }

  function isCurrentWidgetRender(widget, revision) {
    return widgetRenderRevisions.get(widget) === revision;
  }

  function renderWidgetFallback(widget, avatar, fallbackAvatar, revision) {
    if (!isCurrentWidgetRender(widget, revision)) return;
    if (!fallbackAvatar) {
      renderIconMedia(avatar, PROFILE_ICON);
      return;
    }
    renderImageMedia(avatar, fallbackAvatar, function () {
      if (isCurrentWidgetRender(widget, revision)) renderIconMedia(avatar, PROFILE_ICON);
    });
  }

  function renderWidgetGravatar(widget, avatar, identity, fallbackAvatar, revision) {
    if (!identity.email) {
      renderWidgetFallback(widget, avatar, fallbackAvatar, revision);
      return;
    }
    renderIconMedia(avatar, PROFILE_ICON);
    getGravatarUrl(identity.email, 64).then(function (url) {
      if (!isCurrentWidgetRender(widget, revision)) return;
      if (!url) {
        renderWidgetFallback(widget, avatar, fallbackAvatar, revision);
        return;
      }
      renderImageMedia(avatar, url, function () {
        renderWidgetFallback(widget, avatar, fallbackAvatar, revision);
      });
    }, function () {
      renderWidgetFallback(widget, avatar, fallbackAvatar, revision);
    });
  }

  function applyWidgetIdentity(widget, identity, settingsName, revision) {
    if (!isCurrentWidgetRender(widget, revision)) return;
    var hasIdentity = identity.supported && !!identity.name;
    var name = hasIdentity ? identity.name : settingsName;
    var nameElement = widget.querySelector('.settings-widget__name');
    var avatar = widget.querySelector('.settings-widget__avatar');
    if (nameElement) nameElement.textContent = name;
    widget.setAttribute('aria-label', name);
    widget.setAttribute('title', name);
    if (!avatar) return;
    if (!hasIdentity) {
      renderIconMedia(avatar, SETTINGS_ICON);
      return;
    }
    var fallbackAvatar = widget.dataset.profileFallbackAvatar || '';
    if (identity.avatar) {
      renderImageMedia(avatar, identity.avatar, function () {
        renderWidgetGravatar(widget, avatar, identity, fallbackAvatar, revision);
      });
      return;
    }
    renderWidgetGravatar(widget, avatar, identity, fallbackAvatar, revision);
  }

  function renderWidget(widget) {
    var revision = (widgetRenderRevisions.get(widget) || 0) + 1;
    widgetRenderRevisions.set(widget, revision);
    var settingsName = widget.dataset.settingsLabel || widget.getAttribute('aria-label') || 'Settings';
    var identityEnabled = widget.dataset.profileIdentityEnabled === 'true';
    if (!identityEnabled) {
      applyWidgetIdentity(widget, emptyIdentity(widget.dataset.profileProvider), settingsName, revision);
      return;
    }
    var provider = widget.dataset.profileProvider;
    // 原生账号要先问一次 /api/auth/me（异步），第三方后端仍是同步本地存储。
    Promise.resolve(readIdentity(provider)).then(function (identity) {
      applyWidgetIdentity(widget, identity || emptyIdentity(provider), settingsName, revision);
    }, function () {
      applyWidgetIdentity(widget, emptyIdentity(provider), settingsName, revision);
    });
  }

  var mountedRoots = new WeakMap();
  function mount(root) {
    root = root || document;
    if (mountedRoots.has(root)) return mountedRoots.get(root);
    var refresh = function () { root.querySelectorAll?.('.settings-widget').forEach(renderWidget); };
    var delayedRefresh = function () { setTimeout(refresh, 0); };
    var onInteraction = function (event) {
      if (event.target?.closest?.('#comments, .comments, [class*="comment"]')) delayedRefresh();
    };
    window.addEventListener?.('storage', refresh);
    window.addEventListener?.('focus', refresh);
    window.addEventListener?.('stellar:profile-change', refresh);
    document.addEventListener('stellar:navigation-complete', refresh);
    root.addEventListener?.('input', onInteraction);
    root.addEventListener?.('change', onInteraction);
    root.addEventListener?.('submit', onInteraction);
    root.addEventListener?.('click', onInteraction);
    refresh();
    var cleanup = function () {
      window.removeEventListener?.('storage', refresh);
      window.removeEventListener?.('focus', refresh);
      window.removeEventListener?.('stellar:profile-change', refresh);
      document.removeEventListener('stellar:navigation-complete', refresh);
      root.removeEventListener?.('input', onInteraction);
      root.removeEventListener?.('change', onInteraction);
      root.removeEventListener?.('submit', onInteraction);
      root.removeEventListener?.('click', onInteraction);
      mountedRoots.delete(root);
    };
    mountedRoots.set(root, cleanup);
    return cleanup;
  }

  window.stellarProfile = {
    mount: mount,
    supports: function (provider) { return SUPPORTED_PROVIDERS.includes(String(provider || '').toLowerCase()); },
    getGravatarUrl: getGravatarUrl,
    readIdentity: readIdentity,
    writeIdentity: writeIdentity,
    logout: logout
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { mount(document); }, { once: true });
  else mount(document);
})();
