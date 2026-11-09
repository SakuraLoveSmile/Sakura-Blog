import { expect, test, type Page } from "@playwright/test";

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
    await expect(page.locator(".post-record")).toHaveCount(fixtures.length);

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
    const lastCard = page.locator(".post-record").last();
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

  test("管理员可通过侧栏菜单进入系统设置，手机端可通过更多抽屉进入，且支持无障碍焦点管理与关闭返回", async ({ page }, testInfo) => {
    await login(page);

    // 桌面端验证侧栏进入系统设置（“主题设置”旧入口已退役并重定向到设置页）
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/dashboard");

    const settingsNavLink = page.locator('aside a[href="/console/settings"]');
    await expect(settingsNavLink).toBeVisible();
    await settingsNavLink.click();
    await expect(page).toHaveURL(/\/console\/settings/);
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    // 手机端验证通过“更多”抽屉进入系统设置与焦点管理
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

    // 再次打开抽屉并点击系统设置
    await moreBtn.click();
    await expect(drawerDialog).toBeVisible();
    const drawerSettingsLink = page.locator('div[role="dialog"] a[href="/console/settings"]');
    await expect(drawerSettingsLink).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("t1-mobile-drawer.png") });
    await drawerSettingsLink.click();
    await expect(page).toHaveURL(/\/console\/settings/);
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
  });

  test("主按钮采用工具型小圆角、黑字高对比、无外发光", async ({ page }) => {
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
    // 方向 A：工作控件统一 6px 工具型圆角（原 8px 属上一版规范），显式 pill 仍可使用胶囊。
    const radiusNum = parseFloat(styles.borderRadius);
    expect(radiusNum).toBe(6);
  });

  test("分段 Tabs 与按钮支持键盘聚焦并具有清晰焦点环", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=appearance");
    await expect(page.getByRole("heading", { name: "主题外观" })).toBeVisible();

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
    const postAction = page.locator(".post-record .action-icon-btn").first();
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

  test("系统设置站点信息初始加载无未保存黄点误报，真实修改后出现黄点并可保存恢复", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=site");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    // 初始状态：无黄点、无未保存告警徽章
    const dirtyDots = page.locator(".settings-nav:visible .settings-nav-dot");
    await expect(dirtyDots).toHaveCount(0);
    const dirtyBadge = page.locator(".settings-dirty-badge");
    await expect(dirtyBadge).toHaveCount(0);

    // 模拟用户修改站点标题
    const titleInput = page.locator("#settings-site-title");
    await expect(titleInput).toBeVisible();
    await titleInput.fill("E2E Dirty Dot Title");

    // 此时出现黄点与脏标记：站点信息独立保存，只有“站点信息”一项带点
    await expect(dirtyDots).toHaveCount(1);
    await expect(dirtyBadge).toBeVisible();

    // 点击站点信息保存按钮
    const saveBtn = page.locator("#settings-page-site").getByRole("button", { name: "保存", exact: true });
    await saveBtn.click();

    // 保存成功后，黄点消失
    await expect(dirtyDots).toHaveCount(0);
    await expect(dirtyBadge).toHaveCount(0);
  });

  test("列表摘要保持四项事实、紧凑且无额外卡片层级", async ({ page }) => {
    await login(page);
    for (const width of [390, 1280]) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/console/posts");
      const summary = page.locator(".console-summary");
      await expect(summary).toBeVisible();
      const items = summary.locator(".stats-card");
      await expect(items).toHaveCount(4);
      for (const item of await items.all()) {
        await expect(item).toBeVisible();
        const box = await item.boundingBox();
        expect(box).not.toBeNull();
        expect(box!.height).toBeGreaterThanOrEqual(28);
        expect(box!.height).toBeLessThanOrEqual(88);
        const styles = await item.evaluate((el) => {
          const cs = getComputedStyle(el);
          return { shadow: cs.boxShadow, radius: cs.borderRadius, background: cs.backgroundColor };
        });
        expect(styles).toEqual({ shadow: "none", radius: "0px", background: "rgba(0, 0, 0, 0)" });
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
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

  test("旧分段页面重定向到系统设置：没有可停留的空页面，旧 feedback 链接仍可用", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });

    await page.goto("/console/menus");
    await expect(page).toHaveURL(/\/console\/settings\?tab=navigation$/);
    await expect(page.getByRole("heading", { name: "导航与页脚" })).toBeVisible();

    await page.goto("/console/appearance");
    await expect(page).toHaveURL(/\/console\/settings\?tab=appearance$/);
    await expect(page.getByRole("heading", { name: "主题外观" })).toBeVisible();
    await expect(page.locator('.appearance-tabs [role="tablist"]')).toHaveCount(0);

    await page.goto("/console/appearance?tab=feedback");
    await expect(page).toHaveURL(/\/console\/settings\?tab=feedback$/);
    await expect(page.getByRole("heading", { name: "访客反馈" })).toBeVisible();

    await page.goto("/console/authors");
    await expect(page).toHaveURL(/\/console\/settings\?tab=community$/);
    await expect(page.getByRole("heading", { name: "作者与友链" })).toBeVisible();

    // 侧栏不再出现旧分段页面的死胡同入口
    const retiredNavLinks = page.locator(
      'aside a[href="/console/appearance"], aside a[href="/console/menus"], aside a[href="/console/authors"]',
    );
    await expect(retiredNavLinks).toHaveCount(0);
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
    expect(userFirstBox!.height).toBeGreaterThanOrEqual(28);
    expect(userFirstBox!.height).toBeLessThanOrEqual(88);

    // 备份管理
    await page.goto("/console/backup");
    await expect(page.getByRole("heading", { name: "备份与导出" })).toBeVisible();
    const backupCards = page.locator(".stats-card");
    await expect(backupCards).toHaveCount(4);
    const backupFirstBox = await backupCards.first().boundingBox();
    expect(backupFirstBox).not.toBeNull();
    expect(backupFirstBox!.height).toBeGreaterThanOrEqual(28);
    expect(backupFirstBox!.height).toBeLessThanOrEqual(88);
  });

  test("设置页导航与页面区域语义：当前页 aria-current、区域由标题命名，非活动页保持 hidden", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    // 旧链接 ?tab=basic 落到“站点信息”。
    await page.goto("/console/settings?tab=basic");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    const nav = page.getByRole("navigation", { name: "设置分类" });
    await expect(nav).toBeVisible();
    // 分组导航：每个分组由自己的标题命名。
    for (const group of ["站点", "外观与布局", "内容", "互动", "扩展与高级"]) {
      await expect(nav.getByText(group, { exact: true })).toBeVisible();
    }
    const current = nav.locator('[aria-current="page"]');
    await expect(current).toHaveCount(1);
    await expect(current).toHaveAttribute("data-settings-page", "site");

    const sitePage = page.locator("#settings-page-site");
    await expect(sitePage).toBeVisible();
    expect(await sitePage.getAttribute("aria-labelledby")).toBe("settings-page-title-site");
    await expect(page.locator("#settings-page-title-site")).toHaveText("站点信息");

    // 其他页面区域保留草稿但保持 hidden。
    await expect(page.locator("#settings-page-post")).toHaveAttribute("hidden", "");
    await expect(page.locator("#settings-page-comments")).toHaveAttribute("hidden", "");
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
    await page.goto("/console/settings?tab=site");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    const titleInput = page.locator("#settings-site-title");
    await expect(titleInput).toBeVisible();

    // 初始清空脏标记
    const dirtyDots = page.locator(".settings-nav:visible .settings-nav-dot");
    const dirtyBadge = page.locator(".settings-dirty-badge");

    // 修改标题：未保存点只落在“站点信息”一项（SEO 与搜索属于主题草稿，不再与站点信息共用同一分区）
    await titleInput.fill("E2E Test Site");
    await expect(dirtyDots).toHaveCount(1);
    await expect(page.locator('.settings-nav:visible [data-settings-page="site"] .settings-nav-dot')).toHaveCount(1);
    await expect(dirtyBadge).toBeVisible();

    // 保存站点信息
    const saveBtn = page.locator("#settings-page-site").getByRole("button", { name: "保存", exact: true });
    await saveBtn.click();
    await expect(page.locator(".apple-toast-item")).toBeVisible();

    // 保存后脏标记消除
    await expect(dirtyDots).toHaveCount(0);
    await expect(dirtyBadge).toHaveCount(0);

    // 再次修改，黄点立即出现
    await titleInput.fill("E2E Test Site In-Flight");
    await expect(dirtyDots).toHaveCount(1);
    await expect(dirtyBadge).toBeVisible();
  });

  test("结构化数字输入越界时前置拦截：错误提示可见且保存按钮禁用", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=navigation");
    const original = await (await page.request.get("/api/v1/settings/stellar")).text();
    const saveButton = page
      .locator(".canonical-settings-form:visible")
      .getByRole("button", { name: "保存全部主题设置", exact: true });

    // 第三方评论参数与站点信息/评分/投票已从系统设置移除（改由站点原生能力承载），
    // 这里改用仍存在的结构化字段验证通用的无效输入前置拦截：出错时提示可见、保存禁用、不发请求，恢复后解除。
    // 先制造一处真实修改，让保存按钮在修复输入错误后处于可用状态。
    await page.locator('[data-structured-path="theme.topbar.enabled"]').getByRole("switch").click();
    await expect(saveButton).toBeEnabled();

    // 菜单列数在 schema 中限定为 1–5，填 9 必须被结构化表单拦下。
    const columns = page.locator('[data-structured-path="theme.leftbar.menu_columns"]');
    await columns.locator('input[type="number"]').fill("9");
    await expect(columns.getByRole("alert")).toHaveText("请输入范围内的有效数字。");
    await expect(saveButton).toBeDisabled();
    expect(await (await page.request.get("/api/v1/settings/stellar")).text()).toBe(original);

    await columns.getByRole("button", { name: "菜单列数恢复默认", exact: true }).click();
    await expect(columns.getByRole("alert")).toHaveCount(0);
    await expect(saveButton).toBeEnabled();
  });
});

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
    tableToggle: "列表视图",
  },
  {
    path: "/console/pages",
    heading: "独立页面",
    search: "搜索页面标题或 Slug...",
    emptyTitle: "未找到匹配的页面",
    tableToggle: "列表视图",
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

// 系统设置已由“13 个横向页签”改为“分组导航 → 设置页”：桌面是左侧导航，手机是分组列表 → 详情。
// 原先对设置页 VTabbar(autoReveal) 的横向显露断言随页签一并退役，对应契约改为下面的导航断言；
// VTabbar autoReveal 组件自身的源码契约仍由 modules/console/tests/components-a11y.test.ts 覆盖。
test.describe("Console 设置导航与空列表选择入口", () => {
  test("候选验收截图：设置导航（390 分组列表 / 详情，1280 桌面）与主题外观", async ({ page }, testInfo) => {
    await login(page);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await expect(page.locator(".settings-nav.is-list")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("settings-list-390.png") });

    await page.goto("/console/settings?tab=advanced");
    await expect(page.locator("#settings-page-advanced")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("settings-detail-390.png") });

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/console/settings?tab=appearance");
    await expect(page.getByRole("heading", { name: "主题外观" })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("tabs-appearance-1280.png") });
  });

  test("390px 系统设置为“分组列表 → 详情”：列表与详情均无横向溢出，进入详情聚焦标题，返回后回到列表", async ({ page }) => {
    await login(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();

    const overflow = () =>
      page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    const list = page.locator(".settings-nav.is-list");
    await expect(list).toBeVisible();
    // 列表阶段只显示分组列表，不渲染任何一页的内容。
    await expect(page.locator(".settings-content")).toBeHidden();
    const items = list.locator(".settings-nav-item");
    await expect(items).toHaveCount(12);
    // 每一项都是整行点击区（≥56px），并带一句说明。
    for (const height of await items.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height)))
      expect(height).toBeGreaterThanOrEqual(56);
    await expect(items.first().locator(".settings-nav-summary")).toBeVisible();
    expect(await overflow(), "分组列表不得横向溢出").toBeLessThanOrEqual(1);

    // 选择末尾一项进入详情：路由带 ?tab=，焦点落在页面标题，列表隐藏，出现“返回设置”。
    await list.locator('[data-settings-page="advanced"]').click();
    await expect(page).toHaveURL(/tab=advanced/);
    await expect(page.locator("#settings-page-advanced")).toBeVisible();
    await expect(page.locator("#settings-page-title-advanced")).toBeFocused();
    await expect(list).toBeHidden();
    expect(await overflow(), "详情页不得横向溢出").toBeLessThanOrEqual(1);

    await page.getByRole("button", { name: "返回设置", exact: true }).click();
    await expect(list).toBeVisible();
    await expect(page.locator(".settings-content")).toBeHidden();
    await expect(page).not.toHaveURL(/tab=/);
    expect(await overflow()).toBeLessThanOrEqual(1);
  });

  test("query 直接命中末尾页面时初始加载即进入详情；桌面短屏下导航自行滚动并显露当前页", async ({ page }) => {
    await login(page);

    // 手机：带 ?tab= 的链接直接进入详情，而不是停在分组列表。
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/settings?tab=advanced");
    await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
    await expect(page.locator("#settings-page-advanced")).toBeVisible();
    await expect(page.locator(".settings-nav.is-list")).toBeHidden();
    await expect(page.getByRole("button", { name: "返回设置", exact: true })).toBeVisible();

    // 桌面短屏：侧栏有自己的纵向滚动区，初始加载就把当前页显露在侧栏可视范围内，整页纵向位置不被带动。
    await page.setViewportSize({ width: 1280, height: 560 });
    await page.goto("/console/settings?tab=advanced");
    const sidebar = page.locator(".settings-sidebar");
    const current = page.locator('.settings-nav:visible [data-settings-page="advanced"]');
    await expect(current).toHaveAttribute("aria-current", "page");
    const reveal = () =>
      sidebar.evaluate((aside) => {
        const item = aside.querySelector('[aria-current="page"]') as HTMLElement;
        const view = aside.getBoundingClientRect();
        const box = item.getBoundingClientRect();
        return {
          scrollable: aside.scrollHeight > aside.clientHeight,
          scrollTop: aside.scrollTop,
          hiddenAbove: Math.round(view.top - box.top),
          hiddenBelow: Math.round(box.bottom - view.bottom),
        };
      });
    await expect.poll(async () => (await reveal()).scrollable, { message: "560px 高的桌面视口下侧栏应可滚动" }).toBe(true);
    await expect.poll(async () => (await reveal()).scrollTop, { timeout: 8_000 }).toBeGreaterThan(0);
    const shown = await reveal();
    expect(shown.hiddenAbove).toBeLessThanOrEqual(1);
    expect(shown.hiddenBelow).toBeLessThanOrEqual(1);
    // 显露只滚动侧栏自己：窗口与应用主滚动区的纵向位置都不变。
    expect(
      await page.evaluate(() => ({
        windowY: window.scrollY,
        mainScrollTop: (document.querySelector("main.content") as HTMLElement | null)?.scrollTop ?? 0,
      })),
    ).toEqual({ windowY: 0, mainScrollTop: 0 });
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

    await page.getByRole("button", { name: "海报卡片视图", exact: true }).click();
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

  test("记录列表先选择再筛选为空：选择入口与批量浮条一并消失", async ({ page }) => {
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
    await page.locator('button[title="列表视图"]').click();

    const row = page.locator(".post-record").filter({ hasText: title }).first();
    await expect(row).toBeVisible();
    await row.locator('input[type="checkbox"]').click();
    await expect(page.locator(".v-floating-dock")).toBeVisible();

    const search = page.getByPlaceholder("搜索文章标题或 Slug...");
    await search.fill("e2e_table_empty_after_select_9f3a");
    await expect(page.getByText("未找到匹配的文章")).toBeVisible();
    await expect(page.locator('.post-record input[type="checkbox"]')).toHaveCount(0);
    await expect(page.locator(".v-floating-dock")).toHaveCount(0);

    await search.fill("");
    await expect(page.locator(".post-record").filter({ hasText: title }).first()).toBeVisible();
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

  test("仪表盘默认布局保持声明的紧凑尺寸，不被加载归一化扩回", async ({ page }, testInfo) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/dashboard");
    const stat = page.locator('.grid-stack-item[gs-id="stat-posts-1"]');
    const trend = page.locator('.grid-stack-item[gs-id="trend-chart-1"]');
    await expect(stat).toBeVisible({ timeout: 20_000 });
    await expect(trend).toBeVisible();
    const statBox = (await stat.boundingBox())!;
    const trendBox = (await trend.boundingBox())!;
    // 12 行 × 8px + 外边距约 96px；44 行约 352px。
    expect(statBox.height, "1280 统计卡应保持 h12 紧凑高度").toBeGreaterThanOrEqual(88);
    expect(statBox.height, "1280 统计卡不得被扩成 h19").toBeLessThanOrEqual(112);
    expect(trendBox.height, "1280 趋势卡应保持 h44 紧凑高度").toBeGreaterThanOrEqual(330);
    expect(trendBox.height, "1280 趋势卡不得被扩成 h52").toBeLessThanOrEqual(380);
    expect(await stat.getAttribute("gs-h")).toBe("12");
    expect(await trend.getAttribute("gs-h")).toBe("44");
    const quick = await page.locator(".dashboard-quick-links a").allTextContents();
    expect(quick).toEqual(["写文章", "评论", "分类", "设置"]);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/dashboard");
    await expect(stat).toBeVisible({ timeout: 20_000 });
    const stats = await page.locator('.grid-stack-item[gs-id^="stat-"]').evaluateAll((nodes) =>
      nodes.map((node) => {
        const rect = node.getBoundingClientRect();
        return { x: node.getAttribute("gs-x"), w: node.getAttribute("gs-w"), h: rect.height, top: rect.top };
      }),
    );
    expect(stats).toHaveLength(4);
    expect(new Set(stats.map((item) => item.x))).toEqual(new Set(["0", "2"]));
    for (const item of stats) {
      expect(item.w, "390 统计卡保持 2 列").toBe("2");
      expect(item.h, "390 统计卡不得保留旧 h14/h18 的空白").toBeGreaterThanOrEqual(88);
      expect(item.h).toBeLessThanOrEqual(112);
    }
    await page.screenshot({ path: testInfo.outputPath("dashboard-compact-390.png") });
  });

  test("海报视图选择有可见反馈，置顶信息位于可读 meta 行", async ({ page }, testInfo) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    const token = `poster-${Date.now()}`;
    const title = `海报选择反馈 ${token}`;
    const created = await page.request.post("/api/v1/posts", {
      data: {
        slug: token,
        title,
        content_md: "海报卡片选择反馈夹具",
        status: "published",
        pinned: true,
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(created.ok(), await created.text()).toBeTruthy();

    await page.goto("/console/posts");
    await page.getByRole("button", { name: "海报卡片视图" }).click();
    const card = page.locator(".post-card").filter({ hasText: title }).first();
    await expect(card).toBeVisible();
    await card.locator(".post-select").click();
    await expect(card).toHaveClass(/is-selected/);
    // 背景色有过渡，必须等计算值真正落到 accent 而不是过渡起点。
    await expect
      .poll(() => card.evaluate((el) => getComputedStyle(el).backgroundColor), { timeout: 5_000 })
      .not.toBe("rgba(0, 0, 0, 0)");
    await expect
      .poll(() => card.locator(".post-card-actions").evaluate((el) => getComputedStyle(el).opacity), { timeout: 5_000 })
      .toBe("1");

    const pinned = card.locator(".post-pinned");
    await expect(pinned).toBeVisible();
    expect(await pinned.evaluate((el) => getComputedStyle(el).position)).not.toBe("absolute");
    await expect(card.locator(".post-card-meta .post-pinned")).toHaveCount(1);

    await page.setViewportSize({ width: 390, height: 844 });
    const selectBox = (await card.locator(".post-select").boundingBox())!;
    expect(selectBox.width, "390 海报选择命中区 ≥44px").toBeGreaterThanOrEqual(44);
    expect(selectBox.height, "390 海报选择命中区 ≥44px").toBeGreaterThanOrEqual(44);
    await page.screenshot({ path: testInfo.outputPath("poster-selected-390.png") });
  });

  test("方向 A 图形与文字可访问性：设置页标题唯一、头像圆形、标签插件重试对比度、模块单层", async ({ page }) => {
    await login(page);
    const allSettings = await page.request.get("/api/v1/settings");
    expect(allSettings.ok(), await allSettings.text()).toBeTruthy();
    const site = ((await allSettings.json()) as Record<string, unknown>).site as Record<string, unknown>;
    const restoreSite = async () => {
      await page.request.put("/api/v1/settings/site", { data: site });
    };
    await page.request.put("/api/v1/settings/site", {
      data: { ...site, avatar: "/images/e2e-avatar.png", avatar_bg: "/images/e2e-avatar-bg.png" },
    });

    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/settings?tab=site");
    // 页面标题只由页面头部提供；标题下只有一句话说明，没有单独的“效果说明条”，也就没有重复的同义标题。
    // 设置页文字的对比度由 console-settings-ia 的 Q2 逐项断言。
    await expect(page.getByRole("heading", { name: "站点信息", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "站点信息", exact: true })).toHaveCount(1);
    await expect(page.locator(".settings-effect-guide")).toHaveCount(0);
    const avatar = await page.locator(".settings-avatar-128").evaluate((el) => {
      const style = getComputedStyle(el);
      return { radius: style.borderTopLeftRadius, width: el.getBoundingClientRect().width };
    });
    expect(
      avatar.radius === "50%" || Math.abs(parseFloat(avatar.radius) * 2 - avatar.width) < 1,
      `头像预览必须保持圆形，实际 ${avatar.radius}`,
    ).toBe(true);

    await page.route("**/api/v1/plugins/catalog", (route) => route.fulfill({ status: 500, json: { error: "fixture catalog failure" } }));
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/plugins/shortcodes");
    const retry = page.locator(".retry-button");
    await expect(retry).toBeVisible();
    const retryStyles = await retry.evaluate((el) => {
      const style = getComputedStyle(el);
      return { color: style.color, background: style.backgroundColor };
    });
    expect(retryStyles.color).toBe("rgb(0, 0, 0)");
    expect(retryStyles.background).not.toBe("rgb(0, 0, 0)");

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/console/plugins/shortcodes");
    await expect(retry).toBeVisible();
    const mobileRetry = (await retry.boundingBox())!;
    expect(mobileRetry.height, "390 标签插件重试按钮命中区 ≥44px").toBeGreaterThanOrEqual(44);
    expect(mobileRetry.width, "390 标签插件重试按钮命中区 ≥44px").toBeGreaterThanOrEqual(44);

    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/modules");
    await expect(page.getByRole("heading", { name: "功能模块" })).toBeVisible();
    await expect(page.locator("main.content h1 svg")).toHaveCount(0);
    expect(await page.locator(".module-record").first().evaluate((el) => getComputedStyle(el).backgroundColor)).toBe("rgba(0, 0, 0, 0)");

    // 旧的“菜单”页已退役：旧地址必须跳到系统设置，不留下可停留的空页面。
    await page.goto("/console/menus");
    await expect(page).toHaveURL(/\/console\/settings\?tab=navigation$/);
    await expect(page.getByRole("heading", { name: "导航与页脚" })).toBeVisible();
    await restoreSite();
  });

  test("运行时字号白名单包含继承值：仪表盘/设置/外观/评论/文章无 16px 泄漏", async ({ page }) => {
    await login(page);
    const pages = [
      "/console/dashboard",
      "/console/settings?tab=basic",
      "/console/settings?tab=appearance",
      "/console/settings?tab=navigation",
      "/console/comments",
      "/console/posts",
    ];
    await page.setViewportSize({ width: 1280, height: 900 });
    const violations: unknown[] = [];
    for (const url of pages) {
      await page.goto(url);
      await expect(page.locator("main.content h1").first()).toBeVisible();
      const bad = await page.evaluate(() => {
        const root = document.querySelector("main.content");
        if (!root) return [{ text: "missing main.content", size: 0 }];
        const allowed = new Set([11, 12, 13, 14, 20, 24]);
        const result: { text: string; size: number; tag: string; cls: string }[] = [];
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
          const text = node.textContent?.trim();
          if (!text) continue;
          const el = node.parentElement;
          if (!el || el.closest("script,style,svg,.sr-only")) continue;
          const style = getComputedStyle(el);
          const rect = el.getBoundingClientRect();
          if (style.display === "none" || style.visibility === "hidden" || rect.width === 0 || rect.height === 0) continue;
          const size = Math.round(parseFloat(style.fontSize) * 100) / 100;
          if (!allowed.has(size)) {
            result.push({ text: text.slice(0, 24), size, tag: el.tagName, cls: typeof el.className === "string" ? el.className : "" });
          }
        }
        return result;
      });
      for (const item of bad) violations.push({ url, ...item });
    }
    expect(violations, `继承字号必须收敛到 11/12/13/14/20/24：${JSON.stringify(violations, null, 2)}`).toEqual([]);
  });

  test("组件尺寸弹窗与识别共用宽×高：sm/xxs 中档组件三档可见且选中", async ({ page }, testInfo) => {
    await login(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/dashboard/designer");
    await expect(page.getByRole("heading", { name: "设置仪表盘", exact: true })).toBeVisible();

    const widgets = ["trend-chart-1", "recent-posts-1", "recent-comments-1", "category-donut-1", "stellar-version-1"];
    for (const device of ["平板", "手机"]) {
      await page.getByRole("tab", { name: device }).click();
      for (const widgetId of widgets) {
        const item = page.locator(`.grid-stack-item[gs-id="${widgetId}"]`);
        await expect(item, `${widgetId} 在${device}断点可见`).toBeVisible();
        await item.getByRole("button", { name: "选择组件大小" }).click();
        const dialog = page.getByRole("dialog", { name: "组件大小" });
        await expect(dialog).toBeVisible();
        const options = dialog.locator(".size-preset-option");
        await expect(options, `${widgetId}/${device} 必须列出三档`).toHaveCount(3);
        for (const preset of ["small", "medium", "large"]) {
          await expect(dialog.locator(`[data-size-preset="${preset}"]`), `${widgetId}/${device} 缺少 ${preset}`).toBeVisible();
        }
        await expect(
          dialog.locator('[data-size-preset="medium"]'),
          `${widgetId}/${device} 当前尺寸应识别为中档并选中`,
        ).toHaveAttribute("aria-pressed", "true");
        await expect(dialog.locator('[data-size-preset="medium"]')).toHaveClass(/is-selected/);
        if (widgetId === "trend-chart-1") {
          const sizes = await options.locator(".size-preset-grid-size").allTextContents();
          expect(sizes, `趋势 ${device} 三档必须是可区分的宽×高`).toEqual(
            device === "平板" ? ["6 列 × 36 行", "6 列 × 44 行", "6 列 × 52 行"] : ["4 列 × 32 行", "4 列 × 42 行", "4 列 × 50 行"],
          );
          await page.screenshot({ path: testInfo.outputPath(`size-modal-${device === "平板" ? "sm" : "xxs"}.png`) });
        }
        await dialog.getByRole("button", { name: "取消", exact: true }).click();
        await expect(dialog).toBeHidden();
      }
    }
  });

  test("评论快捷键帮助保持紧凑入口、44px 命中区与可访问弹层", async ({ page }, testInfo) => {
    await login(page);
    for (const width of [1280, 390]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      await page.goto("/console/comments");
      const trigger = page.getByRole("button", { name: "评论快捷键" });
      await expect(trigger).toBeVisible();
      const box = (await trigger.boundingBox())!;
      expect(box.width, `${width} 快捷键入口宽度 ≥44px`).toBeGreaterThanOrEqual(44);
      expect(box.height, `${width} 快捷键入口高度 ≥44px`).toBeGreaterThanOrEqual(44);
      await trigger.click();
      const dialog = page.getByRole("dialog", { name: "评论快捷键" });
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText("J / K");
      await expect(dialog).toContainText("批准通过");
      await expect(dialog).toContainText("勾选 / 取消");
      await page.screenshot({ path: testInfo.outputPath(`comments-shortcuts-${width}.png`) });
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(trigger).toHaveAttribute("aria-expanded", "false");
    }
  });

  test("评论 J/K 焦点行同步显示操作按钮", async ({ page }) => {
    await login(page);
    const token = Date.now();
    const content = `J/K 焦点行夹具 ${token}`;
    const created = await page.request.post("/api/comments", {
      data: {
        page_key: `/post/focus-${token}`,
        nick: "Focus Reader",
        email: "admin@e2e.test",
        content,
      },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/console/comments");
    const row = page.locator(".comment-row").filter({ hasText: content }).first();
    await expect(row).toBeVisible();
    await page.keyboard.press("j");
    await expect(row).toHaveClass(/is-focused/);
    await expect
      .poll(() => row.locator(".comment-actions").evaluate((el) => getComputedStyle(el).opacity), { timeout: 5_000 })
      .toBe("1");
  });
});

// 公共外壳和控件影响所有模块：从真实 manifest 导航逐页验证桌面与手机。
test("Console 全部主导航在桌面和手机可达且无整页横向溢出", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  const errors: string[] = [];
  const layoutIssues: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await login(page);
  await ensurePosts(page, 4, `workspace-review-${Date.now()}`);
  const paths = await page.locator('aside a[href^="/console/"]').evaluateAll((links) =>
    Array.from(new Set(links.map((link) => new URL((link as HTMLAnchorElement).href).pathname))),
  );
  expect(paths.length).toBeGreaterThanOrEqual(20);
  // /console/appearance、/console/menus、/console/authors 已退役并重定向到系统设置，不得再挂在侧栏。
  for (const retired of ["/console/appearance", "/console/menus", "/console/authors"]) {
    expect(paths, `${retired} 已退役，不应再出现在侧栏`).not.toContain(retired);
  }
  // 这两个侧栏分组入口按已登记路由跳转到默认子页，仍精确核验最终地址。
  const destinations: Record<string, string> = {
    "/console/plugins": "/console/plugins/shortcodes",
    "/console/seo": "/console/seo/overview",
    // 文库与项目管理合并后，侧栏「项目管理」仍指向 /wiki，落在项目列表。
    "/console/wiki": "/console/wiki/projects",
  };
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
    for (const path of paths) {
      await page.goto(path);
      await expect(page).toHaveURL(new RegExp(`${destinations[path] ?? path}$`));
      await expect(page.locator("main.content h1").first(), path).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), { message: `${path} 在 ${width}px 不得整页横向溢出` }).toBe(true);
      const main = page.locator("main.content");
      const geometry = await main.evaluate((el) => ({ content: el.scrollWidth, viewport: el.clientWidth }));
      if (geometry.content > geometry.viewport + 1) {
        layoutIssues.push(`${path} @ ${width}: ${geometry.content}px / ${geometry.viewport}px`);
        await page.screenshot({ path: testInfo.outputPath(`overflow-${path.split("/").pop()}-${width}.png`) });
      }
      if (["posts", "pages", "settings", "dashboard", "comments", "attachments", "wiki", "users"].includes(path.split("/").pop() ?? "")) {
        await page.screenshot({ path: testInfo.outputPath(`workspace-${path.split("/").pop()}-${width}.png`) });
      }
    }
  }
  await testInfo.attach("navigation-report", { body: JSON.stringify({ paths, destinations, widths: [1280, 390], errors, layoutIssues }, null, 2), contentType: "application/json" });
  expect(errors, "主导航切换不应产生未捕获错误").toEqual([]);
  expect(layoutIssues, "各页主工作区不得横向溢出").toEqual([]);
});
