import { expect, test, type Page } from "@playwright/test";

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

function settingsTab(page: Page, label: string) {
  return page.locator('[role="tab"]').filter({ hasText: label });
}

async function openSettingsTab(page: Page, label: string) {
  await settingsTab(page, label).click();
  await expect(settingsTab(page, label)).toHaveAttribute("aria-selected", "true");
}

const basicSaveButton = (page: Page) => page.locator(".settings-basic-actions button[type='submit']");
const postSaveButton = (page: Page) => page.locator(".settings-post-actions button[type='submit']");
const commentSaveButton = (page: Page) => page.locator(".settings-comment-actions button[type='submit']");
const dirtyBadge = (page: Page) => page.locator(".settings-dirty-badge");
const siteTitle = (page: Page) => page.locator("#settings-site-title");

test.describe("设置保存竞态（拦截式行为验收）", () => {
  test("系统设置普通保存期间改表单：请求体对应提交值，页面保留新输入且快照不污染", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
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

  test("主题设置普通保存期间改表单：新输入保留并继续提示未保存", async ({ page }) => {
    await login(page);
    await page.goto("/console/appearance?tab=home");
    const coverTitle = page.locator('input[placeholder="Welcome"]');
    await expect(coverTitle).toBeVisible();

    const gate = await installGate(page, "cover");
    await coverTitle.fill("A-cover-title");
    const saveButton = page.getByRole("button", { name: "保存 Hero" });
    await saveButton.click();
    await gate.waitForRequest(1);
    expect((gate.bodies[0] as Record<string, unknown>).title).toBe("A-cover-title");

    await coverTitle.fill("B-cover-title");
    gate.releaseOk();

    await expect(coverTitle).toHaveValue("B-cover-title");
    await expect(dirtyBadge(page)).toBeVisible();
    await expect(saveButton).toBeEnabled();

    await saveButton.click();
    await expect(dirtyBadge(page)).toHaveCount(0);
  });

  test("普通保存期间编辑 JSON：新草稿不被响应回写覆盖", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
    await expect(siteTitle(page)).toBeVisible();

    const gate = await installGate(page, "site");
    await siteTitle(page).fill("A-normal-json-race");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);

    // 请求未完成时切到高级标签并写入独立 JSON 草稿
    await openSettingsTab(page, "高级");
    await page.locator("#settings-raw-section").selectOption("site");
    const rawTextarea = page.locator("#settings-raw-content");
    const draft = JSON.stringify({ title: "B-json-draft" }, null, 2);
    await rawTextarea.fill(draft);

    gate.releaseOk();

    await expect(rawTextarea).toHaveValue(draft);
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("JSON 保存期间再次编辑 JSON：新文本保留且继续提示未保存", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=advanced");
    await page.locator("#settings-raw-section").selectOption("site");
    const rawTextarea = page.locator("#settings-raw-content");
    await rawTextarea.fill(JSON.stringify({ title: "json-A" }, null, 2));

    const gate = await installGate(page, "site");
    await page.getByRole("button", { name: /保存 site JSON/ }).click();
    await gate.waitForRequest(1);
    expect((gate.bodies[0] as Record<string, unknown>).title).toBe("json-A");

    const laterText = JSON.stringify({ title: "json-B" }, null, 2);
    await rawTextarea.fill(laterText);
    gate.releaseOk();

    await expect(rawTextarea).toHaveValue(laterText);
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("JSON 保存期间切换到表单编辑：新表单输入不被响应覆盖", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=advanced");
    await page.locator("#settings-raw-section").selectOption("site");
    await page.locator("#settings-raw-content").fill(JSON.stringify({ title: "json-C" }, null, 2));

    const gate = await installGate(page, "site");
    await page.getByRole("button", { name: /保存 site JSON/ }).click();
    await gate.waitForRequest(1);

    // 请求未完成时切回基本表单并改标题
    await openSettingsTab(page, "基本");
    await siteTitle(page).fill("form-during-json");
    gate.releaseOk();

    await expect(siteTitle(page)).toHaveValue("form-during-json");
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("保存 A 分区时 B 分区已有修改：B 的输入与提示不变", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
    await expect(siteTitle(page)).toBeVisible();
    await siteTitle(page).fill("A-section-title");

    await openSettingsTab(page, "文章");
    const rewardInput = page.locator("#settings-post-reward-text");
    await rewardInput.fill("B-section-reward");

    await openSettingsTab(page, "基本");
    const gate = await installGate(page, "site");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);
    gate.releaseOk();

    // site 已保存 → 基本/SEO 黄点消失；文章分区仍为未保存
    await expect(settingsTab(page, "基本").locator(".settings-tab-dirty-dot")).toHaveCount(0);
    await expect(settingsTab(page, "文章").locator(".settings-tab-dirty-dot")).toHaveCount(1);
    await expect(dirtyBadge(page)).toBeVisible();

    await openSettingsTab(page, "文章");
    await expect(rewardInput).toHaveValue("B-section-reward");
  });

  test("保存失败并重试：输入不丢失、失败不更新快照、按钮可重试", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
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

  test("无效 JSON 提交：不发送任何保存请求并保留文本", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=advanced");
    await page.locator("#settings-raw-section").selectOption("site");
    const rawTextarea = page.locator("#settings-raw-content");
    const invalidText = '{\n  "title": "broken",\n}';
    await rawTextarea.fill(invalidText);

    const gate = await installGate(page, "site");
    await page.getByRole("button", { name: /保存 site JSON/ }).click();
    await expect(page.locator(".apple-toast-item").first()).toContainText("JSON 格式错误");

    expect(gate.count).toBe(0);
    await expect(rawTextarea).toHaveValue(invalidText);
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("评论组合保存第一步失败：不发送第二步请求", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=comments");
    await page.locator("#settings-comment-title").fill("A-comments-combo");

    const commentsGate = await installGate(page, "comments");
    const treeGate = await installGate(page, "site_tree");
    await commentSaveButton(page).click();
    await commentsGate.waitForRequest(1);

    commentsGate.failCurrent(500);
    await expect(page.locator(".apple-toast-item").first()).toBeVisible();

    expect(treeGate.count).toBe(0);
    await expect(dirtyBadge(page)).toBeVisible();
  });

  test("评论组合保存第二步失败：只保留已成功部分的快照", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=comments");
    // comments 与 site_tree 两个分区都必须先有未保存修改，失败边界才可观察
    await page.locator("#settings-comment-title").fill("A-comments-partial");
    const homeCommentsSwitch = page.getByRole("switch", { name: "首页显示评论区" });
    const switchBefore = await homeCommentsSwitch.getAttribute("aria-checked");
    await homeCommentsSwitch.click();
    await expect(homeCommentsSwitch).toHaveAttribute("aria-checked", switchBefore === "true" ? "false" : "true");

    const commentsGate = await installGate(page, "comments");
    const treeGate = await installGate(page, "site_tree");
    await commentSaveButton(page).click();
    await commentsGate.waitForRequest(1);
    commentsGate.releaseOk();

    await treeGate.waitForRequest(1);
    // 第二步确实提交了被修改的 site_tree 内容
    expect((treeGate.bodies[0] as Record<string, any>).home.comments).toBe(switchBefore !== "true");
    treeGate.failCurrent(500);
    await expect(page.locator(".apple-toast-item").first()).toBeVisible();

    // 第二步失败不得被当作整体成功：site_tree 仍未保存，页面继续提示未保存
    await expect(dirtyBadge(page)).toBeVisible();

    // 已成功部分（comments）的文本基线确实更新为本次提交内容
    await openSettingsTab(page, "高级");
    await page.locator("#settings-raw-section").selectOption("comments");
    await expect(page.locator("#settings-raw-content")).toHaveValue(/A-comments-partial/);
  });

  test("保存中切换分区：其他保存按钮禁用、输入仍可编辑、结束后恢复", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
    await expect(siteTitle(page)).toBeVisible();

    const gate = await installGate(page, "site");
    await siteTitle(page).fill("A-switch-section");
    await basicSaveButton(page).click();
    await gate.waitForRequest(1);

    await openSettingsTab(page, "文章");
    await expect(postSaveButton(page)).toBeDisabled();
    await expect(page.locator("#settings-post-reward-text")).toBeEditable();

    gate.releaseOk();
    await expect(postSaveButton(page)).toBeEnabled();
  });

  test("请求未完成时用 Enter 提交：全局防重入不产生第二个请求", async ({ page }) => {
    await login(page);
    await page.goto("/console/settings?tab=basic");
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
    await page.goto("/console/settings?tab=basic");
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
