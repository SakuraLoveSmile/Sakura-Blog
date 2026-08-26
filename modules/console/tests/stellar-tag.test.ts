import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("Stellar 标签编辑器识别 links 并刷新站点友链预览", () => {
  const tagSource = read("components/editor/stellar-tag.ts");
  const viewSource = read("components/editor/StellarTagView.vue");
  const singleTagsStart = tagSource.indexOf("const SINGLE_TAGS = [");
  const singleTagsEnd = tagSource.indexOf("];", singleTagsStart);

  assert.ok(singleTagsStart >= 0, "缺少单行标签白名单");
  assert.ok(singleTagsEnd > singleTagsStart, "单行标签白名单未闭合");
  assert.match(tagSource.slice(singleTagsStart, singleTagsEnd), /"links"/);
  assert.match(tagSource, /SINGLE_TAGS\.join\("\|"\)/);
  assert.match(viewSource, /tagNameOf\(key\)\.toLowerCase\(\) !== "links"/);
  assert.match(viewSource, /if \(cacheable\) previewCache\.set\(key, data\.html\)/);
});
