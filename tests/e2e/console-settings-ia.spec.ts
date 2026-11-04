import { expect, test, type Page } from "@playwright/test";
import {
  contentScroller,
  expectClearOfHeader,
  measureTouchTargets,
  openSettingsPage,
  scrollTopOf,
  settingsNav,
} from "./support/settings";

/**
 * 系统设置信息架构（BLG-20261103-01）的专项验收：5 组 12 页、旧链接、保存/放弃语义、行内状态与可读性。
 * 保存接口与字段路径的回归由 stellar-settings-groups / console-settings-canonical 等规格覆盖；
 * 本文件只读取或在本地草稿里编辑，不向服务端写入（需要基线文档的用例在 finally 中还原）。
 */

const endpoint = "/api/v1/settings/stellar";
const NAV_GROUPS = [
  { label: "站点", pages: [["site", "站点信息"], ["seo", "SEO 与搜索"]] },
  { label: "外观与布局", pages: [["appearance", "主题外观"], ["navigation", "导航与页脚"], ["layout", "页面与侧栏"]] },
  { label: "内容", pages: [["post", "文章页"], ["listing", "列表与卡片"], ["community", "作者与友链"]] },
  { label: "互动", pages: [["comments", "评论"], ["feedback", "访客反馈"]] },
  { label: "扩展与高级", pages: [["extensions", "扩展与服务"], ["advanced", "代码与原文"]] },
];
// 页面分区与 settings-pages.ts 一一对应：锚点列出全部分区，页面内渲染的标题与之相同。
const PAGE_SECTIONS: Record<string, string[]> = {
  site: ["站点标识", "Logo", "浏览器与设备图标", "描述与地址"],
  seo: ["站内搜索", "页面地址与社交信息", "搜索引擎与站长验证"],
  appearance: ["外观预设", "颜色", "字体与排版", "形状与圆角", "背景"],
  navigation: ["顶栏", "左侧栏", "站点页脚"],
  layout: ["各类页面布局", "右侧栏", "侧栏组件定义"],
  post: ["阅读体验", "页脚与许可", "文章扩展", "AI 创作标记"],
  listing: ["置顶与文章卡片", "分类与标签样式"],
  community: ["作者", "友链更新", "友链页面"],
  comments: ["原生评论", "评论邮件通知"],
  feedback: ["Feedback 服务接入", "入口与截图"],
  extensions: ["标签插件", "前台增强", "预连接地址", "外部服务", "默认图片"],
  advanced: ["代码注入", "完整配置原文"],
};
const PAGE_LABELS = Object.fromEntries(NAV_GROUPS.flatMap((group) => group.pages));

const form = (page: Page) => page.locator(".canonical-settings-form:visible");
const struct = (page: Page, path: string) => page.locator(`[data-structured-path="${path}"]`);
const row = (page: Page, path: string) => page.locator(`[data-setting-path="${path}"]`);
const navDot = (page: Page, key: string) => settingsNav(page, key).getByRole("img", { name: "有未保存修改" });
// 分区标题 h2：按无障碍名称匹配（模板里的前后空白不影响），限定在分区标题区，不含页面标题。
const sectionHeading = (page: Page, key: string, title: string) =>
  page.locator(`#settings-page-${key} .settings-sec-title`).getByRole("heading", { level: 2, name: new RegExp(`^${title}`) });
const badge = (page: Page) => page.locator(".settings-dirty-badge");

async function put(page: Page, data: string) {
  const response = await page.request.put(endpoint, { data, headers: { "Content-Type": "application/json" } });
  expect(response.ok(), await response.text()).toBeTruthy();
}
// 外观预设是版式“行”（data-setting-path），其单选组直接渲染在行内，没有结构化节点包裹。
async function checkedPreset(page: Page): Promise<string> {
  const preset = row(page, "theme.appearance.preset");
  for (const name of ["卡片", "玻璃", "极简", "平面"])
    if (await preset.getByRole("radio", { name, exact: true }).isChecked()) return name;
  throw new Error("没有选中的外观预设");
}
async function changePreset(page: Page): Promise<string> {
  const current = await checkedPreset(page);
  const next = ["卡片", "玻璃", "极简", "平面"].find((name) => name !== current)!;
  await row(page, "theme.appearance.preset").getByRole("radio", { name: next, exact: true }).check();
  return next;
}

/**
 * 从页面标题开始按 Tab 遍历内容区，返回被 sticky 页头、保存栏或手机底部导航盖住的聚焦控件。
 * 命中测试取控件可见部分的中心：那里命中的元素若不是控件本身、也不与它互相包含，就是被盖住。
 */
