// 仪表盘头部控件的源码契约（BLG-20261110-07）：状态语义、提示取值、44px 双触发与旧样式清理。
// 可量测的尺寸与交互由 tests/e2e/dashboard-header-controls.spec.ts 在真实页面里验证，这里只守住不会悄悄退化的结构约定。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

const VIEW = "modules/console/web/src/modules/dashboard/DashboardView.vue";
const CSS = "modules/console/web/src/modules/dashboard/dashboard.css";

/** 取出所有以 header 开头的 CSS 块（按花括号配平），返回块正文与去掉这些块后的剩余文本。 */
function extractBlocks(css: string, header: string): { blocks: string[]; rest: string } {
  const blocks: string[] = [];
  let rest = css;
  for (;;) {
    const start = rest.indexOf(header);
    if (start === -1) break;
    const open = rest.indexOf("{", start);
    let depth = 0;
    let end = -1;
    for (let i = open; i < rest.length; i += 1) {
      if (rest[i] === "{") depth += 1;
      if (rest[i] === "}") {
        depth -= 1;
        if (depth === 0) {
          end = i;
          break;
        }
      }
    }
    assert.notEqual(end, -1, `${header} 的花括号没有闭合`);
    blocks.push(rest.slice(open + 1, end));
    rest = rest.slice(0, start) + rest.slice(end + 1);
  }
  return { blocks, rest };
}

test("仪表盘头部：视图切换与刷新是带状态语义的幽灵工具按钮，不再借用全站分段控件和原生 title", () => {
  const view = read(VIEW);
  assert.match(view, /class="dashboard-view-switch" role="group" aria-label="视图模式"/);
  assert.match(view, /<button\s+type="button"\s+:class="\['dashboard-tool-btn', mode\.key === viewMode && 'is-active'\]"/);
  assert.match(view, /:aria-label="mode\.label"/);
  assert.match(view, /:aria-pressed="mode\.key === viewMode"/);
  assert.match(view, /:aria-describedby="`dashboard-tip-\$\{mode\.key\}`"/);
  assert.match(view, /class="dashboard-tool-btn dashboard-refresh-btn"/);
  assert.match(view, /:aria-busy="refreshing"/);
  // 提示只在视觉上补充：悬停/聚焦时显示，原生 title 会与它叠成两个提示
  assert.equal((view.match(/role="tooltip"/g) ?? []).length, 2, "视图切换与刷新各有一个提示");
  assert.doesNotMatch(view, /\stitle=/);
  for (const legacy of ["apple-segmented", "seg-btn", "apple-icon-btn", "apple-edit-btn"]) {
    assert.ok(!view.includes(legacy), `头部不应再使用 ${legacy}`);
  }
});

test("仪表盘头部：预览宽度提示取 VIEW_MODE_WIDTH，不另写一份数字", () => {
  const view = read(VIEW);
  assert.match(view, /import \{ provideBreakpoint, VIEW_MODE_WIDTH, type ViewMode \} from "\.\/useBreakpoint"/);
  for (const key of ["lg", "sm", "xxs"]) {
    assert.match(view, new RegExp(`预览宽度 \\$\\{VIEW_MODE_WIDTH\\.${key}\\}px`));
  }
  assert.doesNotMatch(view, /\b(1440|800|390)\b/);
});

test("仪表盘头部：设置布局走 VButton ghost 直达设计页，图标尺寸区分带文字 14px 与纯图标 16px", () => {
  const view = read(VIEW);
  assert.match(view, /<VButton type="ghost" :route="\{ path: '\/dashboard\/designer' \}">/);
  assert.doesNotMatch(view, /useRouter/);
  // 带文字的按钮（继续写草稿 / 撰写文章 / 设置布局）图标 14px
  assert.equal((view.match(/<Icon icon="ri:[a-z0-9-]+" class="w-3\.5 h-3\.5" \/>/g) ?? []).length, 3);
  // 纯图标的工具按钮图标 16px
  assert.match(view, /<Icon :icon="mode\.icon" class="w-4 h-4" \/>/);
  assert.match(view, /:icon="refreshing \? 'ri:loader-4-line' : 'ri:refresh-line'"\s+class="w-4 h-4"/);
});

test("仪表盘头部样式：44px 来自真实手机视口与 data-bp 预览两个触发源，提示悬停只在有悬停能力的设备上出现", () => {
  const css = read(CSS);
  assert.match(css, /\.dashboard-tool-btn \{\s*--dashboard-tool-size: 36px;/);

  // 真实 ≤767px 视口
  const phoneMedia = extractBlocks(css, "@media (max-width: 767px)").blocks.find((block) =>
    block.includes(".dashboard-tool-btn"),
  );
  assert.ok(phoneMedia, "缺少真实手机视口下的工具按钮 44px 规则");
  assert.match(phoneMedia, /\.dashboard-tool-btn \{\s*--dashboard-tool-size: 44px;/);
  // 桌面预览手机 / 极窄布局（data-bp）
  assert.match(
    css,
    /\.dashboard-apple:is\(\[data-bp="xs"\], \[data-bp="xxs"\]\) \.dashboard-tool-btn \{\s*--dashboard-tool-size: 44px;/,
  );
  // VButton 自带的 44px 只认真实视口，头部按钮按 data-bp 补齐
  assert.match(
    css,
    /\.dashboard-apple:is\(\[data-bp="xs"\], \[data-bp="xxs"\]\) :is\(\.dashboard-writing-actions, \.dashboard-display-actions\) > \.btn \{\s*min-height: 44px;/,
  );
  // 写作按钮平分整行
  assert.match(
    css,
    /\.dashboard-apple:is\(\[data-bp="xs"\], \[data-bp="xxs"\]\) \.dashboard-writing-actions > \.btn \{\s*flex: 1 1 0;/,
  );

  // 悬停提示与悬停底只在 (hover: hover) 里；键盘聚焦的提示在外面，所有设备都有
  const hover = extractBlocks(css, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.dashboard-tool-btn:hover \+ \.dashboard-tip/);
  assert.doesNotMatch(hover.rest, /\.dashboard-tool-btn:hover/);
  assert.match(hover.rest, /\.dashboard-tool-btn:focus-visible \+ \.dashboard-tip/);
});

test("仪表盘头部样式：选中态是 16×1 的短线伪元素，不再用 inset 阴影，也不用 transition: all", () => {
  const css = read(CSS);
  const active = css.match(/\.dashboard-tool-btn\.is-active::after \{([^}]*)\}/);
  assert.ok(active, "缺少选中短线规则");
  assert.match(active[1], /width: 16px;/);
  assert.match(active[1], /height: 1px;/);
  assert.match(active[1], /background: var\(--color-foreground\);/);
  assert.doesNotMatch(active[1], /box-shadow/);
  const tool = css.match(/\.dashboard-tool-btn \{([^}]*)\}/);
  assert.ok(tool);
  assert.match(tool[1], /transition: background-color var\(--duration-fast\)/);
  assert.doesNotMatch(css, /transition:\s*all/);
});

test("仪表盘头部样式：只由头部使用的旧规则已清理，设计页仍在用的 .apple-edit-btn 保留", () => {
  const css = read(CSS);
  assert.doesNotMatch(css, /\.apple-icon-btn/);
  assert.doesNotMatch(css, /(^|[^-\w])\.view-switch/m);
  assert.doesNotMatch(css, /\.dashboard-refresh-btn\[aria-busy/);
  assert.match(css, /^\.apple-edit-btn \{/m);
  assert.match(read("modules/console/web/src/modules/dashboard/DashboardDesignerView.vue"), /class="apple-edit-btn"/);
});
