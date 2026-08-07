// 行为面：Artalk 风格评论区（assets/js/comments.js）—— 评论渲染、提交校验、回复态与表情插入。
// 机械校验入口：make test → assets-test → node --test scripts/test/
// 加载真实 comments.js 源码到 vm 沙箱（最小浏览器桩），断言渲染/交互逻辑，不依赖真实 DOM/网络。
// comments.js 是自执行 IIFE，经 window.stellar.comments._internal 暴露测试面（与 pjax.js 模式一致）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), '../../assets/js/comments.js');
const SURFACE = '评论区';

function makeClassList() {
  const set = new Set();
  return {
    add: (...names) => names.forEach((n) => set.add(n)),
    remove: (...names) => names.forEach((n) => set.delete(n)),
    toggle: (name, force) => {
      const on = force !== undefined ? !!force : !set.has(name);
      on ? set.add(name) : set.delete(name);
      return on;
    },
    contains: (name) => set.has(name),
  };
}

function makeLocalStorage() {
  const map = new Map();
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  };
}

// 元素桩：comments.js 只触达 style/textContent/value/innerHTML/classList/getAttribute/
// addEventListener/querySelector(All)/appendChild/insertAdjacentHTML/scrollIntoView/focus/setSelectionRange。
function makeEl(opts = {}) {
  const listeners = Object.create(null);
  const el = {
    tagName: opts.tagName || 'DIV',
    id: opts.id || '',
    style: { ...(opts.style || {}) },
    textContent: opts.textContent ?? '',
    innerHTML: opts.innerHTML ?? '',
    value: opts.value ?? '',
    disabled: false,
    selectionStart: 0,
    selectionEnd: 0,
    attrs: { ...(opts.attrs || {}) },
    classList: makeClassList(),
    children: [],
    adjacent: [], // insertAdjacentHTML 记录
    focused: 0,
    getAttribute: (n) => (n in el.attrs ? el.attrs[n] : null),
    setAttribute: (n, v) => { el.attrs[n] = String(v); },
    hasAttribute: (n) => n in el.attrs,
    addEventListener: (ev, fn) => { (listeners[ev] ||= []).push(fn); },
    dispatch: (ev, e) => (listeners[ev] || []).slice().forEach((fn) => fn(e)),
    querySelector: (sel) => el.childMap?.[sel] ?? null,
    querySelectorAll: () => el.children,
    appendChild: (child) => { el.children.push(child); child.parentNode = el; return child; },
    removeChild: () => {},
    insertAdjacentHTML: (pos, html) => { el.adjacent.push({ pos, html }); },
    scrollIntoView: () => {},
    click: () => { el.clicks = (el.clicks || 0) + 1; },
    focus: () => { el.focused += 1; },
    setSelectionRange: (s, e) => { el.selectionStart = s; el.selectionEnd = e; },
    closest: (sel) => (el.closestMap && sel in el.closestMap ? el.closestMap[sel] : null),
    contains: (t) => !!opts.contains?.(t),
    childMap: {},
  };
  return el;
}

// template 桩：innerHTML 赋值产出 content.firstElementChild（insertNewComment 回复路径用）
function makeTemplate() {
  const t = { content: { firstElementChild: null } };
  Object.defineProperty(t, 'innerHTML', {
    set(html) { t.content.firstElementChild = { _html: html }; },
    get() { return ''; },
  });
  return t;
}