async function coveredFocusStops(page: Page, key: string): Promise<{ stops: number; covered: string[] }> {
  await page.evaluate((k) => document.getElementById(`settings-page-title-${k}`)?.focus({ preventScroll: true }), key);
  let stops = 0;
  const covered: string[] = [];
  for (let i = 0; i < 200; i++) {
    await page.keyboard.press("Tab");
    const stop = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el || el === document.body || !el.closest(".settings-content")) return { end: true as const };
      const box = el.getBoundingClientRect();
      if (box.width === 0 || box.height === 0) return { skip: true as const };
      const left = Math.max(box.left, 0);
      const right = Math.min(box.right, window.innerWidth);
      const hit = document.elementFromPoint(left + (right - left) / 2, box.top + box.height / 2);
      const covered = Boolean(hit && hit !== el && !el.contains(hit) && !hit.contains(el));
      const label = (el.getAttribute("aria-label") || el.id || el.textContent || "").trim().slice(0, 24);
      return { label: `${el.tagName}:${label} ${Math.round(box.top)}-${Math.round(box.bottom)}`, covered };
    });
    if ("end" in stop) break;
    if ("skip" in stop) continue;
    stops += 1;
    if (stop.covered) covered.push(stop.label);
  }
  return { stops, covered };
}

test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
});

test("N1 桌面导航：5 组 12 页，顺序与归属一致，当前页标注，AI 入口跳转助手", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings");
  const nav = page.getByRole("navigation", { name: "设置分类" });
  await expect(nav).toBeVisible();

  const groups = await nav.locator(".settings-nav-group").evaluateAll((els) =>
    els.map((el) => ({
      label: el.querySelector(".settings-nav-heading")?.textContent?.trim(),
      pages: [...el.querySelectorAll(".settings-nav-item")].map((item) => [
        item.getAttribute("data-settings-page"),
        item.querySelector(".settings-nav-label")?.textContent?.trim(),
      ]),
    })),
  );
  expect(groups).toEqual(NAV_GROUPS);

  // 没有 ?tab= 时默认进入“站点信息”，同一时刻只有一个当前页。
  await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
  await expect(settingsNav(page, "site")).toHaveAttribute("aria-current", "page");
  await openSettingsPage(page, "comments");
  await expect(page).toHaveURL(/\/console\/settings\?tab=comments$/);
  await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
  await expect(settingsNav(page, "comments")).toHaveAttribute("aria-current", "page");

  const ai = nav.getByRole("link", { name: /AI 与知识库/ });
  await expect(ai).toContainText("在「助手」中设置");
  await ai.click();
  await expect(page).toHaveURL(/\/console\/assistant\?tab=settings$/);
});

test("N2/P1–P6 每页有标题、说明与本页分区；渲染的分区与归属一致，锚点滚动到对应分区", async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/console/settings");
  for (const [key, titles] of Object.entries(PAGE_SECTIONS)) {
    await openSettingsPage(page, key);
    const head = page.locator(`#settings-page-${key} .settings-page-head`);
    await expect(head.getByRole("heading", { level: 2 })).toHaveText(PAGE_LABELS[key]);
    await expect(head.locator("p").first()).not.toBeEmpty();
    await expect(head.locator(".settings-anchor")).toHaveText(titles);
    // 页面里实际渲染的分区标题：“完整配置原文”是折叠区，由代码与原文页单独渲染。
    const rendered = await page
      .locator(`#settings-page-${key} .settings-sec-title h2`)
      .evaluateAll((els) => els.map((el) => el.textContent?.trim()));
    expect(rendered, `${key} 页渲染的分区标题`).toEqual(titles.filter((title) => title !== "完整配置原文"));

    // 锚点：每个分区跳转后标题都露在 sticky 页头下面——不只是进了视口，也没有被页头盖住。
    for (const title of titles) {
      await head.locator(".settings-anchor").filter({ hasText: title }).click();
      const heading =
        title === "完整配置原文"
          ? page.locator("#settings-page-advanced .settings-raw-editor > summary")
          : sectionHeading(page, key, title);
      await expectClearOfHeader(heading, `${PAGE_LABELS[key]}「${title}」标题`);
    }
  }
  await expect(page.locator("#settings-sec-advanced-raw")).toBeVisible();

  // 中间的分区跳转后落在页面上部，而不是停在原处。
  await openSettingsPage(page, "appearance");
  const target = page.locator("#settings-sec-appearance-type");
  await page.locator("#settings-page-appearance .settings-anchor").filter({ hasText: "字体与排版" }).click();
  await expect
    .poll(() => target.evaluate((el) => Math.round(el.getBoundingClientRect().top)), { timeout: 8_000 })
    .toBeLessThan(220);
  expect(await target.evaluate((el) => el.getBoundingClientRect().top)).toBeGreaterThanOrEqual(0);
});

test("N6 换页后回到内容起点：上一页的滚动位置不带过来，新页标题不被 sticky 页头盖住", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/console/settings?tab=appearance");
  await expect(page.locator("#settings-page-appearance")).toBeVisible();
  await expect(page.locator("#settings-page-appearance .settings-sec").first()).toBeVisible();

  // 滚到页面底部再换页：新页从页首开始，标题在页头下方。
  await contentScroller(page).evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await expect.poll(() => scrollTopOf(page)).toBeGreaterThan(1000);
  await openSettingsPage(page, "navigation");
  await expect.poll(() => scrollTopOf(page)).toBe(0);
  await expectClearOfHeader(page.locator("#settings-page-title-navigation"), "导航与页脚页标题");

  // 只滚动了一点（标题刚好被页头压住）时同样回到页首，而不是认为标题“已在视口内”。
  await contentScroller(page).evaluate((el) => {
    el.scrollTop = 40;
  });
  await openSettingsPage(page, "layout");
  await expect.poll(() => scrollTopOf(page)).toBe(0);
  await expectClearOfHeader(page.locator("#settings-page-title-layout"), "页面与侧栏页标题");
});

