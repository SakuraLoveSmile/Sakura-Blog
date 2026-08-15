import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import vm from "node:vm";

const source = await readFile(new URL("../../frontend/assets/js/comments.js", import.meta.url), "utf8");

function renderer() {
  const window = { stellar: {}, __cmtBuilt: false };
  const document = {
    readyState: "loading",
    addEventListener() {},
    querySelector() { return null; },
    querySelectorAll() { return []; },
    createElement() { return {}; },
    body: { appendChild() {} },
  };
  vm.runInNewContext(source, {
    window,
    document,
    console,
    setTimeout,
    clearTimeout,
    requestAnimationFrame() {},
    encodeURIComponent,
    localStorage: { getItem() { return null; }, setItem() {} },
  });
  return window.stellar.comments._internal.renderCommentItem;
}

test("renderCommentItem renders boost strip and places mine rocket at strip end", () => {
  const render = renderer();
  const html = render({
    id: 7,
    nick: "Alice",
    avatar: "https://avatar.test/a.png",
    content_html: "<p>Hello</p>",
    likes: 2,
    can_boost: true,
    boosts: [
      { emoji: "👍", text: "同意", nick: "Bob", avatar: "https://avatar.test/b.png", mine: false },
      { emoji: "🚀", text: "", nick: "Me", avatar: "https://avatar.test/me.png", mine: true },
    ],
  });

  assert.match(html, /cmt-boost-strip/);
  assert.match(html, /cmt-boost-chip/);
  assert.match(html, /cmt-boost-text[^>]*>同意/);
  assert.match(html, /cmt-boost-btn/);
  assert.ok(html.indexOf("cmt-boost-strip") < html.lastIndexOf("cmt-boost-btn"));
  assert.equal((html.match(/data-boost-action="add"/g) || []).length, 0);
  assert.equal((html.match(/data-boost-action="delete"/g) || []).length, 1);
});

test("renderCommentItem keeps a new boost rocket in actions", () => {
  const render = renderer();
  const html = render({
    id: 8,
    nick: "Alice",
    content_html: "<p>Hello</p>",
    can_boost: true,
    boosts: [],
  });
  const actionStart = html.indexOf("cmt-item-actions");
  assert.ok(actionStart >= 0);
  assert.ok(html.indexOf('data-boost-action="add"', actionStart) > actionStart);
  assert.equal((html.match(/cmt-boost-strip/g) || []).length, 0);
});
