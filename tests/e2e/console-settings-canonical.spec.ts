import { expect, test, type Page } from "@playwright/test";
import { openSettingsPage, openSettingsRaw, settingsNav } from "./support/settings";

const endpoint = "/api/v1/settings/stellar";
const activeForm = (page: Page) => page.locator(".canonical-settings-form:visible");
const field = (page: Page, key: string) => activeForm(page).locator(`[data-setting-path="${key}"]`);
const input = (page: Page, key: string) => field(page, key).locator("input,textarea").first();
const save = (page: Page) => activeForm(page).getByRole("button", { name: "保存全部主题设置", exact: true });
const aiSave = (page: Page) => activeForm(page).getByRole("button", { name: "保存 AI 设置", exact: true });

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
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

test("各设置页共用主题草稿并真实保存，重读和前台热更新使用新值", async ({ page }) => {
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
    await field(page, "theme.article.style").locator('input[type="radio"][value="story"]').check();
    await toggle(page, "blog_extensions.content.enabled", true);
    await toggle(page, "blog_extensions.content.reward.enabled", true);
    await input(page, "blog_extensions.content.reward.text").fill("设置测试打赏");
    await openSettingsPage(page, "listing");
    await input(page, "theme.article.listing.cover_ratio").fill("1.6");

    // 未保存切换页面、完整原文和普通控件互相可见。
    await openSettingsRaw(page);
    await expect(page.locator("#stellar2-document")).toHaveValue(/"style"\s*:\s*"story"/);
    await openSettingsPage(page, "comments");
    await expect(input(page, "blog_extensions.native_comments.notify.password")).toHaveValue("****");
    await toggle(page, "blog_extensions.native_comments.enabled", true);
    await input(page, "blog_extensions.native_comments.title").fill("设置测试评论");
    // 站内搜索与搜索引擎设置同在“SEO 与搜索”。
    await openSettingsPage(page, "seo");
    await input(page, "blog_extensions.seo.default_description").fill("设置测试描述");
    await toggle(page, "blog_extensions.seo.no_index", true);
    await field(page, "theme.search.provider").locator('input[type="radio"][value="local"]').check();
    await toggle(page, "theme.search.local.include_content", false);
    await openSettingsPage(page, "advanced");
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
    await openSettingsPage(page, "post");
    await expect(field(page, "theme.article.style").getByRole("radio", {name: "叙事文章", exact: true})).toBeChecked();
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
    await page.goto("/console/settings?tab=listing");
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
    await openSettingsPage(page, "seo");
    await toggle(page, "theme.search.local.include_content", false);
    await expect(save(page)).toBeDisabled();
    release();
    await expect(save(page)).toBeEnabled();
    await openSettingsPage(page, "listing");
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
    await expect(page.locator("#settings-page-listing").getByText("测试保存失败", { exact: true })).toBeVisible();
    await expect(input(page, "theme.article.listing.excerpt_length")).toHaveValue("202");
    await expect(page.locator(".settings-dirty-badge")).toBeVisible();
    expect(await (await page.request.get(endpoint)).text()).toBe(original);
  } finally {
    await page.unroute(`**${endpoint}`);
    await put(page, original);
  }
});