// 每个用例一个全新沙箱：comments.js 顶层有 __cmtBuilt guard 且持有模块级 state，必须隔离。
function loadComments({ respond = () => null } = {}) {
  const noop = () => {};
  const fetchCalls = [];
  const toasts = [];
  const docListeners = Object.create(null);

  const body = makeEl({ tagName: 'BODY' });
  const origAppend = body.appendChild.bind(body);
  body.appendChild = (t) => { toasts.push(t.textContent); return origAppend(t); };

  const els = {
    '#cmt-content': makeEl({ tagName: 'TEXTAREA', id: 'cmt-content' }),
    '#cmt-nick': makeEl({ tagName: 'INPUT' }),
    '#cmt-email': makeEl({ tagName: 'INPUT' }),
    '#cmt-link': makeEl({ tagName: 'INPUT' }),
    '#cmt-website': makeEl({ tagName: 'INPUT' }),
    '#cmt-submit': makeEl({ tagName: 'BUTTON' }),
    '#cmt-list': makeEl(),
    '#cmt-list-head': makeEl(),
    '#cmt-empty': makeEl(),
    '#cmt-count-badge': makeEl(),
    '#cmt-reply-tip': makeEl(),
    '#cmt-reply-nick': makeEl(),
    '#cmt-id-anon': makeEl(),
    '#cmt-id-logged': makeEl(),
    '#cmt-identity': makeEl(),
    '#cmt-id-nick': makeEl(),
    '#cmt-modal': makeEl(),
    '#cmt-modal-chooser': makeEl(),
    '#cmt-modal-auth': makeEl(),
    '#cmt-choose-email': makeEl({ tagName: 'BUTTON' }),
    '#cmt-choose-skip': makeEl({ tagName: 'BUTTON' }),
    '#cmt-preview': makeEl({ style: { display: 'none' } }),
    '#cmt-tool-md': makeEl({ tagName: 'BUTTON' }),
    '#cmt-emoji-panel': makeEl({ style: { display: 'none' } }),
    '#cmt-emoji-body': makeEl(),
    '#cmt-tool-image': makeEl({ tagName: 'BUTTON' }),
    '#cmt-emoji-btn': makeEl({ tagName: 'BUTTON' }),
  };
  // 列表标题：renderComments/insertNewComment 经 listHead.querySelector('.cmt-list-title') 触达
  const listTitle = makeEl({ tagName: 'SPAN' });
  els['#cmt-list-head'].childMap['.cmt-list-title'] = listTitle;
  els['.cmt-list-title'] = listTitle;

  // 属性选择器注册表：insertNewComment 回复路径查 .cmt-item[data-id="N"]
  const attrEls = Object.create(null);

  const documentStub = {
    readyState: 'complete', // init() 顶层立即执行；无 #cmt-body 时直接返回，不产生副作用
    activeElement: null,
    body,
    createElement: (tag) => (tag === 'template' ? makeTemplate() : makeEl({ tagName: String(tag).toUpperCase() })),
    querySelector: (sel) => els[sel] ?? attrEls[sel] ?? null,
    querySelectorAll: (sel) => (sel in els ? [els[sel]] : []),
    addEventListener: (ev, fn) => { (docListeners[ev] ||= []).push(fn); },
  };
  const dispatchDoc = (ev, e) => (docListeners[ev] || []).slice().forEach((fn) => fn(e));

  const sandbox = {
    console: { log: noop, warn: noop, error: noop },
    localStorage: makeLocalStorage(),
    document: documentStub,
    setTimeout: () => 0, // toast 的延时清理不进测试时钟
    clearTimeout: noop,
    requestAnimationFrame: noop,
    FormData: class {
      constructor() { this.entries = Object.create(null); }
      append(k, v) { this.entries[k] = v; }
    },
    fetch: (url, opts = {}) => {
      fetchCalls.push({ url, opts });
      const data = respond(url, opts) ?? {};
      return Promise.resolve({ ok: data.ok ?? true, status: data.status ?? 200, json: async () => data.body ?? {} });
    },
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(SRC_PATH, 'utf8'), sandbox, { filename: SRC_PATH });

  const internal = sandbox.window.stellar?.comments?._internal;
  assert.ok(
    internal?.renderComments && internal?.insertNewComment && internal?.doSubmit && internal?.renderCommentItem,
    `[${SURFACE}] comments.js 未暴露 window.stellar.comments._internal（渲染/交互核心函数），机械校验入口断裂`
  );
  return { sandbox, internal, els, attrEls, fetchCalls, toasts, dispatchDoc, listTitle };
}

const flush = async (n = 4) => {
  for (let i = 0; i < n; i += 1) await new Promise((r) => setTimeout(r, 0));
};

function sampleCmt(over = {}) {
  return {
    id: 7, nick: 'Alice', email: 'a@example.com', link: '', avatar: '',
    content_html: '<p>hello</p>', likes: 3, liked: false, is_admin: false, verified: false,
    time_ago: '3 分钟前', children: [], ...over,
  };
}

