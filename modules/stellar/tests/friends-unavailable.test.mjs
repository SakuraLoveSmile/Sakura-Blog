import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { mountBlogService } from "./blog-service-fixture.mjs";

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), "../web/assets/js/blog/services/friends.js");

async function renderFriends(payload) {
  const cards = [];
  const sections = [];
  const element = { dataset: { api: "/api/friends" } };
  const gridBox = { append: (html) => cards.push(html) };
  const sandbox = {
    console,
    def: { avatar: "https://example.com/default-avatar.png" },
    document: {
      getElementsByClassName(name) {
        return name === "ds-friends" ? [element] : [];
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
    window: { wrapLazyloadImages() {} },
  };
  mountBlogService(readFileSync(SRC_PATH, "utf8"), sandbox, { request: async () => ({ json: async () => payload }) });
  await new Promise((resolve) => setImmediate(resolve));
  return { cards, sections };
}

test("普通 friends 服务保持兼容，失联卡片在末尾且无 href", async () => {
  const rendered = await renderFriends({
    content: [
      { title: "外部源未提供状态", url: "https://normal.example" },
      { title: "失效站点", url: "https://down.example", unavailable: true, labels: [{ name: "失效", color: "EF4444" }] },
    ],
  });

  assert.equal(rendered.cards.length, 1);
  assert.match(rendered.cards[0], /href="https:\/\/normal\.example"/);
  assert.equal(rendered.sections.length, 1);
  assert.match(rendered.sections[0], /unavailable-friends[\s\S]*失效站点/);
  assert.doesNotMatch(rendered.sections[0], /href=/);
});
