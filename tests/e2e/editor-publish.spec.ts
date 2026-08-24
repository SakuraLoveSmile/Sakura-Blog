import { expect, test } from "@playwright/test";

test("登录 → 写文 → 发布关键路径", async ({ page }) => {
  const title = `Playwright E2E ${Date.now()}`;

  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });

  await page.goto("/console/posts/new");
  await page.getByPlaceholder("文章标题").fill(title);
  const editor = page.locator('[contenteditable="true"]').first();
  await editor.fill("这是一篇由 Playwright 关键路径创建的文章。");

  const publishResponse = page.waitForResponse(
    (response) => response.url().includes("/api/v1/posts") && response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "发布", exact: true }).click();
  const response = await publishResponse;
  expect(response.ok()).toBeTruthy();
  await expect(page.getByText("发布成功", { exact: true })).toBeVisible({ timeout: 15_000 });
});