test("N6 从滚动过的页面进入系统设置：设置页从页首开始，标题不被 sticky 页头盖住", async ({ page }) => {
  // Console 外壳切换路由时不重置内容区滚动位置，所以仪表盘滚动过的位置会被带进设置页。
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/console/dashboard");
    await page.waitForLoadState("networkidle");
    await contentScroller(page).evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    await expect.poll(() => scrollTopOf(page), { message: `${width}px 仪表盘本身要能滚动，否则用例没有意义` }).toBeGreaterThan(100);

    if (width >= 768) {
      await page.locator('aside a[href="/console/settings"]').first().click();
    } else {
      await page.getByRole("button", { name: "更多" }).click();
      await page.getByRole("dialog").locator('a[href="/console/settings"]').click();
    }
    await expect(page.locator(".settings-view")).toBeVisible();
    await expect.poll(() => scrollTopOf(page), { message: `${width}px 进入设置页后应在页首` }).toBe(0);
    // 桌面直接进入站点信息；手机先是分组列表（没有页面标题），量取列表第一项。
    await expectClearOfHeader(
      width >= 768 ? page.locator("#settings-page-title-site") : page.locator(".settings-nav.is-list .settings-nav-item").first(),
      `${width}px 进入设置页后的首个内容`,
    );
  }
});

test("N6 手机：列表与详情互相切换都从页首开始，页头换行时分区跳转也不被盖住", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 560 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/console/settings");
  const list = page.locator(".settings-nav.is-list");
  await expect(list).toBeVisible();

  // 短屏上分组列表本身会滚动：滚到底再点最后一项，详情从页首开始。
  await contentScroller(page).evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await expect.poll(() => scrollTopOf(page)).toBeGreaterThan(100);
  await list.locator('[data-settings-page="advanced"]').click();
  await expect(page.locator("#settings-page-advanced")).toBeVisible();
  await expect.poll(() => scrollTopOf(page)).toBe(0);
  await expectClearOfHeader(page.locator("#settings-page-title-advanced"), "代码与原文页标题");

  // 详情里滚到底再返回：列表同样从顶部开始。
  await contentScroller(page).evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await page.getByRole("button", { name: "返回设置", exact: true }).click();
  await expect(list).toBeVisible();
  await expect.poll(() => scrollTopOf(page)).toBe(0);

  // 主题外观页的页头带“前端预览”按钮，手机上会换成更高的两行；评论页没有，页头更矮。两种高度下分区标题都要露出来。
  for (const [key, title] of [
    ["appearance", "颜色"],
    ["comments", "评论邮件通知"],
  ]) {
    await list.locator(`[data-settings-page="${key}"]`).click();
    await expect(page.locator(`#settings-page-${key}`)).toBeVisible();
    await page.locator(`#settings-page-${key} .settings-anchor`).filter({ hasText: title }).click();
    await expectClearOfHeader(sectionHeading(page, key, title), `${key} 页「${title}」标题`);
    await page.getByRole("button", { name: "返回设置", exact: true }).click();
    await expect(list).toBeVisible();
  }
});

test("N7 键盘聚焦：按 Tab 遍历设置页，聚焦的控件不被 sticky 页头、保存栏或手机底部导航盖住", async ({ page }) => {
  test.setTimeout(150_000);
  // 主题页与独立保存的站点信息各取代表；桌面另验预览面板停靠时保存栏被抬高的情形。
  // 第三方评论配置已从设置页移除，评论页只剩原生评论与邮件通知（可聚焦控件不足以覆盖遍历断言），改用扩展页。
  const rounds = [
    { width: 1280, keys: ["appearance", "post", "site"] },
    { width: 390, keys: ["appearance", "extensions", "site"] },
  ];
  for (const { width, keys } of rounds) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/console/settings");
    for (const key of keys) {
      await openSettingsPage(page, key);
      const { stops, covered } = await coveredFocusStops(page, key);
      expect(stops, `${width}px ${key} 页应能遍历到足够多的控件`).toBeGreaterThan(15);
      expect(covered, `${width}px ${key} 页被盖住的聚焦控件`).toEqual([]);
    }
  }

  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/console/settings?tab=appearance");
  await expect(page.locator("#settings-page-appearance .settings-sec").first()).toBeVisible();
  await page.getByRole("button", { name: "前端预览", exact: true }).click();
  await expect(page.locator(".settings-view.preview-docked")).toBeVisible();
  const docked = await coveredFocusStops(page, "appearance");
  expect(docked.stops, "预览面板停靠时应能遍历到足够多的控件").toBeGreaterThan(15);
  expect(docked.covered, "预览面板停靠时被盖住的聚焦控件").toEqual([]);
});

