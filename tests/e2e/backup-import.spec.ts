import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

for (const width of [1280, 390]) {
  test(`缺失专栏先确认，取消不写入，确认后与真实头部一起导入（${width}px）`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await login(page);
    await page.goto("/console/backup");
    await page.getByRole("checkbox", { name: /从 1.x 格式转换/ }).check();
    const topicSlug = width === 1280 ? "zero_tech" : "zero_tech_mobile";
    const slug = width === 1280 ? "TeamSpeak使用指南" : "TeamSpeak使用指南-手机";
    const source = fs
      .readFileSync(path.resolve("internal/stellarmigration/testdata/teamspeak-legacy.md"), "utf8")
      .replace("topic: zero_tech", `topic: ${topicSlug}`);
    const file = { name: `${slug}.md`, mimeType: "text/markdown", buffer: Buffer.from(source) };
    const select = async () => {
      await page.locator('input[type="file"][accept=".md,.zip"]').setInputFiles(file);
      const result = page.waitForResponse(
        (r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST",
      );
      await page.getByRole("alertdialog").getByRole("button", { name: "确定", exact: true }).click();
      const response = await result;
      expect(response.status()).toBe(400);
      expect((await response.json()).missing_topics).toEqual([topicSlug]);
    };
    await select();
    const confirmation = page.getByRole("alertdialog", { name: "创建缺失的专栏并导入？" });
    await expect(confirmation).toContainText(topicSlug);
    await expect(confirmation).toContainText("同时作为名称和 Slug");
    await expect(confirmation).toContainText("任一文件失败时整批不写入");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    await page.screenshot({ path: path.resolve(`docs/baselines/markdown-topic-import/confirm-${width}.png`) });
    await confirmation.getByRole("button", { name: "取消", exact: true }).click();
    await expect(confirmation).toHaveCount(0);
    expect((await page.request.get(`/api/v1/posts/${encodeURIComponent(slug)}`)).status()).toBe(404);
    const before = await (await page.request.get("/api/v1/topics")).json();
    expect(before.items.filter((topic: { slug: string }) => topic.slug === topicSlug)).toHaveLength(0);

    await select();
    const imported = page.waitForResponse(
      (r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST",
    );
    await confirmation.getByRole("button", { name: "创建并导入", exact: true }).click();
    const response = await imported;
    expect(response.ok(), await response.text()).toBeTruthy();
    await expect(page.locator("#app").getByText("成功导入 1 篇文章", { exact: true })).toBeVisible();
    const saved = await (await page.request.get(`/api/v1/posts/${encodeURIComponent(slug)}`)).json();
    expect(saved.status).toBe("draft");
    expect(saved.content_md).toBe(source.split("---\n").slice(2).join("---\n"));
    const raw = JSON.parse(saved.stellar_config);
    expect(raw.collection).toEqual({ profile: "topic", id: topicSlug });
    expect(raw.tags).toEqual(["零基础", "游戏"]);
    expect(raw.categories).toEqual(["技术分享"]);
    expect(raw.render).toEqual({ math: false, diagrams: false });
    expect(raw.article.style).toBe("tech");
    expect(raw.abbrlink).toBe("TeamSpeak");
    expect(raw.updated).toBe("2023-12-23");
    expect(raw.description).toContain("本篇文章是一篇简单的入门使用教程");
    expect(raw.footer.references).toHaveLength(2);
    expect(raw.cover).toBe("../../assets/cover/TeamSpeak使用指南.jpg");
    expect(JSON.parse(saved.blog_extensions).content.banner_image).toBe(
      "https://bu.dusays.com/2024/09/01/66d42682cd240.jpg",
    );
    expect(saved.created_at).toContain("2023-12-23");
    expect((await page.request.get(`/post/${encodeURIComponent(slug)}`)).status()).toBe(404);
    const after = await (await page.request.get("/api/v1/topics")).json();
    expect(after.items.filter((topic: { slug: string }) => topic.slug === topicSlug)).toHaveLength(1);
    await page.screenshot({ path: path.resolve(`docs/baselines/markdown-topic-import/imported-${width}.png`) });
  });
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
  await (
    await chooser
  ).setFiles({
    name: "different-filename.md",
    mimeType: "text/markdown",
    buffer: Buffer.from(
      `---\ntitle: 旧站批量文章\nslug: ${slug}\ncategory: 升级验收\ndate: 2021-03-04\n---\n\n保留历史文章正文。\n`,
    ),
  });
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("未指定状态的新文章保存为草稿");
  const response = page.waitForResponse(
    (r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST",
  );
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

for (const width of [1280, 390]) {
  test(`旧版字段导入可取消或确认转换并完整保存（${width}px）`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await login(page);
    const slug = `legacy-render-${width}-${Date.now()}`;
    const topicSlug = `import-topic-${slug}`;
    const topic = await page.request.post("/api/v1/topics", {
      data: {
        name: topicSlug,
        slug: topicSlug,
        stellar_version: 2,
        stellar_config: JSON.stringify({ name: topicSlug }),
        blog_extensions: "{}",
      },
    });
    expect(topic.ok(), await topic.text()).toBeTruthy();
    await page.goto("/console/backup");
    const body = "校园网导入回归正文。\r\n\r\n```md\n{% unknown keep %}\n```\n";
    const file = {
      name: "基于UA3F的校园网防检测指南.md",
      mimeType: "text/markdown",
      buffer: Buffer.from(
        `---\ntitle: 校园网导入回归\nslug: ${slug}\nmathjax: true\nmermaid: true\ntype: tech\nreferences: [https://example.com/reference]\ntopic: ${topicSlug}\nbanner: /banner.jpg\n---\n${body}`,
      ),
    };
    const input = page.locator('input[type="file"][accept=".md,.zip"]');
    const legacyCheckbox = page.getByRole("checkbox", { name: /从 1.x 格式转换/ });
    let requests = 0;
    page.on("request", (request) => {
      if (request.url().includes("/backup/import/markdown") && request.method() === "POST") requests++;
    });
    const selectAndConfirm = async () => {
      await input.setInputFiles(file);
      const response = page.waitForResponse(
        (r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST",
      );
      await page.getByRole("alertdialog").getByRole("button", { name: "确定", exact: true }).click();
      const rejected = await response;
      expect(rejected.status()).toBe(400);
      expect((await rejected.json()).type).toBe("legacy_markdown_requires_conversion");
      await expect(page.getByRole("alertdialog", { name: "检测到 1.x 格式文章" })).toBeVisible();
    };

    await selectAndConfirm();
    await expect(page.getByText(/Stellar v2 schema validation failed:/)).toHaveCount(0);
    await page
      .getByRole("alertdialog", { name: "检测到 1.x 格式文章" })
      .getByRole("button", { name: "取消", exact: true })
      .click();
    await expect(page.getByRole("alertdialog", { name: "检测到 1.x 格式文章" })).toBeHidden();
    expect(requests).toBe(1);
    expect((await page.request.get(`/api/v1/posts/${slug}`)).status()).toBe(404);
    await expect(legacyCheckbox).not.toBeChecked();

    await selectAndConfirm();
    const conversionDialog = page.getByRole("alertdialog", { name: "检测到 1.x 格式文章" });
    await expect(conversionDialog).toContainText("未指定状态的新文章保存为草稿");
    await page.screenshot({ path: path.resolve(`docs/baselines/markdown-import-fix/after-conversion-${width}.png`) });
    const converted = page.waitForResponse(
      (r) => r.url().includes("/backup/import/markdown") && r.request().method() === "POST",
    );
    await conversionDialog.getByRole("button", { name: "转换并导入", exact: true }).click();
    const imported = await converted;
    expect(imported.ok(), await imported.text()).toBeTruthy();
    await expect(page.locator("#app").getByText("成功导入 1 篇文章", { exact: true })).toBeVisible();
    expect(requests).toBe(3);
    const saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
    expect(saved.status).toBe("draft");
    expect(saved.content_md).toBe(body);
    const raw = JSON.parse(saved.stellar_config);
    expect(raw.render).toEqual({ math: "mathjax", diagrams: "mermaid" });
    expect(raw.article.style).toBe("tech");
    expect(raw.footer.references).toEqual(["https://example.com/reference"]);
    expect(raw.collection).toEqual({ profile: "topic", id: topicSlug });
    expect(JSON.parse(saved.blog_extensions).content.banner_image).toBe("/banner.jpg");
    expect((await page.request.get(`/post/${slug}`)).status()).toBe(404);
    await expect(legacyCheckbox).toBeChecked();
  });
}

test("普通格式错误只提示一次且不建议旧版转换", async ({ page }) => {
  await login(page);
  await page.goto("/console/backup");
  await page.locator('input[type="file"][accept=".md,.zip"]').setInputFiles({
    name: "invalid.md",
    mimeType: "text/markdown",
    buffer: Buffer.from("---\nmystery: true\n---\nbody"),
  });
  await page.getByRole("alertdialog").getByRole("button", { name: "确定", exact: true }).click();
  await expect(page.getByText(/Stellar v2 schema validation failed:/)).toHaveCount(1);
  await expect(page.getByRole("alertdialog", { name: "检测到 1.x 格式文章" })).toHaveCount(0);
});
