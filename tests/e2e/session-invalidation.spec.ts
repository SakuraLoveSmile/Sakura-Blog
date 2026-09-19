import { expect, test } from "@playwright/test";

for (const width of [390, 768, 1440]) {
  test(`Console 401 清空身份后可用键盘返回登录（${width}px）`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/console/login");
    await page.getByPlaceholder("请输入用户名").fill("admin");
    await page.getByPlaceholder("请输入密码").fill("e2e-password");
    await page.getByRole("button", { name: "登 录", exact: true }).click();
    await expect(page).toHaveURL(/\/console\/dashboard/);

    // 保持 SPA 已登录状态，再让受保护请求返回 401，复现路由守卫曾把登录跳回后台的问题。
    await page.route("**/api/v1/profile", (route) => route.fulfill({ status: 401, json: { error: "未认证" } }));
    await page.evaluate(() => {
      window.history.pushState({}, "", "/console/profile");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    const login = page.getByRole("button", { name: "去登录", exact: true });
    await expect(login).toBeVisible();
    await login.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/console\/login/);
    await expect(page.getByPlaceholder("请输入用户名")).toBeVisible();
  });
}

test("UC 401 清空身份并提示重新登录，不被守卫跳回文章页", async ({ page }) => {
  const admin = await page.request.post("/api/v1/auth/login", {
    data: { username: "admin", password: "e2e-password" },
  });
  expect(admin.ok()).toBeTruthy();
  const username = `expired-${Date.now()}`;
  const create = await page.request.post("/api/v1/users", {
    data: { username, password: "e2e-author-password", role: "author" },
  });
  expect(create.ok()).toBeTruthy();
  await page.context().clearCookies();
  await page.goto("/uc/login");
  await page.getByLabel("用户名", { exact: true }).fill(username);
  await page.getByLabel("密码", { exact: true }).fill("e2e-author-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page).toHaveURL(/\/uc\/posts/);
  await page.route("**/api/v1/profile", (route) => route.fulfill({ status: 401, json: { error: "未认证" } }));
  await page.getByRole("link", { name: "资料", exact: true }).click();
  await expect(page).toHaveURL(/\/uc\/login/);
  await expect(page.getByText("登录已过期，请重新登录。", { exact: true })).toBeVisible();
  await expect(page.getByLabel("用户名", { exact: true })).toBeVisible();
});

test("读者真实封禁后升级请求清空 UC 登录态", async ({ page, request }) => {
  expect(
    (
      await request.post("/api/v1/auth/login", {
        data: { username: "admin", password: "e2e-password" },
      })
    ).ok(),
  ).toBeTruthy();
  const username = `banned-${Date.now()}`;
  const create = await request.post("/api/v1/users", {
    data: { username, password: "e2e-reader-password", role: "reader" },
  });
  expect(create.ok()).toBeTruthy();
  const user = (await create.json()) as { id: number };
  await page.goto("/uc/login");
  await page.getByLabel("用户名", { exact: true }).fill(username);
  await page.getByLabel("密码", { exact: true }).fill("e2e-reader-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page).toHaveURL(/\/uc\/upgrade/);
  expect(
    (
      await request.put(`/api/v1/comment-users/${user.id}/status`, {
        data: { status: "banned" },
      })
    ).ok(),
  ).toBeTruthy();
  await page.getByLabel("当前密码", { exact: true }).fill("e2e-reader-password");
  await page.getByRole("button", { name: "确认升级", exact: true }).click();
  await expect(page.getByText("请先登录", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "稍后处理", exact: true }).click();
  await expect(page).toHaveURL(/\/uc\/login/);
});
