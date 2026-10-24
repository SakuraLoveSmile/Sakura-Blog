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