// ------- 渲染：renderCommentItem / renderBadges / 头像兜底 -------

test(`${SURFACE} · renderCommentItem：nick/属性全部转义，link 渲染为 nofollow 外链`, () => {
  const { internal } = loadComments();
  const html = internal.renderCommentItem(sampleCmt({
    nick: '<script>"a"&b', link: 'https://x.com/?q="1"',
  }));
  assert.match(html, /&lt;script&gt;&quot;a&quot;&amp;b/);
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /<a href="https:\/\/x\.com\/\?q=&quot;1&quot;" rel="nofollow noopener" target="_blank">/);
  assert.match(html, /data-id="7"/);
  assert.match(html, /class="cmt-like-count">3</);
  assert.doesNotMatch(html, /is-liked/);
});

test(`${SURFACE} · renderCommentItem：已点赞状态与嵌套子评论递归渲染`, () => {
  const { internal } = loadComments();
  const html = internal.renderCommentItem(sampleCmt({
    liked: true,
    children: [sampleCmt({ id: 8, nick: 'Bob' })],
  }));
  assert.match(html, /is-liked/);
  assert.match(html, /data-liked="1"/);
  assert.match(html, /class="cmt-children"/);
  assert.match(html, /data-id="8"/);
});

test(`${SURFACE} · renderBadges：管理员徽章用站点配置文案，已验证徽章不与管理员叠加`, () => {
  const { internal } = loadComments();
  internal.getState().adminBadge = '站长';
  assert.match(internal.renderBadges(sampleCmt({ is_admin: true, verified: true })), /cmt-badge-admin[^>]*>.*站长/);
  assert.doesNotMatch(internal.renderBadges(sampleCmt({ is_admin: true, verified: true })), /已验证/);
  assert.match(internal.renderBadges(sampleCmt({ verified: true })), /已验证/);
  assert.equal(internal.renderBadges(sampleCmt()), '');
});

test(`${SURFACE} · buildAvatarUrl：自带 avatar 直出，缺失时按 nick/邮箱前缀生成首字母 SVG data URI`, () => {
  const { internal } = loadComments();
  assert.equal(internal.buildAvatarUrl(sampleCmt({ avatar: 'https://cdn/a.png' })), 'https://cdn/a.png');
  const byNick = internal.buildAvatarUrl(sampleCmt({ nick: 'alice' }));
  assert.match(byNick, /^data:image\/svg\+xml;utf8,/);
  assert.match(decodeURIComponent(byNick), />A</);
  const byEmail = internal.buildAvatarUrl(sampleCmt({ nick: '', email: 'bob@example.com' }));
  assert.match(decodeURIComponent(byEmail), />B</);
});

// ------- 渲染：renderComments 列表与空态 -------

test(`${SURFACE} · renderComments：有评论时写列表、标题与徽章计数`, () => {
  const { internal, els, listTitle } = loadComments();
  internal.renderComments({ ok: true, total: 2, comments: [sampleCmt(), sampleCmt({ id: 8, nick: 'Bob' })] });
  assert.match(els['#cmt-list'].innerHTML, /data-id="7"/);
  assert.match(els['#cmt-list'].innerHTML, /data-id="8"/);
  assert.equal(listTitle.textContent, '2 条评论');
  assert.equal(els['#cmt-list-head'].style.display, 'flex');
  assert.equal(els['#cmt-empty'].style.display, 'none');
  assert.equal(els['#cmt-count-badge'].textContent, '共 2 条评论');
  assert.equal(internal.getState().comments.length, 2);
});

test(`${SURFACE} · renderComments：零评论时隐藏列表头、展示空态、徽章归零`, () => {
  const { internal, els } = loadComments();
  internal.renderComments({ ok: true, total: 0, comments: [] });
  assert.equal(els['#cmt-list-head'].style.display, 'none');
  assert.equal(els['#cmt-empty'].style.display, 'flex');
  assert.equal(els['#cmt-count-badge'].textContent, '0');
});

// ------- 渲染：insertNewComment 顶层与回复挂载 -------

