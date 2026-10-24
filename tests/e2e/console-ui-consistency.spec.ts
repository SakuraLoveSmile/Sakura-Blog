import { expect, test, type Locator, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

interface PostFixture {
  slug: string;
  title: string;
}

/** 创建带唯一 token 的隔离文章夹具，返回 slug 与标题的对应关系。 */
async function ensurePosts(page: Page, count = 5, token = "mobile-e2e-post"): Promise<PostFixture[]> {
  const fixtures: PostFixture[] = [];
  for (let i = 1; i <= count; i++) {
    const slug = `${token}-${i}`;
    const title = `移动端无遮挡验证博文 ${token}-${i}`;
    const response = await page.request.post("/api/v1/posts", {
      data: {
        slug,
        title,
        content_md: `这是第 ${i} 篇用于验证移动端滚动不被遮挡的测试文章内容。`,
        status: "published",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(response.ok(), `夹具文章 ${slug} 必须创建成功（HTTP ${response.status()}）`).toBe(true);
    fixtures.push({ slug, title });
  }
  return fixtures;
}

test.describe("Console UI 一致性测试", () => {
  test("768px、1280px、1440px 下桌面底栏 computed display 为 none，侧边栏可见", async ({ page }, testInfo) => {
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
        await page.screenshot({ path: testInfo.outputPath("t1-desktop-posts.png") });
      }
    }
  });

  test("390px 视口下底栏正常显示，滚动到底后最后一项操作无遮挡且能触发", async ({ page }) => {
    await login(page);
    const token = `mobile-e2e-post-${Date.now()}`;
    const fixtures = await ensurePosts(page, 8, token);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    // 用夹具 token 过滤，保证页面上的卡片全部来自本轮夹具，末项归属不再依赖全局排序
    await page.getByPlaceholder("搜索文章标题或 Slug...").fill(token);
    await expect(page.locator(".post-card")).toHaveCount(fixtures.length);

    const bottomNav = page.locator('nav[aria-label="移动端底部导航"]');
    await expect(bottomNav).toBeVisible();
    expect(await bottomNav.evaluate((el) => window.getComputedStyle(el).display)).toBe("grid");
    const navBox = await bottomNav.boundingBox();
    expect(navBox, "底栏必须有完整矩形").not.toBeNull();

    // 滚动真实主容器到底，并等待滚动位置达到预期（不用固定 sleep 作为完成证据）
    const geometry = await page.evaluate(() => {
      const main = document.querySelector("main.content") as HTMLElement | null;
      if (main) main.scrollTop = main.scrollHeight;
      return { scrollHeight: main?.scrollHeight ?? 0, clientHeight: main?.clientHeight ?? 0 };
    });
    expect(geometry.scrollHeight, "隔离数据必须产生真实纵向滚动").toBeGreaterThan(geometry.clientHeight);
    const expectedBottom = geometry.scrollHeight - geometry.clientHeight;
    await expect
      .poll(
        () =>
          page.evaluate(() => {
            const main = document.querySelector("main.content") as HTMLElement | null;
            return main ? main.scrollTop : -1;
          }),
        { timeout: 8_000, message: "主容器必须真正滚动到底部" },
      )
      .toBeGreaterThanOrEqual(expectedBottom - 1);

    // 末项 = DOM 中最后一张真实卡片；由卡片呈现的标题映射回夹具 slug，不做序号猜测
    const lastCard = page.locator(".post-card").last();
    await expect(lastCard).toBeVisible();
    const label = await lastCard.locator('label[aria-label^="选择文章"]').getAttribute("aria-label");
    const mapped = fixtures.find((item) => (label ?? "").includes(item.title));
    expect(mapped, `末项必须对应本轮夹具，实际 aria-label=${label}`).toBeTruthy();

    const lastEditBtn = lastCard.locator('button[title="编辑博文"]');
    await expect(lastEditBtn).toBeVisible();
    const editBtnBox = await lastEditBtn.boundingBox();
    expect(editBtnBox, "最后一项操作必须有完整矩形，不允许条件跳过").not.toBeNull();
    expect(editBtnBox!.width).toBeGreaterThan(0);
    expect(editBtnBox!.height).toBeGreaterThan(0);
    expect(editBtnBox!.y).toBeGreaterThanOrEqual(0);
    expect(editBtnBox!.y + editBtnBox!.height).toBeLessThanOrEqual(navBox!.y);

    const footerBox = await page.locator("main.content footer").boundingBox();
    expect(footerBox, "页脚必须存在完整矩形").not.toBeNull();
    expect(footerBox!.y + footerBox!.height).toBeLessThanOrEqual(navBox!.y);

    // 实际点击最后一项操作：事件必须到达并进入该夹具对应的编辑页
    await lastEditBtn.click();
    await expect(page).toHaveURL(new RegExp(`/console/posts/edit/${mapped!.slug}`), { timeout: 15_000 });
  });

  test("390px 与 768px 视口下文章管理标题、搜索框与按钮组按三行排列且点击进入编辑页", async ({ page }, testInfo) => {
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

    await page.screenshot({ path: testInfo.outputPath("t1-mobile-posts.png") });

    // 验证 768px (平板竖屏) 视口下同样按三行排列
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const header768 = page.locator("header.page-header-stack-below-lg");
    const title768 = header768.locator("h1");
    const search768 = header768.locator(".apple-search");
    const writeBtn768 = page.getByRole("button", { name: "撰写文章" });

    const titleBox768 = await title768.boundingBox();
    const searchBox768 = await search768.boundingBox();
    const btnBox768 = await writeBtn768.boundingBox();

    expect(titleBox768!.y + titleBox768!.height).toBeLessThanOrEqual(searchBox768!.y);
    expect(searchBox768!.y + searchBox768!.height).toBeLessThanOrEqual(btnBox768!.y);

    // 切回 390px 真实点击触发跳转到文章编辑页
    await page.setViewportSize({ width: 390, height: 844 });
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

  test("1280px 与 1440px 桌面视口下标题与搜索按钮横向单行排列且底栏隐藏", async ({ page }) => {
    await login(page);
    for (const width of [1280, 1440]) {
      await page.setViewportSize({ width, height: 900 });
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
    }
  });

  test("管理员可通过侧栏菜单进入主题设置，手机端可通过更多抽屉进入，且支持无障碍焦点管理与关闭返回", async ({ page }, testInfo) => {
    await login(page);

    // 桌面端验证侧栏进入主题设置
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/dashboard");

    const appearanceNavLink = page.locator('aside a[href="/console/appearance"]');
    await expect(appearanceNavLink).toBeVisible();
    await appearanceNavLink.click();
    await expect(page).toHaveURL(/\/console\/appearance/);
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();

    // 手机端验证通过“更多”抽屉进入主题设置与焦点管理
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/dashboard");

    const moreBtn = page.getByRole("button", { name: "更多导航" });
    await expect(moreBtn).toBeVisible();
    await moreBtn.click();

    // 验证抽屉出现且关闭按钮自动聚焦
    const drawerDialog = page.locator('div[role="dialog"]');
    await expect(drawerDialog).toBeVisible();
    const closeBtn = page.locator('button[aria-label="关闭抽屉"]');
    await expect(closeBtn).toBeVisible();
    await expect(closeBtn).toBeFocused();

    // 验证按下 Escape 关闭抽屉，焦点返回到“更多导航”按钮
    await page.keyboard.press("Escape");
    await expect(drawerDialog).toHaveCount(0);
    await expect(moreBtn).toBeFocused();

    // 再次打开抽屉并点击主题设置
    await moreBtn.click();
    await expect(drawerDialog).toBeVisible();
    const drawerAppearanceLink = page.locator('div[role="dialog"] a[href="/console/appearance"]');
    await expect(drawerAppearanceLink).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("t1-mobile-drawer.png") });
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

  test("系统设置高级页已移除原始 JSON 分区编辑器：不渲染分区选择与 site JSON 保存入口", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=advanced");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "原始 JSON 配置（高级兜底）" })).toHaveCount(0);
    await expect(page.locator("#settings-raw-section")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /保存 site JSON/ })).toHaveCount(0);
  });

  test("系统设置基本分区初始加载无未保存黄点误报，真实修改后出现黄点并可保存恢复", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    // 初始状态：无黄点、无未保存告警徽章
    const dirtyDots = page.locator(".settings-tab-dirty-dot");
    await expect(dirtyDots).toHaveCount(0);
    const dirtyBadge = page.locator(".settings-dirty-badge");
    await expect(dirtyBadge).toHaveCount(0);

    // 模拟用户修改站点标题
    const titleInput = page.locator("#settings-site-title");
    await expect(titleInput).toBeVisible();
    await titleInput.fill("E2E Dirty Dot Title");

    // 此时出现黄点与脏标记
    // 基本与 SEO 共享站点字段，两个分区都会提示未保存
    await expect(dirtyDots).toHaveCount(2);
    await expect(dirtyBadge).toBeVisible();

    // 点击基本分区保存按钮
    const saveBtn = page.locator(".settings-basic-actions button[type='submit']");
    await saveBtn.click();

    // 保存成功后，黄点消失
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

  test("空列表时隐藏选择入口（不渲染全选按钮、全选复选框与批量入口）", async ({ page }, testInfo) => {
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

    await page.screenshot({ path: testInfo.outputPath("t3-posts-empty.png") });
  });

  test("主题设置已退役为通知页：不再渲染分段 Tabs", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/appearance");
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();
    await expect(page.locator('.appearance-tabs [role="tablist"]')).toHaveCount(0);
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

  test("评论分区第三方评论 JSON 语法错误时前置拦截：错误提示可见且保存按钮禁用", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=comments");

    const jsonField = page.locator('[data-setting-path="theme.comments"]').locator("input,textarea");
    await jsonField.fill("{\ninvalid_json: true,\n}");

    await expect(page.getByText("请输入有效的 JSON 对象")).toBeVisible();
    await expect(
      page.locator(".canonical-settings-form:visible").getByRole("button", { name: "保存全部主题设置", exact: true }),
    ).toBeDisabled();
  });
});

/** 活动标签是否完整落在 tabbar 横向滚动区域内。 */
async function activeTabVisibility(tabbar: Locator): Promise<string> {
  return tabbar.evaluate((el) => {
    const active = el.querySelector("[role='tab'][aria-selected='true']") as HTMLElement | null;
    if (!active) return "no-active-tab";
    const container = el.getBoundingClientRect();
    const target = active.getBoundingClientRect();
    if (target.right - container.right > 1) return `overflow-right:${Math.round(target.right - container.right)}`;
    if (container.left - target.left > 1) return `overflow-left:${Math.round(container.left - target.left)}`;
    return "visible";
  });
}

/** 断言 autoReveal 的 Tabs 由组件自身滚动，且显露不改变纵向滚动位置。 */
async function assertTabRevealContract(page: Page, containerSelector: string, firstLabel: string, lastLabel: string) {
  const outer = page.locator(containerSelector);
  const tabbar = outer.locator("[role='tablist']");
  await expect(tabbar).toBeVisible();

  // 外层容器只负责宽度与裁切，不再承担横向滚动
  expect(await outer.evaluate((el) => window.getComputedStyle(el).overflowX)).toBe("hidden");
  expect(await outer.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(false);

  // 内部根元素是唯一横向滚动区域，且在当前视口下真实溢出
  const inner = await tabbar.evaluate((el) => ({
    overflowX: window.getComputedStyle(el).overflowX,
    scrollWidth: el.scrollWidth,
    clientWidth: el.clientWidth,
    autoreveal: el.classList.contains("tabbar-autoreveal"),
  }));
  expect(inner.autoreveal, "autoReveal 必须落在专用修饰类上").toBe(true);
  expect(inner.overflowX).toBe("auto");
  expect(inner.scrollWidth, "390px 下必须真实溢出").toBeGreaterThan(inner.clientWidth);

  // 纵向基准归零后再比较，避免内容高度变化被浏览器夹紧造成假阳性
  await page.evaluate(() => {
    window.scrollTo(0, 0);
    const main = document.querySelector("main.content") as HTMLElement | null;
    if (main) main.scrollTop = 0;
  });
  const verticalBefore = await page.evaluate(() => ({
    windowY: window.scrollY,
    mainScrollTop: (document.querySelector("main.content") as HTMLElement | null)?.scrollTop ?? 0,
  }));

  // 末尾标签：完整可见，且容器确实发生横向位移
  await tabbar.locator("[role='tab']").filter({ hasText: lastLabel }).click();
  await expect.poll(() => activeTabVisibility(tabbar), { timeout: 8_000 }).toBe("visible");
  await expect.poll(() => tabbar.evaluate((el) => el.scrollLeft), { timeout: 8_000 }).toBeGreaterThan(0);

  // 首标签：完整可见，容器回到起点
  await tabbar.locator("[role='tab']").filter({ hasText: firstLabel }).click();
  await expect.poll(() => activeTabVisibility(tabbar), { timeout: 8_000 }).toBe("visible");
  await expect.poll(() => tabbar.evaluate((el) => el.scrollLeft), { timeout: 8_000 }).toBe(0);

  const verticalAfter = await page.evaluate(() => ({
    windowY: window.scrollY,
    mainScrollTop: (document.querySelector("main.content") as HTMLElement | null)?.scrollTop ?? 0,
  }));
  expect(verticalAfter).toEqual(verticalBefore);
}

const LIST_EMPTY_CASES: Array<{
  path: string;
  heading: string;
  search: string;
  emptyTitle: string;
  /** 该列表的「表格视图」切换按钮 title；无表格模式的列表为 null */
  tableToggle: string | null;
}> = [
  {
    path: "/console/posts",
    heading: "文章管理",
    search: "搜索文章标题或 Slug...",
    emptyTitle: "未找到匹配的文章",
    tableToggle: "表格视图",
  },
  {
    path: "/console/pages",
    heading: "独立页面",
    search: "搜索页面标题或 Slug...",
    emptyTitle: "未找到匹配的页面",
    tableToggle: "表格视图",
  },
  {
    path: "/console/attachments",
    heading: "媒体库",
    search: "搜索文件名或 URL...",
    emptyTitle: "未找到匹配的附件",
    tableToggle: "表格视图",
  },
  {
    path: "/console/comments",
    heading: "评论管理",
    search: "搜索昵称、邮箱或正文...",
    emptyTitle: "未找到匹配的评论",
    tableToggle: null,
  },
  {
    path: "/console/friends",
    heading: "友链",
    search: "搜索友链名称/URL/分组...",
    emptyTitle: "未找到匹配的友链",
    tableToggle: "表格列表视图",
  },
];

test.describe("Console Tabs 滚动归属与空列表选择入口", () => {
  test("候选验收截图：设置与主题设置 Tabs（390 溢出 / 1280 桌面）", async ({ page }, testInfo) => {
    await login(page);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=advanced");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await expect.poll(() => activeTabVisibility(page.locator(".settings-tabs [role='tablist']"))).toBe("visible");
    await page.screenshot({ path: testInfo.outputPath("tabs-settings-390.png") });

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/appearance?tab=appearance");
    await expect(page.getByRole("heading", { name: "主题设置" })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("tabs-appearance-1280.png") });
  });

  test("390px 真实溢出下系统设置 Tabs 由组件自身滚动，首尾标签完整可见且纵向位置不变", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await assertTabRevealContract(page, ".settings-tabs", "基本", "高级");
  });

  test("390px 真实溢出下系统设置 Tabs 由组件自身滚动，Stellar 2 首标签与高级末标签完整可达且纵向位置不变", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await assertTabRevealContract(page, ".settings-tabs", "Stellar 2", "高级");
  });

  test("query 直接命中末尾标签时初始加载即完成显露，Home/End 切换保持显露与焦点", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=advanced");
    const tabbar = page.locator(".settings-tabs [role='tablist']");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    // 初始加载（query 驱动）即显露活动标签
    await expect.poll(() => activeTabVisibility(tabbar), { timeout: 8_000 }).toBe("visible");
    await expect(tabbar.locator("#tab-advanced")).toHaveAttribute("aria-selected", "true");

    // Home/End 切换同样触发显露，且焦点停留在被激活的标签上
    await tabbar.locator("#tab-advanced").focus();
    await page.keyboard.press("Home");
    await expect(tabbar.locator("#tab-stellar2")).toHaveAttribute("aria-selected", "true");
    await expect(tabbar.locator("#tab-stellar2")).toBeFocused();
    await expect.poll(() => activeTabVisibility(tabbar), { timeout: 8_000 }).toBe("visible");

    await page.keyboard.press("End");
    await expect(tabbar.locator("#tab-advanced")).toHaveAttribute("aria-selected", "true");
    await expect(tabbar.locator("#tab-advanced")).toBeFocused();
    await expect.poll(() => activeTabVisibility(tabbar), { timeout: 8_000 }).toBe("visible");
  });

  test("空列表不渲染任何选择入口：五类列表默认视图", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    const emptyKeyword = "e2e_empty_list_keyword_9f3a";
    for (const listCase of LIST_EMPTY_CASES) {
      await page.goto(listCase.path);
      await expect(page.getByRole("heading", { name: listCase.heading, exact: true })).toBeVisible();

      const search = page.getByPlaceholder(listCase.search);
      await search.fill(emptyKeyword);
      await expect(page.getByText(listCase.emptyTitle)).toBeVisible();

      // 选择入口与批量浮条一律不存在
      await expect(page.locator('thead input[type="checkbox"]')).toHaveCount(0);
      await expect(page.getByRole("button", { name: "全选本页" })).toHaveCount(0);
      await expect(page.getByRole("button", { name: "取消全选" })).toHaveCount(0);
      await expect(
        page.locator(".v-floating-dock"),
        `${listCase.path}：空列表不得残留批量浮条`,
      ).toHaveCount(0);

      await search.fill("");
      await expect(page.getByText(listCase.emptyTitle)).toHaveCount(0);
    }
  });

  test("表格视图空列表同样不渲染表头选择入口：文章 / 页面 / 附件 / 友链", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    for (const listCase of LIST_EMPTY_CASES.filter((item) => item.tableToggle)) {
      await page.goto(listCase.path);
      await expect(page.getByRole("heading", { name: listCase.heading, exact: true })).toBeVisible();

      // 视图切换入口与数据无关，始终可切换
      await page.locator(`button[title="${listCase.tableToggle}"]`).click();

      const search = page.getByPlaceholder(listCase.search);
      await search.fill("e2e_empty_table_keyword_9f3a");
      await expect(page.getByText(listCase.emptyTitle)).toBeVisible();

      await expect(page.locator('thead input[type="checkbox"]')).toHaveCount(0);
      await expect(page.getByRole("button", { name: "全选本页" })).toHaveCount(0);
      await expect(page.getByRole("button", { name: "取消全选" })).toHaveCount(0);
      await expect(page.locator(".v-floating-dock")).toHaveCount(0);

      await search.fill("");
    }
  });

  test("卡片模式先选择再筛选为空：旧选择被清除，批量浮条消失", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    // 夹具：以返回的 slug 确认目标，不依赖排序猜测「第几条是最后一条」
    const slug = `e2e-empty-selection-${Date.now()}`;
    const title = "空态选择路径夹具";
    const created = await page.request.post("/api/v1/posts", {
      data: {
        slug,
        title,
        content_md: "夹具正文",
        status: "published",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(created.ok(), "夹具文章必须创建成功").toBe(true);

    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();

    const card = page.locator(".post-card").filter({ hasText: title }).first();
    await expect(card).toBeVisible();
    await card.locator('label[aria-label^="选择文章"]').click();
    await expect(page.locator(".v-floating-dock")).toBeVisible();

    // 筛选为空 → 浮条必须消失
    const search = page.getByPlaceholder("搜索文章标题或 Slug...");
    await search.fill("e2e_empty_after_select_9f3a");
    await expect(page.getByText("未找到匹配的文章")).toBeVisible();
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);

    // 恢复列表后旧选择不得复活
    await search.fill("");
    await expect(page.locator(".post-card").filter({ hasText: title }).first()).toBeVisible();
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "全选本页" })).toBeVisible();
  });

  test("表格模式先选择再筛选为空：表头选择入口与批量浮条一并消失", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    const slug = `e2e-table-empty-selection-${Date.now()}`;
    const title = "表格空态选择路径夹具";
    const created = await page.request.post("/api/v1/posts", {
      data: {
        slug,
        title,
        content_md: "夹具正文",
        status: "published",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(created.ok(), "夹具文章必须创建成功").toBe(true);

    await page.goto("/console/posts");
    await expect(page.getByRole("heading", { name: "文章管理" })).toBeVisible();
    await page.locator('button[title="表格视图"]').click();

    const row = page.locator("tbody tr").filter({ hasText: title }).first();
    await expect(row).toBeVisible();
    await row.locator('input[type="checkbox"]').click();
    await expect(page.locator(".v-floating-dock")).toBeVisible();

    const search = page.getByPlaceholder("搜索文章标题或 Slug...");
    await search.fill("e2e_table_empty_after_select_9f3a");
    await expect(page.getByText("未找到匹配的文章")).toBeVisible();
    await expect(page.locator('thead input[type="checkbox"]')).toHaveCount(0);
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);

    await search.fill("");
    await expect(page.locator("tbody tr").filter({ hasText: title }).first()).toBeVisible();
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);
  });

  test("媒体库表格模式先选择再筛选为空：夹具附件上传后旧选择被清除", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    // 通过真实上传接口准备隔离夹具（服务端只接受图片类型，文件名由服务端随机生成）
    const png = Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg==",
      "base64",
    );
    const uploaded = await page.request.post("/api/v1/attachments/upload", {
      multipart: { file: { name: `e2e-empty-path-${Date.now()}.png`, mimeType: "image/png", buffer: png } },
    });
    expect(uploaded.ok(), `夹具附件必须上传成功（HTTP ${uploaded.status()}）`).toBe(true);
    const attachment = (await uploaded.json()) as { id: number; filename: string };
    expect(attachment.filename, "上传响应必须返回服务端文件名").toBeTruthy();

    await page.goto("/console/attachments");
    await expect(page.getByRole("heading", { name: "媒体库", exact: true })).toBeVisible();
    await page.locator('button[title="表格视图"]').click();

    const row = page.locator("tbody tr").filter({ hasText: attachment.filename }).first();
    await expect(row).toBeVisible({ timeout: 20_000 });
    await row.locator('input[type="checkbox"]').click();
    await expect(page.locator(".v-floating-dock")).toBeVisible();

    const search = page.getByPlaceholder("搜索文件名或 URL...");
    await search.fill("e2e_empty_after_select_9f3a");
    await expect(page.getByText("未找到匹配的附件")).toBeVisible();
    await expect(page.locator('thead input[type="checkbox"]')).toHaveCount(0);
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);

    await search.fill("");
    await expect(page.locator("tbody tr").filter({ hasText: attachment.filename }).first()).toBeVisible();
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);
  });
});
