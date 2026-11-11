import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const scripts = readFileSync(new URL('../web/templates/partial/stellar/scripts.html', import.meta.url), 'utf8');
const profile = readFileSync(new URL('../web/assets/js/profile.js', import.meta.url), 'utf8');
const settingsIcon = '<svg data-fixture="settings"></svg>';
const profileIcon = '<svg data-fixture="profile"></svg>';

class Element extends EventTarget {
  constructor() {
    super();
    this.dataset = {};
    this.attrs = new Map();
    this.children = [];
    this.textContent = '';
  }
  setAttribute(name, value) { this.attrs.set(name, value); }
  getAttribute(name) { return this.attrs.get(name) || null; }
  replaceChildren(...children) { this.children = children; }
}

function fixture(identityEnabled = false) {
  const avatar = new Element(), name = new Element(), widget = new Element();
  widget.dataset = { settingsLabel: '设置', profileProvider: 'artalk', profileIdentityEnabled: String(identityEnabled) };
  widget.querySelector = selector => ({ '.settings-widget__avatar': avatar, '.settings-widget__name': name })[selector];
  const document = new Element();
  document.readyState = 'complete';
  document.querySelectorAll = selector => selector === '.settings-widget' ? [widget] : [];
  document.createElement = () => new Element();
  const window = new Element(), values = new Map();
  const storage = {
    getItem: key => values.get(key) || null,
    setItem: (key, value) => values.set(key, value),
    removeItem: key => values.delete(key)
  };
  const clientData = {
    canonical: {}, def: {},
    ctx: { services: { gravatar: { baseUrl: 'https://gravatar.example.invalid/avatar/' } },
      icons: { 'default:settings': settingsIcon, 'default:profile': profileIcon } }
  };
  const defineScript = scripts.match(/<script data-stellar-script="defines">([\s\S]*?)<\/script>/)?.[1];
  assert.ok(defineScript, '模板必须先提供客户端定义');
  const context = vm.createContext({ window, document, localStorage: storage, sessionStorage: storage,
    Event, URL, setTimeout, console });
  vm.runInContext(defineScript.replace('{{sf_json (sf_get .Stellar.Client "ClientData")}}', JSON.stringify(clientData)), context);
  vm.runInContext(profile, context, { filename: 'profile.js' });
  return { window, document, avatar, name, widget, values,
    icon: () => avatar.children[0]?.innerHTML };
}

test('真实模板定义接通官方profile，首次挂载和刷新不清空设置图标', () => {
  const f = fixture();
  assert.equal(f.icon(), settingsIcon);
  assert.equal(f.name.textContent, '设置');
  for (const type of ['focus', 'storage', 'stellar:profile-change']) {
    f.window.dispatchEvent(new Event(type));
    assert.equal(f.icon(), settingsIcon, type);
  }
  f.document.dispatchEvent(new Event('stellar:navigation-complete'));
  assert.equal(f.icon(), settingsIcon, '局部导航完成后保留图标');
});

// 设置挂件对原生账号要等 /api/auth/me 回来才渲染，统一用微任务刷一拍。
const flush = () => new Promise(resolve => setTimeout(resolve, 0));

test('身份头像失败回退到profile图标，退出后恢复设置图标', async () => {
  const f = fixture(true);
  f.values.set('ArtalkUser', JSON.stringify({ name: 'Fixture reader', avatar: 'https://example.invalid/avatar.png' }));
  f.window.dispatchEvent(new Event('stellar:profile-change'));
  await flush();
  assert.equal(f.name.textContent, 'Fixture reader');
  assert.equal(f.avatar.children[0].src, 'https://example.invalid/avatar.png');
  f.avatar.children[0].onerror();
  assert.equal(f.icon(), profileIcon);
  f.values.delete('ArtalkUser');
  f.window.dispatchEvent(new Event('storage'));
  await flush();
  assert.equal(f.name.textContent, '设置');
  assert.equal(f.icon(), settingsIcon);
});

