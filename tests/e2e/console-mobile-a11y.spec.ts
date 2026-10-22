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
    data: { slug, title, content_md: `T1 夹具正文 ${index}`, status: "published" },
  });
  expect(response.ok(), `夹具文章 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { slug, title };
}

async function createPage(page: Page, token: string, index: number) {
  const slug = `${token}-${index}`;
  const title = `T1 夹具页面 ${token}-${index}`;
  const response = await page.request.post("/api/v1/pages", {
    data: { slug, title, content_md: `T1 夹具页面正文 ${index}`, status: "published" },
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
    return { color: cs.color, background: cs.backgroundColor };
  });
}

/** 依次测量默认、hover、active 三态的实际计算颜色并断言普通文字对比度 ≥ 4.5:1。 */
async function assertButtonContrast(page: Page, selector: string, label: string) {
  const button = page.locator(selector);
  await expect(button).toBeVisible();

  const states: Array<{ name: string; colors: { color: string; background: string } }> = [];
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
    const cards = page.locator("main div.group").filter({ has: page.locator('button[title="编辑页面"]') });
    await expect(cards).toHaveCount(fixtures.length);

    const nav = await bottomNavBox(page);
    await scrollMainToBottom(page);

    const lastCard = cards.last();
    await expect(lastCard).toBeVisible();
    const lastTitle = ((await lastCard.locator("h3").first().textContent()) ?? "").trim();
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

  test("键盘可达性：真实 Tab 进入 Tabs、可见焦点环、方向键/Home/End 联动面板", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置", exact: true })).toBeVisible();

    // 真实 Tab 遍历页面，必须能进入 Tabs
    await page.locator("body").click({ position: { x: 5, y: 400 } });
    let reachedTab = false;
    for (let i = 0; i < 40; i++) {
      await page.keyboard.press("Tab");
      const active = await page.evaluate(() => {
        const el = document.activeElement as HTMLElement | null;
        return el ? { role: el.getAttribute("role") ?? "", id: el.id } : { role: "", id: "" };
      });
      if (active.role === "tab") {
        reachedTab = true;
        break;
      }
    }
    expect(reachedTab, "真实 Tab 遍历必须能进入设置 Tabs").toBe(true);

    // 键盘聚焦必须有可见焦点环
    const ring = await page.evaluate(() => {
      const el = document.activeElement as HTMLElement;
      const cs = window.getComputedStyle(el);
      return { style: cs.outlineStyle, width: cs.outlineWidth, color: cs.outlineColor };
    });
    expect(ring.style, "键盘聚焦必须显示焦点环").not.toBe("none");
    expect(parseFloat(ring.width), "焦点环宽度必须清晰（≥2px）").toBeGreaterThanOrEqual(2);

    // 方向键切换选中项并联动 tabpanel
    await page.keyboard.press("ArrowRight");
    await expect(page.locator("#tab-post")).toHaveAttribute("aria-selected", "true");
    await expect(page.locator("#tabpanel-post")).toBeVisible();
    await expect(page.locator("#tabpanel-basic")).toBeHidden();
    await expect(page.locator("#tab-post")).toBeFocused();

    await page.keyboard.press("End");
    await expect(page.locator("#tab-advanced")).toHaveAttribute("aria-selected", "true");
    await expect(page.locator("#tabpanel-advanced")).toBeVisible();
    await expect(page.locator("#tab-advanced")).toBeFocused();

    await page.keyboard.press("Home");
    await expect(page.locator("#tab-basic")).toHaveAttribute("aria-selected", "true");
    await expect(page.locator("#tabpanel-basic")).toBeVisible();
    await expect(page.locator("#tab-basic")).toBeFocused();
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
    const card = page.locator(".post-card").filter({ hasText: fixture.title }).first();
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
          { selector: ".post-card-action", min: 1, square: true },
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

  test("概览卡片：五个统一页面在断点前后守住两列与四列（390/639/640/768/1023 两列，1024/1280 四列）", async ({ page }) => {
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

      // 验证 390px 最小高度 88px
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(item.path);
      await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();
      await expect(page.locator(".stats-card").first()).toBeVisible();
      const mobileHeights = await page.locator(".stats-card").evaluateAll((els) =>
        els.map((el) => el.getBoundingClientRect().height),
      );
      expect(mobileHeights.length, `${item.path} 必须有概览卡片`).toBeGreaterThan(0);
      for (const height of mobileHeights) {
        expect(height, `${item.path} 手机概览卡片最小高度 88px`).toBeGreaterThanOrEqual(88);
      }

      // 验证断点后四列 (宽度 >= 1024px)
      for (const w of fourColViewports) {
        await page.setViewportSize({ width: w, height: 800 });
        await page.goto(item.path);
        await expect(page.getByRole("heading", { name: item.heading, exact: true })).toBeVisible();
        expect(await columnsOf(page), `${item.path} 在 ${w}px 必须为四列`).toBe(4);
      }

      // 验证 1280px 最小高度 80px
      const desktopHeights = await page.locator(".stats-card").evaluateAll((els) =>
        els.map((el) => el.getBoundingClientRect().height),
      );
      for (const height of desktopHeights) {
        expect(height, `${item.path} 桌面概览卡片最小高度 80px`).toBeGreaterThanOrEqual(80);
      }
    }
  });

  test("设置页单卡片层级与边距：390/640/641/767/768/1280px 下无多余嵌套卡片，内层容器透明且无水平溢出，全量断言表单容器与边界保留", async ({ page }, testInfo) => {
    await login(page);

    const checkViewports = [
      { width: 390, height: 844 },
      { width: 640, height: 900 },
      { width: 641, height: 900 },
      { width: 767, height: 900 },
      { width: 768, height: 1024 },
      { width: 1280, height: 800 },
    ];

    const settingTabs = [
      { url: "/console/settings?tab=basic", title: "系统设置", hasBodies: true },
      { url: "/console/settings?tab=post", title: "系统设置", hasBodies: true },
      { url: "/console/settings?tab=comments", title: "系统设置", hasBodies: true },
      { url: "/console/settings?tab=seo", title: "系统设置", hasBodies: true },
      { url: "/console/settings?tab=search", title: "系统设置", hasBodies: true },
      { url: "/console/settings?tab=advanced", title: "系统设置", hasBodies: true },
      { url: "/console/appearance?tab=appearance", title: "主题设置", hasBodies: false },
      { url: "/console/appearance?tab=layout", title: "主题设置", hasBodies: false },
      { url: "/console/appearance?tab=advanced", title: "主题设置", hasBodies: false },
    ];

    for (const tab of settingTabs) {
      for (const vp of checkViewports) {
        await page.setViewportSize(vp);
        await page.goto(tab.url);
        await expect(page.getByRole("heading", { name: tab.title, exact: true })).toBeVisible();

        // 系统设置：必须确认请求的 tab 真的被激活，否则 query 失效退回 basic 会让后续断言空覆盖
        const settingsTab = new URLSearchParams(tab.url.split("?")[1] ?? "").get("tab");
        if (tab.url.startsWith("/console/settings") && settingsTab) {
          await expect(
            page.locator(`#tabpanel-${settingsTab}`),
            `${tab.url} 在 ${vp.width}px 必须激活 #tabpanel-${settingsTab}`,
          ).toBeVisible();
        }

        // 外层只有一个主卡片面板 .settings-basic-panel
        const outerCard = page.locator(".settings-basic-panel");
        await expect(outerCard).toBeVisible();

        // 验证表单分区容器存在，且必须遍历所有匹配项，不可为 0
        const sections = page.locator(".settings-basic-section, .settings-section, .settings-post-section");
        const sectionCount = await sections.count();
        expect(sectionCount, `${tab.url} 在 ${vp.width}px 必须存在表单分区容器`).toBeGreaterThan(0);

        for (let s = 0; s < sectionCount; s++) {
          const sectionStyles = await sections.nth(s).evaluate((el) => {
            const cs = window.getComputedStyle(el);
            return {
              paddingLeft: cs.paddingLeft,
              paddingRight: cs.paddingRight,
            };
          });
          if (vp.width < 768) {
            expect(
              parseFloat(sectionStyles.paddingLeft),
              `${tab.url} ${vp.width}px 表单分区[${s}] 左边距在手机端必须为16px（实测 ${sectionStyles.paddingLeft}）`,
            ).toBe(16);
            expect(
              parseFloat(sectionStyles.paddingRight),
              `${tab.url} ${vp.width}px 表单分区[${s}] 右边距在手机端必须为16px（实测 ${sectionStyles.paddingRight}）`,
            ).toBe(16);
          } else {
            expect(
              parseFloat(sectionStyles.paddingLeft),
              `${tab.url} ${vp.width}px 表单分区[${s}] 左边距在平板/桌面必须为28px（实测 ${sectionStyles.paddingLeft}）`,
            ).toBe(28);
            expect(
              parseFloat(sectionStyles.paddingRight),
              `${tab.url} ${vp.width}px 表单分区[${s}] 右边距在平板/桌面必须为28px（实测 ${sectionStyles.paddingRight}）`,
            ).toBe(28);
          }
        }

        // 针对包含 .settings-section-body 的表单页，明确遍历断言背景透明、无重复边框、内边距为0
        const bodies = page.locator(".settings-section-body");
        const bodyCount = await bodies.count();
        if (tab.hasBodies) {
          expect(bodyCount, `${tab.url} 在 ${vp.width}px 预期存在 .settings-section-body 容器，缺失必须失败`).toBeGreaterThan(0);
          for (let i = 0; i < bodyCount; i++) {
            const bodyStyles = await bodies.nth(i).evaluate((el) => {
              const cs = window.getComputedStyle(el);
              return {
                bg: cs.backgroundColor,
                borderWidth: cs.borderWidth,
                borderStyle: cs.borderStyle,
                borderRadius: cs.borderRadius,
                boxShadow: cs.boxShadow,
                paddingLeft: cs.paddingLeft,
                paddingRight: cs.paddingRight,
                paddingTop: cs.paddingTop,
                paddingBottom: cs.paddingBottom,
              };
            });
            expect(
              bodyStyles.bg === "transparent" || bodyStyles.bg === "rgba(0, 0, 0, 0)",
              `${tab.url} ${vp.width}px .settings-section-body[${i}] 背景必须透明（实际为 ${bodyStyles.bg}）`,
            ).toBe(true);
            expect(
              bodyStyles.borderStyle === "none" || bodyStyles.borderWidth === "0px",
              `${tab.url} ${vp.width}px .settings-section-body[${i}] 不得有可见边框`,
            ).toBe(true);
            expect(bodyStyles.boxShadow, `${tab.url} ${vp.width}px .settings-section-body[${i}] 不得有阴影`).toBe("none");
            expect(parseFloat(bodyStyles.borderRadius), `${tab.url} ${vp.width}px .settings-section-body[${i}] 不得有额外圆角`).toBe(0);
            expect(parseFloat(bodyStyles.paddingLeft), `${tab.url} ${vp.width}px .settings-section-body[${i}] 左内边距必须为0`).toBe(0);
            expect(parseFloat(bodyStyles.paddingRight), `${tab.url} ${vp.width}px .settings-section-body[${i}] 右内边距必须为0`).toBe(0);
          }
        }

        // 不出现无意横向溢出
        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        );
        expect(overflow, `${tab.url} 在 ${vp.width}px 不得有水平溢出`).toBeLessThanOrEqual(1);
      }
    }

    // 重点验证控件保留必要边界：警告框、输入框
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=advanced");
    const warning = page.locator(".settings-warning");
    await expect(warning).toBeVisible();
    const warningStyles = await warning.evaluate((el) => {
      const cs = window.getComputedStyle(el);
      return {
        borderWidth: cs.borderWidth,
        borderStyle: cs.borderStyle,
        borderRadius: cs.borderRadius,
        paddingLeft: cs.paddingLeft,
      };
    });
    expect(parseFloat(warningStyles.borderWidth), "警告框必须保留边框边界").toBeGreaterThan(0);
    expect(warningStyles.borderStyle, "警告框边框样式不可为 none").not.toBe("none");
    expect(parseFloat(warningStyles.paddingLeft), "警告框必须保留内部边距").toBeGreaterThan(0);
    expect(parseFloat(warningStyles.borderRadius), "警告框必须保留圆角").toBeGreaterThan(0);

    // 输入框保留必要边界
    const input = page.locator(".apple-input, .code-editor-wrapper").first();
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
    await page.goto("/console/appearance?tab=appearance");
    await expect(page.locator(".appearance-tabs")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("t3-appearance-desktop-1280.png") });

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

    // 共享按钮样式在 UC 生效（胶囊圆角来自 VButton）
    const readAll = page.getByRole("button", { name: /全部已读/ });
    await expect(readAll).toBeVisible();
    const radius = await readAll.evaluate((el) => parseFloat(window.getComputedStyle(el).borderRadius));
    expect(radius, "UC 共享按钮必须保持胶囊圆角").toBeGreaterThanOrEqual(18);

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
});
