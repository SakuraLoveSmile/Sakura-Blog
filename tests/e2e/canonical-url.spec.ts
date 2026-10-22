import { expect, test } from "@playwright/test";

test("公开页面 canonical、OG 与 sitemap 地址一致且编码路径可访问", async ({ page, request }) => {
  expect(
    (
      await request.post("/api/v1/auth/login", {
        data: { username: "admin", password: "e2e-password" },
      })
    ).ok(),
  ).toBeTruthy();
  const slug = `url-${Date.now()}-中文#?%`;
  const create = await request.post("/api/v1/posts", {
    data: {
      slug,
      title: "阶段 A URL 验证",
      content_md: "URL 验证正文",
      status: "published",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(create.status()).toBe(201);
  const path = `/post/${encodeURIComponent(slug)}`;
  const response = await page.goto(path);
  expect(response?.status()).toBe(200);
  const canonical = await page.locator('link[rel="canonical"]').getAttribute("href");
  expect(canonical).toBeTruthy();
  expect(new URL(canonical!).pathname).toBe(path);
  await expect(page.locator('meta[property="og:url"]')).toHaveAttribute("content", canonical!);
  const sitemap = await request.get("/sitemap.xml");
  expect(sitemap.ok()).toBeTruthy();
  expect(await sitemap.text()).toContain(`<loc>${canonical}</loc>`);
  expect((await request.get(new URL(canonical!).pathname)).status()).toBe(200);
});
