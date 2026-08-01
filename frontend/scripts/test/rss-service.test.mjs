// 行为面：RSS 时间轴（assets/js/services/rss.js）—— JSON Feed 渲染 handleJsonFeed 的纯逻辑。
// 机械校验入口：make test → assets-test → node --test scripts/test/
// 加载真实 rss.js 源码到 vm 沙箱：utils.jq 置空跳过 DOM 初始化，只取顶层渲染函数；
// handleJsonFeed 输入纯数据对象，唯一 DOM 依赖 $(el).append 以桩捕获产出 HTML。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), '../../assets/js/services/rss.js');
const SURFACE = 'RSS 时间轴';

function loadRss() {
  const noop = () => {};
  const appended = [];
  const sandbox = {
    console: { log: noop, warn: noop, error: noop },
    utils: { jq: noop }, // 不执行 DOM 扫描初始化
    $: () => ({ append: (html) => appended.push(html) }),
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(SRC_PATH, 'utf8'), sandbox, { filename: SRC_PATH });
  assert.ok(
    typeof sandbox.handleJsonFeed === 'function',
    `[${SURFACE}] rss.js 未暴露顶层 handleJsonFeed，机械校验入口断裂`
  );
  return { handleJsonFeed: sandbox.handleJsonFeed, appended };
}

function makeFeed(items) {
  return { version: 'https://jsonfeed.org/version/1.1', title: '站点标题', items };
}

const baseItem = {
  id: 'a1',
  title: '第一篇',
  url: 'https://example.com/a1',
  content_html: '<p>正文A</p>',
  summary: '摘要A',
};

test(`${SURFACE} · handleJsonFeed：limit 截断条目数`, () => {
  const { handleJsonFeed, appended } = loadRss();
  const items = Array.from({ length: 5 }, (_, i) => ({ ...baseItem, id: `p${i}`, title: `第${i}篇` }));
  handleJsonFeed({}, makeFeed(items), 'content', true, true, 2);
  const html = appended.join('');
  assert.equal((html.match(/class="timenode"/g) || []).length, 2);
  assert.ok(html.includes('第0篇') && html.includes('第1篇') && !html.includes('第2篇'));
});

test(`${SURFACE} · handleJsonFeed：content_type 决定渲染 summary 还是 content_html`, () => {
  const { handleJsonFeed, appended } = loadRss();
  handleJsonFeed({}, makeFeed([baseItem]), 'summary', true, true, 10);
  assert.ok(appended[0].includes('摘要A') && !appended[0].includes('<p>正文A</p>'));

  const second = loadRss();
  second.handleJsonFeed({}, makeFeed([baseItem]), 'content', true, true, 10);
  assert.ok(second.appended[0].includes('<p>正文A</p>') && !second.appended[0].includes('摘要A'));
});

test(`${SURFACE} · handleJsonFeed：show_title/show_content 开关控制标题与正文输出`, () => {
  const { handleJsonFeed, appended } = loadRss();
  handleJsonFeed({}, makeFeed([baseItem]), 'content', false, false, 10);
  const html = appended[0];
  assert.ok(!html.includes('class="title"'), '关闭 show_title 后不应输出标题节点');
  assert.ok(!html.includes('class="content"'), '关闭 show_content 后不应输出正文节点');
});

test(`${SURFACE} · handleJsonFeed：作者回退链 item.authors → feed.authors → feed.title`, () => {
  const { handleJsonFeed, appended } = loadRss();
  const feed = makeFeed([
    { ...baseItem, id: 'x1', authors: [{ name: '条目作者' }] },
    { ...baseItem, id: 'x2' },
  ]);
  feed.authors = [{ name: '订阅源作者' }];
  handleJsonFeed({}, feed, 'content', true, true, 10);
  assert.ok(appended[0].includes('<strong>条目作者</strong>'));
  assert.ok(appended[0].includes('<strong>订阅源作者</strong>'));

  // 无任何 authors 时回退到 feed.title
  const second = loadRss();
  second.handleJsonFeed({}, makeFeed([baseItem]), 'content', true, true, 10);
  assert.ok(second.appended[0].includes('<strong>站点标题</strong>'));
});

test(`${SURFACE} · handleJsonFeed：非法日期原样输出而非 Invalid Date`, () => {
  const { handleJsonFeed, appended } = loadRss();
  handleJsonFeed({}, makeFeed([{ ...baseItem, date_published: 'not-a-date' }]), 'content', true, true, 10);
  assert.ok(appended[0].includes('not-a-date'));
  assert.ok(!appended[0].includes('Invalid Date'));
});
