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

  test("主按钮采用胶囊圆角、黑字高对比、无外发光", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const writeBtn = page.getByRole("button", { name: "撰写文章" });
    await expect(writeBtn).toBeVisible();

    const styles = await writeBtn.evaluate((el) => {
      const cs = window.getComputedStyle(el);
      return {
        color: cs.color,
        backgroundColor: cs.backgroundColor,
        boxShadow: cs.boxShadow,
        borderRadius: cs.borderRadius,
        height: cs.height,
      };
    });

    // 文字颜色为黑色（rgb(0, 0, 0)）
    expect(styles.color).toBe("rgb(0, 0, 0)");
    // 无外发光
    expect(styles.boxShadow).toBe("none");
    // 高度为标准 36px
    expect(styles.height).toBe("36px");
    // 胶囊圆角（大于等于 18px）
    const radiusNum = parseFloat(styles.borderRadius);
    expect(radiusNum).toBeGreaterThanOrEqual(18);
  });

  test("分段 Tabs 与按钮支持键盘聚焦并具有清晰焦点环", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/appearance");
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();

    // 检查 CSS 样式表中已声明的 focus-visible 规范
    const focusRules = await page.evaluate(() => {
      const results: { selector: string; cssText: string; offset: string }[] = [];
      for (const sheet of document.styleSheets) {
        try {
          for (const rule of sheet.cssRules) {
            const r = rule as CSSStyleRule;
            if (
              r.selectorText &&
              ((r.selectorText.includes(".apple-segmented button") && r.selectorText.includes(":focus-visible")) ||
                (r.selectorText.includes(".btn") && r.selectorText.includes(":focus-visible")) ||
                (r.selectorText.includes(".appearance-tabs button") && r.selectorText.includes(":focus-visible")))
            ) {
              results.push({
                selector: r.selectorText,
                cssText: r.cssText,
                offset: r.style.outlineOffset,
              });
            }
          }
        } catch {}
      }
      return results;
    });

    expect(focusRules.length).toBeGreaterThan(0);
    const segRule = focusRules.find((r) => r.selector.includes(".apple-segmented button") && r.selector.includes(":focus-visible"));
    expect(segRule).toBeDefined();
    expect(segRule!.offset).toBe("2px");
    expect(segRule!.cssText).toContain("2px solid");

    const appTabRule = focusRules.find((r) => r.selector.includes(".appearance-tabs button") && r.selector.includes(":focus-visible"));
    expect(appTabRule).toBeDefined();
    expect(appTabRule!.offset).toBe("2px");
    expect(appTabRule!.cssText).toContain("2px solid");
  });

  test("主题设置初始加载无未保存黄点误报，真实修改后出现黄点并可保存恢复", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/appearance");
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();

    // 等待数据加载完成
    await expect(page.locator(".appearance-state")).toHaveCount(0);

    // 初始状态：无黄点、无未保存告警徽章
    const dirtyDots = page.locator(".settings-tab-dirty-dot");
    await expect(dirtyDots).toHaveCount(0);
    const dirtyBadge = page.locator(".settings-dirty-badge");
    await expect(dirtyBadge).toHaveCount(0);

    // 模拟用户修改主题色输入框
    const themeInput = page.locator('input[placeholder*="#2E8DFF"]').first();
    await expect(themeInput).toBeVisible();
    await themeInput.fill("#3b82f6");

    // 此时出现黄点与脏标记
    await expect(dirtyDots).toHaveCount(1);
    await expect(dirtyBadge).toBeVisible();

    // 点击保存按钮
    const saveBtn = page.getByRole("button", { name: "保存外观设置" });
    await expect(saveBtn).toBeVisible();
    await saveBtn.click();

    // 保存成功提示后，黄点消失
    await expect(dirtyDots).toHaveCount(0);
    await expect(dirtyBadge).toHaveCount(0);
  });

  test("列表概览卡片统一样式且符合最小高度规范（桌面 80px / 手机 88px）", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const statsCards = page.locator(".stats-card");
    await expect(statsCards.first()).toBeVisible();
    const count = await statsCards.count();
    expect(count).toBeGreaterThan(0);

    for (let i = 0; i < count; i++) {
      const card = statsCards.nth(i);
      const minHeight = await card.evaluate((el) => window.getComputedStyle(el).minHeight);
      expect(minHeight).toBe("80px");
      const box = await card.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.height).toBeGreaterThanOrEqual(80);
    }

    // 切换至手机视口 (390px)
    await page.setViewportSize({ width: 390, height: 844 });
    for (let i = 0; i < count; i++) {
      const card = statsCards.nth(i);
      const minHeight = await card.evaluate((el) => window.getComputedStyle(el).minHeight);
      expect(minHeight).toBe("88px");
      const box = await card.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.height).toBeGreaterThanOrEqual(88);
    }
  });

  test("空列表时全选按钮保持显示全选本页且被禁用，杜绝显示取消全选", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    // 搜索不存在的关键字使列表为空
    const searchInput = page.getByPlaceholder("搜索文章标题或 Slug...");
    await searchInput.fill("nonexistent_keyword_to_empty_the_list_12345");
    // 等待空状态提示出现
    await expect(page.getByText("未找到匹配的文章")).toBeVisible();

    // 此时全选按钮必须显示“全选本页”且处于 disabled 状态
    const selectAllBtn = page.getByRole("button", { name: "全选本页" });
    await expect(selectAllBtn).toBeVisible();
    await expect(selectAllBtn).toBeDisabled();

    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t3-posts-empty.png" });

    // 绝对不能出现“取消全选”
    const unselectBtn = page.getByRole("button", { name: "取消全选" });
    await expect(unselectBtn).toHaveCount(0);
  });

  test("主题设置与系统设置统一分段 Tabs 结构，单层卡片包裹并消除多层嵌套卡片", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/appearance");
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();
    await expect(page.locator(".appearance-state")).toHaveCount(0);

    // 检查分段 Tabbar 结构与 ARIA 规范
    const tablist = page.locator('.appearance-tabs [role="tablist"]');
    await expect(tablist).toBeVisible();
    const tabs = tablist.locator('[role="tab"]');
    await expect(tabs).toHaveCount(7);

    // 单一卡片边界：外层为 .settings-basic-panel
    const mainPanel = page.locator(".settings-basic-panel");
    await expect(mainPanel).toBeVisible();

    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t3-appearance-desktop.png" });

    // 切换到“布局”Tab
    const layoutTab = tabs.filter({ hasText: "布局" });
    await expect(layoutTab).toBeVisible();
    await layoutTab.click();
    await expect(page).toHaveURL(/tab=layout/);

    // 验证“布局”中的小节采用 flat 的 section 结构，不再有内层嵌套卡片 .card-wrapper
    await expect(page.getByRole("heading", { name: "右栏与页脚" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "组件编排" })).toBeVisible();
    const innerNestedCards = page.locator(".appearance-tab-content .card-wrapper");
    await expect(innerNestedCards).toHaveCount(0);
  });
});
