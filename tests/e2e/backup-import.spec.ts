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

test("备份页显式转换 1.x Markdown，未指定状态的新文章作为草稿导入", async ({ page }) => {
  await login(page);
  await page.goto("/console/backup");
  await page.getByRole("checkbox", { name: /从 1.x 格式转换/ }).check();
  const slug = `legacy-batch-${Date.now()}`;
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "选择文件导入", exact: true }).click();
  await (await chooser).setFiles({
    name: "different-filename.md",
    mimeType: "text/markdown",
    buffer: Buffer.from(`---\ntitle: 旧站批量文章\nslug: ${slug}\ncategory: 升级验收\ndate: 2021-03-04\n---\n\n保留历史文章正文。\n`),
  });
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("未指定状态的新文章保存为草稿");
  const response = page.waitForResponse((r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST");
  await dialog.getByRole("button", { name: "确定", exact: true }).click();
  const result = await response;
  expect(result.ok(), await result.text()).toBeTruthy();
  await expect(page.locator("#app").getByText("成功导入 1 篇文章", { exact: true })).toBeVisible();
  const saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(saved.status).toBe("draft");
  expect(saved.created_at).toContain("2021-03-04");
  expect(saved.content_md).toContain("保留历史文章正文。");
  expect((await page.request.get(`/post/${slug}`)).status()).toBe(404);
});
