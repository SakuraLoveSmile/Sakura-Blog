import { expect, test, type Page } from "@playwright/test";

/**
 * Round 4 / T1：手机操作矩阵与真实可访问性验收。
 *
 * 边界声明：
 * - 自动截图一律使用 testInfo.outputPath(...)，执行前后 docs/baselines/ 受跟踪基线不得变化。
 * - 所有断言基于真实视口、真实滚动与真实按键/点击；元素缺失或矩形为空直接失败，不做条件跳过。
 * - 概览栅格统一断言：五个页面均在 <1024px 为两列，≥1024px 为四列，并在断点前后（390/639/640/641/767/768/1023/1024/1280）守住规范。
 */

const BOTTOM_NAV = 'nav[aria-label="移动端底部导航"]';

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function loginUserCenter(page: Page) {
  await page.goto("/uc/login");
  await page.locator("#uc-username").fill("e2e-author");
  await page.locator("#uc-password").fill("e2e-password");
  await page.locator('form button[type="submit"]').click();
  await expect(page).toHaveURL(/\/uc\/posts/, { timeout: 30_000 });
}

async function createPost(page: Page, token: string, index: number) {
  const slug = `${token}-${index}`;
  const title = `T1 夹具文章 ${token}-${index}`;
  const response = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title,
      content_md: `T1 夹具正文 ${index}`,
      status: "published",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(response.ok(), `夹具文章 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { slug, title };
}

async function createPage(page: Page, token: string, index: number) {
  const slug = `${token}-${index}`;
  const title = `T1 夹具页面 ${token}-${index}`;
  const response = await page.request.post("/api/v1/pages", {
    data: {
      slug,
      title,
      content_md: `T1 夹具页面正文 ${index}`,
      status: "published",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(response.ok(), `夹具页面 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { slug, title };
}

/** 通过真实上传接口创建图片夹具，返回服务端文件名。token 隔离套件内其他 spec 的遗留附件。 */
async function createAttachments(page: Page, count: number, token = `t1-fixture-${Date.now()}`) {
  const png = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg==",
    "base64",
  );
  const names: string[] = [];
  for (let i = 0; i < count; i++) {
    const response = await page.request.post("/api/v1/attachments/upload", {
      multipart: { file: { name: `${token}-${i}.png`, mimeType: "image/png", buffer: png } },
    });
    expect(response.ok(), `夹具附件必须上传成功（HTTP ${response.status()}）`).toBe(true);
    const payload = (await response.json()) as { filename: string };
    expect(payload.filename, "上传响应必须返回服务端文件名").toBeTruthy();
    names.push(payload.filename);
  }
  return names;
}

/** 把真实主容器滚到底，并等待滚动位置达到预期（不使用固定 sleep 作为完成证据）。 */
async function scrollMainToBottom(page: Page) {
  const geometry = await page.evaluate(() => {
    const main = document.querySelector("main.content") as HTMLElement | null;
    if (main) main.scrollTop = main.scrollHeight;
    return { scrollHeight: main?.scrollHeight ?? 0, clientHeight: main?.clientHeight ?? 0 };
  });
  expect(geometry.scrollHeight, "隔离夹具必须产生真实纵向滚动").toBeGreaterThan(geometry.clientHeight);
  const expected = geometry.scrollHeight - geometry.clientHeight;
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const main = document.querySelector("main.content") as HTMLElement | null;
          return main ? main.scrollTop : -1;
        }),
      { timeout: 8_000, message: "主容器必须真正滚动到底部" },
    )
    .toBeGreaterThanOrEqual(expected - 1);
  return geometry;
}

type Rect = { x: number; y: number; width: number; height: number };

async function bottomNavBox(page: Page): Promise<Rect> {
  const nav = page.locator(BOTTOM_NAV);
  await expect(nav).toBeVisible();
  const box = await nav.boundingBox();
  expect(box, "底栏必须有完整矩形").not.toBeNull();
  return box as Rect;
}

function assertAboveNav(rect: Rect | null, nav: Rect, label: string) {
  expect(rect, `${label} 必须有完整矩形，不允许条件跳过`).not.toBeNull();
  expect(rect!.width, `${label} 宽度必须大于 0`).toBeGreaterThan(0);
  expect(rect!.height, `${label} 高度必须大于 0`).toBeGreaterThan(0);
  expect(rect!.y, `${label} 不得越过视口顶部`).toBeGreaterThanOrEqual(0);
  expect(rect!.y + rect!.height, `${label} 必须完整位于底栏上方`).toBeLessThanOrEqual(nav.y);
}

/**
 * 在页面内把任意 CSS 颜色（含 color-mix / oklab）解析为 sRGB 并计算 WCAG 对比度，
 * 半透明前景按页面底色合成，避免把透明黑当作背景。
 */
