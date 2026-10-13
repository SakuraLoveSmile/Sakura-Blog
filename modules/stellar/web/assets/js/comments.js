/* comments.js — Artalk 风格评论区前端逻辑
 * 依赖：comments.html 模板（#cmt-* DOM）+ comments.css 样式
 * 无外部依赖；PJAX 兼容：document 事件委托 + __cmtBuilt guard 防重复绑定。
 * 表情 CDN：cdn-x/emoticons（与文章 {% emoji %} shortcode 同源），清单为 Artalk items 格式
 */
(function () {
  'use strict';
  if (window.stellar && window.stellar.comments && window.stellar.comments.mount) return;
  var mounted = new WeakMap();

  function createInstance(scope) {
  var document = scope ? scope.document : window.document;
  var fetch = scope ? scope.fetch : window.fetch.bind(window);
  var setTimeout = scope ? scope.setTimeout : window.setTimeout.bind(window);
  var requestAnimationFrame = scope ? scope.requestAnimationFrame : window.requestAnimationFrame;
  var IntersectionObserver = scope ? function (fn, opts) { return scope.observer('IntersectionObserver', fn, opts); } : window.IntersectionObserver;
  function listen(target, event, handler, options) {
    if (scope) scope.listen(target, event, handler, options);
    else target.addEventListener(event, handler, options);
  }

  // ------- 常量 -------
  // artalk/ 目录下有各 pack 的 Artalk 格式清单（items: [{key, val}]），val 为图片直链
  var EMOJI_BASE = 'https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/artalk/';
  var EMOJI_PACKS = ['blobcat', 'qq', 'tieba', 'aru', 'twemoji'];
  var BOOST_EMOJI_DATA_URL = '/assets/js/boost-emoji-data.json?v=15.3.2';
  var BOOST_EMOJI_RECENT_KEY = 'cmt_boost_recent';
  var BOOST_EMOJI_FALLBACK = [
    ['👍', 1, '赞', '赞 thumbs up'], ['❤️', 0, '红心', '红心 heart love'],
    ['😄', 0, '大笑', '大笑 smile'], ['🎉', 6, '庆祝', '庆祝 party'],
    ['😕', 0, '困惑', '困惑 confused'], ['🚀', 5, '火箭', '火箭 rocket'],
    ['👏', 1, '鼓掌', '鼓掌 clap'], ['😂', 0, '笑哭', '笑哭 joy'],
    ['🤝', 1, '握手', '握手 handshake'], ['🔥', 7, '火', '火 fire'],
    ['✨', 6, '闪亮', '闪亮 sparkles'], ['💯', 7, '满分', '满分 hundred']
  ];
  var BOOST_EMOJI_GROUPS = [
    { id: 'recent', icon: '⭐', label: '常用' },
    { id: '0', icon: '😄', label: '笑脸和情感' },
    { id: '1', icon: '👋', label: '人物和身体' },
    { id: '3', icon: '🐵', label: '动物和自然' },
    { id: '4', icon: '🍇', label: '食物和饮料' },
    { id: '5', icon: '✈️', label: '旅行和地点' },
    { id: '6', icon: '⚽', label: '活动' },
    { id: '7', icon: '💡', label: '物品' },
    { id: '8', icon: '❤️', label: '符号' },
    { id: '9', icon: '🚩', label: '旗帜' }
  ];

  // ------- 全局运行时状态 -------
  var state = {
    me: null,            // {id, nickname, verified, avatar, ...} 来自 /api/auth/me
    mode: 'anonymous',   // 'anonymous' | 'logged-in'
    pageKey: '',
    adminBadge: '',
    packLoaded: Object.create(null),  // pack -> true
    packItems: Object.create(null),   // pack -> items 清单缓存（切回已加载 tab 时重渲染）
    activePack: EMOJI_PACKS[0],
    parentId: 0,
    comments: [],         // 已渲染的评论列表（供插入新评论后更新计数）
    loadingComments: false,
    sortMode: 'newest',   // 'newest' | 'hottest' | 'oldest' | 'author'
    boostEmojiItems: null,
    boostEmojiPromise: null
  };

  // ------- 工具 -------
  function $(sel, root) { return (root || document).querySelector(sel); }
  function $$(sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); }

  function escapeHtml(s) {
    if (s == null) return '';
    return String(s).replace(/[&<>"']/g, function (ch) {
      return ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'})[ch];
    });
  }

  function toast(msg, type) {
    var t = document.createElement('div');
    t.textContent = msg;
    var bg = type === 'error' ? '#ef4444' : (type === 'success' ? '#10b981' : '#111827');
    t.style.cssText = 'position:fixed;z-index:10001;left:50%;top:68px;transform:translateX(-50%);padding:.45rem 1rem;font-size:.82rem;color:#fff;background:' + bg + ';border-radius:999px;box-shadow:0 8px 24px rgba(17,24,39,.25);opacity:0;transition:opacity .18s;';
    document.body.appendChild(t);
    if (scope) scope.onCleanup(function () { if (t.parentNode) t.parentNode.removeChild(t); });
    requestAnimationFrame(function () { t.style.opacity = '1'; });
    setTimeout(function () {
      t.style.opacity = '0';
      setTimeout(function () { if (t.parentNode) t.parentNode.removeChild(t); }, 220);
    }, 2200);
  }

  function insertAtCursor(textarea, text) {
    var start = textarea.selectionStart || 0;
    var end = textarea.selectionEnd || 0;
    var value = textarea.value;
    textarea.value = value.substring(0, start) + text + value.substring(end);
    var pos = start + text.length;
    textarea.setSelectionRange(pos, pos);
    textarea.focus();
  }

  // ------- 身份：拉取 /api/auth/me 并渲染身份栏 -------
  // 三态：logged-in（昵称+退出）/ anonymous（跳过验证，三输入）/ guest（无输入行，按钮=登录）
  function skipVerifyFlag() {
    try { return localStorage.getItem('cmt_skip') === '1'; } catch (e) { return false; }
  }

  function fetchMe(cb) {
    fetch('/api/auth/me', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        state.me = d || { id: 0, verified: false };
        state.mode = state.me && state.me.id > 0 ? 'logged-in' : (skipVerifyFlag() ? 'anonymous' : 'guest');
        renderIdentity();
        if (cb) cb();
      })
      .catch(function () {
        state.me = { id: 0, verified: false };
        state.mode = skipVerifyFlag() ? 'anonymous' : 'guest';
        renderIdentity();
        if (cb) cb();
      });
  }

  function renderIdentity() {
    var identity = $('#cmt-identity');
    var anon = $('#cmt-id-anon');
    var logged = $('#cmt-id-logged');
    var submit = $('#cmt-submit');
    var accountBtn = $('#cmt-tool-account');
    if (!anon || !logged) return;

    if (state.mode === 'logged-in') {
      if (identity) identity.style.display = 'flex';
      anon.style.display = 'none';
      logged.style.display = 'flex';
      var me = state.me || {};
      var nickEl = $('#cmt-id-nick');
      if (nickEl) nickEl.textContent = me.nickname || me.username || '';
      if (submit) submit.textContent = '发送';
      if (accountBtn) accountBtn.style.display = 'none';
      // 已登录用户：昵称旁显示 verified 徽章（注册用户即已验证，博主优先显示博主徽章则隐藏）
      var vEl = $('#cmt-id-verified');
      if (vEl) vEl.style.display = (me.verified !== false && !me.is_admin) ? 'inline-flex' : 'none';
    } else if (state.mode === 'anonymous') {
      if (identity) identity.style.display = 'flex';
      anon.style.display = 'block';
      logged.style.display = 'none';
      if (submit) submit.textContent = '发送';
      if (accountBtn) accountBtn.style.display = 'inline-flex';
      // 匿名态恢复上次输入（localStorage 记住 nick/email/link）
      try {
        $('#cmt-nick').value = localStorage.getItem('cmt_nick') || '';
        $('#cmt-email').value = localStorage.getItem('cmt_email') || '';
        $('#cmt-link').value = localStorage.getItem('cmt_link') || '';
      } catch (e) { /* noop */ }
    } else {
      // guest：隐藏身份行，按钮变「登录」（点击开弹窗，对齐参照截图 1）
      if (identity) identity.style.display = 'none';
      if (submit) submit.textContent = '登录';
      if (accountBtn) accountBtn.style.display = 'none';
    }
  }

  function defaultAvatar(name) {
    // SVG data URI：首字母渐变圆，不依赖外部服务
    var letter = String(name || '?').trim().charAt(0).toUpperCase();
    var hue = (letter.charCodeAt(0) || 0) * 37 % 360;
    var svg = '<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(' + hue + ',70%,60%)"/><stop offset="1" stop-color="hsl(' + ((hue + 40) % 360) + ',80%,45%)"/></linearGradient></defs><rect width="100%" height="100%" rx="40" fill="url(#g)"/><text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" font-family="-apple-system,Segoe UI,sans-serif" font-size="36" font-weight="700" fill="#fff">' + letter + '</text></svg>';
    return 'data:image/svg+xml;utf8,' + encodeURIComponent(svg);
  }

  function rememberAnon() {
    try {
      var n = $('#cmt-nick') && $('#cmt-nick').value;
      var e = $('#cmt-email') && $('#cmt-email').value;
      var l = $('#cmt-link') && $('#cmt-link').value;
      localStorage.setItem('cmt_nick', n || '');
      localStorage.setItem('cmt_email', e || '');
      localStorage.setItem('cmt_link', l || '');
    } catch (e) { /* noop */ }
  }

  // ------- 评论：渲染 -------
  function buildAvatarUrl(cmt) {
    if (cmt.avatar) return cmt.avatar;
    // 邮箱 -> 不访问 gravatar 隐私追踪，用首字母 SVG 兜底
    return defaultAvatar(cmt.nick || (cmt.email ? cmt.email.split('@')[0] : 'U'));
  }

  function renderBadges(cmt) {
    var html = '';
    if (cmt.is_admin) {
      html += '<span class="cmt-badge cmt-badge-admin" title="' + escapeHtml(state.adminBadge || '博主') + '">' +
        escapeHtml(state.adminBadge || '博主') + '</span>';
    }
    if (cmt.verified && !cmt.is_admin) {
      // 已验证徽章：Artalk 风格 icon-only 小徽标（对齐 atk-verified-icon）
      html += '<span class="cmt-verified-icon" title="邮箱已验证">' +
        '<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path fill-rule="evenodd" d="M9.5924 3.20027C9.34888 3.4078 9.22711 3.51158 9.09706 3.59874C8.79896 3.79854 8.46417 3.93721 8.1121 4.00672C7.95851 4.03705 7.79903 4.04977 7.48008 4.07522C6.6787 4.13918 6.278 4.17115 5.94371 4.28923C5.17051 4.56233 4.56233 5.17051 4.28923 5.94371C4.17115 6.278 4.13918 6.6787 4.07522 7.48008C4.04977 7.79903 4.03705 7.95851 4.00672 8.1121C3.93721 8.46417 3.79854 8.79896 3.59874 9.09706C3.51158 9.22711 3.40781 9.34887 3.20027 9.5924C2.67883 10.2043 2.4181 10.5102 2.26522 10.8301C1.91159 11.57 1.91159 12.43 2.26522 13.1699C2.41811 13.4898 2.67883 13.7957 3.20027 14.4076C3.40778 14.6511 3.51158 14.7729 3.59874 14.9029C3.79854 15.201 3.93721 15.5358 4.00672 15.8879C4.03705 16.0415 4.04977 16.201 4.07522 16.5199C4.13918 17.3213 4.17115 17.722 4.28923 18.0563C4.56233 18.8295 5.17051 19.4377 5.94371 19.7108C6.278 19.8288 6.6787 19.8608 7.48008 19.9248C7.79903 19.9502 7.95851 19.963 8.1121 19.9933C8.46417 20.0628 8.79896 20.2015 9.09706 20.4013C9.22711 20.4884 9.34887 20.5922 9.5924 20.7997C10.2043 21.3212 10.5102 21.5819 10.8301 21.7348C11.57 22.0884 12.43 22.0884 13.1699 21.7348C13.4898 21.5819 13.7957 21.3212 14.4076 20.7997C14.6511 20.5922 14.7729 20.4884 14.9029 20.4013C15.201 20.2015 15.5358 20.0628 15.8879 19.9933C16.0415 19.963 16.201 19.9502 16.5199 19.9248C17.3213 19.8608 17.722 19.8288 18.0563 19.7108C18.8295 19.4377 19.4377 18.8295 19.7108 18.0563C19.8288 17.722 19.8608 17.3213 19.9248 16.5199C19.9502 16.201 19.963 16.0415 19.9933 15.8879C20.0628 15.5358 20.2015 15.201 20.4013 14.9029C20.4884 14.7729 20.5922 14.6511 20.7997 14.4076C21.3212 13.7957 21.5819 13.4898 21.7348 13.1699C22.0884 12.43 22.0884 11.57 21.7348 10.8301C21.5819 10.5102 21.3212 10.2043 20.7997 9.5924C20.5922 9.34887 20.4884 9.22711 20.4013 9.09706C20.2015 8.79896 20.0628 8.46417 19.9933 8.1121C19.963 7.95851 19.9502 7.79903 19.9248 7.48008C19.8608 6.6787 19.8288 6.278 19.7108 5.94371C19.4377 5.17051 18.8295 4.56233 18.0563 4.28923C17.722 4.17115 17.3213 4.13918 16.5199 4.07522C16.201 4.04977 16.0415 4.03705 15.8879 4.00672C15.5358 3.93721 15.201 3.79854 14.9029 3.59874C14.7729 3.51158 14.6511 3.40781 14.4076 3.20027C13.7957 2.67883 13.4898 2.41811 13.1699 2.26522C12.43 1.91159 11.57 1.91159 10.8301 2.26522C10.5102 2.4181 10.2043 2.67883 9.5924 3.20027ZM16.3735 9.86314C16.6913 9.5453 16.6913 9.03 16.3735 8.71216C16.0557 8.39433 15.5403 8.39433 15.2225 8.71216L10.3723 13.5624L8.77746 11.9676C8.45963 11.6498 7.94432 11.6498 7.62649 11.9676C7.30866 12.2854 7.30866 12.8007 7.62649 13.1186L9.79678 15.2889C10.1146 15.6067 10.6299 15.6067 10.9478 15.2889L16.3735 9.86314Z" clip-rule="evenodd"/></svg></span>';
    }
    return html;
  }

  function renderReplyRef(value) {
    var name = String(value || '').replace(/^\s*▸\s*/, '').trim();
    if (!name) return '';
    return '<span class="cmt-reply-ref"><span aria-hidden="true">▸</span> ' + escapeHtml(name) + '</span>';
  }

  function renderBoostButton(cmt, action) {
    var deleting = action === 'delete';
    var unavailable = !deleting && cmt.can_boost === false;
    var label = deleting ? '删除我的回应' : (unavailable ? '暂时不能回应这条评论' : '回应这条评论');
    return '<button type="button" class="cmt-action-btn cmt-boost-btn' + (deleting ? ' is-mine' : '') + '"' +
      ' data-id="' + cmt.id + '" data-boost-action="' + action + '"' +
      ' data-boost-available="' + (unavailable ? '0' : '1') + '"' +
      ' title="' + label + '" aria-label="' + label + '">' +
      // 实心火箭（Font Awesome Free 6 solid rocket，CC BY 4.0），斜向右上对齐参照图
      '<svg viewBox="0 0 512 512" width="16" height="16" fill="currentColor" aria-hidden="true">' +
        '<path d="M156.6 384.9L125.7 354c-8.5-8.5-11.5-20.8-7.7-32.2c3-8.9 7-20.5 11.8-33.8L24 288c-8.6 0-16.6-4.6-20.9-12.1s-4.2-16.7 .2-24.1l52.5-88.5c13-21.9 36.5-35.3 61.9-35.3l82.3 0c2.4-4 4.8-7.7 7.2-11.3C289.1-4.1 411.1-8.1 483.9 5.3c11.6 2.1 20.6 11.2 22.8 22.8c13.4 72.9 9.3 194.8-111.4 276.7c-3.5 2.4-7.3 4.8-11.3 7.2v82.3c0 25.4-13.4 49-35.3 61.9l-88.5 52.5c-7.4 4.4-16.6 4.5-24.1 .2s-12.1-12.2-12.1-20.9V380.8c-14.1 4.9-26.4 8.9-35.7 11.9c-11.2 3.6-23.4 .5-31.8-7.8zM384 168a40 40 0 1 0 0-80 40 40 0 1 0 0 80z"/>' +
      '</svg></button>';
  }

  function renderBoostStrip(cmt) {
    var boosts = Array.isArray(cmt.boosts) ? cmt.boosts : [];
    if (!boosts.length) return '';
    var mine = false;
    var html = '<div class="cmt-boost-strip" data-count="' + boosts.length + '">';
    html += boosts.map(function (boost) {
      if (boost && boost.mine) mine = true;
      var chip = '<button type="button" class="cmt-boost-chip' + (boost && boost.mine ? ' is-mine' : '') + '"' +
        (boost && boost.mine ? ' data-boost-mine="1" data-id="' + cmt.id + '" title="删除我的回应"' : '') + '>' +
        '<img src="' + escapeHtml(boost && boost.avatar || '') + '" alt="' + escapeHtml(boost && boost.nick || '') + '" loading="lazy">' +
        '<span class="cmt-boost-emoji" aria-hidden="true">' + escapeHtml(boost && boost.emoji || '') + '</span>' +
        (boost && boost.text ? '<span class="cmt-boost-text">' + escapeHtml(boost.text) + '</span>' : '') +
        '</button>';
      return chip;
    }).join('');
    if (mine) html += renderBoostButton(cmt, 'delete');
    html += '</div>';
    return html;
  }

  function renderCommentItem(cmt) {
    var nickHtml = cmt.link
      ? '<a href="' + escapeHtml(cmt.link) + '" rel="nofollow noopener" target="_blank">' + escapeHtml(cmt.nick) + '</a>'
      : escapeHtml(cmt.nick);
    var avatar = buildAvatarUrl(cmt);
    return '<article class="cmt-item" data-id="' + cmt.id + '" data-nick="' + escapeHtml(cmt.nick) + '">' +
      '<img class="cmt-item-avatar" src="' + avatar + '" alt="' + escapeHtml(cmt.nick) + '" loading="lazy">' +
      '<div class="cmt-item-main">' +
        '<div class="cmt-item-head">' +
          '<span class="cmt-item-author"><span class="cmt-item-nick">' + nickHtml + '</span>' + renderBadges(cmt) + '</span>' +
          renderReplyRef(cmt.reply_ref) +
          '<span class="cmt-item-time">' + escapeHtml(cmt.time_ago || cmt.date_created || '') + '</span>' +
        '</div>' +
        '<div class="cmt-item-body">' + (cmt.content_html || '') + '</div>' +
        renderBoostStrip(cmt) +
        '<div class="cmt-item-actions">' +
          '<button type="button" class="cmt-action-btn cmt-reply-btn" data-id="' + cmt.id + '" data-nick="' + escapeHtml(cmt.nick) + '">' +
            '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/></svg>' +
            '<span>回复</span>' +
          '</button>' +
          '<button type="button" class="cmt-action-btn cmt-like-btn' + (cmt.liked ? ' is-liked' : '') + '" data-id="' + cmt.id + '" data-liked="' + (cmt.liked ? '1' : '0') + '">' +
            '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>' +
            '<span class="cmt-like-count">' + (cmt.likes || 0) + '</span>' +
          '</button>' +
          (!((cmt.boosts || []).some(function (boost) { return boost && boost.mine; })) ? renderBoostButton(cmt, 'add') : '') +
        '</div>' +
        (cmt.children && cmt.children.length ? '<div class="cmt-children">' + cmt.children.map(renderCommentItem).join('') + '</div>' : '') +
      '</div>' +
    '</article>';
  }

  // ------- 评论排序：newest(最新)/hottest(最热)/oldest(最早)/author(作者) -------
  function sortComments(list) {
    var arr = list.slice();
    if (state.sortMode === 'newest') {
      arr.sort(function (a, b) { return b.id - a.id; });
    } else if (state.sortMode === 'oldest') {
      arr.sort(function (a, b) { return a.id - b.id; });
    } else if (state.sortMode === 'hottest') {
      arr.sort(function (a, b) {
        var la = a.likes || 0, lb = b.likes || 0;
        if (lb !== la) return lb - la;
        return b.id - a.id; // 点赞相同则最新在前
      });
    } else if (state.sortMode === 'author') {
      // 仅显示博主/作者评论
      arr = arr.filter(function (c) { return c.is_admin; });
      arr.sort(function (a, b) { return a.id - b.id; });
    }
    return arr;
  }

  function renderComments(data) {
    var listEl = $('#cmt-list');
    var listHead = $('#cmt-list-head');
    var emptyEl = $('#cmt-empty');
    var countBadge = $('#cmt-count-badge');
    if (!listEl) return;

    state.comments = data && data.comments ? data.comments : [];
    var total = data && typeof data.total === 'number' ? data.total : state.comments.length;

    listEl.innerHTML = '';

    if (total > 0) {
      listHead.style.display = 'flex';
      listHead.querySelector('.cmt-list-title').textContent = total + ' 条评论';
      emptyEl.style.display = 'none';
      listEl.innerHTML = sortComments(state.comments).map(renderCommentItem).join('');
      maybeScrollToHashComment();
    } else {
      listHead.style.display = 'none';
      emptyEl.style.display = 'flex';
    }

    if (countBadge) countBadge.textContent = total > 0 ? ('共 ' + total + ' 条评论') : '0';
  }

  // 深链支持：URL 形如 /post/xxx#comment-123 时，评论懒加载完成后滚动并高亮对应评论
  function parseHashCommentId() {
    var loc = window.location;
    if (!loc) return 0;
    var m = /(?:#|\/)comment-(\d+)\s*$/.exec(loc.hash || '');
    return m ? parseInt(m[1], 10) : 0;
  }

  function maybeScrollToHashComment() {
    var id = parseHashCommentId();
    if (!id) return;
    var target = document.querySelector('.cmt-item[data-id="' + id + '"]');
    if (!target) return;
    target.scrollIntoView({ block: 'center', behavior: 'smooth' });
    target.classList.add('cmt-item-highlight');
    setTimeout(function () { target.classList.remove('cmt-item-highlight'); }, 2500);
  }

  function rerenderSorted() {
    var listEl = $('#cmt-list');
    if (!listEl || !state.comments.length) return;
    var sorted = sortComments(state.comments);
    if (state.sortMode === 'author' && !sorted.length) {
      // 作者筛选无结果：显示提示
      listEl.innerHTML = '<div class="cmt-empty-author">暂无作者评论</div>';
      return;
    }
    listEl.innerHTML = sorted.map(renderCommentItem).join('');
  }

  function insertNewComment(cmt) {
    var listEl = $('#cmt-list');
    var listHead = $('#cmt-list-head');
    var emptyEl = $('#cmt-empty');
    var countBadge = $('#cmt-count-badge');
    if (!listEl) return;

    var total = 0;
    if (cmt.parent_id > 0) {
      // 回复：挂到父评论的 .cmt-children 下
      var parent = $('.cmt-item[data-id="' + cmt.parent_id + '"]');
      if (parent) {
        var main = $('.cmt-item-main', parent);
        var childrenBox = $('.cmt-children', main);
        if (!childrenBox) {
          childrenBox = document.createElement('div');
          childrenBox.className = 'cmt-children';
          main.appendChild(childrenBox);
        }
        var frag = document.createElement('template');
        cmt.reply_ref = parent.getAttribute('data-nick') || '';
        frag.innerHTML = renderCommentItem(cmt);
        childrenBox.appendChild(frag.content.firstElementChild);
        if (listHead && listHead.querySelector('.cmt-list-title')) {
          var m = listHead.querySelector('.cmt-list-title').textContent.match(/\d+/);
          total = parseInt((m && m[0]) || '0', 10) + 1;
          listHead.querySelector('.cmt-list-title').textContent = total + ' 条评论';
        }
      }
    } else {
      emptyEl.style.display = 'none';
      listHead.style.display = 'flex';
      listHead.querySelector('.cmt-list-title').textContent = '1 条评论';
      listEl.insertAdjacentHTML('afterbegin', renderCommentItem(cmt));
      total = 1;
    }

    // 更新顶部徽章
    if (countBadge) {
      var cur = countBadge.textContent.match(/\d+/);
      var curN = parseInt((cur && cur[0]) || '0', 10);
      countBadge.textContent = '共 ' + Math.max(curN, total > 0 ? total : 0) + ' 条评论';
    }
  }

  // ------- 懒加载评论：IntersectionObserver 触发 -------
  function maybeLoadComments() {
    if (state.loadingComments) return;
    if (state.pageKey == null || state.pageKey === '') return;
    state.loadingComments = true;
    fetch('/api/comments?page_key=' + encodeURIComponent(state.pageKey), { credentials: 'same-origin' })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok || !res.data || !res.data.ok) {
          throw new Error(res.data && res.data.error ? res.data.error : '加载评论失败');
        }
        renderComments(res.data);
      })
      .catch(function (err) {
        var listEl = $('#cmt-list');
        if (listEl) listEl.innerHTML = '<div style="padding:1.5rem 0;color:#ef4444;font-size:.82rem;">加载评论失败：' + escapeHtml(err && err.message ? err.message : '网络错误') + '</div>';
      });
  }

  function observeLazy() {
    var body = $('#cmt-body');
    if (!body) return;
    state.pageKey = body.getAttribute('data-page-key') || '';
    state.adminBadge = body.getAttribute('data-admin-badge') || '';
    if (!('IntersectionObserver' in window)) {
      maybeLoadComments();
      return;
    }
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) {
          io.disconnect();
          maybeLoadComments();
        }
      });
    }, { rootMargin: '200px' });
    io.observe(body);
  }

  // ------- 提交评论 -------
  function doSubmit() {
    rememberAnon();
    var contentEl = $('#cmt-content');
    var content = contentEl && contentEl.value ? contentEl.value.trim() : '';
    if (!content) { toast('评论内容不能为空', 'error'); if (contentEl) contentEl.focus(); return; }

    // 校验匿名态必填
    var nick, email, link;
    if (state.mode === 'logged-in') {
      nick = (state.me && state.me.nickname) || (state.me && state.me.username) || '';
      email = (state.me && state.me.email) || '';
      link = '';
    } else {
      nick = $('#cmt-nick') && $('#cmt-nick').value.trim();
      email = $('#cmt-email') && $('#cmt-email').value.trim();
      link = $('#cmt-link') && $('#cmt-link').value.trim();
      if (!nick) { toast('请填写昵称', 'error'); return; }
      if (!email || email.indexOf('@') < 0) { toast('请填写正确的邮箱', 'error'); return; }
    }

    var website = $('#cmt-website') && $('#cmt-website').value;
    var body = {
      page_key: state.pageKey,
      nick: nick,
      email: email,
      link: link,
      content: content,
      parent_id: state.parentId || 0,
      website: website || ''
    };

    var submitBtn = $('#cmt-submit');
    if (submitBtn) { submitBtn.disabled = true; var prevHtml = submitBtn.innerHTML; submitBtn.innerHTML = '<span>提交中…</span>'; }

    fetch('/api/comments', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then(function (r) { return r.json().then(function (d) { return { status: r.status, data: d }; }); })
      .then(function (res) {
        if (!res.data || !res.data.ok) {
          throw new Error(res.data && res.data.error ? res.data.error : '发布失败');
        }
        // 清空输入
        if (contentEl) contentEl.value = '';
        cancelReply();
        if (res.data.status === 'approved' && res.data.comment) {
          insertNewComment(res.data.comment);
          toast('发布成功', 'success');
        } else {
          toast('评论提交成功，审核通过后显示', 'success');
        }
      })
      .catch(function (err) {
        toast(err && err.message ? err.message : '发布失败', 'error');
      })
      .then(function () {
        if (submitBtn) { submitBtn.disabled = false; submitBtn.innerHTML = prevHtml || submitBtn.innerHTML; }
      });
  }

  // ------- 回复态 -------
  function startReply(id, nick) {
    state.parentId = id;
    var tip = $('#cmt-reply-tip');
    var nickB = $('#cmt-reply-nick');
    if (nickB) nickB.textContent = nick || '';
    if (tip) tip.style.display = 'flex';
    var ta = $('#cmt-content');
    if (ta) { ta.scrollIntoView({ block: 'center', behavior: 'smooth' }); ta.focus(); }
  }
  function cancelReply() {
    state.parentId = 0;
    var tip = $('#cmt-reply-tip');
    if (tip) tip.style.display = 'none';
  }

  // ------- 评论 Boost：单行胶囊 + 本地 Unicode 表情选择器 -------
  function closeBoostPanel() {
    var panel = $('.cmt-boost-popover');
    if (panel && panel.parentNode) panel.parentNode.removeChild(panel);
  }

  function positionBoostPanel(panel, anchor) {
    if (!panel || !anchor) return;
    var rect = anchor.getBoundingClientRect();
    var width = panel.offsetWidth || 520;
    var left = Math.max(8, Math.min(window.innerWidth - width - 8, rect.left));
    var top = rect.bottom + 8;
    if (top + panel.offsetHeight > window.innerHeight - 8) top = Math.max(8, rect.top - panel.offsetHeight - 8);
    panel.style.left = left + 'px';
    panel.style.top = top + 'px';
  }

  function boostRecentEmojis() {
    try {
      var parsed = JSON.parse(localStorage.getItem(BOOST_EMOJI_RECENT_KEY) || '[]');
      return Array.isArray(parsed) ? parsed.filter(function (item) { return typeof item === 'string' && item; }).slice(0, 24) : [];
    } catch (e) { return []; }
  }

  function rememberBoostEmoji(emoji) {
    var recent = boostRecentEmojis().filter(function (item) { return item !== emoji; });
    recent.unshift(emoji);
    try { localStorage.setItem(BOOST_EMOJI_RECENT_KEY, JSON.stringify(recent.slice(0, 24))); } catch (e) { /* noop */ }
  }

  function clearBoostRecent() {
    try { localStorage.removeItem(BOOST_EMOJI_RECENT_KEY); } catch (e) { /* noop */ }
  }

  function loadBoostEmojiData() {
    if (state.boostEmojiItems) return Promise.resolve(state.boostEmojiItems);
    if (state.boostEmojiPromise) return state.boostEmojiPromise;
    state.boostEmojiPromise = fetch(BOOST_EMOJI_DATA_URL, { credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok) throw new Error('emoji data unavailable');
        return r.json();
      })
      .then(function (data) {
        if (!data || !Array.isArray(data.emojis) || !data.emojis.length) throw new Error('emoji data invalid');
        state.boostEmojiItems = data.emojis.filter(function (row) {
          return Array.isArray(row) && typeof row[0] === 'string' && row[0] && Number.isInteger(row[1]);
        });
        return state.boostEmojiItems;
      })
      .catch(function () {
        state.boostEmojiItems = BOOST_EMOJI_FALLBACK.slice();
        return state.boostEmojiItems;
      });
    return state.boostEmojiPromise;
  }

  function boostGroupByID(id) {
    return BOOST_EMOJI_GROUPS.find(function (group) { return group.id === String(id); }) || BOOST_EMOJI_GROUPS[0];
  }

  function filterBoostEmojis(items, category, query, recent) {
    var normalizedQuery = String(query || '').trim().toLocaleLowerCase('zh-CN');
    var all = Array.isArray(items) && items.length ? items : BOOST_EMOJI_FALLBACK;
    var result;
    var title;
    if (normalizedQuery) {
      result = all.filter(function (row) {
        return row[0].includes(normalizedQuery) || String(row[2] || '').toLocaleLowerCase('zh-CN').includes(normalizedQuery) || String(row[3] || '').includes(normalizedQuery);
      });
      title = '搜索结果';
    } else if (category === 'recent') {
      result = (recent || []).map(function (emoji) {
        return all.find(function (row) { return row[0] === emoji; }) || [emoji, -1, emoji, emoji];
      });
      if (!result.length) result = BOOST_EMOJI_FALLBACK.slice(0, 8);
      title = '常用';
    } else {
      result = all.filter(function (row) { return String(row[1]) === String(category); });
      title = boostGroupByID(category).label;
    }
    return { items: result, title: title, query: normalizedQuery };
  }

  function renderBoostPicker(panel, searchValue) {
    if (!panel) return;
    var picker = panel.querySelector('.cmt-boost-picker');
    var grid = panel.querySelector('.cmt-boost-picker-grid');
    var title = panel.querySelector('.cmt-boost-picker-title');
    if (!picker || !grid || !title) return;
    var query = String(searchValue == null ? (panel.querySelector('.cmt-boost-search') || {}).value || '' : searchValue).trim().toLocaleLowerCase('zh-CN');
    var category = panel.getAttribute('data-category') || 'recent';
    var all = state.boostEmojiItems || BOOST_EMOJI_FALLBACK;
    var recent = boostRecentEmojis();
    var filtered = filterBoostEmojis(all, category, query, recent);
    var items = filtered.items;
    title.textContent = filtered.title;

    $$('.cmt-boost-category', panel).forEach(function (button) {
      var active = !query && button.getAttribute('data-category') === category;
      button.classList.toggle('is-active', active);
      button.setAttribute('aria-selected', active ? 'true' : 'false');
    });
    var clear = panel.querySelector('.cmt-boost-clear-recent');
    if (clear) clear.hidden = query || category !== 'recent' || !recent.length;
    grid.innerHTML = items.length ? items.map(function (row) {
      return '<button type="button" class="cmt-boost-emoji-option" data-emoji="' + escapeHtml(row[0]) + '" title="' + escapeHtml(row[2] || row[0]) + '" aria-label="' + escapeHtml(row[2] || row[0]) + '">' + escapeHtml(row[0]) + '</button>';
    }).join('') : '<div class="cmt-boost-picker-empty">没有匹配的表情</div>';
    positionBoostPanel(panel, panel._boostAnchor);
  }

  function boostComposerHTML(nickname, avatar) {
    return '<div class="cmt-boost-bar">' +
      '<img class="cmt-boost-avatar" src="' + escapeHtml(avatar) + '" alt="">' +
      '<input type="text" class="cmt-boost-input" maxlength="16" placeholder="Boost ' + escapeHtml(nickname) + '..." aria-label="回应文字">' +
      '<button type="button" class="cmt-boost-emoji-toggle" aria-label="选择表情" aria-expanded="false">' +
        '<svg viewBox="0 0 24 24" width="21" height="21" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
          '<circle cx="12" cy="12" r="9"/>' +
          '<path d="M8.4 14.1a4.6 4.6 0 0 0 7.2 0"/>' +
          '<circle cx="9" cy="9.4" r="1.15" fill="currentColor" stroke="none"/>' +
          '<circle cx="15" cy="9.4" r="1.15" fill="currentColor" stroke="none"/>' +
        '</svg></button>' +
      '<button type="button" class="cmt-boost-confirm" aria-label="确认回应" title="确认">' +
        '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
          '<circle cx="12" cy="12" r="9.2" stroke-width="1.8"/>' +
          '<path d="m8.1 12.5 2.7 2.7 5.1-5.6" stroke-width="2.4"/>' +
        '</svg></button>' +
      '<button type="button" class="cmt-boost-cancel" aria-label="取消回应" title="取消">' +
        '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-linecap="round" aria-hidden="true">' +
          '<circle cx="12" cy="12" r="9.2" stroke-width="1.8"/>' +
          '<path d="M9.1 9.1l5.8 5.8M14.9 9.1l-5.8 5.8" stroke-width="2.4"/>' +
        '</svg></button>' +
      '</div>' +
      '<div class="cmt-boost-picker" hidden>' +
        '<div class="cmt-boost-picker-head">' +
          '<label class="cmt-boost-search-wrap"><span aria-hidden="true">' +
            '<svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><line x1="20" y1="20" x2="16.2" y2="16.2"/></svg>' +
          '</span><input type="search" class="cmt-boost-search" placeholder="按表情符号名称和别名搜索..." aria-label="搜索表情"></label>' +
          '<span class="cmt-boost-selected-emoji" aria-hidden="true">👍</span>' +
        '</div>' +
        '<div class="cmt-boost-picker-body">' +
          '<div class="cmt-boost-rail">' +
            '<div class="cmt-boost-categories" role="tablist" aria-label="表情分类">' + BOOST_EMOJI_GROUPS.map(function (group, index) {
              return '<button type="button" class="cmt-boost-category' + (index === 0 ? ' is-active' : '') + '" data-category="' + group.id + '" role="tab" aria-selected="' + (index === 0 ? 'true' : 'false') + '" title="' + group.label + '"><span aria-hidden="true">' + group.icon + '</span></button>';
            }).join('') + '</div>' +
            '<button type="button" class="cmt-boost-category-more" aria-label="更多分类">' +
              '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="6 9 12 15 18 9"/></svg>' +
            '</button>' +
          '</div>' +
          '<div class="cmt-boost-picker-content"><div class="cmt-boost-picker-heading"><span class="cmt-boost-picker-title">常用</span><button type="button" class="cmt-boost-clear-recent" title="清空常用" aria-label="清空常用" hidden>⌫</button></div><div class="cmt-boost-picker-grid" role="listbox" aria-label="表情"></div></div>' +
        '</div>' +
      '</div>';
  }

  function selectBoostEmoji(panel, emoji) {
    if (!panel || !emoji) return;
    panel.setAttribute('data-emoji', emoji);
    var selected = panel.querySelector('.cmt-boost-selected-emoji');
    if (selected) selected.textContent = emoji;
    rememberBoostEmoji(emoji);
    var picker = panel.querySelector('.cmt-boost-picker');
    var toggle = panel.querySelector('.cmt-boost-emoji-toggle');
    if (picker) picker.hidden = true;
    if (toggle) toggle.setAttribute('aria-expanded', 'false');
    var input = panel.querySelector('.cmt-boost-input');
    if (input) input.focus();
    positionBoostPanel(panel, panel._boostAnchor);
  }

  function openBoostPanel(btn) {
    if (state.mode !== 'logged-in') {
      toast('Boost 需要登录后使用', 'error');
      openModal();
      return null;
    }
    if (btn.getAttribute('data-boost-available') === '0') {
      toast('当前不能回应这条评论', 'error');
      return null;
    }
    closeBoostPanel();
    var identity = boostPayload();
    var nickname = identity.nick || '访客';
    var avatar = state.mode === 'logged-in' && state.me && state.me.avatar ? state.me.avatar : defaultAvatar(nickname);
    var panel = document.createElement('div');
    panel.className = 'cmt-boost-popover';
    panel.setAttribute('role', 'dialog');
    panel.setAttribute('aria-label', '回应评论');
    panel.setAttribute('data-id', btn.getAttribute('data-id') || '');
    panel.setAttribute('data-emoji', '👍');
    panel.setAttribute('data-category', 'recent');
    panel.innerHTML = boostComposerHTML(nickname, avatar);
    document.body.appendChild(panel);
    panel._boostAnchor = btn;
    renderBoostPicker(panel, '');
    positionBoostPanel(panel, btn);
    panel.querySelector('.cmt-boost-input').focus();
    return panel;
  }

  function boostPayload() {
    var nick = '', email = '';
    if (state.mode === 'logged-in') {
      nick = (state.me && (state.me.nickname || state.me.username)) || '';
      email = (state.me && state.me.email) || '';
    } else {
      try {
        nick = localStorage.getItem('cmt_nick') || '';
        email = localStorage.getItem('cmt_email') || '';
      } catch (e) { /* noop */ }
    }
    return { nick: nick, email: email };
  }

  function findComment(list, id) {
    for (var i = 0; i < list.length; i++) {
      if (String(list[i].id) === String(id)) return list[i];
      var found = findComment(list[i].children || [], id);
      if (found) return found;
    }
    return null;
  }

  function refreshBoostComment(id, boost) {
    var cmt = findComment(state.comments, id);
    if (!cmt) return;
    cmt.boosts = Array.isArray(cmt.boosts) ? cmt.boosts : [];
    var index = cmt.boosts.findIndex(function (item) { return item && item.mine; });
    if (index >= 0) cmt.boosts[index] = boost;
    else cmt.boosts.push(boost);
    cmt.boost_count = cmt.boosts.length;
    if (cmt.boosts.length >= 50) cmt.can_boost = false;
    rerenderSorted();
  }

  function removeBoostComment(id) {
    var cmt = findComment(state.comments, id);
    if (!cmt) return;
    cmt.boosts = (cmt.boosts || []).filter(function (boost) { return !boost || !boost.mine; });
    cmt.boost_count = cmt.boosts.length;
    if (cmt.boosts.length < 50 && !cmt.is_author) cmt.can_boost = true;
    rerenderSorted();
  }

  function doBoost(id, panel) {
    var input = panel.querySelector('.cmt-boost-input');
    var send = panel.querySelector('.cmt-boost-confirm');
    var emoji = panel.getAttribute('data-emoji');
    if (!emoji) return;
    if (send) send.disabled = true;
    var body = boostPayload();
    body.emoji = emoji;
    body.text = input ? input.value.trim() : '';
    fetch('/api/comments/' + encodeURIComponent(id) + '/boost', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    }).then(function (r) { return r.json().then(function (d) { return { ok: r.ok, status: r.status, data: d }; }); })
      .then(function (res) {
        if (res.status === 401) {
          closeBoostPanel();
          toast('Boost 需要登录后使用', 'error');
          openModal();
          return;
        }
        if (!res.ok || !res.data || !res.data.ok) throw new Error(res.data && res.data.error ? res.data.error : '回应失败');
        refreshBoostComment(id, res.data.boost);
        closeBoostPanel();
        toast('回应成功', 'success');
      }).catch(function (err) { toast(err && err.message ? err.message : '回应失败', 'error'); if (send) send.disabled = false; });
  }

  function doUnboost(id) {
    fetch('/api/comments/' + encodeURIComponent(id) + '/boost', { method: 'DELETE', credentials: 'same-origin' })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok || !res.data || !res.data.ok) throw new Error(res.data && res.data.error ? res.data.error : '删除回应失败');
        removeBoostComment(id);
        toast('已删除回应', 'success');
      }).catch(function (err) { toast(err && err.message ? err.message : '删除回应失败', 'error'); });
  }

  // ------- 表情面板 -------
  function toggleEmojiPanel() {
    var p = $('#cmt-emoji-panel');
    if (!p) return;
    var open = p.style.display !== 'none' && p.style.display !== '';
    p.style.display = open ? 'none' : 'flex';
    if (!open) {
      // 首次打开：懒加载默认 pack
      ensurePackLoaded(state.activePack);
    }
  }

  function switchEmojiPack(pack) {
    state.activePack = pack;
    $$('.cmt-emoji-tab').forEach(function (b) {
      b.classList.toggle('is-active', b.getAttribute('data-pack') === pack);
      b.setAttribute('aria-selected', b.getAttribute('data-pack') === pack ? 'true' : 'false');
    });
    ensurePackLoaded(pack);
  }

  function ensurePackLoaded(pack) {
    var cached = state.packItems[pack];
    if (cached) { renderEmojiGrid(pack, cached); return; }
    var body = $('#cmt-emoji-body');
    if (!body) return;
    body.innerHTML = '<div class="cmt-emoji-loading">' +
      '<svg class="loading" viewBox="0 0 1024 1024" width="1em" height="1em" aria-hidden="true"><path fill="currentColor" d="M512 1024a512 512 0 1 1 512-512h-96a416 416 0 1 0-416 416z"/></svg>' +
      '<span>加载中…</span></div>';

    // 拉取 Artalk 格式清单 <base>/<pack>.json（items: [{key, val}]，val 为图片直链）
    fetch(EMOJI_BASE + pack + '.json', { cache: 'force-cache' })
      .then(function (r) {
        if (r.ok) return r.json();
        throw new Error('http ' + r.status);
      })
      .then(function (data) {
        var items = (data && Array.isArray(data.items)) ? data.items : null;
        if (!items || !items.length) throw new Error('empty');
        state.packItems[pack] = items;
        state.packLoaded[pack] = true;
        renderEmojiGrid(pack, items);
      })
      .catch(function () {
        // 加载失败：当前仍停留在此 tab 时给出提示，允许重新点开重试
        if (state.activePack === pack && body && !state.packLoaded[pack]) {
          body.innerHTML = '<div class="cmt-emoji-error">表情包加载失败，请检查网络后重试</div>';
        }
      });
  }

  function renderEmojiGrid(pack, items) {
    // 只在当前激活 tab 仍为这个 pack 时渲染（避免切 tab 期间闪回）
    if (state.activePack !== pack) return;
    var body = $('#cmt-emoji-body');
    if (!body) return;
    var html = '<div class="cmt-emoji-grid">' + items.map(function (it) {
      var u = it && it.val;
      if (!u) return '';
      var key = (it && it.key) || '';
      return '<button type="button" class="cmt-emoji-cell" data-src="' + escapeHtml(u) + '" title="' + escapeHtml(key) + '">' +
        '<img class="cmt-emoji-img" src="' + escapeHtml(u) + '" alt="' + escapeHtml(key) + '" loading="lazy"></button>';
    }).join('') + '</div>';
    body.innerHTML = html;
  }

  function insertEmojiAtCursor(src) {
    // 插入 Markdown 图片形式，后端 bluemonday 放行 img.cmt-emoji-img + 指定 CDN
    var md = '![emoji](' + src + ')';
    var ta = $('#cmt-content');
    if (ta) insertAtCursor(ta, md);
  }

  // ------- Markdown 预览（M↓ 按钮开关，对齐参照截图 4：上半原文下半渲染） -------
  function safeUrl(u) { return /^(https?:\/\/|\/)/i.test(u); }

  // 行内格式：先转义再套格式，防 XSS；仅支持 图片/链接/粗体/斜体/行内代码
  function mdInline(s) {
    var codes = [];
    s = escapeHtml(s).replace(/`([^`]+)`/g, function (m, c) {
      codes.push('<code>' + c + '</code>');
      return '\u0000C' + (codes.length - 1) + '\u0000';
    });
    s = s.replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, function (m, alt, u) {
      return safeUrl(u) ? '<img class="cmt-emoji-img" src="' + u + '" alt="' + alt + '">' : m;
    });
    s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, function (m, t, u) {
      return safeUrl(u) ? '<a href="' + u + '" rel="nofollow noopener" target="_blank">' + t + '</a>' : m;
    });
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    s = s.replace(/\u0000C(\d+)\u0000/g, function (m, i) { return codes[+i]; });
    return s;
  }

  // 客户端最小 Markdown 子集（预览用，提交后以后端渲染为准）：
  // 标题/引用/代码块/段落 + 行内格式；标题降一级输出（评论区正文层级）
  function mdToHtml(src) {
    var text = String(src || '');
    if (!text.trim()) return '';
    var blocks = [];
    text = text.replace(/```([\s\S]*?)```/g, function (m, code) {
      blocks.push('<pre><code>' + escapeHtml(code.replace(/^\n+|\n+$/g, '')) + '</code></pre>');
      return '\u0000B' + (blocks.length - 1) + '\u0000';
    });
    var html = [];
    var para = [];
    var quote = [];
    function flushPara() { if (para.length) { html.push('<p>' + mdInline(para.join(' ')) + '</p>'); para = []; } }
    function flushQuote() { if (quote.length) { html.push('<blockquote>' + quote.map(mdInline).join('<br>') + '</blockquote>'); quote = []; } }
    text.split('\n').forEach(function (line) {
      var ph = line.match(/^\u0000B(\d+)\u0000$/);
      if (ph) { flushPara(); flushQuote(); html.push(blocks[+ph[1]]); return; }
      var h = line.match(/^(#{1,6})\s+(.*)$/);
      if (h) {
        flushPara(); flushQuote();
        var lv = Math.min(h[1].length + 1, 6);
        html.push('<h' + lv + '>' + mdInline(h[2]) + '</h' + lv + '>');
        return;
      }
      if (/^>\s?/.test(line)) { flushPara(); quote.push(line.replace(/^>\s?/, '')); return; }
      if (!line.trim()) { flushPara(); flushQuote(); return; }
      flushQuote();
      para.push(line);
    });
    flushPara(); flushQuote();
    return html.join('');
  }

  function renderPreview() {
    var p = $('#cmt-preview');
    var ta = $('#cmt-content');
    if (!p || !ta) return;
    var html = mdToHtml(ta.value);
    p.innerHTML = html ? '<div class="cmt-preview-body">' + html + '</div>' : '<div class="cmt-preview-empty">（无内容）</div>';
  }

  function togglePreview() {
    var p = $('#cmt-preview');
    if (!p) return;
    var open = p.style.display !== 'none' && p.style.display !== '';
    p.style.display = open ? 'none' : 'block';
    if (!open) renderPreview();
  }

  // 评论图片上传：5MB 限制与后端一致；成功后光标处插入 Markdown 图片语法
  function uploadCommentImage(file) {
    if (file.size > 5 * 1024 * 1024) { toast('图片不能超过 5MB', 'error'); return; }
    var btn = $('#cmt-tool-image');
    if (btn) btn.disabled = true;
    var fd = new FormData();
    fd.append('file', file);
    fetch('/api/comments/upload', { method: 'POST', credentials: 'same-origin', body: fd })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok || !res.data || !res.data.ok || !res.data.url) {
          throw new Error(res.data && res.data.error ? res.data.error : '上传失败');
        }
        var ta = $('#cmt-content');
        if (ta) insertAtCursor(ta, '\n![image](' + res.data.url + ')\n');
        toast('图片已插入', 'success');
      })
      .catch(function (err) { toast(err && err.message ? err.message : '上传失败', 'error'); })
      .finally(function () { if (btn) btn.disabled = false; });
  }

  // ------- 工具栏按钮：内容支持 Markdown，仅保留表情 + 图片上传 -------
  function bindToolbar() {
    var ta = $('#cmt-content');
    if (!ta) return;

    // 图片上传：选图后 POST /api/comments/upload，成功在光标处插入 Markdown 图片
    var fileInput = null;
    on($('#cmt-tool-image'), 'click', function () {
      if (!fileInput) {
        fileInput = document.createElement('input');
        fileInput.type = 'file';
        fileInput.accept = 'image/png,image/jpeg,image/gif,image/webp';
        fileInput.style.display = 'none';
        document.body.appendChild(fileInput);
        listen(fileInput, 'change', function () {
          if (fileInput.files && fileInput.files[0]) uploadCommentImage(fileInput.files[0]);
          fileInput.value = '';
        });
      }
      fileInput.click();
    });

    // Markdown 预览开关（M↓：输入区下半展示渲染结果）
    on($('#cmt-tool-md'), 'click', function (e) {
      e.stopPropagation();
      togglePreview();
    });
    // 预览打开时，输入实时重渲染
    on(ta, 'input', function () {
      var p = $('#cmt-preview');
      if (p && p.style.display !== 'none' && p.style.display !== '') renderPreview();
    });
    // 表情按钮
    on($('#cmt-emoji-btn'), 'click', function (e) { e.stopPropagation(); toggleEmojiPanel(); });
    // 点击表情面板外关闭
    on(document, 'click', function (e) {
      var panel = $('#cmt-emoji-panel');
      var btn = $('#cmt-emoji-btn');
      if (panel && panel.style.display !== 'none') {
        if (!(panel.contains(e.target) || (btn && btn.contains(e.target)))) panel.style.display = 'none';
      }
    });
  }

  // ------- 登录弹窗：首屏二选一（Email / 跳过验证），选 Email 才进登录注册表单 -------
  function openModal() {
    var m = $('#cmt-modal');
    if (!m) return;
    m.style.display = 'flex';
    var c = $('#cmt-modal-chooser');
    var a = $('#cmt-modal-auth');
    if (c) c.style.display = 'flex';
    if (a) a.style.display = 'none';
  }
  function closeModal() {
    var m = $('#cmt-modal');
    if (m) m.style.display = 'none';
  }
  function showAuth(mode) {
    var c = $('#cmt-modal-chooser');
    var a = $('#cmt-modal-auth');
    if (c) c.style.display = 'none';
    if (a) a.style.display = 'flex';
    switchModalMode(mode || 'login');
  }
  function switchModalMode(mode) {
    var isLogin = mode === 'login';
    $$('.cmt-modal-tab').forEach(function (t) { t.classList.toggle('is-active', t.getAttribute('data-mode') === mode); });
    var loginF = $('#cmt-form-login');
    var regF = $('#cmt-form-register');
    if (loginF) loginF.style.display = isLogin ? 'flex' : 'none';
    if (regF) regF.style.display = isLogin ? 'none' : 'flex';
  }
  function bindModal() {
    on($('#cmt-do-logout'), 'click', function () {
      doLogout(function () {
        try { localStorage.removeItem('cmt_skip'); } catch (e) { /* noop */ }
        state.mode = 'guest';
        renderIdentity();
        toast('已退出登录', 'success');
      });
    });
    // 匿名态工具栏的账户图标：点击打开登录弹窗（Email 登录/切换账号）
    on($('#cmt-tool-account'), 'click', openModal);
    on($('#cmt-modal-close'), 'click', closeModal);
    on($('#cmt-modal-mask'), 'click', closeModal);
    // 二选一：Email → 登录/注册表单；跳过验证 → 匿名三输入态
    on($('#cmt-choose-email'), 'click', function () { showAuth('login'); });
    on($('#cmt-choose-skip'), 'click', function () {
      try { localStorage.setItem('cmt_skip', '1'); } catch (e) { /* noop */ }
      state.mode = 'anonymous';
      renderIdentity();
      closeModal();
    });
    on($('#cmt-goto-register'), 'click', function () { switchModalMode('register'); });
    on($('#cmt-goto-login'), 'click', function () { switchModalMode('login'); });
    $$('.cmt-modal-tab').forEach(function (t) {
      on(t, 'click', function () { switchModalMode(t.getAttribute('data-mode') || 'login'); });
    });

    // Esc 关闭
    on(document, 'keydown', function (e) {
      if (e.key === 'Escape') {
        var m = $('#cmt-modal');
        if (m && m.style.display === 'flex') closeModal();
      }
    });

    // 登录表单
    on($('#cmt-form-login'), 'submit', function (e) {
      e.preventDefault();
      var u = $('#cmt-login-username').value.trim();
      var p = $('#cmt-login-password').value;
      if (!u || !p) { toast('请填写用户名和密码', 'error'); return; }
      var btn = $('#cmt-do-login'); if (btn) btn.disabled = true;
      fetch('/api/auth/login', {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: u, password: p })
      }).then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
        .then(function (res) {
          if (!res.ok || !res.data || !res.data.ok) throw new Error(res.data && res.data.error ? res.data.error : '登录失败');
          toast('登录成功', 'success');
          closeModal();
          fetchMe();
        })
        .catch(function (err) { toast(err.message || '登录失败', 'error'); })
        .then(function () { if (btn) btn.disabled = false; });
    });

    // 注册表单
    on($('#cmt-form-register'), 'submit', function (e) {
      e.preventDefault();
      var u = $('#cmt-reg-username').value.trim();
      var p = $('#cmt-reg-password').value;
      var n = $('#cmt-reg-nick').value.trim();
      var em = $('#cmt-reg-email').value.trim();
      var av = $('#cmt-reg-avatar').value.trim();
      if (!u || !p || !em) { toast('请填写必填项', 'error'); return; }
      var btn = $('#cmt-do-register'); if (btn) btn.disabled = true;
      fetch('/api/auth/register', {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: u, password: p, display_name: n, email: em, avatar: av })
      }).then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
        .then(function (res) {
          if (!res.ok || !res.data || !res.data.ok) throw new Error(res.data && res.data.error ? res.data.error : '注册失败');
          toast('注册成功并已登录', 'success');
          closeModal();
          fetchMe();
        })
        .catch(function (err) { toast(err.message || '注册失败', 'error'); })
        .then(function () { if (btn) btn.disabled = false; });
    });
  }

  function doLogout(cb) {
    fetch('/api/auth/logout', { method: 'POST', credentials: 'same-origin' })
      .then(function () { return true; })
      .catch(function () { return true; })
      .then(function () {
        state.me = { id: 0, verified: false };
        if (cb) cb();
      });
  }

  // ------- 通用事件绑定（委托 + PJAX 兼容） -------
  function on(el, ev, fn) {
    if (!el) return;
    listen(el, ev, fn);
  }

  function bindDelegated() {
    // 回复按钮（委托到 document，因为评论列表是动态插入的）
    listen(document, 'click', function (e) {
      var like = e.target.closest && e.target.closest('.cmt-like-btn');
      if (like) {
        if (like.disabled || like.getAttribute('data-liked') === '1') return;
        var likeID = like.getAttribute('data-id');
        if (!likeID) return;
        var count = like.querySelector('.cmt-like-count');
        var previous = count ? parseInt(count.textContent, 10) || 0 : 0;
        like.disabled = true;
        if (count) count.textContent = String(previous + 1);
        fetch('/api/comments/' + encodeURIComponent(likeID) + '/like', { method: 'POST', credentials: 'same-origin' })
          .then(function (response) { return response.json(); })
          .then(function (data) {
            if (!data || !data.ok) { if (count) count.textContent = String(previous); like.disabled = false; return; }
            if (count && typeof data.likes === 'number') count.textContent = String(data.likes);
            like.setAttribute('data-liked', '1'); like.classList.add('is-liked');
          }).catch(function () { if (count) count.textContent = String(previous); like.disabled = false; });
        return;
      }
      var replyBtn = e.target.closest && e.target.closest('.cmt-reply-btn');
      if (replyBtn) {
        var id = replyBtn.getAttribute('data-id');
        var nick = replyBtn.getAttribute('data-nick');
        if (id) startReply(parseInt(id, 10) || 0, nick || '');
        return;
      }
      var boostChip = e.target.closest && e.target.closest('.cmt-boost-chip[data-boost-mine="1"]');
      if (boostChip) {
        var chipID = boostChip.getAttribute('data-id');
        if (chipID) doUnboost(chipID);
        return;
      }
      var boostBtn = e.target.closest && e.target.closest('.cmt-boost-btn');
      if (boostBtn) {
        var boostID = boostBtn.getAttribute('data-id');
        if (!boostID) return;
        if (boostBtn.getAttribute('data-boost-action') === 'delete') doUnboost(boostID);
        else openBoostPanel(boostBtn);
        return;
      }
      var boostToggle = e.target.closest && e.target.closest('.cmt-boost-emoji-toggle');
      if (boostToggle) {
        var togglePanel = boostToggle.closest('.cmt-boost-popover');
        var picker = togglePanel && togglePanel.querySelector('.cmt-boost-picker');
        if (picker) {
          picker.hidden = !picker.hidden;
          boostToggle.setAttribute('aria-expanded', picker.hidden ? 'false' : 'true');
          if (!picker.hidden) {
            renderBoostPicker(togglePanel);
            loadBoostEmojiData().then(function () {
              if (togglePanel.parentNode && !picker.hidden) renderBoostPicker(togglePanel);
            });
            var search = togglePanel.querySelector('.cmt-boost-search');
            if (search) search.focus();
          } else {
            positionBoostPanel(togglePanel, togglePanel._boostAnchor);
          }
        }
        return;
      }
      var boostMore = e.target.closest && e.target.closest('.cmt-boost-category-more');
      if (boostMore) {
        var rail = boostMore.closest('.cmt-boost-rail');
        var cats = rail && rail.querySelector('.cmt-boost-categories');
        if (cats) cats.scrollBy({ top: cats.clientHeight || 132, behavior: 'smooth' });
        return;
      }
      var boostCategory = e.target.closest && e.target.closest('.cmt-boost-category');
      if (boostCategory) {
        var categoryPanel = boostCategory.closest('.cmt-boost-popover');
        if (categoryPanel) {
          categoryPanel.setAttribute('data-category', boostCategory.getAttribute('data-category') || 'recent');
          var categorySearch = categoryPanel.querySelector('.cmt-boost-search');
          if (categorySearch) categorySearch.value = '';
          renderBoostPicker(categoryPanel, '');
        }
        return;
      }
      var boostEmoji = e.target.closest && e.target.closest('.cmt-boost-emoji-option');
      if (boostEmoji) {
        var emojiPanel = boostEmoji.closest('.cmt-boost-popover');
        var emoji = boostEmoji.getAttribute('data-emoji');
        if (emojiPanel && emoji) selectBoostEmoji(emojiPanel, emoji);
        return;
      }
      var clearRecent = e.target.closest && e.target.closest('.cmt-boost-clear-recent');
      if (clearRecent) {
        var clearPanel = clearRecent.closest('.cmt-boost-popover');
        clearBoostRecent();
        if (clearPanel) renderBoostPicker(clearPanel, '');
        return;
      }
      var boostConfirm = e.target.closest && e.target.closest('.cmt-boost-confirm');
      if (boostConfirm) {
        var boostPanel = boostConfirm.closest('.cmt-boost-popover');
        if (boostPanel) doBoost(boostPanel.getAttribute('data-id'), boostPanel);
        return;
      }
      if (e.target.closest && e.target.closest('.cmt-boost-cancel')) {
        closeBoostPanel();
        return;
      }
      // 取消回复
      if (e.target.closest && e.target.closest('#cmt-reply-cancel')) {
        cancelReply();
        return;
      }
      // 提交按钮
      if (e.target.closest && e.target.closest('#cmt-submit')) {
        // guest 态按钮为「登录」：开弹窗；其余态正常提交
        if (state.mode === 'guest') openModal(); else doSubmit();
        return;
      }
      // 空状态："抢个沙发" → 滚动到输入框
      if (e.target.closest && e.target.closest('#cmt-empty-action')) {
        var ta = $('#cmt-content');
        if (ta) { ta.scrollIntoView({ block: 'center', behavior: 'smooth' }); ta.focus(); }
        return;
      }
      // 表情 tab 切换
      var tab = e.target.closest && e.target.closest('.cmt-emoji-tab');
      if (tab) {
        var pack = tab.getAttribute('data-pack');
        if (pack) switchEmojiPack(pack);
        return;
      }
      // 表情格子点击
      var cell = e.target.closest && e.target.closest('.cmt-emoji-cell');
      if (cell) {
        var src = cell.getAttribute('data-src');
        if (src) insertEmojiAtCursor(src);
        // 保持面板打开允许连发
        return;
      }
      // 排序：切换下拉菜单
      var sortBtn = e.target.closest && e.target.closest('#cmt-sort-btn');
      if (sortBtn) {
        var menu = $('#cmt-sort-menu');
        if (menu) {
          var open = menu.style.display === 'none';
          menu.style.display = open ? 'block' : 'none';
          sortBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
        }
        return;
      }
      // 排序：选择某项
      var sortItem = e.target.closest && e.target.closest('.cmt-sort-item');
      if (sortItem) {
        var mode = sortItem.getAttribute('data-sort');
        if (mode && mode !== state.sortMode) {
          state.sortMode = mode;
          $$('.cmt-sort-item').forEach(function (it) { it.classList.toggle('is-active', it === sortItem); });
          var label = $('#cmt-sort-label');
          if (label) label.textContent = sortItem.textContent.trim();
          rerenderSorted();
        }
        var m = $('#cmt-sort-menu');
        if (m) m.style.display = 'none';
        var b = $('#cmt-sort-btn');
        if (b) b.setAttribute('aria-expanded', 'false');
        return;
      }
      // 点击外部关闭排序菜单
      if (e.target.closest && !e.target.closest('.cmt-sort-wrap')) {
        var sm = $('#cmt-sort-menu');
        if (sm && sm.style.display !== 'none') {
          sm.style.display = 'none';
          var sb = $('#cmt-sort-btn');
          if (sb) sb.setAttribute('aria-expanded', 'false');
        }
      }
      if (!e.target.closest || !e.target.closest('.cmt-boost-popover')) closeBoostPanel();
    });

    listen(document, 'input', function (e) {
      var search = e.target.closest && e.target.closest('.cmt-boost-search');
      if (!search) return;
      var panel = search.closest('.cmt-boost-popover');
      if (panel) renderBoostPicker(panel, search.value);
    });

    if (window.addEventListener) {
      listen(window, 'resize', function () {
        var panel = $('.cmt-boost-popover');
        if (panel) positionBoostPanel(panel, panel._boostAnchor);
      });
      listen(window, 'scroll', function () {
        var panel = $('.cmt-boost-popover');
        if (panel) positionBoostPanel(panel, panel._boostAnchor);
      }, true);
    }

    // Boost 键盘操作，以及评论 Ctrl/Cmd + Enter 快捷提交。
    listen(document, 'keydown', function (e) {
      var boostPanel = e.target && e.target.closest && e.target.closest('.cmt-boost-popover');
      if (boostPanel) {
        if (e.key === 'Escape') {
          var picker = boostPanel.querySelector('.cmt-boost-picker');
          if (picker && !picker.hidden) {
            picker.hidden = true;
            var toggle = boostPanel.querySelector('.cmt-boost-emoji-toggle');
            if (toggle) toggle.setAttribute('aria-expanded', 'false');
            positionBoostPanel(boostPanel, boostPanel._boostAnchor);
          } else {
            closeBoostPanel();
          }
          e.preventDefault();
          return;
        }
        if (e.key === 'Enter' && e.target.closest('.cmt-boost-input')) {
          e.preventDefault();
          doBoost(boostPanel.getAttribute('data-id'), boostPanel);
          return;
        }
      }
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key !== 'Enter') return;
      if (state.mode === 'guest') return;
      var ta = document.activeElement;
      if (ta && ta.id === 'cmt-content') { e.preventDefault(); doSubmit(); }
    });

    // 匿名输入记忆
    ['input', 'change'].forEach(function (ev) {
      var n = $('#cmt-nick'), em = $('#cmt-email'), l = $('#cmt-link');
      if (n) listen(n, ev, rememberAnon);
      if (em) listen(em, ev, rememberAnon);
      if (l) listen(l, ev, rememberAnon);
    });
  }

  // ------- 初始化 -------
  function init() {
    var body = $('#cmt-body');
    if (!body) return;  // 当前页面没有评论区
    if (body.getAttribute('data-cmt-init') === '1') return;
    body.setAttribute('data-cmt-init', '1');
    if (scope) scope.onCleanup(function () { body.removeAttribute('data-cmt-init'); });

    bindDelegated();
    bindToolbar();
    bindModal();

    // 1. 拉身份  2. 观察懒加载
    fetchMe(function () {
      observeLazy();
    });
  }

  // 测试面：机械校验入口（node --test modules/stellar/tests/comments.test.mjs 在 vm 沙箱断言）。
  // 与 pjax.js 的 window.stellar.pjax._internal 模式一致；真实浏览器不消费该对象。
  // _resetState 仅供测试重建运行时状态；_init 为 init 别名（测试按需显式初始化）。
  window.stellar = window.stellar || {};
  var instance = {
    _internal: {
      escapeHtml: escapeHtml,
      defaultAvatar: defaultAvatar,
      buildAvatarUrl: buildAvatarUrl,
      renderBadges: renderBadges,
      renderReplyRef: renderReplyRef,
      renderBoostStrip: renderBoostStrip,
      renderCommentItem: renderCommentItem,
      renderComments: renderComments,
      insertNewComment: insertNewComment,
      insertAtCursor: insertAtCursor,
      insertEmojiAtCursor: insertEmojiAtCursor,
      mdToHtml: mdToHtml,
      togglePreview: togglePreview,
      renderPreview: renderPreview,
      openModal: openModal,
      startReply: startReply,
      cancelReply: cancelReply,
      doSubmit: doSubmit,
      openBoostPanel: openBoostPanel,
      closeBoostPanel: closeBoostPanel,
      boostComposerHTML: boostComposerHTML,
      loadBoostEmojiData: loadBoostEmojiData,
      renderBoostPicker: renderBoostPicker,
      filterBoostEmojis: filterBoostEmojis,
      selectBoostEmoji: selectBoostEmoji,
      boostRecentEmojis: boostRecentEmojis,
      rememberBoostEmoji: rememberBoostEmoji,
      doBoost: doBoost,
      bindToolbar: bindToolbar,
      bindDelegated: bindDelegated,
      bindModal: bindModal,
      ensurePackLoaded: ensurePackLoaded,
      renderEmojiGrid: renderEmojiGrid,
      fetchMe: fetchMe,
      renderIdentity: renderIdentity,
      getState: function () { return state; },
      _resetState: function () {
        state.parentId = 0;
        state.mode = 'anonymous';
        state.me = null;
        state.comments = [];
        state.pageKey = '';
        state.boostEmojiItems = null;
        state.boostEmojiPromise = null;
      },
      _init: init
    }
  };

  return { init: init, internal: instance._internal };
  }
  var testing = createInstance(null);
  window.stellar.comments = {
    _internal: testing.internal,
    mount: function (root, context) {
      if (mounted.has(root)) return mounted.get(root);
      var scope = window.blogRuntime.createScope(root, context);
      var cleanup = function () { scope.cleanup(); mounted.delete(root); };
      scope.onCleanup(function () { mounted.delete(root); });
      var instance = createInstance(scope);
      instance.init();
      mounted.set(root, cleanup);
      return cleanup;
    }
  };
})();
