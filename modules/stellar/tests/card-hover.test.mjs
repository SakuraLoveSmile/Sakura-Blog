// Stellar 1.44 Card Hover 客户端行为：生命周期、指针降级和动态内容挂载。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const SOURCE_PATH = join(dirname(fileURLToPath(import.meta.url)), "../web/assets/js/plugins/card-hover.js");
const SOURCE = readFileSync(SOURCE_PATH, "utf8");

function classList(initial = []) {
  const values = new Set(initial);
  return {
    add(value) { values.add(value); },
    remove(value) { values.delete(value); },
    contains(value) { return values.has(value); },
  };
}

function style() {
  const values = new Map();
  return {
    setProperty(name, value) { values.set(name, value); },
    getPropertyValue(name) { return values.get(name) || ""; },
    removeProperty(name) { values.delete(name); },
  };
}

function createCard() {
  const listeners = new Map();
  const children = [];
  let focusWithin = false;
  return {
    nodeType: 1,
    children,
    classList: classList(["card-hover", "card-hover--spotlight", "card-hover--tilt"]),
    style: style(),
    appendChild(child) { child.parentNode = this; children.push(child); },
    addEventListener(type, listener) { listeners.set(type, listener); },
    removeEventListener(type) { listeners.delete(type); },
    getBoundingClientRect() { return { left: 0, top: 0, width: 100, height: 80 }; },
    matches(selector) {
      if (selector === ".card-hover") return true;
      if (selector === ":focus-within") return focusWithin;
      return false;
    },
    querySelectorAll() { return []; },
    setFocusWithin(value) { focusWithin = value; },
    listeners,
  };
}

function createRuntime({ finePointer = true, reduceMotion = false } = {}) {
  const media = new Map();
  const documentListeners = new Map();
  const windowListeners = new Map();
  const documentElement = { contains() { return true; }, style: style() };
  const document = {
    hidden: false,
    documentElement,
    addEventListener(type, listener) { documentListeners.set(type, listener); },
    removeEventListener(type) { documentListeners.delete(type); },
    createElement() {
      const listeners = new Map();
      return {
        parentNode: null,
        className: "",
        addEventListener(type, listener) { listeners.set(type, listener); },
        removeEventListener(type) { listeners.delete(type); },
        setAttribute(name, value) { this[name] = value; },
        remove() {
          if (!this.parentNode) return;
          const index = this.parentNode.children.indexOf(this);
          if (index >= 0) this.parentNode.children.splice(index, 1);
          this.parentNode = null;
        },
        listeners,
      };
    },
    querySelectorAll() { return []; },
  };
  const stellar = {};
  const window = {
    CSS: { supports() { return true; } },
    stellar,
    addEventListener(type, listener) { windowListeners.set(type, listener); },
    removeEventListener(type) { windowListeners.delete(type); },
    cancelAnimationFrame() {},
    requestAnimationFrame(callback) { callback(); return 1; },
    matchMedia(query) {
      if (!media.has(query)) {
        const listeners = new Set();
        media.set(query, {
          matches: query.includes("prefers-reduced-motion") ? reduceMotion : finePointer,
          addEventListener(type, listener) { if (type === "change") listeners.add(listener); },
          removeEventListener(type, listener) { if (type === "change") listeners.delete(listener); },
          dispatch(matches) { this.matches = matches; listeners.forEach((listener) => listener({ matches })); },
        });
      }
      return media.get(query);
    },
  };
  const context = vm.createContext({ console, ctx: { card_hover: { maxTilt: 3, spotlightColor: "rgba(255, 255, 255, 0.25)" } }, document, stellar, window });
  vm.runInContext(SOURCE, context, { filename: SOURCE_PATH });
  return { api: stellar.cardHover, document, documentListeners, windowListeners, media };
}

test("Card Hover 可挂载动态卡片并完整清理", () => {
  const runtime = createRuntime();
  const card = createCard();
  runtime.api.mountAll(card);

  assert.equal(card.classList.contains("is-card-hover-ready"), true);
  assert.equal(card.children.length, 1);
  assert.equal(card.children[0].className, "card-hover__spotlight");
  assert.deepEqual([...card.listeners.keys()].sort(), ["focusin", "pointerenter", "pointerleave", "pointermove"]);

  card.listeners.get("pointerenter")({ clientX: 90, clientY: 20 });
  assert.equal(card.style.getPropertyValue("--card-hover-mouse-x"), "90px");
  assert.equal(card.style.getPropertyValue("--card-hover-rotate-y"), "2.400deg");

  runtime.api.destroy();
  assert.equal(card.classList.contains("is-card-hover-ready"), false);
  assert.equal(card.children.length, 0);
  assert.equal(card.listeners.size, 0);
  assert.equal(runtime.documentListeners.size, 0);
  assert.equal(runtime.windowListeners.size, 0);
});

test("Card Hover 支持 Markdown 动态内容，并按容器卸载", () => {
  const runtime = createRuntime();
  const first = createCard();
  const second = createCard();
  runtime.api.mountAll(first);
  runtime.documentListeners.get("stellar:mdrender")({ detail: { target: { nodeType: 1, matches() { return false; }, querySelectorAll() { return [second]; } } } });
  assert.equal(second.classList.contains("is-card-hover-ready"), true);

  runtime.api.unmountAll({ contains(element) { return element === first; } });
  assert.equal(first.classList.contains("is-card-hover-ready"), false);
  assert.equal(second.classList.contains("is-card-hover-ready"), true);
  runtime.api.destroy();
});

test("减少动态效果或非精细指针时不挂载", () => {
  for (const options of [{ reduceMotion: true }, { finePointer: false }]) {
    const runtime = createRuntime(options);
    const card = createCard();
    runtime.api.mountAll(card);
    assert.equal(card.classList.contains("is-card-hover-ready"), false);
    assert.equal(card.children.length, 0);
    runtime.api.destroy();
  }
});

test("媒体条件变化会卸载已挂载卡片", () => {
  const runtime = createRuntime();
  const card = createCard();
  runtime.api.mountAll(card);
  runtime.media.get("(hover: hover) and (pointer: fine)").dispatch(false);
  assert.equal(card.classList.contains("is-card-hover-ready"), false);
  runtime.api.destroy();
});
