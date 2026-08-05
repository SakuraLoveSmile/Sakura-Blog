/* comments.js — Artalk 风格评论区前端逻辑
 * 依赖：comments.html 模板（#cmt-* DOM）+ comments.css 样式
 * 无外部依赖；PJAX 兼容：document 事件委托 + __cmtBuilt guard 防重复绑定。
 * 表情 CDN：Artalk-Emoticons (jsdelivr/gh) —— 与后端 bluemonday 白名单 cdn.jsdelivr.net 对齐
 */
(function () {
  'use strict';

  if (window.__cmtBuilt) return;
  window.__cmtBuilt = true;

  // ------- 常量 -------
  var EMOJI_BASE = 'https://cdn.jsdelivr.net/gh/ArtalkJS/Emoticons/menus/';
  var EMOJI_PACKS = ['blobcat', 'qq', 'tieba', 'twemoji', 'aru'];
  // 每个 pack 的本地文件名（jsdelivr 以 index.json 暴露列表，失败则用 hardcoded 兜底）
  var EMOJI_FALLBACK = {
    blobcat: ['blobcatflushed.png', 'blobcathearts.png', 'blobcatsleepo.png', 'blobcatsweats.png', 'blobcatchefkiss.png', 'blobcatcozy.png', 'blobcatderpy.png', 'blobcatgoogly.png', 'blobcatnom.png', 'blobcatsip.png', 'blobcatthink.png', 'blobcatheart.png'],
    qq: ['微笑.png', '撇嘴.png', '色.png', '发呆.png', '得意.png', '流泪.png', '害羞.png', '闭嘴.png', '睡.png', '大哭.png', '尴尬.png', '发怒.png', '调皮.png', '呲牙.png', '惊讶.png', '难过.png', '酷.png', '冷汗.png', '抓狂.png', '吐.png'],
    tieba: ['image_emoticon1.png', 'image_emoticon2.png', 'image_emoticon3.png', 'image_emoticon4.png', 'image_emoticon5.png', 'image_emoticon6.png', 'image_emoticon7.png', 'image_emoticon8.png', 'image_emoticon9.png', 'image_emoticon10.png', 'image_emoticon11.png', 'image_emoticon12.png', 'image_emoticon13.png', 'image_emoticon14.png', 'image_emoticon15.png', 'image_emoticon16.png'],
    twemoji: ['1f600.svg', '1f601.svg', '1f602.svg', '1f603.svg', '1f604.svg', '1f605.svg', '1f606.svg', '1f607.svg', '1f609.svg', '1f60a.svg', '1f60d.svg', '1f60f.svg', '1f612.svg', '1f613.svg', '1f614.svg', '1f616.svg', '1f617.svg', '1f618.svg', '1f61b.svg', '1f61c.svg', '1f61d.svg', '1f61e.svg', '1f620.svg', '1f621.svg', '2764.svg', '1f44d.svg'],
    aru: ['aru-0.png', 'aru-1.png', 'aru-2.png', 'aru-3.png', 'aru-4.png', 'aru-5.png', 'aru-6.png', 'aru-7.png', 'aru-8.png', 'aru-9.png', 'aru-10.png', 'aru-11.png', 'aru-12.png', 'aru-13.png', 'aru-14.png', 'aru-15.png']
  };

  // ------- 全局运行时状态 -------
  var state = {
    me: null,            // {id, nickname, verified, avatar, ...} 来自 /api/auth/me
    mode: 'anonymous',   // 'anonymous' | 'logged-in'
    pageKey: '',
    adminBadge: '',
    packLoaded: Object.create(null),  // pack -> true
    activePack: EMOJI_PACKS[0],
    parentId: 0,
    comments: [],         // 已渲染的评论列表（供插入新评论后更新计数）
    loadingComments: false
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

  function wrapSelection(textarea, before, after, placeholder) {
    var start = textarea.selectionStart || 0;
    var end = textarea.selectionEnd || 0;
    var value = textarea.value;
    var selected = value.substring(start, end) || (placeholder || '');
    var newValue = value.substring(0, start) + before + selected + after + value.substring(end);
    textarea.value = newValue;
    var pos = start + before.length + selected.length;
    textarea.setSelectionRange(start + before.length, pos);
    textarea.focus();
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
  function fetchMe(cb) {
    fetch('/api/auth/me', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        state.me = d || { id: 0, verified: false };
        state.mode = state.me && state.me.id > 0 ? 'logged-in' : 'anonymous';
        renderIdentity();
        if (cb) cb();
      })
      .catch(function () {
        state.me = { id: 0, verified: false };
        state.mode = 'anonymous';
        renderIdentity();
        if (cb) cb();
      });
  }

  function renderIdentity() {
    var anon = $('#cmt-id-anon');
    var logged = $('#cmt-id-logged');
    if (!anon || !logged) return;

    if (state.mode === 'logged-in') {
      anon.style.display = 'none';
      logged.style.display = 'flex';
      var me = state.me || {};
      var nick = me.nickname || me.username || '';
      $('#cmt-id-nick').textContent = nick;
      var avatar = me.avatar || defaultAvatar(nick || 'U');
      var avatarImg = $('#cmt-id-avatar');
      if (avatarImg) avatarImg.src = avatar;
    } else {
      anon.style.display = 'block';
      logged.style.display = 'none';
      // 匿名态恢复上次输入（localStorage 记住 nick/email/link）
      try {
        $('#cmt-nick').value = localStorage.getItem('cmt_nick') || '';
        $('#cmt-email').value = localStorage.getItem('cmt_email') || '';
        $('#cmt-link').value = localStorage.getItem('cmt_link') || '';
      } catch (e) { /* noop */ }
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
          '<span class="cmt-item-time">' + escapeHtml(cmt.time_ago || cmt.date_created || '') + '</span>' +
        '</div>' +
        (cmt.reply_ref ? '<div class="cmt-reply-ref">' + cmt.reply_ref + '</div>' : '') +
        '<div class="cmt-item-body">' + (cmt.content_html || '') + '</div>' +
        '<div class="cmt-item-actions">' +
          '<button type="button" class="cmt-action-btn cmt-reply-btn" data-id="' + cmt.id + '" data-nick="' + escapeHtml(cmt.nick) + '">' +
            '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/></svg>' +
            '<span>回复</span>' +
          '</button>' +
          '<button type="button" class="cmt-action-btn cmt-like-btn' + (cmt.liked ? ' is-liked' : '') + '" data-id="' + cmt.id + '" data-liked="' + (cmt.liked ? '1' : '0') + '">' +
            '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>' +
            '<span class="cmt-like-count">' + (cmt.likes || 0) + '</span>' +
          '</button>' +
        '</div>' +
        (cmt.children && cmt.children.length ? '<div class="cmt-children">' + cmt.children.map(renderCommentItem).join('') + '</div>' : '') +
      '</div>' +
    '</article>';
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
      listEl.innerHTML = state.comments.map(renderCommentItem).join('');
    } else {
      listHead.style.display = 'none';
      emptyEl.style.display = 'flex';
    }

    if (countBadge) countBadge.textContent = total > 0 ? ('共 ' + total + ' 条评论') : '0';
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
        cmt.reply_ref = '回复 <b>' + escapeHtml(parent.getAttribute('data-nick')) + '</b>';
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

  function emojiUrl(pack, file) {
    return EMOJI_BASE + pack + '/' + encodeURIComponent(file);
  }

  function ensurePackLoaded(pack) {
    if (state.packLoaded[pack]) { return; }
    var body = $('#cmt-emoji-body');
    if (!body) return;
    body.innerHTML = '<div class="cmt-emoji-loading">' +
      '<svg class="loading" viewBox="0 0 1024 1024" width="1em" height="1em" aria-hidden="true"><path fill="currentColor" d="M512 1024a512 512 0 1 1 512-512h-96a416 416 0 1 0-416 416z"/></svg>' +
      '<span>加载中…</span></div>';

    // 先尝试 index.json，失败走 fallback
    var url = EMOJI_BASE + pack + '/index.json';
    fetch(url, { cache: 'force-cache' })
      .then(function (r) {
        if (r.ok) return r.json();
        throw new Error('no-index');
      })
      .then(function (list) {
        var files = Array.isArray(list) ? list : (Array.isArray(list && list.items) ? list.items : null);
        if (!files || !files.length) throw new Error('empty');
        return files.map(function (f) { return typeof f === 'string' ? f : (f && f.name ? f.name : null); }).filter(Boolean);
      })
      .catch(function () {
        // 兜底：fallback 列表
        var fallback = EMOJI_FALLBACK[pack] || ['1f600.svg'];
        return fallback;
      })
      .then(function (files) {
        state.packLoaded[pack] = true;
        renderEmojiGrid(pack, files);
      });
  }

  function renderEmojiGrid(pack, files) {
    // 只在当前激活 tab 仍为这个 pack 时渲染（避免切 tab 期间闪回）
    if (state.activePack !== pack) return;
    var body = $('#cmt-emoji-body');
    if (!body) return;
    var html = '<div class="cmt-emoji-grid">' + files.map(function (f) {
      var u = emojiUrl(pack, f);
      return '<button type="button" class="cmt-emoji-cell" data-src="' + escapeHtml(u) + '" title="' + escapeHtml(f.replace(/\.[^.]+$/, '')) + '">' +
        '<img class="cmt-emoji-img" src="' + escapeHtml(u) + '" alt="' + escapeHtml(f) + '" loading="lazy"></button>';
    }).join('') + '</div>';
    body.innerHTML = html;
  }

  function insertEmojiAtCursor(src) {
    // 插入 Markdown 图片形式，后端 bluemonday 放行 img.cmt-emoji-img + 指定 CDN
    var md = '![emoji](' + src + ')';
    var ta = $('#cmt-content');
    if (ta) insertAtCursor(ta, md);
  }

  // ------- 工具栏按钮：Markdown 包装 -------
  function bindToolbar() {
    var ta = $('#cmt-content');
    if (!ta) return;

    on($('#cmt-tool-bold'), 'click', function () { wrapSelection(ta, '**', '**', '加粗文本'); });
    on($('#cmt-tool-italic'), 'click', function () { wrapSelection(ta, '*', '*', '斜体文本'); });
    on($('#cmt-tool-quote'), 'click', function () { wrapSelection(ta, '> ', '\n', '引用一句话'); });
    on($('#cmt-tool-code'), 'click', function () { wrapSelection(ta, '\n```\n', '\n```\n', '代码'); });
    on($('#cmt-tool-link'), 'click', function () {
      var url = (typeof prompt === 'function') ? prompt('请输入链接地址：', 'https://') : null;
      if (!url) return;
      wrapSelection(ta, '[', '](' + url + ')', '链接文字');
    });

    // 表情按钮
    on($('#cmt-emoji-btn'), 'click', function (e) { e.stopPropagation(); toggleEmojiPanel(); });
    // 点击表情面板外关闭
    on(document, 'click', function (e) {
      var panel = $('#cmt-emoji-panel');
      var btn = $('#cmt-emoji-btn');
      if (!panel || panel.style.display === 'none') return;
      if (panel.contains(e.target) || (btn && btn.contains(e.target))) return;
      panel.style.display = 'none';
    });
  }

  // ------- 登录/注册弹窗 -------
  function openModal(mode) {
    var m = $('#cmt-modal');
    if (!m) return;
    m.style.display = 'flex';
    switchModalMode(mode || 'login');
  }
  function closeModal() {
    var m = $('#cmt-modal');
    if (m) m.style.display = 'none';
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
    on($('#cmt-open-login'), 'click', function () { openModal('login'); });
    on($('#cmt-use-anon'), 'click', function () {
      // 从登录态切到匿名态：退出登录
      doLogout(function () {
        state.mode = 'anonymous';
        renderIdentity();
      });
    });
    on($('#cmt-do-logout'), 'click', function () {
      doLogout(function () {
        state.mode = 'anonymous';
        renderIdentity();
        toast('已退出登录', 'success');
      });
    });
    on($('#cmt-modal-close'), 'click', closeModal);
    on($('#cmt-modal-mask'), 'click', closeModal);
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
      // 取消回复
      if (e.target.closest && e.target.closest('#cmt-reply-cancel')) {
        cancelReply();
        return;
      }
      // 提交按钮
      if (e.target.closest && e.target.closest('#cmt-submit')) {
        doSubmit();
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
    });

    // Ctrl/Cmd + Enter 快捷提交
    document.addEventListener('keydown', function (e) {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key !== 'Enter') return;
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
      renderCommentItem: renderCommentItem,
      renderComments: renderComments,
      insertNewComment: insertNewComment,
      insertAtCursor: insertAtCursor,
      wrapSelection: wrapSelection,
      insertEmojiAtCursor: insertEmojiAtCursor,
      startReply: startReply,
      cancelReply: cancelReply,
      doSubmit: doSubmit,
      bindToolbar: bindToolbar,
      bindDelegated: bindDelegated,
      emojiUrl: emojiUrl,
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

  // DOMContentLoaded 已过也没关系，立即跑；PJAX 换页后重新跑（DOM 重新生成）
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
  document.addEventListener('pjax:complete', function () {
    // pjax 后重新初始化新页面的评论区
    setTimeout(init, 0);
  });
})();