test(`${SURFACE} · insertNewComment：顶层评论前插列表并点亮标题/徽章`, () => {
  const { internal, els, listTitle } = loadComments();
  internal.renderComments({ ok: true, total: 0, comments: [] });
  internal.insertNewComment(sampleCmt({ id: 9, nick: 'Carol', parent_id: 0 }));
  assert.equal(els['#cmt-list'].adjacent.length, 1);
  assert.equal(els['#cmt-list'].adjacent[0].pos, 'afterbegin');
  assert.match(els['#cmt-list'].adjacent[0].html, /data-id="9"/);
  assert.equal(listTitle.textContent, '1 条评论');
  assert.equal(els['#cmt-list-head'].style.display, 'flex');
  assert.equal(els['#cmt-count-badge'].textContent, '共 1 条评论');
});

test(`${SURFACE} · insertNewComment：回复挂到父评论 children 容器，计数在现有基数上 +1`, () => {
  const { internal, els, attrEls, listTitle } = loadComments();
  listTitle.textContent = '2 条评论';
  els['#cmt-count-badge'].textContent = '共 2 条评论';
  const mainBox = makeEl();
  const parent = makeEl({ attrs: { 'data-id': '3', 'data-nick': 'Alice' } });
  parent.querySelector = (sel) => (sel === '.cmt-item-main' ? mainBox : null);
  attrEls['.cmt-item[data-id="3"]'] = parent;

  internal.insertNewComment(sampleCmt({ id: 10, parent_id: 3 }));

  assert.equal(mainBox.children.length, 1, '父评论下新建 .cmt-children 容器');
  const childrenBox = mainBox.children[0];
  assert.equal(childrenBox.className, 'cmt-children');
  assert.equal(childrenBox.children.length, 1);
  assert.match(childrenBox.children[0]._html, /▸ Alice/);
  assert.match(childrenBox.children[0]._html, /data-id="10"/);
  assert.equal(listTitle.textContent, '3 条评论');
  assert.equal(els['#cmt-count-badge'].textContent, '共 3 条评论');
});

// ------- 交互：回复态（委托点击 + 直接调用） -------

test(`${SURFACE} · 委托点击回复按钮进入回复态，取消按钮还原`, () => {
  const { internal, els, dispatchDoc } = loadComments();
  internal.bindDelegated();
  const btn = makeEl({ attrs: { 'data-id': '12', 'data-nick': 'Dave' } });
  btn.closestMap = { '.cmt-reply-btn': btn };
  dispatchDoc('click', { target: btn });
  assert.equal(internal.getState().parentId, 12);
  assert.equal(els['#cmt-reply-tip'].style.display, 'flex');
  assert.equal(els['#cmt-reply-nick'].textContent, 'Dave');
  assert.ok(els['#cmt-content'].focused > 0, '进入回复态后焦点回到输入框');

  const cancel = makeEl();
  cancel.closestMap = { '#cmt-reply-cancel': cancel };
  dispatchDoc('click', { target: cancel });
  assert.equal(internal.getState().parentId, 0);
  assert.equal(els['#cmt-reply-tip'].style.display, 'none');
});

// ------- 交互：表情格子点击插入 Markdown 图片 -------

test(`${SURFACE} · 委托点击表情格子：在光标处插入 ![emoji](src) 且面板保持打开`, () => {
  const { internal, els, dispatchDoc } = loadComments();
  internal.bindDelegated();
  const ta = els['#cmt-content'];
  ta.value = 'hi ';
  ta.selectionStart = 3; ta.selectionEnd = 3;
  const src = 'https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/blobcat/nom.gif';
  const cell = makeEl({ attrs: { 'data-src': src } });
  cell.closestMap = { '.cmt-emoji-cell': cell };
  dispatchDoc('click', { target: cell });
  assert.equal(ta.value, 'hi ![emoji](' + src + ')');
  assert.equal(ta.selectionStart, ta.value.length, '光标停在插入内容之后，允许连发');
});