test("N3/N5 手机分组列表：未保存点带无障碍名称，返回设置与列表项点击区足够大，输入保留", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/console/settings?tab=site");
  const title = page.locator("#settings-site-title");
  await expect(title).toBeVisible();
  const original = await title.inputValue();
  await title.fill(`${original}-ia`);
  await expect(badge(page)).toBeVisible();

  const back = page.getByRole("button", { name: "返回设置", exact: true });
  const backBox = (await back.boundingBox())!;
  expect(backBox.height).toBeGreaterThanOrEqual(44);
  await back.click();
  const list = page.locator(".settings-nav.is-list");
  await expect(list).toBeVisible();
  await expect(list.locator('[data-settings-page="site"]').getByRole("img", { name: "有未保存修改" })).toHaveCount(1);
  await expect(list.getByRole("img", { name: "有未保存修改" })).toHaveCount(1);

  await list.locator('[data-settings-page="site"]').click();
  await expect(title).toHaveValue(`${original}-ia`);
});

test("Q2b 手机点击区：分组列表与 12 页里每个可点击控件的点击区都 ≥44px，扩大区不抢相邻控件的点击", async ({ page }) => {
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 390, height: 844 });
  const problems: string[] = [];
  let controls = 0;
  let boxUnder44 = 0;
  let reached44 = 0;
  for (const key of ["", ...Object.keys(PAGE_SECTIONS)]) {
    await page.goto(key ? `/console/settings?tab=${key}` : "/console/settings");
    await expect(key ? page.locator(`#settings-page-${key}`) : page.locator(".settings-nav.is-list")).toBeVisible();
    await page.waitForLoadState("networkidle");
    // 盒子尺寸断言看不到透明伪元素补出来的点击区，所以逐控件做命中测试（见 support/settings.ts）。
    const report = await measureTouchTargets(page, ".settings-view");
    controls += report.controls;
    boxUnder44 += report.boxUnder44;
    reached44 += report.reached44;
    const where = key || "分组列表";
    problems.push(
      ...report.uncovered.map((line) => `${where} 点击区不足（扩大区没补上或被盖住）：${line}`),
      ...report.blocked.map((line) => `${where} 点击区不足（被相邻控件挡住，调整 settings-form.css 的“手机点击区”）：${line}`),
      ...report.stolen.map((line) => `${where} 抢了别的控件的点击：${line}`),
    );
  }
  expect(problems).toEqual([]);
  // 防止空转：12 页加分组列表应有上千个控件，其中数百个自身盒子不足 44px，全部由扩大区补足。
  expect(controls).toBeGreaterThan(1500);
  expect(boxUnder44).toBeGreaterThan(500);
  expect(reached44).toBe(boxUnder44);
});

test("Q2b 手机点击区：点在开关、恢复默认、折叠摘要的扩大区里，与点控件本身效果一致", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const center = (el: Element) => el.scrollIntoView({ block: "center", behavior: "instant" as ScrollBehavior });

  // 开关（31px 高）：上下各补 7px，点在控件外 5px 处。
  await page.goto("/console/settings?tab=seo");
  const toggle = page.getByRole("switch", { name: "禁止搜索引擎收录", exact: true });
  await expect(toggle).toBeVisible();
  await toggle.evaluate(center);
  const initial = (await toggle.getAttribute("aria-checked"))!;
  const toggleBox = (await toggle.boundingBox())!;
  await page.mouse.click(toggleBox.x + toggleBox.width / 2, toggleBox.y - 5);
  await expect(toggle).not.toHaveAttribute("aria-checked", initial);
  await page.mouse.click(toggleBox.x + toggleBox.width / 2, toggleBox.y + toggleBox.height + 5);
  await expect(toggle).toHaveAttribute("aria-checked", initial);

  // 标题行里的“恢复默认”（18px 高）：向上补 17px，点在按钮上方 10px 处，字段回到“继承默认”。
  // Algolia 段已随第三方搜索移除，改用扩展页「外部服务 → Markdown 渲染」的“脚本地址”。
  await page.goto("/console/settings?tab=extensions");
  const markdown = page.locator('[data-structured-path="theme.services.markdown"]');
  const reset = markdown.getByRole("button", { name: "脚本地址恢复默认", exact: true }).first();
  await reset.evaluate((el) => {
    for (let d = el.closest("details"); d; d = d.parentElement?.closest("details") ?? null) d.open = true;
  });
  const field = reset.locator("xpath=ancestor::div[@data-structured-path='theme.services.markdown.js'][1]");
  await field.locator("input.apple-input").first().fill("https://example.com/search.js");
  await expect(field.locator(".structured-value-state").first()).toHaveText("已自定义");
  await reset.evaluate(center);
  const resetBox = (await reset.boundingBox())!;
  await page.mouse.click(resetBox.x + resetBox.width / 2, resetBox.y - 10);
  await expect(field.locator(".structured-value-state").first()).toHaveText("继承默认");

  // 折叠摘要：上下各补 12px，点在摘要外 8px 处，展开与收起都生效。分组默认展开，先把第一个收起来再点。
  await page.goto("/console/settings?tab=appearance");
  const first = page.locator("#settings-page-appearance details.structured-item > summary").first();
  await expect(first).toBeVisible();
  // 点击会改变展开状态，定位器随之漂移：先取成元素句柄再点。
  const summary = (await first.elementHandle())!;
  const isOpen = () => summary.evaluate((el) => (el.parentElement as HTMLDetailsElement).open);
  expect(await isOpen(), "分组默认展开").toBe(true);
  await summary.evaluate((el) => {
    (el.parentElement as HTMLDetailsElement).open = false;
  });
  await summary.evaluate(center);
  const summaryBox = (await summary.boundingBox())!;
  await page.mouse.click(summaryBox.x + 40, summaryBox.y - 8);
  await expect.poll(isOpen).toBe(true);
  await page.mouse.click(summaryBox.x + 40, summaryBox.y + summaryBox.height + 8);
  await expect.poll(isOpen).toBe(false);
});