async function contrastRatio(page: Page, foreground: string, background: string): Promise<number> {
  return page.evaluate(
    ({ fg, bg }) => {
      const canvas = document.createElement("canvas");
      canvas.width = 1;
      canvas.height = 1;
      const ctx = canvas.getContext("2d");
      if (!ctx) throw new Error("无法创建 canvas 上下文");
      const toRgb = (color: string, backdrop: string | null) => {
        ctx.clearRect(0, 0, 1, 1);
        if (backdrop) {
          ctx.fillStyle = backdrop;
          ctx.fillRect(0, 0, 1, 1);
        }
        ctx.fillStyle = color;
        ctx.fillRect(0, 0, 1, 1);
        const data = ctx.getImageData(0, 0, 1, 1).data;
        return [data[0], data[1], data[2]];
      };
      const pageBackground = window.getComputedStyle(document.body).backgroundColor;
      const luminance = (rgb: number[]) => {
        const [r, g, b] = rgb.map((channel) => {
          const c = channel / 255;
          return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
        });
        return 0.2126 * r + 0.7152 * g + 0.0722 * b;
      };
      const l1 = luminance(toRgb(fg, pageBackground));
      const l2 = luminance(toRgb(bg, pageBackground));
      const [light, dark] = l1 >= l2 ? [l1, l2] : [l2, l1];
      return (light + 0.05) / (dark + 0.05);
    },
    { fg: foreground, bg: background },
  );
}

async function readButtonColors(page: Page, selector: string) {
  return page.locator(selector).evaluate((el) => {
    const cs = window.getComputedStyle(el);
    return { color: cs.color, background: cs.backgroundColor, opacity: cs.opacity };
  });
}

/** 依次测量默认、hover、active 三态的实际计算颜色并断言普通文字对比度 ≥ 4.5:1。 */
async function assertButtonContrast(page: Page, selector: string, label: string) {
  const button = page.locator(selector);
  await expect(button).toBeVisible();

  const states: Array<{ name: string; colors: { color: string; background: string; opacity: string } }> = [];
  states.push({ name: "默认", colors: await readButtonColors(page, selector) });

  await button.hover();
  states.push({ name: "hover", colors: await readButtonColors(page, selector) });

  const box = await button.boundingBox();
  expect(box, `${label} 必须有完整矩形`).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.down();
  states.push({ name: "active", colors: await readButtonColors(page, selector) });
  // 先把指针移开再松开，避免在按钮上完成一次真实点击（会导航或打开弹层）
  await page.mouse.move(0, 0);
  await page.mouse.up();

  for (const state of states) {
    expect(state.colors.opacity, `${label} ${state.name} 态不应通过透明度降低对比度`).toBe("1");
    const ratio = await contrastRatio(page, state.colors.color, state.colors.background);
    expect(
      ratio,
      `${label} ${state.name} 态对比度必须 ≥ 4.5:1（实际 ${ratio.toFixed(2)}:1，前景 ${state.colors.color} / 背景 ${state.colors.background}）`,
    ).toBeGreaterThanOrEqual(4.5);
  }
}

const HEADER_VIEWPORTS = [
  { width: 390, height: 844, stacked: true, mobileNav: true },
  { width: 768, height: 1024, stacked: true, mobileNav: false },
  { width: 1280, height: 800, stacked: false, mobileNav: false },
  { width: 1440, height: 900, stacked: false, mobileNav: false },
];

const MATRIX_PAGES = [
  { path: "/console/posts", heading: "文章管理" },
  { path: "/console/pages", heading: "独立页面" },
  { path: "/console/attachments", heading: "媒体库" },
];