test(`${SURFACE} · renderEmojiGrid：按 Artalk 清单项渲染格子，非激活 pack 不覆盖`, () => {
  const { internal, els } = loadComments();
  const body = els['#cmt-emoji-body'];
  internal.renderEmojiGrid('blobcat', [
    { key: 'blobcat nom', val: 'https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/blobcat/nom.gif' },
    { key: 'blobcat <x>', val: 'https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/blobcat/x.gif' },
  ]);
  assert.match(body.innerHTML, /src="https:\/\/gcore.jsdelivr.net[^"]*nom.gif"/, 'img src 用清单的 val 直链');
  assert.match(body.innerHTML, /title="blobcat &lt;x&gt;"/, 'key 需转义');
  // 当前激活 pack 是 blobcat，渲染 qq 应被跳过（防切 tab 闪回）
  internal.renderEmojiGrid('qq', [{ key: 'a', val: 'https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/a.gif' }]);
  assert.ok(!body.innerHTML.includes('qq/a.gif'), '非激活 pack 不渲染');
});

test(`${SURFACE} · ensurePackLoaded：拉 Artalk 清单渲染，已加载 pack 切回时用缓存重渲染不重复请求`, async () => {
  const packs = {
    blobcat: [{ key: 'b1', val: 'https://gcore.jsdelivr.net/x/b1.gif' }],
    qq: [{ key: 'q1', val: 'https://gcore.jsdelivr.net/x/q1.gif' }],
  };
  const { internal, els, fetchCalls } = loadComments({
    respond: (url) => {
      const m = url.match(/artalk\/([a-z]+)\.json$/);
      return m && packs[m[1]] ? { body: { name: m[1], type: 'image', items: packs[m[1]] } } : { ok: false, status: 404 };
    },
  });
  const body = els['#cmt-emoji-body'];
  const tick = () => new Promise((r) => setTimeout(r, 0));

  internal.ensurePackLoaded('blobcat');
  await tick();
  assert.match(body.innerHTML, /b1\.gif/, '首次拉取清单并渲染 blobcat');
  assert.equal(fetchCalls.length, 1);

  internal.getState().activePack = 'qq';
  internal.ensurePackLoaded('qq');
  await tick();
  assert.match(body.innerHTML, /q1\.gif/, '切换 tab 渲染 qq');
  assert.equal(fetchCalls.length, 2);

  internal.getState().activePack = 'blobcat';
  internal.ensurePackLoaded('blobcat');
  await tick();
  assert.match(body.innerHTML, /b1\.gif/, '切回已加载 pack 用缓存重渲染');
  assert.equal(fetchCalls.length, 2, '缓存命中不再发起请求');
});

// ------- 交互：工具栏（精简为表情 + 图片上传，Markdown 语法由用户直接书写） -------

test(`${SURFACE} · 工具栏表情按钮：点击切换面板显隐`, () => {
  const { internal, els } = loadComments();
  internal.bindToolbar();
  const panel = els['#cmt-emoji-panel'];
  const stop = { stopPropagation: () => {} };
  els['#cmt-emoji-btn'].dispatch('click', stop);
  assert.equal(panel.style.display, 'flex', '首次点击打开面板');
  els['#cmt-emoji-btn'].dispatch('click', stop);
  assert.equal(panel.style.display, 'none', '再次点击关闭面板');
});

test(`${SURFACE} · 工具栏图片上传：选图后 POST /api/comments/upload，成功在光标处插入 Markdown 图片`, async () => {
  const { sandbox, internal, els, fetchCalls, toasts } = loadComments({
    respond: (url) => (url === '/api/comments/upload' ? { body: { ok: true, url: '/uploads/x.png' } } : null),
  });
  internal.bindToolbar();
  const ta = els['#cmt-content'];
  ta.value = 'abc';
  ta.selectionStart = 3; ta.selectionEnd = 3;

  els['#cmt-tool-image'].dispatch('click', {});
  // 隐藏的 file 选择器应挂载到 body；模拟选中一张图触发上传
  const input = sandbox.document.body.children.find((c) => c.type === 'file');
  assert.ok(input, '应创建隐藏文件选择器');
  assert.equal(input.accept, 'image/png,image/jpeg,image/gif,image/webp');
  input.files = [{ size: 1024 }];
  input.dispatch('change', {});
  await flush(6);

  const post = fetchCalls.find((c) => c.url === '/api/comments/upload');
  assert.ok(post, '应 POST 上传端点');
  assert.equal(post.opts.method, 'POST');
  assert.match(ta.value, /!\[image\]\(\/uploads\/x\.png\)/, '光标处插入 Markdown 图片');
  assert.ok(toasts.includes('图片已插入'));
  assert.equal(els['#cmt-tool-image'].disabled, false, '上传完成后按钮恢复可用');
});

