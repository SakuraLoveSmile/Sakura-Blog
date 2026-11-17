// 仪表盘「实例配置 + 叠放容器」端到端验收。
// 参照 Halo 的 per-widget config 与 core:stack 机制，验证本项目实现：
// 1) 同类型两个实例的配置互相独立并落库；2) 叠放容器在只读页可切换子组件。
import { expect, test, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

/** 在设计器里添加一个叠放容器（控件中心的同名条目）。 */
async function addStack(page: Page) {
  await page.getByRole("button", { name: "添加组件" }).click();
  const center = page.getByRole("dialog", { name: "添加组件" });
  await expect(center).toBeVisible();
  await center.getByRole("button", { name: /叠放容器/ }).first().click();
  await expect(page.locator('.grid-stack-item[data-widget-id="stack"]').first()).toBeVisible();
}

test.describe("仪表盘组件实例配置", () => {
  test("热门文章配置 8 条会请求并显示 8 条，刷新后仍保留", async ({ page }) => {
    // 按实际请求的 limit 返回条数，确保固定请求 5 条的旧实现会失败。
    await page.route("**/api/v1/stats/popular?*", async (route) => {
      const limit = Number(new URL(route.request().url()).searchParams.get("limit"));
      await route.fulfill({ json: { items: Array.from({ length: Math.min(limit, 8) }, (_, index) => ({
        path: `/post/popular-${index}`, title: `热门夹具 ${index + 1}`, pv: 80 - index, uv: 8,
      })) } });
    });
    await login(page);
    await page.goto("/console/dashboard/designer");
    await page.getByRole("button", { name: "添加组件" }).click();
    await page.getByRole("dialog", { name: "添加组件" }).getByRole("button", { name: /热门文章/ }).first().click();
    const card = page.locator('.grid-stack-item[data-widget-id="top-posts"]').first();
    await card.getByRole("button", { name: "组件设置" }).click();
    const dialog = page.getByRole("dialog", { name: "组件设置" });
    await dialog.locator("select").selectOption("8");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await page.getByRole("button", { name: "保存布局" }).click();
    await expect(page).toHaveURL(/\/console\/dashboard$/);
    const rows = page.locator('.grid-stack-item[data-widget-id="top-posts"]').first().locator(".top-post-row");
    await expect(rows).toHaveCount(8);
    await page.reload();
    await expect(rows).toHaveCount(8);
  });

  test("叠放容器的配置只作用于自身实例，并能在只读页切换子组件", async ({ page }) => {
    await login(page);
    await page.goto("/console/dashboard/designer");
    await expect(page.getByRole("heading", { name: "设置仪表盘", exact: true })).toBeVisible();

    // 清掉历史 stack（用例可重复运行）
    const existing = page.locator('.grid-stack-item[data-widget-id="stack"]');
    for (let i = await existing.count(); i > 0; i -= 1) {
      await existing.first().getByRole("button", { name: "移除组件" }).click();
      await page.waitForTimeout(200);
    }

    await addStack(page);
    const first = page.locator('.grid-stack-item[data-widget-id="stack"]').first();
    await first.getByRole("button", { name: "组件设置" }).click();
    const dialog = page.getByRole("dialog", { name: "组件设置" });
    await expect(dialog).toBeVisible();
    // 默认选 3 个：最近文章 / 最近评论 / 热门文章
    await expect(dialog.locator(".config-multi-item.is-selected")).toHaveCount(3);
    // 多选里必须没有「叠放容器」自身，避免无限嵌套
    await expect(dialog.getByRole("button", { name: "叠放容器", exact: true })).toHaveCount(0);
    await dialog.getByRole("button", { name: "热门文章", exact: true }).click();
    await dialog.locator("select").last().selectOption("8");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toBeHidden();

    // 第二个实例：只留「热门文章」，验证配置隔离
    await page.getByRole("button", { name: "添加组件" }).click();
    const center = page.getByRole("dialog", { name: "添加组件" });
    await center.getByRole("button", { name: /叠放容器/ }).first().click();
    await expect(page.locator('.grid-stack-item[data-widget-id="stack"]')).toHaveCount(2);
    const second = page.locator('.grid-stack-item[data-widget-id="stack"]').nth(1);
    await second.getByRole("button", { name: "组件设置" }).click();
    const dialog2 = page.getByRole("dialog", { name: "组件设置" });
    await expect(dialog2).toBeVisible();
    const preset = ["最近文章", "最近评论", "热门文章"];
    for (const label of preset) {
      const item = dialog2.getByRole("button", { name: label, exact: true });
      if ((await item.getAttribute("aria-pressed")) === "true") await item.click();
    }
    await dialog2.getByRole("button", { name: "热门文章", exact: true }).click();
    await dialog2.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog2).toBeHidden();

    // 保存并回读：两个实例的 config 不同
    await page.getByRole("button", { name: "保存布局" }).click();
    await expect(page).toHaveURL(/\/console\/dashboard$/, { timeout: 20_000 });
    const resp = await page.request.get("/api/v1/dashboard/layout");
    expect(resp.ok()).toBe(true);
    const doc = await resp.json();
    const stacks = (doc.layouts?.lg ?? []).filter((item: { id: string }) => item.id === "stack");
    expect(stacks).toHaveLength(2);
    const configs = stacks.map((item: { config?: Record<string, unknown> }) => item.config ?? {});
    const withInterval = configs.find((config: Record<string, unknown>) => config.interval === "8");
    const withoutInterval = configs.find((config: Record<string, unknown>) => config.interval === "0");
    expect(withInterval, "第一个实例应保存 8 秒轮播").toBeTruthy();
    expect(withoutInterval, "第二个实例应保持不轮播").toBeTruthy();
    expect(withInterval.stackWidgets).toEqual(["recent-posts", "recent-comments"]);
    expect(withoutInterval.stackWidgets).toEqual(["top-posts"]);

    // 只读页：两个实例各自按自己的配置渲染
    await page.reload();
    const viewStacks = page.locator('.grid-stack-item[data-widget-id="stack"]');
    await expect(viewStacks).toHaveCount(2);
    await expect(viewStacks.nth(0).locator(".stack-tab")).toHaveCount(2);
    // 只有一个子组件时不渲染页签栏
    await expect(viewStacks.nth(1).locator(".stack-tab")).toHaveCount(0);

    // 切换子组件：内容真的换了
    const tabs = viewStacks.nth(0).locator(".stack-tab");
    await expect(tabs.first()).toHaveClass(/is-active/);
    await tabs.nth(1).click();
    await expect(tabs.nth(1)).toHaveClass(/is-active/);
    await expect(tabs.first()).not.toHaveClass(/is-active/);
  });

  test("未声明 configFields 的组件不出现「组件设置」按钮", async ({ page }) => {
    await login(page);
    await page.goto("/console/dashboard/designer");
    await expect(page.getByRole("heading", { name: "设置仪表盘", exact: true })).toBeVisible();
    // 统计卡没有配置项
    const stat = page.locator('.grid-stack-item[data-widget-id="stat-posts"]').first();
    await expect(stat).toBeVisible();
    await expect(stat.getByRole("button", { name: "组件设置" })).toHaveCount(0);
    await expect(stat.getByRole("button", { name: "选择组件大小" })).toBeVisible();
  });

  test("趋势卡的默认区间与 UV 配置生效且可临时切换", async ({ page }) => {
    await login(page);
    await page.goto("/console/dashboard/designer");
    const trend = page.locator('.grid-stack-item[data-widget-id="trend-chart"]').first();
    await expect(trend).toBeVisible();
    await trend.getByRole("button", { name: "组件设置" }).click();
    const dialog = page.getByRole("dialog", { name: "组件设置" });
    await expect(dialog).toBeVisible();
    // 两个配置项：默认区间 + 默认显示 UV
    await expect(dialog.locator(".config-field")).toHaveCount(2);
    await dialog.locator("select").first().selectOption("30");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toBeHidden();
    await page.getByRole("button", { name: "保存布局" }).click();
    await expect(page).toHaveURL(/\/console\/dashboard$/, { timeout: 20_000 });

    const resp = await page.request.get("/api/v1/dashboard/layout");
    const doc = await resp.json();
    const saved = (doc.layouts?.lg ?? []).find((item: { id: string }) => item.id === "trend-chart");
    expect(saved?.config?.defaultRange).toBe("30");

    // 只读页默认落在 30 天
    await page.reload();
    const trendView = page.locator('.grid-stack-item[data-widget-id="trend-chart"]').first();
    await expect(trendView.getByRole("button", { name: "30 天" })).toHaveClass(/active/);
    // 仍允许临时切回 7 天，不回写配置
    await trendView.getByRole("button", { name: "7 天" }).click();
    await expect(trendView.getByRole("button", { name: "7 天" })).toHaveClass(/active/);
  });
});