test("N4 旧链接：旧页签与分段键落到新页面或原跳转目标，未知值回到站点信息", async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1280, height: 900 });
  const legacy: Array<[string, string]> = [
    ["basic", "site"],
    ["search", "seo"],
    ["plugins", "extensions"],
    ["services", "extensions"],
    ["article", "post"],
    ["inject", "advanced"],
    ["data_cache", "advanced"],
    ["theme_yaml", "advanced"],
    ["cover", "layout"],
    ["nav_tabs", "navigation"],
    ["widgets", "layout"],
    ["tag_plugins", "extensions"],
    ["default", "extensions"],
    ["menubar", "navigation"],
    ["rightbar", "layout"],
    ["footer", "navigation"],
    ["authors", "community"],
    ["stellar2", "advanced"],
    ["no-such-tab", "site"],
  ];
  for (const [from, to] of legacy) {
    await page.goto(`/console/settings?tab=${from}`);
    await expect(page, `?tab=${from}`).toHaveURL(new RegExp(`/console/settings\\?tab=${to}$`));
    await expect(settingsNav(page, to), `?tab=${from}`).toHaveAttribute("aria-current", "page");
    await expect(page.locator(`#settings-page-${to}`)).toBeVisible();
  }
  // stellar2 是完整配置原文：落到代码与原文并直接展开原文。
  await page.goto("/console/settings?tab=stellar2");
  await expect(page.locator(".settings-raw-editor")).toHaveJSProperty("open", true);
  await expect(page.locator("#stellar2-document")).toBeVisible();

  // 不在设置页内的旧入口保持原跳转目标。
  await page.goto("/console/settings?tab=ai");
  await expect(page).toHaveURL(/\/console\/assistant\?tab=settings$/);
  await page.goto("/console/settings?tab=security");
  await expect(page).toHaveURL(/\/console\/profile$/);
});

