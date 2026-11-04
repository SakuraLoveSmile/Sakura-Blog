import { expect, test, type Page } from "@playwright/test";
import { openSettingsPage, settingsNav } from "./support/settings";

/**
 * 设置保存竞态的行为验收（拦截式）。
 *
 * 本文件区分两类证据，禁止互相替代：
 * 1. 受控模拟：拦截 PUT /api/v1/settings/{section}，由测试显式决定响应到达时机与结果，
 *    用于证明「请求未完成期间继续编辑」时输入、草稿与快照的边界行为。
 * 2. 真实服务：最后一个用例不拦截任何响应，走隔离 e2e 服务完成真实持久化并回读。
 *
 * 源码正则检查位于 modules/console/tests/settings-save-race.test.ts，仅作为结构检查保留，
 * 不构成行为验收；禁止把保存函数复制进测试来证明生产代码正确。
 */

type Outcome = { status: number; body?: unknown };

/** 可手动释放的响应等待点：请求到达后挂起，直到测试显式给出结果。 */
function createGate() {
  const bodies: unknown[] = [];
  const waiters: Array<(outcome: Outcome) => void> = [];
  let passThrough = false;
  return {
    bodies,
    get count() {
      return bodies.length;
    },
    async waitForRequest(n: number) {
      await expect
        .poll(() => bodies.length, { timeout: 15_000, message: `等待第 ${n} 个保存请求到达` })
        .toBeGreaterThanOrEqual(n);
    },
    /** 释放当前全部挂起请求为成功；此后同分区请求直接放行成功。 */
    releaseOk() {
      passThrough = true;
      waiters.splice(0).forEach((resolve) => resolve({ status: 200, body: { ok: true } }));
    },
    /** 让当前挂起的请求失败；后续请求继续保持挂起。 */
    failCurrent(status = 500) {
      const resolve = waiters.shift();
      if (!resolve) throw new Error("没有处于挂起状态的保存请求可供失败");
      resolve({ status, body: { error: "e2e induced failure" } });
    },
    takeImmediate(): Outcome | null {
      return passThrough ? { status: 200, body: { ok: true } } : null;
    },
    hold(resolve: (outcome: Outcome) => void) {
      waiters.push(resolve);
    },
  };
}

type Gate = ReturnType<typeof createGate>;

