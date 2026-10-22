// 友链 RSS 服务渲染：DOM 结构必须对齐 Stellar 的 post-panel 样式。
// 该测试直接加载真实 service，避免只检查字符串快照而漏掉运行时分支。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { mountBlogService } from "./blog-service-fixture.mjs";

const SRC_PATH = join(
  dirname(fileURLToPath(import.meta.url)),
  "../web/assets/js/blog/services/friends_and_posts.js",
);

async function renderFriend(payload, options = {}) {
  const appended = [];
  const sections = [];
  appended.sections = sections;
  const element = {
    dataset: {
      api: "/api/friends-aggregated",
      ...(options.showPosts === false ? { showPosts: "false" } : {}),
    },
  };
  const gridBox = {
    append(html) {
      appended.push(html);
    },
  };
  const sandbox = {
    console,
    def: { avatar: "https://example.com/default-avatar.png" },
    document: {
      getElementsByClassName(name) {
        return name === "ds-friends_and_posts" ? [element] : [];
      },
    },
    utils: {
      request(_element, _api, callback) {
        Promise.resolve(callback({ json: async () => payload }));
      },
      dom() {
        return { find: () => gridBox, append: (html) => sections.push(html) };
      },
    },
    window: {
      wrapLazyloadImages() {},
    },
  };
  mountBlogService(readFileSync(SRC_PATH, "utf8"), sandbox, { request: async () => ({ json: async () => payload }) });
  await new Promise((resolve) => setImmediate(resolve));
  return appended;
}

test("友链 RSS 使用 post-panel 结构，标题和日期按纵向列表渲染", async () => {
  const cards = await renderFriend({
    content: [
      {
        title: "示例友链",
        url: "https://example.com",
        description: "友链描述",
        posts: [{
          title: "第一篇文章",
          link: "https://example.com/post/1",
          published: "2026-08-21",
        }],
      },
    ],
  });

  assert.equal(cards.length, 1);
  assert.match(cards[0], /class="post-panel"><div class="desc">友链描述<\/div><div class="posts">/);
  assert.match(cards[0], /class="post-link"[\s\S]*class="title">第一篇文章<\/span>[\s\S]*class="date">2026-08-21<\/span>/);
  assert.doesNotMatch(cards[0], /class="previews"/);
});

test("友链 RSS 文本会转义，缺少 labels 也不会中断渲染", async () => {
  const cards = await renderFriend({
    content: [
      {
        title: "<script>alert(1)</script>",
        url: "https://example.com",
        description: "<img src=x onerror=alert(1)>",
        posts: [{
          title: "<b>不应被当成标签</b>",
          link: "https://example.com/post/1?a=1&b=2",
          published: "2026-08-21",
        }],
      },
    ],
  });

  assert.equal(cards.length, 1);
  assert.doesNotMatch(cards[0], /<script>|<img src=x|<b>不应被当成标签<\/b>/);
  assert.match(cards[0], /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.match(cards[0], /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.match(cards[0], /&lt;b&gt;不应被当成标签&lt;\/b&gt;/);
});

test("关闭 RSS 文章列表时保留友链卡片但不渲染文章", async () => {
  const cards = await renderFriend({
    content: [{
      title: "示例友链",
      url: "https://example.com",
      description: "友链描述",
      posts: [{ title: "不应显示", link: "https://example.com/post/1", published: "2026-08-21" }],
    }],
  }, { showPosts: false });

  assert.equal(cards.length, 1);
  assert.doesNotMatch(cards[0], /不应显示/);
  assert.match(cards[0], /class="posts"><\/div>/);
});

test("失联友链移入页尾分区且不渲染链接或 RSS 错误", async () => {
  const cards = await renderFriend({
    content: [
      { title: "正常友链", url: "https://example.com", posts: [] },
      {
        title: "失联友链",
        url: "https://down.example",
        description: "暂时无法访问",
        unavailable: true,
        feed: [{ title: "error" }],
        labels: [{ name: "失联", color: "FF453A", hue: 3, saturation: 100, lightness: 61 }],
      },
    ],
  });

  assert.equal(cards.length, 1);
  assert.equal(cards.sections.length, 1);
  assert.match(cards[0], /href="https:\/\/example\.com"/);
  assert.match(cards.sections[0], /unavailable-friends[\s\S]*失联友链/);
  assert.doesNotMatch(cards.sections[0], /href=|post-link|RSS 解析失败|class="no-post"/);
  assert.match(cards.sections[0], /暂时无法访问/);
});