test(`${SURFACE} · 工具栏图片上传：超过 5MB 直接拦截不发请求`, async () => {
  const { sandbox, internal, els, fetchCalls, toasts } = loadComments();
  internal.bindToolbar();
  els['#cmt-tool-image'].dispatch('click', {});
  const input = sandbox.document.body.children.find((c) => c.type === 'file');
  assert.ok(input, '应创建隐藏文件选择器');
  input.files = [{ size: 6 * 1024 * 1024 }];
  input.dispatch('change', {});
  await flush(6);
  assert.deepEqual(fetchCalls.filter((c) => c.url === '/api/comments/upload'), [], '不发上传请求');
  assert.ok(toasts.includes('图片不能超过 5MB'));
});

// ------- 交互：提交校验与发布 -------

test(`${SURFACE} · doSubmit：空内容/匿名缺昵称/邮箱非法 均被拦截且不发请求`, async () => {
  const { internal, els, fetchCalls, toasts } = loadComments();
  const ta = els['#cmt-content'];

  ta.value = '   ';
  internal.doSubmit();
  assert.deepEqual(fetchCalls, []);
  assert.ok(toasts.includes('评论内容不能为空'));

  ta.value = 'hello';
  internal.doSubmit();
  assert.deepEqual(fetchCalls, []);
  assert.ok(toasts.includes('请填写昵称'));

  els['#cmt-nick'].value = 'Alice';
  els['#cmt-email'].value = 'not-an-email';
  internal.doSubmit();
  assert.deepEqual(fetchCalls, []);
  assert.ok(toasts.includes('请填写正确的邮箱'));
  await flush();
});

test(`${SURFACE} · doSubmit：匿名合法输入 POST /api/comments，成功后清空输入并前插评论`, async () => {
  const { internal, els, fetchCalls, toasts } = loadComments({
    respond: (url, opts) => (opts?.method === 'POST' && url === '/api/comments'
      ? { body: { ok: true, status: 'approved', comment: sampleCmt({ id: 21, nick: 'Alice', parent_id: 0 }) } }
      : null),
  });
  internal.renderComments({ ok: true, total: 0, comments: [] });
  internal.getState().pageKey = 'posts/hello.html';
  els['#cmt-content'].value = '  好文章  ';
  els['#cmt-nick'].value = 'Alice';
  els['#cmt-email'].value = 'a@example.com';
  els['#cmt-link'].value = 'https://alice.dev';

  internal.doSubmit();
  await flush();

  const post = fetchCalls.find((c) => c.url === '/api/comments' && c.opts.method === 'POST');
  assert.ok(post, '应 POST /api/comments');
  assert.deepEqual(JSON.parse(post.opts.body), {
    page_key: 'posts/hello.html',
    nick: 'Alice',
    email: 'a@example.com',
    link: 'https://alice.dev',
    content: '好文章',
    parent_id: 0,
    website: '',
  });
  assert.equal(els['#cmt-content'].value, '', '发布成功后清空正文');
  assert.equal(internal.getState().parentId, 0, '发布成功后退出回复态');
  assert.equal(els['#cmt-submit'].disabled, false, '请求落定后恢复提交按钮');
  assert.ok(toasts.includes('发布成功'));
  assert.equal(els['#cmt-list'].adjacent.length, 1, 'approved 评论立即前插');
});

test(`${SURFACE} · doSubmit：待审核状态不插入列表，仅提示审核中`, async () => {
  const { internal, els, toasts } = loadComments({
    respond: (url, opts) => (opts?.method === 'POST'
      ? { body: { ok: true, status: 'pending' } } : null),
  });
  els['#cmt-content'].value = 'hi';
  els['#cmt-nick'].value = 'Alice';
  els['#cmt-email'].value = 'a@example.com';
  internal.doSubmit();
  await flush();
  assert.ok(toasts.includes('评论提交成功，审核通过后显示'));
  assert.equal(els['#cmt-list'].adjacent.length, 0);
});

