/* comments.js — Artalk 风格评论区前端逻辑
 * 依赖：comments.html 模板（#cmt-* DOM）+ comments.css 样式
 * 无外部依赖；PJAX 兼容：document 事件委托 + __cmtBuilt guard 防重复绑定。
 * 表情 CDN：cdn-x/emoticons（与文章 {% emoji %} shortcode 同源），清单为 Artalk items 格式
 */
(function () {
  'use strict';

  if (window.__cmtBuilt) return;
  window.__cmtBuilt = true;

  // ------- 常量 -------
  // artalk/ 目录下有各 pack 的 Artalk 格式清单（items: [{key, val}]），val 为图片直链
  var EMOJI_BASE = 'https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/artalk/';
  var EMOJI_PACKS = ['blobcat', 'qq', 'tieba', 'aru', 'twemoji'];
  var BOOST_EMOJIS = ['👍', '❤️', '😄', '🎉', '😕', '🚀'];

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
    sortMode: 'newest'    // 'newest' | 'hottest' | 'oldest' | 'author'
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
        '<svg viewBox="0 0 24 24" width="10" height="10" fill="currentColor" aria-hidden="true"><path d="M12 2l2.4 5 5.6.4-4.2 3.7 1.4 5.5L12 14.8 6.8 16.6 8.2 11.1 4 7.4 9.6 7z"/></svg>' +
        escapeHtml(state.adminBadge || '博主') + '</span>';
    }
    if (cmt.verified && !cmt.is_admin) {
      html += '<span class="cmt-badge cmt-badge-verified" title="已验证">' +
        '<svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 12l2 2 4-4"/><circle cx="12" cy="12" r="9"/></svg>' +
        '已验证</span>';
    }
    return html;
  }

  function renderBoostButton(cmt, action) {
    var deleting = action === 'delete';
    return '<button type="button" class="cmt-action-btn cmt-boost-btn' + (deleting ? ' is-mine' : '') + '"' +
      ' data-id="' + cmt.id + '" data-boost-action="' + action + '"' +
      ' title="' + (deleting ? '删除我的回应' : '回应这条评论') + '" aria-label="' + (deleting ? '删除我的回应' : '回应这条评论') + '">' +
      '<span aria-hidden="true">🚀</span></button>';
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
    if (mine && cmt.can_boost !== false && boosts.length < 50) html += renderBoostButton(cmt, 'delete');
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
          '<span class="cmt-item-nick">' + nickHtml + '</span>' +
          renderBadges(cmt) +
          (cmt.reply_ref ? '<span class="cmt-reply-ref">' + cmt.reply_ref + '</span>' : '') +
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
          (!((cmt.boosts || []).some(function (boost) { return boost && boost.mine; })) && cmt.can_boost !== false ? renderBoostButton(cmt, 'add') : '') +
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
        cmt.reply_ref = '▸ ' + escapeHtml(parent.getAttribute('data-nick'));
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

  // ------- 评论 Boost：固定表情 + 16 字短文本 -------
  function closeBoostPanel() {
    var panel = $('.cmt-boost-popover');
    if (panel && panel.parentNode) panel.parentNode.removeChild(panel);
  }

  function positionBoostPanel(panel, anchor) {
    if (!panel || !anchor) return;
    var rect = anchor.getBoundingClientRect();
    var width = panel.offsetWidth || 250;
    var left = Math.max(8, Math.min(window.innerWidth - width - 8, rect.left));
    var top = rect.bottom + 8;
    if (top + panel.offsetHeight > window.innerHeight - 8) top = Math.max(8, rect.top - panel.offsetHeight - 8);
    panel.style.left = left + 'px';
    panel.style.top = top + 'px';
  }

  function openBoostPanel(btn) {
    closeBoostPanel();
    var panel = document.createElement('div');
    panel.className = 'cmt-boost-popover';
    panel.setAttribute('role', 'dialog');
    panel.setAttribute('aria-label', '回应评论');
    panel.setAttribute('data-id', btn.getAttribute('data-id') || '');
    panel.innerHTML = '<div class="cmt-boost-emojis" role="group" aria-label="选择表情">' +
      BOOST_EMOJIS.map(function (emoji, i) {
        return '<button type="button" class="cmt-boost-emoji-btn' + (i === 0 ? ' is-selected' : '') + '" data-emoji="' + escapeHtml(emoji) + '" aria-label="' + escapeHtml(emoji) + '">' + emoji + '</button>';
      }).join('') + '</div>' +
      '<div class="cmt-boost-compose"><input type="text" class="cmt-boost-input" maxlength="16" placeholder="说点什么（可选）" aria-label="回应文字"><button type="button" class="cmt-boost-send" aria-label="发送回应">发送</button></div>';
    document.body.appendChild(panel);
    panel._boostAnchor = btn;
    positionBoostPanel(panel, btn);
    panel.querySelector('.cmt-boost-input').focus();
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
    var emojiBtn = panel.querySelector('.cmt-boost-emoji-btn.is-selected') || panel.querySelector('.cmt-boost-emoji-btn');
    var input = panel.querySelector('.cmt-boost-input');
    var send = panel.querySelector('.cmt-boost-send');
    var emoji = emojiBtn && emojiBtn.getAttribute('data-emoji');
    if (!emoji) return;
    if (send) send.disabled = true;
    var body = boostPayload();
    body.emoji = emoji;
    body.text = input ? input.value.trim() : '';
    fetch('/api/comments/' + encodeURIComponent(id) + '/boost', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    }).then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
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
        fileInput.addEventListener('change', function () {
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
    el.addEventListener(ev, fn);
  }

  function bindDelegated() {
    // 回复按钮（委托到 document，因为评论列表是动态插入的）
    document.addEventListener('click', function (e) {
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
      var boostEmoji = e.target.closest && e.target.closest('.cmt-boost-emoji-btn');
      if (boostEmoji) {
        var popover = boostEmoji.closest('.cmt-boost-popover');
        if (popover) {
          $$('.cmt-boost-emoji-btn', popover).forEach(function (item) { item.classList.toggle('is-selected', item === boostEmoji); });
        }
        return;
      }
      var boostSend = e.target.closest && e.target.closest('.cmt-boost-send');
      if (boostSend) {
        var boostPanel = boostSend.closest('.cmt-boost-popover');
        if (boostPanel) doBoost(boostPanel.getAttribute('data-id'), boostPanel);
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

    if (window.addEventListener) {
      window.addEventListener('resize', function () {
        var panel = $('.cmt-boost-popover');
        if (panel) positionBoostPanel(panel, panel._boostAnchor);
      });
      window.addEventListener('scroll', function () {
        var panel = $('.cmt-boost-popover');
        if (panel) positionBoostPanel(panel, panel._boostAnchor);
      }, true);
    }

    // Ctrl/Cmd + Enter 快捷提交
    document.addEventListener('keydown', function (e) {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key !== 'Enter') return;
      if (state.mode === 'guest') return;
      var ta = document.activeElement;
      if (ta && ta.id === 'cmt-content') { e.preventDefault(); doSubmit(); }
    });

    // 匿名输入记忆
    ['input', 'change'].forEach(function (ev) {
      var n = $('#cmt-nick'), em = $('#cmt-email'), l = $('#cmt-link');
      if (n) n.addEventListener(ev, rememberAnon);
      if (em) em.addEventListener(ev, rememberAnon);
      if (l) l.addEventListener(ev, rememberAnon);
    });
  }

  // ------- 初始化 -------
  function init() {
    var body = $('#cmt-body');
    if (!body) return;  // 当前页面没有评论区
    if (body.getAttribute('data-cmt-init') === '1') return;
    body.setAttribute('data-cmt-init', '1');

    bindDelegated();
    bindToolbar();
    bindModal();

    // 1. 拉身份  2. 观察懒加载
    fetchMe(function () {
      observeLazy();
    });
  }

  // 测试面：机械校验入口（node --test scripts/test/comments.test.mjs 在 vm 沙箱断言）。
  // 与 pjax.js 的 window.stellar.pjax._internal 模式一致；真实浏览器不消费该对象。
  // _resetState 仅供测试重建运行时状态；_init 为 init 别名（测试按需显式初始化）。
  window.stellar = window.stellar || {};
  window.stellar.comments = {
    _internal: {
      escapeHtml: escapeHtml,
      defaultAvatar: defaultAvatar,
      buildAvatarUrl: buildAvatarUrl,
      renderBadges: renderBadges,
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
      },
      _init: init
    }
  };

  // DOMContentLoaded 已过也没关系，立即跑（整页刷新模型，无需换页重初始化）
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
