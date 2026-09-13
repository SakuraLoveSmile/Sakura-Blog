import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

test.describe("Console UI 一致性测试", () => {
  test("768px、1280px、1440px 下桌面底栏 computed display 为 none，侧边栏可见", async ({ page }) => {
    await login(page);

    const viewports = [
      { width: 768, height: 1024 },
      { width: 1280, height: 800 },
      { width: 1440, height: 900 },
    ];

    for (const vp of viewports) {
      await page.setViewportSize(vp);
      await page.goto("/console/posts");
      await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

      const bottomNav = page.locator('nav[aria-label="移动端底部导航"]');
      await expect(bottomNav).toBeAttached();

      const display = await bottomNav.evaluate((el) => window.getComputedStyle(el).display);
      expect(display, `底栏在 ${vp.width}px 下 computed display 必须为 none`).toBe("none");

      const sidebar = page.locator("aside");
      const sidebarDisplay = await sidebar.evaluate((el) => window.getComputedStyle(el).display);
      expect(sidebarDisplay, `侧边栏在 ${vp.width}px 下必须可见`).not.toBe("none");

      if (vp.width === 1440) {
        await page.screenshot({ path: "docs/baselines/console-ui-consistency/t1-desktop-posts.png" });
      }
    }
  });

  test("390px 视口下底栏正常显示且内容底部不被遮挡", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const bottomNav = page.locator('nav[aria-label="移动端底部导航"]');
    await expect(bottomNav).toBeVisible();
    const display = await bottomNav.evaluate((el) => window.getComputedStyle(el).display);
    expect(display).toBe("grid");

    const navBox = await bottomNav.boundingBox();
    expect(navBox).not.toBeNull();
    expect(navBox!.y + navBox!.height).toBeGreaterThanOrEqual(840);
  });

  test("390px 手机视口下文章管理主要操作完整可见且点击进入编辑页", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const writeBtn = page.getByRole("button", { name: "撰写文章" });
    await expect(writeBtn).toBeVisible();

    const box = await writeBtn.boundingBox();
    expect(box).not.toBeNull();
    // 确保主按钮完全在视口宽度内，不被裁切或顶出屏幕
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);

    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t1-mobile-posts.png" });

    // 真实点击触发跳转
    await writeBtn.click();
    await expect(page).toHaveURL(/\/console\/posts\/new/, { timeout: 15_000 });
  });

  test("390px 手机视口下独立页面主要操作完整可见且点击进入新建页", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/pages");
    await expect(page.getByRole("heading", { name: "独立页面" })).toBeVisible();

    const createBtn = page.getByRole("button", { name: "新建页面" });
    await expect(createBtn).toBeVisible();

    const box = await createBtn.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);

    await createBtn.click();
    await expect(page).toHaveURL(/\/console\/pages\/edit\/new/, { timeout: 15_000 });
  });

  test("390px 手机视口下媒体库主要操作完整可见、文件选择器可触发、筛选可换行", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/attachments");
    await expect(page.getByRole("heading", { name: "媒体库" })).toBeVisible();

    const uploadBtn = page.getByRole("button", { name: "上传附件" });
    await expect(uploadBtn).toBeVisible();

    const box = await uploadBtn.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);

    // 点击上传附件能够触发文件选择器
    const fileChooserPromise = page.waitForEvent("filechooser");
    await uploadBtn.click();
    const fileChooser = await fileChooserPromise;
    expect(fileChooser).toBeDefined();

    // 媒体筛选标签分段区域正常显示
    const typeSegment = page.locator(".apple-segmented").first();
    await expect(typeSegment).toBeVisible();
    const segBox = await typeSegment.boundingBox();
    expect(segBox).not.toBeNull();
    expect(segBox!.x + segBox!.width).toBeLessThanOrEqual(390);
  });

  test("管理员可通过侧栏菜单进入主题设置，手机端可通过更多抽屉进入", async ({ page }) => {
    await login(page);

    // 桌面端验证侧栏进入主题设置
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/dashboard");

    const appearanceNavLink = page.locator('aside a[href="/console/appearance"]');
    await expect(appearanceNavLink).toBeVisible();
    await appearanceNavLink.click();
    await expect(page).toHaveURL(/\/console\/appearance/);
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();

    // 手机端验证通过“更多”抽屉进入主题设置
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/dashboard");

    const moreBtn = page.getByRole("button", { name: "更多导航" });
    await expect(moreBtn).toBeVisible();
    await moreBtn.click();

    const drawerAppearanceLink = page.locator('div[role="dialog"] a[href="/console/appearance"]');
    await expect(drawerAppearanceLink).toBeVisible();
    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t1-mobile-drawer.png" });
    await drawerAppearanceLink.click();
    await expect(page).toHaveURL(/\/console\/appearance/);
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();
  });
});
