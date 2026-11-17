// 仪表盘卡片内控件（BLG-20261111-01）：在真实 SPA 上量点击区、状态、悬停与手机排布。
// 验收项编号（D / M / X）对应 docs/baselines/dashboard-card-controls/README.md。
// 本用例只断言、不写任何仓库文件；取证截图与 measurements.json 由一次性脚本生成，避免常规 e2e 运行弄脏已跟踪的基线。
// 只断言 CSS 直接定义的尺寸（28 / 44px、间距、颜色令牌），不断言字体度量决定的行高，免得 CI 的字体与本机不同而误报。
import { expect, test, type Browser, type Locator, type Page, type Route, type TestInfo } from "@playwright/test";
import {
  CARDS,
  cardLocator,
  cardSelector,
  cleanupCardData,
  commentAction,
  focusStyle,
  hasCard,
  hoverStyle,
  loginToDashboard,
  measureCard,
  measurePage,
  openCardDashboard,
  postAction,
  resetDashboardLayout,
  saveCardLayout,
  seedCardData,
  styleOf,
  type CardRecord,
  type ControlRecord,
} from "./support/dashboard-cards";
import { measureTouchTargets } from "./support/settings";

const DESKTOP = { viewport: { width: 1440, height: 900 } };
const PHONE = { viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true };
const TRANSPARENT = "rgba(0, 0, 0, 0)";

const RANGE_GROUPS = [
  { key: "trend-medium", label: "访问趋势统计范围" },
  { key: "trend-large", label: "访问趋势统计范围" },
  { key: "hourly-medium", label: "时段统计范围" },
  { key: "top-posts-medium", label: "热门文章统计范围" },
] as const;

const COMMENT_ACTIONS = ["通过", "回复", "标记垃圾", "删除"] as const;

/** 整行宽度的行级按钮（列表行本身）不是本任务的小控件，它们的高度（41px 等）改前就是这样。 */
const ROW_BUTTON = /recent-post-main|recent-mini-row/;
/**
 * 命中测试里允许出现的 blocked：点击区补到邻居为止、不抢邻居的点击。
 * 目前只有两类：小档（107px 高）卡片头的「全部」被紧邻的第一行挡在 35px，以及行级按钮之间。
 */
const ALLOWED_BLOCKED = /^button\.recent-mini-row|^button\.apple-link「全部」/;

function baseURLOf(testInfo: TestInfo): string | undefined {
  return testInfo.project.use.baseURL;
}

test.beforeAll(async ({ browser }, testInfo) => {
  testInfo.setTimeout(120_000);
  const context = await browser.newContext({ baseURL: baseURLOf(testInfo), ...DESKTOP });
  try {
    const page = await context.newPage();
    await loginToDashboard(page);
    await seedCardData(page);
    await saveCardLayout(page);
  } finally {
    await context.close();
  }
});

test.afterAll(async ({ browser }, testInfo) => {
  const context = await browser.newContext({ baseURL: baseURLOf(testInfo), ...DESKTOP });
  try {
    const page = await context.newPage();
    await loginToDashboard(page);
    // 还原默认布局并清掉夹具内容，后面共用同一个服务的用例不该看到它们
    await resetDashboardLayout(page);
    await cleanupCardData(page);
  } finally {
    await context.close();
  }
});

async function open(browser: Browser, baseURL: string | undefined, options: object) {
  const context = await browser.newContext({ baseURL, ...options });
  const page = await context.newPage();
  await loginToDashboard(page);
  await openCardDashboard(page);
  return { context, page };
}

/** 切视图模式，并等画布宽度的 0.3s 过渡与重排落定。 */
async function previewMode(page: Page, name: "电脑" | "平板" | "手机", bp: string, width: number) {
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.locator(".dashboard-apple")).toHaveAttribute("data-bp", bp);
  await expect(page.locator(".dashboard-canvas")).toHaveCSS("width", `${width}px`);
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(500);
}

/** 把设计令牌解析成浏览器实际使用的 rgb 字符串，避免在用例里写死色值。 */
async function tokenColor(page: Page, token: string): Promise<string> {
  return page.evaluate((name) => {
    const probe = document.createElement("div");
    probe.style.color = `var(${name})`;
    document.body.appendChild(probe);
    const color = getComputedStyle(probe).color;
    probe.remove();
    return color;
  }, token);
}