test(`${SURFACE} · doSubmit：服务端报错走错误 toast，不发第二条请求`, async () => {
  const { internal, els, fetchCalls, toasts } = loadComments({
    respond: (url, opts) => (opts?.method === 'POST'
      ? { body: { ok: false, error: '频率太快' } } : null),
  });
  els['#cmt-content'].value = 'hi';
  els['#cmt-nick'].value = 'Alice';
  els['#cmt-email'].value = 'a@example.com';
  internal.doSubmit();
  await flush();
  assert.ok(toasts.includes('频率太快'));
  assert.equal(fetchCalls.filter((c) => c.opts.method === 'POST').length, 1);
  assert.equal(els['#cmt-submit'].disabled, false);
});

test(`${SURFACE} · doSubmit：登录态昵称/邮箱取自 /api/auth/me，跳过匿名校验`, async () => {
  const { internal, els, fetchCalls } = loadComments({
    respond: (url, opts) => (opts?.method === 'POST'
      ? { body: { ok: true, status: 'approved', comment: sampleCmt({ id: 22, parent_id: 0 }) } } : null),
  });
  const st = internal.getState();
  st.mode = 'logged-in';
  st.me = { id: 5, nickname: '站长', email: 'admin@example.com', verified: true };
  els['#cmt-content'].value = '来自站长';
  internal.doSubmit();
  await flush();
  const post = fetchCalls.find((c) => c.opts.method === 'POST');
  const body = JSON.parse(post.opts.body);
  assert.equal(body.nick, '站长');
  assert.equal(body.email, 'admin@example.com');
});

test(`${SURFACE} · Ctrl/Cmd+Enter：焦点在正文输入框时快捷提交`, async () => {
  const { internal, els, fetchCalls, toasts, dispatchDoc, sandbox } = loadComments();
  internal.bindDelegated();
  sandbox.document.activeElement = els['#cmt-content'];
  let prevented = false;
  dispatchDoc('keydown', { ctrlKey: true, key: 'Enter', preventDefault: () => { prevented = true; } });
  assert.ok(prevented, '快捷键命中时阻止默认换行');
  assert.ok(toasts.includes('评论内容不能为空'), '快捷键进入 doSubmit 校验链路');
  assert.deepEqual(fetchCalls, []);
  // 焦点不在正文框时不触发
  sandbox.document.activeElement = els['#cmt-nick'];
  dispatchDoc('keydown', { ctrlKey: true, key: 'Enter', preventDefault: () => {} });
  assert.equal(toasts.length, 1, '非正文焦点不触发提交');
});

// ------- 交互：fetchMe 身份分支 -------

test(`${SURFACE} · fetchMe：登录用户进入 logged-in 态并渲染昵称`, async () => {
  const { internal, els } = loadComments({
    respond: (url) => (url === '/api/auth/me' ? { body: { id: 5, nickname: '站长', avatar: 'https://cdn/me.png' } } : null),
  });
  let done = false;
  internal.fetchMe(() => { done = true; });
  await flush();
  assert.ok(done, 'fetchMe 回调触发');
  assert.equal(internal.getState().mode, 'logged-in');
  assert.equal(els['#cmt-identity'].style.display, 'flex');
  assert.equal(els['#cmt-id-anon'].style.display, 'none');
  assert.equal(els['#cmt-id-logged'].style.display, 'flex');
  assert.equal(els['#cmt-id-nick'].textContent, '站长');
  assert.equal(els['#cmt-submit'].textContent, '发送');
});

test(`${SURFACE} · fetchMe：未登录且无跳过标记 → guest 态（身份行隐藏，按钮=登录）`, async () => {
  const { internal, els } = loadComments({
    respond: (url) => (url === '/api/auth/me' ? { body: { id: 0, verified: false } } : null),
  });
  internal.fetchMe();
  await flush();
  assert.equal(internal.getState().mode, 'guest');
  assert.equal(els['#cmt-identity'].style.display, 'none');
  assert.equal(els['#cmt-submit'].textContent, '登录');
});

