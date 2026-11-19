import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { isFullDocument } from "../web/src/components/editor/raw-html-paste.ts";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("RawHtml 节点契约与 StellarTag 同构，序列化原样回写", () => {
  const source = read("components/editor/raw-html.ts");
  assert.match(source, /name:\s*"rawHtml"/);
  assert.match(source, /group:\s*"block"/);
  assert.match(source, /atom:\s*true/);
  assert.match(source, /draggable:\s*true/);
  assert.match(source, /tag:\s*`div\[\$\{RAW_HTML_ATTR\}\]`/);
  // 原样回写 + closeBlock，不做任何转义或重建。
  assert.match(source, /state\.write\(node\.attrs\.raw\)/);
  assert.match(source, /state\.closeBlock\(node\)/);
  assert.match(source, /registerRawHtmlRule\(md\)/);
  // 拖拽归属复用既有实现，不另造一套。
  assert.match(source, /withOwnerDocumentDragEvents/);
  assert.match(source, /rawHtmlPastePlugin\(\)/);
});

test("行内节点不 closeBlock，mark/sub/sup 用 open/close 原样回写", () => {
  const source = read("components/editor/raw-html-inline.ts");
  assert.match(source, /name:\s*"rawHtmlInline"/);
  assert.match(source, /group:\s*"inline"/);
  assert.match(source, /inline:\s*true/);
  assert.match(source, /atom:\s*true/);
  assert.match(source, /registerRawHtmlInlineRule\(md\)/);
  const serialize = source.slice(source.indexOf("serialize(state: any, node: any)"));
  assert.match(serialize, /state\.write\(node\.attrs\.raw\)/);
  assert.doesNotMatch(serialize.slice(0, 120), /closeBlock/);
  assert.match(source, /open:\s*`<\$\{tag\}>`/);
  assert.match(source, /close:\s*`<\/\$\{tag\}>`/);
  for (const tag of ["mark", "sub", "sup"]) {
    assert.ok(source.includes(`htmlMark("${tag === "mark" ? "markHighlight" : tag === "sub" ? "subscript" : "superscript"}", "${tag}")`), `缺少 ${tag} Mark`);
  }
});

test("两个节点与三个 Mark 都注册进宿主编辑器", () => {
  const wrapper = read("components/RichTextEditor.vue");
  assert.match(wrapper, /:extensions="\[StellarTag, RawHtml, RawHtmlInline, MarkHighlight, Subscript, Superscript\]"/);
});

test("NodeView 复用共享预览调度与 opaque sandbox，不另造协议", () => {
  const view = read("components/editor/RawHtmlView.vue");
  assert.match(view, /usePluginPreview/);
  assert.match(view, /sharedPreviewQueue/);
  assert.match(view, /new PreviewCache\(20\)/);
  assert.match(view, /profile:\s*null/);
  assert.match(view, /withPreviewContentWidth/);
  assert.match(view, /sandbox="allow-scripts"/);
  assert.doesNotMatch(view, /allow-same-origin/);
  assert.match(view, /preview\.onMessage\(event,\s*iframeRef\.value\)/);
  assert.match(view, /ownerDocument\.defaultView/);
  assert.match(view, /transition-\[height\] duration-150/);
  assert.match(view, /data-drag-handle/);
  // 源码编辑内联在 NodeView 内，避免跨 document Teleport。
  assert.match(view, /aria-label="HTML 源码"/);
  assert.doesNotMatch(view, /VModal/);
});

test("插入弹窗复用 VModal 与共享预览，提交走既有 insert 通道", () => {
  const modal = read("components/editor/RawHtmlInsertModal.vue");
  assert.match(modal, /VModal/);
  assert.match(modal, /usePluginPreview/);
  assert.match(modal, /sandbox="allow-scripts"/);
  assert.doesNotMatch(modal, /allow-same-origin/);
  assert.match(modal, /id="raw-html-source"/);
  // 高度回传必须绑定，否则预览停在初始高度造成双层滚动。
  assert.match(modal, /preview\.onMessage\(event,\s*frameRef\.value\)/);
  assert.match(modal, /@load="bindFrameMessages"/);
  assert.match(modal, /ownerDocument\.defaultView/);
  const toolbar = read("modules/posts/EditorToolbar.vue");
  assert.match(toolbar, /title="插入外部 HTML"/);
  assert.match(toolbar, /ri:html5-line/);
  assert.match(toolbar, /emit\("insert",\s*`\\n\$\{raw\}\\n`\)/);
  assert.match(toolbar, /<RawHtmlInsertModal v-model:visible="htmlInsertOpen" @submit="insertRawHtml" \/>/);
});

test("粘贴判定：整文档保真，片段逐块判定", () => {
  assert.equal(isFullDocument("<!DOCTYPE html><html><body>x</body></html>"), true);
  assert.equal(isFullDocument('<html lang="zh"><body>x</body></html>'), true);
  assert.equal(isFullDocument('<div class="x">片段</div>'), false);
  assert.equal(isFullDocument("<p>普通</p>"), false);
  const paste = read("components/editor/raw-html-paste.ts");
  // 与 core rule 共用同一判定，避免两套规则漂移。
  assert.match(paste, /isForeignHtml\(serialized\)/);
  assert.match(paste, /transformPastedHTML/);
});