function control(record: CardRecord, name: string | RegExp): ControlRecord {
  const found = record.controls.find((item) => (typeof name === "string" ? item.name === name : name.test(item.name)));
  if (!found) throw new Error(`${record.key} 里没有按钮「${String(name)}」，实际有：${record.controls.map((item) => item.name).join("、")}`);
  return found;
}

function inGroup(record: CardRecord, label: string): ControlRecord[] {
  return record.controls.filter((item) => item.groupLabel === label);
}

/** 选中的下划线（.card-range-label 的 ::after）：样式，以及它底边相对按钮竖直中心的偏移。 */
async function rangeUnderline(button: Locator) {
  return button.evaluate((el) => {
    const label = el.querySelector(".card-range-label") as HTMLElement;
    const after = getComputedStyle(label, "::after");
    const box = el.getBoundingClientRect();
    return {
      height: after.height,
      background: after.backgroundColor,
      position: after.position,
      belowCenter: Math.round((label.getBoundingClientRect().bottom - (box.top + box.height / 2)) * 100) / 100,
    };
  });
}

/** 文字 16px 行高 + 3px 下边距：下划线底边离按钮中心 9.5px，与按钮是 28px 还是 44px 高无关。 */
const UNDERLINE_BELOW_CENTER = 9.5;

async function popoverButtons(popover: Locator) {
  return popover.evaluate((root) => ({
    width: Math.round(root.getBoundingClientRect().width),
    buttons: [...root.querySelectorAll<HTMLElement>("button")].map((el) => {
      const box = el.getBoundingClientRect();
      return {
        name: (el.textContent || "").trim(),
        className: el.className,
        width: Math.round(box.width * 100) / 100,
        height: Math.round(box.height * 100) / 100,
        minHeight: getComputedStyle(el).minHeight,
      };
    }),
  }));
}

/** 所有卡片：不裁切、没有横向溢出（点击区伪元素不能把卡片或列表撑出滚动区）。 */
async function expectNoOverflow(page: Page) {
  const pageBox = await measurePage(page);
  expect(pageBox.scrollWidth, "页面横向滚动").toBeLessThanOrEqual(pageBox.clientWidth);
  for (const spec of CARDS) {
    if (!(await hasCard(page, spec.key))) continue;
    const record = await measureCard(page, spec.key);
    const box = record.box!;
    expect(box.scrollWidth, `${spec.key} 横向溢出`).toBeLessThanOrEqual(box.clientWidth);
    expect(box.scrollHeight, `${spec.key} 被裁切`).toBeLessThanOrEqual(box.clientHeight + 1);
    for (const list of record.lists) {
      expect(list.scrollWidth, `${spec.key} 的 ${list.className.slice(0, 30)} 横向滚动`).toBeLessThanOrEqual(list.clientWidth);
    }
  }
}

/**
 * 完整布局里（11 张卡）盒子不足 44px、靠扩大区补点击区的小控件，补到 44px 的下限：
 * 改前 / 把 --card-hit 钉回 28px / 去掉 ::before 扩大区这三种情况下该数都是 0，当前实现是 44，
 * 所以它能挡住「没量到控件、检查空转通过」。留一点余量，是因为最近说说取决于 moments 模块是否启用。
 */
const MIN_REACHED_44 = 40;

/** 命中测试：小控件的点击区不被截断、不抢邻居的点击（行级按钮与小档卡片头的 blocked 见常量说明）。 */
async function expectTouchTargets(page: Page) {
  let reached = 0;
  for (const spec of CARDS) {
    if (!(await hasCard(page, spec.key))) continue;
    const report = await measureTouchTargets(page, cardSelector(spec));
    reached += report.reached44;
    expect(
      report.uncovered.filter((line) => !ROW_BUTTON.test(line)),
      `${spec.key} 点击区被截断`,
    ).toEqual([]);
    expect(report.stolen, `${spec.key} 抢了邻居的点击`).toEqual([]);
    expect(
      report.blocked.filter((line) => !ALLOWED_BLOCKED.test(line)),
      `${spec.key} 点击区被意料之外的控件挡住`,
    ).toEqual([]);
  }
  expect(reached, "点击区补到 44px 的小控件个数").toBeGreaterThanOrEqual(MIN_REACHED_44);
}