test("AI 设置写入独立 Blog 扩展，对话与向量接口分别保存且密钥占位保持", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(
      page,
      '{"version":2,"theme":{},"blog_extensions":{"ai":{' +
        '"base_url":"https://ai.e2e.test/v1","api_key":"e2e-only-key","model":"e2e-chat","embedding_model":"e2e-embedding",' +
        '"chat":{"base_url":"https://chat.e2e.test/v1","api_key":"e2e-chat-only-key","model":"e2e-chat-model"},' +
        '"embedding":{"base_url":"https://embedding.e2e.test/v1","api_key":"e2e-embedding-only-key","model":"e2e-embedding-model"},' +
        '"summary":{"enabled":false,"allow_user_api":false},"ask":{"enabled":false,"allow_user_llm":false}},' +
        '"unrelated":{"large":9007199254740993}}}',
    );
    // 脱敏后的读取原文就是表单基线；保存时除被编辑的叶子外必须逐字保留。
    const baseline = JSON.parse(await (await page.request.get(endpoint)).text());
    await page.goto("/console/assistant?tab=settings");
    // 两组接口分别读取自己的地址、模型与占位密钥。
    await expect(input(page, "blog_extensions.ai.chat.api_key")).toHaveValue("****");
    await expect(input(page, "blog_extensions.ai.embedding.api_key")).toHaveValue("****");
    await expect(input(page, "blog_extensions.ai.chat.base_url")).toHaveValue("https://chat.e2e.test/v1");
    await expect(input(page, "blog_extensions.ai.chat.model")).toHaveValue("e2e-chat-model");
    await expect(input(page, "blog_extensions.ai.embedding.base_url")).toHaveValue("https://embedding.e2e.test/v1");
    await expect(input(page, "blog_extensions.ai.embedding.model")).toHaveValue("e2e-embedding-model");
    // 旧平铺字段只作为留空回退，不再渲染为表单控件，避免覆盖未迁移的旧配置。
    for (const key of ["base_url", "api_key", "model", "embedding_model"])
      await expect(field(page, `blog_extensions.ai.${key}`)).toHaveCount(0);

    await toggle(page, "blog_extensions.ai.summary.enabled", true);
    await input(page, "blog_extensions.ai.chat.base_url").fill("https://chat-next.e2e.test/v1");
    await input(page, "blog_extensions.ai.chat.model").fill("e2e-chat-next");
    await input(page, "blog_extensions.ai.embedding.base_url").fill("https://embedding-next.e2e.test/v1");
    await input(page, "blog_extensions.ai.embedding.model").fill("e2e-embedding-next");
    const request = page.waitForRequest((r) => r.url().endsWith(endpoint) && r.method() === "PUT");
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await aiSave(page).click();
    // 两组 api_key 仍以 **** 占位提交，由服务端各自恢复；旧平铺字段、无关字段与大整数原样透传。
    const submittedRequest = await request;
    const payload = submittedRequest.postData() || "";
    // 旧平铺字段按原文留在原位置：AI 表单只替换被编辑的叶子，不删除也不重建 ai 对象。
    for (const raw of [
      '"base_url": "https://ai.e2e.test/v1"',
      '"model": "e2e-chat"',
      '"embedding_model": "e2e-embedding"',
    ])
      expect(payload).toContain(raw);
    expect(payload.indexOf('"embedding_model": "e2e-embedding"')).toBeLessThan(payload.indexOf('"summary"'));
    const expected = structuredClone(baseline.blog_extensions);
    expected.ai.chat.base_url = "https://chat-next.e2e.test/v1";
    expected.ai.chat.model = "e2e-chat-next";
    expected.ai.embedding.base_url = "https://embedding-next.e2e.test/v1";
    expected.ai.embedding.model = "e2e-embedding-next";
    expected.ai.summary.enabled = true;
    expect(submittedRequest.postDataJSON().blog_extensions).toEqual(expected);
    expect((await response).ok()).toBeTruthy();
    const stored = await (await page.request.get(endpoint)).text();
    expect(stored).toMatch(/9007199254740993/);
    expect(JSON.parse(stored).blog_extensions).toEqual(expected);
    await page.reload();
    await expect(input(page, "blog_extensions.ai.chat.api_key")).toHaveValue("****");
    await expect(input(page, "blog_extensions.ai.embedding.api_key")).toHaveValue("****");
    await expect(input(page, "blog_extensions.ai.chat.base_url")).toHaveValue("https://chat-next.e2e.test/v1");
    await expect(input(page, "blog_extensions.ai.embedding.model")).toHaveValue("e2e-embedding-next");
    await expect(field(page, "blog_extensions.ai.summary.enabled").getByRole("switch")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  } finally {
    await put(page, original);
  }
});

test("有效普通草稿通过真实预览渲染，预览不写入数据库", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  await page.goto("/console/settings?tab=listing");
  await field(page, "theme.article.listing.card_layout").locator('input[type="radio"][value="classic"]').check();
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
  // 旧的 ?tab=stellar2 链接落到“代码与原文”并展开完整配置。
  await page.goto("/console/settings?tab=stellar2");
  await expect(settingsNav(page, "advanced")).toHaveAttribute("aria-current", "page");
  const document = page.locator("#stellar2-document");
  await expect(document).toBeVisible();
  await expect(document).toBeEnabled();
  const original = await document.inputValue();
  await document.fill("{");
  await openSettingsPage(page, "post");
  await expect(activeForm(page)).toHaveCount(0);
  await expect(page.locator("#settings-page-post").getByRole("alert")).toContainText("完整配置暂时无法用于表单编辑");
  await openSettingsRaw(page);
  await document.fill(original);
  await openSettingsPage(page, "post");
  await expect(activeForm(page)).toHaveCount(1);
  await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
});

