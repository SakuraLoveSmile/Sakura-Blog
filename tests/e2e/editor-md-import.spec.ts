import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function importFile(page: Page, content: string, legacy: boolean, name = "editor-import.md") {
  await page.getByTitle("导入 Markdown 文件").click();
  await expect(page.getByRole("dialog", { name: "选择 Markdown 格式" })).toBeVisible();
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: legacy ? "从 1.x 转换" : "Stellar 2 / 普通 Markdown", exact: true }).click();
  await (await chooser).setFiles({ name, mimeType: "text/markdown", buffer: Buffer.from(content) });
}

test("旧文章导入先预览：取消不替换，保存草稿重开后发布并显示在前台", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("post-editor:kind", "markdown"));
  await login(page);
  await page.goto("/console/posts/new");
  const slug = `legacy-editor-${Date.now()}`;
  const bodyText = "旧版文章正文。\n\n```text\n{% tip 示例代码 %}\n```\n";
  const markdown = `---\ntitle: 旧版导入标题\nslug: ${slug}\ndate: 2022-02-03\nstatus: published\npinned: true\ncategory: 升级验收\ntags: [Go, Stellar]\n---\n${bodyText}`;
  const title = page.getByPlaceholder("文章标题");
  const body = page.getByPlaceholder(/Markdown 内容/);
  await title.fill("原文章标题");
  await body.fill("原正文");

  await importFile(page, markdown, true);
  const preview = page.getByRole("dialog", { name: "审查 1.x 转换预览" });
  await expect(preview).toBeVisible();
  await expect(preview).toContainText("不会自动保存或发布");
  expect((await page.request.get(`/api/v1/posts/${slug}`)).status()).toBe(404);
  await preview.getByRole("button", { name: "取消", exact: true }).click();
  await expect(title).toHaveValue("原文章标题");
  await expect(body).toHaveValue("原正文");

  await importFile(page, markdown, true);
  await preview.getByRole("button", { name: "确认回填", exact: true }).click();
  await expect(title).toHaveValue("旧版导入标题");
  await expect(body).toHaveValue(bodyText);
  expect((await page.request.get(`/api/v1/posts/${slug}`)).status()).toBe(404);

  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/console/posts/edit/${slug}$`));
  let saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(saved.status).toBe("draft");
  expect(saved.content_md).toBe(bodyText);
  expect(JSON.parse(saved.stellar_config).date).toBe("2022-02-03");
  expect((await page.request.get(`/post/${slug}`)).status()).toBe(404);
  await page.reload();
  await expect(body).toHaveValue(bodyText);
  await expect(title).toHaveValue("旧版导入标题");
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/posts$/);
  saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(saved.status).toBe("published");
  await page.goto(`/post/${slug}`);
  await expect(page.getByRole("heading", { name: "旧版导入标题", exact: true })).toBeVisible();
  await expect(page.getByText("旧版文章正文。", { exact: true })).toBeVisible();
});

test("手机页面编辑器可选择普通 Markdown，并在预览中取消", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem("page-editor:kind", "markdown"));
  await login(page);
  await page.goto("/console/pages/edit/new");
  await importFile(page, "# 页面\n\n原始 Markdown\n", false, "page.md");
  const preview = page.getByRole("dialog", { name: "审查 Markdown 导入预览" });
  await expect(preview).toBeVisible();
  await expect(preview.getByRole("button", { name: "确认回填", exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await preview.getByRole("button", { name: "取消", exact: true }).click();
  await expect(preview).toBeHidden();
});

test("严格导入不会自动回退为旧格式转换", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("post-editor:kind", "markdown"));
  await login(page);
  await page.goto("/console/posts/new");
  const response = page.waitForResponse((r) => r.url().includes("/editor/import/stellar") && r.request().method() === "POST");
  await importFile(page, "---\ntitle: 旧格式\nstatus: draft\n---\n正文", false);
  expect((await response).status()).toBe(400);
  await expect(page.getByRole("dialog", { name: "审查 1.x 转换预览" })).toHaveCount(0);
  await expect(page.getByPlaceholder("文章标题")).toHaveValue("");
});