/** .dashboard-apple 上解析出的 --card-hit。 */
async function cardHit(page: Page): Promise<string> {
  return page.locator(".dashboard-apple").evaluate((el) => getComputedStyle(el).getPropertyValue("--card-hit").trim());
}

/**
 * 每个小控件在盒子不足 44px 的方向上，::before 补到 ≥44px。
 * 横向滚动的画布（手机上看「电脑」布局）里控件会被滚到视口边缘，命中测试在边缘处量不到外侧，
 * 所以验证 44px 触发源时直接读计算样式，不依赖滚动位置。返回检查过的控件个数，供调用方断言「确实检查到了控件」。
 */
async function expectZonesAre44(page: Page, key: string): Promise<number> {
  const { checked, short } = await cardLocator(page, key).evaluate((root) => {
    const out: string[] = [];
    let count = 0;
    for (const el of root.querySelectorAll<HTMLElement>("button.card-hit, a.card-hit")) {
      const box = el.getBoundingClientRect();
      if (box.width < 1 || box.height < 1) continue;
      count += 1;
      const zone = getComputedStyle(el, "::before");
      const width = Math.max(box.width, parseFloat(zone.width) || 0);
      const height = Math.max(box.height, parseFloat(zone.height) || 0);
      if (width < 43.5 || height < 43.5) out.push(`${el.getAttribute("aria-label") || (el.textContent || "").trim()} ${width}×${height}`);
    }
    return { checked: count, short: out };
  });
  expect(short, `${key} 里点击区不足 44px 的控件`).toEqual([]);
  return checked;
}

test("D1–D2 趋势图例：PV 与大档的图例只是文字，只有中档的 UV 是开关（aria-pressed），点击区高 ≥28px", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const foreground = await tokenColor(page, "--color-foreground");
    const medium = cardLocator(page, "trend-medium");
    const record = await measureCard(page, "trend-medium");
    expect(record.dataBp).toBe("lg");
    expect(record.controls.map((item) => item.name)).not.toContain("PV");
    const uv = control(record, "UV");
    expect(uv).toMatchObject({ tag: "button", type: "button", ariaPressed: "false", fontSize: "11px" });
    expect(uv.hit.height, "UV 点击区高").toBeGreaterThanOrEqual(28);
    await expect(medium.locator(".trend-legend-static")).toHaveText("PV");
    await expect(medium.getByRole("button", { name: "PV", exact: true })).toHaveCount(0);

    const toggle = medium.getByRole("button", { name: "UV", exact: true });
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
    const shadow = await toggle.evaluate((el) => getComputedStyle(el).boxShadow);
    expect(shadow, "选中的 UV 文字下有前景色 1px 线").toContain(foreground);
    expect(shadow).toContain("inset");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "false");

    const large = cardLocator(page, "trend-large");
    await expect(large.locator(".trend-legend-static")).toHaveCount(2);
    await expect(large.locator(".trend-legend button")).toHaveCount(0);
    await expect(large.locator(".trend-legend [aria-pressed]")).toHaveCount(0);
  } finally {
    await context.close();
  }
});

