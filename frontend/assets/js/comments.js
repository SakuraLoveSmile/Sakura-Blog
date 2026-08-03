/* comments.js: 自建评论区交互。
   - 事件委托绑在 document 上，PJAX 换页（.l_main 被替换）不丢监听
   - fetch 提交：approved 无刷新插入列表，pending 提示待审核
   - 回复按钮：编辑器单实例内联移动到被回复评论下（Halo/Artalk 同款交互）
   - localStorage 记忆 nick/email/link；Cmd/Ctrl+Enter 提交
   - 状态与 page_key 绑定：PJAX 换页后回复状态自动失效，防挂错父评论 */
(function () {
  'use strict';

  var LS_KEY = 'blog_comment_identity';

  // 当前回复状态：pageKey 用于换页失效判定，parentId 0=顶级
  var replyState = { pageKey: '', parentId: 0 };

  function $(id) { return document.getElementById(id); }
  function cmtBody() { return $('cmt-body'); }
  function editor() { return $('cmt-editor'); }

  function currentPageKey() {
    var b = cmtBody();
    return b ? b.getAttribute('data-page-key') : '';
  }

  function adminBadge() {
    var b = cmtBody();
    return b ? (b.getAttribute('data-admin-badge') || '博主') : '博主';
  }

  /* ===== HTML 转义（昵称/链接等用户输入插入 DOM 前） ===== */
  function esc(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  /* ===== 提示浮层 ===== */
  function toast(msg) {
    var old = document.querySelector('.cmt-toast');
    if (old) old.remove();
    var t = document.createElement('div');
    t.className = 'cmt-toast';
    t.textContent = msg;
    document.body.appendChild(t);
    setTimeout(function () { t.classList.add('show'); }, 10);
    setTimeout(function () {
      t.classList.remove('show');
      setTimeout(function () { t.remove(); }, 300);
    }, 2400);
  }

  /* ===== 身份记忆 ===== */
  function saveIdentity(nick, email, link) {
    try {
      localStorage.setItem(LS_KEY, JSON.stringify({ nick: nick, email: email, link: link }));
    } catch (e) { /* 隐私模式等场景静默失败 */ }
  }

  function loadIdentity() {
    var nickEl = $('cmt-nick');
    if (!nickEl) return; // 非文章页
    try {
      var s = localStorage.getItem(LS_KEY);
      if (!s) return;
      var id = JSON.parse(s);
      if (id.nick && !nickEl.value) nickEl.value = id.nick;
      if (id.email && !$('cmt-email').value) $('cmt-email').value = id.email;
      if (id.link && !$('cmt-link').value) $('cmt-link').value = id.link;
    } catch (e) { /* 解析失败忽略 */ }
  }

  /* ===== 编辑器移动（回复内联） ===== */
  function showReplyTip(nick) {
    $('cmt-reply-nick').textContent = '@' + nick;
    $('cmt-reply-tip').style.display = '';
  }

  function hideReplyTip() {
    var tip = $('cmt-reply-tip');
    if (tip) tip.style.display = 'none';
  }

  function moveEditorTo(item) {
    var main = item.querySelector(':scope > .cmt-main');
    var actions = main ? main.querySelector(':scope > .cmt-actions') : null;
    if (!main || !actions) return;
    main.insertBefore(editor(), actions.nextSibling);
  }

  function resetEditor() {
    var b = cmtBody();
    if (b && editor()) b.insertBefore(editor(), b.firstChild);
    hideReplyTip();
    replyState = { pageKey: '', parentId: 0 };
  }

  function startReply(btn) {
    var id = parseInt(btn.getAttribute('data-id'), 10);
    var nick = btn.getAttribute('data-nick') || '';
    var item = $('comment-' + id);
    if (!item || !editor()) return;
    replyState = { pageKey: currentPageKey(), parentId: id };
    moveEditorTo(item);
    showReplyTip(nick);
    $('cmt-content').focus();
  }

  /* ===== 评论 DOM 构建（结构与 SSR 模板 comments.html 保持一致；children 递归 + like 按钮，Spec 50 懒加载复用） ===== */
  function buildItem(cm) {
    var item = document.createElement('div');
    item.className = 'cmt-item';
    item.id = 'comment-' + cm.id;
    item.setAttribute('data-id', cm.id);

    var nickHtml = cm.link
      ? '<a class="cmt-nick" href="' + esc(cm.link) + '" target="_blank" rel="external nofollow noopener noreferrer">' + esc(cm.nick) + '</a>'
      : '<span class="cmt-nick">' + esc(cm.nick) + '</span>';
    var badgeHtml = cm.is_admin ? '<span class="cmt-badge">' + esc(adminBadge()) + '</span>' : '';

    item.innerHTML =
      '<img class="cmt-avatar" src="' + esc(cm.avatar) + '" alt="' + esc(cm.nick) + '" loading="lazy">' +
      '<div class="cmt-main">' +
        '<div class="cmt-meta">' + nickHtml + badgeHtml +
          '<span class="cmt-time" title="' + esc(cm.date) + '">' + esc(cm.time_ago) + '</span>' +
        '</div>' +
        '<div class="cmt-content">' + cm.content_html + '</div>' + // 服务端已消毒，直接插入
        '<div class="cmt-actions">' +
          '<button type="button" class="cmt-reply-btn" data-id="' + cm.id + '" data-nick="' + esc(cm.nick) + '">回复</button>' +
          '<button type="button" class="cmt-reply-btn cmt-like-btn" data-id="' + cm.id + '" aria-label="赞">赞 <span class="cmt-like-count">' + (cm.likes || 0) + '</span></button>' +
        '</div>' +
      '</div>';
    if (cm.children && cm.children.length) {
      var children = document.createElement('div');
      children.className = 'cmt-children';
      cm.children.forEach(function (child) { children.appendChild(buildItem(child)); });
      item.querySelector('.cmt-main').appendChild(children);
    }
    return item;
  }

  /* 插入新评论：顶级 → 列表末尾；回复 → 父评论 children 末尾。
     父评论本身是子评论时挂到同一 children 容器（平级，保持两层嵌套）。 */
  function insertComment(cm, parentId) {
    var item = buildItem(cm);
    var parent = parentId ? $('comment-' + parentId) : null;
    if (!parent) {
      $('cmt-list').appendChild(item);
      return;
    }
    var parentChildren = parent.closest('.cmt-children');
    if (parentChildren) {
      parentChildren.appendChild(item);
      return;
    }
    var main = parent.querySelector(':scope > .cmt-main');
    var children = main.querySelector(':scope > .cmt-children');
    if (!children) {
      children = document.createElement('div');
      children.className = 'cmt-children';
      main.appendChild(children);
    }
    children.appendChild(item);
  }

  function updateCount(delta) {
    var el = $('cmt-count');
    if (!el) return;
    var m = el.textContent.match(/(\d+)/);
    var n = (m ? parseInt(m[1], 10) : 0) + delta;
    el.textContent = n + ' 条评论';
  }

  /* ===== 提交 ===== */
  function submit() {
    var b = cmtBody();
    if (!b) return;
    var content = $('cmt-content').value.trim();
    var nick = $('cmt-nick').value.trim();
    var email = $('cmt-email').value.trim();
    var link = $('cmt-link').value.trim();
    var website = $('cmt-website').value; // 蜜罐：正常用户留空

    if (!nick) { toast('请填写昵称'); return; }
    if (!email) { toast('请填写邮箱'); return; }
    if (!content) { toast('请填写评论内容'); return; }

    // PJAX 换页后回复状态失效，防挂错父评论
    if (replyState.pageKey !== currentPageKey()) replyState.parentId = 0;

    var btn = $('cmt-submit');
    btn.disabled = true;

    fetch('/api/comments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        page_key: currentPageKey(),
        nick: nick,
        email: email,
        link: link,
        content: content,
        parent_id: replyState.parentId,
        website: website
      })
    })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        btn.disabled = false;
        if (!res.ok) { toast(res.error || '提交失败，请稍后再试'); return; }
        saveIdentity(nick, email, link);
        $('cmt-content').value = '';
        if (res.status === 'pending') {
          resetEditor();
          toast('评论已提交，审核通过后显示');
          return;
        }
        if (res.comment) {
          insertComment(res.comment, replyState.parentId);
          updateCount(1);
        }
        resetEditor();
        toast('发布成功');
      })
      .catch(function () {
        btn.disabled = false;
        toast('网络错误，请稍后再试');
      });
  }

  /* ===== 事件委托（PJAX 换页不丢） ===== */
  document.addEventListener('click', function (e) {
    var replyBtn = e.target.closest('.cmt-reply-btn');
    if (replyBtn) {
      e.preventDefault();
      startReply(replyBtn);
      return;
    }
    if (e.target.closest('#cmt-reply-cancel')) {
      resetEditor();
      return;
    }
    if (e.target.closest('#cmt-submit')) {
      submit();
    }
  });

  // Cmd/Ctrl+Enter 提交（编辑器内）
  document.addEventListener('keydown', function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && e.target.closest && e.target.closest('.cmt-editor')) {
      e.preventDefault();
      submit();
    }
  });

  /* ===== Spec 50 评论区懒加载 =====
     首屏不请求评论数据：IntersectionObserver 监听 .cmt-body（rootMargin 200px），
     进入视口才 fetch /api/comments 渲染列表；不支持 IO 时保底立即加载；
     PJAX 换页后 DOM 重建（data-lazy 重新出现）重新监听；data-loaded 防重复。 */
  function renderCommentList(comments) {
    var list = $('cmt-list');
    if (!list) return;
    var lazy = $('cmt-lazy');
    if (lazy) lazy.remove();
    list.textContent = '';
    (comments || []).forEach(function (cm) { list.appendChild(buildItem(cm)); });
  }

  function loadComments() {
    var b = cmtBody();
    if (!b || b.getAttribute('data-loaded') === '1') return;
    b.setAttribute('data-loaded', '1');
    fetch('/api/comments?page_key=' + encodeURIComponent(currentPageKey()))
      .then(function (r) { return r.json(); })
      .then(function (res) {
        if (res && res.ok) {
          renderCommentList(res.comments);
        } else {
          // 失败：移除占位（避免永久“加载中”），PJAX 换页或下次进入时可重试
          b.removeAttribute('data-loaded');
          var lazy = $('cmt-lazy');
          if (lazy) lazy.remove();
        }
      })
      .catch(function () {
        b.removeAttribute('data-loaded');
        var lazy = $('cmt-lazy');
        if (lazy) lazy.remove();
      });
  }

  function initLazyComments() {
    var b = cmtBody();
    if (!b || b.getAttribute('data-lazy') !== '1' || b.getAttribute('data-loaded') === '1') return;
    if (!('IntersectionObserver' in window)) {
      loadComments(); // 老浏览器保底：立即加载
      return;
    }
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          io.disconnect();
          loadComments();
        }
      });
    }, { rootMargin: '200px' });
    io.observe(b);
  }

  // 首屏（DOMContentLoaded 兜底 readyState）+ PJAX 换页后重新监听
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initLazyComments);
  } else {
    initLazyComments();
  }
  document.addEventListener('pjax:complete', initLazyComments);

  // 身份回填：首屏 + PJAX 换页后（新 DOM 需重新回填）
  document.addEventListener('DOMContentLoaded', loadIdentity);
  document.addEventListener('pjax:complete', function () {
    replyState = { pageKey: '', parentId: 0 };
    loadIdentity();
  });
})();
