// 仪表盘卡片内控件的夹具、布局与量测辅助：BLG-20261111-01 的 e2e 用例与改前/改后取证脚本共用。
// 量测只经改前改后都存在的卡片外壳（.trend-card-*、.recent-posts-*、.recent-comments-*、.top-posts-*、
// .hourly-card-*、.recent-moments-widget）与按钮的可读名称定位，不依赖本任务新增的类名，
// 因此同一份代码可以在改动前后的构建上取同口径的数据。
import { expect, type Locator, type Page } from "@playwright/test";
import { WIDGET_PRESET_FOOTPRINTS } from "../../../modules/console/web/src/modules/dashboard/widgets/layout";

export { loginToDashboard } from "./dashboard-header";

export type CardKind = "trend" | "hourly" | "top-posts" | "recent-posts" | "recent-comments" | "recent-moments";
export type CardPreset = "small" | "medium" | "large";

export interface CardSpec {
  /** 取证与断言里的稳定名字，如 `recent-comments-large`。 */
  key: string;
  kind: CardKind;
  /** `recent-moments` 没有档位。 */
  preset: CardPreset | null;
}

/** 摆进布局的卡片，按纵向顺序排列；同一张卡各占一格，互不重叠。 */
export const CARDS: CardSpec[] = [
  { key: "trend-medium", kind: "trend", preset: "medium" },
  { key: "trend-large", kind: "trend", preset: "large" },
  { key: "hourly-medium", kind: "hourly", preset: "medium" },
  { key: "top-posts-medium", kind: "top-posts", preset: "medium" },
  { key: "recent-posts-small", kind: "recent-posts", preset: "small" },
  { key: "recent-posts-medium", kind: "recent-posts", preset: "medium" },
  { key: "recent-posts-large", kind: "recent-posts", preset: "large" },
  { key: "recent-comments-small", kind: "recent-comments", preset: "small" },
  { key: "recent-comments-medium", kind: "recent-comments", preset: "medium" },
  { key: "recent-comments-large", kind: "recent-comments", preset: "large" },
  { key: "recent-moments", kind: "recent-moments", preset: null },
];

const WIDGET_ID: Record<CardKind, string> = {
  trend: "trend-chart",
  hourly: "hourly-chart",
  "top-posts": "top-posts",
  "recent-posts": "recent-posts",
  "recent-comments": "recent-comments",
  "recent-moments": "recent-moments",
};

export function card(key: string): CardSpec {
  const found = CARDS.find((item) => item.key === key);
  if (!found) throw new Error(`没有名为 ${key} 的卡片`);
  return found;
}

/** 卡片根元素的选择器（`.apple-card` 本身，带档位类）。 */
export function cardSelector(spec: CardSpec): string {
  switch (spec.kind) {
    case "trend":
      return `.trend-card-${spec.preset}`;
    case "hourly":
      return `.hourly-card-${spec.preset}`;
    case "top-posts":
      return `.top-posts-${spec.preset}`;
    case "recent-posts":
      return `.recent-posts-${spec.preset}`;
    case "recent-comments":
      return `.recent-comments-${spec.preset}`;
    case "recent-moments":
      return ".recent-moments-widget";
  }
}

export function cardLocator(page: Page, key: string): Locator {
  return page.locator(cardSelector(card(key))).first();
}

type LayoutBp = "lg" | "sm" | "xxs";

function buildLayout(bp: LayoutBp) {
  let y = 0;
  return CARDS.map((spec, index) => {
    // 最近说说没有档位，按该断点的中档足迹摆。
    const size = WIDGET_PRESET_FOOTPRINTS[bp][spec.preset ?? "medium"];
    const widget = { i: `${WIDGET_ID[spec.kind]}-${spec.preset ?? "medium"}-${index + 1}`, id: WIDGET_ID[spec.kind], x: 0, y, w: size.w, h: size.h };
    y += size.h;
    return widget;
  });
}

/**
 * 空站点的默认布局里没有「大档」的最近文章 / 最近评论，也没有趋势中档，而行操作、内联评论操作、弹窗、UV 开关
 * 只在这些档位渲染；这里用布局 API 把 CARDS 全部摆出来（桌面、平板、手机各一份）。
 */
