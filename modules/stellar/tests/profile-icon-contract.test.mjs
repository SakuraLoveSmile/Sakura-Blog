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

test('身份头像失败回退到profile图标，退出后恢复设置图标', () => {
  const f = fixture(true);
  f.values.set('ArtalkUser', JSON.stringify({ name: 'Fixture reader', avatar: 'https://example.invalid/avatar.png' }));
  f.window.dispatchEvent(new Event('stellar:profile-change'));
  assert.equal(f.name.textContent, 'Fixture reader');
  assert.equal(f.avatar.children[0].src, 'https://example.invalid/avatar.png');
  f.avatar.children[0].onerror();
  assert.equal(f.icon(), profileIcon);
  f.values.delete('ArtalkUser');
  f.window.dispatchEvent(new Event('storage'));
  assert.equal(f.name.textContent, '设置');
  assert.equal(f.icon(), settingsIcon);
});

test('原生评论身份走站点账号接口：读资料、保存资料、未登录返回服务端提示', async () => {
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
  assert.equal(identity.name, '九月');
  assert.equal(identity.email, 'september@example.com');
  assert.equal(identity.url, 'https://example.com/');
  assert.equal(calls[0].path, '/api/auth/me');
  assert.equal(calls[0].init.credentials, 'same-origin');

  const saved = await f.window.stellarProfile.writeIdentity('native', { name: ' 九月 ', email: ' september@example.com ', url: ' example.com ' });
  assert.equal(saved.ok, true);
  const profileCall = calls.find(call => call.path === '/api/auth/profile');
  assert.equal(profileCall.init.method, 'POST');
  assert.deepEqual(JSON.parse(profileCall.init.body), { name: '九月', email: 'september@example.com', url: 'example.com' });

  f.window.fetch = path => {
    calls.push({ path });
    if (path === '/api/auth/profile') return jsonResponse(401, { error: '请先登录后再保存个人资料' });
    return jsonResponse(200, { verified: false });
  };
  const anonymous = await f.window.stellarProfile.readIdentity('native');
  assert.equal(anonymous.name, '');
  assert.equal(anonymous.supported, true);
  const rejected = await f.window.stellarProfile.writeIdentity('native', { name: '路人', email: '', url: '' });
  assert.equal(rejected.ok, false);
  assert.equal(rejected.unauthenticated, true);
  assert.match(rejected.message, /请先登录/);

  const loggedOut = await f.window.stellarProfile.logout('native');
  assert.equal(loggedOut.ok, true);
});
