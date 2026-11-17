// 仪表盘卡片内控件的源码契约（BLG-20261111-01）：图例 / 范围切换 / 「全部 ›」/ 行操作 / 评论弹窗 / 热门行悬停。
// 可量测的尺寸、点击区与交互由 tests/e2e/dashboard-card-controls.spec.ts 在真实页面里验证，这里只守住不会悄悄退化的结构约定。
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

const WIDGETS = "modules/console/web/src/modules/dashboard/widgets";
const TREND = `${WIDGETS}/TrendChartWidget.vue`;
const HOURLY = `${WIDGETS}/HourlyChartWidget.vue`;
const TOP_POSTS = `${WIDGETS}/TopPostsWidget.vue`;
const RECENT_POSTS = `${WIDGETS}/RecentPostsWidget.vue`;
const RECENT_COMMENTS = `${WIDGETS}/RecentCommentsWidget.vue`;
const RECENT_MOMENTS = `${WIDGETS}/RecentMomentsWidget.vue`;
const DESIGNER = "modules/console/web/src/modules/dashboard/DashboardDesignerView.vue";
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

/** 单个顶层规则块的正文（不含嵌套媒体查询）。 */
function ruleBody(css: string, selector: string): string {
  const start = css.indexOf(`${selector} {`);
  assert.notEqual(start, -1, `缺少规则 ${selector}`);
  const open = css.indexOf("{", start);
  const close = css.indexOf("}", open);
  return css.slice(open + 1, close);
}

