import { expect, test, type Frame, type Locator, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";

// BLG-20261031-03：编辑器标签插件实时预览的独立验收。
// 只断言可观察行为：请求合并计数、iframe 元素身份、高度不回退、动态 Fixture 内容、
// 390px 横向溢出、Markdown 分屏失败保留旧文档，以及预览不写真实评分/投票。
// 所有数据都来自 scripts/e2e/run-server.sh 的隔离根目录。

const metadata = { stellar_version: 2, stellar_config: "{}", blog_extensions: "{}" };
const PREVIEW_PATH = "/api/v1/preview/markdown";
const BASELINE_DIR = "docs/baselines/tag-live-preview";

type Diagnostics = {
  previewRequests: { url: string; body: string }[];
  realWrites: { method: string; url: string }[];
  apiResponses: { url: string; status: number; cors: string | null }[];
  failed: string[];
  consoleErrors: string[];
  pageErrors: string[];
};

function watch(page: Page): Diagnostics {
  const diag: Diagnostics = {
    previewRequests: [],
    realWrites: [],
    apiResponses: [],
    failed: [],
    consoleErrors: [],
    pageErrors: [],
  };
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (request.method() === "POST" && url.pathname === PREVIEW_PATH) {
      diag.previewRequests.push({ url: request.url(), body: request.postData() ?? "" });
    }
    // 真实写端点（/api/rating、/api/vote…）与预览 Fixture（/api/v1/preview/*）分开统计。
    if (request.method() === "POST" && /^\/api\/(rating|vote)(\/|$)/.test(url.pathname)) {
      diag.realWrites.push({ method: request.method(), url: request.url() });
    }
  });
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (url.pathname.startsWith("/api/v1/preview/")) {
      diag.apiResponses.push({
        url: response.url(),
        status: response.status(),
        cors: response.headers()["access-control-allow-origin"] ?? null,
      });
    }
  });
  page.on("requestfailed", (request) => diag.failed.push(`${request.method()} ${request.url()}`));
  page.on("console", (message) => {
    if (message.type() === "error") diag.consoleErrors.push(message.text());
  });
  page.on("pageerror", (error) => diag.pageErrors.push(String(error)));
  return diag;
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function createRecord(page: Page, kind: "posts" | "pages", content: string) {
  const slug = `tag-live-${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  const response = await page.request.post(`/api/v1/${kind}`, {
    data: { ...metadata, slug, title: "标签实时预览验收", content_md: content, status: "draft" },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  return slug;
}

async function openPostEditor(page: Page, slug: string) {
  await page.goto(`/console/posts/edit/${slug}`);
  await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
}

const editorMarkdown = (page: Page) =>
  page.evaluate(() => {
    const editor = (window as Window & { __consoleTiptapEditor?: any }).__consoleTiptapEditor;
    if (!editor) throw new Error("development TipTap editor hook is unavailable");
    return editor.storage.markdown.getMarkdown() as string;
  });

async function selectTagNode(page: Page) {
  await page.locator(".stellar-tag-node").first().click({ force: true });
  if (await page.locator(".stellar-bubble").isVisible().catch(() => false)) return;
  await page.evaluate(() => {
    const editor = (window as Window & { __consoleTiptapEditor?: any }).__consoleTiptapEditor;
    if (!editor) throw new Error("development TipTap editor hook is unavailable");
    let found = -1;
    editor.state.doc.descendants((node: any, pos: number) => {
      if (found < 0 && node.type.name === "stellarTag") found = pos;
    });
    if (found < 0) throw new Error("stellarTag node not found");
    editor.commands.setNodeSelection(found);
  });
}

async function openPropertyPanel(page: Page) {
  await selectTagNode(page);
  const bubble = page.locator(".stellar-bubble");
  await expect(bubble).toBeVisible({ timeout: 10_000 });
  await bubble.getByRole("button", { name: "编辑属性" }).click();
  const dialog = page.getByRole("dialog", { name: "编辑组件属性" });
  await expect(dialog).toBeVisible({ timeout: 10_000 });
  await expect(dialog.locator("text=正在读取属性")).toHaveCount(0, { timeout: 15_000 });
  return dialog;
}

const folding = (body: string) =>
  `{% folding color:cyan open:true 折叠标题 %}\n${body}\n第二行正文用于撑开预览高度\n{% endfolding %}`;

const srcdocOf = (frame: ReturnType<Page["locator"]>) =>
  frame.getAttribute("srcdoc").then((html) => html ?? "");

async function heightOf(frame: ReturnType<Page["locator"]>) {
  return frame.evaluate((el) => (el as HTMLElement).getBoundingClientRect().height);
}

/** 在元素上打标记；若 Vue 重建了元素，标记会消失。 */
async function markIdentity(frame: ReturnType<Page["locator"]>) {
  await frame.evaluate((el) => el.setAttribute("data-e2e-identity", "initial"));
}

const hasIdentity = (frame: ReturnType<Page["locator"]>) =>
  frame.getAttribute("data-e2e-identity").then((value) => value === "initial");

async function contentFrame(locator: Locator): Promise<Frame> {
  const handle = await locator.elementHandle();
  if (!handle) throw new Error("iframe element handle is unavailable");
  const frame = await handle.contentFrame();
  if (!frame) throw new Error("iframe content frame is unavailable");
  return frame;
}

/** 只统计顶栏/面板的 component-library 预览请求（不混入正文 NodeView 的真实上下文请求）。 */
function componentPreviewRequests(diag: Diagnostics) {
  return diag.previewRequests.filter((item) => {
    try {
      return JSON.parse(item.body).preview_profile === "component-library";
    } catch {
      return false;
    }
  });
}

function previewResponse(diag: Diagnostics, pathname: string) {
  return diag.apiResponses.filter((item) => new URL(item.url).pathname === pathname);
}

const corsConsoleErrors = (diag: Diagnostics) =>
  diag.consoleErrors.filter((text) => /cors|access-control/i.test(text));

type RGBA = [number, number, number, number];

function parseColor(value: string): RGBA {
  const match = value.match(/rgba?\(([^)]+)\)/);
  if (!match) return [0, 0, 0, 0];
  const parts = match[1].split(",").map((item) => Number(item.trim()));
  return [parts[0] ?? 0, parts[1] ?? 0, parts[2] ?? 0, parts[3] ?? 1];
}

function blend(fg: RGBA, bg: RGBA): RGBA {
  const alpha = fg[3] + bg[3] * (1 - fg[3]);
  if (alpha === 0) return [0, 0, 0, 0];
  const channel = (index: number) => (fg[index] * fg[3] + bg[index] * bg[3] * (1 - fg[3])) / alpha;
  return [channel(0), channel(1), channel(2), alpha];
}

function luminance(color: RGBA): number {
  const channel = (value: number) => {
    const v = value / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(color[0]) + 0.7152 * channel(color[1]) + 0.0722 * channel(color[2]);
}

function contrastRatio(text: string, background: string): number {
  const bg = parseColor(background);
  const fg = blend(parseColor(text), bg);
  const l1 = luminance(fg);
  const l2 = luminance(bg);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

/**
 * 读取 iframe 内 tabs 文本色与预览表面（V1）：html/body 背景必须不透明，
 * 返回真实浏览器计算出的对比度。Opaque 表面缺失时 contrastRatio 记为 0（必然失败）。
 *
 * 先等真实首绘：站点文档含外部高亮样式表（gcore.jsdelivr.net，render-blocking），
 * 未就绪前 iframe 一像素都不画（透出 Console 深色）。计算样式在解析后即可读，
 * 若不等绘制，截图会把「尚未绘制」误当成「不可见」。
 */
async function measurePreviewTabs(iframe: Locator) {
  const frame = await contentFrame(iframe);
  await expect(frame.locator(".nav-tabs").first()).toBeVisible({ timeout: 25_000 });
  await frame
    .waitForFunction(
      () =>
        document.readyState === "complete" &&
        [...document.querySelectorAll('link[rel="stylesheet"]')].every((link) => Boolean((link as HTMLLinkElement).sheet)),
      null,
      { timeout: 20_000, polling: 100 },
    )
    .catch(() => {
      // CDN 不可达时仍继续：对比度断言不受影响，但截图可能仍是空白。
    });
  // iframe 内的 requestAnimationFrame 在非顶层/砂箱文档可能被节流，改用父页面短等待让合成器落帧。
  await iframe.page().waitForTimeout(150);
  const data = await frame.evaluate(() => {
    const tabs = document.querySelector(".nav-tabs");
    const bg = (el: Element | null) => (el ? getComputedStyle(el).backgroundColor : "");
    return {
      textColor: tabs ? getComputedStyle(tabs).color : "",
      theme: document.documentElement.getAttribute("data-theme"),
      article: bg(document.querySelector("article")),
      body: bg(document.body),
      html: bg(document.documentElement),
    };
  });
  const opaque = [data.body, data.html].filter((bg) => parseColor(bg)[3] === 1);
  const background = opaque[0] ?? "rgba(0, 0, 0, 0)";
  return {
    ...data,
    background,
    backgroundLuminance: opaque.length ? luminance(parseColor(background)) : 0,
    ratio: opaque.length ? contrastRatio(data.textColor, background) : 0,
  };
}

const tabsMarkdown =
  "{% tabs active:1 %}\n<!-- tab 第一页 -->\n第一页正文\n<!-- tab 第二页 -->\n第二页正文\n{% endtabs %}";

/**
 * 证据截图前先丢掉一张并稍等：本轮实测发现，Chrome 窗口被其它窗口遮挡时窗口帧会被抑制，
 * 首张截图只触发帧生成、拍到的是空白；第二张才包含真实绘制内容。
 * - element：对 iframe 元素截图，能可靠触发子帧绘制（非顶栏场景）；
 * - page：整页截图，避免元素截图滚动页面，导致顶栏菜单项在指针下改换悬停预览。
 * 仅影响取证，不影响断言。
 */
async function warmupPaint(iframe: Locator, mode: "element" | "page" = "element") {
  if (mode === "page") await iframe.page().screenshot().catch(() => undefined);
  else await iframe.screenshot().catch(() => undefined);
  await iframe.page().waitForTimeout(300);
}

test.describe("BLG-20261031-03 标签实时预览", () => {
  test("正文 NodeView：标题连续输入合并请求，属性面板开关零新增请求", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    const slug = await createRecord(page, "posts", folding("初始正文"));
    await openPostEditor(page, slug);

    const iframe = page.locator(".stellar-tag-node iframe").first();
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => srcdocOf(iframe)).toContain("初始正文");
    // 等首屏请求与高度消息落定，再开始计数。
    await page.waitForTimeout(800);
    diag.previewRequests.length = 0;

    const title = page.locator("#post-title");
    await title.click();
    await title.pressSequentially("abcdefghij", { delay: 40 });
    await page.waitForTimeout(1_800);
    const afterTyping = diag.previewRequests.length;
    // 10 次击键须被合并；同时必须至少触发 1 次（证明确实跟踪 context 变化）。
    expect(afterTyping).toBeGreaterThan(0);
    expect(afterTyping).toBeLessThanOrEqual(2);

    const beforePanel = diag.previewRequests.length;
    const dialog = await openPropertyPanel(page);
    await dialog.getByRole("button", { name: "取消" }).click();
    await page.waitForTimeout(900);
    expect(diag.previewRequests.length - beforePanel).toBe(0);

    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        nodeviewTitleTypingRequests: afterTyping,
        propertyPanelExtraRequests: diag.previewRequests.length - beforePanel,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("正文 NodeView：更新中保留旧 iframe 与高度，成功后替换内容", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    const slug = await createRecord(page, "posts", folding("初始正文"));
    await openPostEditor(page, slug);

    const iframe = page.locator(".stellar-tag-node iframe").first();
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => srcdocOf(iframe)).toContain("初始正文");
    await expect.poll(() => heightOf(iframe), { timeout: 20_000 }).toBeGreaterThan(64);
    const oldSrcdoc = await srcdocOf(iframe);
    const oldHeight = await heightOf(iframe);
    await markIdentity(iframe);

    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    let held = 0;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });

    let pendingHeight = 0;
    try {
      const dialog = await openPropertyPanel(page);
      await dialog.locator("#schema-field-content").fill("更新后的正文");
      await expect.poll(() => editorMarkdown(page), { timeout: 20_000 }).toContain("更新后的正文");
      await expect.poll(() => held, { timeout: 20_000 }).toBeGreaterThan(0);

      // 响应被扣住时：同一 iframe 元素仍在，旧 srcdoc 与高度不回退。
      expect(await hasIdentity(iframe)).toBe(true);
      expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
      pendingHeight = await heightOf(iframe);
      expect(pendingHeight).toBeGreaterThanOrEqual(oldHeight - 1);
      expect(pendingHeight).toBeGreaterThan(64);
    } finally {
      release();
    }

    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).toContain("更新后的正文");
    await page.unroute(`**${PREVIEW_PATH}`);
    expect(await hasIdentity(iframe)).toBe(true);
    const finalHeight = await heightOf(iframe);
    expect(finalHeight).toBeGreaterThan(64);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        nodeviewOldHeight: oldHeight,
        nodeviewPendingHeight: pendingHeight,
        nodeviewFinalHeight: finalHeight,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("侧栏插入面板：friends 更新中 iframe 身份与高度保持，最终渲染动态卡片", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });

    await page.locator("button[aria-pressed]", { hasText: "设置" }).click();
    const panel = page.locator("[data-shortcode-insert-panel]");
    await expect(panel).toBeVisible({ timeout: 15_000 });
    await panel.getByLabel("选择标签插件").selectOption("friends");
    // friends 是结构化插件：选中后原文为空（“填写参数后预览”），按真实输入流程填入源码。
    const source = panel.getByRole("textbox", { name: "Shortcode 源码", exact: true });
    await source.fill("{% friends api:/api/friends limit:3 %}");

    const iframe = panel.locator("iframe");
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    const frame = panel.frameLocator("iframe");
    await expect(frame.locator(".user-card").first()).toBeVisible({ timeout: 25_000 });
    const initialCards = await frame.locator(".user-card").count();
    expect(initialCards).toBeGreaterThan(0);
    const oldHeight = await heightOf(iframe);
    expect(oldHeight).toBeGreaterThan(100);
    const oldSrcdoc = await srcdocOf(iframe);
    await markIdentity(iframe);

    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    let held = 0;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });
    let pendingHeight = 0;
    try {
      await source.fill("{% friends api:/api/friends limit:5 %}");
      await expect.poll(() => held, { timeout: 20_000 }).toBeGreaterThan(0);
      expect(await hasIdentity(iframe)).toBe(true);
      expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
      pendingHeight = await heightOf(iframe);
      expect(pendingHeight).toBeGreaterThanOrEqual(oldHeight - 1);
    } finally {
      release();
    }

    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).not.toBe(oldSrcdoc);
    await page.unroute(`**${PREVIEW_PATH}`);
    expect(await hasIdentity(iframe)).toBe(true);
    await expect(frame.locator(".user-card").first()).toBeVisible({ timeout: 25_000 });
    expect(await heightOf(iframe)).toBeGreaterThan(100);

    // 动态数据必须来自预览 fixture，且带 CORS 头；不能落到真实数据端点。
    expect(diag.apiResponses.some((item) => item.url.includes("/api/v1/preview/friends"))).toBe(true);
    const friendsResponses = diag.apiResponses.filter((item) => item.url.includes("/api/v1/preview/friends"));
    expect(friendsResponses.every((item) => item.cors === "*")).toBe(true);
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        insertPanelCards: initialCards,
        insertPanelOldHeight: oldHeight,
        insertPanelPendingHeight: pendingHeight,
        insertPanelFinalHeight: await heightOf(iframe),
        friendsFixtureResponses: friendsResponses,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("标签 Studio：friends 更新中 iframe 身份与高度保持，最终渲染动态卡片", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/plugins/shortcodes");
    await page.getByPlaceholder("搜索插件...").fill("friends");
    await page.locator(".studio-list-item").first().click();

    const iframe = page.locator(".studio-preview-iframe");
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    const frame = page.frameLocator(".studio-preview-iframe");
    await expect(frame.locator(".user-card").first()).toBeVisible({ timeout: 25_000 });
    const initialCards = await frame.locator(".user-card").count();
    expect(initialCards).toBeGreaterThan(0);
    const oldHeight = await heightOf(iframe);
    expect(oldHeight).toBeGreaterThan(100);
    const oldSrcdoc = await srcdocOf(iframe);
    await markIdentity(iframe);

    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    let held = 0;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });
    let pendingHeight = 0;
    try {
      const source = page.locator("textarea.studio-code-editor");
      await source.fill("{% friends api:/api/friends limit:2 %}");
      await expect.poll(() => held, { timeout: 20_000 }).toBeGreaterThan(0);
      expect(await hasIdentity(iframe)).toBe(true);
      expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
      pendingHeight = await heightOf(iframe);
      expect(pendingHeight).toBeGreaterThanOrEqual(oldHeight - 1);
    } finally {
      release();
    }

    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).not.toBe(oldSrcdoc);
    await page.unroute(`**${PREVIEW_PATH}`);
    expect(await hasIdentity(iframe)).toBe(true);
    expect(await heightOf(iframe)).toBeGreaterThan(100);

    expect(diag.apiResponses.some((item) => item.url.includes("/api/v1/preview/friends"))).toBe(true);

    // 同一管线下的 sites 也必须渲染动态卡片（验收要求 friends/sites 可见）。
    await page.getByPlaceholder("搜索插件...").fill("sites");
    await page.locator(".studio-list-item").first().click();
    const sitesFrame = page.frameLocator(".studio-preview-iframe");
    await expect(sitesFrame.locator(".site-card").first()).toBeVisible({ timeout: 25_000 });
    const siteCards = await sitesFrame.locator(".site-card").count();
    expect(siteCards).toBeGreaterThan(0);
    const sitesResponse = diag.apiResponses.filter((item) => item.url.includes("/api/v1/preview/sites"));
    expect(sitesResponse.length).toBeGreaterThan(0);
    expect(sitesResponse.every((item) => item.cors === "*")).toBe(true);

    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        studioCards: initialCards,
        studioSiteCards: siteCards,
        studioOldHeight: oldHeight,
        studioPendingHeight: pendingHeight,
        studioFinalHeight: await heightOf(iframe),
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("顶栏悬停/聚焦：动态标签气泡有内容，390×844 无横向溢出", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page);
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });

    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });

    await menu.locator("button").filter({ hasText: "friends" }).first().hover();
    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    const bubbleFrame = bubble.frameLocator("iframe");
    await expect(bubbleFrame.locator(".user-card").first()).toBeVisible({ timeout: 25_000 });
    const hoverCards = await bubbleFrame.locator(".user-card").count();
    expect(hoverCards).toBeGreaterThan(0);
    const hoverCaption = (await bubbleFrame.locator(".image-caption").first().textContent())?.trim() ?? "";
    expect(hoverCaption.length).toBeGreaterThan(0);

    const hoverMetrics = await page.evaluate(() => {
      const el = document.querySelector(".insert-bubble") as HTMLElement | null;
      const rect = el?.getBoundingClientRect();
      return {
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
        bubbleWidth: rect?.width ?? 0,
        bubbleRight: rect?.right ?? 0,
      };
    });
    expect(hoverMetrics.scrollWidth - hoverMetrics.innerWidth).toBeLessThanOrEqual(0);
    expect(hoverMetrics.bubbleRight).toBeLessThanOrEqual(hoverMetrics.innerWidth);
    expect(hoverMetrics.bubbleWidth).toBeLessThanOrEqual(320 + 1);

    await page.screenshot({ path: `${BASELINE_DIR}/e2e-toolbar-bubble-390.png` });

    // 键盘 focus 同样触发预览；动态 tabs 必须渲染真实结构而非 CSS 空壳。
    await page.mouse.move(0, 0);
    const tabsItem = menu.locator("button").filter({ hasText: "tabs" }).first();
    await tabsItem.focus();
    await expect(bubbleFrame.locator(".nav-tabs").first()).toBeVisible({ timeout: 25_000 });
    const tabsPanes = await bubbleFrame.locator(".tab-pane").count();
    expect(tabsPanes).toBeGreaterThan(0);
    // 快速切换不串台：friends 的卡片不应残留在 tabs 预览里。
    await expect(bubbleFrame.locator(".user-card")).toHaveCount(0);

    const focusMetrics = await page.evaluate(() => {
      const el = document.querySelector(".insert-bubble") as HTMLElement | null;
      const rect = el?.getBoundingClientRect();
      return {
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
        bubbleRight: rect?.right ?? 0,
      };
    });
    expect(focusMetrics.scrollWidth - focusMetrics.innerWidth).toBeLessThanOrEqual(0);
    expect(focusMetrics.bubbleRight).toBeLessThanOrEqual(focusMetrics.innerWidth);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-toolbar-focus-390.png` });

    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    await writeFile(
      `${BASELINE_DIR}/e2e-toolbar-390.json`,
      JSON.stringify({ hoverMetrics, focusMetrics, hoverCards, tabsPanes }, null, 2),
    );
    console.log(
      JSON.stringify({
        hoverMetrics,
        focusMetrics,
        hoverCards,
        tabsPanes,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("Markdown 分屏：500 失败保留旧预览与错误条，重试恢复", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/pages/edit/new");
    await page.locator('select[title="编辑器模式"]').selectOption("markdown");
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });

    const initialDoc = "# 分屏标题\n\n{% note color:green 提示 初始预览正文 %}";
    await textarea.fill(initialDoc);
    const iframe = page.locator('iframe[title="预览"]');
    await expect(iframe).toHaveAttribute("srcdoc", /初始预览正文/, { timeout: 30_000 });
    const oldSrcdoc = await srcdocOf(iframe);
    await markIdentity(iframe);

    let forceFailure = false;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      if (forceFailure) {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ error: "forced preview failure" }),
        });
        return;
      }
      await route.continue();
    });

    try {
      forceFailure = true;
      await textarea.fill(initialDoc + "\n\n新增段落触发失败");
      await expect(page.getByText("预览渲染失败，仍显示上一次结果")).toBeVisible({ timeout: 20_000 });
      expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
      expect(await hasIdentity(iframe)).toBe(true);
    } finally {
      forceFailure = false;
    }

    await page.getByRole("button", { name: "重试", exact: true }).click();
    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).toContain("新增段落触发失败");
    expect(await hasIdentity(iframe)).toBe(true);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        markdownFailureRetained: true,
        markdownFinalHasNewText: true,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("Fixture 对照：Studio rating/vote 契约正确，交互只写 /api/v1/preview/*", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/plugins/shortcodes");

    // rating：rating.js 走站内原生形状（GET /api/v1/preview/rating → {avg,count}），均分 4.5、130 人。
    await page.getByPlaceholder("搜索插件...").fill("rating");
    await page.locator(".studio-list-item").first().click();
    const ratingFrame = page.frameLocator(".studio-preview-iframe");
    await expect(page.locator(".studio-preview-iframe")).toBeVisible({ timeout: 30_000 });
    await expect(ratingFrame.locator(".ds-rating").first()).toBeVisible({ timeout: 25_000 });
    await expect(ratingFrame.locator(".avg").first()).toHaveText("(4.5)", { timeout: 25_000 });
    await expect(ratingFrame.locator(".count").first()).toHaveText("130");
    await expect
      .poll(() => previewResponse(diag, "/api/v1/preview/rating").length, { timeout: 20_000 })
      .toBeGreaterThan(0);
    const ratingInfo = previewResponse(diag, "/api/v1/preview/rating");
    expect(ratingInfo.every((item) => item.status === 200 && item.cors === "*")).toBe(true);

    await ratingFrame.locator('.star[data-value="5"]').click({ force: true });
    await expect
      .poll(() => previewResponse(diag, "/api/v1/preview/rating").length, { timeout: 20_000 })
      .toBeGreaterThan(1);
    const ratingUpdate = previewResponse(diag, "/api/v1/preview/rating");
    expect(ratingUpdate.every((item) => item.status === 200 && item.cors === "*")).toBe(true);
    // 预览写请求只允许命中 /api/v1/preview/*，真实 /api/rating 必须 0 次。
    expect(diag.realWrites).toEqual([]);

    // vote：fixture 计数 86/12，点赞后乐观更新为 87。
    await page.getByPlaceholder("搜索插件...").fill("vote");
    await page.locator(".studio-list-item").first().click();
    const voteFrame = page.frameLocator(".studio-preview-iframe");
    await expect(voteFrame.locator(".ds-vote").first()).toBeVisible({ timeout: 25_000 });
    await expect(voteFrame.locator(".up").first()).toHaveText("86", { timeout: 25_000 });
    await voteFrame.locator(".vote-up").first().click({ force: true });
    await expect
      .poll(() => previewResponse(diag, "/api/v1/preview/vote/update").length, { timeout: 20_000 })
      .toBeGreaterThan(0);
    const voteUpdate = previewResponse(diag, "/api/v1/preview/vote/update");
    expect(voteUpdate.every((item) => item.status === 200 && item.cors === "*")).toBe(true);
    await expect(voteFrame.locator(".up").first()).toHaveText("87");

    // 预览写请求只能命中 /api/v1/preview/*；真实端点必须 0 次，且无 CORS 报错。
    expect(diag.realWrites).toEqual([]);
    expect(corsConsoleErrors(diag)).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        ratingInfo,
        ratingUpdate,
        voteUpdate,
        realWrites: diag.realWrites,
        corsConsoleErrors: corsConsoleErrors(diag),
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("H1 正文 NodeView 真实上下文：无 Storage 回退，写守卫阻断真实 vote 写", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    const slug = await createRecord(page, "posts", "{% vote 有帮助吗 id:preview-guard %}");
    await openPostEditor(page, slug);

    const iframe = page.locator(".stellar-tag-node iframe").first();
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => srcdocOf(iframe)).toContain('data-preview-write-guard="1"');
    const srcdoc = await srcdocOf(iframe);
    expect(srcdoc).not.toContain('data-preview-storage="1"');

    await page.locator(".stellar-tag-node").first().click({ force: true });
    const frame = await contentFrame(iframe);
    await expect(frame.locator(".ds-vote").first()).toBeVisible({ timeout: 20_000 });
    await frame.locator(".vote-up").first().click({ force: true }).catch(() => {});
    await page.waitForTimeout(500);

    const guard = await frame.evaluate(async () => {
      const w = window as Window & { __stellarPreviewWriteBlocked?: unknown; fetch: typeof fetch };
      w.__stellarPreviewWriteBlocked = null;
      let fetchError = "";
      try {
        await w.fetch("/api/vote/update?id=guard&value=up", { method: "POST" });
      } catch (error) {
        fetchError = (error as Error)?.name || String(error);
      }
      const afterFetch = w.__stellarPreviewWriteBlocked;
      w.__stellarPreviewWriteBlocked = null;
      let ratingError = "";
      try {
        await w.fetch("/api/rating/update?id=guard&value=5", { method: "POST" });
      } catch (error) {
        ratingError = (error as Error)?.name || String(error);
      }
      const afterRating = w.__stellarPreviewWriteBlocked;
      w.__stellarPreviewWriteBlocked = null;
      let xhrError = "";
      try {
        const xhr = new XMLHttpRequest();
        xhr.open("POST", "/api/vote/update?id=guard");
        xhr.send("x");
      } catch (error) {
        xhrError = (error as Error)?.name || String(error);
      }
      const afterXhr = w.__stellarPreviewWriteBlocked;
      w.__stellarPreviewWriteBlocked = null;
      let beacon: unknown = null;
      try {
        beacon = navigator.sendBeacon("/api/vote/update?id=guard", "x");
      } catch (error) {
        beacon = "throw:" + ((error as Error)?.name || String(error));
      }
      return {
        fetchError,
        afterFetch,
        ratingError,
        afterRating,
        xhrError,
        afterXhr,
        beacon,
        afterBeacon: w.__stellarPreviewWriteBlocked,
        hasGuard: Boolean(document.querySelector('script[data-preview-write-guard="1"]')),
        hasStorage: Boolean(document.querySelector('script[data-preview-storage="1"]')),
      };
    });

    expect(guard.hasGuard).toBe(true);
    expect(guard.hasStorage).toBe(false);
    expect(guard.fetchError).toBe("PreviewWriteBlockedError");
    expect((guard.afterFetch as { method?: string } | null)?.method).toBe("POST");
    expect(String((guard.afterFetch as { url?: string } | null)?.url)).toContain("/api/vote/update");
    expect(guard.ratingError).toBe("PreviewWriteBlockedError");
    expect(String((guard.afterRating as { url?: string } | null)?.url)).toContain("/api/rating/update");
    expect(guard.xhrError).toBe("PreviewWriteBlockedError");
    expect(String((guard.afterXhr as { url?: string } | null)?.url)).toContain("/api/vote/update");
    expect(guard.beacon).toBe(false);
    expect(String((guard.afterBeacon as { url?: string } | null)?.url)).toContain("/api/vote/update");
    await page.waitForTimeout(300);
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-nodeview-write-guard.png` });
    console.log(
      JSON.stringify({
        nodeviewGuard: guard,
        realWrites: diag.realWrites,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("H1 Markdown 真实上下文：写守卫阻断真实 vote 写，页面不崩", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/pages/edit/new");
    await page.locator('select[title="编辑器模式"]').selectOption("markdown");
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });
    await textarea.fill("{% vote 有帮助吗 id:md-guard %}");
    const iframe = page.locator('iframe[title="预览"]');
    await expect(iframe).toHaveAttribute("srcdoc", /data-preview-write-guard/, { timeout: 30_000 });
    const srcdoc = await srcdocOf(iframe);
    expect(srcdoc).not.toContain('data-preview-storage="1"');

    const frame = await contentFrame(iframe);
    await expect(frame.locator(".ds-vote").first()).toBeVisible({ timeout: 20_000 });
    await frame.locator(".vote-up").first().click({ force: true }).catch(() => {});
    await page.waitForTimeout(500);
    const guard = await frame.evaluate(async () => {
      const w = window as Window & { __stellarPreviewWriteBlocked?: unknown; fetch: typeof fetch };
      w.__stellarPreviewWriteBlocked = null;
      let fetchError = "";
      try {
        await w.fetch("/api/vote/update?id=md-guard&value=down", { method: "POST" });
      } catch (error) {
        fetchError = (error as Error)?.name || String(error);
      }
      return { fetchError, marker: w.__stellarPreviewWriteBlocked };
    });

    expect(guard.fetchError).toBe("PreviewWriteBlockedError");
    expect(String((guard.marker as { url?: string } | null)?.url)).toContain("/api/vote/update");
    await page.waitForTimeout(300);
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    await expect(textarea).toBeVisible();
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-markdown-write-guard.png` });
    console.log(
      JSON.stringify({
        markdownGuard: guard,
        realWrites: diag.realWrites,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("M1 顶栏切换插件：B 加载前不展示 A 的标题/内容", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });

    await menu.locator("button").filter({ hasText: "friends" }).first().hover();
    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await expect(bubble.frameLocator("iframe").locator(".user-card").first()).toBeVisible({ timeout: 25_000 });
    await page.waitForTimeout(500);

    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    let held = 0;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });

    let mixedWhilePending = false;
    try {
      await menu.locator("button").filter({ hasText: "tabs" }).first().hover();
      await expect.poll(() => held, { timeout: 20_000 }).toBeGreaterThan(0);
      await expect(bubble).toContainText(/Tabs/i);
      // 新文档未返回前旧 friends 文档必须已清空（不出现“标题 B 内容 A”）。
      const iframeCount = await bubble.locator("iframe").count();
      mixedWhilePending = iframeCount > 0;
      expect(mixedWhilePending).toBe(false);
    } finally {
      release();
    }

    const bubbleFrame = bubble.frameLocator("iframe");
    await expect(bubbleFrame.locator(".nav-tabs").first()).toBeVisible({ timeout: 25_000 });
    await expect(bubbleFrame.locator(".user-card")).toHaveCount(0);
    await page.unroute(`**${PREVIEW_PATH}`);
    expect(diag.pageErrors).toEqual([]);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-toolbar-m1-switch.png` });
    console.log(
      JSON.stringify({
        mixedWhilePending,
        held,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("M2 顶栏 focus 快速遍历：只渲染最后一项，插入不补发请求", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });

    const names = ["note", "tabs", "friends", "sites"];
    const items = names.map((name) => menu.locator("button").filter({ hasText: name }).first());
    for (const item of items) {
      await item.focus();
      await page.waitForTimeout(60);
    }
    await page.waitForTimeout(900);
    const afterFocus = componentPreviewRequests(diag).length;
    expect(afterFocus).toBeGreaterThan(0);
    expect(afterFocus).toBeLessThanOrEqual(2);

    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await expect(bubble.frameLocator("iframe").locator(".site-card").first()).toBeVisible({ timeout: 25_000 });

    const beforeClick = componentPreviewRequests(diag).length;
    await items[items.length - 1].click();
    await page.waitForTimeout(800);
    const afterClick = componentPreviewRequests(diag).length;
    expect(afterClick).toBe(beforeClick);
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        afterFocus,
        beforeClick,
        afterClick,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("M3 Studio 无效参数：旧预览保留并提示，补全后提示消失", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/plugins/shortcodes");
    await page.getByPlaceholder("搜索插件...").fill("copy");
    await page.locator(".studio-list-item").first().click();
    const iframe = page.locator(".studio-preview-iframe");
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => srcdocOf(iframe)).not.toBe("");
    const oldSrcdoc = await srcdocOf(iframe);
    await markIdentity(iframe);

    const source = page.locator("textarea.studio-code-editor");
    await source.fill("{% copy %}");
    await expect(page.getByText("参数无效，预览未更新", { exact: true }).first()).toBeVisible({ timeout: 20_000 });
    expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
    expect(await hasIdentity(iframe)).toBe(true);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-studio-invalid-params-notice.png` });

    await source.fill("{% copy 复制内容 %}");
    await expect(page.getByText("参数无效，预览未更新", { exact: true })).toHaveCount(0, { timeout: 20_000 });
    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).not.toBe(oldSrcdoc);
    expect(await hasIdentity(iframe)).toBe(true);
    expect(diag.pageErrors).toEqual([]);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-studio-invalid-params-recovered.png` });
    console.log(
      JSON.stringify({
        studioInvalidRetained: true,
        studioNoticeCleared: true,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("M3 插入面板无效参数：旧预览保留并提示，补全后提示消失", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator("button[aria-pressed]", { hasText: "设置" }).click();
    const panel = page.locator("[data-shortcode-insert-panel]");
    await expect(panel).toBeVisible({ timeout: 15_000 });
    await panel.getByLabel("选择标签插件").selectOption("copy");
    const source = panel.getByRole("textbox", { name: "Shortcode 源码", exact: true });
    await source.fill("{% copy npm install %}");
    const iframe = panel.locator("iframe");
    await expect(iframe).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => srcdocOf(iframe)).not.toBe("");
    const oldSrcdoc = await srcdocOf(iframe);
    await markIdentity(iframe);

    await source.fill("{% copy %}");
    await expect(panel.getByText("参数无效，预览未更新", { exact: true })).toBeVisible({ timeout: 20_000 });
    expect(await srcdocOf(iframe)).toBe(oldSrcdoc);
    expect(await hasIdentity(iframe)).toBe(true);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-insert-invalid-params-notice.png` });

    await source.fill("{% copy npm install stellar-theme %}");
    await expect(panel.getByText("参数无效，预览未更新", { exact: true })).toHaveCount(0, { timeout: 20_000 });
    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).not.toBe(oldSrcdoc);
    expect(await hasIdentity(iframe)).toBe(true);
    expect(diag.pageErrors).toEqual([]);
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-insert-invalid-params-recovered.png` });
    console.log(
      JSON.stringify({
        insertInvalidRetained: true,
        insertNoticeCleared: true,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("Markdown 首次失败：显示“预览渲染失败”而非“仍显示上一次结果”", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    await login(page);

    // “首次”必须从 MarkdownEditor 挂载时的第一次预览就失败：路由要在导航/切换前安装，
    // 否则空内容的首屏预览会先成功，后续失败自然属于“有历史文档”。
    let forceFailure = true;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      if (forceFailure) {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ error: "forced first preview failure" }),
        });
        return;
      }
      await route.continue();
    });

    await page.goto("/console/pages/edit/new");
    await page.locator('select[title="编辑器模式"]').selectOption("markdown");
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });

    await textarea.fill("# 首次失败\n\n{% note color:green 提示 首次内容 %}");
    await expect(page.getByText("预览渲染失败", { exact: true }).first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText("预览渲染失败，仍显示上一次结果")).toHaveCount(0);
    const iframe = page.locator('iframe[title="预览"]');
    expect(await srcdocOf(iframe)).toBe("");
    await page.screenshot({ path: `${BASELINE_DIR}/e2e-markdown-first-failure.png` });

    forceFailure = false;
    await page.getByRole("button", { name: "重试", exact: true }).first().click();
    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).toContain("首次内容");
    expect(diag.pageErrors).toEqual([]);
    console.log(
      JSON.stringify({
        firstFailureMessage: "预览渲染失败",
        historicalMessageAbsent: true,
        retryRecovered: true,
        pageErrors: diag.pageErrors,
        consoleErrors: diag.consoleErrors,
      }),
    );
  });

  test("可见性：tabs 预览文本在深色容器中必须可读（Studio/插入面板/顶栏）", async ({ page }) => {
    test.setTimeout(180_000);
    const diag = watch(page);
    await login(page);
    const results: Record<string, unknown>[] = [];

    async function measure(label: string, iframe: Locator, shot: string) {
      const frame = await contentFrame(iframe);
      await expect(frame.locator(".nav-tabs").first()).toBeVisible({ timeout: 25_000 });
      // 等外部高亮样式表结算并完成首绘，否则截图只是「尚未绘制」。
      await frame
        .waitForFunction(
          () =>
            document.readyState === "complete" &&
            [...document.querySelectorAll('link[rel="stylesheet"]')].every((link) =>
              Boolean((link as HTMLLinkElement).sheet),
            ),
          null,
          { timeout: 20_000 },
        )
        .catch(() => {});
      // iframe 内的 rAF 在砂箱文档可能被节流；等父页面落帧后再截图/取值。
      await page.waitForTimeout(150);
      const textColor = await frame
        .locator(".nav-tabs")
        .first()
        .evaluate((el) => getComputedStyle(el).color);
      const frameBg = await frame.evaluate(() => {
        const bg = (el: Element | null) => (el ? getComputedStyle(el).backgroundColor : null);
        return {
          article: bg(document.querySelector("article")),
          body: bg(document.body),
          html: bg(document.documentElement),
        };
      });
      const ancestorBackgrounds = await iframe.evaluate((el) => {
        const out: string[] = [];
        let node: Element | null = el;
        while (node) {
          out.push(getComputedStyle(node).backgroundColor);
          node = node.parentElement;
        }
        return out;
      });
      const parentBg =
        ancestorBackgrounds.find((bg) => Boolean(bg) && parseColor(bg)[3] > 0) ?? "rgb(255, 255, 255)";
      const candidates = [frameBg.article, frameBg.body, frameBg.html].filter(
        (bg): bg is string => Boolean(bg) && parseColor(bg as string)[3] > 0,
      );
      const background = candidates[0] ?? parentBg;
      const ratio = contrastRatio(textColor, background);
      results.push({ label, textColor, frameBg, parentBg, background, ratio });
      await warmupPaint(iframe, label === "toolbar" ? "page" : "element");
      if (shot.includes("studio")) {
        await page.locator(".studio-canvas-body").screenshot({ path: `${BASELINE_DIR}/${shot}` });
      } else if (shot.includes("insert")) {
        await page.locator("[data-shortcode-insert-panel]").screenshot({ path: `${BASELINE_DIR}/${shot}` });
      } else {
        await page.locator(".insert-bubble").screenshot({ path: `${BASELINE_DIR}/${shot}` });
      }
      expect.soft(ratio, `${label} 文本对比度 ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(4.5);
      return ratio;
    }

    // Studio
    await page.goto("/console/plugins/shortcodes");
    await page.getByPlaceholder("搜索插件...").fill("tabs");
    await page.locator(".studio-list-item").first().click();
    await expect(page.locator(".studio-preview-iframe")).toBeVisible({ timeout: 30_000 });
    await measure("studio", page.locator(".studio-preview-iframe"), "e2e-visibility-studio-tabs.png");

    // 插入面板
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator("button[aria-pressed]", { hasText: "设置" }).click();
    const panel = page.locator("[data-shortcode-insert-panel]");
    await expect(panel).toBeVisible({ timeout: 15_000 });
    await panel.getByLabel("选择标签插件").selectOption("tabs");
    await panel
      .getByRole("textbox", { name: "Shortcode 源码", exact: true })
      .fill("{% tabs active:1 %}\n<!-- tab 第一页 -->\n第一页正文\n<!-- tab 第二页 -->\n第二页正文\n{% endtabs %}");
    await expect(panel.locator("iframe")).toBeVisible({ timeout: 30_000 });
    await measure("insert", panel.locator("iframe"), "e2e-visibility-insert-tabs.png");

    // 顶栏气泡
    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });
    await menu.locator("button").filter({ hasText: "tabs" }).first().focus();
    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await measure("toolbar", bubble.locator("iframe"), "e2e-visibility-toolbar-tabs.png");

    console.log(JSON.stringify({ visibility: results, pageErrors: diag.pageErrors }));
    expect(diag.pageErrors).toEqual([]);
  });

  // 最终验收补充：计划要求「五处」预览都可见。上面只覆盖 Studio/插入面板/顶栏，
  // 这里补 NodeView 与 Markdown 分屏，并在 390px 视口复核顶栏气泡的同一表面。
  test("可见性补充：NodeView/Markdown 分屏与 390px 气泡的 tabs 文本可读、表面不透明", async ({ page }) => {
    test.setTimeout(180_000);
    const diag = watch(page);
    await login(page);
    const results: Record<string, unknown>[] = [];

    async function probe(label: string, iframe: Locator, shot: string) {
      const data = await measurePreviewTabs(iframe);
      // 预览表面必须不透明（V1）；否则深色 Console 会透上来，文本与背景同色系。
      for (const key of ["html", "body"] as const) {
        expect(parseColor(data[key])[3], `${label} ${key} 背景必须不透明（实际 ${data[key]}）`).toBe(1);
      }
      results.push({ label, ...data });
      await warmupPaint(iframe, label.startsWith("toolbar") ? "page" : "element");
      await page.screenshot({ path: `${BASELINE_DIR}/${shot}` });
      expect(data.ratio, `${label} 文本对比度 ${data.ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(4.5);
      return data;
    }

    // 1) 正文 NodeView：真实上下文（不注入内存 Storage）。
    const slug = await createRecord(page, "posts", tabsMarkdown);
    await openPostEditor(page, slug);
    const nodeIframe = page.locator(".stellar-tag-node iframe").first();
    await expect(nodeIframe).toBeVisible({ timeout: 30_000 });
    await probe("nodeview", nodeIframe, "final-visibility-nodeview-tabs.png");
    // 无布局跳动：文档稳定后 800ms 内高度不因表面注入/回传反复收缩。
    const stableHeight = await heightOf(nodeIframe);
    await page.waitForTimeout(800);
    const laterHeight = await heightOf(nodeIframe);
    expect(Math.abs(laterHeight - stableHeight), `NodeView 高度跳动 ${stableHeight}→${laterHeight}`).toBeLessThanOrEqual(2);

    // 2) Markdown 分屏预览。
    await page.goto("/console/pages/edit/new");
    await page.locator('select[title="编辑器模式"]').selectOption("markdown");
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });
    await textarea.fill(tabsMarkdown);
    const mdIframe = page.locator('iframe[title="预览"]');
    await expect(mdIframe).toBeVisible({ timeout: 30_000 });
    await probe("markdown", mdIframe, "final-visibility-markdown-tabs.png");

    // 3) 390×844 顶栏气泡：同一表面在窄视口同样不透明可读。
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });
    await menu.locator("button").filter({ hasText: "tabs" }).first().focus();
    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await probe("toolbar-390", bubble.locator("iframe"), "final-visibility-toolbar-390-tabs.png");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    await writeFile(
      `${BASELINE_DIR}/final-visibility.json`,
      JSON.stringify({ colorScheme: "light", viewport: { desktop: "1280x720", mobile: "390x844" }, results }, null, 2),
    );
    console.log(JSON.stringify({ finalVisibility: results, overflow, pageErrors: diag.pageErrors }));
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
  });

  // 系统深色偏好（站点 appearance 为 auto）时，预览表面必须是「深底 + 浅字」，
  // 而不是把深色 Console 透上来。Playwright 默认模拟浅色，需显式 emulateMedia。
  test("可见性补充：深色系统偏好下五处 tabs 表面为不透明深底且文本可读", async ({ page }) => {
    test.setTimeout(240_000);
    const diag = watch(page);
    await page.emulateMedia({ colorScheme: "dark" });
    await login(page);
    const results: Record<string, unknown>[] = [];

    async function probe(label: string, iframe: Locator) {
      const data = await measurePreviewTabs(iframe);
      for (const key of ["html", "body"] as const) {
        expect(parseColor(data[key])[3], `${label} ${key} 深色背景必须不透明（实际 ${data[key]}）`).toBe(1);
      }
      expect(data.backgroundLuminance, `${label} 深色主题表面应为深底（实际 ${data.background}）`).toBeLessThan(0.1);
      expect(data.ratio, `${label} 深色文本对比度 ${data.ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(4.5);
      results.push({ label, ...data });
      await warmupPaint(iframe, label === "toolbar" ? "page" : "element");
      return data;
    }

    await page.goto("/console/plugins/shortcodes");
    await page.getByPlaceholder("搜索插件...").fill("tabs");
    await page.locator(".studio-list-item").first().click();
    await expect(page.locator(".studio-preview-iframe")).toBeVisible({ timeout: 30_000 });
    await probe("studio", page.locator(".studio-preview-iframe"));
    await page.locator(".studio-canvas-body").screenshot({ path: `${BASELINE_DIR}/final-dark-studio-tabs.png` });

    await page.goto("/console/posts/new");
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible({ timeout: 20_000 });
    await page.locator("button[aria-pressed]", { hasText: "设置" }).click();
    const panel = page.locator("[data-shortcode-insert-panel]");
    await expect(panel).toBeVisible({ timeout: 15_000 });
    await panel.getByLabel("选择标签插件").selectOption("tabs");
    await panel.getByRole("textbox", { name: "Shortcode 源码", exact: true }).fill(tabsMarkdown);
    await expect(panel.locator("iframe")).toBeVisible({ timeout: 30_000 });
    await probe("insert", panel.locator("iframe"));
    await panel.screenshot({ path: `${BASELINE_DIR}/final-dark-insert-tabs.png` });

    await page.locator('button[title="插入 Stellar 标签"]').click();
    const menu = page.locator(".insert-menu");
    await expect(menu).toBeVisible({ timeout: 15_000 });
    await menu.locator("button").filter({ hasText: "tabs" }).first().focus();
    const bubble = page.locator(".insert-bubble");
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await probe("toolbar", bubble.locator("iframe"));
    await bubble.screenshot({ path: `${BASELINE_DIR}/final-dark-toolbar-tabs.png` });

    const slug = await createRecord(page, "posts", tabsMarkdown);
    await openPostEditor(page, slug);
    const nodeIframe = page.locator(".stellar-tag-node iframe").first();
    await expect(nodeIframe).toBeVisible({ timeout: 30_000 });
    await probe("nodeview", nodeIframe);
    await page.screenshot({ path: `${BASELINE_DIR}/final-dark-nodeview-tabs.png` });

    await page.goto("/console/pages/edit/new");
    await page.locator('select[title="编辑器模式"]').selectOption("markdown");
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });
    await textarea.fill(tabsMarkdown);
    const mdIframe = page.locator('iframe[title="预览"]');
    await expect(mdIframe).toBeVisible({ timeout: 30_000 });
    await probe("markdown", mdIframe);

    await writeFile(
      `${BASELINE_DIR}/final-visibility-dark.json`,
      JSON.stringify({ colorScheme: "dark", results }, null, 2),
    );
    console.log(JSON.stringify({ finalVisibilityDark: results, pageErrors: diag.pageErrors }));
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
  });

  // 收尾修复 M1（真实 UI 流程）：新建文章保存草稿后 storageKey 从 post:new 变为 post:<slug>，
  // 正文不变、编辑器实例不重挂。此前只清空不补调度会让预览长期空白；这里用「扣住保存后的
  // 预览请求」证明新身份确实重新发起请求，并确认释放后预览恢复为非空旧正文。
  test("M1 真实流程：新建文章保存草稿后预览重新请求，不长期空白", async ({ page }) => {
    test.setTimeout(150_000);
    const diag = watch(page);
    await login(page);
    await page.goto("/console/posts/new");
    await expect(
      page.locator('[contenteditable="true"], textarea[placeholder^="Markdown 内容"]').first(),
    ).toBeVisible({ timeout: 20_000 });

    // 切到 Markdown 编辑器（带 storageKey 的同实例预览路径）。
    await page.getByRole("button", { name: "切换编辑器" }).click();
    await page.getByRole("option", { name: "Markdown 编辑器" }).click();
    const textarea = page.locator('textarea[placeholder^="Markdown 内容"]');
    await expect(textarea).toBeVisible({ timeout: 20_000 });

    await page.locator("#post-title").fill("M1 身份切换验收");
    await textarea.fill("# 身份切换\n\n{% note color:green 提示 新建文章预览正文 %}");
    const iframe = page.locator('iframe[title="预览"]');
    await expect(iframe).toHaveAttribute("srcdoc", /新建文章预览正文/, { timeout: 30_000 });
    const beforeSrcdoc = await srcdocOf(iframe);
    expect(beforeSrcdoc.length).toBeGreaterThan(0);
    // 标记编辑器实例与 iframe 元素：保存后若被重挂，标记会消失。
    await textarea.evaluate((el) => el.setAttribute("data-e2e-identity", "initial"));
    await markIdentity(iframe);
    // 等标题/正文防抖请求落定，避免把保存前的在途请求误计入保存后的重渲染。
    await page.waitForTimeout(1_200);

    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    let held = 0;
    await page.route(`**${PREVIEW_PATH}`, async (route) => {
      held += 1;
      await gate;
      await route.continue();
    });
    diag.previewRequests.length = 0;

    let heldSrcdoc = "";
    try {
      await page.getByRole("button", { name: "保存草稿" }).click();
      await expect(page.getByText("保存成功")).toBeVisible({ timeout: 30_000 });
      await expect(page).toHaveURL(/\/console\/posts\/edit\/(?!new)/, { timeout: 30_000 });
      // 同实例：保存只改 storageKey，不重挂编辑器/iframe。
      expect(await textarea.getAttribute("data-e2e-identity")).toBe("initial");
      expect(await hasIdentity(iframe)).toBe(true);
      // 旧实现只清空不补调度，这里会一直等不到请求而失败。
      await expect.poll(() => held, { timeout: 20_000 }).toBeGreaterThan(0);
      expect(diag.previewRequests.length).toBeGreaterThan(0);
      heldSrcdoc = await srcdocOf(iframe);
      expect(heldSrcdoc, "身份切换时应先清空旧身份的文档").toBe("");
      expect(await iframe.isVisible()).toBe(true);
    } finally {
      release();
    }

    // 释放后新身份下的预览必须恢复为非空且仍是当前正文。
    await expect.poll(() => srcdocOf(iframe), { timeout: 30_000 }).toContain("新建文章预览正文");
    const finalSrcdoc = await srcdocOf(iframe);
    expect(finalSrcdoc.length).toBeGreaterThan(0);
    expect(await hasIdentity(iframe)).toBe(true);
    await page.unroute(`**${PREVIEW_PATH}`);
    expect(diag.pageErrors).toEqual([]);
    expect(diag.realWrites).toEqual([]);
    // 证据截图前等新文档真正可就绪并出帧（站点外部高亮 CSS 为 render-blocking）。
    const finalFrame = await contentFrame(iframe);
    await finalFrame
      .waitForFunction(
        () =>
          document.readyState === "complete" &&
          [...document.querySelectorAll('link[rel="stylesheet"]')].every((link) => Boolean((link as HTMLLinkElement).sheet)),
        null,
        { timeout: 20_000, polling: 100 },
      )
      .catch(() => {});
    await warmupPaint(iframe);
    await page.screenshot({ path: `${BASELINE_DIR}/fix-m1-storagekey-flow.png` });
    await writeFile(
      `${BASELINE_DIR}/fix-m1-storagekey-flow.json`,
      JSON.stringify(
        { held, previewRequestsAfterSave: diag.previewRequests.length, heldSrcdoc, finalSrcdocLength: finalSrcdoc.length, url: page.url() },
        null,
        2,
      ),
    );
    console.log(
      JSON.stringify({
        m1HeldRequests: held,
        m1PreviewRequestsAfterSave: diag.previewRequests.length,
        m1HeldSrcdocEmpty: heldSrcdoc === "",
        m1FinalSrcdocLength: finalSrcdoc.length,
        pageErrors: diag.pageErrors,
      }),
    );
  });

  // 收尾修复 L1：反斜杠在 WHATWG URL 中等价于 `/`，`\\evil.example/...`、`/\evil.example/...`
  // 会被解析成外域协议相对 URL。写守卫必须拒绝，且不得发出任何网络请求；正常 fixture POST 仍放行。
  test("L1 写守卫：反斜杠伪造 fixture 写被拒绝且无网络请求，正常 fixture POST 放行", async ({ page }) => {
    test.setTimeout(120_000);
    const diag = watch(page);
    const evilRequests: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("evil.example")) evilRequests.push(`${request.method()} ${request.url()}`);
    });
    await login(page);
    await page.goto("/console/plugins/shortcodes");
    await page.getByPlaceholder("搜索插件...").fill("rating");
    await page.locator(".studio-list-item").first().click();
    await expect(page.locator(".studio-preview-iframe")).toBeVisible({ timeout: 30_000 });
    const iframe = page.locator(".studio-preview-iframe");
    const frame = await contentFrame(iframe);
    await expect(frame.locator(".ds-rating").first()).toBeVisible({ timeout: 25_000 });

    const outcome = await frame.evaluate(async () => {
      const w = window as Window & { __stellarPreviewWriteBlocked?: { method?: string; url?: string } | null; fetch: typeof fetch };
      async function attempt(url: string) {
        w.__stellarPreviewWriteBlocked = null;
        let name = "";
        try {
          await w.fetch(url, { method: "POST" });
          name = "resolved";
        } catch (error) {
          name = (error as Error)?.name || String(error);
        }
        return { name, marker: w.__stellarPreviewWriteBlocked };
      }
      const backslashAbsolute = await attempt("\\\\evil.example/api/v1/preview/rating");
      const slashBackslash = await attempt("/\\evil.example/api/v1/preview/rating");
      // 正常 fixture 写必须仍然放行（含 query）。
      w.__stellarPreviewWriteBlocked = null;
      let allowedStatus = 0;
      let allowedName = "";
      try {
        const response = await w.fetch("/api/v1/preview/rating/update?id=l1-e2e&value=5", { method: "POST" });
        allowedStatus = response.status;
        allowedName = "resolved";
      } catch (error) {
        allowedName = (error as Error)?.name || String(error);
      }
      return {
        hasGuard: Boolean(document.querySelector('script[data-preview-write-guard="1"]')),
        backslashAbsolute,
        slashBackslash,
        allowedName,
        allowedStatus,
        markerAfterAllowed: w.__stellarPreviewWriteBlocked,
      };
    });

    expect(outcome.hasGuard).toBe(true);
    expect(outcome.backslashAbsolute.name).toBe("PreviewWriteBlockedError");
    expect(String(outcome.backslashAbsolute.marker?.url)).toContain("evil.example");
    expect(outcome.slashBackslash.name).toBe("PreviewWriteBlockedError");
    expect(String(outcome.slashBackslash.marker?.url)).toContain("evil.example");
    // 正常 fixture POST 放行：200 + CORS，且不写真实评分。
    expect(outcome.allowedName).toBe("resolved");
    expect(outcome.allowedStatus).toBe(200);
    expect(outcome.markerAfterAllowed).toBeNull();
    await expect
      .poll(() => previewResponse(diag, "/api/v1/preview/rating/update").length, { timeout: 20_000 })
      .toBeGreaterThan(0);
    // 被拒绝的写不得产生任何网络请求。
    expect(evilRequests).toEqual([]);
    expect(diag.realWrites).toEqual([]);
    expect(diag.pageErrors).toEqual([]);
    await writeFile(
      `${BASELINE_DIR}/fix-l1-backslash-guard.json`,
      JSON.stringify({ outcome, evilRequests, ratingUpdates: previewResponse(diag, "/api/v1/preview/rating/update") }, null, 2),
    );
    console.log(JSON.stringify({ l1: outcome, evilRequests, realWrites: diag.realWrites }));
  });
});