export async function saveCardLayout(page: Page) {
  const lg = buildLayout("lg");
  const response = await page.request.put("/api/v1/dashboard/layout", {
    data: { version: 10, layouts: { lg, md: lg, sm: buildLayout("sm"), xs: buildLayout("xxs"), xxs: buildLayout("xxs") } },
  });
  expect(response.ok(), `保存布局失败：${response.status()} ${await response.text()}`).toBe(true);
}

/** 还原默认布局，避免后续经过仪表盘的用例在控制台收到系统状态组件的 perf 403。 */
export async function resetDashboardLayout(page: Page) {
  const response = await page.request.put("/api/v1/dashboard/layout", { data: { version: 10, layouts: {} } });
  expect(response.ok(), `还原布局失败：${response.status()} ${await response.text()}`).toBe(true);
}

const POST_COUNT = 8;
const COMMENT_COUNT = 9;

/**
 * 灌数据：8 篇文章（7 篇已发布、1 篇草稿）、每篇已发布文章访问 3 次产生 PV / UV、9 条评论（其中两条置为待审）。
 * 已灌过就跳过，所以同一个服务可以重复取证。评论邮箱用验收站点的 admin_email，豁免同 IP 60 秒 1 条的公开限流。
 */
export async function seedCardData(page: Page) {
  // 仪表盘的 recent-posts 接口只回最近 5 篇，不能用它数夹具；直接问第一篇夹具文章在不在。
  const seeded = await page.request.get("/api/v1/posts/card-controls-1");
  if (seeded.ok()) return;

  const published: string[] = [];
  for (let index = 1; index <= POST_COUNT; index += 1) {
    const status = index === POST_COUNT ? "draft" : "published";
    const slug = `card-controls-${index}`;
    const response = await page.request.post("/api/v1/posts", {
      data: {
        slug,
        title: `卡片控件验收文章 ${index}`,
        content_md: `用于仪表盘卡片控件验收的第 ${index} 篇文章，内容仅作占位。`,
        status,
        category: index % 2 === 0 ? "技术" : "随笔",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(response.ok(), `创建夹具文章失败：${response.status()} ${await response.text()}`).toBe(true);
    if (status === "published") published.push(slug);
  }
  for (const [order, slug] of published.entries()) {
    // 访问次数递减，热门文章里才有高低不同的条形。
    for (let hit = 0; hit < 6 - Math.min(order, 4); hit += 1) await page.request.get(`/post/${slug}`);
  }

  for (let index = 1; index <= COMMENT_COUNT; index += 1) {
    const created = await page.request.post("/api/comments", {
      data: {
        page_key: `/post/${published[index % published.length]}`,
        nick: `验收访客 ${index}`,
        email: "admin@e2e.test",
        content: `第 ${index} 条夹具评论：用来验收仪表盘评论行上的审核、回复和删除操作，文字要长到能折成两行显示。`,
      },
    });
    expect(created.ok(), `创建夹具评论失败：${created.status()} ${await created.text()}`).toBe(true);
  }
  // 评论是否进入待审取决于站点审核策略；这里显式把最近两条置为待审，保证「待审」徽标一定出现。
  const recent = await page.request.get("/api/v1/dashboard/recent-comments");
  const comments = ((await recent.json()) as { items?: { id: number; status: string }[] }).items ?? [];
  for (const item of comments.slice(0, 2)) {
    if (item.status === "pending") continue;
    const response = await page.request.put(`/api/v1/comments/${item.id}/status`, { data: { status: "pending" } });
    expect(response.ok(), `置评论待审失败：${response.status()} ${await response.text()}`).toBe(true);
  }
  // PV 走 3 秒批量落库，等缓冲 flush 后再进仪表盘。
  await page.waitForTimeout(4000);
}

/**
 * 清掉 seedCardData 灌的文章与评论：e2e 的全部用例共用一个服务和数据库，后面的用例不该看到多出来的内容
 * （文章要先软删再彻底删；PV 记录没有清理入口，只是历史计数，对其它用例无影响）。
 */
export async function cleanupCardData(page: Page) {
  for (let index = 1; index <= POST_COUNT; index += 1) {
    const slug = `card-controls-${index}`;
    await page.request.delete(`/api/v1/posts/${slug}`);
    await page.request.delete(`/api/v1/posts/${slug}/purge`);
  }
  const listed = await page.request.get("/api/v1/comments", { params: { keyword: "夹具评论", size: 100 } });
  const items = ((await listed.json()) as { items?: { id: number }[] }).items ?? [];
  for (const item of items) await page.request.delete(`/api/v1/comments/${item.id}`);
}

/** 打开仪表盘并等卡片渲染稳定：骨架屏退场、图表绘制完成。 */
export async function openCardDashboard(page: Page) {
  await page.goto("/console/dashboard");
  await page.locator(".grid-stack-item").first().waitFor({ state: "visible", timeout: 20_000 });
  await page
    .waitForFunction(() => document.querySelectorAll(".dashboard-skeleton-list").length === 0, null, { timeout: 15_000 })
    .catch(() => undefined);
  await page.waitForTimeout(600);
}

/** 当前布局里这张卡有没有渲染出来（最近说说取决于 moments 模块是否启用）。 */
export async function hasCard(page: Page, key: string): Promise<boolean> {
  return (await page.locator(cardSelector(card(key))).count()) > 0;
}

export interface ControlRecord {
  name: string;
  tag: string;
  type: string | null;
  ariaPressed: string | null;
  ariaLabel: string | null;
  title: string | null;
  className: string;
  /** 最近的带 role=group 或 aria-label 的祖先。 */
  groupRole: string | null;
  groupLabel: string | null;
  /** 相对卡片左上角的盒子。 */
  box: { x: number; y: number; width: number; height: number };
  /** 命中测试量到的点击区：沿控件中心的竖线 / 横线，连续落在该控件（含伪元素）上的长度。 */
  hit: { width: number; height: number };
  fontSize: string;
  color: string;
  background: string;
  border: string;
  opacity: string;
  disabled: boolean;
  /** 选中态下划线的两种常见画法：盒内阴影、::after。 */
  underline: { boxShadow: string; afterContent: string; afterHeight: string; afterBackground: string; afterPosition: string; afterBottom: string };
}

export interface RowRecord {
  className: string;
  height: number;
  /** 行内操作栏（文章行 / 评论行）的宽度，没有时为 null。 */
  actionsWidth: number | null;
  actionsHeight: number | null;
  actionsOpacity: string | null;
  /** 行内主内容（第一个子元素）的宽度。 */
  mainWidth: number | null;
}

export interface ListRecord {
  className: string;
  overflowX: string;
  overflowY: string;
  scrollWidth: number;
  clientWidth: number;
  scrollHeight: number;
  clientHeight: number;
}

export interface CardRecord {
  key: string;
  present: boolean;
  dataBp: string | null;
  box: { width: number; height: number; scrollWidth: number; clientWidth: number; scrollHeight: number; clientHeight: number } | null;
  /** 卡片头行（第一个子元素）的高度。 */
  headerHeight: number | null;
  controls: ControlRecord[];
  /** 整行宽度的行级按钮（列表行本身）个数，不计入 controls。 */
  rowButtons: number;
  rows: RowRecord[];
  lists: ListRecord[];
}

export interface PageRecord {
  viewport: { width: number; height: number };
  dataBp: string | null;
  canvasWidth: number;
  scrollWidth: number;
  clientWidth: number;
  hover: boolean;
  coarse: boolean;
}

export async function measurePage(page: Page): Promise<PageRecord> {
  return page.evaluate(() => {
    const canvas = document.querySelector(".dashboard-canvas") as HTMLElement | null;
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      dataBp: document.querySelector(".dashboard-apple")?.getAttribute("data-bp") ?? null,
      canvasWidth: canvas ? Math.round(canvas.getBoundingClientRect().width * 100) / 100 : 0,
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      hover: window.matchMedia("(hover: hover)").matches,
      coarse: window.matchMedia("(pointer: coarse)").matches,
    };
  });
}

/** 量取一张卡片里全部小控件与行的盒子、点击区与计算样式；卡片没渲染时 present 为 false。 */
export async function measureCard(page: Page, key: string): Promise<CardRecord> {
  const selector = cardSelector(card(key));
  return page.evaluate(
    ({ key, selector }) => {
      const root = document.querySelector(selector) as HTMLElement | null;
      if (!root) {
        return { key, present: false, dataBp: null, box: null, headerHeight: null, controls: [], rowButtons: 0, rows: [], lists: [] };
      }
      const round = (value: number) => Math.round(value * 100) / 100;
      // 卡片相对页面的位置会随滚动变化，先量卡片、再逐控件滚动到视口中央量命中；盒子一律换算成相对卡片左上角。
      const reach = (el: HTMLElement, vertical: boolean) => {
        const box = el.getBoundingClientRect();
        const cx = box.left + box.width / 2;
        const cy = box.top + box.height / 2;
        let total = 0;
        for (const sign of [-1, 1]) {
          for (let step = 0; step < 90; step += 1) {
            const offset = sign * (step + 0.5);
            const x = vertical ? cx : cx + offset;
            const y = vertical ? cy + offset : cy;
            if (x < 0 || y < 0 || x >= window.innerWidth || y >= window.innerHeight) break;
            const hit = document.elementFromPoint(x, y);
            if (!hit || !el.contains(hit)) break;
            total += 1;
          }
        }
        return total;
      };

      const header = root.firstElementChild as HTMLElement | null;
      const controls: unknown[] = [];
      let rowButtons = 0;
      for (const el of root.querySelectorAll<HTMLElement>("button, a[href]")) {
        const box0 = el.getBoundingClientRect();
        if (box0.width < 1 || box0.height < 1) continue;
        // 整行宽度的行级按钮（列表行本身、图表下方的大块点击区）不是「小控件」。
        if (box0.width >= 160 && box0.height >= 36) {
          rowButtons += 1;
          continue;
        }
        el.scrollIntoView({ block: "center", inline: "nearest", behavior: "instant" as ScrollBehavior });
        const box = el.getBoundingClientRect();
        const rootBox = root.getBoundingClientRect();
        const style = getComputedStyle(el);
        const after = getComputedStyle(el, "::after");
        const group = el.parentElement?.closest("[role=group], [aria-label]") ?? null;
        controls.push({
          name: el.getAttribute("aria-label") || el.getAttribute("title") || (el.textContent || "").trim().replace(/\s+/g, " "),
          tag: el.tagName.toLowerCase(),
          type: el.getAttribute("type"),
          ariaPressed: el.getAttribute("aria-pressed"),
          ariaLabel: el.getAttribute("aria-label"),
          title: el.getAttribute("title"),
          className: typeof el.className === "string" ? el.className : "",
          groupRole: group?.getAttribute("role") ?? null,
          groupLabel: group?.getAttribute("aria-label") ?? null,
          box: { x: round(box.left - rootBox.left), y: round(box.top - rootBox.top), width: round(box.width), height: round(box.height) },
          hit: { width: reach(el, false), height: reach(el, true) },
          fontSize: style.fontSize,
          color: style.color,
          background: style.backgroundColor,
          border: `${style.borderTopWidth} ${style.borderTopStyle} ${style.borderTopColor}`,
          opacity: style.opacity,
          disabled: (el as HTMLButtonElement).disabled === true,
          underline: {
            boxShadow: style.boxShadow,
            afterContent: after.content,
            afterHeight: after.height,
            afterBackground: after.backgroundColor,
            afterPosition: after.position,
            afterBottom: after.bottom,
          },
        });
      }
      root.scrollIntoView({ block: "start", inline: "nearest", behavior: "instant" as ScrollBehavior });

      const rows = [...root.querySelectorAll<HTMLElement>(".apple-list-item, .apple-comment-item, .top-post-row, .recent-mini-row")].map((row) => {
        const actions = row.querySelector<HTMLElement>(".recent-post-actions, .comment-inline-actions");
        const main = row.firstElementChild as HTMLElement | null;
        return {
          className: typeof row.className === "string" ? row.className : "",
          height: round(row.getBoundingClientRect().height),
          actionsWidth: actions ? round(actions.getBoundingClientRect().width) : null,
          actionsHeight: actions ? round(actions.getBoundingClientRect().height) : null,
          actionsOpacity: actions ? getComputedStyle(actions).opacity : null,
          mainWidth: main ? round(main.getBoundingClientRect().width) : null,
        };
      });
      const lists = [...root.querySelectorAll<HTMLElement>(".overflow-y-auto, .top-posts-list, .apple-comment-list")].map((list) => {
        const style = getComputedStyle(list);
        return {
          className: typeof list.className === "string" ? list.className : "",
          overflowX: style.overflowX,
          overflowY: style.overflowY,
          scrollWidth: list.scrollWidth,
          clientWidth: list.clientWidth,
          scrollHeight: list.scrollHeight,
          clientHeight: list.clientHeight,
        };
      });
      return {
        key,
        present: true,
        dataBp: root.getAttribute("data-bp"),
        box: {
          width: round(root.getBoundingClientRect().width),
          height: round(root.getBoundingClientRect().height),
          scrollWidth: root.scrollWidth,
          clientWidth: root.clientWidth,
          scrollHeight: root.scrollHeight,
          clientHeight: root.clientHeight,
        },
        headerHeight: header ? round(header.getBoundingClientRect().height) : null,
        controls,
        rowButtons,
        rows,
        lists,
      };
    },
    { key, selector },
  ) as Promise<CardRecord>;
}

export interface StyleRecord {
  color: string;
  background: string;
  borderColor: string;
  borderWidth: string;
  transform: string;
  opacity: string;
  outline: string;
  outlineOffset: string;
  width: number;
  height: number;
}

/** 取一个元素当前的计算样式；悬停 / 聚焦探测时先把鼠标移上去或聚焦，再调这个。 */
export async function styleOf(locator: Locator): Promise<StyleRecord> {
  return locator.evaluate((el) => {
    const style = getComputedStyle(el);
    const box = el.getBoundingClientRect();
    return {
      color: style.color,
      background: style.backgroundColor,
      borderColor: style.borderTopColor,
      borderWidth: style.borderTopWidth,
      transform: style.transform,
      opacity: style.opacity,
      outline: `${style.outlineWidth} ${style.outlineStyle} ${style.outlineColor}`,
      outlineOffset: style.outlineOffset,
      width: Math.round(box.width * 100) / 100,
      height: Math.round(box.height * 100) / 100,
    };
  });
}

/** 悬停探测：样式有 150ms 过渡，等过渡走完再取值。 */
export async function hoverStyle(locator: Locator): Promise<StyleRecord> {
  await locator.scrollIntoViewIfNeeded();
  await locator.hover();
  await locator.page().waitForTimeout(260);
  return styleOf(locator);
}

/** 键盘聚焦探测：先按一次 Tab 让浏览器进入键盘模式，再聚焦，才会命中 :focus-visible。 */
export async function focusStyle(locator: Locator): Promise<StyleRecord> {
  const page = locator.page();
  await page.mouse.move(2, 2);
  await locator.scrollIntoViewIfNeeded();
  await page.keyboard.press("Tab");
  await locator.focus();
  await page.waitForTimeout(260);
  return styleOf(locator);
}

/** 最近评论大档第一行里按钮的 locator（按改前改后都有的 title 定位）。 */
export function commentAction(page: Page, name: "通过" | "回复" | "标记垃圾" | "删除", row = 0): Locator {
  return cardLocator(page, "recent-comments-large").locator(".apple-comment-item").nth(row).locator(`.comment-inline-actions button[title="${name}"]`);
}

/** 最近文章大档第一行的行操作按钮。 */
export function postAction(page: Page, name: "编辑" | "前台预览", row = 0): Locator {
  return cardLocator(page, "recent-posts-large").locator(".apple-list-item").nth(row).locator(`.recent-post-actions button[title="${name}"]`);
}