test("D3 范围切换（趋势中/大、访问时段、热门文章）：12px、带名称的 group、aria-pressed、紧贴文字的 1px 下划线、点击区高 ≥28px", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const foreground = await tokenColor(page, "--color-foreground");
    const tertiary = await tokenColor(page, "--color-text-tertiary");
    const ring = await tokenColor(page, "--color-ring");
    for (const { key, label } of RANGE_GROUPS) {
      const buttons = cardLocator(page, key).getByRole("group", { name: label }).getByRole("button");
      await expect(buttons, `${key} 的范围按钮`).toHaveText(["7 天", "30 天"]);
      await expect(buttons.nth(0)).toHaveAttribute("aria-pressed", "true");
      await expect(buttons.nth(1)).toHaveAttribute("aria-pressed", "false");

      const record = await measureCard(page, key);
      const [active, idle] = inGroup(record, label);
      for (const item of [active, idle]) {
        expect(item, `${key} ${item.name}`).toMatchObject({ tag: "button", type: "button", fontSize: "12px" });
        expect(item.hit.height, `${key} ${item.name} 点击区高`).toBeGreaterThanOrEqual(28);
        expect(item.box.height, `${key} ${item.name} 盒高`).toBeCloseTo(28, 0);
      }
      expect(active.color, `${key} 选中色`).toBe(foreground);
      expect(idle.color, `${key} 未选中色`).toBe(tertiary);

      const underline = await rangeUnderline(buttons.nth(0));
      expect(underline, `${key} 下划线`).toMatchObject({ height: "1px", background: foreground, position: "absolute" });
      expect(Math.abs(underline.belowCenter - UNDERLINE_BELOW_CENTER), `${key} 下划线贴着文字`).toBeLessThanOrEqual(1);
    }

    // 切换：选中态与下划线跟着走，再切回去
    const trend = cardLocator(page, "trend-medium").getByRole("group", { name: "访问趋势统计范围" }).getByRole("button");
    await trend.nth(1).click();
    await expect(trend.nth(1)).toHaveAttribute("aria-pressed", "true");
    await expect(trend.nth(0)).toHaveAttribute("aria-pressed", "false");
    expect((await rangeUnderline(trend.nth(1))).height).toBe("1px");
    await trend.nth(0).click();
    await expect(trend.nth(0)).toHaveAttribute("aria-pressed", "true");

    // 悬停变前景色、键盘聚焦 2px ring
    expect((await hoverStyle(trend.nth(1))).color).toBe(foreground);
    const focus = await focusStyle(trend.nth(1));
    expect(focus.outline).toBe(`2px solid ${ring}`);
    expect(focus.outlineOffset).toBe("2px");
  } finally {
    await context.close();
  }
});

test("D4/D11 桌面 1440：所有卡片不被裁切，卡片与列表都没有横向溢出", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    await expectNoOverflow(page);
  } finally {
    await context.close();
  }
});

test("D5 「全部 ›」：12px、28px 高的幽灵小按钮，悬停 accent 底，键盘聚焦 2px ring，点击区高 ≥28px", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const accent = await tokenColor(page, "--color-accent");
    const foreground = await tokenColor(page, "--color-foreground");
    const ring = await tokenColor(page, "--color-ring");
    for (const key of ["recent-posts-large", "recent-comments-large", "recent-moments"]) {
      if (!(await hasCard(page, key))) continue;
      const link = cardLocator(page, key).locator(".card-link");
      await expect(link, `${key} 的卡片头链接`).toHaveAttribute("type", "button");
      await expect(link).toHaveClass(/apple-link/);
      const record = await measureCard(page, key);
      const item = control(record, /^(全部|查看全部)$/);
      expect(item.fontSize, `${key} 字号`).toBe("12px");
      expect(item.box.height, `${key} 盒高`).toBeCloseTo(28, 0);
      expect(item.hit.height, `${key} 点击区高`).toBeGreaterThanOrEqual(28);
      const hover = await hoverStyle(link);
      expect(hover.background, `${key} 悬停底`).toBe(accent);
      expect(hover.color, `${key} 悬停色`).toBe(foreground);
      const focus = await focusStyle(link);
      expect(focus.outline, `${key} 聚焦环`).toBe(`2px solid ${ring}`);
    }
  } finally {
    await context.close();
  }
});