test(`${SURFACE} · fetchMe：有跳过标记 → 匿名态（三输入显示）并恢复本地记忆输入`, async () => {
  const { internal, els, sandbox } = loadComments({
    respond: (url) => (url === '/api/auth/me' ? { body: { id: 0, verified: false } } : null),
  });
  sandbox.localStorage.setItem('cmt_skip', '1');
  sandbox.localStorage.setItem('cmt_nick', 'Alice');
  internal.fetchMe();
  await flush();
  assert.equal(internal.getState().mode, 'anonymous');
  assert.equal(els['#cmt-id-anon'].style.display, 'block');
  assert.equal(els['#cmt-nick'].value, 'Alice');
  assert.equal(els['#cmt-submit'].textContent, '发送');
});

// ------- 交互：guest 登录流（弹窗二选一） -------

test(`${SURFACE} · guest 态点「登录」开二选一弹窗；跳过验证 → 匿名态；Email → 表单屏`, async () => {
  const { internal, els, dispatchDoc, sandbox } = loadComments();
  internal.getState().mode = 'guest';
  internal.renderIdentity();
  internal.bindDelegated();
  internal.bindModal();

  dispatchDoc('click', { target: { closest: (s) => (s === '#cmt-submit' ? els['#cmt-submit'] : null) } });
  assert.equal(els['#cmt-modal'].style.display, 'flex', '弹窗打开');
  assert.equal(els['#cmt-modal-chooser'].style.display, 'flex', '首屏为二选一');
  assert.equal(els['#cmt-modal-auth'].style.display, 'none', '表单屏初始隐藏');

  els['#cmt-choose-skip'].dispatch('click', {});
  assert.equal(internal.getState().mode, 'anonymous');
  assert.equal(sandbox.localStorage.getItem('cmt_skip'), '1', '跳过选择持久化');
  assert.equal(els['#cmt-modal'].style.display, 'none', '弹窗关闭');
  assert.equal(els['#cmt-submit'].textContent, '发送');

  internal.openModal();
  els['#cmt-choose-email'].dispatch('click', {});
  assert.equal(els['#cmt-modal-chooser'].style.display, 'none');
  assert.equal(els['#cmt-modal-auth'].style.display, 'flex', '选 Email 进表单屏');
});

// ------- 交互：Markdown 预览（M↓） -------

test(`${SURFACE} · mdToHtml：标题/粗体/引用/代码块渲染且 HTML 全转义`, () => {
  const { internal } = loadComments();
  const html = internal.mdToHtml('## hi1\n**b** <script>alert(1)</script>\n> q\n```\ncode <b>\n```');
  assert.match(html, /<h3>hi1<\/h3>/, '标题降一级输出');
  assert.match(html, /<strong>b<\/strong>/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /<blockquote>q<\/blockquote>/);
  assert.match(html, /<pre><code>code &lt;b&gt;<\/code><\/pre>/);
});

test(`${SURFACE} · 预览：M↓ 打开预览渲染输入内容，输入实时重渲染，再点关闭`, () => {
  const { internal, els } = loadComments();
  internal.bindToolbar();
  els['#cmt-content'].value = '## hi1';
  const stop = { stopPropagation: () => {} };
  els['#cmt-tool-md'].dispatch('click', stop);
  assert.equal(els['#cmt-preview'].style.display, 'block');
  assert.match(els['#cmt-preview'].innerHTML, /<h3>hi1<\/h3>/);
  els['#cmt-content'].value = '**x**';
  els['#cmt-content'].dispatch('input', {});
  assert.match(els['#cmt-preview'].innerHTML, /<strong>x<\/strong>/, '打开时输入实时重渲染');
  els['#cmt-tool-md'].dispatch('click', stop);
  assert.equal(els['#cmt-preview'].style.display, 'none');
});

// ------- 结构：__cmtBuilt guard 与测试面稳定性 -------

test(`${SURFACE} · __cmtBuilt guard：重复加载源码不会二次初始化、测试面引用保持`, () => {
  const { sandbox } = loadComments();
  const before = sandbox.window.stellar.comments._internal;
  vm.runInContext(readFileSync(SRC_PATH, 'utf8'), sandbox, { filename: SRC_PATH });
  assert.equal(sandbox.window.stellar.comments._internal, before, 'guard 命中后整体 early-return，_internal 不被重建');
});
