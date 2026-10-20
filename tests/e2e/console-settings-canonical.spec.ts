import { expect, test, type Page } from "@playwright/test";
import path from "node:path";

const endpoint = "/api/v1/settings/stellar";
const activeForm = (page: Page) => page.locator(".canonical-settings-form:visible");
const field = (page: Page, key: string) => activeForm(page).locator(`[data-setting-path="${key}"]`);
const input = (page: Page, key: string) => field(page, key).locator("input,textarea").first();
const save = (page: Page) => activeForm(page).getByRole("button", { name: "保存全部主题设置", exact: true });

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function tab(page: Page, label: string) {
  const keys: Record<string, string> = { "Stellar 2": "stellar2", 基本: "basic", 文章: "post", 评论: "comments", SEO: "seo", 搜索: "search", 反馈: "feedback", 高级: "advanced" };
  const target = page.locator(`#tab-${keys[label]}`);
  await target.click();
  await expect(target).toHaveAttribute("aria-selected", "true");
}

async function toggle(page: Page, key: string, on: boolean) {
  const control = field(page, key).getByRole("switch");
  if ((await control.getAttribute("aria-checked")) !== String(on)) await control.click();
  await expect(control).toHaveAttribute("aria-checked", String(on));
}

async function put(page: Page, raw: string) {
  const response = await page.request.put(endpoint, { data: raw, headers: { "Content-Type": "application/json" } });
  expect(response.ok(), await response.text()).toBeTruthy();
}

test.beforeEach(async ({ page }) => { await login(page); });

test("普通标签共用草稿并真实保存，重读和前台热更新使用新值", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  const slug = `settings-canonical-${Date.now()}`;
  try {
    await put(page, '{"version":2,"theme":{},"blog_extensions":{"unrelated":{"large":9007199254740993,"nil":null,"flag":false},"native_comments":{"notify":{"password":"fixture-only-password"}}}}');
    const post = await page.request.post("/api/v1/posts", {
      data: { slug, title: "设置恢复验收文章", content_md: "设置恢复验收正文。", status: "published",
        stellar_version: 2, stellar_config: "{}", blog_extensions: "{}" },
    });
    expect(post.ok(), await post.text()).toBeTruthy();
    await page.goto("/console/settings?tab=post");
    await field(page, "theme.article.style").locator("select").selectOption("story");
    await input(page, "theme.article.listing.cover_ratio").fill("1.6");
    await toggle(page, "blog_extensions.content.enabled", true);
    await toggle(page, "blog_extensions.content.reward.enabled", true);
    await input(page, "blog_extensions.content.reward.text").fill("设置测试打赏");

    // 未保存切换标签、完整原文和普通控件互相可见。
    await tab(page, "Stellar 2");
    await expect(page.locator("#stellar2-document")).toHaveValue(/"style"\s*:\s*"story"/);
    await tab(page, "评论");
    await expect(input(page, "blog_extensions.native_comments.notify.password")).toHaveValue("****");
    await toggle(page, "blog_extensions.native_comments.enabled", true);
    await input(page, "blog_extensions.native_comments.title").fill("设置测试评论");
    await tab(page, "SEO");
    await input(page, "blog_extensions.seo.default_description").fill("设置测试描述");
    await toggle(page, "blog_extensions.seo.no_index", true);
    await tab(page, "搜索");
    await field(page, "theme.search.provider").locator("select").selectOption("local");
    await toggle(page, "theme.search.local.include_content", false);
    await tab(page, "高级");
    for (const position of ["head_begin", "head_end", "body_begin", "body_end"]) {
      await input(page, `theme.inject.${position}`).fill(`<meta name="settings-${position}" content="current">`);
    }
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await save(page).click();
    expect((await response).ok()).toBeTruthy();
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
    const stored = await (await page.request.get(endpoint)).text();
    expect(stored).toMatch(/9007199254740993/);
    const document = JSON.parse(stored);
    expect(document.blog_extensions.unrelated).toMatchObject({ nil: null, flag: false });
    expect(document.blog_extensions.native_comments.notify.password).toBe("****");
    expect(document.theme.article).toMatchObject({ style: "story", listing: { cover_ratio: 1.6 } });
    expect(document.blog_extensions.content.reward.text).toBe("设置测试打赏");
    expect(document.blog_extensions.native_comments.title).toBe("设置测试评论");

    await page.reload();
    await tab(page, "文章");
    await expect(field(page, "theme.article.style").locator("select")).toHaveValue("story");
    await expect(input(page, "blog_extensions.content.reward.text")).toHaveValue("设置测试打赏");
    const publicPage = await page.request.get(`/post/${slug}`);
    expect(publicPage.ok()).toBeTruthy();
    const html = await publicPage.text();
    expect(html).toContain("设置测试打赏");
    expect(html).toContain("设置测试评论");
    expect(html).toContain("设置测试描述");
    expect(html).toMatch(/noindex/);
    for (const position of ["head_begin", "head_end", "body_begin", "body_end"]) expect(html).toContain(`settings-${position}`);
    const index = await (await page.request.get("/search.json")).json();
    const indexed = index.find((item: { path: string }) => item.path.includes(slug));
    expect(indexed).toBeTruthy();
    expect(indexed).not.toHaveProperty("content");
  } finally {
    await put(page, original);
    await page.request.delete(`/api/v1/posts/${slug}`);
  }
});

