import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import vm from 'node:vm';

const source = await fs.readFile(new URL('../web/assets/js/services/friends_and_posts.js', import.meta.url), 'utf8');

async function mount(showPosts, posts, items) {
  const document = { currentScript: {}, baseURI: 'https://example.test/friends' };
  const cells = [];
  const sections = [];
  const requests = [];
  const lazyRoots = [];
  const element = { dataset: { api: '/fixture/friends' } };
  if (showPosts !== undefined) element.dataset.showPosts = showPosts;
  vm.runInNewContext(source, { document, URL, def: { avatar: '/avatar.png' }, window: { wrapLazyloadImages(root) { lazyRoots.push(root); } } });
  const pending = [];
  const serviceUtils = {
    escapeHTML: value => String(value ?? ''), safeURL: value => String(value ?? ''), safeColor: value => String(value ?? ''),
    request(root, url, callback) {
      requests.push([root, url]);
      pending.push(callback({ async json() { return { content: items || [{ title: 'Friend', url: '/friend', description: 'Description', labels: [], posts, feed: [] }] }; } }));
    },
    dom(root) {
      assert.equal(root, element);
      return {
        find(selector) { assert.equal(selector, '.grid-box'); return { append(cell) { cells.push(cell); } }; },
        append(html) { sections.push(html); },
      };
    },
  };
  document.currentScript.stellarMount({ getElementsByClassName(name) { assert.equal(name, 'ds-friends_and_posts'); return [element]; } }, { serviceUtils });
  await Promise.all(pending);
  assert.equal(requests.length, 1);
  assert.equal(requests[0][1], '/fixture/friends');
  assert.equal(lazyRoots[0], element);
  return { html: cells.join(''), sections };
}

test('friends host false keeps the official card and description without RSS links or empty messages', async () => {
  for (const posts of [[], [{ link: '/entry', title: 'RSS entry', published: '2026-10-13' }]]) {
    const { html } = await mount('false', posts);
    assert.ok(html.includes('user-post-card'));
    assert.ok(html.includes('Description'));
    assert.ok(html.includes('Friend'));
    assert.ok(!html.includes('post-link'));
    assert.ok(!html.includes('no-post'));
  }
});

test('friends omitted or true keeps RSS articles and quietly retains cards without RSS', async () => {
  for (const flag of [undefined, 'true']) {
    const { html } = await mount(flag, [{ link: '/entry', title: 'RSS entry', published: '2026-10-13' }]);
    assert.ok(html.includes('post-link'));
    assert.ok(html.includes('RSS entry'));
    const empty = (await mount(flag, [])).html;
    assert.match(empty, /href="\/friend"/);
    assert.ok(empty.includes('Friend'));
    assert.ok(empty.includes('Description'));
    assert.doesNotMatch(empty, /no-post|未设置 RSS|RSS 解析失败/);
  }
});

test('RSS failure is separate from missing RSS and respects the article switch', async () => {
  const friend = { title: 'Failed feed', url: '/friend', posts: [], feed: [{ title: 'error' }] };
  assert.match((await mount(undefined, [], [friend])).html, /动态暂不可用/);
  assert.doesNotMatch((await mount('false', [], [friend])).html, /no-post|动态暂不可用/);
});

test('unavailable friends move after reachable cards without website or stale article links', async () => {
  const rendered = await mount(undefined, [], [
    { title: 'Unavailable', url: 'https://down.example', unavailable: true, description: 'Offline description',
      posts: [{ title: 'Stale article', link: '/stale' }], feed: [{ title: 'error' }], labels: [{ name: '失联', color: 'FF453A' }] },
    { title: 'Reachable', url: '/reachable', posts: [] },
  ]);
  assert.match(rendered.html, /href="\/reachable"/);
  assert.doesNotMatch(rendered.html, /Unavailable|Offline description/);
  assert.equal(rendered.sections.length, 1);
  assert.match(rendered.sections[0], /unavailable-friends[\s\S]*失联友链[\s\S]*Unavailable/);
  assert.match(rendered.sections[0], /aria-disabled="true"/);
  assert.match(rendered.sections[0], /Offline description/);
  assert.doesNotMatch(rendered.sections[0], /href=|post-link|Stale article|no-post|动态暂不可用|未设置 RSS/);
});
