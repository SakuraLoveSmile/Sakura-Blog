import { test, expect, type Page } from "@playwright/test";
import { openSettingsPage } from "./support/settings";

const endpoint = "/api/v1/settings/stellar";
// 外观预设与配色模式是版式“行”（data-setting-path），不再包在结构化节点里。
const node = (page: Page, key: string) => page.locator(`#settings-page-appearance [data-setting-path="${key}"]`);
const ALL_PAGES = ["site", "seo", "appearance", "navigation", "layout", "post", "listing", "community", "comments", "feedback", "extensions", "advanced"];

test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
});

test("选项示意跟随草稿，恢复默认且浏览预览均不保存", async ({ page }, testInfo) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=appearance");
  const preset = node(page, "theme.appearance.preset");
  for (const name of ["卡片", "玻璃", "极简", "平面"]) {
    await preset.getByRole("radio", { name, exact: true }).check();
    await expect(preset.getByRole("radio", { name, exact: true })).toBeChecked();
  }
  await node(page, "theme.appearance.color_scheme").getByRole("radio", { name: "浅色", exact: true }).check();
  await expect(page.locator(".settings-dirty-badge")).toContainText("未保存修改");
  await page.getByRole("button", { name: "前端预览", exact: true }).click();
  const frame = page.frameLocator(".preview-frame");
  await expect(frame.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(frame.locator('link[rel="stylesheet"][href*="main-flat.css"]')).toHaveCount(1);
  await expect(frame.locator(".site-shell")).toHaveCount(1);
  await expect(page.locator(".preview-frame")).toHaveAttribute("sandbox", "");
  await expect(frame.locator("script")).toHaveCount(0);
  await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(1200);
  await page.getByRole("button", { name: "手机", exact: true }).click();
  await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(390);
  // 截图写入 Playwright 输出目录，不覆写受跟踪的 docs/baselines。
  await page.screenshot({ path: testInfo.outputPath("draft-preview.png") });
  await page.getByRole("button", { name: "关闭预览", exact: true }).click();
  await page.getByRole("button", { name: "恢复主题外观默认", exact: true }).click();
  await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("预览失败明确显示诊断，重试使用最新草稿且不写入配置", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=appearance");
  await node(page, "theme.appearance.preset").getByRole("radio", { name: "玻璃", exact: true }).check();
  await page.route("**/api/v1/preview/site", route => route.fulfill({ status: 500, contentType: "application/json", body: '{"error":"预览服务暂不可用"}' }));
  await page.getByRole("button", { name: "前端预览", exact: true }).click();
  await expect(page.locator(".preview-error")).toBeVisible();
  await expect(page.locator(".preview-frame")).toHaveCount(0);
  await page.unroute("**/api/v1/preview/site");
  await page.getByRole("button", { name: "重试预览", exact: true }).click();
  await expect(page.frameLocator(".preview-frame").locator('link[rel="stylesheet"][href*="main-glass.css"]')).toHaveCount(1);
  await openSettingsPage(page, "navigation");
  await expect(page.getByLabel("预览页面", { exact: true })).toBeVisible();
  await page.getByLabel("预览页面", { exact: true }).selectOption("wiki");
  await expect(page.locator(".preview-frame")).toBeVisible();
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("全部设置页说明与选项在桌面及手机可读，开关与依赖解释当前效果", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/console/settings?tab=appearance");
    for (const key of ALL_PAGES) {
      await openSettingsPage(page, key);
      // 页面标题下只有一句话说明，没有单独的“效果说明条”；看效果的入口是页头的“前端预览”。
      await expect(page.locator(`#settings-page-${key} .settings-page-head > p`)).toBeVisible();
      await expect(page.locator(".settings-effect-guide")).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
    }
    await openSettingsPage(page, "appearance");
    await page.screenshot({ path: testInfo.outputPath(`after-appearance-${width}.png`) });
    const preset = node(page, "theme.appearance.preset");
    if (width === 390) {
      await preset.scrollIntoViewIfNeeded();
      await page.screenshot({ path: testInfo.outputPath("after-choices-390.png") });
    }
    await preset.getByRole("radio", { name: "卡片", exact: true }).focus();
    await preset.getByRole("radio", { name: "卡片", exact: true }).press("ArrowRight");
    await expect(preset.getByRole("radio", { name: "玻璃", exact: true })).toBeChecked();
    await page.getByRole("button", { name: "恢复主题外观默认", exact: true }).click();
    await openSettingsPage(page, "post");
    await expect(page.locator('[data-setting-path="blog_extensions.content.reward.text"] .settings-dependency')).toContainText("文章扩展尚未启用");
    // 同一分区里依赖项的说明只在第一项可见，其余只留给读屏；变淡的样式逐行标示，但仍可编辑。
    await expect(page.locator('[data-setting-path="blog_extensions.content.banner_ratio"] .settings-dependency')).toBeVisible();
    // 其余依赖项的说明用“仅读屏”样式裁成 1×1；Playwright 把非空包围盒视为 visible，所以按包围盒断言视觉上不占位。
    const quiet = page.locator('[data-setting-path="blog_extensions.content.reward.text"] .settings-dependency');
    await expect(quiet).toHaveClass(/is-quiet/);
    const quietBox = await quiet.boundingBox();
    expect(quietBox!.width).toBeLessThanOrEqual(1);
    expect(quietBox!.height).toBeLessThanOrEqual(1);
    await expect(page.locator('[data-setting-path="blog_extensions.content.reward.text"]')).toHaveClass(/is-dim/);
    await expect(page.locator('[data-setting-path="blog_extensions.content.reward.text"] input').first()).toBeEditable();
    await openSettingsPage(page, "comments");
    const moderation = page.locator('[data-setting-path="blog_extensions.native_comments.moderation"]');
    // 隔离夹具为公开评论用例预先开启审核；先恢复默认（关闭）再验证开关与效果文案，避免依赖夹具初始值。
    await moderation.getByRole("button", { name: "评论需要审核恢复默认", exact: true }).click();
    await expect(moderation.getByRole("switch")).toHaveAttribute("aria-checked", "false");
    await expect(moderation.locator(".settings-field-effect")).toContainText("无需经过这项审核开关");
    await moderation.getByRole("switch").click();
    await expect(moderation.getByRole("switch")).toHaveAttribute("aria-checked", "true");
    await expect(moderation.locator(".settings-field-effect")).toContainText("经审核后才公开");
    await page.screenshot({ path: testInfo.outputPath(`after-comments-${width}.png`) });
    await moderation.getByRole("button", { name: "评论需要审核恢复默认", exact: true }).click();
  }
});

test("静态文章预览保留横幅与正文图片", async ({ page }, testInfo) => {
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
    await page.getByRole("button", { name: "前端预览", exact: true }).click();
    const frame = page.frameLocator(".preview-frame");
    const banner = frame.locator(".article.banner img.bg");
    await expect(banner).toHaveAttribute("src", new RegExp(imagePath));
    await expect.poll(() => banner.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await expect(banner).toHaveCSS("opacity", "1");
    await expect(frame.getByRole("img", { name: "正文图片" })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("article-images-preview.png") });
  } finally { await page.request.delete(`/api/v1/posts/${slug}`); }
});