test.describe("T1 手机操作矩阵与可访问性验收", () => {
  test("三页在 390 / 768 / 1280 / 1440 四视口的页头与底栏矩阵", async ({ page }) => {
    await login(page);

    for (const target of MATRIX_PAGES) {
      for (const viewport of HEADER_VIEWPORTS) {
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await page.goto(target.path);
        await expect(page.getByRole("heading", { name: target.heading, exact: true })).toBeVisible();

        const header = page.locator("header.page-header-stack-below-lg");
        await expect(header).toBeVisible();
        const titleBox = await header.locator("h1").boundingBox();
        const searchBox = await header.locator(".apple-search").boundingBox();
        const actionsBox = await header.locator(".page-header-actions").boundingBox();
        const actionButtonBox = await header.locator(".page-header-actions .btn").first().boundingBox();
        expect(titleBox, `${target.path} ${viewport.width}px 标题矩形`).not.toBeNull();
        expect(searchBox, `${target.path} ${viewport.width}px 搜索矩形`).not.toBeNull();
        expect(actionsBox, `${target.path} ${viewport.width}px 操作组矩形`).not.toBeNull();
        expect(actionButtonBox, `${target.path} ${viewport.width}px 操作按钮矩形`).not.toBeNull();

        if (viewport.stacked) {
          // 标题 / 搜索 / 按钮组三行依次排列
          expect(
            titleBox!.y + titleBox!.height,
            `${target.path} ${viewport.width}px：标题必须在搜索之上`,
          ).toBeLessThanOrEqual(searchBox!.y);
          expect(
            searchBox!.y + searchBox!.height,
            `${target.path} ${viewport.width}px：搜索必须在按钮组之上`,
          ).toBeLessThanOrEqual(actionButtonBox!.y);
        } else {
          // 页头横排：搜索与按钮组在同一水平带内
          expect(
            Math.abs(searchBox!.y + searchBox!.height / 2 - (actionsBox!.y + actionsBox!.height / 2)),
            `${target.path} ${viewport.width}px：搜索与按钮组必须横排居中对齐`,
          ).toBeLessThanOrEqual(24);
          const overflow = await page.evaluate(
            () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
          );
          expect(overflow, `${target.path} ${viewport.width}px 不得出现横向溢出`).toBeLessThanOrEqual(1);
        }

        const nav = page.locator(BOTTOM_NAV);
        const navDisplay = await nav.evaluate((el) => window.getComputedStyle(el).display);
        if (viewport.mobileNav) {
          expect(navDisplay, `${target.path} ${viewport.width}px 底栏必须显示`).toBe("grid");
        } else {
          expect(navDisplay, `${target.path} ${viewport.width}px 底栏必须隐藏`).toBe("none");
        }

        const sidebarDisplay = await page
          .locator("aside")
          .evaluate((el) => window.getComputedStyle(el).display);
        if (viewport.width >= 768) {
          expect(sidebarDisplay, `${target.path} ${viewport.width}px 桌面侧栏必须显示`).not.toBe("none");
        }
      }
    }
  });

  test("独立页面 390px：滚动到底后最后一项操作无遮挡且进入对应编辑页", async ({ page }) => {
    await login(page);
    const token = `t1-page-${Date.now()}`;
    const fixtures = [];
    for (let i = 1; i <= 6; i++) fixtures.push(await createPage(page, token, i));

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/pages");
    await expect(page.getByRole("heading", { name: "独立页面", exact: true })).toBeVisible();

    // 用夹具 token 过滤，保证页面上的卡片全部来自本轮夹具
    await page.getByPlaceholder("搜索页面标题或 Slug...").fill(token);
    const cards = page.locator(".page-record");
    await expect(cards).toHaveCount(fixtures.length);

    const nav = await bottomNavBox(page);
    await scrollMainToBottom(page);

    const lastCard = cards.last();
    await expect(lastCard).toBeVisible();
    const lastTitle = ((await lastCard.locator(".record-title").textContent()) ?? "").trim();
    const mapped = fixtures.find((item) => item.title === lastTitle);
    expect(mapped, `末项必须对应本轮夹具，实际标题=${lastTitle}`).toBeTruthy();

    const lastEdit = lastCard.locator('button[title="编辑页面"]');
    await expect(lastEdit).toBeVisible();
    const rect = await lastEdit.boundingBox();
    assertAboveNav(rect, nav, "独立页面末项编辑操作");

    await lastEdit.click();
    await expect(page).toHaveURL(new RegExp(`/console/pages/edit/${mapped!.slug}`), { timeout: 15_000 });
  });

  test("媒体库 390px：滚动到底后最后一项可打开对应附件详情", async ({ page }) => {
    await login(page);
    const token = `t1-att-${Date.now()}`;
    const names = await createAttachments(page, 12, token);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/attachments");
    await expect(page.getByRole("heading", { name: "媒体库", exact: true })).toBeVisible();
    await expect(page.locator(".attachment-card").first()).toBeVisible();

    // 用夹具 token 过滤：套件内其他 spec 的遗留附件不参与末项断言（列表按创建时间倒序）
    await page.getByPlaceholder("搜索文件名或 URL...").fill(token);
    await expect(page.locator(".attachment-card")).toHaveCount(names.length);

    const nav = await bottomNavBox(page);
    await scrollMainToBottom(page);

    // 末项 = DOM 中最后一张真实卡片；其名称与上传返回的夹具文件名核对，不猜测序号
    const target = await page.evaluate(() => {
      const cards = Array.from(document.querySelectorAll<HTMLElement>(".attachment-card"));
      const last = cards[cards.length - 1];
      if (!last) return null;
      const rect = last.getBoundingClientRect();
      return {
        index: cards.length - 1,
        name: last.querySelector("img[alt]")?.getAttribute("alt") ?? "",
        rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
      };
    });
    expect(target, "媒体库必须存在真实的末项卡片").not.toBeNull();
    expect(names, "末项必须是本轮上传的夹具附件（按上传返回的文件名核对）").toContain(target!.name);
    assertAboveNav(target!.rect, nav, "媒体库末项卡片");

    // 夹具存在性：本轮上传的附件必须都出现在列表中
    const renderedNames = await page
      .locator(".attachment-card img[alt]")
      .evaluateAll((els) => els.map((el) => el.getAttribute("alt") ?? ""));
    for (const name of names) expect(renderedNames, `夹具附件 ${name} 必须出现在列表中`).toContain(name);

    const targetCard = page.locator(".attachment-card").nth(target!.index);
    const targetCardBox = await targetCard.boundingBox();
    expect(targetCardBox, "媒体库末项卡片必须有完整矩形").not.toBeNull();
    // 卡片缩略图上的 hover 操作层带 @click.stop，点击卡片下方区域触发详情
    await targetCard.click({ position: { x: targetCardBox!.width / 2, y: targetCardBox!.height - 6 } });
    const dialog = page.locator('[role="dialog"]');
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(target!.name);
  });

  // 设置导航是一组原生链接（不是 tablist）：Tab 顺序即视觉顺序，Enter 激活，不使用方向键/Home/End 的漫游焦点。
  test("键盘可达性：真实 Tab 进入设置导航、可见焦点环、Tab 顺序与分组一致、Enter 联动页面区域", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=site");
    await expect(page.getByRole("heading", { name: "系统设置", exact: true })).toBeVisible();

    const focusedPage = () =>
      page.evaluate(() => (document.activeElement as HTMLElement | null)?.getAttribute("data-settings-page") ?? "");

    // 真实 Tab 遍历页面，必须能进入设置导航
    await page.locator("body").click({ position: { x: 5, y: 400 } });
    let first = "";
    for (let i = 0; i < 40 && !first; i++) {
      await page.keyboard.press("Tab");
      first = await focusedPage();
    }
    expect(first, "真实 Tab 遍历必须能进入设置导航").toBe("site");

    // 键盘聚焦必须有可见焦点环
    const ring = await page.evaluate(() => {
      const el = document.activeElement as HTMLElement;
      const cs = window.getComputedStyle(el);
      return { style: cs.outlineStyle, width: cs.outlineWidth, color: cs.outlineColor };
    });
    expect(ring.style, "键盘聚焦必须显示焦点环").not.toBe("none");
    expect(parseFloat(ring.width), "焦点环宽度必须清晰（≥2px）").toBeGreaterThanOrEqual(2);

    // Tab 顺序与分组导航的视觉顺序一致，之后才是“AI 与知识库”外链。
    const order = [first];
    for (let i = 0; i < 11; i++) {
      await page.keyboard.press("Tab");
      order.push(await focusedPage());
    }
    expect(order).toEqual([
      "site", "seo", "appearance", "navigation", "layout", "post",
      "listing", "community", "comments", "feedback", "extensions", "advanced",
    ]);
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement?.classList.contains("settings-nav-link"))).toBe(true);
    await page.keyboard.press("Shift+Tab");
    expect(await focusedPage()).toBe("advanced");

    // Enter 激活当前导航项并联动页面区域，焦点停留在被激活的导航项上。
    await page.keyboard.press("Enter");
    await expect(page.locator('.settings-nav:visible [data-settings-page="advanced"]')).toHaveAttribute("aria-current", "page");
    await expect(page.locator("#settings-page-advanced")).toBeVisible();
    await expect(page.locator("#settings-page-site")).toBeHidden();
    await expect(page.locator('.settings-nav:visible [data-settings-page="advanced"]')).toBeFocused();
  });

  test("主操作与危险操作三态对比度 ≥ 4.5:1", async ({ page }) => {
    await login(page);
    const fixture = await createPost(page, `t1-contrast-${Date.now()}`, 1);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理", exact: true })).toBeVisible();

    // 主操作：页头「撰写文章」
    await assertButtonContrast(page, 'header.page-header-stack-below-lg .btn-primary', "主操作按钮");

    // 危险操作：选中夹具后批量浮条中的「批量移入回收站」
    const card = page.locator(".post-record").filter({ hasText: fixture.title }).first();
    await expect(card).toBeVisible();
    await card.locator('label[aria-label^="选择文章"]').click();
    const dock = page.locator(".v-floating-dock");
    await expect(dock).toBeVisible();
    await assertButtonContrast(page, ".v-floating-dock .btn-danger", "危险操作按钮");
  });

  test("390px 输入、搜索、选择器与筛选按钮 ≥ 44px，独立图标目标 ≥ 44×44", async ({ page }) => {
    await login(page);
    const token = `t1-touch-${Date.now()}`;
    await createPost(page, token, 1);
    await createPage(page, token, 1);
    await createAttachments(page, 1);

    const cases = [
      {
        path: "/console/posts",
        heading: "文章管理",
        required: [
          { selector: ".apple-search", min: 1 },
          { selector: ".apple-select", min: 1 },
          { selector: ".apple-segmented button", min: 1 },
          { selector: ".post-record .action-icon-btn", min: 1, square: true },
        ],
      },
      {
        path: "/console/pages",
        heading: "独立页面",
        required: [
          { selector: ".apple-search", min: 1 },
          { selector: ".apple-select", min: 1 },
          { selector: ".apple-segmented button", min: 1 },
          { selector: ".action-icon-btn", min: 1, square: true },
        ],
      },
      {
        path: "/console/attachments",
        heading: "媒体库",
        required: [
          { selector: ".apple-search", min: 1 },
          { selector: ".apple-segmented button", min: 1 },
        ],
      },
      {
        path: "/console/comments",
        heading: "评论管理",
        required: [
          { selector: ".apple-search", min: 1 },
          { selector: ".apple-segmented button", min: 1 },
        ],
      },
      {
        path: "/console/friends",
        heading: "友链",
        required: [
          { selector: ".apple-search", min: 1 },
          { selector: ".apple-segmented button", min: 1 },
        ],
      },
      {
        path: "/console/settings?tab=basic",
        heading: "系统设置",
        required: [
          { selector: ".settings-basic-form .apple-input", min: 1 },
          { selector: ".settings-basic-form button[type=\"submit\"]", min: 1 },
        ],
      },
    ];

    await page.setViewportSize({ width: 390, height: 844 });
    for (const item of cases) {
      await page.goto(item.path);
      await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();

      for (const requirement of item.required) {
        const targets = page.locator(requirement.selector);
        const count = await targets.count();
        expect(
          count,
          `${item.path} 必须存在 ${requirement.selector}（期望至少 ${requirement.min} 个，实际 ${count} 个）`,
        ).toBeGreaterThanOrEqual(requirement.min);

        for (let i = 0; i < Math.min(count, 2); i++) {
          const box = await targets.nth(i).boundingBox();
          expect(box, `${item.path} ${requirement.selector}[${i}] 必须有完整矩形`).not.toBeNull();
          expect(box!.height, `${item.path} ${requirement.selector}[${i}] 高度必须 ≥ 44px`).toBeGreaterThanOrEqual(44);
          if (requirement.square) {
            expect(box!.width, `${item.path} ${requirement.selector}[${i}] 宽度必须 ≥ 44px`).toBeGreaterThanOrEqual(44);
          }
        }
      }
    }
  });

  test("紧凑摘要：五个统一页面在断点前后守住两列与四列（390/639/640/768/1023 两列，1024/1280 四列）", async ({ page }) => {
    await login(page);
    const token = `t1-stats-${Date.now()}`;
    await createPost(page, token, 1);
    await createPage(page, token, 1);
    await createAttachments(page, 1);

    const pages = [
      { path: "/console/posts", heading: "文章管理" },
      { path: "/console/pages", heading: "独立页面" },
      { path: "/console/attachments", heading: "媒体库" },
      { path: "/console/users", heading: "用户管理" },
      { path: "/console/backup", heading: "备份与导出" },
    ];

    const columnsOf = (page: Page) =>
      page.locator(".stats-card").first().evaluate((el) => {
        const grid = el.parentElement as HTMLElement;
        return window.getComputedStyle(grid).gridTemplateColumns.split(" ").filter(Boolean).length;
      });

    const twoColViewports = [390, 639, 640, 641, 767, 768, 1023];
    const fourColViewports = [1024, 1280];

    for (const item of pages) {
      // 验证断点前两列 (宽度 < 1024px)
      for (const w of twoColViewports) {
        await page.setViewportSize({ width: w, height: 800 });
        await page.goto(item.path);
        await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();
        expect(await columnsOf(page), `${item.path} 在 ${w}px 必须为两列`).toBe(2);
      }

      // 新摘要保留信息可读且不再使用 88px 高的独立卡片。
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(item.path);
      await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();
      await expect(page.locator(".stats-card").first()).toBeVisible();
      const mobileHeights = await page.locator(".stats-card").evaluateAll((els) =>
        els.map((el) => el.getBoundingClientRect().height),
      );
      expect(mobileHeights.length, `${item.path} 必须有概览卡片`).toBeGreaterThan(0);
      for (const height of mobileHeights) {
        expect(height).toBeGreaterThanOrEqual(28);
        expect(height, `${item.path} 手机摘要应保持紧凑`).toBeLessThanOrEqual(88);
      }

      // 验证断点后四列 (宽度 >= 1024px)
      for (const w of fourColViewports) {
        await page.setViewportSize({ width: w, height: 800 });
        await page.goto(item.path);
        await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();
        expect(await columnsOf(page), `${item.path} 在 ${w}px 必须为四列`).toBe(4);
      }

      // 桌面摘要同样不保留大卡片的最小高度。
      const desktopHeights = await page.locator(".stats-card").evaluateAll((els) =>
        els.map((el) => el.getBoundingClientRect().height),
      );
      for (const height of desktopHeights) {
        expect(height).toBeGreaterThanOrEqual(28);
        expect(height, `${item.path} 桌面摘要应保持紧凑`).toBeLessThanOrEqual(88);
      }
    }
  });

  const checkViewports = [
    { width: 390, height: 844 },
    { width: 640, height: 900 },
    { width: 641, height: 900 },
    { width: 767, height: 900 },
    { width: 768, height: 1024 },
    { width: 1280, height: 800 },
  ];

  // 系统设置的 12 个页面（旧链接 basic/search 等经重定向落到这些页面，由 console-settings-ia 专项覆盖）。
  const settingPages = [
    { key: "site", form: ".settings-basic-form" },
    { key: "seo", form: ".canonical-settings-form" },
    { key: "appearance", form: ".canonical-settings-form" },
    { key: "navigation", form: ".canonical-settings-form" },
    { key: "layout", form: ".canonical-settings-form" },
    { key: "post", form: ".canonical-settings-form" },
    { key: "listing", form: ".canonical-settings-form" },
    { key: "community", form: ".canonical-settings-form" },
    { key: "comments", form: ".canonical-settings-form" },
    { key: "feedback", form: ".settings-feedback-form" },
    { key: "extensions", form: ".canonical-settings-form" },
    { key: "advanced", form: ".canonical-settings-form" },
  ];
  const settingTabs = settingPages.map(({ key, form }) => ({
    url: `/console/settings?tab=${key}`,
    key,
    form,
    title: "系统设置",
  }));

  // 每个设置页保留完整视口矩阵，避免所有页面共用一次测试的 30 秒预算。
  // “单卡片层级”随新布局调整为“页面本身不再套卡片”：导航 + 内容直接铺在页面上，分区只靠留白与分隔线，
  // 因此原先对外层 .settings-basic-panel 卡片和 .settings-section-body 的断言改为：外层卡片不存在，分区/行不带框、阴影与圆角。
  for (const tab of settingTabs) {
    test(`设置页无嵌套卡片与边距：${tab.url} 在 390/640/641/767/768/1280px 下保留全部布局断言`, async ({ page }) => {
      await login(page);
      for (const vp of checkViewports) {
        await page.setViewportSize(vp);
        await page.goto(tab.url);
        await expect(page.getByRole("heading", { name: tab.title, exact: true })).toBeVisible();

        // 必须确认请求的页面真的被激活，否则 query 失效退回默认页会让后续断言空覆盖
        await expect(
          page.locator(`#settings-page-${tab.key}`),
          `${tab.url} 在 ${vp.width}px 必须激活 #settings-page-${tab.key}`,
        ).toBeVisible();
        await expect(
          page.locator(`#settings-page-${tab.key} ${tab.form}`),
          `${tab.url} 在 ${vp.width}px 必须加载活动表单`,
        ).toBeVisible();

        // 页面不再包一层主卡片面板：导航与内容直接铺在页面容器里。
        await expect(page.locator(".settings-basic-panel"), `${tab.url} ${vp.width}px 不应再有外层主卡片`).toHaveCount(0);
        const shell = await page.locator(".settings-content").evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return {
            bg: cs.backgroundColor,
            borderWidth: cs.borderTopWidth,
            boxShadow: cs.boxShadow,
            borderRadius: cs.borderTopLeftRadius,
          };
        });
        expect(shell.bg === "transparent" || shell.bg === "rgba(0, 0, 0, 0)", `${tab.url} ${vp.width}px 内容区背景必须透明`).toBe(true);
        expect(parseFloat(shell.borderWidth), `${tab.url} ${vp.width}px 内容区不得有边框`).toBe(0);
        expect(shell.boxShadow, `${tab.url} ${vp.width}px 内容区不得有阴影`).toBe("none");
        expect(parseFloat(shell.borderRadius), `${tab.url} ${vp.width}px 内容区不得有圆角`).toBe(0);

        // 验证表单分区容器存在，且必须遍历所有匹配项，不可为 0
        const sections = page.locator(".settings-sec");
        // 批量读取全部分区（含保留草稿的隐藏页面），避免逐节点跨浏览器往返耗尽测试预算。
        const sectionStylesList = await sections.evaluateAll((elements) => elements.map((el) => {
          const cs = window.getComputedStyle(el);
          return {
            paddingLeft: cs.paddingLeft,
            paddingRight: cs.paddingRight,
            bg: cs.backgroundColor,
            borderLeft: cs.borderLeftWidth,
            borderRight: cs.borderRightWidth,
            borderBottom: cs.borderBottomWidth,
            boxShadow: cs.boxShadow,
            borderRadius: cs.borderTopLeftRadius,
          };
        }));
        const sectionCount = sectionStylesList.length;
        expect(sectionCount, `${tab.url} 在 ${vp.width}px 必须存在表单分区容器`).toBeGreaterThan(0);

        // 方向 A：页面左右边距上移到 .settings-container（手机 16px / 平板桌面 32px），
        // 分区只负责纵向留白与分隔线，不再重复承担横向内边距。
        const containerPadding = await page.locator(".settings-container").evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { left: parseFloat(cs.paddingLeft), right: parseFloat(cs.paddingRight) };
        });
        const expectedContainerPadding = vp.width < 768 ? 16 : 32;
        expect(
          containerPadding.left,
          `${tab.url} ${vp.width}px 设置容器左边距必须为${expectedContainerPadding}px（实测 ${containerPadding.left}px）`,
        ).toBe(expectedContainerPadding);
        expect(
          containerPadding.right,
          `${tab.url} ${vp.width}px 设置容器右边距必须为${expectedContainerPadding}px（实测 ${containerPadding.right}px）`,
        ).toBe(expectedContainerPadding);

        for (let s = 0; s < sectionCount; s++) {
          const sectionStyles = sectionStylesList[s];
          expect(
            parseFloat(sectionStyles.paddingLeft),
            `${tab.url} ${vp.width}px 表单分区[${s}] 不再承担横向边距，左边距必须为0（实测 ${sectionStyles.paddingLeft}）`,
          ).toBe(0);
          expect(
            parseFloat(sectionStyles.paddingRight),
            `${tab.url} ${vp.width}px 表单分区[${s}] 不再承担横向边距，右边距必须为0（实测 ${sectionStyles.paddingRight}）`,
          ).toBe(0);
          // 分区不是卡片：透明背景、无左右/下边框（上边框是分区之间的分隔线）、无阴影、无圆角。
          expect(
            sectionStyles.bg === "transparent" || sectionStyles.bg === "rgba(0, 0, 0, 0)",
            `${tab.url} ${vp.width}px 表单分区[${s}] 背景必须透明（实际为 ${sectionStyles.bg}）`,
          ).toBe(true);
          expect(parseFloat(sectionStyles.borderLeft), `${tab.url} ${vp.width}px 表单分区[${s}] 不得有左边框`).toBe(0);
          expect(parseFloat(sectionStyles.borderRight), `${tab.url} ${vp.width}px 表单分区[${s}] 不得有右边框`).toBe(0);
          expect(parseFloat(sectionStyles.borderBottom), `${tab.url} ${vp.width}px 表单分区[${s}] 不得有下边框`).toBe(0);
          expect(sectionStyles.boxShadow, `${tab.url} ${vp.width}px 表单分区[${s}] 不得有阴影`).toBe("none");
          expect(parseFloat(sectionStyles.borderRadius), `${tab.url} ${vp.width}px 表单分区[${s}] 不得有圆角`).toBe(0);
        }

        // 分区里的“行”同样不带卡片外观：透明背景、无阴影、无圆角、左右内边距为 0。
        const rowStylesList = await page.locator(".settings-row").evaluateAll((elements) => elements.map((el) => {
          const cs = window.getComputedStyle(el);
          return {
            bg: cs.backgroundColor,
            boxShadow: cs.boxShadow,
            borderRadius: cs.borderTopLeftRadius,
            paddingLeft: cs.paddingLeft,
            paddingRight: cs.paddingRight,
          };
        }));
        for (let i = 0; i < rowStylesList.length; i++) {
          const rowStyles = rowStylesList[i];
          expect(
            rowStyles.bg === "transparent" || rowStyles.bg === "rgba(0, 0, 0, 0)",
            `${tab.url} ${vp.width}px .settings-row[${i}] 背景必须透明（实际为 ${rowStyles.bg}）`,
          ).toBe(true);
          expect(rowStyles.boxShadow, `${tab.url} ${vp.width}px .settings-row[${i}] 不得有阴影`).toBe("none");
          expect(parseFloat(rowStyles.borderRadius), `${tab.url} ${vp.width}px .settings-row[${i}] 不得有圆角`).toBe(0);
          expect(parseFloat(rowStyles.paddingLeft), `${tab.url} ${vp.width}px .settings-row[${i}] 左内边距必须为0`).toBe(0);
          expect(parseFloat(rowStyles.paddingRight), `${tab.url} ${vp.width}px .settings-row[${i}] 右内边距必须为0`).toBe(0);
        }

        // 不出现无意横向溢出
        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        );
        expect(overflow, `${tab.url} 在 ${vp.width}px 不得有水平溢出`).toBeLessThanOrEqual(1);
      }
    });
  }

  test("设置页无嵌套卡片与边距：控件边界与验收截图", async ({ page }, testInfo) => {
    await login(page);

    // 重点验证控件保留必要边界：输入框（原高级页警告框随 AdvancedSettings 退役，不再断言）
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=advanced");
    // 输入框保留必要边界
    const input = page.locator(".canonical-settings-form:visible").getByRole("textbox").first();
    await expect(input).toBeVisible();
    const inputStyles = await input.evaluate((el) => {
      const cs = window.getComputedStyle(el);
      return {
        borderWidth: cs.borderWidth,
        borderStyle: cs.borderStyle,
      };
    });
    expect(parseFloat(inputStyles.borderWidth), "输入框必须保留边框边界").toBeGreaterThan(0);
    expect(inputStyles.borderStyle, "输入框边框样式不可为 none").not.toBe("none");

    await page.screenshot({ path: testInfo.outputPath("t1-settings-advanced-390.png") });

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=site");
    await expect(page.locator(".settings-nav:visible")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("t3-settings-nav-desktop-1280.png") });

    // 768px 概览截图（文章管理）
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理", exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("t1-posts-overview-768.png") });
  });

  test("User Center：隔离非管理员账号的资料保存、通知页与共享弹层", async ({ page }) => {
    await loginUserCenter(page);

    // 资料页：实际输入并保存，刷新后回读
    await page.goto("/uc/profile");
    await expect(page.getByRole("heading", { name: "个人资料", exact: true })).toBeVisible();
    const displayName = `E2E Author ${Date.now()}`;
    const nameInput = page.locator('input[placeholder="输入显示名称"]');
    await expect(nameInput).toBeVisible();
    await nameInput.fill(displayName);
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await expect(page.locator(".apple-toast-item").first()).toBeVisible();
    await page.reload();
    await expect(page.locator('input[placeholder="输入显示名称"]')).toHaveValue(displayName);

    // 通知页：页面显示、分类 tablist 与现有操作
    await page.goto("/uc/notifications");
    await expect(page.getByRole("heading", { name: "通知中心", exact: true })).toBeVisible();
    const categories = page.getByRole("tablist", { name: "通知分类" });
    await expect(categories).toBeVisible();
    await expect(categories.getByRole("tab").first()).toBeVisible();
    const refresh = page.locator('button[title="刷新"]');
    await expect(refresh).toBeVisible();
    await refresh.click();
    await expect(page.getByRole("heading", { name: "通知中心", exact: true })).toBeVisible();

    // 共享按钮样式在 UC 生效（工具型圆角来自 VButton）
    const readAll = page.getByRole("button", { name: /全部已读/ });
    await expect(readAll).toBeVisible();
    const radius = await readAll.evaluate((el) => parseFloat(window.getComputedStyle(el).borderRadius));
    // 方向 A 将共享 VButton 圆角从 8px 收紧到 6px，UC 与 Console 同步生效。
    expect(radius, "UC 共享按钮应使用同一工具型圆角").toBe(6);

    // 共享弹层：附件选择弹层具备 dialog 语义并可 Esc 关闭
    await page.goto("/uc/profile");
    const openPicker = page.getByRole("button", { name: /从附件库选择/ }).first();
    await expect(openPicker).toBeVisible();
    await openPicker.click();
    const dialog = page.locator('[role="dialog"][aria-modal="true"]');
    await expect(dialog).toBeVisible();

    // 打开后焦点必须进入弹层
    const focusInside = await page.evaluate(() => {
      const dialogEl = document.querySelector('[role="dialog"][aria-modal="true"]');
      return !!dialogEl && !!document.activeElement && dialogEl.contains(document.activeElement);
    });
    expect(focusInside, "打开共享弹层后焦点必须进入弹层").toBe(true);

    // Esc 关闭并把焦点还给触发元素
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(openPicker).toBeFocused();
  });

  test("390 评论行信息可读、操作换行，快捷链接与评论勾选命中区 ≥44px", async ({ page }, testInfo) => {
    await login(page);
    const token = Date.now();
    const content = `E2E 评论行可读性夹具 ${token}`;
    for (let i = 0; i < 3; i++) {
      const created = await page.request.post("/api/comments", {
        data: {
          page_key: `/post/e2e-comments-${token}`,
          nick: `E2E Reader ${i}`,
          // 与验收站点 native_comments.admin_email 一致，豁免同 IP 60 秒 1 条的公开限流。
          email: "admin@e2e.test",
          content: `${content} #${i}`,
        },
      });
      expect(created.ok(), await created.text()).toBeTruthy();
    }

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/comments");
    const row = page.locator(".comment-row").filter({ hasText: content }).first();
    await expect(row).toBeVisible({ timeout: 20_000 });
    const info = row.locator(".comment-row-main > div").first();
    const actions = row.locator(".comment-actions");
    const status = row.locator(".comment-status").first();
    const email = row.locator("span.font-mono").filter({ hasText: "@" }).first();
    const infoBox = (await info.boundingBox())!;
    const actionsBox = (await actions.boundingBox())!;
    const statusBox = (await status.boundingBox())!;
    const emailBox = (await email.boundingBox())!;
    expect(actionsBox.y, "操作按钮必须换到信息行下方").toBeGreaterThanOrEqual(infoBox.y + infoBox.height - 1);
    expect(statusBox.height, "状态必须单行，不得竖排折行").toBeLessThan(26);
    expect(await status.evaluate((el) => getComputedStyle(el).whiteSpace)).toBe("nowrap");
    expect(emailBox.width, "邮箱必须有可辨识宽度").toBeGreaterThanOrEqual(80);
    const actionButtons = await actions.locator("button").evaluateAll((nodes) =>
      nodes.map((node) => {
        const rect = node.getBoundingClientRect();
        return { width: rect.width, height: rect.height };
      }),
    );
    expect(actionButtons.length).toBeGreaterThan(0);
    for (const button of actionButtons) {
      expect(button.width).toBeGreaterThanOrEqual(44);
      expect(button.height).toBeGreaterThanOrEqual(44);
    }
    const selectBox = (await row.locator(".comment-select").boundingBox())!;
    expect(selectBox.width, "评论勾选命中区 ≥44px").toBeGreaterThanOrEqual(44);
    expect(selectBox.height, "评论勾选命中区 ≥44px").toBeGreaterThanOrEqual(44);
    await page.screenshot({ path: testInfo.outputPath("comments-readable-390.png") });

    await page.goto("/console/dashboard");
    const links = await page.locator(".dashboard-quick-links a").evaluateAll((nodes) =>
      nodes.map((node) => {
        const rect = node.getBoundingClientRect();
        return { width: rect.width, height: rect.height, label: node.textContent?.trim() };
      }),
    );
    expect(links.length).toBeGreaterThan(0);
    for (const link of links) {
      expect(link.width, `快捷入口「${link.label}」命中区宽度 ≥44px`).toBeGreaterThanOrEqual(44);
      expect(link.height, `快捷入口「${link.label}」命中区高度 ≥44px`).toBeGreaterThanOrEqual(44);
    }
  });
});
