import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

test("文章导入支持选择、取消与重复选择同一文件", async ({ page }) => {
  await login(page);
  await page.goto("/console/backup");
  await expect(page.getByRole("heading", { name: "备份与导出", exact: true })).toBeVisible();

  const fileName = `playwright-import-${Date.now()}.md`;
  const file = {
    name: fileName,
    mimeType: "text/markdown",
    buffer: Buffer.from("---\ntitle: Playwright 导入验证\n---\n\n导入验证正文。\n"),
  };
  const importButton = page.getByRole("button", { name: "选择文件导入", exact: true });
  const importInput = page.locator('input[type="file"][accept=".md,.zip"]');
  let importRequests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/v1/backup/import/markdown") && request.method() === "POST") {
      importRequests++;
    }
  });

  const firstChooser = page.waitForEvent("filechooser");
  await importButton.click();
  await (await firstChooser).setFiles(file);
  await expect(page.getByText(`确定导入「${fileName}」吗？`, { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(page.getByText(`确定导入「${fileName}」吗？`, { exact: true })).toBeHidden();
  await expect(importInput).toHaveValue("");
  expect(importRequests).toBe(0);

  const secondChooser = page.waitForEvent("filechooser");
  await importButton.click();
  await (await secondChooser).setFiles(file);
  await expect(page.getByText(`确定导入「${fileName}」吗？`, { exact: true })).toBeVisible();

  const importResponse = page.waitForResponse(
    (response) => response.url().includes("/api/v1/backup/import/markdown") && response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "确定", exact: true }).click();
  expect((await importResponse).ok()).toBeTruthy();
  await expect(page.locator("#app").getByText("成功导入 1 篇文章", { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(importInput).toHaveValue("");
});

test("整站恢复按钮支持选择备份文件并进入确认流程", async ({ page }) => {
  await login(page);
  await page.goto("/console/backup");
  await expect(page.getByRole("heading", { name: "备份与导出", exact: true })).toBeVisible();

  const restoreButton = page.getByRole("button", { name: "上传备份并恢复", exact: true });
  const chooser = page.waitForEvent("filechooser");
  await restoreButton.click();
  await (
    await chooser
  ).setFiles({
    name: "blog-backup-e2e.zip",
    mimeType: "application/zip",
    buffer: Buffer.from("placeholder"),
  });

  await expect(page.getByRole("heading", { name: /确认从备份恢复整站数据/ })).toBeVisible();
  await expect(page.getByText("本地上传文件：blog-backup-e2e.zip", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).click();
});