test("范围切换：访问趋势、访问时段、热门文章统一为 .card-range 分组，按钮有 type 与 aria-pressed，不再借用全站分段控件", () => {
  const cases: { file: string; label: string; day: string }[] = [
    { file: TREND, label: "访问趋势统计范围", day: "days" },
    { file: HOURLY, label: "时段统计范围", day: "trendDays" },
    { file: TOP_POSTS, label: "热门文章统计范围", day: "days" },
  ];
  for (const { file, label, day } of cases) {
    const source = read(file);
    assert.match(source, new RegExp(`class="card-range" role="group" aria-label="${label}"`), `${file} 的范围切换要是带名称的 role=group`);
    assert.equal((source.match(/class="card-range-btn card-hit"/g) ?? []).length, 2, `${file} 有 7 天 / 30 天两个按钮`);
    assert.equal((source.match(/<button type="button" class="card-range-btn card-hit"/g) ?? []).length, 2, `${file} 的范围按钮要有 type="button"`);
    for (const value of [7, 30]) {
      assert.match(source, new RegExp(`:aria-pressed="${day} === ${value}"`));
      assert.match(source, new RegExp(`:class="\\{ 'is-active': ${day} === ${value} \\}"`));
      assert.match(source, new RegExp(`@click="${day} = ${value}"`));
    }
    assert.equal((source.match(/class="card-range-label"/g) ?? []).length, 2, "下划线画在文字外层的 label 上，才会紧贴文字而不是按钮底边");
    for (const legacy of ["apple-segmented", "seg-btn", "hourly-range-switch", "top-posts-range"]) {
      assert.ok(!source.includes(legacy), `${file} 不应再使用 ${legacy}`);
    }
  }
  // 注释里可以提到旧类名，规则里不能再有
  const rules = read(CSS).replace(/\/\*[\s\S]*?\*\//g, "");
  assert.doesNotMatch(rules, /\.apple-segmented/, "仪表盘不再有 .apple-segmented 的覆盖规则（全站规则在 apple-base.css，本任务不动）");
  assert.doesNotMatch(rules, /seg-btn/);
});

test("范围切换样式：12px、盒子高度跟 --card-hit、选中色 + 文字下 1px 线，悬停只在有悬停能力的设备上", () => {
  const css = read(CSS);
  const btn = ruleBody(css, ".card-range-btn");
  assert.match(btn, /min-height: var\(--card-hit, 28px\);/);
  assert.match(btn, /font-size: 12px;/);
  assert.match(btn, /color: var\(--color-text-tertiary\);/);
  assert.doesNotMatch(btn, /transition: all/);
  assert.match(css, /\.card-range-btn\.is-active,\s*\.card-range-btn:active \{\s*color: var\(--color-foreground\);/);
  assert.match(css, /\.card-range-btn:focus-visible \{\s*outline: 2px solid var\(--color-ring\);\s*outline-offset: 2px;/);

  const underline = css.match(/\.card-range-btn\.is-active \.card-range-label::after \{([^}]*)\}/);
  assert.ok(underline, "缺少选中下划线规则");
  assert.match(underline[1], /height: 1px;/);
  assert.match(underline[1], /background: var\(--color-foreground\);/);

  const hover = extractBlocks(css, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.card-range-btn:hover/);
  assert.doesNotMatch(hover.rest, /\.card-range-btn:hover/);

  // 范围组贴着卡片右缘，而卡片是 overflow:hidden：点击区只能向左补，向右补出去既点不到又会让卡片多出横向溢出
  const zone = css.match(/\.card-range-btn\.card-hit::before \{([^}]*)\}/);
  assert.ok(zone, "缺少范围按钮的点击区规则");
  assert.match(zone[1], /right: -2px;/);
  assert.match(zone[1], /left: auto;/);
  assert.match(zone[1], /transform: translateY\(calc\(-50% \+ var\(--hit-dy, 0px\)\)\);/);
  assert.match(css, /\.card-range-btn\.card-hit:last-child::before \{\s*right: 0;/);
});

test("访问趋势图例：PV 只是静态图例，UV 是唯一的开关（仅中档，aria-pressed = 折线是否显示），大档两个都静态", () => {
  const source = read(TREND);
  assert.match(source, /<span class="trend-legend-static"><span class="trend-legend-line pv" \/>PV<\/span>/);
  assert.match(source, /<span v-else class="trend-legend-static"><span class="trend-legend-line uv" \/>UV<\/span>/);
  assert.equal((source.match(/class="trend-legend-toggle card-hit"/g) ?? []).length, 1, "整个图例里只有 UV 一个开关");
  assert.match(source, /v-if="sizePreset === 'medium'"[\s\S]{0,120}class="trend-legend-toggle card-hit"/);
  assert.match(source, /:aria-pressed="showUv"/);
  assert.match(source, /@click="showUv = !showUv"/);
  assert.doesNotMatch(source, /showUv = false/, "PV 不再是会改 showUv 的按钮");

  const css = read(CSS);
  assert.ok(css.length > 0);
  const style = source.slice(source.indexOf("<style"));
  assert.match(style, /\.trend-legend-toggle\.active \{\s*box-shadow: inset 0 -1px 0 var\(--color-foreground\);/);
  // 大屏布局里 UV 与范围切换同排、只隔 16px，手机 44px 模式下「7 天」的点击区向左补得更远：UV 要压在它上面，否则 4 个点被抢走。
  assert.match(style, /\.trend-legend-toggle \{[^}]*z-index: 1;/);
  const hover = extractBlocks(style, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.trend-legend-toggle:hover/);
  assert.doesNotMatch(hover.rest, /\.trend-legend-toggle:hover/);
});

test("「全部 ›」：卡片头的链接在 .apple-link 之上补 .card-link（28px 点击区、头行高不变），设计页的 .apple-link 不动", () => {
  for (const file of [RECENT_POSTS, RECENT_COMMENTS, RECENT_MOMENTS]) {
    const source = read(file);
    assert.match(source, /<button type="button" class="apple-link card-link card-hit text-xs" @click=/, `${file} 的卡片头链接`);
    assert.doesNotMatch(source, /<button @click=[^>]*class="apple-link/, "卡片头链接要有 type=button");
  }
  assert.match(read(DESIGNER), /<button class="apple-link" @click="importLegacy">导入当前浏览器布局<\/button>/);

  const css = read(CSS);
  const link = ruleBody(css, ".card-link");
  assert.match(link, /min-height: 28px;/);
  assert.match(link, /margin-block: -4\.5px -5\.5px;/, "负外边距抵掉多出的高度（上比下少 0.5px，手机点击区正好落在 [0, 44]），头行高不变");
  assert.match(link, /--hit-dy: 0px;/);
  assert.doesNotMatch(link, /transition: all/);
  // 小档头部与第一行只隔 4px：点击区整体上挪、按钮盒不下移
  const small = css.match(/\.recent-posts-small \.card-link,\s*\.recent-comments-small \.card-link \{([^}]*)\}/);
  assert.ok(small, "缺少小档卡片头链接的规则");
  assert.match(small[1], /--hit-dy: -9px;/);
  assert.match(small[1], /margin-block: -5px;/);
  // .apple-link 规则本身不变
  const base = ruleBody(css, ".apple-link");
  assert.match(base, /font-size: 12px;/);
  assert.match(base, /color: var\(--color-text-secondary\);/);
  assert.doesNotMatch(base, /min-height/);
  assert.match(css, /\.card-link:focus-visible \{\s*outline: 2px solid var\(--color-ring\);\s*outline-offset: 2px;/);
  const hover = extractBlocks(css, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.card-link:hover \{\s*color: var\(--color-foreground\);\s*background: var\(--color-accent\);/);
  assert.doesNotMatch(hover.rest, /\.card-link:hover \{\s*color: var\(--color-foreground\);\s*background/);
  // 触屏上 .apple-link:hover 的文字亮色不残留
  assert.match(extractBlocks(css, "@media (hover: none)").blocks.join("\n"), /\.card-link:hover \{\s*color: var\(--color-text-secondary\);/);
});

test("点击区：44px 来自真实 ≤767px 视口与 data-bp 预览两个触发源，视觉盒子不变，点击区由 ::before 补", () => {
  const css = read(CSS);
  assert.match(css, /\.dashboard-apple \{\s*--card-hit: 28px;/);
  const phone = extractBlocks(css, "@media (max-width: 767px)").blocks.find((block) => block.includes("--card-hit"));
  assert.ok(phone, "缺少真实手机视口下的 --card-hit");
  assert.match(phone, /\.dashboard-apple \{[^}]*--card-hit: 44px;[^}]*--card-rail-gap: 16px;[^}]*--card-rail-gap-post: 16px;[^}]*--card-post-inset: 8px;[^}]*--card-comment-inset: 8px;/);
  const preview = css.match(/\.dashboard-apple:is\(\[data-bp="xs"\], \[data-bp="xxs"\]\) \{\s*--card-hit: 44px;([^}]*)\}/);
  assert.ok(preview, "缺少 data-bp 预览手机布局下的 --card-hit");
  assert.match(preview[1], /--card-rail-gap: 16px;/);
  assert.match(preview[1], /--card-rail-gap-post: 16px;/);
  assert.match(preview[1], /--card-post-inset: 8px;/);
  // 手机布局里评论行自带内边距，不再额外左移
  assert.match(preview[1], /--card-comment-inset: 0px;/);

  const hit = ruleBody(css, ".card-hit::before");
  assert.match(hit, /content: "";/);
  assert.match(hit, /width: max\(100%, var\(--card-hit, 0px\)\);/);
  assert.match(hit, /height: max\(100%, var\(--card-hit, 0px\)\);/);
  assert.match(hit, /transform: translate\(-50%, calc\(-50% \+ var\(--hit-dy, 0px\)\)\);/);
  assert.match(ruleBody(css, ".card-hit"), /position: relative;/);

  // VButton 自带的 44px 只认真实视口，评论弹窗里的按钮按 data-bp 补齐
  assert.match(
    css,
    /\.dashboard-apple:is\(\[data-bp="xs"\], \[data-bp="xxs"\]\) :is\(\.comment-reply-popover, \.comment-delete-popover\) \.btn \{\s*min-height: 44px;/,
  );
});

test("文章行操作：与评论行同一条显隐规则、28px 幽灵按钮、各有可读名称，悬停不再出边框", () => {
  const source = read(RECENT_POSTS);
  assert.match(source, /<div class="recent-post-actions" role="group" aria-label="文章快捷操作">/);
  assert.equal((source.match(/class="post-action card-hit"/g) ?? []).length, 2);
  assert.match(source, /title="编辑" :aria-label="`编辑 \$\{item\.title\}`"/);
  assert.match(source, /title="前台预览" :aria-label="`前台预览 \$\{item\.title\}`"/);

  const style = source.slice(source.indexOf("<style"));
  // 隐藏规则只存在于「能悬停且 ≥1024px」媒体查询里，其它设备常显
  const hidden = extractBlocks(style, "@media (hover: hover) and (min-width: 1024px)");
  assert.match(hidden.blocks.join("\n"), /\.recent-post-actions \{\s*opacity: 0;/);
  assert.match(hidden.blocks.join("\n"), /\.apple-list-item:focus-within \.recent-post-actions/);
  assert.doesNotMatch(hidden.rest, /opacity: 0;/, "行操作不再有无条件的 opacity: 0（触屏上看不见却点得到的隐形热区）");
  assert.match(style, /gap: var\(--card-rail-gap-post, 6px\);/);
  assert.match(style, /margin-right: var\(--card-post-inset, 0px\);/);

  const button = ruleBody(style, ".post-action");
  assert.match(button, /width: 28px;/);
  assert.match(button, /height: 28px;/);
  assert.match(button, /border: 1px solid transparent;/);
  assert.doesNotMatch(button, /transition: all/);
  assert.match(style, /\.post-action:focus-visible \{\s*outline: 2px solid var\(--color-ring\);\s*outline-offset: 2px;/);
  const hover = extractBlocks(style, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.post-action:hover \{[^}]*background: var\(--color-secondary-700\);/);
  assert.doesNotMatch(hover.blocks.join("\n"), /border-color/, "悬停只换底色，不再出 1px 边框");
  assert.doesNotMatch(hover.rest, /\.post-action:hover/);
});

test("评论行操作：28px、中性悬停底、只有图标变语义色；处理中只转被点的那枚，其余禁用变淡", () => {
  const source = read(RECENT_COMMENTS);
  assert.match(source, /<div class="comment-inline-actions" role="group" aria-label="评论操作">/);
  for (const action of ["action-approve", "action-reply", "action-spam", "action-delete"]) {
    assert.match(source, new RegExp(`class="${action} card-hit"`));
  }
  assert.match(source, /:aria-label="`通过 \$\{item\.nick\} 的评论`"/);
  assert.match(source, /:aria-label="`回复 \$\{item\.nick\}`"/);
  assert.match(source, /:aria-label="`标记 \$\{item\.nick\} 的评论为垃圾`"/);
  assert.match(source, /:aria-label="`删除 \$\{item\.nick\} 的评论`"/);

  // 忙碌态：另有 busyAction，actionBusy 的含义（哪一行在处理）保持不变；回复不占用转动图标
  assert.match(source, /const actionBusy = ref<number \| null>\(null\);/);
  assert.match(source, /const busyAction = ref<"approved" \| "spam" \| "delete" \| null>\(null\);/);
  assert.match(source, /function isBusy\(item: CommentItem, action: "approved" \| "spam" \| "delete"\): boolean \{\s*return actionBusy\.value === item\.id && busyAction\.value === action;/);
  assert.match(source, /busyAction\.value = action;/);
  assert.match(source, /actionBusy\.value = null;\s*busyAction\.value = null;/);
  assert.match(source, /<Icon icon="ri:reply-line" \/>/);
  assert.doesNotMatch(source, /actionBusy === item\.id \? 'ri:loader-4-line'/, "不能再让同一行四枚图标一起换成 loader");
  assert.equal((source.match(/:class="\{ 'animate-spin': isBusy\(item, '(approved|spam|delete)'\) \}"/g) ?? []).length, 3);
  assert.equal((source.match(/:disabled="actionBusy === item\.id"/g) ?? []).length, 5, "四枚图标按钮与确认删除，处理中整行禁用");

  const style = source.slice(source.indexOf("<style"));
  const button = ruleBody(style, ".comment-inline-actions button");
  assert.match(button, /width: 28px;/);
  assert.match(button, /height: 28px;/);
  assert.match(button, /border: 1px solid transparent;/);
  assert.doesNotMatch(button, /transition: all/);
  assert.match(style, /gap: var\(--card-rail-gap, 2px\);/);
  assert.match(style, /margin-right: var\(--card-comment-inset, 0px\);/);
  assert.match(style, /\.comment-inline-actions button:disabled\[aria-busy="true"\] \{\s*opacity: 1;/);
  assert.match(style, /\.comment-inline-actions button:disabled \{\s*cursor: progress;\s*opacity: 0\.5;/);

  // 悬停只在有悬停能力的设备上；语义色只落在图标文字色上，不再有彩色底与彩边
  const hover = extractBlocks(style, "@media (hover: hover)");
  const hoverCss = hover.blocks.join("\n");
  assert.match(hoverCss, /\.comment-inline-actions button:hover:not\(:disabled\) \{[^}]*background: var\(--color-secondary-700\);/);
  assert.match(hoverCss, /\.action-approve:hover:not\(:disabled\) \{ color: var\(--color-state-success\); \}/);
  assert.match(hoverCss, /\.action-reply:hover:not\(:disabled\) \{ color: var\(--color-primary\); \}/);
  assert.match(hoverCss, /\.action-spam:hover:not\(:disabled\) \{ color: var\(--color-state-warning\); \}/);
  assert.match(hoverCss, /\.action-delete:hover:not\(:disabled\) \{ color: var\(--color-state-error\); \}/);
  assert.doesNotMatch(style, /color-mix\(in srgb, var\(--color-state-/, "语义色不再混进悬停底");
  assert.doesNotMatch(style, /border-color: var\(--color-state-/, "语义色不再画悬停边框");
  assert.doesNotMatch(hover.rest, /\.comment-inline-actions button[^{]*:hover/);

  // 弹窗另起一行，手机上正文不会被压成一条缝
  assert.match(style, /\.apple-card\.recent-comments-large \.apple-comment-item \{\s*flex-wrap: wrap;\s*row-gap: 0;/);
  assert.match(style, /\.comment-reply-popover,\s*\.comment-delete-popover \{\s*flex: 1 1 100%;/);
});

test("评论弹窗按钮：VButton xs 的 ghost / primary / danger，不再有独立的 .comment-popover-btn", () => {
  const source = read(RECENT_COMMENTS);
  assert.match(source, /import \{ Toast, VButton \} from "@\/components\/ui";/);
  assert.match(source, /<VButton class="card-hit" type="ghost" size="xs" @click="cancelReply">取消<\/VButton>/);
  assert.match(source, /<VButton class="card-hit" type="primary" size="xs" :disabled="!replyText\.trim\(\)" :loading="replyBusy === item\.id" @click="sendReply\(item\)">/);
  assert.match(source, /<template #icon><Icon icon="ri:send-plane-line" class="w-3 h-3" \/><\/template>/);
  assert.match(source, /<VButton class="card-hit" type="ghost" size="xs" @click="deleteConfirmId = null">取消<\/VButton>/);
  assert.match(source, /<VButton class="card-hit" type="danger" size="xs" :loading="isBusy\(item, 'delete'\)"/);
  assert.ok(!source.includes("comment-popover-btn"), "旧弹窗按钮样式要整块清掉");
});

test("热门文章行悬停：中性浅底，不描边、不位移，悬停只在有悬停能力的设备上", () => {
  const source = read(TOP_POSTS);
  const style = source.slice(source.indexOf("<style"));
  const row = style.match(/\.top-post-row \{([^}]*)\}/);
  assert.ok(row, "缺少 .top-post-row 规则");
  assert.match(row[1], /border: 1px solid transparent;/);
  assert.match(row[1], /transition: background-color var\(--duration-fast\) var\(--ease-standard\);/);
  assert.doesNotMatch(row[1], /transform|border-color/);
  assert.doesNotMatch(style, /translateX/);

  const hover = extractBlocks(style, "@media (hover: hover)");
  assert.match(hover.blocks.join("\n"), /\.top-post-row:hover \{ background: var\(--color-accent\); \}/);
  assert.match(hover.blocks.join("\n"), /\.top-post-row:hover \.top-post-arrow \{ opacity: 1; \}/);
  assert.doesNotMatch(hover.rest, /\.top-post-row:hover/);
  assert.match(style, /\.top-post-row:focus-visible \{ outline: 2px solid var\(--color-ring\); outline-offset: -2px; \}/);
  assert.match(style, /\.top-post-row:focus-visible \.top-post-arrow \{ opacity: 1; \}/);
  assert.match(style, /\.top-posts-small \.card-range \{ display: none; \}/);
  // 排名徽标的彩色底不动
  assert.match(style, /\.top-post-rank\.rank-1 \{ color: var\(--color-state-warning\); background: color-mix\(in srgb, var\(--color-state-warning\) 13%, transparent\); \}/);
});
