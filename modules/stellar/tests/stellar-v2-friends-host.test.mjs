import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import vm from 'node:vm';

const source = await fs.readFile(new URL('../web/assets/js/services/friends_and_posts.js', import.meta.url), 'utf8');

async function mount(showPosts, posts) {
  const document = { currentScript: {} };
  const cells = [];
  const requests = [];
  const lazyRoots = [];
  const element = { dataset: { api: '/fixture/friends' } };
  if (showPosts !== undefined) element.dataset.showPosts = showPosts;
  vm.runInNewContext(source, { document, def: { avatar: '/avatar.png' }, window: { wrapLazyloadImages(root) { lazyRoots.push(root); } } });
  const pending = [];
  const serviceUtils = {
    escapeHTML: value => String(value ?? ''), safeURL: value => String(value ?? ''), safeColor: value => String(value ?? ''),
    request(root, url, callback) {
      requests.push([root, url]);
      pending.push(callback({ async json() { return { content: [{ title: 'Friend', url: '/friend', description: 'Description', labels: [], posts, feed: [] }] }; } }));
    },
    dom(root) {
      assert.equal(root, element);
      return { find(selector) { assert.equal(selector, '.grid-box'); return { append(cell) { cells.push(cell); } }; } };
    },
  };
  document.currentScript.stellarMount({ getElementsByClassName(name) { assert.equal(name, 'ds-friends_and_posts'); return [element]; } }, { serviceUtils });
  await Promise.all(pending);
  assert.equal(requests.length, 1);
  assert.equal(requests[0][1], '/fixture/friends');
  assert.equal(lazyRoots[0], element);
  assert.equal(cells.length, 1);
  return cells[0];
}

test('friends host false keeps the official card and description without RSS links or empty messages', async () => {
  for (const posts of [[], [{ link: '/entry', title: 'RSS entry', published: '2026-10-13' }]]) {
    const html = await mount('false', posts);
    assert.ok(html.includes('user-post-card'));
    assert.ok(html.includes('Description'));
    assert.ok(html.includes('Friend'));
    assert.ok(!html.includes('post-link'));
    assert.ok(!html.includes('no-post'));
  }
});

test('friends omitted or true keeps the official RSS and empty-state behavior', async () => {
  for (const flag of [undefined, 'true']) {
    const html = await mount(flag, [{ link: '/entry', title: 'RSS entry', published: '2026-10-13' }]);
    assert.ok(html.includes('post-link'));
    assert.ok(html.includes('RSS entry'));
    assert.ok((await mount(flag, [])).includes('no-post'));
  }
});
