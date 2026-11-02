import { expect, test, type Page, type Response } from "@playwright/test";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

/**
 * BLG-20261102-04：文章管理批量发布 / 批量设为草稿的浏览器验收。
 *
 * 边界声明：
 * - 夹具一律通过真实 Console API 创建（page.request），slug 带时间戳 token，隔离同库其他 spec 的数据。
 * - 验收对象是 Playwright webServer 重建后的当前 Console 产物：证据用例核对入口 HTML 与
 *   包含 /posts/batch 的功能 chunk 的 SHA256 与磁盘 dist 文件一致。
 * - 截图写入 docs/baselines/post-batch-status/（本任务约定的基线归档目录），其余临时产物交给 Playwright。
 * - 只读产品实现：本文件不修改任何实现代码。
 */

const BASELINE_DIR = join(process.cwd(), "docs", "baselines", "post-batch-status");
const EVIDENCE_PATH = join(BASELINE_DIR, "browser-evidence.json");
const DOCK = ".v-floating-dock";
const DIST_DIR = join(process.cwd(), "modules", "console", "web", "dist");
const BATCH_URL = "/api/v1/posts/batch";

test.describe.configure({ timeout: 120_000 });

interface PostFixture {
  slug: string;
  title: string;
}

interface BatchBody {
  succeeded: number;
  skipped: number;
  failed: number;
}

interface ScreenshotMeta {
  file: string;
  width: number;
  height: number;
  bytes: number;
  sha256: string;
}