test("S1/S3/N5 主题保存栏计数按页统计；放弃本页修改只还原本页，其他页草稿保留，且不写入服务端", async ({ page }) => {
  test.setTimeout(90_000);
  const original = await (await page.request.get(endpoint)).text();
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings?tab=appearance");
  const bar = () => form(page).locator(".settings-save-bar");
  const save = () => bar().getByRole("button", { name: "保存全部主题设置", exact: true });
  const discard = () => bar().getByRole("button", { name: "放弃本页修改", exact: true });

  await expect(bar()).toContainText("没有未保存修改");
  await expect(save()).toBeDisabled();
  await expect(discard()).toBeDisabled();
  // 保存栏固定在可视区底部，长页面滚动时不消失。
  expect(await bar().evaluate((el) => getComputedStyle(el).position)).toBe("sticky");

  const before = await checkedPreset(page);
  const changed = await changePreset(page);
  await expect(bar()).toContainText("1 处未保存修改");
  await expect(bar()).toContainText(/主题外观 1.*草稿在各页共享/);
  await expect(save()).toBeEnabled();
  await expect(navDot(page, "appearance")).toHaveCount(1);
  await expect(badge(page)).toBeVisible();

  await openSettingsPage(page, "post");
  const reward = row(page, "blog_extensions.content.reward.text").locator("input");
  const rewardBefore = await reward.inputValue();
  await reward.fill(`${rewardBefore}-ia`);
  await expect(bar()).toContainText("2 处未保存修改");
  await expect(bar()).toContainText(/主题外观 1.*文章页 1.*草稿在各页共享/);
  await expect(navDot(page, "post")).toHaveCount(1);

  // 放弃“文章页”：只还原本页；主题外观的修改仍在，计数回到 1。
  await discard().click();
  await expect(reward).toHaveValue(rewardBefore);
  await expect(navDot(page, "post")).toHaveCount(0);
  await expect(navDot(page, "appearance")).toHaveCount(1);
  await expect(bar()).toContainText("1 处未保存修改");
  await openSettingsPage(page, "appearance");
  expect(await checkedPreset(page)).toBe(changed);

  // 放弃“主题外观”：回到进入前的预设，页头与导航的未保存提示全部消失。
  await discard().click();
  expect(await checkedPreset(page)).toBe(before);
  await expect(navDot(page, "appearance")).toHaveCount(0);
  await expect(badge(page)).toHaveCount(0);
  await expect(bar()).toContainText("没有未保存修改");
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("S2/S3 站点信息独立保存：只保存本页并提示主题草稿，标题为空时拒绝保存并聚焦，放弃本页回到加载值", async ({ page }) => {
  test.setTimeout(60_000);
  const original = await (await page.request.get("/api/v1/settings")).json();
  const originalTitle = (original as { site: { title: string } }).site.title;
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings?tab=site");
  const bar = page.locator("#settings-page-site .settings-save-bar");
  const save = bar.getByRole("button", { name: "保存", exact: true });
  const title = page.locator("#settings-site-title");
  await expect(title).toHaveValue(originalTitle);
  await expect(bar).toContainText("没有未保存修改");
  await expect(save).toBeDisabled();

  await title.fill("");
  await expect(bar).toContainText("1 处未保存修改");
  await expect(bar).toContainText("仅保存本页");
  await save.click();
  // 标题带 required：保存按钮是表单内的提交按钮，浏览器原生校验先拦截提交并聚焦标题；
  // 页面里的 Toast 只是不执行原生校验时的兜底，所以这里断言校验状态和焦点，不断言 Toast。
  await expect(title).toBeFocused();
  expect(await title.evaluate((el) => (el as HTMLInputElement).validity.valueMissing)).toBe(true);
  // 请求没有发出：服务端标题不变。
  expect(((await (await page.request.get("/api/v1/settings")).json()) as { site: { title: string } }).site.title).toBe(originalTitle);

  // 主题页另有未保存草稿时，站点信息的保存栏说明保存范围。
  await openSettingsPage(page, "appearance");
  await changePreset(page);
  await openSettingsPage(page, "site");
  await expect(bar).toContainText("仅保存本页；主题草稿另有 1 处未保存");
  await expect(navDot(page, "site")).toHaveCount(1);
  await expect(navDot(page, "appearance")).toHaveCount(1);

  // 放弃本页：站点标题回到加载值；主题草稿不受影响。
  await bar.getByRole("button", { name: "放弃本页修改", exact: true }).click();
  await expect(title).toHaveValue(originalTitle);
  await expect(navDot(page, "site")).toHaveCount(0);
  await expect(navDot(page, "appearance")).toHaveCount(1);
  await expect(bar).toContainText("没有未保存修改");
});

test("R2 行内状态：橙点只表示相对上次保存有改动；恢复默认移除显式覆盖，放弃本页回到已保存值", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{},"blog_extensions":{"native_comments":{"moderation":true}}}');
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/settings?tab=comments");
    const moderation = row(page, "blog_extensions.native_comments.moderation");
    const toggle = moderation.getByRole("switch");
    const dot = moderation.getByRole("img", { name: "有未保存修改" });
    const reset = moderation.getByRole("button", { name: "评论需要审核恢复默认", exact: true });

    // 已保存的显式覆盖：没有橙点，但有“恢复默认”。
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await expect(dot).toHaveCount(0);
    await expect(reset).toBeVisible();

    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await expect(dot).toHaveCount(1);
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await expect(dot).toHaveCount(0);

    // 恢复默认 = 移除覆盖：显示默认值，相对已保存值有改动所以出现橙点，覆盖不再存在所以按钮消失。
    await reset.click();
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await expect(dot).toHaveCount(1);
    await expect(reset).toHaveCount(0);

    await form(page).getByRole("button", { name: "放弃本页修改", exact: true }).click();
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await expect(dot).toHaveCount(0);
    await expect(reset).toBeVisible();
    expect(await (await page.request.get(endpoint)).text()).toContain('"moderation"');
  } finally {
    await put(page, original);
  }
});

test("Q2 对比度：导航、页面说明、行标签与保存栏文字在深色界面下 ≥4.5:1", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings?tab=appearance");
  await expect(form(page)).toBeVisible();
  // 制造一处修改，使保存栏和未保存徽章处于“有修改”状态。
  await changePreset(page);
  await expect(badge(page)).toBeVisible();

  const targets: Array<[string, string]> = [
    ["导航分组标题", ".settings-nav-heading:visible"],
    ["导航项", ".settings-nav-item:not(.is-active):visible .settings-nav-label"],
    ["当前导航项", ".settings-nav-item.is-active:visible .settings-nav-label"],
    ["AI 入口", ".settings-nav-link:visible"],
    ["页面说明", ".settings-page-head:visible > p"],
    ["分区说明", ".settings-sec-title:visible > p"],
    ["分组名称", ".structured-summary-name:visible"],
    ["分组说明", ".structured-summary-help:visible"],
    ["行标签", ".settings-row-title:visible label"],
    ["保存栏标题", ".settings-save-title:visible"],
    ["保存栏说明", ".settings-save-detail:visible"],
    ["未保存徽章", ".settings-dirty-badge"],
  ];
  const failures: string[] = [];
  for (const [name, selector] of targets) {
    const handle = page.locator(selector).first();
    if ((await handle.count()) === 0) {
      failures.push(`${name}：没有找到 ${selector}`);
      continue;
    }
    const ratio = await handle.evaluate((el) => {
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 1;
      const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
      const rgba = (value: string) => {
        ctx.clearRect(0, 0, 1, 1);
        ctx.fillStyle = value;
        ctx.fillRect(0, 0, 1, 1);
        const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
        return { r, g, b, a: a / 255 };
      };
      const mix = (top: { r: number; g: number; b: number; a: number }, base: { r: number; g: number; b: number }) => ({
        r: top.r * top.a + base.r * (1 - top.a),
        g: top.g * top.a + base.g * (1 - top.a),
        b: top.b * top.a + base.b * (1 - top.a),
      });
      // 背景：自元素向上收集半透明/不透明层，再从底层叠回来。
      const layers: Array<{ r: number; g: number; b: number; a: number }> = [];
      for (let node: Element | null = el; node; node = node.parentElement) {
        const layer = rgba(getComputedStyle(node).backgroundColor);
        if (layer.a > 0) layers.push(layer);
        if (layer.a === 1) break;
      }
      let base = { r: 255, g: 255, b: 255 };
      for (const layer of layers.reverse()) base = mix(layer, base);
      const text = mix(rgba(getComputedStyle(el).color), base);
      const lum = ({ r, g, b }: { r: number; g: number; b: number }) => {
        const [R, G, B] = [r, g, b].map((v) => {
          const c = v / 255;
          return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
        });
        return 0.2126 * R + 0.7152 * G + 0.0722 * B;
      };
      const [hi, lo] = [lum(text), lum(base)].sort((a, b) => b - a);
      return (hi + 0.05) / (lo + 0.05);
    });
    if (ratio < 4.5) failures.push(`${name}（${selector}）对比度 ${ratio.toFixed(2)} < 4.5`);
  }
  expect(failures, failures.join("\n")).toEqual([]);
});

