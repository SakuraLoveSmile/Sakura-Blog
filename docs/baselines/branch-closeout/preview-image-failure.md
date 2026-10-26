# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: settings-effects.spec.ts >> 静态文章预览保留横幅与正文图片
- Location: tests/e2e/settings-effects.spec.ts:102:1

# Error details

```
Error: expect(locator).toHaveCSS(expected) failed

Locator:  locator('.preview-frame').contentFrame().locator('.article.banner img.bg')
Expected: "1"
Received: "0"
Timeout:  5000ms

Call log:
  - Expect "toHaveCSS" with timeout 5000ms
  - waiting for locator('.preview-frame').contentFrame().locator('.article.banner img.bg')
    13 × locator resolved to <img class="lazy bg loaded" data-src="/assets/img/preview/image-demo.jpg" src="http://127.0.0.1:8096/assets/img/preview/image-demo.jpg"/>
       - unexpected value "0"

```

```yaml
- img
```

# Test source

```ts
  18  |   await page.goto("/console/settings?tab=appearance");
  19  |   const preset = node(page, "theme.appearance.preset");
  20  |   for (const name of ["卡片", "玻璃", "极简", "平面"]) {
  21  |     await preset.getByRole("radio", { name, exact: true }).check();
  22  |     await expect(preset.getByRole("radio", { name, exact: true })).toBeChecked();
  23  |   }
  24  |   await node(page, "theme.appearance.color_scheme").getByRole("radio", { name: "浅色", exact: true }).check();
  25  |   await expect(page.locator(".settings-effect-guide")).toContainText("未保存修改");
  26  |   await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
  27  |   const frame = page.frameLocator(".preview-frame");
  28  |   await expect(frame.locator("html")).toHaveAttribute("data-theme", "light");
  29  |   await expect(frame.locator('link[rel="stylesheet"][href*="main-flat.css"]')).toHaveCount(1);
  30  |   await expect(frame.locator(".site-shell")).toHaveCount(1);
  31  |   await expect(page.locator(".preview-frame")).toHaveAttribute("sandbox", "");
  32  |   await expect(frame.locator("script")).toHaveCount(0);
  33  |   await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(1200);
  34  |   await page.getByRole("button", { name: "手机", exact: true }).click();
  35  |   await expect.poll(() => frame.locator("html").evaluate(() => innerWidth)).toBe(390);
  36  |   await page.screenshot({ path: screenshot("draft-preview") });
  37  |   await page.getByRole("button", { name: "关闭预览", exact: true }).click();
  38  |   await page.getByRole("button", { name: "主题外观恢复默认", exact: true }).first().click();
  39  |   await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
  40  |   expect(await (await page.request.get(endpoint)).text()).toBe(original);
  41  | });
  42  |
  43  | test("预览失败明确显示诊断，重试使用最新草稿且不写入配置", async ({ page }) => {
  44  |   const original = await (await page.request.get(endpoint)).text();
  45  |   await page.goto("/console/settings?tab=appearance");
  46  |   await node(page, "theme.appearance.preset").getByRole("radio", { name: "玻璃", exact: true }).check();
  47  |   await page.route("**/api/v1/preview/site", route => route.fulfill({ status: 500, contentType: "application/json", body: '{"error":"预览服务暂不可用"}' }));
  48  |   await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
  49  |   await expect(page.locator(".preview-error")).toBeVisible();
  50  |   await expect(page.locator(".preview-frame")).toHaveCount(0);
  51  |   await page.unroute("**/api/v1/preview/site");
  52  |   await page.getByRole("button", { name: "重试预览", exact: true }).click();
  53  |   await expect(page.frameLocator(".preview-frame").locator('link[rel="stylesheet"][href*="main-glass.css"]')).toHaveCount(1);
  54  |   await page.locator("#tab-navigation").click();
  55  |   await expect(page.getByLabel("预览页面", { exact: true })).toBeVisible();
  56  |   await page.getByLabel("预览页面", { exact: true }).selectOption("wiki");
  57  |   await expect(page.locator(".preview-frame")).toBeVisible();
  58  |   expect(await (await page.request.get(endpoint)).text()).toBe(original);
  59  | });
  60  |
  61  | test("全部分类说明与选项在桌面及手机可读，开关与依赖解释当前效果", async ({ page }) => {
  62  |   test.setTimeout(90_000);
  63  |   for (const width of [1280, 390]) {
  64  |     await page.setViewportSize({ width, height: 900 });
  65  |     await page.goto("/console/settings?tab=appearance");
  66  |     for (const key of ["basic", "appearance", "navigation", "layout", "post", "comments", "seo", "search", "plugins", "services", "community", "feedback", "advanced"]) {
  67  |       await page.locator(`#tab-${key}`).click();
  68  |       await expect(page.locator(".settings-effect-guide")).toContainText("影响位置");
  69  |       await expect(page.locator(".settings-effect-guide")).toContainText("如何确认");
  70  |       expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  71  |     }
  72  |     await page.locator("#tab-appearance").click();
  73  |     if (width === 390) {
  74  |       const explanation = page.locator(".guide-explanation");
  75  |       await expect(explanation).not.toHaveAttribute("open", "");
  76  |       await explanation.locator("summary").click();
  77  |       await expect(explanation.getByRole("heading", { name: "影响位置", exact: true })).toBeVisible();
  78  |       await explanation.locator("summary").click();
  79  |     }
  80  |     await page.locator(".settings-card-tabs").scrollIntoViewIfNeeded();
  81  |     await page.screenshot({ path: screenshot(`after-appearance-${width}`) });
  82  |     const preset = node(page, "theme.appearance.preset");
  83  |     if (width === 390) {
  84  |       await preset.scrollIntoViewIfNeeded();
  85  |       await page.screenshot({ path: screenshot("after-choices-390") });
  86  |     }
  87  |     await preset.getByRole("radio", { name: "卡片", exact: true }).focus();
  88  |     await preset.getByRole("radio", { name: "卡片", exact: true }).press("ArrowRight");
  89  |     await expect(preset.getByRole("radio", { name: "玻璃", exact: true })).toBeChecked();
  90  |     await page.getByRole("button", { name: "主题外观恢复默认", exact: true }).first().click();
  91  |     await page.locator("#tab-post").click();
  92  |     await expect(page.locator('[data-setting-path="blog_extensions.content.reward.text"] .settings-dependency')).toContainText("文章扩展尚未启用");
  93  |     await page.locator("#tab-comments").click();
  94  |     const moderation = page.locator('[data-setting-path="blog_extensions.native_comments.moderation"]');
  95  |     await moderation.getByRole("switch").click();
  96  |     await expect(moderation.locator(".settings-field-effect")).toContainText("经审核后才公开");
  97  |     await page.screenshot({ path: screenshot(`after-comments-${width}`) });
  98  |     await moderation.getByRole("button", { name: "评论需要审核恢复默认", exact: true }).click();
  99  |   }
  100 | });
  101 |
  102 | test("静态文章预览保留横幅与正文图片", async ({ page }) => {
  103 |   const slug = `settings-image-preview-${Date.now()}`;
  104 |   const imagePath = "/assets/img/preview/image-demo.jpg";
  105 |   const response = await page.request.post("/api/v1/posts", { data: {
  106 |     slug, title: "设置效果预览文章", status: "published", cover: imagePath, banner: imagePath,
  107 |     content_md: `## 阅读效果\n\n预览应显示图片与真实页面骨架。\n\n![正文图片](${imagePath})`,
  108 |     stellar_version: 2, stellar_config: JSON.stringify({cover: imagePath, banner: {background: true, avatar: imagePath}}), blog_extensions: "{}",
  109 |   } });
  110 |   expect(response.ok(), await response.text()).toBeTruthy();
  111 |   try {
  112 |     await page.goto("/console/settings?tab=post");
  113 |     await page.getByRole("button", { name: "查看草稿效果", exact: true }).click();
  114 |     const frame = page.frameLocator(".preview-frame");
  115 |     const banner = frame.locator(".article.banner img.bg");
  116 |     await expect(banner).toHaveAttribute("src", new RegExp(imagePath));
  117 |     await expect.poll(() => banner.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
> 118 |     await expect(banner).toHaveCSS("opacity", "1");
      |                          ^ Error: expect(locator).toHaveCSS(expected) failed
  119 |     await expect(frame.getByRole("img", { name: "正文图片" })).toBeVisible();
  120 |     await page.screenshot({ path: screenshot("article-images-preview") });
  121 |   } finally { await page.request.delete(`/api/v1/posts/${slug}`); }
  122 | });
  123 |
```
