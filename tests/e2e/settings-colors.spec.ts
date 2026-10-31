import { test, expect, type Page } from "@playwright/test";
import path from "node:path";

const endpoint = "/api/v1/settings/stellar";
const node = (page: Page, key: string) => page.locator(`[data-structured-path="${key}"]`);
const field = (page: Page, key: string) => page.locator(`[data-setting-path="${key}"]`);
const save = (page: Page) => page.getByRole("button", { name: "保存全部主题设置", exact: true });
async function put(page: Page, data: string) {
  const response = await page.request.put(endpoint, { data, headers: { "Content-Type": "application/json" } });
  expect(response.ok(), await response.text()).toBeTruthy();
}
async function openNode(page: Page, key: string) {
  const target = node(page, key);
  await target.evaluate((el) => {
    for (let parent = el.parentElement; parent; parent = parent.parentElement)
      if (parent instanceof HTMLDetailsElement) parent.open = true;
  });
  return target;
}
test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
});

test("颜色取色、色板、透明度跟随草稿，预览不保存，恢复默认不补写", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{},"blog_extensions":{"unrelated":{"large":9007199254740993}}}');
    const baseline = await (await page.request.get(endpoint)).text();
    await page.goto("/console/settings?tab=appearance");
    const primary = node(page, "theme.appearance.colors.primary");
    const text = primary.getByRole("textbox", { name: "主题色", exact: true });
    await expect(text).toHaveValue("hsl(192 98% 55%)");
    await expect(primary.locator('input[type="color"]')).toHaveValue("#1cd0fd");
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
    await text.fill("hsl(120 100% 50% / 40%)");
    await primary.getByRole("button", { name: "蓝色 #007aff", exact: true }).click();
    await expect(text).toHaveValue("rgba(0, 122, 255, 0.4)");
    await primary.locator('input[type="color"]').fill("#af52de");
    await expect(text).toHaveValue("rgba(175, 82, 222, 0.4)");
    await primary.getByRole("slider", { name: "主题色不透明度", exact: true }).fill("65");
    await expect(text).toHaveValue("rgba(175, 82, 222, 0.65)");
    await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
    await expect
      .poll(() =>
        page
          .frameLocator(".preview-frame")
          .locator("html")
          .evaluate((el) => getComputedStyle(el).getPropertyValue("--stellar-source-color").trim()),
      )
      .toBe("rgba(175, 82, 222, 0.65)");
    expect(await (await page.request.get(endpoint)).text()).toBe(baseline);
    await page.getByRole("button", { name: "关闭预览", exact: true }).click();
    await text.fill("var(--custom-color)");
    await expect(primary.getByRole("slider")).toBeDisabled();
    await expect(text).toHaveValue("var(--custom-color)");
    await primary.getByRole("button", { name: "主题色恢复默认", exact: true }).click();
    await expect(text).toHaveValue("hsl(192 98% 55%)");
    await page.getByRole("button", { name: "主题外观恢复默认", exact: true }).first().click();
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
    expect(await (await page.request.get(endpoint)).text()).toBe(baseline);

    const overlay = await openNode(page, "theme.appearance.backgrounds.page.backdrop.overlay");
    await expect(overlay.locator('input[type="text"]')).toHaveValue("var(--bg-a75)");
    await expect(overlay.getByRole("slider")).toBeDisabled();
    await expect(overlay).toContainText("此颜色依赖前台环境");
    await overlay.locator('input[type="text"]').fill("rgba(20, 30, 40, 0.25)");
    await overlay.getByRole("button", { name: "蓝色 #007aff", exact: true }).click();
    await expect(overlay.locator('input[type="text"]')).toHaveValue("rgba(0, 122, 255, 0.25)");
    const gradient = await openNode(page, "theme.appearance.backgrounds.leftbar.gradient.light.0");
    await expect(gradient.getByTestId("setting-color-editor")).toHaveCount(1);
    await gradient.getByRole("button", { name: "绿色 #34c759", exact: true }).click();
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await save(page).click();
    expect((await response).ok()).toBeTruthy();
    await page.reload();
    const stored = await (await page.request.get(endpoint)).text();
    expect(stored).toContain("9007199254740993");
    await expect(
      (await openNode(page, "theme.appearance.backgrounds.page.backdrop.overlay")).locator('input[type="text"]'),
    ).toHaveValue("rgba(0, 122, 255, 0.25)");
    expect(JSON.parse(stored).theme.appearance.backgrounds.leftbar.gradient.light[0]).toBe("#34c759");
  } finally {
    await put(page, original);
  }
});