// 用户体验后的三处反馈：去掉页面标题下的“效果说明条”、导航项之间留空隙、结构化分组全部展开并整理标题行。

/** 一个结构化分组（details）的标题行几何：箭头、名称、状态、恢复默认与说明的位置。 */
async function groupGeometry(page: Page, path: string) {
  return struct(page, path)
    .locator("xpath=ancestor::details[1]")
    .evaluate((details) => {
      const summary = details.querySelector(":scope > summary") as HTMLElement;
      const box = (el: Element | null) => {
        if (!el) return null;
        const r = el.getBoundingClientRect();
        return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, mid: (r.top + r.bottom) / 2 };
      };
      const name = summary.querySelector(".structured-summary-name") as HTMLElement;
      const label = document.querySelector(".structured-heading label") as HTMLElement | null;
      return {
        summary: box(summary)!,
        chevron: box(summary.querySelector(".structured-chevron"))!,
        name: box(name)!,
        state: box(summary.querySelector(".structured-value-state"))!,
        reset: box(summary.querySelector(".settings-reset"))!,
        help: box(summary.querySelector(".structured-summary-help")),
        nameFont: { size: parseFloat(getComputedStyle(name).fontSize), weight: Number(getComputedStyle(name).fontWeight) },
        labelFont: label
          ? { size: parseFloat(getComputedStyle(label).fontSize), weight: Number(getComputedStyle(label).fontWeight) }
          : null,
      };
    });
}

test("U1 页面标题下只有一句话说明：12 页都没有效果说明条，看效果的入口是页头的“前端预览”", async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings?tab=site");
  for (const key of Object.keys(PAGE_SECTIONS)) {
    await openSettingsPage(page, key);
    const head = page.locator(`#settings-page-${key} .settings-page-head`);
    await expect(head.getByRole("heading", { level: 2, name: PAGE_LABELS[key], exact: true })).toBeVisible();
    await expect(head.locator(":scope > p")).toHaveCount(1);
    await expect(page.locator(".settings-effect-guide"), `${PAGE_LABELS[key]}不应有效果说明条`).toHaveCount(0);
    await expect(page.getByText("影响位置与检查方法")).toHaveCount(0);
    await expect(page.getByText("查看草稿效果")).toHaveCount(0);
  }
  // 预览入口在页头：有固定预览页面的设置页点“前端预览”打开预览面板，再点一次收起。
  await openSettingsPage(page, "appearance");
  const preview = page.getByRole("button", { name: "前端预览", exact: true });
  await preview.click();
  await expect(page.locator(".preview-panel")).toBeVisible();
  await preview.click();
  await expect(page.locator(".preview-panel")).toHaveCount(0);
});

test("U2 分组默认全部展开：收起只是临时操作，不产生未保存修改，也不会被之后的编辑重新展开；只有完整配置原文默认折叠", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/console/settings?tab=site");
  let grouped = 0;
  for (const key of Object.keys(PAGE_SECTIONS)) {
    await openSettingsPage(page, key);
    grouped += await page.locator(`#settings-page-${key} details.structured-item`).count();
    await expect(
      page.locator(`#settings-page-${key} details.structured-item:not([open])`),
      `${PAGE_LABELS[key]}里不应有收起的分组`,
    ).toHaveCount(0);
  }
  // 防止空转：外观、导航、布局、扩展里一共有一百多个分组。
  expect(grouped).toBeGreaterThan(100);
  await openSettingsPage(page, "advanced");
  await expect(page.locator(".settings-raw-editor"), "完整配置原文有意默认折叠").not.toHaveAttribute("open", "");

  await openSettingsPage(page, "navigation");
  const brand = struct(page, "theme.topbar.brand").locator("xpath=ancestor::details[1]");
  const brandName = brand.locator(":scope > summary .structured-summary-name");
  await expect(brand).toHaveAttribute("open", "");
  await brandName.click();
  await expect(brand).not.toHaveAttribute("open", "");
  await expect(badge(page), "收起不是对设置的修改").toHaveCount(0);
  await expect(navDot(page, "navigation")).toHaveCount(0);

  // 编辑别处触发重新渲染，收起的分组仍然收起。
  await struct(page, "theme.topbar.enabled").getByRole("switch").click();
  await expect(badge(page)).toBeVisible();
  await expect(brand).not.toHaveAttribute("open", "");
  // 放弃本页修改会按已保存的值重建这一页的表单，分组随之回到默认展开。
  await form(page).getByRole("button", { name: "放弃本页修改", exact: true }).click();
  await expect(badge(page)).toHaveCount(0);
  await expect(brand).toHaveAttribute("open", "");
});

