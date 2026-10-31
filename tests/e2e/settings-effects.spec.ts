import { test, expect, type Page } from "@playwright/test";
import path from "node:path";

const endpoint = "/api/v1/settings/stellar";
const node = (page: Page, key: string) => page.locator(`#tabpanel-appearance [data-structured-path="${key}"]`);
const screenshot = (name: string) => path.resolve(`docs/baselines/settings-effects/${name}.png`);

test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
});

test("选项示意跟随草稿，恢复默认且浏览预览均不保存", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=appearance");
  const preset = node(page, "theme.appearance.preset");
  for (const name of ["卡片", "玻璃", "极简", "平面"]) {
    await preset.getByRole("radio", { name, exact: true }).check();
    await expect(preset.getByRole("radio", { name, exact: true })).toBeChecked();
  }
  await node(page, "theme.appearance.color_scheme").getByRole("radio", { name: "浅色", exact: true }).check();
  await expect(page.locator(".settings-effect-guide")).toContainText("未保存修改");
  await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
  const frame = page.frameLocator(".preview-frame");
  await expect(frame.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(frame.locator('link[rel="stylesheet"][href*="main-flat.css"]')).toHaveCount(1);
  await expect(frame.locator(".site-shell")).toHaveCount(1);
  await expect(page.locator(".preview-frame")).toHaveAttribute("sandbox", "");
  await expect(frame.locator("script")).toHaveCount(0);
  await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(1200);
  await page.getByRole("button", { name: "手机", exact: true }).click();
  await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(390);
  await page.screenshot({ path: screenshot("draft-preview") });
  await page.getByRole("button", { name: "关闭预览", exact: true }).click();
  await page.getByRole("button", { name: "主题外观恢复默认", exact: true }).first().click();
  await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("预览失败明确显示诊断，重试使用最新草稿且不写入配置", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=appearance");
  await node(page, "theme.appearance.preset").getByRole("radio", { name: "玻璃", exact: true }).check();
  await page.route("**/api/v1/preview/site", route => route.fulfill({ status: 500, contentType: "application/json", body: '{"error":"预览服务暂不可用"}' }));
  await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
  await expect(page.locator(".preview-error")).toBeVisible();
  await expect(page.locator(".preview-frame")).toHaveCount(0);
  await page.unroute("**/api/v1/preview/site");
  await page.getByRole("button", { name: "重试预览", exact: true }).click();
  await expect(page.frameLocator(".preview-frame").locator('link[rel="stylesheet"][href*="main-glass.css"]')).toHaveCount(1);
  await page.locator("#tab-navigation").click();
  await expect(page.getByLabel("预览页面", { exact: true })).toBeVisible();
  await page.getByLabel("预览页面", { exact: true }).selectOption("wiki");
  await expect(page.locator(".preview-frame")).toBeVisible();
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("全部分类说明与选项在桌面及手机可读，开关与依赖解释当前效果", async ({ page }) => {
  test.setTimeout(90_000);
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/console/settings?tab=appearance");
    for (const key of ["basic", "appearance", "navigation", "layout", "post", "comments", "seo", "search", "plugins", "services", "community", "feedback", "advanced"]) {
      await page.locator(`#tab-${key}`).click();
      await expect(page.locator(".settings-effect-guide")).toContainText("影响位置");
      await expect(page.locator(".settings-effect-guide")).toContainText("如何确认");
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
    }
    await page.locator("#tab-appearance").click();
    if (width === 390) {
      const explanation = page.locator(".guide-explanation");
      await expect(explanation).not.toHaveAttribute("open", "");
      await explanation.locator("summary").click();
      await expect(explanation.getByRole("heading", { name: "影响位置", exact: true })).toBeVisible();
      await explanation.locator("summary").click();
    }
    await page.locator(".settings-card-tabs").scrollIntoViewIfNeeded();
    await page.screenshot({ path: screenshot(`after-appearance-${width}`) });
    const preset = node(page, "theme.appearance.preset");
    if (width === 390) {
      await preset.scrollIntoViewIfNeeded();
      await page.screenshot({ path: screenshot("after-choices-390") });
    }
    await preset.getByRole("radio", { name: "卡片", exact: true }).focus();
    await preset.getByRole("radio", { name: "卡片", exact: true }).press("ArrowRight");
    await expect(preset.getByRole("radio", { name: "玻璃", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "主题外观恢复默认", exact: true }).first().click();
    await page.locator("#tab-post").click();
    await expect(page.locator('[data-setting-path="blog_extensions.content.reward.text"] .settings-dependency')).toContainText("文章扩展尚未启用");
    await page.locator("#tab-comments").click();
    const moderation = page.locator('[data-setting-path="blog_extensions.native_comments.moderation"]');
    await moderation.getByRole("switch").click();
    await expect(moderation.locator(".settings-field-effect")).toContainText("经审核后才公开");
    await page.screenshot({ path: screenshot(`after-comments-${width}`) });
    await moderation.getByRole("button", { name: "评论需要审核恢复默认", exact: true }).click();
  }
});

test("静态文章预览保留横幅与正文图片", async ({ page }) => {
  const slug = `settings-image-preview-${Date.now()}`;
  const imagePath = "/assets/img/preview/image-demo.jpg";
  const response = await page.request.post("/api/v1/posts", { data: {
    slug, title: "设置效果预览文章", status: "published", cover: imagePath, banner: imagePath,
    content_md: `## 阅读效果\n\n预览应显示图片与真实页面骨架。\n\n![正文图片](${imagePath})`,
    stellar_version: 2, stellar_config: JSON.stringify({cover: imagePath, banner: {background: true, avatar: imagePath}}), blog_extensions: "{}",
  } });
  expect(response.ok(), await response.text()).toBeTruthy();
  try {
    await page.goto("/console/settings?tab=post");
    await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
    const frame = page.frameLocator(".preview-frame");
    const banner = frame.locator(".article.banner img.bg");
    await expect(banner).toHaveAttribute("src", new RegExp(imagePath));
    await expect.poll(() => banner.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await expect(banner).toHaveCSS("opacity", "1");
    await expect(frame.getByRole("img", { name: "正文图片" })).toBeVisible();
    await page.screenshot({ path: screenshot("article-images-preview") });
  } finally { await page.request.delete(`/api/v1/posts/${slug}`); }
});