test("D6 文章行操作：28×28、各有名称，桌面未悬停时隐藏、悬停该行或键盘聚焦时出现，悬停按钮只换底色不出边框", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const secondary = await tokenColor(page, "--color-secondary-700");
    const ring = await tokenColor(page, "--color-ring");
    const record = await measureCard(page, "recent-posts-large");
    const buttons = inGroup(record, "文章快捷操作");
    expect(buttons.length, "每行两枚操作").toBeGreaterThanOrEqual(2);
    expect(buttons.map((item) => item.title).slice(0, 2)).toEqual(["编辑", "前台预览"]);
    for (const item of buttons) {
      expect(item.box, `${item.name} 盒子`).toMatchObject({ width: 28, height: 28 });
      expect(item.ariaLabel, "每枚按钮有带文章名的可读名称").toMatch(/^(编辑|前台预览) .+/);
    }

    const row = cardLocator(page, "recent-posts-large").locator(".apple-list-item").first();
    const rail = row.locator(".recent-post-actions");
    await row.scrollIntoViewIfNeeded();
    await page.mouse.move(2, 2);
    await expect(rail, "未悬停时隐藏").toHaveCSS("opacity", "0");
    await row.hover();
    await expect(rail, "悬停该行时出现").toHaveCSS("opacity", "1");

    const hover = await hoverStyle(postAction(page, "编辑"));
    expect(hover.background, "悬停底").toBe(secondary);
    expect(hover.borderColor, "悬停不再出边框").toBe(TRANSPARENT);
    expect(hover).toMatchObject({ width: 28, height: 28 });

    await page.mouse.move(2, 2);
    await expect(rail).toHaveCSS("opacity", "0");
    const focus = await focusStyle(postAction(page, "编辑"));
    expect(focus.outline).toBe(`2px solid ${ring}`);
    await expect(rail, "键盘聚焦进入时出现").toHaveCSS("opacity", "1");
  } finally {
    await context.close();
  }
});

test("D7 评论行操作：28×28、悬停只出中性底，图标色分别是成功 / 主色 / 警告 / 错误，不再有彩色底与彩边", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const secondary = await tokenColor(page, "--color-secondary-700");
    const colors: Record<(typeof COMMENT_ACTIONS)[number], string> = {
      通过: await tokenColor(page, "--color-state-success"),
      回复: await tokenColor(page, "--color-primary"),
      标记垃圾: await tokenColor(page, "--color-state-warning"),
      删除: await tokenColor(page, "--color-state-error"),
    };
    const record = await measureCard(page, "recent-comments-large");
    const actions = inGroup(record, "评论操作").slice(0, 4);
    expect(actions.map((item) => item.title)).toEqual([...COMMENT_ACTIONS]);
    for (const item of actions) {
      expect(item.box, `${item.title} 盒子`).toMatchObject({ width: 28, height: 28 });
      expect(item.ariaLabel, `${item.title} 的可读名称`).toBeTruthy();
    }

    for (const name of COMMENT_ACTIONS) {
      const hover = await hoverStyle(commentAction(page, name));
      expect(hover.color, `${name} 悬停图标色`).toBe(colors[name]);
      expect(hover.background, `${name} 悬停底是中性浅底`).toBe(secondary);
      expect(hover.borderColor, `${name} 悬停不画彩边`).toBe(TRANSPARENT);
    }
  } finally {
    await context.close();
  }
});

test("D8 忙碌态：点「通过」并挂起状态请求，只有它的图标在转，其余三枚禁用变淡，请求结束后恢复", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const hold: { route: Route | null } = { route: null };
    await page.route("**/api/v1/comments/*/status", (route) => {
      hold.route = route;
    });
    await commentAction(page, "通过", 3).click();
    await expect.poll(() => hold.route !== null, { message: "状态请求要被挂起" }).toBe(true);
    await expect(commentAction(page, "通过", 3)).toBeDisabled();

    for (const name of COMMENT_ACTIONS) {
      const button = commentAction(page, name, 3);
      const spinning = (await button.locator("svg.animate-spin").count()) > 0;
      await expect(button, `${name} 处理中整行禁用`).toBeDisabled();
      if (name === "通过") {
        expect(spinning, "被点的那枚转").toBe(true);
        await expect(button, "被点的那枚不变淡").toHaveCSS("opacity", "1");
      } else {
        expect(spinning, `${name} 不转`).toBe(false);
        await expect(button, `${name} 变淡`).toHaveCSS("opacity", "0.5");
      }
    }

    await hold.route?.continue();
    await page.unroute("**/api/v1/comments/*/status");
    await expect(page.locator(".comment-inline-actions button:disabled")).toHaveCount(0);
    await expect(commentAction(page, "通过", 3).locator("svg.animate-spin")).toHaveCount(0);
  } finally {
    await context.close();
  }
});

