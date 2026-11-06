import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";

/**
 * BLG-20261106-03：文库管理从「文章」导入文档的浏览器验收。
 *
 * 边界声明：
 * - 夹具一律通过真实 Console API 创建（page.request），slug 带时间戳 token，隔离同库其他 spec 的数据。
 * - 验收对象是 Playwright webServer 重建后的当前 Console 产物：入口 HTML 必须与磁盘 dist/index.html 一致，
 *   页面里必须出现新入口「导入文章」。
 * - 截图与实测记录写入 docs/baselines/wiki-post-import/（本任务约定的基线归档目录）。
 * - 只读产品实现：本文件不修改任何实现代码。
 */

const BASELINE_DIR = join(process.cwd(), "docs", "baselines", "wiki-post-import");
const EVIDENCE_PATH = join(BASELINE_DIR, "browser-evidence.json");
const DIST_DIR = join(process.cwd(), "modules", "console", "web", "dist");

test.describe.configure({ timeout: 120_000 });

interface PostFixture {
  slug: string;
  title: string;
  body: string;
  status: "published" | "draft";
}

interface WikiProjectFixture {
  id: string;
  title: string;
}

function sha256(data: Buffer | string): string {
  return createHash("sha256").update(data).digest("hex");
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function createPost(
  page: Page,
  token: string,
  suffix: string,
  status: "published" | "draft",
  body: string,
): Promise<PostFixture> {
  const slug = `${token}-${suffix}`;
  const title = `导入验收 ${token}-${suffix}`;
  const response = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title,
      content_md: body,
      status,
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(response.ok(), `夹具文章 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { slug, title, body, status };
}

async function createWikiProject(page: Page, id: string, title: string): Promise<WikiProjectFixture> {
  const response = await page.request.post("/api/v1/wiki/projects", {
    data: { id, title, name: title, visible: true, stellar_version: 2, stellar_config: "{}", blog_extensions: "{}" },
  });
  expect(response.ok(), `夹具文库项目 ${id} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { id, title };
}

async function createParentDoc(page: Page, projectId: string, slug: string, title: string): Promise<number> {
  const response = await page.request.post(`/api/v1/wiki/${projectId}/docs`, {
    data: {
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
      slug,
      title,
      content_md: "",
      status: "published",
    },
  });
  expect(response.ok(), `父文档 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  const { id } = (await response.json()) as { id: number };
  return id;
}

/** 打开文库页并选中指定项目，等待文档列表加载完成（空项目显示「暂无文档」）。 */
async function openWikiProject(page: Page, project: WikiProjectFixture) {
  await page.goto("/console/wiki");
  await expect(page.getByRole("heading", { name: "文库管理", exact: true })).toBeVisible();
  const projectSelect = page.getByTitle("选择文库项目");
  await expect(projectSelect.locator(`option[value="${project.id}"]`)).toHaveCount(1, { timeout: 15_000 });
  await projectSelect.selectOption(project.id);
  await expect(page.locator(".page-header").getByRole("button", { name: "导入文章", exact: true })).toBeEnabled();
}

function importModal(page: Page) {
  return page.getByRole("dialog", { name: /导入文章到/ });
}

async function openImportModal(page: Page) {
  const modal = importModal(page);
  if (await modal.isVisible()) return modal;
  await page.locator(".page-header").getByRole("button", { name: "导入文章", exact: true }).click();
  await expect(modal).toBeVisible();
  return modal;
}

function postRow(modal: ReturnType<typeof importModal>, title: string) {
  // 只匹配文章多选列表里的行：父文档下拉的 <label> 也会包含文档标题，不能共用。
  return modal.locator("[data-wiki-import-list] li label").filter({ hasText: title });
}

async function screenshot(page: Page, name: string) {
  mkdirSync(BASELINE_DIR, { recursive: true });
  const buffer = await page.screenshot({ fullPage: false });
  writeFileSync(join(BASELINE_DIR, name), buffer);
  const viewport = page.viewportSize() ?? { width: 0, height: 0 };
  return {
    file: name,
    viewport,
    bytes: buffer.length,
    sha256: sha256(buffer),
  };
}

/** 每个用例把自己的实测值合并进同一份原始证据 JSON（按用例 key 覆盖，避免并行写坏）。 */
function recordEvidence(key: string, payload: Record<string, unknown>) {
  mkdirSync(BASELINE_DIR, { recursive: true });
  let doc: { generatedAt?: string; tests?: Record<string, unknown> } = {};
  try {
    doc = JSON.parse(readFileSync(EVIDENCE_PATH, "utf8")) as typeof doc;
  } catch {
    // 首次运行没有证据文件，直接新建。
  }
  const generatedAt = new Date().toISOString();
  doc.generatedAt = generatedAt;
  doc.tests = { ...(doc.tests ?? {}), [key]: { recordedAt: generatedAt, ...payload } };
  writeFileSync(EVIDENCE_PATH, `${JSON.stringify(doc, null, 2)}\n`, "utf8");
}

/** 入口 HTML 必须与磁盘当前构建一致；页面里出现新入口才能证明浏览器加载了本次产物。 */
test("浏览器加载当前 Console 产物：入口 HTML 与 dist 一致且文库页带「导入文章」", async ({ page }) => {
  await login(page);
  const entryResponse = await page.goto("/console/wiki");
  expect(entryResponse, "入口文档必须有响应").not.toBeNull();
  const entryHtml = await entryResponse!.text();
  const diskEntrySha = sha256(readFileSync(join(DIST_DIR, "index.html")));
  const entrySha = sha256(entryHtml);
  expect(entrySha, "浏览器入口 HTML 必须与磁盘当前构建的 dist/index.html 一致").toBe(diskEntrySha);

  await expect(page.getByRole("heading", { name: "文库管理", exact: true })).toBeVisible();
  await expect(page.locator(".page-header").getByRole("button", { name: "导入文章", exact: true })).toBeVisible();

  // 页面实际加载的入口 JS 也必须是磁盘产物（用入口 HTML 里的 modulepreload/script URL 对照）。
  const assetUrls = Array.from(entryHtml.matchAll(/(?:src|href)="([^"]*\/console\/assets\/[^"]+)"/g)).map(
    (match) => match[1],
  );
  expect(assetUrls.length, "入口 HTML 必须引用构建后的 assets").toBeGreaterThan(0);
  const checkedAssets: { url: string; matchesDisk: boolean; sha256: string }[] = [];
  for (const url of assetUrls.slice(0, 4)) {
    const response = await page.request.get(url);
    expect(response.ok(), `入口资源 ${url} 必须可取`).toBe(true);
    const body = Buffer.from(await response.body());
    const pathname = new URL(url, "http://127.0.0.1").pathname.replace(/^\/console\//, "");
    const sha = sha256(body);
    let matchesDisk = false;
    try {
      matchesDisk = sha256(readFileSync(join(DIST_DIR, pathname))) === sha;
    } catch {
      matchesDisk = false;
    }
    checkedAssets.push({ url, matchesDisk, sha256: sha });
  }
  expect(
    checkedAssets.every((asset) => asset.matchesDisk),
    "页面引用的入口资源必须与磁盘构建逐字一致",
  ).toBe(true);

  recordEvidence("console-assets", {
    viewport: page.viewportSize(),
    entryHtml: { sha256: entrySha, diskSha256: diskEntrySha, matchesDisk: true },
    assets: checkedAssets,
    pageShowsImportEntry: true,
  });
});

test("桌面 1280×800：从文章导入两篇为草稿，文档树/编辑器/API 一致，原文章不变且草稿不公开", async ({ page }) => {
  await login(page);
  const token = `pw-wiki-import-${Date.now()}`;
  const project = await createWikiProject(page, `pw-wiki-${token}`, `导入验收文库 ${token}`);
  const first = await createPost(page, token, "quick-start", "published", "# 快速开始\n\n导入验收正文甲。\n");
  const second = await createPost(page, token, "deep-dive", "draft", "# 深入指南\n\n导入验收正文乙。\n");

  await page.setViewportSize({ width: 1280, height: 800 });
  await openWikiProject(page, project);
  const modal = await openImportModal(page);
  await expect(modal).toContainText("只复制标题与 Markdown 正文");
  await expect(modal).toContainText(project.title);

  await modal.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
  await expect(modal.getByText("共 2 条")).toBeVisible();
  const firstRow = postRow(modal, first.title);
  const secondRow = postRow(modal, second.title);
  await expect(firstRow).toBeVisible();
  await expect(secondRow).toBeVisible();
  await expect(firstRow).toContainText("已发布");
  await expect(secondRow).toContainText("草稿");

  await firstRow.locator('input[type="checkbox"]').check();
  await secondRow.locator('input[type="checkbox"]').check();
  await expect(modal.getByText("已选 2 篇")).toBeVisible();
  const selectedShot = await screenshot(page, "modal-selected-2.png");

  const importResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes(`/api/v1/wiki/${project.id}/docs/import`) && response.request().method() === "POST",
    { timeout: 20_000 },
  );
  await modal.getByRole("button", { name: "导入 2 篇", exact: true }).click();
  const importResponse = await importResponsePromise;
  expect(importResponse.status()).toBe(200);
  const importBody = (await importResponse.json()) as {
    imported: { slug: string; post_slug: string; title: string; status?: string }[];
    skipped: unknown[];
    failed: unknown[];
  };
  expect(importBody.imported.map((item) => item.slug)).toEqual([first.slug, second.slug]);
  expect(importBody.skipped).toEqual([]);
  expect(importBody.failed).toEqual([]);

  await expect(modal.getByText("已导入 2 篇")).toBeVisible();
  const resultShot = await screenshot(page, "result-panel.png");
  await modal.getByRole("button", { name: "完成", exact: true }).click();
  await expect(modal).toBeHidden();

  // 树里出现两篇文档；导入后自动打开第一篇文档。
  await expect(page.getByRole("button", { name: /全部文档/ })).toContainText("2 篇");
  await expect(page.getByPlaceholder("输入文档标题...")).toHaveValue(first.title);

  const listResponse = await page.request.get(`/api/v1/wiki/${project.id}/docs`);
  const list = (await listResponse.json()) as {
    docs: { slug: string; title: string; status: string; content_md: string; parent: number }[];
  };
  expect(list.docs).toHaveLength(2);
  const docsBySlug = new Map(list.docs.map((doc) => [doc.slug, doc]));
  for (const fixture of [first, second]) {
    const doc = docsBySlug.get(fixture.slug);
    expect(doc, `${fixture.slug} 必须已写入项目`).toBeTruthy();
    expect(doc!.title).toBe(fixture.title);
    expect(doc!.content_md).toBe(fixture.body);
    expect(doc!.status).toBe("draft");
    expect(doc!.parent).toBe(0);
  }
  await expect(page.locator(".apple-toast-item")).toHaveCount(0, { timeout: 8_000 });
  const afterImportShot = await screenshot(page, "docs-after-import.png");

  // 原文章只读：正文与状态不变；草稿文档不在公开面。
  for (const fixture of [first, second]) {
    const post = (await (await page.request.get(`/api/v1/posts/${fixture.slug}`)).json()) as {
      content_md: string;
      status: string;
    };
    expect(post.content_md).toBe(fixture.body);
    expect(post.status).toBe(fixture.status);
    expect((await page.request.get(`/wiki/${project.id}/${fixture.slug}`)).status()).toBe(404);
  }

  recordEvidence("desktop-import-drafts-1280x800", {
    viewport: { width: 1280, height: 800 },
    project,
    fixtures: [first, second],
    modal: {
      title: `导入文章到「${project.title}」`,
      copyHint: "只复制标题与 Markdown 正文",
      optionLabels: ["归属父文档", "导入后状态"],
      rows: [
        { title: first.title, statusLabel: "已发布" },
        { title: second.title, statusLabel: "草稿" },
      ],
      selectedCount: "已选 2 篇",
      screenshot: selectedShot,
    },
    import: {
      status: importResponse.status(),
      imported: importBody.imported.map((item) => ({ slug: item.slug, post_slug: item.post_slug })),
      skipped: importBody.skipped.length,
      failed: importBody.failed.length,
      resultSummary: "已导入 2 篇",
      screenshot: resultShot,
    },
    afterImport: {
      docs: list.docs.map((doc) => ({ slug: doc.slug, title: doc.title, status: doc.status, parent: doc.parent })),
      editorTitle: first.title,
      treeCountLabel: "2 篇",
      screenshot: afterImportShot,
    },
    sourcePostsReadOnly: true,
    draftPublicStatus: 404,
  });
});

test("已发布导入并归属父文档；重复导入显示「已导入」、回收站文章不进入候选", async ({ page }) => {
  await login(page);
  const token = `pw-wiki-import-pub-${Date.now()}`;
  const project = await createWikiProject(page, `pw-wiki-pub-${token}`, `导入验收已发布 ${token}`);
  const parentTitle = `指南目录 ${token}`;
  const parentId = await createParentDoc(page, project.id, "guide", parentTitle);
  const published = await createPost(page, token, "published", "published", "# 发布导入\n\n已发布正文。\n");
  const trashed = await createPost(page, token, "trashed", "published", "# 回收站\n\n不应导入。\n");
  const deleted = await page.request.delete(`/api/v1/posts/${trashed.slug}`);
  expect(deleted.ok(), `夹具文章 ${trashed.slug} 必须能移入回收站（HTTP ${deleted.status()}）`).toBe(true);

  await page.setViewportSize({ width: 1280, height: 800 });
  await openWikiProject(page, project);
  const modal = await openImportModal(page);
  await modal.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
  await expect(modal.getByText("共 1 条")).toBeVisible();
  const publishedRow = postRow(modal, published.title);
  await expect(publishedRow).toBeVisible();
  // 文章列表默认排除回收站：回收站文章不会出现在导入候选里。
  await expect(postRow(modal, trashed.title)).toHaveCount(0);

  await modal.getByLabel("归属父文档").selectOption(String(parentId));
  await modal.getByLabel("导入后状态").selectOption("published");
  await publishedRow.locator('input[type="checkbox"]').check();

  const importResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes(`/api/v1/wiki/${project.id}/docs/import`) && response.request().method() === "POST",
    { timeout: 20_000 },
  );
  await modal.getByRole("button", { name: "导入 1 篇", exact: true }).click();
  const importResponse = await importResponsePromise;
  expect(importResponse.status()).toBe(200);
  await expect(modal.getByText("已导入 1 篇")).toBeVisible();
  await modal.getByRole("button", { name: "完成", exact: true }).click();
  await expect(modal).toBeHidden();

  const doc = (await (await page.request.get(`/api/v1/wiki/${project.id}/docs/${published.slug}`)).json()) as {
    status: string;
    parent: number;
    content_md: string;
  };
  expect(doc.status).toBe("published");
  expect(doc.parent).toBe(parentId);
  expect(doc.content_md).toBe(published.body);
  const publicResponse = await page.request.get(`/wiki/${project.id}/${published.slug}`);
  expect(publicResponse.status(), "已发布导入的文档应可在线访问").toBe(200);

  // 即使绕过界面直接提交回收站文章，后端也只记失败、不写入。
  const rejected = await page.request.post(`/api/v1/wiki/${project.id}/docs/import`, {
    data: { slugs: [trashed.slug], parent: 0, status: "draft" },
  });
  expect(rejected.status()).toBe(200);
  const rejectedBody = (await rejected.json()) as {
    imported: unknown[];
    failed: { post_slug: string; reason: string }[];
  };
  expect(rejectedBody.imported).toHaveLength(0);
  expect(rejectedBody.failed).toHaveLength(1);
  expect(rejectedBody.failed[0].reason).toContain("回收站");

  // 重新打开弹窗：同一篇文章标记「已导入」且不可再选。
  await expect(page.locator(".apple-toast-item")).toHaveCount(0, { timeout: 8_000 });
  const reopened = await openImportModal(page);
  await reopened.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
  await expect(reopened.getByText("共 1 条")).toBeVisible();
  const importedRow = postRow(reopened, published.title);
  await expect(importedRow).toContainText("已导入");
  await expect(importedRow.locator('input[type="checkbox"]')).toBeDisabled();
  await expect(reopened.getByRole("button", { name: "导入 0 篇", exact: true })).toBeDisabled();
  const statesShot = await screenshot(page, "modal-imported-and-trashed.png");
  await reopened.getByRole("button", { name: "取消", exact: true }).click();

  recordEvidence("published-parent-and-duplicate", {
    viewport: { width: 1280, height: 800 },
    project,
    parentDoc: { id: parentId, title: parentTitle },
    imported: { slug: published.slug, status: doc.status, parent: doc.parent, publicStatus: publicResponse.status() },
    trashedPost: { hiddenFromList: true, apiImported: 0, apiFailedReason: rejectedBody.failed[0].reason },
    duplicateRow: { label: "已导入", checkboxDisabled: true, submitDisabled: true },
    screenshot: statesShot,
  });
});

test("移动 390×844：导入弹窗可用、选项完整且无横向溢出", async ({ page }) => {
  await login(page);
  const token = `pw-wiki-import-mobile-${Date.now()}`;
  const project = await createWikiProject(page, `pw-wiki-m-${token}`, `移动导入 ${token}`);
  await createPost(page, token, "mobile-a", "published", "# 移动甲\n\n正文甲。\n");
  await createPost(page, token, "mobile-b", "draft", "# 移动乙\n\n正文乙。\n");

  await page.setViewportSize({ width: 390, height: 844 });
  await openWikiProject(page, project);
  const modal = await openImportModal(page);
  await modal.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
  await expect(modal.getByText("共 2 条")).toBeVisible();
  const rows = modal.locator("[data-wiki-import-list] li label").filter({ hasText: token });
  await expect(rows).toHaveCount(2);
  await expect(modal.getByLabel("归属父文档")).toBeVisible();
  await expect(modal.getByLabel("导入后状态")).toBeVisible();
  await rows.nth(0).locator('input[type="checkbox"]').check();
  await rows.nth(1).locator('input[type="checkbox"]').check();
  await expect(modal.getByText("已选 2 篇")).toBeVisible();

  const noHorizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  expect(noHorizontalOverflow, "导入弹窗在 390px 宽度下不得产生横向滚动").toBe(true);
  const modalBox = await modal.boundingBox();
  expect(modalBox, "弹窗必须有可见盒子").not.toBeNull();
  expect(modalBox!.x, "弹窗左边缘不得越界").toBeGreaterThanOrEqual(0);
  expect(modalBox!.x + modalBox!.width, "弹窗右边缘不得越界").toBeLessThanOrEqual(390 + 1);
  const mobileShot = await screenshot(page, "mobile-modal-390.png");

  recordEvidence("mobile-modal-390x844", {
    viewport: { width: 390, height: 844 },
    project,
    modalBox,
    noHorizontalOverflow,
    labels: { parent: "归属父文档", status: "导入后状态" },
    selectedCount: "已选 2 篇",
    screenshot: mobileShot,
  });
});
