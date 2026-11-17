import { expect, test, type Page } from "@playwright/test";
import { loginToDashboard, seedCardData, cleanupCardData, resetDashboardLayout } from "./support/dashboard-cards";
import { WIDGET_PRESET_FOOTPRINTS } from "../../modules/console/web/src/modules/dashboard/widgets/layout";

async function layout(page: Page, entries: { id: string; preset?: "small" | "medium" | "large"; config?: Record<string, unknown> }[]) {
  const layouts = Object.fromEntries(["lg", "sm", "xxs"].map((bp) => {
    return [bp, entries.map(({ preset = "large", ...entry }, index) => {
      const size = WIDGET_PRESET_FOOTPRINTS[bp as "lg" | "sm" | "xxs"][preset];
      return { ...entry, i: `${entry.id}-${index}`, x: 0, y: index * 64, ...size };
    })];
  }));
  const response = await page.request.put("/api/v1/dashboard/layout", { data: { version: 10, layouts } });
  expect(response.ok()).toBe(true);
  await page.goto("/console/dashboard");
  await page.reload();
}

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await loginToDashboard(page);
  await seedCardData(page);
});

test("旧热门文章实例没有配置时仍按小中大档显示2/4/5条", async ({ page }) => {
  await page.route("**/api/v1/stats/popular?*", (route) => route.fulfill({ json: { items: Array.from({ length: 5 }, (_, index) => ({
    path: `/post/legacy-popular-${index}`, title: `热门夹具 ${index + 1}`, pv: 50 - index, uv: 5,
  })) } }));
  await layout(page, ["small", "medium", "large"].map((preset) => ({ id: "top-posts", preset: preset as "small" | "medium" | "large" })));
  for (const [preset, count] of [["small", 2], ["medium", 4], ["large", 5]] as const) {
    await expect(page.locator(`.top-posts-${preset} .top-post-row`)).toHaveCount(count);
  }
});

test.afterEach(async ({ page }) => {
  await resetDashboardLayout(page);
  await cleanupCardData(page);
});

test("旧布局没有实例配置时保留大档文章7条、评论8条和全部快捷操作", async ({ page }) => {
  await page.route("**/api/v1/dashboard/recent-posts", (route) => route.fulfill({ json: { items: Array.from({ length: 8 }, (_, index) => ({
    slug: `legacy-${index}`, title: `旧布局文章 ${index + 1}`, category: "夹具", status: "published", updated_at: "2026-11-17",
  })) } }));
  await page.route("**/api/v1/dashboard/recent-comments", (route) => route.fulfill({ json: { items: Array.from({ length: 8 }, (_, index) => ({
    id: index + 1, nick: `旧布局访客 ${index + 1}`, email: "fixture@e2e.test", content: "评论夹具", page_key: "/post/fixture", status: "approved", date: "2026-11-17", avatar: "",
  })) } }));
  await page.route("**/api/v1/dashboard/quick-actions", (route) => route.fulfill({ json: { items: [
    "write_post", "review_comments", "manage_categories", "settings", "manage_posts", "upload_attachment", "system_update", "open_frontend",
  ] } }));
  await layout(page, [{ id: "recent-posts" }, { id: "recent-comments" }, { id: "quick-actions" }]);
  await expect(page.locator(".recent-posts-large .apple-list-item")).toHaveCount(7);
  await expect(page.locator(".recent-comments-large .apple-comment-item")).toHaveCount(8);
  await expect(page.locator(".quick-card-large .quick-action-item")).toHaveCount(8);
});

test("叠放自动轮播不会丢掉正在输入的评论回复", async ({ page }) => {
  await layout(page, [{ id: "stack", config: { stackWidgets: ["recent-comments", "recent-posts"], interval: "8" } }]);
  const stack = page.locator(".stack-widget");
  await stack.locator(".action-reply").first().click();
  const draft = stack.getByRole("textbox", { name: "回复内容" });
  await draft.fill("尚未发送的叠放回复草稿");
  // 超过用户配置的完整轮播周期，旧实现此时会卸载评论组件并丢掉草稿。
  await page.waitForTimeout(8500);
  await expect(draft).toBeVisible();
  await expect(draft).toHaveValue("尚未发送的叠放回复草稿");
  await expect(stack.locator(".stack-tab.is-active")).toHaveText("最近评论");
  await stack.getByRole("tab", { name: "最近文章" }).click();
  await expect(draft).toHaveCount(0);
  await stack.getByRole("tab", { name: "最近评论" }).click();
  await expect(draft).toHaveValue("尚未发送的叠放回复草稿");
});