async function installGate(page: Page, section: string): Promise<Gate> {
  const gate = createGate();
  await page.route(`**/api/v1/settings/${section}`, async (route) => {
    if (route.request().method() !== "PUT") {
      await route.fallback();
      return;
    }
    try {
      gate.bodies.push(route.request().postDataJSON());
    } catch {
      gate.bodies.push(null);
    }
    const immediate = gate.takeImmediate();
    const outcome =
      immediate ??
      (await new Promise<Outcome>((resolve) => {
        gate.hold(resolve);
      }));
    await route.fulfill({
      status: outcome.status,
      contentType: "application/json",
      body: JSON.stringify(outcome.body ?? {}),
    });
  });
  return gate;
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

// 站点信息独立保存：保存按钮在本页底部保存栏，主题页共用另一枚“保存全部主题设置”。
const basicSaveButton = (page: Page) => page.locator("#settings-page-site").getByRole("button", { name: "保存", exact: true });
const navDot = (page: Page, key: string) => settingsNav(page, key).locator(".settings-nav-dot");
const dirtyBadge = (page: Page) => page.locator(".settings-dirty-badge");
const siteTitle = (page: Page) => page.locator("#settings-site-title");
// 规范表单（文章、评论等）的字段按 data-setting-path 定位，保存按钮统一为“保存全部主题设置”。
const canonicalField = (page: Page, key: string) => page.locator(`[data-setting-path="${key}"]`).locator("input,textarea");
const canonicalSaveButton = (page: Page) =>
  page.locator(".canonical-settings-form:visible").getByRole("button", { name: "保存全部主题设置", exact: true });

test.describe("设置保存竞态（拦截式行为验收）", () => {
  test("系统设置普通保存期间改表单：请求体对应提交值，页面保留新输入且快照不污染", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=site");
    await expect(siteTitle(page)).toBeVisible();

    const gate = await installGate(page, "site");
    await siteTitle(page).fill("A-race-title");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);
    expect((gate.bodies[0] as Record<string, unknown>).title).toBe("A-race-title");

    // 请求仍未完成时把输入改为 B
    await siteTitle(page).fill("B-race-title");
    await expect(siteTitle(page)).toHaveValue("B-race-title");

    gate.releaseOk();

    // 新输入保留、未保存提示仍在、保存按钮恢复可用
    await expect(siteTitle(page)).toHaveValue("B-race-title");
    await expect(dirtyBadge(page)).toBeVisible();
    await expect(basicSaveButton(page)).toBeEnabled();

    // 再次保存 B 后提示清除
    await basicSaveButton(page).click();
    await expect(dirtyBadge(page)).toHaveCount(0);
  });

  test("保存 A 分区时 B 分区已有修改：B 的输入与提示不变", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=site");
    await expect(siteTitle(page)).toBeVisible();
    await siteTitle(page).fill("A-section-title");

    await openSettingsPage(page, "post");
    const rewardInput = canonicalField(page, "blog_extensions.content.reward.text");
    await rewardInput.fill("B-section-reward");

    await openSettingsPage(page, "site");
    // 站点信息只保存本页，保存栏明确说明主题草稿另有未保存修改。
    await expect(page.locator("#settings-page-site .settings-save-bar")).toContainText("主题草稿另有 1 处未保存");
    const gate = await installGate(page, "site");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);
    gate.releaseOk();

    // site 已保存 → “站点信息”未保存点消失；“文章页”属于主题草稿，仍为未保存
    await expect(navDot(page, "site")).toHaveCount(0);
    await expect(navDot(page, "post")).toHaveCount(1);
    await expect(dirtyBadge(page)).toBeVisible();

    await openSettingsPage(page, "post");
    await expect(rewardInput).toHaveValue("B-section-reward");
  });

  test("保存失败并重试：输入不丢失、失败不更新快照、按钮可重试", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=site");
    await expect(siteTitle(page)).toBeVisible();

    const gate = await installGate(page, "site");
    await siteTitle(page).fill("A-retry-title");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);

    gate.failCurrent(500);
    await expect(page.locator(".apple-toast-item").first()).toBeVisible();

    // 失败不丢失输入、不更新快照（仍为未保存）、按钮恢复可点
    await expect(siteTitle(page)).toHaveValue("A-retry-title");
    await expect(dirtyBadge(page)).toBeVisible();
    await expect(basicSaveButton(page)).toBeEnabled();

    await basicSaveButton(page).click();
    await gate.waitForRequest(2);
    gate.releaseOk();
    await expect(dirtyBadge(page)).toHaveCount(0);
  });

  test("评论分区保存失败：整体保存失败后保留未保存状态", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=comments");
    await canonicalField(page, "blog_extensions.native_comments.title").fill("A-comments-failure");

    const gate = await installGate(page, "stellar");
    await canonicalSaveButton(page).click();
    await gate.waitForRequest(1);

    gate.failCurrent(500);
    // 规范保存失败不弹 Toast，错误写入 Stellar 2 状态区（role=status）。
    await expect(page.locator('[role="status"]:visible').filter({ hasText: "e2e induced failure" })).toBeVisible();
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("评论分区一次规范保存成功：提交全部修改并清除未保存提示", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=comments");
    await canonicalField(page, "blog_extensions.native_comments.title").fill("A-comments-success");

    const gate = await installGate(page, "stellar");
    await canonicalSaveButton(page).click();
    await gate.waitForRequest(1);

    gate.releaseOk();
    await expect(dirtyBadge(page)).toHaveCount(0);
    expect(gate.count).toBe(1);
  });

  test("保存中切换分区：全局保存按钮保持禁用、输入仍可编辑、结束后恢复", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=post");
    await canonicalField(page, "blog_extensions.content.reward.text").fill("A-switch-section");

    const gate = await installGate(page, "stellar");
    await canonicalSaveButton(page).click();
    await gate.waitForRequest(1);

    await openSettingsPage(page, "comments");
    await expect(canonicalSaveButton(page)).toBeDisabled();
    const commentTitle = canonicalField(page, "blog_extensions.native_comments.title");
    await expect(commentTitle).toBeEditable();
    // 保存进行中又产生新修改：按钮仍因“保存中”保持禁用，新输入不丢。
    await commentTitle.fill("B-while-saving");
    await expect(canonicalSaveButton(page)).toBeDisabled();

    gate.releaseOk();
    // 保存栏在没有未保存修改时本来就禁用，所以“结束后恢复”要用保存期间产生的新修改来验证：
    // 请求结束后按钮不再因保存中禁用，且这处新修改保留为未保存、可再次保存。
    await expect(canonicalSaveButton(page)).toBeEnabled();
    await expect(commentTitle).toHaveValue("B-while-saving");
  });

  test("请求未完成时用 Enter 提交：全局防重入不产生第二个请求", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=site");
    await expect(siteTitle(page)).toBeVisible();

    const gate = await installGate(page, "site");
    await siteTitle(page).fill("A-enter-submit");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);

    // 表单隐式提交（Enter）也必须落在同一个防重入判断里
    await siteTitle(page).press("Enter");
    await expect(siteTitle(page)).toHaveValue("A-enter-submit");

    gate.releaseOk();
    expect(gate.count).toBe(1);
  });

  test("隔离服务真实保存：不拦截响应，刷新后重新读取提交值", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=site");
    await expect(siteTitle(page)).toBeVisible();

    const marker = `e2e-persist-${Date.now()}`;
    await siteTitle(page).fill(marker);
    await basicSaveButton(page).click();
    await expect(dirtyBadge(page)).toHaveCount(0);

    // 服务端真实持久化：直接回读 API
    const response = await page.request.get("/api/v1/settings");
    expect(response.ok()).toBe(true);
    const payload = (await response.json()) as { site?: { title?: string } };
    expect(payload.site?.title).toBe(marker);

    // 浏览器刷新后重新读取提交值
    await page.reload();
    await expect(siteTitle(page)).toHaveValue(marker);
    await expect(dirtyBadge(page)).toHaveCount(0);
  });
});
