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

    // 滚动主容器到底部，验证内容不被底栏遮挡
    await page.evaluate(() => {
      const main = document.querySelector("main.content");
      if (main) main.scrollTop = main.scrollHeight;
    });
    await page.waitForTimeout(300);

    const lastVisibleElement = page.locator("main.content .stats-card, main.content .post-card, main.content .apple-card, main.content footer").last();
    const lastBox = await lastVisibleElement.boundingBox();
    if (lastBox) {
      expect(lastBox.y + lastBox.height).toBeLessThanOrEqual(navBox!.y + 1);
    }
  });

  test("390px 手机视口下文章管理标题、搜索框与按钮组按三行排列且点击进入编辑页", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const header = page.locator("header.page-header-stack-below-lg");
    await expect(header).toBeVisible();

    const titleEl = header.locator("h1");
    const searchEl = header.locator(".apple-search");
    const writeBtn = page.getByRole("button", { name: "撰写文章" });

    await expect(titleEl).toBeVisible();
    await expect(searchEl).toBeVisible();
    await expect(writeBtn).toBeVisible();

    const titleBox = await titleEl.boundingBox();
    const searchBox = await searchEl.boundingBox();
    const btnBox = await writeBtn.boundingBox();

    expect(titleBox).not.toBeNull();
    expect(searchBox).not.toBeNull();
    expect(btnBox).not.toBeNull();

    // 验证手机 390px 视口下严格三行从上到下排列
    expect(titleBox!.y + titleBox!.height).toBeLessThanOrEqual(searchBox!.y);
    expect(searchBox!.y + searchBox!.height).toBeLessThanOrEqual(btnBox!.y);

    // 搜索框在手机端整行展示
    expect(searchBox!.width).toBeGreaterThanOrEqual(330);

    // 主操作按钮在视口安全宽度内
    expect(btnBox!.x).toBeGreaterThanOrEqual(0);
    expect(btnBox!.x + btnBox!.width).toBeLessThanOrEqual(390);

    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t1-mobile-posts.png" });

    // 真实点击触发跳转到文章编辑页
    await writeBtn.click();
    await expect(page).toHaveURL(/\/console\/posts\/new/, { timeout: 15_000 });
  });

  test("390px 手机视口下独立页面标题、搜索与新建按钮三行排列且点击进入新建页", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/pages");
    await expect(page.getByRole("heading", { name: "独立页面" })).toBeVisible();

    const header = page.locator("header.page-header-stack-below-lg");
    await expect(header).toBeVisible();

    const titleEl = header.locator("h1");
    const searchEl = header.locator(".apple-search");
    const createBtn = page.getByRole("button", { name: "新建页面" });

    await expect(titleEl).toBeVisible();
    await expect(searchEl).toBeVisible();
    await expect(createBtn).toBeVisible();

    const titleBox = await titleEl.boundingBox();
    const searchBox = await searchEl.boundingBox();
    const btnBox = await createBtn.boundingBox();

    expect(titleBox).not.toBeNull();
    expect(searchBox).not.toBeNull();
    expect(btnBox).not.toBeNull();

    // 验证严格三行上下排列
    expect(titleBox!.y + titleBox!.height).toBeLessThanOrEqual(searchBox!.y);
    expect(searchBox!.y + searchBox!.height).toBeLessThanOrEqual(btnBox!.y);
    expect(searchBox!.width).toBeGreaterThanOrEqual(330);

    expect(btnBox!.x).toBeGreaterThanOrEqual(0);
    expect(btnBox!.x + btnBox!.width).toBeLessThanOrEqual(390);

    await createBtn.click();
    await expect(page).toHaveURL(/\/console\/pages\/edit\/new/, { timeout: 15_000 });
  });

  test("390px 手机视口下媒体库标题、搜索与按钮三行排列、文件选择器可触发、筛选可换行", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/attachments");
    await expect(page.getByRole("heading", { name: "媒体库" })).toBeVisible();

    const header = page.locator("header.page-header-stack-below-lg");
    await expect(header).toBeVisible();

    const titleEl = header.locator("h1");
    const searchEl = header.locator(".apple-search");
    const uploadBtn = page.getByRole("button", { name: "上传附件" });

    await expect(titleEl).toBeVisible();
    await expect(searchEl).toBeVisible();
    await expect(uploadBtn).toBeVisible();

    const titleBox = await titleEl.boundingBox();
    const searchBox = await searchEl.boundingBox();
    const btnBox = await uploadBtn.boundingBox();

    expect(titleBox).not.toBeNull();
    expect(searchBox).not.toBeNull();
    expect(btnBox).not.toBeNull();

    // 验证严格三行上下排列
    expect(titleBox!.y + titleBox!.height).toBeLessThanOrEqual(searchBox!.y);
    expect(searchBox!.y + searchBox!.height).toBeLessThanOrEqual(btnBox!.y);
    expect(searchBox!.width).toBeGreaterThanOrEqual(330);

    expect(btnBox!.x).toBeGreaterThanOrEqual(0);
    expect(btnBox!.x + btnBox!.width).toBeLessThanOrEqual(390);

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

  test("1280px 桌面视口下标题与搜索按钮横向单行排列且底栏隐藏", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const header = page.locator("header.page-header-stack-below-lg");
    await expect(header).toBeVisible();

    const titleEl = header.locator("h1");
    const searchEl = header.locator(".apple-search");
    const writeBtn = page.getByRole("button", { name: "撰写文章" });

    const titleBox = await titleEl.boundingBox();
    const searchBox = await searchEl.boundingBox();
    const btnBox = await writeBtn.boundingBox();

    expect(titleBox).not.toBeNull();
    expect(searchBox).not.toBeNull();
    expect(btnBox).not.toBeNull();

    // 桌面端横向单行布局：标题与搜索垂直对齐相差较小
    expect(Math.abs(titleBox!.y - searchBox!.y)).toBeLessThanOrEqual(25);
    expect(Math.abs(searchBox!.y - btnBox!.y)).toBeLessThanOrEqual(5);
    // 标题在左侧，搜索在右侧
    expect(titleBox!.x + titleBox!.width).toBeLessThan(searchBox!.x);
    // 底栏隐藏
    const bottomNav = page.locator('nav[aria-label="移动端底部导航"]');
    const display = await bottomNav.evaluate((el) => window.getComputedStyle(el).display);
    expect(display).toBe("none");
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

  test("data-app 标识在 Console 和 UC 独立区分且样式隔离", async ({ page }) => {
    await login(page);
    await page.goto("/console/dashboard");
    const consoleAppAttr = await page.evaluate(() => document.documentElement.getAttribute("data-app"));
    expect(consoleAppAttr).toBe("console");

    await page.goto("/uc/login");
    const ucAppAttr = await page.evaluate(() => document.documentElement.getAttribute("data-app"));
    expect(ucAppAttr).toBe("uc");
  });

  test("390px 手机视口下表单输入、搜索、选择器、筛选按钮高度至少 44px，图标按钮至少 44x44px", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    // 搜索框高度 >= 44px
    const searchEl = page.locator(".apple-search").first();
    await expect(searchEl).toBeVisible();
    const searchBox = await searchEl.boundingBox();
    expect(searchBox).not.toBeNull();
    expect(searchBox!.height).toBeGreaterThanOrEqual(44);

    // 下拉选择器高度 >= 44px
    const selectEl = page.locator(".apple-select").first();
    await expect(selectEl).toBeVisible();
    const selectBox = await selectEl.boundingBox();
    expect(selectBox).not.toBeNull();
    expect(selectBox!.height).toBeGreaterThanOrEqual(44);

    // 分段筛选按钮高度 >= 44px
    const segBtn = page.locator(".apple-segmented button").first();
    await expect(segBtn).toBeVisible();
    const segBox = await segBtn.boundingBox();
    expect(segBox).not.toBeNull();
    expect(segBox!.height).toBeGreaterThanOrEqual(44);

    // 全选/工具栏筛选按钮高度 >= 44px（若列表非空存在筛选按钮时）
    const filterBtn = page.locator(".apple-filter-btn").first();
    if (await filterBtn.count() > 0) {
      await expect(filterBtn).toBeVisible();
      const filterBox = await filterBtn.boundingBox();
      expect(filterBox).not.toBeNull();
      expect(filterBox!.height).toBeGreaterThanOrEqual(44);
    }

    // 卡片独立操作图标按钮 >= 44x44px
    const postAction = page.locator(".post-card-action").first();
    if (await postAction.count() > 0) {
      const actionBox = await postAction.boundingBox();
      expect(actionBox).not.toBeNull();
      expect(actionBox!.width).toBeGreaterThanOrEqual(44);
      expect(actionBox!.height).toBeGreaterThanOrEqual(44);
    }
  });

  test("系统设置保存当前分区 JSON 后同步规范化快照，消除未保存假脏态", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=advanced");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "原始 JSON 配置（高级兜底）" })).toBeVisible();

    // 初始状态：无黄点与脏标记
    const dirtyDots = page.locator(".settings-tab-dirty-dot");
    await expect(dirtyDots).toHaveCount(0);
    const dirtyBadge = page.locator(".settings-dirty-badge");
    await expect(dirtyBadge).toHaveCount(0);

    // 选择 site 分区
    const sectionSelect = page.locator("#settings-raw-section");
    await sectionSelect.selectOption("site");

    // 获取当前 raw JSON 内容并做轻微合规修改
    const rawTextarea = page.locator("#settings-raw-content");
    const currentVal = await rawTextarea.inputValue();
    const parsed = JSON.parse(currentVal || "{}");
    parsed.subtitle = parsed.subtitle === "E2E Tested" ? "E2E Updated" : "E2E Tested";
    await rawTextarea.fill(JSON.stringify(parsed, null, 2));

    // 此时表单在本地发生变更，出现未保存标记
    const saveJsonBtn = page.getByRole("button", { name: /保存 site JSON/ });
    await expect(saveJsonBtn).toBeVisible();
    await saveJsonBtn.click();

    // 等待保存完成
    await expect(page.locator(".apple-toast-item")).toBeVisible();

    // 保存后快照已同步更新，不产生未保存假脏态
    await expect(dirtyDots).toHaveCount(0);
    await expect(dirtyBadge).toHaveCount(0);
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

  test("空列表时隐藏选择入口（不渲染全选按钮、全选复选框与批量入口）", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    // 搜索不存在的关键字使列表为空
    const searchInput = page.getByPlaceholder("搜索文章标题或 Slug...");
    await searchInput.fill("nonexistent_keyword_to_empty_the_list_12345");
    // 等待空状态提示出现
    await expect(page.getByText("未找到匹配的文章")).toBeVisible();

    // 此时全选按钮与取消全选入口均不渲染
    const selectAllBtn = page.getByRole("button", { name: "全选本页" });
    await expect(selectAllBtn).toHaveCount(0);
    const unselectBtn = page.getByRole("button", { name: "取消全选" });
    await expect(unselectBtn).toHaveCount(0);

    // 批量浮层不可见
    const floatingDock = page.locator(".v-floating-dock");
    await expect(floatingDock).toHaveCount(0);

    await page.screenshot({ path: "docs/baselines/console-ui-consistency/t3-posts-empty.png" });
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

  test("ConsoleStatsCard 在用户管理与备份管理均采用统一规范", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    // 用户管理
    await page.goto("/console/users");
    await expect(page.getByRole("heading", { name: "用户管理" })).toBeVisible();
    const userCards = page.locator(".stats-card");
    await expect(userCards).toHaveCount(4);
    const userFirstBox = await userCards.first().boundingBox();
    expect(userFirstBox).not.toBeNull();
    expect(userFirstBox!.height).toBeGreaterThanOrEqual(80);

    // 备份管理
    await page.goto("/console/backup");
    await expect(page.getByRole("heading", { name: "备份与导出" })).toBeVisible();
    const backupCards = page.locator(".stats-card");
    await expect(backupCards).toHaveCount(4);
    const backupFirstBox = await backupCards.first().boundingBox();
    expect(backupFirstBox).not.toBeNull();
    expect(backupFirstBox!.height).toBeGreaterThanOrEqual(80);
  });

  test("设置页具有合法 tabpanel 容器与 aria-labelledby 联动，非活动 panel 保持 hidden", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    const basicPanel = page.locator('#tabpanel-basic[role="tabpanel"]');
    await expect(basicPanel).toBeVisible();
    const labelledby = await basicPanel.getAttribute("aria-labelledby");
    expect(labelledby).toBe("tab-basic");

    const tab = page.locator("#tab-basic");
    await expect(tab).toBeVisible();
    const controls = await tab.getAttribute("aria-controls");
    expect(controls).toBe("tabpanel-basic");

    // 验证其他未激活的 tabpanel 也存在且带有 hidden 属性
    const postPanel = page.locator("#tabpanel-post");
    await expect(postPanel).toHaveAttribute("hidden", "");
  });

  test("SEO 二级导航高度（桌面 36px / 手机 44px）、字号 13px 与深灰选中样式对齐", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/seo/overview");
    await expect(page.getByRole("heading", { name: "SEO 工具集" })).toBeVisible();

    const activeBtn = page.getByRole("button", { name: "概览" });
    await expect(activeBtn).toBeVisible();
    const activeBox = await activeBtn.boundingBox();
    expect(activeBox).not.toBeNull();
    // 桌面视口高度 36px
    expect(Math.round(activeBox!.height)).toBe(36);

    // 字号 13px
    const fontSize = await activeBtn.evaluate((el) => window.getComputedStyle(el).fontSize);
    expect(fontSize).toBe("13px");

    // 具有 aria-current="page"
    const ariaCurrent = await activeBtn.getAttribute("aria-current");
    expect(ariaCurrent).toBe("page");

    // 选中项使用深灰背景 (#2c2c2e -> rgb(44, 44, 46))
    const activeBg = await activeBtn.evaluate((el) => window.getComputedStyle(el).backgroundColor);
    expect(activeBg).toBe("rgb(44, 44, 46)");

    // 手机视口 (390px) 高度 44px
    await page.setViewportSize({ width: 390, height: 844 });
    // 等待视口调整后样式重新计算
    await page.waitForTimeout(200);
    const mobileBox = await activeBtn.boundingBox();
    expect(mobileBox).not.toBeNull();
    const mobileHeight = await activeBtn.evaluate((el) => window.getComputedStyle(el).height);
    expect(Math.round(parseFloat(mobileHeight))).toBe(44);
  });

  test("系统设置保存时支持持续编辑且保留新内容与未保存提示", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    const titleInput = page.locator("#settings-site-title");
    await expect(titleInput).toBeVisible();

    // 初始清空脏标记
    const dirtyDots = page.locator(".settings-tab-dirty-dot");
    const dirtyBadge = page.locator(".settings-dirty-badge");

    // 修改标题（由于 basic 与 seo 标签均涉及 site 分区，两者均会出现黄点提示）
    await titleInput.fill("E2E Test Site");
    await expect(dirtyDots).toHaveCount(2);
    await expect(dirtyBadge).toBeVisible();

    // 保存基本设置
    const saveBtn = page.locator(".settings-basic-actions button[type='submit']");
    await saveBtn.click();
    await expect(page.locator(".apple-toast-item")).toBeVisible();

    // 保存后脏标记消除
    await expect(dirtyDots).toHaveCount(0);
    await expect(dirtyBadge).toHaveCount(0);

    // 再次修改，黄点立即出现
    await titleInput.fill("E2E Test Site In-Flight");
    await expect(dirtyDots).toHaveCount(2);
    await expect(dirtyBadge).toBeVisible();
  });

  test("系统设置高级原始 JSON 语法错误时前置拦截并保持未保存状态", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=advanced");
    await expect(page.getByRole("heading", { name: "原始 JSON 配置（高级兜底）" })).toBeVisible();

    const sectionSelect = page.locator("#settings-raw-section");
    await sectionSelect.selectOption("site");

    const rawTextarea = page.locator("#settings-raw-content");
    await rawTextarea.fill("{\ninvalid_json: true,\n}");

    // 未保存黄点与标记出现
    const dirtyBadge = page.locator(".settings-dirty-badge");
    await expect(dirtyBadge).toBeVisible();

    const saveJsonBtn = page.getByRole("button", { name: /保存 site JSON/ });
    await saveJsonBtn.click();

    // 弹出错误 Toast
    await expect(page.locator(".apple-toast-item")).toContainText("JSON 格式错误");
    // 依然保持脏状态
    await expect(dirtyBadge).toBeVisible();
  });
});
