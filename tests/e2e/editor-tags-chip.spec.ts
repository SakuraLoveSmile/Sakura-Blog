import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}

// 标签改为 token 式输入：回车/逗号生成 chip，退格删除最后一个，保存后按配置回显。
test("文章编辑器标签：回车与逗号生成 chip、退格删除、保存后回显", async ({ page }) => {
  test.setTimeout(120_000);
  await login(page);
  const slug = `tags-chip-${Date.now()}`;
  const created = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title: "标签 chip 验收",
      content_md: "正文",
      status: "draft",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(created.ok()).toBeTruthy();
  try {
    await page.goto(`/console/posts/edit/${slug}`);
    await page.waitForSelector(".editor-toolbar", { timeout: 20_000 });
    await page.locator("button[aria-pressed]").filter({ hasText: "设置" }).first().click();

    const input = page.getByPlaceholder("输入标签后回车");
    await expect(input).toBeVisible();
    await input.fill("测试");
    await input.press("Enter");
    await expect(page.getByRole("button", { name: "移除标签 测试", exact: true })).toBeVisible();
    await expect(input).toHaveValue("");

    await input.fill("abb");
    await input.press(",");
    await expect(page.getByRole("button", { name: "移除标签 abb", exact: true })).toBeVisible();
    await expect(input).toHaveValue("");

    await input.fill("1");
    await input.press("Enter");
    await expect(page.getByRole("button", { name: "移除标签 1", exact: true })).toBeVisible();

    // 输入框为空时退格删除最后一个 chip。
    await input.click();
    await input.press("Backspace");
    await expect(page.getByRole("button", { name: "移除标签 1", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "移除标签 abb", exact: true })).toBeVisible();

    const saved = page.waitForResponse(
      (response) =>
        response.url().includes(`/api/v1/posts/${slug}`) && ["PUT", "POST"].includes(response.request().method()),
      { timeout: 20_000 },
    );
    await page
      .getByRole("button", { name: /保存草稿/ })
      .first()
      .click();
    expect((await saved).ok()).toBeTruthy();

    await page.reload({ waitUntil: "networkidle" });
    await page.waitForSelector(".editor-toolbar", { timeout: 20_000 });
    await page.locator("button[aria-pressed]").filter({ hasText: "设置" }).first().click();
    await expect(page.getByRole("button", { name: "移除标签 测试", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "移除标签 abb", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "移除标签 1", exact: true })).toHaveCount(0);
  } finally {
    await page.request.delete(`/api/v1/posts/${slug}`).catch(() => {});
  }
});
