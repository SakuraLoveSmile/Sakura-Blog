// 行为面：PJAX 导航（assets/js/plugins/pjax.js）—— 链接过滤决策 shouldHandleLink 与 hash 解码 decodeHashId。
// 机械校验入口：make test → assets-test → node --test scripts/test/
// 加载真实 pjax.js 源码到 vm 沙箱（最小浏览器桩），断言纯逻辑，不依赖真实 DOM/网络。
// 测试文件放 scripts/ 而非 assets/：assets/ 被 go:embed all:js 全量嵌入二进制，不能混入测试代码。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const PJAX_PATH = join(dirname(fileURLToPath(import.meta.url)), '../../assets/js/plugins/pjax.js');
const SURFACE = 'PJAX 导航';

// 最小浏览器桩：只满足 pjax.js 顶层执行与 init() 所需，行为函数在测试中显式调用。
function loadPjax() {
  const noop = () => {};
  const location = { href: 'https://example.com/posts/current/', hostname: 'example.com' };
  const sandbox = {
    URL,
    console: { log: noop, warn: noop, error: noop },
    document: { readyState: 'complete', addEventListener: noop },
    history: { replaceState: noop, pushState: noop },
    location,
    addEventListener: noop, // window.addEventListener（sandbox 即 window）
    StellarPjaxConfig: undefined, // 测试中不覆盖，使用 defaultConfig
  };
  sandbox.window = sandbox;
  sandbox.window.location = location;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(PJAX_PATH, 'utf8'), sandbox, { filename: PJAX_PATH });
  const internal = sandbox.window.stellar?.pjax?._internal;
  assert.ok(
    internal?.shouldHandleLink && internal?.decodeHashId,
    `[${SURFACE}] pjax.js 未暴露 window.stellar.pjax._internal（shouldHandleLink/decodeHashId），机械校验入口断裂`
  );
  return internal;
}

// 链接桩：shouldHandleLink 只读 tagName/href/target/dataset/hasAttribute
function makeLink(href, extra = {}) {
  return {
    tagName: 'A',
    href,
    target: '',
    dataset: {},
    hasAttribute: (name) => extra.attrs?.includes(name) ?? false,
    ...extra,
  };
}

const { shouldHandleLink, decodeHashId } = loadPjax();

test(`${SURFACE} · shouldHandleLink：站内普通链接走 PJAX`, () => {
  assert.equal(shouldHandleLink(makeLink('https://example.com/about/')), true);
  assert.equal(shouldHandleLink(makeLink('https://example.com/posts/hello.html')), true);
});

test(`${SURFACE} · shouldHandleLink：空链接 / 非 A 标签不处理`, () => {
  assert.equal(shouldHandleLink(null), false);
  assert.equal(shouldHandleLink(makeLink('https://example.com/', { tagName: 'BUTTON' })), false);
  assert.equal(shouldHandleLink(makeLink('')), false);
});

test(`${SURFACE} · shouldHandleLink：外部域名整页跳转`, () => {
  assert.equal(shouldHandleLink(makeLink('https://other.com/posts/')), false);
});

test(`${SURFACE} · shouldHandleLink：同页 hash 锚点不走 PJAX，跨页 hash 走`, () => {
  assert.equal(shouldHandleLink(makeLink('https://example.com/posts/current/#sec')), false);
  assert.equal(shouldHandleLink(makeLink('https://example.com/other/#sec')), true);
});

test(`${SURFACE} · shouldHandleLink：target 仅 _self 可走 PJAX`, () => {
  assert.equal(shouldHandleLink(makeLink('https://example.com/a/', { target: '_blank' })), false);
  assert.equal(shouldHandleLink(makeLink('https://example.com/a/', { target: '_self' })), true);
});

test(`${SURFACE} · shouldHandleLink：download 属性与 data-pjax="false" 显式退出`, () => {
  assert.equal(shouldHandleLink(makeLink('https://example.com/a/', { attrs: ['download'] })), false);
  assert.equal(shouldHandleLink(makeLink('https://example.com/a/', { dataset: { pjax: 'false' } })), false);
});

test(`${SURFACE} · shouldHandleLink：非 HTML 资源后缀整页请求`, () => {
  for (const ext of ['xml', 'rss', 'pdf', 'zip', 'mp4']) {
    assert.equal(
      shouldHandleLink(makeLink(`https://example.com/file.${ext}`)),
      false,
      `.${ext} 应被排除在 PJAX 之外`
    );
  }
});

test(`${SURFACE} · decodeHashId：URL 编码的中文锚点解码后供 getElementById 使用`, () => {
  assert.equal(decodeHashId('#%E4%B8%AD%E6%96%87'), '中文');
  assert.equal(decodeHashId('#plain-id'), 'plain-id');
});

test(`${SURFACE} · decodeHashId：非法编码回退原始值而非抛错`, () => {
  assert.equal(decodeHashId('#100%'), '100%');
});