test("分类与 AI 颜色使用 HEX 色板，新增待添加状态和无损保存保持", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(
      page,
      '{"version":2,"theme":{},"blog_extensions":{"unrelated":{"large":9007199254740993},"content":{"enabled":true,"ai_label":{"reviewed":{"color":"4caf50","icon":"default:shield-check"}}}}}',
    );
    await page.goto("/console/settings?tab=post");
    const categories = field(page, "theme.article.category_colors");
    await categories.getByRole("textbox", { name: "新增分类颜色名称", exact: true }).fill("调色板验收");
    const newColor = categories
      .getByTestId("setting-color-editor")
      .filter({ has: page.getByRole("textbox", { name: "新增分类颜色值", exact: true }) });
    await newColor.getByRole("button", { name: "紫色 #af52de", exact: true }).click();
    await expect(save(page)).toBeDisabled();
    await expect(newColor.getByRole("slider")).toHaveCount(0);
    await categories.getByRole("button", { name: "添加", exact: true }).click();
    await expect(save(page)).toBeEnabled();
    const ai = field(page, "blog_extensions.content.ai_label.reviewed");
    await expect(ai.getByRole("textbox", { name: "AI 辅助审阅颜色", exact: true })).toHaveValue("4caf50");
    await expect(ai.locator('input[type="color"]')).toHaveValue("#4caf50");
    await expect(ai.getByRole("slider")).toHaveCount(0);
    await ai.getByRole("button", { name: "橙色 #ff9500", exact: true }).click();
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await save(page).click();
    expect((await response).ok()).toBeTruthy();
    await page.reload();
    await expect(categories.getByRole("textbox", { name: "分类颜色值 调色板验收", exact: true })).toHaveValue(
      "#af52de",
    );
    await expect(ai.getByRole("textbox", { name: "AI 辅助审阅颜色", exact: true })).toHaveValue("#ff9500");
    const raw = await (await page.request.get(endpoint)).text();
    expect(raw).toContain("9007199254740993");
    expect(JSON.parse(raw).blog_extensions.content.ai_label.reviewed.icon).toBe("default:shield-check");
  } finally {
    await put(page, original);
  }
});

test("桌面与手机色板可达、键盘可操作且无横向溢出", async ({ page }) => {
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/console/settings?tab=appearance");
    const primary = node(page, "theme.appearance.colors.primary");
    await primary.getByRole("button", { name: "紫色 #af52de", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(primary.getByRole("textbox", { name: "主题色", exact: true })).toHaveValue("#af52de");
    await primary.scrollIntoViewIfNeeded();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth),
    ).toBeLessThanOrEqual(1);
    for (const color of await primary.getByRole("group").getByRole("button").all()) {
      const box = await color.boundingBox();
      expect(box?.width).toBeGreaterThanOrEqual(44);
      expect(box?.height).toBeGreaterThanOrEqual(44);
    }
    await page.screenshot({ path: path.resolve(`docs/baselines/settings-effects/after-color-${width}.png`) });
    await primary.getByRole("button", { name: "主题色恢复默认", exact: true }).click();
    await page.locator("#tab-post").click();
    const categories = field(page, "theme.article.category_colors");
    await categories.scrollIntoViewIfNeeded();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth),
    ).toBeLessThanOrEqual(1);
    await page.screenshot({ path: path.resolve(`docs/baselines/settings-effects/after-category-color-${width}.png`) });
  }
});