test("保存期间继续编辑，响应只更新提交基线；失败保留草稿", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await page.goto("/console/settings?tab=post");
    await input(page, "theme.article.listing.excerpt_length").fill("101");
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    let submitted = "";
    await page.route(`**${endpoint}`, async (route) => {
      if (route.request().method() !== "PUT") return route.fallback();
      submitted = route.request().postData() || "";
      await gate;
      // 这里只控制响应时序；真实持久化由上一用例独立覆盖。
      await route.fulfill({ status: 200, contentType: "application/json", body: submitted });
    });
    await save(page).click();
    await expect.poll(() => submitted).toContain('"excerpt_length":101');
    await input(page, "theme.article.listing.excerpt_length").fill("202");
    await tab(page, "搜索");
    await toggle(page, "theme.search.local.include_content", false);
    await expect(save(page)).toBeDisabled();
    release();
    await expect(save(page)).toBeEnabled();
    await tab(page, "文章");
    await expect(input(page, "theme.article.listing.excerpt_length")).toHaveValue("202");
    await expect(page.locator(".settings-dirty-badge")).toBeVisible();
    await page.unroute(`**${endpoint}`);
    await page.route(`**${endpoint}`, async (route) => {
      if (route.request().method() !== "PUT") return route.fallback();
      await route.fulfill({ status: 400, contentType: "application/json", body: JSON.stringify({
        type: "stellar_validation_failed", title: "校验失败", status: 400,
        detail: "测试保存失败", error: "测试保存失败", issues: [],
      }) });
    });
    await save(page).click();
    await expect(page.locator("#tabpanel-post").getByText("测试保存失败", { exact: true })).toBeVisible();
    await expect(input(page, "theme.article.listing.excerpt_length")).toHaveValue("202");
    await expect(page.locator(".settings-dirty-badge")).toBeVisible();
    expect(await (await page.request.get(endpoint)).text()).toBe(original);
  } finally {
    await page.unroute(`**${endpoint}`);
    await put(page, original);
  }
});

test("有效普通草稿通过真实预览渲染，预览不写入数据库", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=post");
  await field(page, "theme.article.listing.card_layout").locator("select").selectOption("classic");
  const response = page.waitForResponse((r) => r.url().endsWith("/api/v1/preview/site"));
  await page.getByRole("button", { name: "前端预览", exact: true }).click();
  const preview = await response;
  expect(preview.ok(), await preview.text()).toBeTruthy();
  const request = preview.request().postDataJSON();
  expect(Object.keys(request.sections).sort()).toEqual(["blog_extensions", "site", "stellar_v2"]);
  expect(request.sections.stellar_v2.theme.article.listing.card_layout).toBe("classic");
  await expect(page.locator(".preview-frame")).toBeVisible();
  expect(await (await page.request.get(endpoint)).text()).toBe(original);
});

test("完整配置语法错误不显示可误写的普通表单，修正后保留可编辑值", async ({ page }) => {
  await page.goto("/console/settings?tab=stellar2");
  const document = page.locator("#stellar2-document");
  await expect(document).toBeEnabled();
  const original = await document.inputValue();
  await document.fill("{");
  await tab(page, "文章");
  await expect(activeForm(page)).toHaveCount(0);
  await expect(page.locator("#tabpanel-post").getByRole("alert")).toContainText("完整配置暂时无法用于表单编辑");
  await tab(page, "Stellar 2");
  await document.fill(original);
  await tab(page, "文章");
  await expect(save(page)).toBeEnabled();
  await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
});

test("局部 JSON 和新增映射草稿跨标签保留，恢复默认解除保存锁", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await page.goto("/console/settings?tab=post");
    const mapping = field(page, "theme.article.category_colors");
    await mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true }).fill("测试分类");
    await expect(save(page)).toBeDisabled();
    await tab(page, "评论");
    await expect(save(page)).toBeDisabled();
    await tab(page, "Stellar 2");
    await expect(page.locator("#stellar2-document")).toBeDisabled();
    await tab(page, "文章");
    await expect(mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true })).toHaveValue("测试分类");
    await mapping.getByRole("button", { name: "分类颜色恢复默认", exact: true }).click();
    await expect(mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true })).toHaveValue("");
    await expect(save(page)).toBeEnabled();
    await tab(page, "搜索");
    const parameters = input(page, "theme.search.algolia");
    await parameters.fill("{");
    await expect(save(page)).toBeDisabled();
    await tab(page, "高级");
    await expect(save(page)).toBeDisabled();
    await tab(page, "搜索");
    await expect(parameters).toHaveValue("{");
    await parameters.fill('{"large":9007199254740993,"optional":null}');
    await expect(save(page)).toBeEnabled();
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await save(page).click();
    expect((await response).ok()).toBeTruthy();
    expect(await (await page.request.get(endpoint)).text()).toMatch(/9007199254740993/);
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
  } finally { await put(page, original); }
});