function sha256(data: Buffer | string): string {
  return createHash("sha256").update(data).digest("hex");
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

async function screenshot(page: Page, name: string): Promise<ScreenshotMeta> {
  const file = join(BASELINE_DIR, name);
  mkdirSync(BASELINE_DIR, { recursive: true });
  const buffer = await page.screenshot({ path: file });
  return {
    file: `docs/baselines/post-batch-status/${name}`,
    width: buffer.readUInt32BE(16),
    height: buffer.readUInt32BE(20),
    bytes: buffer.length,
    sha256: sha256(buffer),
  };
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

/** 通过真实 API 创建隔离文章夹具；token 同时出现在 slug 与标题中，供列表搜索过滤。 */
async function createPost(
  page: Page,
  token: string,
  suffix: string,
  status: "draft" | "published",
): Promise<PostFixture> {
  const slug = `${token}-${suffix}`;
  const title = `批量状态验收 ${token}-${suffix}`;
  const response = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title,
      content_md: `用于 BLG-20261102-04 批量状态验收的夹具正文（${suffix}）。`,
      status,
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(response.ok(), `夹具文章 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
  return { slug, title };
}

async function openPostList(page: Page, token: string, status?: string) {
  await page.goto(status ? `/console/posts?status=${status}` : "/console/posts");
  await expect(page.getByRole("heading", { name: "文章管理", exact: true })).toBeVisible();
  await page.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
}

function rowFor(page: Page, title: string) {
  return page.locator(".post-record").filter({ hasText: title }).first();
}

async function selectRow(page: Page, title: string) {
  const row = rowFor(page, title);
  await expect(row, `列表行必须可见：${title}`).toBeVisible();
  const checkbox = row.locator('input[type="checkbox"]');
  await checkbox.click();
  await expect(checkbox).toBeChecked();
  return row;
}

function batchButton(page: Page, name: string) {
  return page.locator(DOCK).getByRole("button", { name, exact: true });
}

function waitForBatchResponse(page: Page) {
  return page.waitForResponse(
    (response) => response.url().includes(BATCH_URL) && response.request().method() === "POST",
    { timeout: 20_000 },
  );
}

async function readBatchBody(response: Response): Promise<{ status: number; body: BatchBody }> {
  return { status: response.status(), body: (await response.json()) as BatchBody };
}

async function dialogDescription(page: Page): Promise<string> {
  return page.locator("[aria-modal='true'] p").first().innerText();
}

function collectBatchRequests(page: Page): string[] {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes(BATCH_URL) && request.method() === "POST") requests.push(request.url());
  });
  return requests;
}

test("桌面 1280×800：两篇草稿批量发布，再选中单篇批量设为草稿", async ({ page }) => {
  await login(page);
  const token = `pw-batch-desktop-${Date.now()}`;
  const draftA = await createPost(page, token, "draft-a", "draft");
  const draftB = await createPost(page, token, "draft-b", "draft");

  await page.setViewportSize({ width: 1280, height: 800 });
  await openPostList(page, token);
  await expect(page.locator(".post-record")).toHaveCount(2);

  const rowA = rowFor(page, draftA.title);
  const rowB = rowFor(page, draftB.title);
  await expect(rowA.locator(".record-status")).toHaveText("草稿");
  await expect(rowB.locator(".record-status")).toHaveText("草稿");

  await selectRow(page, draftA.title);
  await selectRow(page, draftB.title);

  const dock = page.locator(DOCK);
  await expect(dock).toBeVisible();
  await expect(dock.locator("span.rounded-full").first()).toHaveText("2");

  const publishBtn = batchButton(page, "批量发布");
  const draftBtn = batchButton(page, "批量设为草稿");
  const trashBtn = batchButton(page, "批量移入回收站");
  for (const [label, button] of [
    ["批量发布", publishBtn],
    ["批量设为草稿", draftBtn],
    ["批量移入回收站", trashBtn],
  ] as const) {
    await expect(button, `浮条按钮必须可见：${label}`).toBeVisible();
  }
  // 两篇草稿都已是草稿：发布可执行，设为草稿没有目标（禁用并给出原因），回收站保持可用。
  await expect(publishBtn).toBeEnabled();
  await expect(draftBtn).toBeDisabled();
  await expect(trashBtn).toBeEnabled();
  const draftOnlyDisabledTitle = await draftBtn.getAttribute("title");
  expect(draftOnlyDisabledTitle).toBe("所选文章均已是草稿");

  const desktopDockBox = await dock.boundingBox();
  const dockDesktopShot = await screenshot(page, "dock-desktop-1280.png");

  // 发布：信息型弹窗（role=dialog）+ 主色确认
  await publishBtn.click();
  const publishDialog = page.locator("[aria-modal='true']");
  await expect(publishDialog).toHaveAttribute("role", "dialog");
  await expect(page.getByText("将选中的 2 篇文章设为已发布？", { exact: true })).toBeVisible();
  const publishDescription = await dialogDescription(page);
  const publishConfirmBg = await page
    .locator("[data-dialog-action='confirm']")
    .evaluate((el) => getComputedStyle(el as HTMLElement).backgroundColor);

  const publishResponsePromise = waitForBatchResponse(page);
  await page.locator("[data-dialog-action='confirm']").click();
  const publishResult = await readBatchBody(await publishResponsePromise);
  expect(publishResult.status).toBe(200);
  expect(publishResult.body).toEqual({ succeeded: 2, skipped: 0, failed: 0 });

  await expect(page.locator(".apple-toast-item").filter({ hasText: "已发布 2 篇文章" })).toBeVisible();
  await expect(dock).toHaveCount(0);
  await expect(rowA.locator(".record-status")).toHaveText("已发布");
  await expect(rowB.locator(".record-status")).toHaveText("已发布");
  const afterPublishedShot = await screenshot(page, "after-published.png");

  // 再选中其中 1 篇设为草稿：警告型弹窗（role=alertdialog）+ 次级确认
  await selectRow(page, draftA.title);
  await expect(dock).toBeVisible();
  await expect(dock.locator("span.rounded-full").first()).toHaveText("1");
  await expect(publishBtn).toBeDisabled();
  await expect(draftBtn).toBeEnabled();

  await draftBtn.click();
  const draftDialog = page.locator("[aria-modal='true']");
  await expect(draftDialog).toHaveAttribute("role", "alertdialog");
  await expect(page.getByText("将选中的 1 篇文章设为草稿？", { exact: true })).toBeVisible();
  const draftDescription = await dialogDescription(page);
  const draftConfirmBg = await page
    .locator("[data-dialog-action='confirm']")
    .evaluate((el) => getComputedStyle(el as HTMLElement).backgroundColor);
  expect(draftConfirmBg, "设为草稿的确认按钮必须是次级样式，与发布的主色确认不同").not.toBe(publishConfirmBg);

  const draftResponsePromise = waitForBatchResponse(page);
  await page.locator("[data-dialog-action='confirm']").click();
  const draftResult = await readBatchBody(await draftResponsePromise);
  expect(draftResult.status).toBe(200);
  expect(draftResult.body).toEqual({ succeeded: 1, skipped: 0, failed: 0 });

  await expect(page.locator(".apple-toast-item").filter({ hasText: "已设为草稿 1 篇" })).toBeVisible();
  await expect(dock).toHaveCount(0);
  await expect(rowA.locator(".record-status")).toHaveText("草稿");
  await expect(rowB.locator(".record-status")).toHaveText("已发布");
  const afterDraftShot = await screenshot(page, "after-draft.png");

  recordEvidence("desktop-roundtrip-1280x800", {
    viewport: { width: 1280, height: 800 },
    fixtures: [draftA, draftB],
    dock: {
      box: desktopDockBox,
      countPill: "2",
      buttons: ["批量发布", "批量设为草稿", "批量移入回收站"],
      publishEnabled: true,
      draftEnabled: false,
      draftDisabledTitle: draftOnlyDisabledTitle,
      trashEnabled: true,
      screenshot: dockDesktopShot,
    },
    publish: {
      dialogRole: "dialog",
      dialogTitle: "将选中的 2 篇文章设为已发布？",
      dialogDescription: publishDescription,
      confirmBackground: publishConfirmBg,
      response: publishResult,
      toast: "已发布 2 篇文章",
      statusLabelsAfter: ["已发布", "已发布"],
      selectionCleared: true,
      screenshot: afterPublishedShot,
    },
    draft: {
      dialogRole: "alertdialog",
      dialogTitle: "将选中的 1 篇文章设为草稿？",
      dialogDescription: draftDescription,
      confirmBackground: draftConfirmBg,
      response: draftResult,
      toast: "已设为草稿 1 篇",
      statusLabelsAfter: { [draftA.title]: "草稿", [draftB.title]: "已发布" },
      selectionCleared: true,
      screenshot: afterDraftShot,
    },
  });
});

test("已发布选择时「批量发布」禁用并给出 title，混合选择按目标状态过滤", async ({ page }) => {
  await login(page);
  const token = `pw-batch-disabled-${Date.now()}`;
  const published = await createPost(page, token, "published", "published");
  const draft = await createPost(page, token, "draft", "draft");

  await page.setViewportSize({ width: 1280, height: 800 });
  await openPostList(page, token);
  await expect(page.locator(".post-record")).toHaveCount(2);
  await expect(rowFor(page, published.title).locator(".record-status")).toHaveText("已发布");
  await expect(rowFor(page, draft.title).locator(".record-status")).toHaveText("草稿");

  const batchRequests = collectBatchRequests(page);
  await selectRow(page, published.title);
  const dock = page.locator(DOCK);
  await expect(dock).toBeVisible();

  const publishBtn = batchButton(page, "批量发布");
  const draftBtn = batchButton(page, "批量设为草稿");
  await expect(publishBtn).toBeDisabled();
  const disabledTitle = await publishBtn.getAttribute("title");
  expect(disabledTitle, "已发布选择的禁用原因必须是实际文案").toBe("所选文章均已发布");
  await expect(draftBtn).toBeEnabled();

  // 加入一篇草稿后，发布目标恰好剩下这篇草稿；按钮恢复可用并移除 title。
  await selectRow(page, draft.title);
  await expect(publishBtn).toBeEnabled();
  expect(await publishBtn.getAttribute("title"), "可执行时不应保留禁用原因 title").toBeNull();
  await expect(dock.locator("span.rounded-full").first()).toHaveText("2");

  await publishBtn.click();
  await expect(page.getByText("将选中的 1 篇文章设为已发布？", { exact: true })).toBeVisible();
  const description = await dialogDescription(page);
  await page.locator("[data-dialog-action='cancel']").click();
  await expect(page.locator("[aria-modal='true']")).toHaveCount(0);
  await page.waitForTimeout(500);
  expect(batchRequests, "取消确认不得发出 batch 请求").toEqual([]);

  recordEvidence("disabled-and-target-filter", {
    viewport: { width: 1280, height: 800 },
    publishedFixture: published,
    draftFixture: draft,
    publishedOnlySelection: {
      publishDisabled: true,
      publishTitle: disabledTitle,
      draftEnabled: true,
    },
    mixedSelection: {
      publishEnabled: true,
      publishTitle: null,
      dialogTitle: "将选中的 1 篇文章设为已发布？",
      dialogDescription: description,
      countPill: "2",
    },
  });
});

test("回收站选择时两个状态按钮均禁用并给出原因", async ({ page }) => {
  await login(page);
  const token = `pw-batch-trashed-${Date.now()}`;
  const trashed = await createPost(page, token, "trashed", "published");
  const deleted = await page.request.delete(`/api/v1/posts/${trashed.slug}`);
  expect(deleted.ok(), `夹具文章 ${trashed.slug} 必须能移入回收站（HTTP ${deleted.status()}）`).toBe(true);

  await page.setViewportSize({ width: 1280, height: 800 });
  await openPostList(page, token, "trashed");
  await expect(page.locator(".post-record")).toHaveCount(1);
  const row = rowFor(page, trashed.title);
  await expect(row.locator(".record-status")).toHaveText("回收站");
  await selectRow(page, trashed.title);

  const dock = page.locator(DOCK);
  await expect(dock).toBeVisible();
  const publishBtn = batchButton(page, "批量发布");
  const draftBtn = batchButton(page, "批量设为草稿");
  const trashBtn = batchButton(page, "批量移入回收站");

  await expect(publishBtn).toBeDisabled();
  await expect(draftBtn).toBeDisabled();
  const publishTitle = await publishBtn.getAttribute("title");
  const draftTitle = await draftBtn.getAttribute("title");
  expect(publishTitle).toBe("回收站文章需先恢复再设置状态");
  expect(draftTitle).toBe("回收站文章需先恢复再设置状态");
  await expect(trashBtn).toBeEnabled();

  recordEvidence("trashed-selection-disabled", {
    viewport: { width: 1280, height: 800 },
    fixture: trashed,
    statusLabel: "回收站",
    publishDisabled: true,
    draftDisabled: true,
    publishTitle,
    draftTitle,
    trashButtonEnabled: true,
  });
});

test("取消确认不发 batch 请求且保留选择，确认后才发起一次请求", async ({ page }) => {
  await login(page);
  const token = `pw-batch-cancel-${Date.now()}`;
  const draft = await createPost(page, token, "draft", "draft");

  await page.setViewportSize({ width: 1280, height: 800 });
  await openPostList(page, token);
  await expect(page.locator(".post-record")).toHaveCount(1);
  const row = await selectRow(page, draft.title);

  const batchRequests = collectBatchRequests(page);
  const dock = page.locator(DOCK);
  const publishBtn = batchButton(page, "批量发布");
  await expect(dock).toBeVisible();

  await publishBtn.click();
  await expect(page.getByText("将选中的 1 篇文章设为已发布？", { exact: true })).toBeVisible();
  await page.locator("[data-dialog-action='cancel']").click();
  await expect(page.locator("[aria-modal='true']")).toHaveCount(0);
  await page.waitForTimeout(600);
  expect(batchRequests, "点取消不得发出 batch 请求").toEqual([]);
  await expect(dock).toBeVisible();
  await expect(dock.locator("span.rounded-full").first()).toHaveText("1");
  await expect(row.locator('input[type="checkbox"]')).toBeChecked();
  await expect(row.locator(".record-status")).toHaveText("草稿");

  // 再打开并确认：请求必须只由确认动作发起。
  await publishBtn.click();
  await expect(page.getByText("将选中的 1 篇文章设为已发布？", { exact: true })).toBeVisible();
  const responsePromise = waitForBatchResponse(page);
  await page.locator("[data-dialog-action='confirm']").click();
  const result = await readBatchBody(await responsePromise);
  expect(result.status).toBe(200);
  expect(result.body).toEqual({ succeeded: 1, skipped: 0, failed: 0 });
  expect(batchRequests).toHaveLength(1);
  await expect(row.locator(".record-status")).toHaveText("已发布");

  recordEvidence("cancel-keeps-selection", {
    viewport: { width: 1280, height: 800 },
    fixture: draft,
    requestsAfterCancel: 0,
    selectionAfterCancel: { dockVisible: true, countPill: "1", checkboxChecked: true, statusLabel: "草稿" },
    requestsAfterConfirm: batchRequests.length,
    confirmResponse: result,
    statusLabelAfterConfirm: "已发布",
  });
});

test("移动 390×844：浮条折行、三个按钮可见可点且无横向溢出", async ({ page }) => {
  await login(page);
  const token = `pw-batch-mobile-${Date.now()}`;
  // 混合状态选择：草稿进「批量发布」目标、已发布进「批量设为草稿」目标，三个按钮同时可点。
  const draft = await createPost(page, token, "mobile-a", "draft");
  const published = await createPost(page, token, "mobile-b", "published");

  await page.setViewportSize({ width: 390, height: 844 });
  await openPostList(page, token);
  await expect(page.locator(".post-record")).toHaveCount(2);
  await selectRow(page, draft.title);
  await selectRow(page, published.title);

  const dock = page.locator(DOCK);
  await expect(dock).toBeVisible();
  await expect(dock.locator("span.rounded-full").first()).toHaveText("2");

  const buttons = {
    批量发布: batchButton(page, "批量发布"),
    批量设为草稿: batchButton(page, "批量设为草稿"),
    批量移入回收站: batchButton(page, "批量移入回收站"),
  };
  for (const [label, button] of Object.entries(buttons)) {
    await expect(button, `移动端浮条按钮必须可见：${label}`).toBeVisible();
    await expect(button, `移动端浮条按钮必须可点：${label}`).toBeEnabled();
  }

  const overflow = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
    bodyScrollWidth: document.body.scrollWidth,
  }));
  expect(
    overflow.scrollWidth,
    `390px 下整页不得横向溢出（scrollWidth=${overflow.scrollWidth}, innerWidth=${overflow.innerWidth}）`,
  ).toBeLessThanOrEqual(overflow.innerWidth + 1);

  const flexWrap = await dock.evaluate((el) => getComputedStyle(el).flexWrap);
  expect(flexWrap, "浮条必须启用 flex-wrap 以在窄屏折行").toBe("wrap");
  const dockBox = await dock.boundingBox();
  expect(dockBox, "浮条必须有完整矩形").not.toBeNull();
  expect(dockBox!.x, "浮条左边界不得超出视口").toBeGreaterThanOrEqual(-1);
  expect(dockBox!.x + dockBox!.width, "浮条右边界不得超出视口").toBeLessThanOrEqual(391);
  // 1280px 单行浮条约 56–60px 高；390px 折行后必须明显更高（实测值写入证据）。
  expect(dockBox!.height, "390px 浮条必须折行（高度大于单行）").toBeGreaterThan(70);

  const buttonBoxes: Record<string, { x: number; y: number; width: number; height: number } | null> = {};
  for (const [label, button] of Object.entries(buttons)) {
    const box = await button.boundingBox();
    expect(box, `按钮必须有完整矩形：${label}`).not.toBeNull();
    expect(box!.x, `${label} 左边界不得超出视口`).toBeGreaterThanOrEqual(-1);
    expect(box!.x + box!.width, `${label} 右边界不得超出视口`).toBeLessThanOrEqual(391);
    buttonBoxes[label] = box;
  }

  const mobileShot = await screenshot(page, "dock-mobile-390.png");

  // 「可点」的完整证据：移动视口下走一次真实批量发布（只发布草稿，已发布被跳过）。
  await buttons["批量发布"].click();
  await expect(page.getByText("将选中的 1 篇文章设为已发布？", { exact: true })).toBeVisible();
  const responsePromise = waitForBatchResponse(page);
  await page.locator("[data-dialog-action='confirm']").click();
  const result = await readBatchBody(await responsePromise);
  expect(result.status).toBe(200);
  expect(result.body).toEqual({ succeeded: 1, skipped: 0, failed: 0 });
  await expect(page.locator(".apple-toast-item").filter({ hasText: "已发布 1 篇文章" })).toBeVisible();
  await expect(page.locator(DOCK)).toHaveCount(0);
  await expect(rowFor(page, draft.title).locator(".record-status")).toHaveText("已发布");
  await expect(rowFor(page, published.title).locator(".record-status")).toHaveText("已发布");

  recordEvidence("mobile-390x844", {
    viewport: { width: 390, height: 844 },
    fixtures: [draft, published],
    dock: {
      box: dockBox,
      buttonBoxes,
      flexWrap,
      countPill: "2",
      heightGreaterThanSingleDesktopRow: true,
      screenshot: mobileShot,
    },
    overflow,
    publishResponse: result,
    toast: "已发布 1 篇文章",
    statusLabelsAfter: { [draft.title]: "已发布", [published.title]: "已发布" },
  });
});

test("证据：浏览器实际加载的 Console 入口与功能 chunk 对应当前构建", async ({ page, browser }) => {
  const scriptResponses: Response[] = [];
  const documentResponses: Response[] = [];
  page.on("response", (response) => {
    const resourceType = response.request().resourceType();
    if (resourceType === "script") scriptResponses.push(response);
    else if (resourceType === "document") documentResponses.push(response);
  });

  await login(page);
  const token = `pw-batch-assets-${Date.now()}`;
  const fixture = await createPost(page, token, "assets", "draft");
  await page.goto("/console/posts");
  await expect(page.getByRole("heading", { name: "文章管理", exact: true })).toBeVisible();
  await page.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
  await expect(page.locator(".post-record")).toHaveCount(1);
  await selectRow(page, fixture.title);
  await expect(page.locator(DOCK)).toBeVisible();

  // 入口 HTML：浏览器最后一次 document 响应的已解码正文。
  const documentResponse = documentResponses[documentResponses.length - 1];
  expect(documentResponse, "必须捕获浏览器实际加载的 Console 入口文档响应").toBeTruthy();
  const entryResponse = await page.request.get("/console/", { headers: { "Accept-Encoding": "identity" } });
  expect(entryResponse.ok()).toBe(true);
  const entryHtml = await entryResponse.text();
  const entryDocumentBody = await documentResponse.body();
  const entryHtmlSha = sha256(entryHtml);
  const entryDocumentSha = sha256(entryDocumentBody);
  const entryUrl = documentResponse.url();
  const scriptSources = Array.from(entryHtml.matchAll(/<script[^>]+src="([^"]+)"/g)).map((match) => match[1]);
  const modulePreloads = Array.from(entryHtml.matchAll(/<link[^>]+rel="modulepreload"[^>]+href="([^"]+)"/g)).map(
    (match) => match[1],
  );

  let diskEntrySha: string | null = null;
  try {
    diskEntrySha = sha256(readFileSync(join(DIST_DIR, "index.html")));
  } catch {
    diskEntrySha = null;
  }
  expect(diskEntrySha, "磁盘 dist/index.html 必须存在（证明本次 E2E 已重建 Console）").not.toBeNull();
  expect(entryHtmlSha, "浏览器入口 HTML 必须与磁盘当前构建的 dist/index.html 一致").toBe(diskEntrySha);
  expect(entryDocumentSha, "浏览器实际收到的入口文档 SHA256 必须与入口 HTML 一致").toBe(entryHtmlSha);

  // 功能 chunk：在浏览器实际加载的 script 响应里找包含 /posts/batch 的当前构建 chunk。
  interface ChunkInfo {
    url: string;
    sha256: string;
    diskPath: string | null;
    diskSha256: string | null;
    matchesDisk: boolean;
    containsBatchEndpoint: boolean;
    containsBatchPublishText: boolean;
  }

  const scriptUrls: string[] = [];
  const batchChunks: ChunkInfo[] = [];
  let uiChunk: ChunkInfo | null = null;

  for (const response of scriptResponses) {
    const url = response.url();
    if (!url.includes("/console/assets/") || scriptUrls.includes(url)) continue;
    scriptUrls.push(url);
    let body: Buffer;
    try {
      body = await response.body();
    } catch {
      continue;
    }
    const text = body.toString("utf8");
    const containsBatchEndpoint = text.includes("/posts/batch");
    const containsBatchPublishText = text.includes("批量发布") || text.includes("\\u6279\\u91cf\\u53d1\\u5e03");
    if (!containsBatchEndpoint && !containsBatchPublishText) continue;

    const pathname = new URL(url).pathname.replace(/^\/console\//, "");
    const diskFile = join(DIST_DIR, pathname);
    let diskSha: string | null = null;
    try {
      diskSha = sha256(readFileSync(diskFile));
    } catch {
      diskSha = null;
    }
    const chunk: ChunkInfo = {
      url,
      sha256: sha256(body),
      diskPath: `modules/console/web/dist/${pathname}`,
      diskSha256: diskSha,
      matchesDisk: diskSha !== null && diskSha === sha256(body),
      containsBatchEndpoint,
      containsBatchPublishText,
    };
    if (containsBatchEndpoint && !batchChunks.some((item) => item.url === url)) batchChunks.push(chunk);
    if (containsBatchPublishText && uiChunk === null) uiChunk = chunk;
  }

  expect(batchChunks.length, "浏览器必须加载包含 /posts/batch 的当前构建 chunk").toBeGreaterThan(0);
  expect(uiChunk, "浏览器必须加载包含「批量发布」UI 文案的 PostList 当前构建 chunk").not.toBeNull();
  for (const chunk of batchChunks) {
    expect(chunk.matchesDisk, `${chunk.url} 的实际加载内容必须与磁盘 dist 同名文件一致`).toBe(true);
  }
  expect(uiChunk!.matchesDisk, `${uiChunk!.url} 的实际加载内容必须与磁盘 dist 同名文件一致`).toBe(true);

  recordEvidence("console-assets", {
    browserVersion: browser.version(),
    browserName: browser.browserType().name(),
    baseURL: "http://127.0.0.1:8096",
    entry: {
      documentUrl: entryUrl,
      scriptSources,
      modulePreloads,
      sha256: entryHtmlSha,
      documentSha256: entryDocumentSha,
      diskSha256: diskEntrySha,
      matchesDisk: diskEntrySha === entryHtmlSha,
    },
    featureChunks: batchChunks,
    uiChunk,
    loadedScriptCount: scriptUrls.length,
  });
});