test("局部 JSON 和新增映射草稿跨页面保留，恢复默认解除保存锁", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await page.goto("/console/settings?tab=listing");
    // 先制造一处真实修改，保存按钮因此可用；之后只有未完成的输入才会把它锁住。
    await toggle(page, "theme.article.listing.show_tags", true);
    await expect(save(page)).toBeEnabled();
    const mapping = field(page, "theme.article.category_colors");
    await mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true }).fill("测试分类");
    await expect(save(page)).toBeDisabled();
    await expect(page.locator(".settings-save-bar").getByRole("alert")).toContainText("请先添加或清空尚未加入的映射项");
    await openSettingsPage(page, "comments");
    await expect(save(page)).toBeDisabled();
    await openSettingsRaw(page);
    await expect(page.locator("#stellar2-document")).toBeDisabled();
    await openSettingsPage(page, "listing");
    await expect(mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true })).toHaveValue("测试分类");
    await mapping.getByRole("button", { name: "分类颜色恢复默认", exact: true }).click();
    await expect(mapping.getByRole("textbox", { name: "新增分类颜色名称", exact: true })).toHaveValue("");
    await expect(save(page)).toBeEnabled();
    await openSettingsPage(page, "seo");
    // 完整原文仍可诊断无效输入；Algolia 参数已不在设置页提供结构化控件，但仍可通过完整原文保留和编辑
    // （schema.json 未移除该节点，旧配置需要原样保存），这里验证的就是原文编辑链路。
    await openSettingsRaw(page);
    const parameters = page.locator("#stellar2-document");
    const draft = await parameters.inputValue();
    await parameters.fill("{");
    await openSettingsPage(page, "seo");
    await expect(activeForm(page)).toHaveCount(0);
    await openSettingsRaw(page);
    const next = JSON.parse(draft);
    next.theme.search = { ...(next.theme.search ?? {}), algolia: { large: "REPLACE_LARGE", optional: null } };
    await parameters.fill(JSON.stringify(next).replace('"REPLACE_LARGE"', '9007199254740993'));
    await openSettingsPage(page, "seo");
    const response = page.waitForResponse((r) => r.url().endsWith(endpoint) && r.request().method() === "PUT");
    await save(page).click();
    expect((await response).ok()).toBeTruthy();
    expect(await (await page.request.get(endpoint)).text()).toMatch(/9007199254740993/);
    await expect(page.locator(".settings-dirty-badge")).toHaveCount(0);
  } finally { await put(page, original); }
});

test("读取迁移门禁时保留宿主设置入口，普通页面不能写入默认配置", async ({ page }) => {
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
  await expect(page.locator("#settings-page-post")).toContainText("当前配置尚未迁移");
  await expect(activeForm(page)).toHaveCount(0);
  await openSettingsPage(page, "site");
  await expect(page.locator("#settings-site-title")).toBeEditable();
  await openSettingsRaw(page);
  await expect(page.getByRole("button", { name: "保存 Stellar 2 配置", exact: true })).toBeDisabled();
  expect(writes).toBe(0);
});

test("Go 搜索覆盖保留到用户显式切换，搜索接口随保存切换", async ({ page }) => {
  const original = await (await page.request.get(endpoint)).text();
  try {
    await put(page, '{"version":2,"theme":{"search":{"provider":null}},"blog_extensions":{"search":{"provider":"go","custom":9007199254740993}}}');
    // 旧的 ?tab=search 链接落到“SEO 与搜索”。
    await page.goto("/console/settings?tab=search");
    await expect(settingsNav(page, "seo")).toHaveAttribute("aria-current", "page");
    const provider = field(page, "theme.search.provider");
    await expect(provider.locator('input[type="radio"][value="go"]')).toBeChecked();
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
    await provider.locator('input[type="radio"][value="local"]').check();
    await persist();
    expect((await page.request.get("/api/search?q=settings")).status()).toBe(404);
    expect((await page.request.get("/search.json")).status()).toBe(200);
    expect(JSON.parse(await (await page.request.get(endpoint)).text()).blog_extensions.search).not.toHaveProperty("provider");
    await provider.locator('input[type="radio"][value="none"]').check();
    await persist();
    expect((await page.request.get("/search.json")).status()).toBe(404);
    await page.reload();
    await expect(field(page, "theme.search.provider").locator('input[type="radio"][value="none"]')).toBeChecked();
  } finally { await put(page, original); }
});