test("U3 导航项之间留有空隙，选中项的底色不贴着目录的滚动条，AI 入口一行放得下", async ({ page }) => {
  // 视口矮一些，目录高过视口出现滚动条：选中项最容易贴上去的情形。
  await page.setViewportSize({ width: 1280, height: 520 });
  await page.goto("/console/settings?tab=navigation");
  const nav = page.getByRole("navigation", { name: "设置分类" });
  await expect(nav).toBeVisible();
  const gaps = await nav.locator(".settings-nav-list").evaluateAll((lists) =>
    lists.flatMap((list) => {
      const boxes = [...list.querySelectorAll(".settings-nav-item")].map((item) => item.getBoundingClientRect());
      return boxes.slice(1).map((box, i) => box.top - boxes[i].bottom);
    }),
  );
  expect(gaps, "5 组共 12 项，每组内相邻两项之间").toHaveLength(7);
  for (const gap of gaps) expect(gap, "相邻导航项的间隔").toBeGreaterThanOrEqual(4);

  const sidebar = await page.locator(".settings-sidebar").evaluate((el) => {
    const active = el.querySelector(".settings-nav-item.is-active")!.getBoundingClientRect();
    return { scrolls: el.scrollHeight > el.clientHeight, clearRight: el.getBoundingClientRect().left + el.clientWidth - active.right };
  });
  expect(sidebar.scrolls, "矮视口下目录应出现滚动").toBe(true);
  expect(sidebar.clearRight, "选中项右缘到滚动条之间的空隙").toBeGreaterThanOrEqual(8);

  const lines = await nav.locator(".settings-nav-link > span").evaluate((el) => {
    const range = document.createRange();
    range.selectNodeContents(el);
    return new Set([...range.getClientRects()].map((rect) => Math.round(rect.top))).size;
  });
  expect(lines, "“AI 与知识库”入口不换行").toBe(1);
});

test("U4 分组标题行：箭头、名称与状态、恢复默认在同一行，说明与名称左对齐；嵌套的分组逐层退进", async ({ page }) => {
  for (const [width, height] of [
    [1280, 900],
    [390, 844],
  ] as const) {
    await page.setViewportSize({ width, height });
    await page.goto("/console/settings?tab=navigation");
    await expect(struct(page, "theme.topbar.brand")).toBeVisible();
    const outer = await groupGeometry(page, "theme.topbar.brand");
    const inner = await groupGeometry(page, "theme.topbar.brand.image");
    const where = `${width}px`;

    for (const group of [outer, inner]) {
      expect(group.chevron.right, `${where} 箭头在名称前`).toBeLessThanOrEqual(group.name.left);
      expect(Math.abs(group.chevron.mid - group.name.mid), `${where} 箭头与名称同一行`).toBeLessThanOrEqual(3);
      expect(group.state.left, `${where} 状态紧跟名称`).toBeGreaterThanOrEqual(group.name.right);
      expect(Math.abs(group.state.mid - group.name.mid), `${where} 状态与名称同一行`).toBeLessThanOrEqual(4);
      expect(Math.abs(group.reset.right - group.summary.right), `${where} 恢复默认靠右`).toBeLessThanOrEqual(1);
      expect(Math.abs(group.reset.mid - group.name.mid), `${where} 恢复默认与名称同一行`).toBeLessThanOrEqual(4);
      expect(group.help, `${where} 这两个分组都有说明`).not.toBeNull();
      expect(Math.abs(group.help!.left - group.name.left), `${where} 说明与名称左对齐`).toBeLessThanOrEqual(0.5);
      expect(group.help!.top, `${where} 说明在名称下面`).toBeGreaterThanOrEqual(group.name.bottom - 1);
      // 分组名称比下面各字段的标签更醒目，层级才读得出来。
      expect(group.labelFont, `${where} 页面上有字段标签可比`).not.toBeNull();
      expect(group.nameFont.size).toBeGreaterThanOrEqual(group.labelFont!.size);
      expect(group.nameFont.weight).toBeGreaterThan(group.labelFont!.weight);
    }

    const indent = inner.summary.left - outer.summary.left;
    if (width >= 768) expect(indent, "宽屏每层缩进").toBeGreaterThanOrEqual(16);
    else {
      // 手机宽度金贵：只退进一点，让嵌套关系看得出来。
      expect(indent, "手机每层缩进").toBeGreaterThanOrEqual(4);
      expect(indent).toBeLessThanOrEqual(16);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${where} 横向溢出`).toBeLessThanOrEqual(1);
  }
});
