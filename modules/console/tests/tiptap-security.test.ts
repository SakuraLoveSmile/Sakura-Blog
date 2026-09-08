import { test } from "node:test";
import assert from "node:assert/strict";
import { mergeAttributes } from "../web/node_modules/@tiptap/core/dist/index.js";
import { DOMSerializer } from "../web/node_modules/@tiptap/pm/model/dist/index.js";

test("tiptap security · mergeAttributes ignores own __proto__ key (GHSA-cp6q-959q-f8rh)", () => {
  const maliciousInput = JSON.parse(`{
    "__proto__": {
      "data-inherited-canary": "present",
      "src": "x-invalid://canary",
      "onerror": "globalThis.__tiptapXss += 1"
    },
    "id": "safe-id",
    "class": "custom-class"
  }`);

  const attrs = mergeAttributes(maliciousInput);

  // 原型未被篡改，依然是标准的 Object.prototype
  assert.equal(Object.getPrototypeOf(attrs), Object.prototype);

  // 恶意属性未被继承或作为自有属性存在
  assert.equal(attrs["data-inherited-canary"], undefined);
  assert.equal(attrs["src"], undefined);
  assert.equal(attrs["onerror"], undefined);

  // for...in 枚举不会产生恶意属性
  const enumeratedKeys: string[] = [];
  for (const key in attrs) {
    enumeratedKeys.push(key);
  }
  assert.ok(!enumeratedKeys.includes("data-inherited-canary"));
  assert.ok(!enumeratedKeys.includes("src"));
  assert.ok(!enumeratedKeys.includes("onerror"));

  // 正常属性依然正确保留与合并
  assert.equal(attrs["id"], "safe-id");
  assert.equal(attrs["class"], "custom-class");
});

test("tiptap security · DOMSerializer does not render inherited prototype attributes", () => {
  const maliciousInput = JSON.parse(`{
    "__proto__": {
      "data-inherited-canary": "present",
      "src": "x-invalid://canary",
      "onerror": "globalThis.__tiptapXss += 1"
    },
    "alt": "safe-alt"
  }`);

  const attrs = mergeAttributes(maliciousInput);

  const appliedAttributes: Record<string, string> = {};
  const mockDoc = {
    createElement(tag: string) {
      return {
        tagName: tag,
        setAttribute(k: string, v: string) {
          appliedAttributes[k] = v;
        },
        appendChild() {},
      };
    },
    createTextNode(text: string) {
      return { text };
    },
  };

  DOMSerializer.renderSpec(mockDoc as never, ["img", attrs]);

  // 断言仅合法属性被写入 DOM，原型注入属性均被阻断
  assert.equal(appliedAttributes["alt"], "safe-alt");
  assert.equal(appliedAttributes["src"], undefined);
  assert.equal(appliedAttributes["onerror"], undefined);
  assert.equal(appliedAttributes["data-inherited-canary"], undefined);
});

test("tiptap regression · legitimate class, style, and data attribute merging", () => {
  const base = {
    class: "btn primary",
    style: "color: red; margin: 4px",
    "data-id": "1",
  };
  const override = {
    class: "large primary",
    style: "margin: 8px; background: blue",
    "data-type": "action",
  };

  const merged = mergeAttributes(base, override);
  assert.equal(merged["class"], "btn primary large");
  assert.ok(merged["style"].includes("color: red"));
  assert.ok(merged["style"].includes("margin: 8px"));
  assert.ok(merged["style"].includes("background: blue"));
  assert.equal(merged["data-id"], "1");
  assert.equal(merged["data-type"], "action");
});
