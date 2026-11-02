import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("Stellar 标签编辑器识别动态 links/moments 并刷新预览", () => {
  const tagSource = read("components/editor/stellar-tag-syntax.ts");
  const viewSource = read("components/editor/StellarTagView.vue");
  const singleTagsStart = tagSource.indexOf("const SINGLE_TAGS = [");
  const singleTagsEnd = tagSource.indexOf("];", singleTagsStart);

  assert.ok(singleTagsStart >= 0, "缺少单行标签白名单");
  assert.ok(singleTagsEnd > singleTagsStart, "单行标签白名单未闭合");
  assert.match(tagSource.slice(singleTagsStart, singleTagsEnd), /"links"/);
  assert.match(tagSource.slice(singleTagsStart, singleTagsEnd), /"moments"/);
  const pairedStart = tagSource.indexOf("const PAIRED_TAGS = [");
  const pairedEnd = tagSource.indexOf("];", pairedStart);
  assert.match(tagSource.slice(pairedStart, pairedEnd), /"table"/);
  assert.match(tagSource.slice(pairedStart, pairedEnd), /"tip"/);
  assert.match(read("components/editor/stellar-tag.ts"), /wrapShortcodes\(state\.src\)/);
  // 预览改走共享调度器：raw/context 分别防抖，迟到响应与缓存由引擎统一处理。
  assert.match(viewSource, /usePluginPreview/);
  assert.match(viewSource, /preview\.scheduleRender/);
  assert.match(viewSource, /profile:\s*null/);
  assert.match(viewSource, /preview\.srcdoc/);
  // 高度校验统一交给 composable，NodeView 不再维护第二套 iframeH 规则。
  assert.match(viewSource, /preview\.onMessage\(event,\s*iframeRef\.value\)/);
  assert.match(viewSource, /preview\.frameHeight\.value/);
  assert.doesNotMatch(viewSource, /iframeH/);
  assert.doesNotMatch(viewSource, /window\.utils\s*=/);
  // 计划要求的平滑高度过渡（150ms）覆盖四处预览。
  for (const path of [
    "components/editor/StellarTagView.vue",
    "modules/plugins/ShortcodeInsertPanel.vue",
    "modules/plugins/ShortcodeStudioView.vue",
    "modules/posts/EditorToolbar.vue",
  ]) {
    const source = read(path);
    assert.ok(
      source.includes("transition-[height] duration-150") || source.includes("transition: height 0.15s"),
      `${path} 缺少 150ms 高度过渡`,
    );
  }
  // 无效参数保留旧预览时必须说明它没有更新。
  for (const path of ["modules/plugins/ShortcodeInsertPanel.vue", "modules/plugins/ShortcodeStudioView.vue"]) {
    const source = read(path);
    assert.match(source, /previewInvalidNoticeVisible/);
    assert.match(source, /参数无效，预览未更新/);
  }
});
