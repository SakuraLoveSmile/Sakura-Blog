import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

const markdown =
  "---\ntitle: 导入标题\ntags: [Go, Stellar]\nstatus: published\n---\n\n{% tip text:提示 %}\n无损正文\n{% endtip %}\n";

test("文章编辑器导入 Markdown：取消不替换，确认后识别 Stellar 标签", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("post-editor:kind", "markdown"));
  await login(page);
  await page.goto("/console/posts/new");

  const title = page.getByPlaceholder("文章标题");
  const body = page.getByPlaceholder(/Markdown 内容/);
  await title.fill("原文章标题");
  await body.fill("原正文");

  const file = { name: "editor-import.md", mimeType: "text/markdown", buffer: Buffer.from(markdown) };
  const chooser = page.waitForEvent("filechooser");
  await page.getByTitle("导入 Markdown 文件").click();
  await (await chooser).setFiles(file);
  await expect(page.getByRole("heading", { name: "确认导入 Markdown？" })).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(title).toHaveValue("原文章标题");
  await expect(body).toHaveValue("原正文");

  const chooserAgain = page.waitForEvent("filechooser");
  await page.getByTitle("导入 Markdown 文件").click();
  await (await chooserAgain).setFiles(file);
  await expect(page.getByRole("heading", { name: "确认导入 Markdown？" })).toBeVisible();
  await expect(page.getByText(/将识别 Stellar 标签：tip/)).toBeVisible();
  await page.getByRole("button", { name: "导入并替换", exact: true }).click();
  await expect(title).toHaveValue("导入标题");
  await expect(body).toHaveValue(markdown.replace(/^---[\s\S]*?---\n\n/, ""));
});

test("页面编辑器提供同一 Markdown 导入入口", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("page-editor:kind", "markdown"));
  await login(page);
  await page.goto("/console/pages/edit/new");
  await expect(page.getByTitle("导入 Markdown 文件")).toBeVisible();
  const chooser = page.waitForEvent("filechooser");
  await page.getByTitle("导入 Markdown 文件").click();
  await (await chooser).setFiles({ name: "page.md", mimeType: "text/markdown", buffer: Buffer.from("# 页面") });
  await expect(page.getByRole("heading", { name: "确认导入 Markdown？" })).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).nth(1).click();
});
