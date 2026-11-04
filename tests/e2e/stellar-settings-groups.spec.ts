import { expect, test, type Page } from "@playwright/test";
import { openSettingsPage } from "./support/settings";

const endpoint = "/api/v1/settings/stellar";
const form = (page: Page) => page.locator(".canonical-settings-form:visible");
const save = (page: Page) => form(page).getByRole("button", { name: "保存全部主题设置", exact: true });
// 外观预设与配色模式是版式“行”（data-setting-path）；其余项仍是结构化节点（data-structured-path）。
const ROW_KEYS = new Set(["theme.appearance.preset", "theme.appearance.color_scheme"]);
async function node(page: Page, key: string) {
  const target = form(page).locator(`[${ROW_KEYS.has(key) ? "data-setting-path" : "data-structured-path"}="${key}"]`);
  await target.evaluate((el) => {
    for (let parent = el.parentElement; parent; parent = parent.parentElement)
      if (parent instanceof HTMLDetailsElement) parent.open = true;
  });
  return target;
}
async function text(page: Page, key: string) { return (await node(page, key)).locator(":scope > input:not([type=number]), :scope > textarea"); }
async function select(page: Page, key: string, label: string) {
  const target = await node(page, key);
  if (await target.getByRole("radiogroup").count()) await target.getByRole("radio", { name: label, exact: true }).check();
  else await target.locator(":scope > select").first().selectOption({ label });
}
async function put(page: Page, raw: string) {
  const response = await page.request.put(endpoint, { data: raw, headers: { "Content-Type": "application/json" } });
  expect(response.ok(), await response.text()).toBeTruthy();
}
async function persist(page: Page) {
  const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
  await save(page).click();
  const result = await response;
  expect(result.ok(), await result.text()).toBeTruthy();
  await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
}
test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
});

test("各页共用原文草稿，保存重读后外观与页脚在前台生效", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    const initial = '{"version":2,"theme":{},"blog_extensions":{"unrelated":{"large":9007199254740993,"nil":null,"flag":false},"ai":{"api_key":"fixture-secret"}}}';
    await put(page, initial);
    await page.goto("/console/settings?tab=appearance");
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
    // 仅逐页打开全部设置页不会把显示默认值写入服务器。
    for (const key of ["navigation", "layout", "post", "listing", "community", "comments", "feedback", "seo", "extensions", "advanced", "site", "appearance"])
      await openSettingsPage(page, key);
    expect(JSON.parse(await (await page.request.get(endpoint)).text()).theme).toEqual({});
    for (const preset of ["卡片", "玻璃", "极简", "平面"]) {
      await select(page, "theme.appearance.preset", preset);
      await expect((await node(page, "theme.appearance.preset")).getByRole("radio", { name: preset, exact: true })).toBeChecked();
    }
    await select(page, "theme.appearance.color_scheme", "浅色");
    await openSettingsPage(page, "navigation");
    await (await text(page, "theme.footer.content")).fill("分组设置验收页脚");
    await openSettingsPage(page, "extensions");
    await (await text(page, "theme.services.github.api_url")).fill("https://api.github.com");
    await persist(page);
    const stored = await (await page.request.get(endpoint)).text();
    expect(stored).toContain("9007199254740993");
    expect(JSON.parse(stored).blog_extensions.ai.api_key).toBe("****");
    expect(JSON.parse(stored).theme.appearance).toEqual({ preset: "flat", color_scheme: "light" });
    await page.reload();
    await openSettingsPage(page, "appearance");
    await expect((await node(page, "theme.appearance.preset")).getByRole("radio", { name: "平面", exact: true })).toBeChecked();
    const html = await (await page.request.get("/")).text();
    expect(html).toContain("分组设置验收页脚");
    expect(html).toMatch(/data-theme="light"/);
    expect(html).toMatch(/flat/);
  } finally { await put(page, original); }
});

test("组件列表增删排序保留开放参数的大整数与空值，字典支持原型同名键", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{"leftbar":{"widgets":[{"layout":"markdown","content":"First","large":9007199254740993,"large_values":[9007199254740992,9007199254740993],"optional":null,"flag":false},{"layout":"markdown","content":"Second","large":9007199254740995}]}},"blog_extensions":{}}');
    await page.goto("/console/settings?tab=navigation");
    const list = await node(page, "theme.leftbar.widgets");
    await list.getByRole("button", { name: "侧栏组件第 2 项上移", exact: true }).click();
    await (await text(page, "theme.leftbar.widgets.0.content")).fill("Second edited");
    const item = await node(page, "theme.leftbar.widgets.0");
    await item.getByRole("textbox", { name: "新增侧栏组件第 1 项名称", exact: true }).fill("__proto__");
    await expect(save(page)).toBeDisabled();
    await item.getByRole("button", { name: "添加项目", exact: true }).last().click();
    await select(page, "theme.leftbar.widgets.0.__proto__", "数字");
    const special = await node(page, "theme.leftbar.widgets.0.__proto__");
    await special.locator("input[type=number]").fill("9007199254740994");
    await expect(save(page)).toBeDisabled();
    await expect(list.getByRole("button", { name: "移除侧栏组件第 1 项", exact: true })).toBeDisabled();
    await special.getByRole("button", { name: "__proto__恢复默认", exact: true }).click();
    await expect(save(page)).toBeEnabled();
    await persist(page);
    const raw = await (await page.request.get(endpoint)).text();
    expect(raw).toMatch(/"large"\s*:\s*9007199254740993/);
    expect(raw).toMatch(/"large"\s*:\s*9007199254740995/);
    expect(JSON.parse(raw).theme.leftbar.widgets.map((item: any) => item.content)).toEqual(["Second edited", "First"]);
    expect(JSON.parse(raw).theme.leftbar.widgets[1]).toMatchObject({ optional: null, flag: false });
    const uncertain = await node(page, "theme.leftbar.widgets.1.large_values");
    await uncertain.getByRole("button", { name: "large_values第 2 项上移", exact: true }).click();
    await expect(page.locator("#settings-page-navigation")).toContainText("此项未修改");
    expect(await (await page.request.get(endpoint)).text()).toBe(raw);
    await list.getByRole("button", { name: "移除侧栏组件第 2 项", exact: true }).click();
    await list.getByRole("button", { name: "添加侧栏组件", exact: true }).click();
    // 新联合类型列表项必须选择合法初值，不能沿用 schema 的占位 null。
    await persist(page);
    expect(JSON.parse(await (await page.request.get(endpoint)).text()).theme.leftbar.widgets[1]).not.toBeNull();
  } finally { await put(page, original); }
});

