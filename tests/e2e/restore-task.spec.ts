import { expect, test, type Page } from "@playwright/test";

async function login(page: Page, password = "e2e-password") {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill(password);
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
  await page.goto("/console/backup");
  await expect(page.getByRole("heading", { name: "备份与导出", exact: true })).toBeVisible();
}

async function chooseRestore(page: Page, buffer: Buffer) {
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传备份并恢复", exact: true }).click();
  await (await chooser).setFiles({ name: "restore-test.zip", mimeType: "application/zip", buffer });
  await page.getByPlaceholder("RESTORE", { exact: true }).fill("RESTORE");
}

test("恢复任务维护期间禁用重复提交、刷新续查并按终态解锁", async ({ page }) => {
  await login(page);
  const task = {
    id: "restore-0123456789abcdef0123456789abcdef",
    status: "waiting",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    has_preimage: false,
  };
  let maintaining = true;
  let submissions = 0;
  let detailReads = 0;
  let publicReads = 0;
  await page.route("**/api/maintenance", (route) => {
    publicReads++;
    return route.fulfill({ json: { maintenance: maintaining, retry_after: 5 } });
  });
  await page.route("**/api/v1/backup/restores", (route) => {
    if (route.request().method() === "POST") {
      submissions++;
      return route.fulfill({ status: 202, json: task });
    }
    return route.fulfill({ json: { tasks: [task] } });
  });
  await page.route(`**/api/v1/backup/restores/${task.id}`, (route) => {
    detailReads++;
    return route.fulfill({ json: task });
  });
  await chooseRestore(page, Buffer.from("mock upload"));
  await page.getByRole("button", { name: "确认覆盖并恢复", exact: true }).click();
  await expect(page.getByRole("button", { name: "上传备份并恢复", exact: true })).toBeDisabled();
  await expect.poll(() => publicReads).toBeGreaterThan(0);
  expect(detailReads).toBe(0);
  expect(submissions).toBe(1);
  await expect(page.getByText("恢复任务已提交，系统将在维护完成后更新结果", { exact: true })).toBeHidden({
    timeout: 10_000,
  });
  for (const width of [390, 768, 1440]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.locator("main.content").evaluate((element) => element.scrollTo(0, 0));
    await page.evaluate(() => window.scrollTo(0, 0));
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
    for (const label of await page.locator(".stats-label").all()) {
      const box = await label.boundingBox();
      expect(box?.height).toBeLessThanOrEqual(40);
    }
    const screenshotDir = process.env.BLOG_E2E_SCREENSHOT_DIR || "docs/baselines/worktree-consolidation";
    await page.screenshot({ path: `${screenshotDir}/active-${width}.png`, fullPage: true });
  }
  await page.reload();
  await expect(page.getByRole("button", { name: "上传备份并恢复", exact: true })).toBeDisabled();
  expect(detailReads).toBe(0);
  task.status = "succeeded";
  task.has_preimage = true;
  maintaining = false;
  await expect(page.getByRole("button", { name: "上传备份并恢复", exact: true })).toBeEnabled({ timeout: 10_000 });
  await expect.poll(() => page.evaluate(() => localStorage.getItem("console.restore.task_id"))).toBeNull();
  expect(submissions).toBe(1);
});

test("真实整站恢复撤销当前凭据，使用备份账号登录后查询成功任务", async ({ page }) => {
  await login(page);
  const exported = await page.request.get("/api/v1/backup/export");
  expect(exported.ok()).toBeTruthy();
  const archive = await exported.body();
  const extra = `after-backup-${Date.now()}`;
  expect(
    (
      await page.request.post("/api/v1/posts", {
        data: { slug: extra, title: "备份之后的新文章", content_md: "将被恢复移除", status: "draft" },
      })
    ).status(),
  ).toBe(201);
  const password = await page.request.put("/api/v1/auth/password", {
    data: { current: "e2e-password", new: "temporary-restore-password" },
  });
  expect(password.ok()).toBeTruthy();
  expect(
    (
      await page.request.post("/api/v1/auth/login", {
        data: { username: "admin", password: "temporary-restore-password" },
      })
    ).ok(),
  ).toBeTruthy();
  await chooseRestore(page, archive);
  const submitted = page.waitForResponse(
    (response) => response.url().endsWith("/api/v1/backup/restores") && response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "确认覆盖并恢复", exact: true }).click();
  const response = await submitted;
  expect(response.status()).toBe(202);
  const task = (await response.json()) as { id: string };
  await expect(page.getByRole("button", { name: "去备份账号登录", exact: true })).toBeVisible({ timeout: 30_000 });
  expect(await page.evaluate(() => localStorage.getItem("console.restore.task_id"))).toBe(task.id);
  await page.getByRole("button", { name: "去备份账号登录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/login/);
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).not.toHaveURL(/\/console\/login/);
  const result = await page.request.get(`/api/v1/backup/restores/${task.id}`);
  expect(result.ok()).toBeTruthy();
  expect(await result.json()).toMatchObject({ id: task.id, status: "succeeded", has_preimage: true });
  expect((await page.request.get(`/api/v1/posts/${extra}`)).status()).toBe(404);
});