test("旧文章链接定位文章页，取消离开时保留未保存草稿", async ({ page }) => {
  await page.goto("/console/settings?tab=article");
  await expect(settingsNav(page, "post")).toHaveAttribute("aria-current", "page");
  await expect(page).toHaveURL(/\/console\/settings\?tab=post$/);
  const related = input(page, "theme.article.related_posts_limit");
  const original = await related.inputValue();
  const edited = original === "7" ? "8" : "7";
  await related.fill(edited);
  await expect(page.locator(".settings-dirty-badge")).toBeVisible();

  const dialogs: string[] = [];
  page.once("dialog", async (dialog) => {
    dialogs.push(dialog.message());
    await dialog.dismiss();
  });
  await page.getByRole("link", { name: "仪表盘", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/settings\?tab=post$/);
  await expect(related).toHaveValue(edited);
  expect(dialogs).toEqual(["您有未保存的设置更改，离开后修改将丢失，是否确认离开？"]);

  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("link", { name: "仪表盘", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard$/);
});

test("普通设置在六种宽度保持无卡片外框、正确边距和完整可用页面", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  await page.goto("/console/settings?tab=post");
  for (const width of [390, 640, 641, 767, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const key of ["post", "comments", "seo", "listing", "advanced"]) {
      await openSettingsPage(page, key);
      const panel = page.locator(`#settings-page-${key}`);
      await expect(panel).toBeVisible();
      await expect(panel.getByRole("button", { name: "保存全部主题设置", exact: true })).toBeVisible();
      await expect(panel.locator(".card-wrapper")).toHaveCount(0);
      const styles = await panel.locator(".settings-sec").evaluateAll((sections) => sections.map((section) => {
        const style = getComputedStyle(section);
        return { paddingX: [style.paddingLeft, style.paddingRight], background: style.backgroundColor,
          borderX: [style.borderLeftWidth, style.borderRightWidth], borderBottom: style.borderBottomWidth,
          radius: style.borderRadius, shadow: style.boxShadow };
      }));
      // 横向页面边距由 .settings-container 承担（手机 16px / 桌面 32px），分区自身不再叠加横向 padding、外框或背景。
      const containerPadding = await page.locator(".settings-container").evaluate((el) => getComputedStyle(el).paddingLeft);
      expect(containerPadding).toBe(width < 768 ? "16px" : "32px");
      expect(styles.length).toBeGreaterThan(0);
      for (const style of styles) expect(style).toEqual({ paddingX: ["0px", "0px"],
        background: "rgba(0, 0, 0, 0)", borderX: ["0px", "0px"], borderBottom: "0px", radius: "0px", shadow: "none" });
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
      if (width === 390 || width === 1280) {
        // 测试不得覆写受跟踪基线目录，证据统一写入 Playwright 输出目录。
        await page.screenshot({ path: testInfo.outputPath(`settings-${key}-${width}.png`) });
      }
    }
  }
});

test("统一保存条与预览面板不透明，横向边距由容器统一提供", async ({ page }, testInfo) => {
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/console/settings?tab=appearance");
    // 十个主题页的表单同时挂载（非当前页 hidden），只检查当前可见页的保存栏。
    const actions = page.locator(".settings-save-bar:visible").first();
    await expect(actions).toBeVisible();
    const bar = await actions.evaluate((el) => {
      const style = getComputedStyle(el);
      return {
        background: style.backgroundColor,
        paddingLeft: style.paddingLeft,
        paddingRight: style.paddingRight,
        position: style.position,
        bottom: style.bottom,
      };
    });
    expect(bar.position, "保存条保持 sticky").toBe("sticky");
    expect(bar.bottom).toBe("0px");
    expect(bar.paddingLeft, "横向留白由 .settings-container 统一提供").toBe("0px");
    expect(bar.paddingRight).toBe("0px");
    expect(bar.background, "保存条必须不透明，滚动时不得透出正文").not.toBe("rgba(0, 0, 0, 0)");
    expect(bar.background).toMatch(/^rgb\(/);
    const containerPadding = await page.locator(".settings-container").evaluate((el) => getComputedStyle(el).paddingLeft);
    expect(containerPadding).toBe(width < 768 ? "16px" : "32px");

    await page.getByRole("button", { name: "前端预览", exact: true }).click();
    const panel = page.locator(".preview-panel");
    await expect(panel).toBeVisible();
    const preview = await panel.evaluate((el) => {
      const header = el.querySelector(".preview-panel-header") as HTMLElement;
      return {
        background: getComputedStyle(el).backgroundColor,
        headerBackground: getComputedStyle(header).backgroundColor,
      };
    });
    expect(preview.background, "预览面板必须不透明").not.toBe("rgba(0, 0, 0, 0)");
    expect(preview.headerBackground, "预览头部必须不透明").not.toBe("rgba(0, 0, 0, 0)");
    await page.screenshot({ path: testInfo.outputPath(`canonical-appearance-${width}.png`) });
    await page.getByRole("button", { name: "关闭预览", exact: true }).click();
    await expect(panel).toBeHidden();
  }
});