test("D9 评论弹窗：按钮是 VButton xs 的 ghost / primary / danger（24px 高），弹窗另起一行不被压窄，旧的 .comment-popover-btn 不再存在", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const root = cardLocator(page, "recent-comments-large");
    await commentAction(page, "回复", 0).click();
    const reply = root.locator(".comment-reply-popover");
    await expect(reply).toBeVisible();
    const replyInfo = await popoverButtons(reply);
    expect(replyInfo.width, "回复弹窗不被压成窄条").toBeGreaterThanOrEqual(240);
    expect(replyInfo.buttons.map((item) => item.name)).toEqual(["取消", "发送"]);
    expect(replyInfo.buttons[0].className).toMatch(/btn-ghost/);
    expect(replyInfo.buttons[1].className).toMatch(/btn-primary/);
    for (const item of replyInfo.buttons) {
      expect(item.className, `${item.name} 是 VButton xs`).toMatch(/btn-xs/);
      expect(item.height, `${item.name} 高`).toBe(24);
    }
    await expect(page.locator(".comment-popover-btn")).toHaveCount(0);
    await reply.getByRole("button", { name: "取消" }).click();
    await expect(reply).toHaveCount(0);

    await commentAction(page, "删除", 0).click();
    const del = root.locator(".comment-delete-popover");
    await expect(del).toBeVisible();
    const delInfo = await popoverButtons(del);
    expect(delInfo.width, "删除确认不被压成窄条").toBeGreaterThanOrEqual(240);
    expect(delInfo.buttons.map((item) => item.name)).toEqual(["取消", "确认删除"]);
    expect(delInfo.buttons[0].className).toMatch(/btn-ghost/);
    expect(delInfo.buttons[1].className).toMatch(/btn-danger/);
    for (const item of delInfo.buttons) {
      expect(item.className, `${item.name} 是 VButton xs`).toMatch(/btn-xs/);
      expect(item.height, `${item.name} 高`).toBe(24);
    }
    await del.getByRole("button", { name: "取消" }).click();
    await expect(del).toHaveCount(0);
  } finally {
    await context.close();
  }
});

test("D10 热门文章行悬停：accent 浅底，不描边、不位移，箭头淡入", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    const accent = await tokenColor(page, "--color-accent");
    const row = cardLocator(page, "top-posts-medium").locator(".top-post-row").first();
    await row.scrollIntoViewIfNeeded();
    await page.mouse.move(2, 2);
    await page.waitForTimeout(300);
    const idle = await styleOf(row);
    expect(idle).toMatchObject({ background: TRANSPARENT, borderColor: TRANSPARENT, transform: "none" });

    const hover = await hoverStyle(row);
    expect(hover.background, "悬停底").toBe(accent);
    expect(hover.borderColor, "悬停不描边").toBe(TRANSPARENT);
    expect(hover.transform, "悬停不位移").toBe("none");
    await expect(row.locator(".top-post-arrow")).toHaveCSS("opacity", "1");
  } finally {
    await context.close();
  }
});

test("D12 桌面 800 预览（data-bp=sm）：卡片控件仍是 28px 点击区，不套用手机的 44px", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    await previewMode(page, "平板", "sm", 800);
    expect(await cardHit(page)).toBe("28px");
    for (const { key, label } of RANGE_GROUPS) {
      for (const item of inGroup(await measureCard(page, key), label)) {
        expect(item.box.height, `${key} ${item.name} 盒高`).toBeCloseTo(28, 0);
      }
    }
    const comments = await measureCard(page, "recent-comments-large");
    for (const item of inGroup(comments, "评论操作")) expect(item.box, `${item.title} 盒子`).toMatchObject({ width: 28, height: 28 });
    await expectNoOverflow(page);
  } finally {
    await context.close();
  }
});

test("M1 手机 390：逐控件命中测试，小控件点击区 ≥44px、不被截断、不抢邻居的点击", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    expect(await page.evaluate(() => window.matchMedia("(hover: hover)").matches), "触屏没有悬停能力").toBe(false);
    expect(await cardHit(page)).toBe("44px");
    await expectTouchTargets(page);
  } finally {
    await context.close();
  }
});

