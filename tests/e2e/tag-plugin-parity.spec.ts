import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";

// 使用 run-server.sh 的隔离站点；远端内容仅通过浏览器路由夹具提供。
const metadata = { stellar_version: 2, stellar_config: "{}", blog_extensions: "{}" };
const nested = `{% tabs active:1 %}
<!-- tab 外层一 -->
{% folding open:false color:cyan 外层折叠 %}
{% note color:green 标题 内层正文 %}
{% endfolding %}
{% tabs active:1 %}
<!-- tab 内层一 -->
内层第一段
<!-- tab 内层二 -->
内层第二段
{% endtabs %}
<!-- tab 外层二 -->
外层第二段
{% endtabs %}`;
const literal = "```markdown\n{% note 示例 不应激活 %}\n```\n\n`{% button 示例 / %}`";
const body = `${nested}\n\n${literal}`;

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}

async function create(page: Page, kind: "posts" | "pages", content = "原始正文", status = "draft") {
  const slug = `tag-parity-${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  const response = await page.request.post(`/api/v1/${kind}`, {
    data: { ...metadata, slug, title: "标签完整适配验收", content_md: content, status },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  return slug;
}

async function editorMarkdown(page: Page) {
  return page.evaluate(() => (window as any).__consoleTiptapEditor.storage.markdown.getMarkdown() as string);
}

test.beforeEach(async ({ page }) => login(page));

test("完整目录：54 项官方标签与 Blog 扩展示例可解析、往返并生成正式运行时文档", async ({ page }) => {
  test.setTimeout(90_000);
  const { items } = await (await page.request.get("/api/v1/plugins/catalog")).json();
  expect(items.filter((item: any) => !item.name.startsWith("blog_")).length).toBe(54);
  for (const item of items) {
    const parse = await page.request.post("/api/v1/plugins/shortcode/parse", { data: { raw: item.example } });
    expect(parse.ok(), `${item.name}: ${await parse.text()}`).toBeTruthy();
    const parsed = await parse.json();
    expect(parsed.editable, item.name).toBe(true);
    const serialize = await page.request.post("/api/v1/plugins/shortcode/serialize", {
      data: { name: item.name, values: parsed.values, present: parsed.present },
    });
    expect(serialize.ok(), `${item.name}: ${await serialize.text()}`).toBeTruthy();
    const { raw } = await serialize.json();
    const render = async (markdown: string) => {
      const response = await page.request.post("/api/v1/preview/markdown", {
        data: {
          ...metadata,
          markdown,
          preview_profile: "component-library",
          mode: "content",
          preview_token: "tag_parity_preview_token",
        },
      });
      expect(response.ok(), `${item.name}: ${await response.text()}`).toBeTruthy();
      const result = await response.json();
      expect(result.document, item.name).toContain("stellar-runtime-config");
      expect(result.preview_token).toBe("tag_parity_preview_token");
      return result.html;
    };
    expect(await render(raw), item.name).toBe(await render(item.example));
  }
});

for (const kind of ["posts", "pages"] as const) {
  test(`${kind}：未知、未闭合与交错源码保留，保存拒绝后不覆盖已有正文`, async ({ page }) => {
    const sources = [
      "{% custom %}\n<!-- tab Keep -->\n{% note Title Body %}\n{% endcustom %}",
      "{% tabs %}\n<!-- tab Keep -->\n{% note Title Body %}",
      "{% box %}\n{% tabs %}\n<!-- tab Keep -->\n{% endbox %}\n{% endtabs %}",
    ];
    const slug = await create(page, kind);
    for (const source of sources) {
      await page.goto(`/console/${kind}/edit/${slug}`);
      await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
      await page.evaluate(
        (markdown) => (window as any).__consoleTiptapEditor.commands.setContent(markdown, true),
        source,
      );
      await expect.poll(() => editorMarkdown(page)).toBe(source);
      const saved = page.waitForResponse(
        (r) => r.url().endsWith(`/api/v1/${kind}/${slug}`) && r.request().method() === "PUT",
      );
      await page.getByRole("button", { name: kind === "posts" ? "保存草稿" : "保存", exact: true }).click();
      // 既有后端严格校验拒绝未知/残缺结构；不得为测试放宽保存语义。
      expect((await saved).status()).toBe(400);
      expect((await (await page.request.get(`/api/v1/${kind}/${slug}`)).json()).content_md).toBe("原始正文");
      await expect(page.locator(".stellar-tag-node")).toHaveCount(1);
      expect(await editorMarkdown(page)).toBe(source);
    }
  });
  test(`${kind}：富文本嵌套与代码示例保存重开保持原始标签`, async ({ page }) => {
    const slug = await create(page, kind);
    await page.goto(`/console/${kind}/edit/${slug}`);
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
    await page.evaluate((markdown) => (window as any).__consoleTiptapEditor.commands.setContent(markdown, true), body);
    await expect(page.locator(".stellar-tag-node")).toHaveCount(1);
    await expect(page.locator("pre code")).toContainText("{% note 示例 不应激活 %}");
    const response = page.waitForResponse(
      (r) => r.url().endsWith(`/api/v1/${kind}/${slug}`) && r.request().method() === "PUT",
    );
    await page.getByRole("button", { name: kind === "posts" ? "保存草稿" : "保存", exact: true }).click();
    expect((await response).ok()).toBeTruthy();
    const persisted = await (await page.request.get(`/api/v1/${kind}/${slug}`)).json();
    expect(persisted.content_md).toContain(nested);
    expect(persisted.content_md).toContain("{% note 示例 不应激活 %}");
    expect(persisted.content_md).toContain("`{% button 示例 / %}`");
    expect(persisted.content_md).not.toContain("data-stellar-tag");
    await page.goto(`/console/${kind}/edit/${slug}`);
    await expect(page.locator(".stellar-tag-node")).toHaveCount(1);
    expect(await editorMarkdown(page)).toContain(nested);
  });

  test(`${kind}：共享面板可交互预览 tabs 并插入保存`, async ({ page }) => {
    const slug = await create(page, kind);
    await page.goto(`/console/${kind}/edit/${slug}`);
    await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
    if (kind === "posts") await page.locator("button[aria-pressed]", { hasText: "设置" }).click();
    else await page.getByRole("button", { name: "标签", exact: true }).click();
    const panel = page.locator("[data-shortcode-insert-panel]");
    await panel.getByLabel("选择标签插件").selectOption("tabs");
    await panel
      .getByRole("textbox", { name: "Shortcode 源码", exact: true })
      .fill("{% tabs active:1 %}\n<!-- tab 第一页 -->\n第一页正文\n<!-- tab 第二页 -->\n第二页正文\n{% endtabs %}");
    const frame = panel.frameLocator("iframe");
    await expect(frame.locator(".tab-pane").first()).toBeVisible();
    await frame.getByText("第二页", { exact: true }).click();
    await expect(frame.getByText("第二页正文", { exact: true })).toBeVisible();
    await panel.getByRole("button", { name: "插入到正文" }).click();
    await expect.poll(() => editorMarkdown(page)).toContain("<!-- tab 第二页 -->");
    // 页面头部沿现有行为下滚隐藏；回到顶部使用真正的保存按钮。
    if (kind === "pages") await page.locator("main.content").evaluate((el) => el.scrollTo(0, 0));
    const saved = page.waitForResponse(
      (r) => r.url().endsWith(`/api/v1/${kind}/${slug}`) && r.request().method() === "PUT",
    );
    await page.getByRole("button", { name: kind === "posts" ? "保存草稿" : "保存", exact: true }).click();
    expect((await saved).ok()).toBeTruthy();
    await page.goto(`/console/${kind}/edit/${slug}`);
    await expect(page.locator(".stellar-tag-node")).toHaveCount(1);
    expect(await editorMarkdown(page)).toContain("第二页正文");
  });
}

test("Studio：示例参数同步、直接源码保留未知参数，切换插件不复用旧值", async ({ page }) => {
  await page.goto("/console/plugins/shortcodes");
  await page.getByPlaceholder("搜索插件...").fill("note");
  await page.locator(".studio-list-item").first().click();
  const source = page.locator("textarea.studio-code-editor");
  await source.fill("{% note color:green 新标题 新正文 %}");
  await expect(page.locator("#schema-field-title")).toHaveValue("新标题");
  await page.locator("#schema-field-title").fill("表单标题");
  await expect(source).toHaveValue(/表单标题/);
  await source.fill("{% note future:keep color:green 原样标题 原样正文 %}");
  await expect(page.getByText(/源码编辑：/)).toBeVisible();
  await expect(page.locator("#schema-field-title")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "复制示例", exact: true })).toBeEnabled();
  await expect(source).toHaveValue("{% note future:keep color:green 原样标题 原样正文 %}");
  await page.getByPlaceholder("搜索插件...").fill("button");
  await page.locator(".studio-list-item").first().click();
  await expect(page.locator("#schema-field-url")).toHaveValue("https://example.com/");
  await expect(source).not.toHaveValue(/future:keep/);
});

async function remount(page: Page) {
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        // 生产使用 page-controls；tabs:register 是其完成重新绑定的真实事件。
        window.addEventListener("tabs:register", () => resolve(), { once: true });
        window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true }));
        window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true }));
      }),
  );
}

const lifecycleTabs = "{% tabs %}\n<!-- tab 页签一 -->\n内容一\n<!-- tab 页签二 -->\n内容二\n{% endtabs %}";

test("公开页面：真实动态标签净化远端内容，相对 API 与保留 DOM 重挂可用", async ({ page }) => {
  const calls: Record<string, number> = {};
  const malicious = '<img src=x onerror="window.__tagXss++">';
  await page.addInitScript(() => {
    (window as any).__tagXss = 0;
  });
  await page.route("**/__tag-fixture/**", async (route) => {
    const key = new URL(route.request().url()).pathname.split("/").at(-1)!;
    calls[key] = (calls[key] || 0) + 1;
    const data: Record<string, unknown> = {
      friends: [
        {
          title: malicious,
          url: "javascript:window.__tagXss++",
          avatar: "/assets/img/default/avatar.svg",
          labels: [{ name: "<b>普通标签</b>", color: "123456" }],
        },
      ],
      sites: {
        content: [
          {
            title: "<b>普通站点</b>",
            description: malicious,
            url: "https://example.com/",
            cover: "/assets/img/default/cover.svg",
          },
        ],
      },
      timeline: [
        {
          title: malicious,
          body:
            "**正常 Markdown**\n\n" +
            malicious +
            '<a href="java&#x73;cript:window.__tagXss++">危险链接</a><iframe srcdoc="<script>window.__tagXss++</script>"></iframe>',
          html_url: "javascript:window.__tagXss++",
          created_at: "2026-10-26T00:00:00Z",
        },
      ],
      empty: [],
    };
    await route.fulfill({ json: data[key] });
  });
  const content =
    "{% friends api:/__tag-fixture/friends %}\n\n{% sites api:/__tag-fixture/sites %}\n\n{% timeline api:/__tag-fixture/timeline %}\n{% endtimeline %}\n\n{% friends api:/__tag-fixture/empty %}";
  const slug = await create(page, "pages", content + "\n\n" + lifecycleTabs, "published");
  await page.goto(`/${slug}`);
  await expect(page.locator(".ds-friends .user-card")).toHaveCount(1);
  await expect(page.locator(".ds-sites .site-card")).toHaveCount(1);
  await expect(page.locator(".ds-timeline strong")).toHaveText("正常 Markdown");
  await expect(page.locator(".ds-friends .image-caption")).toHaveText(malicious);
  await expect(page.locator(".ds-sites .title")).toHaveText("<b>普通站点</b>");
  await expect.poll(() => calls.empty).toBe(1);
  const unsafe = await page.locator(".ds-friends, .ds-sites, .ds-timeline").evaluateAll((roots) =>
    roots.flatMap((root) =>
      Array.from(root.querySelectorAll("*")).flatMap((el) =>
        Array.from(el.attributes)
          .filter(
            (a) => /^on/i.test(a.name) || (/^(href|src)$/i.test(a.name) && /^\s*(javascript|data):/i.test(a.value)),
          )
          .map((a) => a.name),
      ),
    ),
  );
  expect(unsafe).toEqual([]);
  expect(await page.evaluate(() => (window as any).__tagXss)).toBe(0);
  await expect(page.locator(".ds-timeline script, .ds-timeline iframe")).toHaveCount(0);
  await remount(page);
  await expect(page.locator(".ds-friends .user-card")).toHaveCount(1);
  await expect(page.locator(".ds-sites .site-card")).toHaveCount(1);
  await expect(page.locator(".ds-timeline .timenode")).toHaveCount(1);
  expect(calls).toEqual({ friends: 1, sites: 1, timeline: 1, empty: 1 });
});

test("公开页面：动态请求失败后可重挂重试，卸载后迟到响应不能追加", async ({ page }) => {
  let retry = false;
  let calls = 0;
  let lateRelease: (() => void) | undefined;
  await page.route("**/__tag-fixture/retry", async (route) => {
    calls++;
    await route.fulfill(retry ? { json: [{ title: "已恢复", url: "/" }] } : { status: 503, json: {} });
  });
  await page.route("**/__tag-fixture/late", async (route) => {
    await new Promise<void>((resolve) => {
      lateRelease = resolve;
    });
    await route.fulfill({ json: [{ title: "过期响应", url: "/" }] }).catch(() => {});
  });
  const slug = await create(
    page,
    "pages",
    "{% friends api:/__tag-fixture/retry %}\n\n{% friends api:/__tag-fixture/late %}\n\n" + lifecycleTabs,
    "published",
  );
  await page.goto(`/${slug}`);
  await expect.poll(() => calls).toBeGreaterThan(0);
  await expect.poll(() => !!lateRelease).toBe(true);
  await expect(page.locator('.ds-friends[data-api="/__tag-fixture/retry"] .loading-wrap.error')).toBeVisible({
    timeout: 20_000,
  });
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true })));
  lateRelease!();
  // 取消已触发后，通过后续浏览器任务确认迟到正文没有进入保留 DOM。
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await expect(page.locator(".user-card")).toHaveCount(0);
  retry = true;
  await page.unroute("**/__tag-fixture/late");
  await page.route("**/__tag-fixture/late", (route) => route.fulfill({ json: [] }));
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
  await expect(page.locator(".ds-friends .image-caption")).toHaveText("已恢复");
});

test("四种外观、浅深色和自动配色：嵌套标签可交互且窄屏无页面溢出", async ({ page }) => {
  test.setTimeout(90_000);
  const endpoint = "/api/v1/settings/stellar";
  const original = await (await page.request.get(endpoint)).json();
  const slug = await create(page, "pages", nested, "published");
  const results: unknown[] = [];
  try {
    for (const preset of ["card", "glass", "minimal", "flat"]) {
      const backgrounds: Record<string, string> = {};
      for (const [color_scheme, os, width] of [
        ["light", "light", 1280],
        ["dark", "dark", 390],
        ["auto", "light", 390],
        ["auto", "dark", 1280],
      ] as const) {
        const result = await page.request.put(endpoint, {
          data: { ...original, theme: { ...original.theme, appearance: { preset, color_scheme } } },
        });
        expect(result.ok(), await result.text()).toBeTruthy();
        await page.emulateMedia({ colorScheme: os });
        await page.setViewportSize({ width, height: 900 });
        await page.goto(`/${slug}`);
        if (color_scheme !== "auto") await expect(page.locator("html")).toHaveAttribute("data-theme", os);
        else await expect(page.locator("html")).not.toHaveAttribute("data-theme");
        await expect(page.locator(`link[href*="main-${preset}.css"]`)).toHaveCount(1);
        const background = await page
          .locator("html")
          .evaluate((el) => getComputedStyle(el).getPropertyValue("--background").trim());
        if (color_scheme === "auto") expect(background).toBe(backgrounds[os]);
        else backgrounds[os] = background;
        const outer = page.locator(".tag-plugin.tabs").first();
        await outer.locator(":scope > .nav-tabs").getByText("外层二", { exact: true }).click();
        await expect(outer.getByText("外层第二段", { exact: true })).toBeVisible();
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
        expect(overflow, `${preset}/${color_scheme}/${width}`).toBe(false);
        results.push({ preset, color_scheme, os, width, background, overflow });
        if (color_scheme === "dark")
          await page.screenshot({
            path: `docs/baselines/tag-plugin-parity/after-${preset}-mobile.png`,
            fullPage: true,
          });
      }
    }
    await writeFile("docs/baselines/tag-plugin-parity/appearance-browser.json", JSON.stringify(results, null, 2));
  } finally {
    expect((await page.request.put(endpoint, { data: original })).ok()).toBeTruthy();
  }
});