test("读取迁移门禁时保留宿主设置入口，普通标签不能写入默认配置", async ({ page }) => {
  let writes = 0;
  await page.route(`**${endpoint}`, async (route) => {
    if (route.request().method() !== "GET") { writes++; return route.abort(); }
    await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({
      type: "stellar_migration_required", title: "需要迁移", status: 409,
      detail: "先审阅完整迁移预览", error: "先审阅完整迁移预览", issues: [],
      migration: { source: "settings", path: "stellar_v2", reason: "missing", sections: ["article"] },
    }) });
  });
  await page.goto("/console/settings?tab=post");
  await expect(page.locator("#tabpanel-post")).toContainText("当前配置尚未迁移");
  await expect(activeForm(page)).toHaveCount(0);
  await tab(page, "基本");
  await expect(page.locator("#settings-site-title")).toBeEditable();
  await tab(page, "Stellar 2");
  await expect(page.getByRole("button", { name: "保存 Stellar 2 配置", exact: true })).toBeDisabled();
  expect(writes).toBe(0);
});

test("Go 搜索覆盖保留到用户显式切换，搜索接口随保存切换", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{"search":{"provider":null}},"blog_extensions":{"search":{"provider":"go","custom":9007199254740993}}}');
    await page.goto("/console/settings?tab=search");
    const provider = field(page, "theme.search.provider").locator("select");
    await expect(provider).toHaveValue("go");
    const persist = async () => {
      const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
      await save(page).click();
      expect((await response).ok()).toBeTruthy();
    };
    await input(page, "theme.search.local.cache_ttl_seconds").fill("13");
    await persist();
    expect((await page.request.get("/api/search?q=settings")).status()).toBe(200);
    const retained = await (await page.request.get(endpoint)).text();
    expect(retained).toMatch(/9007199254740993/);
    expect(JSON.parse(retained).blog_extensions.search.provider).toBe("go");
    await provider.selectOption("local");
    await persist();
    expect((await page.request.get("/api/search?q=settings")).status()).toBe(404);
    expect((await page.request.get("/search.json")).status()).toBe(200);
    expect(JSON.parse(await (await page.request.get(endpoint)).text()).blog_extensions.search).not.toHaveProperty("provider");
    await provider.selectOption("none");
    await persist();
    expect((await page.request.get("/search.json")).status()).toBe(404);
    await page.reload();
    await expect(field(page, "theme.search.provider").locator("select")).toHaveValue("none");
  } finally { await put(page, original); }
});

test("旧文章链接定位普通表单，取消离开时保留未保存草稿", async ({ page }) => {
  await page.goto("/console/settings?tab=article");
  await expect(page.locator("#tab-post")).toHaveAttribute("aria-selected", "true");
  const excerpt = input(page, "theme.article.listing.excerpt_length");
  const original = await excerpt.inputValue();
  const edited = original === "139" ? "140" : "139";
  await excerpt.fill(edited);
  await expect(page.locator(".settings-dirty-badge")).toBeVisible();

  const dialogs: string[] = [];
  page.once("dialog", async (dialog) => {
    dialogs.push(dialog.message());
    await dialog.dismiss();
  });
  await page.getByRole("link", { name: "仪表盘", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/settings\?tab=post$/);
  await expect(excerpt).toHaveValue(edited);
  expect(dialogs).toEqual(["您有未保存的设置更改，离开后修改将丢失，是否确认离开？"]);

  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("link", { name: "仪表盘", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard$/);
});

test("普通设置在六种宽度保持单卡片、正确边距和完整可用标签", async ({ page }) => {
  test.setTimeout(90_000);
  await page.goto("/console/settings?tab=post");
  for (const width of [390, 640, 641, 767, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const [label, key] of [["文章", "post"], ["评论", "comments"], ["SEO", "seo"], ["搜索", "search"], ["高级", "advanced"]]) {
      await tab(page, label);
      const panel = page.locator(`#tabpanel-${key}`);
      await expect(panel).toBeVisible();
      await expect(save(page)).toBeEnabled();
      await expect(panel.locator(".card-wrapper")).toHaveCount(0);
      const styles = await panel.locator(".settings-section").evaluateAll((sections) => sections.map((section) => {
        const body = section.querySelector(".settings-section-body")!;
        const style = getComputedStyle(body);
        return { padding: getComputedStyle(section).paddingLeft, background: style.backgroundColor,
          border: style.borderWidth, radius: style.borderRadius, shadow: style.boxShadow, bodyPadding: style.paddingLeft };
      }));
      expect(styles.length).toBeGreaterThan(0);
      for (const style of styles) expect(style).toEqual({ padding: width < 768 ? "16px" : "28px",
        background: "rgba(0, 0, 0, 0)", border: "0px", radius: "0px", shadow: "none", bodyPadding: "0px" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
      if (width === 390 || width === 1280) {
        await page.locator(".settings-card-tabs").scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.resolve(`docs/baselines/settings-canonical-forms/after-${key}-${width}.png`) });
      }
    }
  }
});