test('原生评论身份：已登录走站点账号接口，读写都落到 /api/auth', async () => {
  const f = fixture(true);
  const calls = [];
  const jsonResponse = (status, body) => Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body)
  });
  f.window.fetch = (path, init) => {
    calls.push({ path, init });
    if (path === '/api/auth/me') {
      return jsonResponse(200, { id: 7, verified: true, nickname: '九月', email: 'september@example.com', avatar: '', website: 'https://example.com' });
    }
    if (path === '/api/auth/profile') {
      return jsonResponse(200, { ok: true });
    }
    return jsonResponse(404, {});
  };

  const identity = await f.window.stellarProfile.readIdentity('native');
  assert.equal(identity.supported, true);
  assert.equal(identity.account, true);
  assert.equal(identity.name, '九月');
  assert.equal(identity.email, 'september@example.com');
  assert.equal(identity.url, 'https://example.com/');
  assert.equal(calls[0].path, '/api/auth/me');
  assert.equal(calls[0].init.credentials, 'same-origin');

  const saved = await f.window.stellarProfile.writeIdentity('native', { name: ' 九月 ', email: ' september@example.com ', url: ' example.com ' });
  assert.equal(saved.ok, true);
  assert.equal(saved.account, true);
  const profileCall = calls.find(call => call.path === '/api/auth/profile');
  assert.equal(profileCall.init.method, 'POST');
  assert.deepEqual(JSON.parse(profileCall.init.body), { name: '九月', email: 'september@example.com', url: 'example.com' });
});

test('原生评论身份：未登录照上游存本地资料，退出登录清空本地身份', async () => {
  const f = fixture(true);
  const calls = [];
  const jsonResponse = (status, body) => Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body)
  });
  f.window.fetch = (path, init) => {
    calls.push({ path, init });
    if (path === '/api/auth/me') return jsonResponse(200, { id: 0, verified: false });
    if (path === '/api/auth/logout') return jsonResponse(200, { ok: true });
    return jsonResponse(401, { error: '请先登录后再保存个人资料' });
  };

  const anonymous = await f.window.stellarProfile.readIdentity('native');
  assert.equal(anonymous.supported, true);
  assert.equal(anonymous.local, true);
  assert.equal(anonymous.name, '');

  // 访客能保存：写本地键，不请求 /api/auth/profile。
  const saved = await f.window.stellarProfile.writeIdentity('native', { name: '上杉九月', email: 'sakurasep@qq.com', url: 'example.com' });
  assert.equal(saved.ok, true);
  assert.equal(saved.local, true);
  assert.equal(f.values.get('cmt_nick'), '上杉九月');
  assert.equal(f.values.get('cmt_email'), 'sakurasep@qq.com');
  assert.equal(f.values.get('cmt_link'), 'https://example.com/');
  assert.equal(calls.some(call => call.path === '/api/auth/profile'), false);

  // 回读拿到的就是刚才保存的本地资料（设置页重进能回填）。
  const reread = await f.window.stellarProfile.readIdentity('native');
  assert.equal(reread.name, '上杉九月');
  assert.equal(reread.url, 'https://example.com/');

  // 字段级校验与第三方后端一致。
  const badUrl = await f.window.stellarProfile.writeIdentity('native', { name: '上杉九月', email: '', url: 'javascript:alert(1)' });
  assert.equal(badUrl.ok, false);
  assert.equal(badUrl.validation, 'url');
  const badEmail = await f.window.stellarProfile.writeIdentity('native', { name: '上杉九月', email: 'not-an-email', url: '' });
  assert.equal(badEmail.validation, 'email');

  // 未登录的「退出登录」= 清空本地匿名资料。
  const cleared = await f.window.stellarProfile.logout('native');
  assert.equal(cleared.ok, true);
  assert.equal(cleared.local, true);
  assert.equal(f.values.has('cmt_nick'), false);
  assert.equal((await f.window.stellarProfile.readIdentity('native')).name, '');
});