test("继承、空列表、关闭和未完成子表单相互区分，恢复整个分组清理输入状态", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{"profiles":{"home":{"leftbar":{"widgets":null},"rightbar":{"widgets":[]}}},"topbar":{"brand":false}},"blog_extensions":{}}');
    await page.goto("/console/settings?tab=layout");
    await expect((await node(page, "theme.profiles.home.leftbar.widgets")).locator(":scope > select")).toHaveValue("null");
    await expect((await node(page, "theme.profiles.home.rightbar.widgets")).locator(":scope > select")).toHaveValue("array");
    await select(page, "theme.profiles.home.leftbar.enabled", "开关");
    await persist(page);
    const stored = JSON.parse(await (await page.request.get(endpoint)).text());
    expect(stored.theme.profiles.home.leftbar).toEqual({ widgets: null, enabled: false });
    expect(stored.theme.profiles.home.rightbar.widgets).toEqual([]);
    expect(stored.theme.topbar.brand).toBe(false);
    await openSettingsPage(page, "appearance");
    // 主题外观没有任何显式覆盖时不提供页面级“恢复默认”。
    await expect(page.getByRole("button", { name: "恢复主题外观默认", exact: true })).toHaveCount(0);
    await (await node(page, "theme.appearance.backgrounds.leftbar.opacity")).locator("input").fill("2");
    await expect(save(page)).toBeDisabled();
    await expect(page.locator(".settings-save-bar").getByRole("alert")).toBeVisible();
    await openSettingsPage(page, "comments");
    await expect(save(page)).toBeDisabled();
    await openSettingsPage(page, "appearance");
    // 页面级恢复默认移除本页全部显式覆盖，同时清掉未同步的输入状态。
    await page.locator('[data-setting-path="theme.appearance"]').getByRole("button", { name: "恢复主题外观默认", exact: true }).click();
    await expect(page.locator(".settings-save-bar").getByRole("alert")).toHaveCount(0);
    await expect(form(page).getByRole("alert")).toHaveCount(0);
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
    // 恢复后草稿与已保存配置一致；再做一处真实修改并保存，确认没有残留空的 appearance 对象。
    await openSettingsPage(page, "comments");
    const moderation = form(page).locator('[data-setting-path="blog_extensions.native_comments.moderation"]').getByRole("switch");
    await moderation.click();
    await persist(page);
    expect(JSON.parse(await (await page.request.get(endpoint)).text()).theme).not.toHaveProperty("appearance");
    await page.goto("/console/settings?tab=stellar2");
    await expect(page.locator(`.settings-nav:visible [data-settings-page="advanced"]`)).toHaveAttribute("aria-current", "page");
    await expect(page.locator("#stellar2-document")).toBeVisible();
  } finally { await put(page, original); }
});

test("新增设置在移动与桌面宽度保持可达、无卡片外框及当前构建产物", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  await page.goto("/console/settings?tab=appearance");
  for (const width of [390, 640, 641, 767, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const key of ["appearance", "navigation", "layout", "extensions", "community"]) {
      await openSettingsPage(page, key);
      await expect(form(page)).toHaveAttribute("data-settings-tab", key);
      await expect(save(page)).toBeVisible();
      // 横向页边距由 .settings-container 提供，分区自身横向 padding 为 0。
      const padding = await form(page).locator(".settings-sec").first().evaluate(el => getComputedStyle(el).paddingLeft);
      expect(padding).toBe("0px");
      const containerPadding = await page.locator(".settings-container").evaluate(el => getComputedStyle(el).paddingLeft);
      expect(containerPadding).toBe(width < 768 ? "16px" : "32px");
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
    }
    if (width === 390 || width === 1280) {
      for (const key of ["appearance", "navigation", "layout"]) {
        await openSettingsPage(page, key);
        // 截图写入 Playwright 输出目录，不覆写受跟踪的 docs/baselines。
        await page.screenshot({ path: testInfo.outputPath(`after-${key}-${width}.png`) });
      }
    }
  }
  const scripts = await page.locator("script[src]").evaluateAll(nodes => nodes.map(node => (node as HTMLScriptElement).src));
  expect(scripts.some(src => /\/console\/assets\/.*\.js/.test(src))).toBeTruthy();
});