test("M2/M8 手机 390 范围切换：盒子高 44px 且仍在卡片内，选中下划线紧贴文字（与桌面同一偏移），卡片头不被撑开", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    for (const { key, label } of RANGE_GROUPS) {
      const record = await measureCard(page, key);
      const items = inGroup(record, label);
      expect(items, `${key} 的范围按钮`).toHaveLength(2);
      for (const item of items) {
        expect(item.box.height, `${key} ${item.name} 盒高`).toBeGreaterThanOrEqual(43.5);
        expect(item.box.x + item.box.width, `${key} ${item.name} 不超出卡片右缘`).toBeLessThanOrEqual(record.box!.width + 0.5);
      }
      const underline = await rangeUnderline(cardLocator(page, key).getByRole("group", { name: label }).getByRole("button").nth(0));
      expect(underline.height).toBe("1px");
      expect(Math.abs(underline.belowCenter - UNDERLINE_BELOW_CENTER), `${key} 下划线贴着文字`).toBeLessThanOrEqual(1);
    }
    // 访问时段、热门文章的头行本来就是 44px 高：补点击区没有布局代价
    for (const key of ["hourly-medium", "top-posts-medium"]) {
      expect((await measureCard(page, key)).headerHeight, `${key} 头行高`).toBeLessThanOrEqual(44.5);
    }
  } finally {
    await context.close();
  }
});

test("M3–M4 手机 390 行操作：常显、28px 视觉、16px 间距，评论内容区仍 ≥150px", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    const posts = await measureCard(page, "recent-posts-large");
    for (const row of posts.rows) expect(row.actionsOpacity, "文章行操作在触屏上常显").toBe("1");
    const postButtons = inGroup(posts, "文章快捷操作");
    expect(postButtons.length).toBeGreaterThanOrEqual(2);
    for (const item of postButtons) expect(item.box, `${item.name} 盒子`).toMatchObject({ width: 28, height: 28 });
    expect(postButtons[1].box.x - postButtons[0].box.x, "两枚间距 16px（28 + 16）").toBeCloseTo(44, 0);
    expect(posts.rows[0].actionsWidth, "文章操作栏宽").toBeCloseTo(72, 0);

    const comments = await measureCard(page, "recent-comments-large");
    for (const row of comments.rows) expect(row.actionsOpacity, "评论行操作在触屏上常显").toBe("1");
    const commentButtons = inGroup(comments, "评论操作");
    for (const item of commentButtons) expect(item.box, `${item.title} 盒子`).toMatchObject({ width: 28, height: 28 });
    for (let index = 1; index < 4; index += 1) {
      expect(commentButtons[index].box.x - commentButtons[index - 1].box.x, `评论操作第 ${index} 枚间距 16px`).toBeCloseTo(44, 0);
    }
    expect(comments.rows[0].actionsWidth, "评论操作栏宽（4 × 28 + 3 × 16）").toBeCloseTo(160, 0);
    for (const row of comments.rows) expect(row.mainWidth!, "评论内容区").toBeGreaterThanOrEqual(150);
  } finally {
    await context.close();
  }
});

test("M5 手机 390 评论弹窗：VButton 高 ≥44px，点击区达标，弹窗不被压成窄条", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    const root = cardLocator(page, "recent-comments-large");
    await commentAction(page, "回复", 0).click();
    const reply = root.locator(".comment-reply-popover");
    await expect(reply).toBeVisible();
    const replyInfo = await popoverButtons(reply);
    expect(replyInfo.width, "回复弹窗宽").toBeGreaterThanOrEqual(240);
    for (const item of replyInfo.buttons) expect(item.height, `${item.name} 高`).toBeGreaterThanOrEqual(44);
    const replyReport = await measureTouchTargets(page, ".comment-reply-popover");
    expect(replyReport.uncovered).toEqual([]);
    expect(replyReport.stolen).toEqual([]);
    await reply.getByRole("button", { name: "取消" }).click();
    await expect(reply).toHaveCount(0);

    await commentAction(page, "删除", 0).click();
    const del = root.locator(".comment-delete-popover");
    await expect(del).toBeVisible();
    const delInfo = await popoverButtons(del);
    expect(delInfo.width, "删除确认宽").toBeGreaterThanOrEqual(240);
    for (const item of delInfo.buttons) expect(item.height, `${item.name} 高`).toBeGreaterThanOrEqual(44);
    const delReport = await measureTouchTargets(page, ".comment-delete-popover");
    expect(delReport.uncovered).toEqual([]);
    expect(delReport.stolen).toEqual([]);
    await del.getByRole("button", { name: "取消" }).click();
    await expect(del).toHaveCount(0);
  } finally {
    await context.close();
  }
});

