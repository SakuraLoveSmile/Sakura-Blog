import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}

test("首页草稿入口、状态筛选与浏览器返回保持一致", async ({ page }) => {
  await login(page);
  const token = `workspace-${Date.now()}`;
  for (const status of ["draft", "published"]) {
    const response = await page.request.post("/api/v1/posts", {
      data: {
        slug: `${token}-${status}`,
        title: `${token} ${status}`,
        content_md: "写作夹具",
        status,
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(response.ok()).toBeTruthy();
  }
  await page.getByRole("button", { name: "继续写草稿", exact: true }).click();
  await expect(page).toHaveURL(/status=draft/);
  const statusGroup = page.getByRole("group", { name: "文章状态" });
  await expect(statusGroup.getByRole("button", { name: "草稿", exact: true })).toHaveAttribute("aria-pressed", "true");
  await statusGroup.getByRole("button", { name: "全部", exact: true }).click();
  await expect(page).not.toHaveURL(/status=draft/);
  await expect(statusGroup.getByRole("button", { name: "全部", exact: true })).toHaveAttribute("aria-pressed", "true");
  await page.goBack();
  await expect(statusGroup.getByRole("button", { name: "草稿", exact: true })).toHaveAttribute("aria-pressed", "true");
  await page.getByLabel("搜索文章", { exact: true }).fill(`${token}-no-match`);
  await expect(page.getByRole("heading", { name: "未找到匹配的文章" })).toBeVisible();
  await page.getByRole("button", { name: "清除筛选", exact: true }).first().click();
  await expect(page.getByLabel("搜索文章", { exact: true })).toHaveValue("");
  await expect(statusGroup.getByRole("button", { name: "全部", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator(".post-card").filter({ hasText: `${token} published` })).toBeVisible();
});

test("手机底栏保持五项，其余内容入口仍可达", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  const nav = page.getByRole("navigation", { name: "移动端底部导航" });
  await expect(nav.getByRole("link")).toHaveCount(4);
  await expect(nav.getByRole("link", { name: "媒体库", exact: true })).toBeVisible();
  await nav.getByRole("button", { name: "更多导航" }).click();
  const drawer = page.getByRole("dialog", { name: "全部菜单" });
  await expect(drawer.getByRole("link", { name: "页面", exact: true })).toBeVisible();
  await drawer.getByRole("link", { name: "页面", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/pages/);
  await expect(drawer).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
});

test("文章视图偏好刷新后保留，滚动时页头操作可用", async ({ page }) => {
  await login(page);
  for (let i = 0; i < 8; i++) {
    const response = await page.request.post("/api/v1/posts", {
      data: {
        slug: `workspace-scroll-${Date.now()}-${i}`,
        title: `文章 ${i}`,
        content_md: "正文",
        status: "draft",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(response.ok()).toBeTruthy();
  }
  await page.goto("/console/posts");
  await page.getByRole("button", { name: "表格视图", exact: true }).click();
  await page.reload();
  await expect(page.getByRole("button", { name: "表格视图", exact: true })).toHaveAttribute("aria-pressed", "true");
  await page.locator("main.content").evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  const header = page.locator("main.content header");
  await expect(header.getByRole("button", { name: "撰写文章", exact: true })).toBeVisible();
  await expect(header).not.toHaveAttribute("aria-hidden", "true");
});

test("全局搜索菜单点击可跳转并关闭弹窗", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: "搜索", exact: true }).click();
  await page.getByPlaceholder(/搜索菜单/).fill("分类标签");
  await page.locator("[data-search-index]").filter({ hasText: "分类标签" }).click();
  await expect(page).toHaveURL(/\/console\/taxonomy$/);
  await expect(page.getByPlaceholder(/搜索菜单/)).toBeHidden();
});