for (const width of [390, 375, 360]) {
  test(`M6 手机 ${width}：页面、卡片与大档列表都没有横向滚动`, async ({ browser, baseURL }) => {
    const { context, page } = await open(browser, baseURL, { ...PHONE, viewport: { width, height: 844 } });
    try {
      await expectNoOverflow(page);
    } finally {
      await context.close();
    }
  });
}

test("M7 触屏（hover: hover 不成立）：悬停样式不生效，控件不残留悬停底或高亮色", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    expect(await page.evaluate(() => window.matchMedia("(hover: hover)").matches)).toBe(false);
    const secondaryText = await tokenColor(page, "--color-text-secondary");
    const success = await tokenColor(page, "--color-state-success");

    expect((await hoverStyle(postAction(page, "编辑"))).background, "文章行操作").toBe(TRANSPARENT);
    const approve = await hoverStyle(commentAction(page, "通过"));
    expect(approve.background, "评论行操作").toBe(TRANSPARENT);
    expect(approve.color, "评论行操作的图标不变成语义色").not.toBe(success);
    const link = await hoverStyle(cardLocator(page, "recent-posts-large").locator(".card-link"));
    expect(link.background, "卡片头链接").toBe(TRANSPARENT);
    expect(link.color, "卡片头链接的文字不残留亮色").toBe(secondaryText);
    const range = cardLocator(page, "trend-medium").getByRole("group", { name: "访问趋势统计范围" }).getByRole("button").nth(1);
    const tertiary = await tokenColor(page, "--color-text-tertiary");
    expect((await hoverStyle(range)).color, "未选中的范围按钮").toBe(tertiary);
    expect((await hoverStyle(cardLocator(page, "top-posts-medium").locator(".top-post-row").first())).background, "热门文章行").toBe(TRANSPARENT);
  } finally {
    await context.close();
  }
});

test("X1 真机视口 390 + 「电脑」预览（data-bp=lg，走真实 ≤767px 视口）：卡片控件的点击区仍补到 44px，弹窗按钮 44px", async ({ browser, baseURL }) => {
  const { context, page } = await open(browser, baseURL, PHONE);
  try {
    await previewMode(page, "电脑", "lg", 1440);
    expect(await cardHit(page)).toBe("44px");
    let checked = 0;
    for (const spec of CARDS) {
      if (await hasCard(page, spec.key)) checked += await expectZonesAre44(page, spec.key);
    }
    expect(checked, "检查过的卡片内小控件个数").toBeGreaterThanOrEqual(MIN_REACHED_44);

    const reply = cardLocator(page, "recent-comments-large").locator(".comment-reply-popover");
    await commentAction(page, "回复", 0).click();
    await expect(reply).toBeVisible();
    for (const item of (await popoverButtons(reply)).buttons) expect(item.minHeight, `${item.name} min-height`).toBe("44px");
  } finally {
    await context.close();
  }
});

test("X2 桌面 1440 + 「手机」预览（data-bp=xxs，走 data-bp 这一路）：命中测试 ≥44px，弹窗 VButton min-height 为 44px", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await open(browser, baseURL, DESKTOP);
  try {
    await previewMode(page, "手机", "xxs", 390);
    expect(await cardHit(page)).toBe("44px");
    await expectTouchTargets(page);
    await expectNoOverflow(page);

    const reply = cardLocator(page, "recent-comments-large").locator(".comment-reply-popover");
    await commentAction(page, "回复", 0).click();
    await expect(reply).toBeVisible();
    for (const item of (await popoverButtons(reply)).buttons) expect(item.height, `${item.name} 高`).toBeGreaterThanOrEqual(44);
  } finally {
    await context.close();
  }
});
